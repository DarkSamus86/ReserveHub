package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (p *Postgres) CreateSession(ctx context.Context, id, userID string, hash []byte, expiresAt time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var created string
	err = tx.QueryRow(ctx, `INSERT INTO auth_service.tr_auth_session (id,user_id,current_refresh_token_hash,expires_at)
		SELECT $1,u.id,$3,$4 FROM auth_service.md_user u WHERE u.id=$2 AND u.status='ACTIVE'
		RETURNING id::text`, id, userID, hash, expiresAt).Scan(&created)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrUnauthenticated
	}
	if err != nil {
		return fmt.Errorf("create auth session: %w", err)
	}
	if err = insertAuthEvent(ctx, tx, "auth_session", id, "auth.session.created.v1", map[string]any{"user_id": userID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) RotateSession(ctx context.Context, id string, oldHash, newHash []byte, now time.Time) (auth.User, time.Time, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, time.Time{}, err
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `SELECT user_id::text FROM auth_service.tr_auth_session WHERE id=$1`, id).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, time.Time{}, auth.ErrInvalidSecret
	}
	if err != nil {
		return auth.User{}, time.Time{}, err
	}
	var user auth.User
	err = tx.QueryRow(ctx, `SELECT u.id::text,u.email,r.code,r.scope,u.created_at FROM auth_service.md_user u JOIN auth_service.md_role r ON r.id=u.role_id WHERE u.id=$1 AND u.status='ACTIVE' FOR UPDATE OF u`, userID).Scan(&user.ID, &user.Email, &user.Role, &user.Scope, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, time.Time{}, auth.ErrInvalidSecret
	}
	if err != nil {
		return auth.User{}, time.Time{}, err
	}
	var rotated string
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `UPDATE auth_service.tr_auth_session SET current_refresh_token_hash=$3,rotation_counter=rotation_counter+1,rotated_at=$4
		WHERE id=$1 AND current_refresh_token_hash=$2 AND revoked_at IS NULL AND expires_at>$4 RETURNING id::text,expires_at`, id, oldHash, newHash, now).Scan(&rotated, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, time.Time{}, auth.ErrInvalidSecret
	}
	if err != nil {
		return auth.User{}, time.Time{}, fmt.Errorf("rotate auth session: %w", err)
	}
	if err = insertAuthEvent(ctx, tx, "auth_session", id, "auth.session.refreshed.v1", map[string]any{"user_id": user.ID}); err != nil {
		return auth.User{}, time.Time{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return auth.User{}, time.Time{}, fmt.Errorf("commit refresh: %w", err)
	}
	return user, expiresAt, nil
}

func (p *Postgres) RevokeSession(ctx context.Context, id string, hash []byte) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `UPDATE auth_service.tr_auth_session SET revoked_at=now() WHERE id=$1 AND current_refresh_token_hash=$2 AND revoked_at IS NULL AND expires_at>now() RETURNING user_id::text`, id, hash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrInvalidSecret
	}
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if err = insertAuthEvent(ctx, tx, "auth_session", id, "auth.session.revoked.v1", map[string]any{"user_id": userID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
