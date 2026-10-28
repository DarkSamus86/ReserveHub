package repository

import (
	"context"
	"errors"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (p *Postgres) CreateUser(ctx context.Context, email, passwordHash string, role auth.Role) (auth.User, error) {
	var user auth.User
	err := p.pool.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO auth_service.md_user (id, email, password_hash, role_id)
			SELECT $1, $2, $3, roles.id
			FROM auth_service.md_role AS roles
			WHERE roles.scope = 'platform' AND roles.tenant_id IS NULL AND roles.code = $4
			RETURNING id, email, role_id, created_at
		)
		SELECT inserted.id::text, inserted.email, roles.code, roles.scope, inserted.created_at
		FROM inserted
		JOIN auth_service.md_role AS roles ON roles.id = inserted.role_id`, auth.NewUserID(), email, passwordHash, string(role)).
		Scan(&user.ID, &user.Email, &user.Role, &user.Scope, &user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return auth.User{}, auth.ErrEmailTaken
		}
		return auth.User{}, err
	}
	return user, nil
}

func (p *Postgres) UserByEmail(ctx context.Context, email string) (auth.UserCredential, error) {
	var user auth.UserCredential
	err := p.pool.QueryRow(ctx, `
		SELECT users.id::text, users.email, COALESCE(users.password_hash, ''), roles.code, roles.scope, users.created_at
		FROM auth_service.md_user AS users
		JOIN auth_service.md_role AS roles ON roles.id = users.role_id
		WHERE users.email = $1 AND users.status = 'ACTIVE'`, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.Scope, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.UserCredential{}, auth.ErrInvalidCredentials
	}
	return user, err
}

func (p *Postgres) UserByID(ctx context.Context, id string) (auth.User, error) {
	var user auth.User
	err := p.pool.QueryRow(ctx, `
		SELECT users.id::text, users.email, roles.code, roles.scope, users.created_at
		FROM auth_service.md_user AS users
		JOIN auth_service.md_role AS roles ON roles.id = users.role_id
		WHERE users.id = $1 AND users.status = 'ACTIVE'`, id).
		Scan(&user.ID, &user.Email, &user.Role, &user.Scope, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	return user, err
}
