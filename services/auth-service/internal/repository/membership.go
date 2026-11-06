package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (p *Postgres) ListUserMemberships(ctx context.Context, userID string) ([]auth.TenantMembership, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT m.id::text, m.tenant_id::text, r.code, m.permission_profile, m.status, m.created_at, m.activated_at
		FROM auth.ln_user_tenant_membership AS m
		JOIN auth.md_role AS r ON r.id = m.tenant_role_id
		WHERE m.user_id = $1 AND m.status = 'ACTIVE'
		ORDER BY m.created_at, m.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	memberships := make([]auth.TenantMembership, 0)
	for rows.Next() {
		var m auth.TenantMembership
		if err := rows.Scan(&m.MembershipID, &m.TenantID, &m.RoleCode, &m.PermissionProfile, &m.Status, &m.CreatedAt, &m.ActivatedAt); err != nil {
			return nil, err
		}
		m.Status = strings.ToLower(m.Status)
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}

func (p *Postgres) ActiveMembership(ctx context.Context, userID, tenantID string) (auth.TenantMembership, error) {
	var m auth.TenantMembership
	err := p.pool.QueryRow(ctx, `
		SELECT m.id::text, m.user_id::text, m.tenant_id::text, r.code, m.permission_profile, m.status, m.created_at, m.activated_at
		FROM auth.ln_user_tenant_membership AS m
		JOIN auth.md_role AS r ON r.id = m.tenant_role_id
		JOIN auth.md_user AS u ON u.id = m.user_id
		WHERE m.user_id = $1 AND m.tenant_id = $2 AND m.status = 'ACTIVE' AND u.status = 'ACTIVE'`,
		userID, tenantID).Scan(
		&m.MembershipID, &m.UserID, &m.TenantID, &m.RoleCode, &m.PermissionProfile, &m.Status, &m.CreatedAt, &m.ActivatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.TenantMembership{}, auth.ErrMembershipNotFound
	}
	if err != nil {
		return auth.TenantMembership{}, err
	}
	m.Status = strings.ToLower(m.Status)
	return m, nil
}

func (p *Postgres) ListTenantMembers(ctx context.Context, tenantID string) ([]auth.TenantMembership, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT m.id::text, m.user_id::text, u.email, m.tenant_id::text, r.code, m.permission_profile, m.status,
		       COALESCE(i.id::text, ''), COALESCE(i.status, ''), m.created_at, m.activated_at, m.revoked_at
		FROM auth.ln_user_tenant_membership AS m
		JOIN auth.md_user AS u ON u.id = m.user_id
		JOIN auth.md_role AS r ON r.id = m.tenant_role_id
		LEFT JOIN auth.tr_tenant_invitation AS i ON i.membership_id = m.id
		WHERE m.tenant_id = $1
		ORDER BY m.created_at, m.id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := make([]auth.TenantMembership, 0)
	for rows.Next() {
		var m auth.TenantMembership
		if err := rows.Scan(
			&m.MembershipID, &m.UserID, &m.Email, &m.TenantID, &m.RoleCode, &m.PermissionProfile, &m.Status,
			&m.InvitationID, &m.InvitationStatus, &m.CreatedAt, &m.ActivatedAt, &m.RevokedAt,
		); err != nil {
			return nil, err
		}
		m.Status = strings.ToLower(m.Status)
		m.InvitationStatus = strings.ToLower(m.InvitationStatus)
		members = append(members, m)
	}
	return members, rows.Err()
}

func (p *Postgres) InviteTenantMember(ctx context.Context, input auth.InviteMemberInput) (auth.InviteMemberResult, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return auth.InviteMemberResult{}, fmt.Errorf("begin member invite: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, input.TenantID); err != nil {
		return auth.InviteMemberResult{}, fmt.Errorf("lock tenant member invite: %w", err)
	}

	var roleID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM auth.md_role WHERE tenant_id = $1 AND scope = 'tenant'`, input.TenantID).Scan(&roleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.InviteMemberResult{}, auth.ErrProvisionConflict
	}
	if err != nil {
		return auth.InviteMemberResult{}, fmt.Errorf("lookup tenant role: %w", err)
	}

	newUserID := uuid.NewString()
	_, err = tx.Exec(ctx, `
		INSERT INTO auth.md_user (id, email, password_hash, role_id, status)
		SELECT $1, $2, NULL, roles.id, 'INVITED'
		FROM auth.md_role AS roles
		WHERE roles.scope = 'platform' AND roles.code = 'user' AND roles.tenant_id IS NULL
		ON CONFLICT (email) DO NOTHING`, newUserID, input.Email)
	if err != nil {
		return auth.InviteMemberResult{}, provisionSQLError("create invited member identity", err)
	}

	var result auth.InviteMemberResult
	result.TenantID = input.TenantID
	if err := tx.QueryRow(ctx, `SELECT id::text FROM auth.md_user WHERE email = $1 FOR UPDATE`, input.Email).Scan(&result.UserID); err != nil {
		return auth.InviteMemberResult{}, fmt.Errorf("load member identity: %w", err)
	}

	var existingID, existingStatus string
	err = tx.QueryRow(ctx, `
		SELECT id::text, status FROM auth.ln_user_tenant_membership
		WHERE user_id = $1 AND tenant_id = $2 FOR UPDATE`, result.UserID, input.TenantID).Scan(&existingID, &existingStatus)
	if err == nil {
		if existingStatus != "REVOKED" {
			return auth.InviteMemberResult{}, auth.ErrProvisionConflict
		}
		result.MembershipID = existingID
		_, err = tx.Exec(ctx, `
			UPDATE auth.ln_user_tenant_membership
			SET permission_profile = $2, status = 'PENDING', activated_at = NULL, revoked_at = NULL, updated_at = now()
			WHERE id = $1`, existingID, input.PermissionProfile)
		if err != nil {
			return auth.InviteMemberResult{}, err
		}
		err = tx.QueryRow(ctx, `
			UPDATE auth.tr_tenant_invitation
			SET email = $2, status = 'PENDING_DELIVERY', token_hash = NULL, expires_at = NULL, sent_at = NULL, accepted_at = NULL, revoked_at = NULL
			WHERE membership_id = $1 RETURNING id::text`, existingID, input.Email).Scan(&result.InvitationID)
		if errors.Is(err, pgx.ErrNoRows) {
			result.InvitationID = uuid.NewString()
			err = tx.QueryRow(ctx, `
				INSERT INTO auth.tr_tenant_invitation (id, membership_id, email, status)
				VALUES ($1, $2, $3, 'PENDING_DELIVERY') RETURNING id::text`,
				result.InvitationID, existingID, input.Email).Scan(&result.InvitationID)
		}
		if err != nil {
			return auth.InviteMemberResult{}, err
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		result.MembershipID = uuid.NewString()
		err = tx.QueryRow(ctx, `
			INSERT INTO auth.ln_user_tenant_membership
			    (id, user_id, tenant_id, tenant_role_id, permission_profile, status)
			VALUES ($1, $2, $3, $4, $5, 'PENDING')
			RETURNING id::text`,
			result.MembershipID, result.UserID, input.TenantID, roleID, input.PermissionProfile).Scan(&result.MembershipID)
		if err != nil {
			return auth.InviteMemberResult{}, provisionSQLError("create member membership", err)
		}
		result.InvitationID = uuid.NewString()
		err = tx.QueryRow(ctx, `
			INSERT INTO auth.tr_tenant_invitation (id, membership_id, email, status)
			VALUES ($1, $2, $3, 'PENDING_DELIVERY') RETURNING id::text`,
			result.InvitationID, result.MembershipID, input.Email).Scan(&result.InvitationID)
		if err != nil {
			return auth.InviteMemberResult{}, provisionSQLError("create member invitation", err)
		}
	} else {
		return auth.InviteMemberResult{}, err
	}

	if err := insertAuthEvent(ctx, tx, "tenant_invitation", result.InvitationID, "auth.invitation.created.v1", map[string]any{
		"tenant_id":          input.TenantID,
		"membership_id":      result.MembershipID,
		"user_id":            result.UserID,
		"actor_id":           input.ActorID,
		"permission_profile": input.PermissionProfile,
	}); err != nil {
		return auth.InviteMemberResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.InviteMemberResult{}, fmt.Errorf("commit member invitation: %w", err)
	}
	result.Status = "pending_delivery"
	return result, nil
}

func (p *Postgres) RevokeTenantMember(ctx context.Context, tenantID, membershipID, actorID string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var status, userID, profile string
	err = tx.QueryRow(ctx, `
		SELECT status, user_id::text, permission_profile
		FROM auth.ln_user_tenant_membership
		WHERE id = $1 AND tenant_id = $2 FOR UPDATE`, membershipID, tenantID).Scan(&status, &userID, &profile)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrMembershipNotFound
	}
	if err != nil {
		return err
	}
	if status == "REVOKED" {
		return nil
	}
	if profile == auth.ProfileTenantOwner && status == "ACTIVE" {
		var otherActiveOwners int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM auth.ln_user_tenant_membership
			WHERE tenant_id = $1 AND permission_profile = 'tenant_owner' AND status = 'ACTIVE' AND id <> $2`,
			tenantID, membershipID).Scan(&otherActiveOwners); err != nil {
			return err
		}
		if otherActiveOwners == 0 {
			return auth.ErrProvisionConflict
		}
	}

	var invitationID string
	var invitationStatus string
	err = tx.QueryRow(ctx, `
		SELECT id::text, status FROM auth.tr_tenant_invitation
		WHERE membership_id = $1 FOR UPDATE`, membershipID).Scan(&invitationID, &invitationStatus)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if invitationID != "" && invitationStatus != "ACCEPTED" && invitationStatus != "REVOKED" {
		rows, err := tx.Query(ctx, `
			SELECT id FROM auth.sys_outbox_event
			WHERE aggregate_id = $1 AND event_type = $2 AND published_at IS NULL
			ORDER BY id FOR UPDATE`, invitationID, invitationDeliveryEvent)
		if err != nil {
			return err
		}
		for rows.Next() {
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if _, err := tx.Exec(ctx, `
			UPDATE auth.tr_tenant_invitation
			SET status = 'REVOKED', revoked_at = now(), token_hash = NULL
			WHERE id = $1`, invitationID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE auth.sys_outbox_event
			SET published_at = now(), payload = '{}'::jsonb
			WHERE aggregate_id = $1 AND event_type = $2 AND published_at IS NULL`,
			invitationID, invitationDeliveryEvent); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE auth.ln_user_tenant_membership
		SET status = 'REVOKED', revoked_at = now(), updated_at = now()
		WHERE id = $1 AND tenant_id = $2`, membershipID, tenantID); err != nil {
		return err
	}

	if err := insertAuthEvent(ctx, tx, "tenant_membership", membershipID, "auth.membership.revoked.v1", map[string]any{
		"tenant_id":     tenantID,
		"membership_id": membershipID,
		"user_id":       userID,
		"actor_id":      actorID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
