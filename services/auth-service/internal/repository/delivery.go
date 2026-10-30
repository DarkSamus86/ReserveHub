package repository

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const invitationDeliveryEvent = "auth.invitation.delivery.v1"

type InvitationSender interface {
	SendInvitation(context.Context, string, string) error
}
type invitationCommand struct {
	Email string `json:"email"`
	Token string `json:"token"`
}
type encryptedCommand struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// RunInvitationDelivery keeps secrets only in memory or AES-GCM ciphertext in the durable outbox.
func (p *Postgres) RunInvitationDelivery(ctx context.Context, key []byte, sender InvitationSender, ttl time.Duration) error {
	if sender == nil || len(key) != 32 || ttl <= 0 {
		return errors.New("invitation delivery is not configured")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		stepCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err = p.queueInvitation(stepCtx, aead, ttl)
		if err == nil {
			err = p.sendInvitation(stepCtx, aead, sender)
		}
		cancel()
		if err != nil && ctx.Err() == nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (p *Postgres) queueInvitation(ctx context.Context, aead cipher.AEAD, ttl time.Duration) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id, email string
	err = tx.QueryRow(ctx, `SELECT id::text,email FROM auth_service.tr_tenant_invitation WHERE status='PENDING_DELIVERY' ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var raw [32]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(token))
	command, err := json.Marshal(invitationCommand{Email: email, Token: token})
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	sealed := aead.Seal(nil, nonce, command, []byte(id))
	payload, err := json.Marshal(encryptedCommand{Nonce: base64.RawURLEncoding.EncodeToString(nonce), Ciphertext: base64.RawURLEncoding.EncodeToString(sealed)})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE auth_service.tr_tenant_invitation SET status='READY_TO_SEND',token_hash=$2,expires_at=now()+($3::bigint * interval '1 second') WHERE id=$1 AND status='PENDING_DELIVERY'`, id, hash[:], int64(ttl.Seconds()))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO auth_service.sys_outbox_event(aggregate_type,aggregate_id,event_type,payload) VALUES ('tenant_invitation',$1,$2,$3)`, id, invitationDeliveryEvent, payload)
	if err != nil {
		return err
	}
	if err = insertAuthEvent(ctx, tx, "tenant_invitation", id, "auth.invitation.queued.v1", map[string]any{"invitation_id": id}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) sendInvitation(ctx context.Context, aead cipher.AEAD, sender InvitationSender) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var eventID, invitationID, status string
	var payload []byte
	var expired bool
	err = tx.QueryRow(ctx, `SELECT o.id::text,i.id::text,i.status,i.expires_at<=now(),o.payload FROM auth_service.sys_outbox_event o JOIN auth_service.tr_tenant_invitation i ON i.id=o.aggregate_id WHERE o.event_type=$1 AND o.published_at IS NULL AND o.next_attempt_at<=now() ORDER BY o.next_attempt_at,o.created_at,o.id FOR UPDATE OF o,i SKIP LOCKED LIMIT 1`, invitationDeliveryEvent).Scan(&eventID, &invitationID, &status, &expired, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status != "READY_TO_SEND" || expired {
		_, err = tx.Exec(ctx, `UPDATE auth_service.sys_outbox_event SET published_at=now(),payload='{}'::jsonb WHERE id=$1`, eventID)
		if err != nil {
			return err
		}
		if expired && status == "READY_TO_SEND" {
			if err = insertAuthEvent(ctx, tx, "tenant_invitation", invitationID, "auth.invitation.delivery_expired.v1", map[string]any{"invitation_id": invitationID}); err != nil {
				return err
			}
		}
		return tx.Commit(ctx)
	}
	var encrypted encryptedCommand
	if err = json.Unmarshal(payload, &encrypted); err != nil {
		return fmt.Errorf("decode encrypted invitation command: %w", err)
	}
	nonce, err := base64.RawURLEncoding.DecodeString(encrypted.Nonce)
	if err != nil {
		return err
	}
	sealed, err := base64.RawURLEncoding.DecodeString(encrypted.Ciphertext)
	if err != nil {
		return err
	}
	plain, err := aead.Open(nil, nonce, sealed, []byte(invitationID))
	if err != nil {
		return fmt.Errorf("decrypt invitation command: %w", err)
	}
	var command invitationCommand
	if err = json.Unmarshal(plain, &command); err != nil {
		return err
	}
	if err = sender.SendInvitation(ctx, command.Email, command.Token); err != nil {
		_, updateErr := tx.Exec(ctx, `UPDATE auth_service.sys_outbox_event SET attempts=attempts+1,next_attempt_at=now()+interval '1 minute' WHERE id=$1`, eventID)
		if updateErr != nil {
			return updateErr
		}
		return tx.Commit(ctx)
	}
	result, err := tx.Exec(ctx, `UPDATE auth_service.tr_tenant_invitation SET status='SENT',sent_at=now() WHERE id=$1 AND status='READY_TO_SEND' AND expires_at>now()`, invitationID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		_, err = tx.Exec(ctx, `UPDATE auth_service.sys_outbox_event SET published_at=now(),payload='{}'::jsonb WHERE id=$1`, eventID)
		if err != nil {
			return err
		}
		if err = insertAuthEvent(ctx, tx, "tenant_invitation", invitationID, "auth.invitation.delivery_expired.v1", map[string]any{"invitation_id": invitationID}); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `UPDATE auth_service.sys_outbox_event SET published_at=now(),attempts=attempts+1,payload='{}'::jsonb WHERE id=$1`, eventID)
	if err != nil {
		return err
	}
	if err = insertAuthEvent(ctx, tx, "tenant_invitation", invitationID, "auth.invitation.sent.v1", map[string]any{"invitation_id": invitationID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
