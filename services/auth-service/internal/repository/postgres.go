package repository

import (
	"context"
	"errors"
	"fmt"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close()                         { p.pool.Close() }
func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

func (p *Postgres) CreateUser(ctx context.Context, email, passwordHash string, role auth.Role) (auth.User, error) {
	var user auth.User
	err := p.pool.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO auth_service.users (id, email, password_hash, role_id)
			SELECT $1, $2, $3, roles.id
			FROM auth_service.roles AS roles
			WHERE roles.scope = 'platform' AND roles.tenant_id IS NULL AND roles.code = $4
			RETURNING id, email, role_id, created_at
		)
		SELECT inserted.id::text, inserted.email, roles.code, inserted.created_at
		FROM inserted
		JOIN auth_service.roles AS roles ON roles.id = inserted.role_id`, auth.NewUserID(), email, passwordHash, string(role)).
		Scan(&user.ID, &user.Email, &user.Role, &user.CreatedAt)
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
		SELECT users.id::text, users.email, users.password_hash, roles.code, users.created_at
		FROM auth_service.users AS users
		JOIN auth_service.roles AS roles ON roles.id = users.role_id
		WHERE users.email = $1`, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.UserCredential{}, auth.ErrInvalidCredentials
	}
	return user, err
}

func (p *Postgres) UserByID(ctx context.Context, id string) (auth.User, error) {
	var user auth.User
	err := p.pool.QueryRow(ctx, `
		SELECT users.id::text, users.email, roles.code, users.created_at
		FROM auth_service.users AS users
		JOIN auth_service.roles AS roles ON roles.id = users.role_id
		WHERE users.id = $1`, id).
		Scan(&user.ID, &user.Email, &user.Role, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	return user, err
}

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
