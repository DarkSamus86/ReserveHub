package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (p *Postgres) AcceptInvitation(ctx context.Context, hash []byte, passwordHash string) (auth.AcceptInvitationResult, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return auth.AcceptInvitationResult{}, err
	}
	defer tx.Rollback(ctx)
	var invitationID, membershipID, userID string
	err = tx.QueryRow(ctx, `SELECT i.id::text,m.id::text,m.user_id::text FROM auth_service.tr_tenant_invitation i JOIN auth_service.ln_user_tenant_membership m ON m.id=i.membership_id WHERE i.token_hash=$1`, hash).Scan(&invitationID, &membershipID, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.AcceptInvitationResult{}, auth.ErrInvalidSecret
	}
	if err != nil {
		return auth.AcceptInvitationResult{}, err
	}
	var userStatus, email string
	err = tx.QueryRow(ctx, `SELECT status,email FROM auth_service.md_user WHERE id=$1 FOR UPDATE`, userID).Scan(&userStatus, &email)
	if err != nil {
		return auth.AcceptInvitationResult{}, err
	}
	var result auth.AcceptInvitationResult
	var membershipStatus string
	err = tx.QueryRow(ctx, `SELECT tenant_id::text,status FROM auth_service.ln_user_tenant_membership WHERE id=$1 AND user_id=$2 FOR UPDATE`, membershipID, userID).Scan(&result.TenantID, &membershipStatus)
	if err != nil {
		return auth.AcceptInvitationResult{}, err
	}
	var invitationStatus, inviteEmail string
	var actualHash []byte
	var valid bool
	err = tx.QueryRow(ctx, `SELECT status,email,token_hash,expires_at>now() FROM auth_service.tr_tenant_invitation WHERE id=$1 AND membership_id=$2 FOR UPDATE`, invitationID, membershipID).Scan(&invitationStatus, &inviteEmail, &actualHash, &valid)
	if err != nil {
		return auth.AcceptInvitationResult{}, err
	}
	if invitationStatus != "SENT" || !valid || membershipStatus != "PENDING" || email != inviteEmail || !bytes.Equal(actualHash, hash) {
		return auth.AcceptInvitationResult{}, auth.ErrInvalidSecret
	}
	if userStatus == "INVITED" {
		if passwordHash == "" {
			return auth.AcceptInvitationResult{}, auth.ErrInvalidSecret
		}
		if _, err = tx.Exec(ctx, `UPDATE auth_service.md_user SET status='ACTIVE',password_hash=$2 WHERE id=$1 AND status='INVITED'`, userID, passwordHash); err != nil {
			return auth.AcceptInvitationResult{}, err
		}
	} else if userStatus != "ACTIVE" || passwordHash != "" {
		return auth.AcceptInvitationResult{}, auth.ErrInvalidSecret
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_service.ln_user_tenant_membership SET status='ACTIVE',activated_at=now(),updated_at=now() WHERE id=$1 AND status='PENDING'`, membershipID); err != nil {
		return auth.AcceptInvitationResult{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_service.tr_tenant_invitation SET status='ACCEPTED',accepted_at=now() WHERE id=$1 AND status='SENT'`, invitationID); err != nil {
		return auth.AcceptInvitationResult{}, err
	}
	if err = insertAuthEvent(ctx, tx, "tenant_invitation", invitationID, "auth.invitation.accepted.v1", map[string]any{"tenant_id": result.TenantID, "membership_id": membershipID, "user_id": userID}); err != nil {
		return auth.AcceptInvitationResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return auth.AcceptInvitationResult{}, fmt.Errorf("commit invitation acceptance: %w", err)
	}
	result.MembershipID = membershipID
	result.Status = "active"
	return result, nil
}

func (p *Postgres) ReissueInvitation(ctx context.Context, id string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Delivery claims outbox before invitation; use the same lock order.
	rows, err := tx.Query(ctx, `SELECT id FROM auth_service.sys_outbox_event WHERE aggregate_id=$1 AND event_type=$2 AND published_at IS NULL ORDER BY id FOR UPDATE`, id, invitationDeliveryEvent)
	if err != nil {
		return err
	}
	for rows.Next() {
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var status string
	var expired bool
	err = tx.QueryRow(ctx, `SELECT status,COALESCE(expires_at<=now(),false) FROM auth_service.tr_tenant_invitation WHERE id=$1 FOR UPDATE`, id).Scan(&status, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrInvalidSecret
	}
	if err != nil {
		return err
	}
	if !expired || (status != "READY_TO_SEND" && status != "SENT") {
		return auth.ErrProvisionConflict
	}
	_, err = tx.Exec(ctx, `UPDATE auth_service.tr_tenant_invitation SET status='PENDING_DELIVERY',token_hash=NULL,expires_at=NULL,sent_at=NULL WHERE id=$1`, id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE auth_service.sys_outbox_event SET published_at=now(),payload='{}'::jsonb WHERE aggregate_id=$1 AND event_type=$2 AND published_at IS NULL`, id, invitationDeliveryEvent)
	if err != nil {
		return err
	}
	if err = insertAuthEvent(ctx, tx, "tenant_invitation", id, "auth.invitation.reissued.v1", map[string]any{"invitation_id": id}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) InvitationStatus(ctx context.Context, id string) (auth.InvitationStatus, error) {
	var result auth.InvitationStatus
	err := p.pool.QueryRow(ctx, `SELECT id::text,status,expires_at FROM auth_service.tr_tenant_invitation WHERE id=$1`, id).Scan(&result.InvitationID, &result.Status, &result.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.InvitationStatus{}, auth.ErrInvitationNotFound
	}
	if err != nil {
		return auth.InvitationStatus{}, err
	}
	return result, nil
}
