package repository

import (
	"context"
	"errors"
	"fmt"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"github.com/jackc/pgx/v5/pgconn"
)

func (p *Postgres) HasAdmin(ctx context.Context) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM auth_service.users AS users
				JOIN auth_service.roles AS roles ON roles.id = users.role_id
				WHERE roles.scope = 'platform' AND roles.tenant_id IS NULL AND roles.code = 'admin'
			)`).Scan(&exists)
	return exists, err
}

func (p *Postgres) BootstrapAdmin(ctx context.Context, email, passwordHash string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin admin bootstrap: %w", err)
	}
	defer tx.Rollback(ctx)

	// Serialize concurrent bootstrap jobs so none can replace or add an administrator after the first one.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(734081, 1)`); err != nil {
		return fmt.Errorf("lock admin bootstrap: %w", err)
	}
	var hasAdmin bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM auth_service.users AS users
			JOIN auth_service.roles AS roles ON roles.id = users.role_id
			WHERE roles.scope = 'platform' AND roles.tenant_id IS NULL AND roles.code = 'admin'
		)`).Scan(&hasAdmin); err != nil {
		return fmt.Errorf("check for administrator: %w", err)
	}
	if hasAdmin {
		return tx.Commit(ctx)
	}

	result, err := tx.Exec(ctx, `
		INSERT INTO auth_service.users (id, email, password_hash, role_id)
		SELECT $1, $2, $3, roles.id
		FROM auth_service.roles AS roles
		WHERE roles.scope = 'platform' AND roles.tenant_id IS NULL AND roles.code = 'admin'`,
		auth.NewUserID(), email, passwordHash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("bootstrap email already belongs to another account: %w", auth.ErrEmailTaken)
		}
		return fmt.Errorf("create bootstrap administrator: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("platform admin role is missing; run auth migrations first")
	}
	return tx.Commit(ctx)
}
