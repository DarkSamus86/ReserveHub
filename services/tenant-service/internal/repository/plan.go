package repository

import (
	"context"
	"errors"
	"time"

	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
)

const planSelect = `
	SELECT tpa.id, tpa.tenant_id, sp.id, sp.code, sp.name,
	       COALESCE(sp.description, ''), sp.is_active,
	       tpa.starts_at, tpa.ends_at, tpa.created_at
	FROM hs_tenant_plan_assignment tpa
	JOIN md_subscription_plan sp ON sp.id = tpa.plan_id`

func scanPlan(row pgx.Row) (domain.TenantPlan, error) {
	var plan domain.TenantPlan
	err := row.Scan(
		&plan.ID, &plan.TenantID, &plan.Plan.ID, &plan.Plan.Code, &plan.Plan.Name,
		&plan.Plan.Description, &plan.Plan.Active, &plan.StartsAt, &plan.EndsAt, &plan.CreatedAt,
	)
	return plan, err
}

func (s *PostgresStore) GetCurrentPlan(ctx context.Context, tenantID uuid.UUID) (domain.TenantPlan, error) {
	plan, err := scanPlan(s.pool.QueryRow(ctx, planSelect+`
		WHERE tpa.tenant_id = $1
		  AND tpa.starts_at <= now()
		  AND (tpa.ends_at IS NULL OR tpa.ends_at > now())
		ORDER BY tpa.starts_at DESC LIMIT 1`, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TenantPlan{}, domain.ErrTenantPlanNotFound
	}
	return plan, err
}

func (s *PostgresStore) GetPlanHistory(ctx context.Context, tenantID uuid.UUID) ([]domain.TenantPlan, error) {
	if _, err := s.GetTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, planSelect+`
		WHERE tpa.tenant_id = $1 ORDER BY tpa.starts_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := make([]domain.TenantPlan, 0)
	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, rows.Err()
}

func (s *PostgresStore) ChangePlan(ctx context.Context, tenantID, planID uuid.UUID, effectiveFrom time.Time) (domain.TenantPlan, error) {
	return withTx(ctx, s.pool, func(tx pgx.Tx) (domain.TenantPlan, error) {
		if err := lockMutableTenant(ctx, tx, tenantID); err != nil {
			return domain.TenantPlan{}, err
		}
		var transactionTime time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&transactionTime); err != nil {
			return domain.TenantPlan{}, err
		}
		if effectiveFrom.IsZero() {
			effectiveFrom = transactionTime
		} else if effectiveFrom.Before(transactionTime) {
			return domain.TenantPlan{}, domain.ErrInvalidArgument
		}
		var active bool
		if err := tx.QueryRow(ctx, `SELECT is_active FROM md_subscription_plan WHERE id = $1`, planID).Scan(&active); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.TenantPlan{}, domain.ErrPlanNotFound
			}
			return domain.TenantPlan{}, err
		}
		if !active {
			return domain.TenantPlan{}, domain.ErrPlanInactive
		}

		var currentID, oldPlanID uuid.UUID
		var currentStarts time.Time
		var currentEnds *time.Time
		err := tx.QueryRow(ctx, `
			SELECT id, plan_id, starts_at, ends_at
			FROM hs_tenant_plan_assignment
			WHERE tenant_id = $1
			ORDER BY starts_at DESC LIMIT 1 FOR UPDATE`, tenantID).Scan(&currentID, &oldPlanID, &currentStarts, &currentEnds)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.TenantPlan{}, domain.ErrTenantPlanNotFound
		}
		if err != nil {
			return domain.TenantPlan{}, err
		}
		if currentEnds != nil || !effectiveFrom.After(currentStarts) {
			return domain.TenantPlan{}, domain.ErrInvalidStateTransition
		}
		if _, err := tx.Exec(ctx, `UPDATE hs_tenant_plan_assignment SET ends_at = $2 WHERE id = $1`, currentID, effectiveFrom); err != nil {
			return domain.TenantPlan{}, err
		}
		assignmentID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO hs_tenant_plan_assignment (id, tenant_id, plan_id, starts_at)
			VALUES ($1, $2, $3, $4)`, assignmentID, tenantID, planID, effectiveFrom); err != nil {
			return domain.TenantPlan{}, err
		}
		if err := insertOutbox(ctx, tx, tenantID, "tenant.plan.changed", map[string]any{
			"old_plan_id": oldPlanID, "new_plan_id": planID, "effective_from": effectiveFrom,
		}); err != nil {
			return domain.TenantPlan{}, err
		}
		return scanPlan(tx.QueryRow(ctx, planSelect+` WHERE tpa.id = $1`, assignmentID))
	})
}
