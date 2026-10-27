package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool      *pgxpool.Pool
	lookupTXT func(context.Context, string) ([]string, error)
}

func NewPostgres(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	return NewPostgresWithTXTResolver(ctx, databaseURL, net.DefaultResolver.LookupTXT)
}

// NewPostgresWithTXTResolver permits a trusted DNS resolver to be supplied by the caller.
func NewPostgresWithTXTResolver(ctx context.Context, databaseURL string, lookupTXT func(context.Context, string) ([]string, error)) (*PostgresStore, error) {
	if lookupTXT == nil {
		return nil, fmt.Errorf("TXT resolver is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	config.MaxConns = 20
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	return &PostgresStore{pool: pool, lookupTXT: lookupTXT}, nil
}

func (s *PostgresStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *PostgresStore) Close()                         { s.pool.Close() }

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// lockMutableTenant serializes all tenant writes with lifecycle transitions.
// Call it before locking or changing any child row in the same transaction.
func lockMutableTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	status, err := lockTenantStatus(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	if status == domain.StatusSuspended || status == domain.StatusArchived {
		return fmt.Errorf("%w: tenant is %s", domain.ErrInvalidStateTransition, status)
	}
	return nil
}

func lockTenantStatus(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (string, error) {
	var statusID int16
	if err := tx.QueryRow(ctx, `SELECT status_id FROM md_tenant WHERE id = $1 FOR UPDATE`, tenantID).Scan(&statusID); err != nil {
		return "", translateError(err)
	}
	var status string
	if err := tx.QueryRow(ctx, `SELECT code FROM di_tenant_status WHERE id = $1`, statusID).Scan(&status); err != nil {
		return "", err
	}
	return status, nil
}

func withTx[T any](ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := fn(tx)
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}
	return result, nil
}

func insertOutbox(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, eventType string, data any) error {
	eventID := uuid.New()
	payload, err := json.Marshal(map[string]any{
		"event_id":    eventID,
		"event_type":  eventType,
		"occurred_at": time.Now().UTC(),
		"tenant_id":   tenantID,
		"data":        data,
	})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO sys_outbox_event (id, aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, 'tenant', $2, $3, $4)`, eventID, tenantID, eventType, payload)
	return err
}

func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTenantNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uq_md_tenant_slug":
			return domain.ErrSlugAlreadyExists
		case "uq_md_tenant_domain_domain":
			return domain.ErrDomainAlreadyExists
		}
	}
	return err
}
