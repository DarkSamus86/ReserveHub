package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Operation struct {
	ID           uuid.UUID
	RequestHash  []byte
	TenantID     uuid.NullUUID
	InvitationID uuid.NullUUID
	Status       string
	InviteStatus string
	SafeError    string
}

var ErrNotFound = errors.New("provision operation not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close()                         { s.pool.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) Get(ctx context.Context, id uuid.UUID) (Operation, error) {
	op := Operation{ID: id}
	err := s.pool.QueryRow(ctx, `
		SELECT tenant_id, invitation_id, status, COALESCE(invite_status,''), COALESCE(safe_error,'')
		FROM admin_service.tr_tenant_provision_operation WHERE operation_id=$1`, id).
		Scan(&op.TenantID, &op.InvitationID, &op.Status, &op.InviteStatus, &op.SafeError)
	if errors.Is(err, pgx.ErrNoRows) {
		return Operation{}, ErrNotFound
	}
	return op, err
}

type Transaction struct {
	tx        pgx.Tx
	Operation Operation
	actorID   uuid.UUID
}

func (s *Store) Begin(ctx context.Context, id, actor uuid.UUID, hash, requestJSON []byte, ownerEmail string) (*Transaction, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO admin_service.tr_tenant_provision_operation
			(operation_id,request_hash,actor_id,owner_email,tenant_request,status)
		VALUES ($1,$2,$3,$4,$5,'requested') ON CONFLICT (operation_id) DO NOTHING`,
		id, hash, actor, ownerEmail, requestJSON)
	if err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	op := Operation{ID: id}
	err = tx.QueryRow(ctx, `
		SELECT request_hash,tenant_id,invitation_id,status,COALESCE(invite_status,''),COALESCE(safe_error,'')
		FROM admin_service.tr_tenant_provision_operation WHERE operation_id=$1 FOR UPDATE`, id).
		Scan(&op.RequestHash, &op.TenantID, &op.InvitationID, &op.Status, &op.InviteStatus, &op.SafeError)
	if err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return &Transaction{tx: tx, Operation: op, actorID: actor}, nil
}

func (t *Transaction) SetTenant(ctx context.Context, tenantID uuid.UUID) error {
	_, err := t.tx.Exec(ctx, `
		UPDATE admin_service.tr_tenant_provision_operation SET tenant_id=$2,updated_at=now()
		WHERE operation_id=$1`, t.Operation.ID, tenantID)
	if err == nil {
		t.Operation.TenantID = uuid.NullUUID{UUID: tenantID, Valid: true}
	}
	return err
}

func (t *Transaction) SetInvitation(ctx context.Context, invitationID uuid.UUID) error {
	_, err := t.tx.Exec(ctx, `
		UPDATE admin_service.tr_tenant_provision_operation SET invitation_id=$2,updated_at=now()
		WHERE operation_id=$1`, t.Operation.ID, invitationID)
	if err == nil {
		t.Operation.InvitationID = uuid.NullUUID{UUID: invitationID, Valid: true}
	}
	return err
}

func (t *Transaction) SetResult(ctx context.Context, status, inviteStatus, safeError string) error {
	_, err := t.tx.Exec(ctx, `
		UPDATE admin_service.tr_tenant_provision_operation
		SET status=$2,invite_status=NULLIF($3,''),safe_error=NULLIF($4,''),updated_at=now()
		WHERE operation_id=$1`, t.Operation.ID, status, inviteStatus, safeError)
	if err == nil {
		t.Operation.Status = status
		t.Operation.InviteStatus = inviteStatus
		t.Operation.SafeError = safeError
		_, err = t.tx.Exec(ctx, `
			INSERT INTO admin_service.sys_outbox_event
			    (aggregate_type, aggregate_id, event_type, payload)
			VALUES ('tenant_provision_operation', $1, 'admin.tenant_provision.status_changed',
			        jsonb_build_object('operation_id', $1::uuid::text, 'actor_id', $2::uuid::text,
			                           'tenant_id', $3::uuid, 'status', $4::text,
			                           'safe_error', NULLIF($5::text, ''),
			                           'invite_status', NULLIF($6::text, ''),
			                           'invitation_id', $7::uuid))`,
			t.Operation.ID, t.actorID, t.Operation.TenantID, status, safeError, inviteStatus, t.Operation.InvitationID)
	}
	return err
}

func (t *Transaction) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *Transaction) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

func (s *Store) RecordInvitationReissue(ctx context.Context, op Operation, actor uuid.UUID, result string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO admin_service.sys_outbox_event
		    (aggregate_type,aggregate_id,event_type,payload)
		VALUES ('tenant_provision_operation',$1,'admin.invitation.reissue_requested.v1',
		        jsonb_build_object('operation_id',$1::uuid::text,'actor_id',$2::uuid::text,
		                           'tenant_id',$3::uuid,'invitation_id',$4::uuid,
		                           'result',$5::text))`,
		op.ID, actor, op.TenantID, op.InvitationID, result)
	return err
}
