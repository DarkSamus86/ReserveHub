package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
)

const tenantSelect = `
	SELECT t.id, t.name, t.slug, ts.code, t.created_at, t.updated_at,
	       t.activated_at, t.suspended_at, t.archived_at
	FROM md_tenant t
	JOIN di_tenant_status ts ON ts.id = t.status_id`

func scanTenant(row pgx.Row) (domain.Tenant, error) {
	var tenant domain.Tenant
	err := row.Scan(
		&tenant.ID, &tenant.Name, &tenant.Slug, &tenant.Status,
		&tenant.CreatedAt, &tenant.UpdatedAt, &tenant.ActivatedAt,
		&tenant.SuspendedAt, &tenant.ArchivedAt,
	)
	return tenant, err
}

func getTenant(ctx context.Context, q rowQuerier, id uuid.UUID) (domain.Tenant, error) {
	tenant, err := scanTenant(q.QueryRow(ctx, tenantSelect+` WHERE t.id = $1`, id))
	return tenant, translateError(err)
}

func (s *PostgresStore) CreateTenant(ctx context.Context, in domain.CreateTenantInput) (domain.Tenant, error) {
	tenant, err := withTx(ctx, s.pool, func(tx pgx.Tx) (domain.Tenant, error) {
		var planActive bool
		if err := tx.QueryRow(ctx, `SELECT is_active FROM md_subscription_plan WHERE id = $1`, in.PlanID).Scan(&planActive); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.Tenant{}, domain.ErrPlanNotFound
			}
			return domain.Tenant{}, err
		}
		if !planActive {
			return domain.Tenant{}, domain.ErrPlanInactive
		}

		tenantID := uuid.New()
		_, err := tx.Exec(ctx, `
			INSERT INTO md_tenant (id, name, slug, status_id)
			VALUES ($1, $2, $3, (SELECT id FROM di_tenant_status WHERE code = 'PENDING'))`,
			tenantID, in.Name, in.Slug)
		if err != nil {
			return domain.Tenant{}, translateError(err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO cfg_tenant_settings (tenant_id, timezone, currency_code, locale_code, default_language)
			VALUES ($1, $2, $3, $4, $5)`,
			tenantID, in.Timezone, in.CurrencyCode, in.LocaleCode, in.DefaultLanguage)
		if err != nil {
			return domain.Tenant{}, err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO cfg_tenant_branding (tenant_id, status_id, display_name)
			VALUES ($1, (SELECT id FROM di_branding_status WHERE code = 'DEFAULT'), $2)`, tenantID, in.Name)
		if err != nil {
			return domain.Tenant{}, err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO hs_tenant_plan_assignment (id, tenant_id, plan_id, starts_at)
			VALUES ($1, $2, $3, now())`, uuid.New(), tenantID, in.PlanID)
		if err != nil {
			return domain.Tenant{}, err
		}
		if err := insertOutbox(ctx, tx, tenantID, "tenant.created", map[string]any{
			"name": in.Name, "slug": in.Slug, "status": domain.StatusPending,
		}); err != nil {
			return domain.Tenant{}, err
		}
		return getTenant(ctx, tx, tenantID)
	})
	return tenant, translateError(err)
}

func (s *PostgresStore) GetTenant(ctx context.Context, id uuid.UUID) (domain.Tenant, error) {
	return getTenant(ctx, s.pool, id)
}

func (s *PostgresStore) ListTenants(ctx context.Context, limit, offset int, status string) ([]domain.Tenant, bool, error) {
	rows, err := s.pool.Query(ctx, tenantSelect+`
		WHERE ($3 = '' OR ts.code = $3)
		ORDER BY t.created_at DESC, t.id
		LIMIT $1 OFFSET $2`, limit+1, offset, status)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	tenants := make([]domain.Tenant, 0, limit+1)
	for rows.Next() {
		tenant, err := scanTenant(rows)
		if err != nil {
			return nil, false, err
		}
		tenants = append(tenants, tenant)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(tenants) > limit
	if hasMore {
		tenants = tenants[:limit]
	}
	return tenants, hasMore, nil
}

func (s *PostgresStore) UpdateTenant(ctx context.Context, in domain.UpdateTenantInput) (domain.Tenant, error) {
	tenant, err := withTx(ctx, s.pool, func(tx pgx.Tx) (domain.Tenant, error) {
		if err := lockMutableTenant(ctx, tx, in.TenantID); err != nil {
			return domain.Tenant{}, err
		}
		result, err := tx.Exec(ctx, `
			UPDATE md_tenant
			SET name = COALESCE($2, name), slug = COALESCE($3, slug), updated_at = now()
			WHERE id = $1`,
			in.TenantID, in.Name, in.Slug)
		if err != nil {
			return domain.Tenant{}, translateError(err)
		}
		if result.RowsAffected() == 0 {
			return domain.Tenant{}, domain.ErrTenantNotFound
		}
		if err := insertOutbox(ctx, tx, in.TenantID, "tenant.updated", map[string]any{}); err != nil {
			return domain.Tenant{}, err
		}
		return getTenant(ctx, tx, in.TenantID)
	})
	return tenant, translateError(err)
}

func (s *PostgresStore) ChangeTenantState(ctx context.Context, id uuid.UUID, target string) (domain.Tenant, error) {
	return withTx(ctx, s.pool, func(tx pgx.Tx) (domain.Tenant, error) {
		current, err := lockTenantStatus(ctx, tx, id)
		if err != nil {
			return domain.Tenant{}, err
		}
		if err := domain.ValidateTransition(current, target); err != nil {
			return domain.Tenant{}, err
		}

		if current == domain.StatusPending && target == domain.StatusActive {
			var ready bool
			err := tx.QueryRow(ctx, `
				SELECT EXISTS (SELECT 1 FROM cfg_tenant_settings WHERE tenant_id = $1)
				   AND EXISTS (
				       SELECT 1 FROM hs_tenant_plan_assignment tpa
				       JOIN md_subscription_plan sp ON sp.id = tpa.plan_id
				       WHERE tpa.tenant_id = $1 AND sp.is_active
				         AND tpa.starts_at <= now() AND (tpa.ends_at IS NULL OR tpa.ends_at > now())
				   )`, id).Scan(&ready)
			if err != nil {
				return domain.Tenant{}, err
			}
			if !ready {
				return domain.Tenant{}, fmt.Errorf("%w: tenant settings and an active current plan are required", domain.ErrInvalidStateTransition)
			}
		}

		_, err = tx.Exec(ctx, `
			UPDATE md_tenant
			SET status_id = (SELECT id FROM di_tenant_status WHERE code = $2),
			    updated_at = now(),
			    activated_at = CASE WHEN $2 = 'ACTIVE' AND $3 = 'PENDING' THEN now() ELSE activated_at END,
			    suspended_at = CASE WHEN $2 = 'SUSPENDED' THEN now() ELSE suspended_at END,
			    archived_at = CASE WHEN $2 = 'ARCHIVED' THEN now() ELSE archived_at END
			WHERE id = $1`, id, target, current)
		if err != nil {
			return domain.Tenant{}, err
		}
		event := map[string]string{
			domain.StatusActive:    "tenant.activated",
			domain.StatusSuspended: "tenant.suspended",
			domain.StatusArchived:  "tenant.archived",
		}[target]
		if current == domain.StatusSuspended && target == domain.StatusActive {
			event = "tenant.resumed"
		}
		if err := insertOutbox(ctx, tx, id, event, map[string]any{"status": target}); err != nil {
			return domain.Tenant{}, err
		}
		return getTenant(ctx, tx, id)
	})
}

func (s *PostgresStore) GetSettings(ctx context.Context, id uuid.UUID) (domain.Settings, error) {
	var settings domain.Settings
	err := s.pool.QueryRow(ctx, `
		SELECT tenant_id, timezone, currency_code, locale_code, default_language, created_at, updated_at
		FROM cfg_tenant_settings WHERE tenant_id = $1`, id).Scan(
		&settings.TenantID, &settings.Timezone, &settings.CurrencyCode, &settings.LocaleCode,
		&settings.DefaultLanguage, &settings.CreatedAt, &settings.UpdatedAt,
	)
	return settings, translateError(err)
}

func (s *PostgresStore) UpdateSettings(ctx context.Context, in domain.UpdateSettingsInput) (domain.Settings, error) {
	return withTx(ctx, s.pool, func(tx pgx.Tx) (domain.Settings, error) {
		if err := lockMutableTenant(ctx, tx, in.TenantID); err != nil {
			return domain.Settings{}, err
		}
		var settings domain.Settings
		err := tx.QueryRow(ctx, `
			UPDATE cfg_tenant_settings
			SET timezone = $2, currency_code = $3, locale_code = $4,
			    default_language = $5, updated_at = now()
			WHERE tenant_id = $1
			RETURNING tenant_id, timezone, currency_code, locale_code, default_language, created_at, updated_at`,
			in.TenantID, in.Timezone, in.CurrencyCode, in.LocaleCode, in.DefaultLanguage).Scan(
			&settings.TenantID, &settings.Timezone, &settings.CurrencyCode, &settings.LocaleCode,
			&settings.DefaultLanguage, &settings.CreatedAt, &settings.UpdatedAt,
		)
		if err != nil {
			return domain.Settings{}, translateError(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE md_tenant SET updated_at = now() WHERE id = $1`, in.TenantID); err != nil {
			return domain.Settings{}, err
		}
		if err := insertOutbox(ctx, tx, in.TenantID, "tenant.settings.changed", map[string]any{
			"timezone": in.Timezone, "currency_code": in.CurrencyCode,
			"locale_code": in.LocaleCode, "default_language": in.DefaultLanguage,
		}); err != nil {
			return domain.Settings{}, err
		}
		return settings, nil
	})
}

var _ = time.Time{}
