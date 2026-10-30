package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (p *Postgres) ProvisionOwner(ctx context.Context, input auth.ProvisionOwnerInput) (auth.ProvisionOwnerResult, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return auth.ProvisionOwnerResult{}, fmt.Errorf("begin owner provision: %w", err)
	}
	defer tx.Rollback(ctx)

	// One tenant's initial role, membership, and request must be created in order.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, input.TenantID); err != nil {
		return auth.ProvisionOwnerResult{}, fmt.Errorf("lock owner provision: %w", err)
	}

	var previous auth.ProvisionOwnerResult
	var previousTenant, previousSlug, previousEmail, previousStatus string
	err = tx.QueryRow(ctx, `
		SELECT provision.tenant_id::text, provision.slug, provision.owner_email,
		       provision.user_id::text, provision.membership_id::text, provision.invitation_id::text,
		       invitation.status
		FROM auth_service.sys_owner_provision_request AS provision
		JOIN auth_service.tr_tenant_invitation AS invitation ON invitation.id = provision.invitation_id
		WHERE provision.operation_id = $1`, input.OperationID).
		Scan(&previousTenant, &previousSlug, &previousEmail, &previous.UserID, &previous.MembershipID, &previous.InvitationID, &previousStatus)
	if err == nil {
		if previousTenant != input.TenantID || previousSlug != input.Slug || previousEmail != input.Email {
			return auth.ProvisionOwnerResult{}, auth.ErrProvisionConflict
		}
		previous.TenantID = previousTenant
		previous.Status = strings.ToLower(previousStatus)
		if err := tx.Commit(ctx); err != nil {
			return auth.ProvisionOwnerResult{}, fmt.Errorf("commit owner provision replay: %w", err)
		}
		return previous, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return auth.ProvisionOwnerResult{}, fmt.Errorf("lookup owner provision: %w", err)
	}

	var otherOperation string
	err = tx.QueryRow(ctx, `SELECT operation_id::text FROM auth_service.sys_owner_provision_request WHERE tenant_id = $1`, input.TenantID).Scan(&otherOperation)
	if err == nil {
		return auth.ProvisionOwnerResult{}, auth.ErrProvisionConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return auth.ProvisionOwnerResult{}, fmt.Errorf("lookup tenant provision: %w", err)
	}

	var roleExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM auth_service.md_role WHERE tenant_id = $1)`, input.TenantID).Scan(&roleExists); err != nil {
		return auth.ProvisionOwnerResult{}, fmt.Errorf("check tenant role: %w", err)
	}
	if roleExists {
		return auth.ProvisionOwnerResult{}, auth.ErrProvisionConflict
	}
	var roleID string
	err = tx.QueryRow(ctx, `
		INSERT INTO auth_service.md_role (tenant_id, code, name, scope)
		VALUES ($1, $2, $2, 'tenant') RETURNING id::text`, input.TenantID, input.Slug).Scan(&roleID)
	if err != nil {
		return auth.ProvisionOwnerResult{}, provisionSQLError("create tenant role", err)
	}

	userID := uuid.NewString()
	_, err = tx.Exec(ctx, `
		INSERT INTO auth_service.md_user (id, email, password_hash, role_id, status)
		SELECT $1, $2, NULL, roles.id, 'INVITED'
		FROM auth_service.md_role AS roles
		WHERE roles.scope = 'platform' AND roles.code = 'user' AND roles.tenant_id IS NULL
		ON CONFLICT (email) DO NOTHING`, userID, input.Email)
	if err != nil {
		return auth.ProvisionOwnerResult{}, provisionSQLError("create invited identity", err)
	}
	var result auth.ProvisionOwnerResult
	err = tx.QueryRow(ctx, `SELECT id::text FROM auth_service.md_user WHERE email = $1 FOR UPDATE`, input.Email).Scan(&result.UserID)
	if err != nil {
		return auth.ProvisionOwnerResult{}, fmt.Errorf("load owner identity: %w", err)
	}
	result.TenantID = input.TenantID
	result.MembershipID = uuid.NewString()
	err = tx.QueryRow(ctx, `
		INSERT INTO auth_service.ln_user_tenant_membership
		    (id, user_id, tenant_id, tenant_role_id, permission_profile, status)
		VALUES ($1, $2, $3, $4, 'tenant_owner', 'PENDING')
		RETURNING id::text`, result.MembershipID, result.UserID, input.TenantID, roleID).Scan(&result.MembershipID)
	if err != nil {
		return auth.ProvisionOwnerResult{}, provisionSQLError("create owner membership", err)
	}
	result.InvitationID = uuid.NewString()
	err = tx.QueryRow(ctx, `
		INSERT INTO auth_service.tr_tenant_invitation (id, membership_id, email, status)
		VALUES ($1, $2, $3, 'PENDING_DELIVERY') RETURNING id::text`, result.InvitationID, result.MembershipID, input.Email).
		Scan(&result.InvitationID)
	if err != nil {
		return auth.ProvisionOwnerResult{}, provisionSQLError("create owner invitation", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO auth_service.sys_owner_provision_request
		    (operation_id, tenant_id, slug, owner_email, user_id, membership_id, invitation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		input.OperationID, input.TenantID, input.Slug, input.Email, result.UserID, result.MembershipID, result.InvitationID)
	if err != nil {
		return auth.ProvisionOwnerResult{}, provisionSQLError("record owner provision", err)
	}
	if err = insertAuthEvent(ctx, tx, "tenant_invitation", result.InvitationID, "auth.invitation.created.v1", map[string]any{"tenant_id": input.TenantID, "membership_id": result.MembershipID, "user_id": result.UserID}); err != nil {
		return auth.ProvisionOwnerResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.ProvisionOwnerResult{}, provisionSQLError("commit owner provision", err)
	}
	result.Status = "pending_delivery"
	result.Created = true
	return result, nil
}

func provisionSQLError(action string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return auth.ErrProvisionConflict
	}
	return fmt.Errorf("%s: %w", action, err)
}
