package repository

import (
	"context"
	"errors"

	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) GetEffectiveLimit(ctx context.Context, tenantID uuid.UUID, code string) (domain.EffectiveLimit, error) {
	if _, err := s.GetTenant(ctx, tenantID); err != nil {
		return domain.EffectiveLimit{}, err
	}
	var result domain.EffectiveLimit
	var overrideValue, planValue *int64
	err := s.pool.QueryRow(ctx, `
		SELECT lt.code, ov.limit_value, pl.limit_value
		FROM di_limit_type lt
		LEFT JOIN LATERAL (
			SELECT limit_value FROM cfg_tenant_limit_override
			WHERE tenant_id = $1 AND limit_type_id = lt.id
			  AND starts_at <= now() AND (ends_at IS NULL OR ends_at > now())
			ORDER BY starts_at DESC LIMIT 1
		) ov ON true
		LEFT JOIN LATERAL (
			SELECT limits.limit_value
			FROM hs_tenant_plan_assignment tpa
			JOIN ln_plan_limit limits ON limits.plan_id = tpa.plan_id AND limits.limit_type_id = lt.id
			WHERE tpa.tenant_id = $1 AND tpa.starts_at <= now()
			  AND (tpa.ends_at IS NULL OR tpa.ends_at > now())
			ORDER BY tpa.starts_at DESC LIMIT 1
		) pl ON true
		WHERE lt.code = $2`, tenantID, code).Scan(&result.Code, &overrideValue, &planValue)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EffectiveLimit{}, domain.ErrLimitTypeNotFound
	}
	if err != nil {
		return domain.EffectiveLimit{}, err
	}
	result.TenantID = tenantID
	if overrideValue != nil {
		result.Value, result.Overridden = *overrideValue, true
		return result, nil
	}
	if planValue == nil {
		return domain.EffectiveLimit{}, domain.ErrEffectiveLimitNotFound
	}
	result.Value = *planValue
	return result, nil
}

func (s *PostgresStore) SetLimitOverride(ctx context.Context, tenantID uuid.UUID, code string, value int64, period domain.Period) (domain.LimitOverride, error) {
	return withTx(ctx, s.pool, func(tx pgx.Tx) (domain.LimitOverride, error) {
		if err := lockMutableTenant(ctx, tx, tenantID); err != nil {
			return domain.LimitOverride{}, err
		}
		var typeID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM di_limit_type WHERE code = $1`, code).Scan(&typeID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.LimitOverride{}, domain.ErrLimitTypeNotFound
			}
			return domain.LimitOverride{}, err
		}
		result := domain.LimitOverride{ID: uuid.New(), TenantID: tenantID, Code: code, Value: value, StartsAt: period.StartsAt, EndsAt: period.EndsAt}
		err := tx.QueryRow(ctx, `
			INSERT INTO cfg_tenant_limit_override (id, tenant_id, limit_type_id, limit_value, starts_at, ends_at)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at`,
			result.ID, tenantID, typeID, value, period.StartsAt, period.EndsAt).Scan(&result.CreatedAt)
		if err != nil {
			return domain.LimitOverride{}, err
		}
		if err := insertOutbox(ctx, tx, tenantID, "tenant.limit.changed", map[string]any{"code": code, "value": value}); err != nil {
			return domain.LimitOverride{}, err
		}
		return result, nil
	})
}

func (s *PostgresStore) RemoveLimitOverride(ctx context.Context, tenantID, overrideID uuid.UUID) error {
	_, err := withTx(ctx, s.pool, func(tx pgx.Tx) (struct{}, error) {
		if err := lockMutableTenant(ctx, tx, tenantID); err != nil {
			return struct{}{}, err
		}
		result, err := tx.Exec(ctx, `DELETE FROM cfg_tenant_limit_override WHERE id = $1 AND tenant_id = $2`, overrideID, tenantID)
		if err != nil {
			return struct{}{}, err
		}
		if result.RowsAffected() == 0 {
			return struct{}{}, domain.ErrLimitTypeNotFound
		}
		return struct{}{}, insertOutbox(ctx, tx, tenantID, "tenant.limit.changed", map[string]any{"override_id": overrideID, "removed": true})
	})
	return err
}

func effectiveFeatureQuery() string {
	return `
		SELECT f.code, ov.enabled, pf.enabled
		FROM di_feature f
		LEFT JOIN LATERAL (
			SELECT enabled FROM cfg_tenant_feature_override
			WHERE tenant_id = $1 AND feature_id = f.id
			  AND starts_at <= now() AND (ends_at IS NULL OR ends_at > now())
			ORDER BY starts_at DESC LIMIT 1
		) ov ON true
		LEFT JOIN LATERAL (
			SELECT features.enabled
			FROM hs_tenant_plan_assignment tpa
			JOIN ln_plan_feature features ON features.plan_id = tpa.plan_id AND features.feature_id = f.id
			WHERE tpa.tenant_id = $1 AND tpa.starts_at <= now()
			  AND (tpa.ends_at IS NULL OR tpa.ends_at > now())
			ORDER BY tpa.starts_at DESC LIMIT 1
		) pf ON true`
}

func (s *PostgresStore) HasFeature(ctx context.Context, tenantID uuid.UUID, code string) (domain.EffectiveFeature, error) {
	if _, err := s.GetTenant(ctx, tenantID); err != nil {
		return domain.EffectiveFeature{}, err
	}
	var result domain.EffectiveFeature
	var overrideValue, planValue *bool
	err := s.pool.QueryRow(ctx, effectiveFeatureQuery()+` WHERE f.code = $2`, tenantID, code).Scan(&result.Code, &overrideValue, &planValue)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EffectiveFeature{}, domain.ErrFeatureNotFound
	}
	if err != nil {
		return domain.EffectiveFeature{}, err
	}
	result.TenantID = tenantID
	if overrideValue != nil {
		result.Enabled, result.Overridden = *overrideValue, true
	} else if planValue != nil {
		result.Enabled = *planValue
	}
	return result, nil
}

func (s *PostgresStore) GetEffectiveFeatures(ctx context.Context, tenantID uuid.UUID) ([]domain.EffectiveFeature, error) {
	if _, err := s.GetTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, effectiveFeatureQuery()+` ORDER BY f.code`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	features := make([]domain.EffectiveFeature, 0)
	for rows.Next() {
		var feature domain.EffectiveFeature
		var overrideValue, planValue *bool
		if err := rows.Scan(&feature.Code, &overrideValue, &planValue); err != nil {
			return nil, err
		}
		feature.TenantID = tenantID
		if overrideValue != nil {
			feature.Enabled, feature.Overridden = *overrideValue, true
		} else if planValue != nil {
			feature.Enabled = *planValue
		}
		features = append(features, feature)
	}
	return features, rows.Err()
}

func (s *PostgresStore) SetFeatureOverride(ctx context.Context, tenantID uuid.UUID, code string, enabled bool, period domain.Period) (domain.FeatureOverride, error) {
	return withTx(ctx, s.pool, func(tx pgx.Tx) (domain.FeatureOverride, error) {
		if err := lockMutableTenant(ctx, tx, tenantID); err != nil {
			return domain.FeatureOverride{}, err
		}
		var featureID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM di_feature WHERE code = $1`, code).Scan(&featureID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.FeatureOverride{}, domain.ErrFeatureNotFound
			}
			return domain.FeatureOverride{}, err
		}
		result := domain.FeatureOverride{ID: uuid.New(), TenantID: tenantID, Code: code, Enabled: enabled, StartsAt: period.StartsAt, EndsAt: period.EndsAt}
		err := tx.QueryRow(ctx, `
			INSERT INTO cfg_tenant_feature_override (id, tenant_id, feature_id, enabled, starts_at, ends_at)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at`,
			result.ID, tenantID, featureID, enabled, period.StartsAt, period.EndsAt).Scan(&result.CreatedAt)
		if err != nil {
			return domain.FeatureOverride{}, err
		}
		if err := insertOutbox(ctx, tx, tenantID, "tenant.feature.changed", map[string]any{"code": code, "enabled": enabled}); err != nil {
			return domain.FeatureOverride{}, err
		}
		return result, nil
	})
}

func (s *PostgresStore) RemoveFeatureOverride(ctx context.Context, tenantID, overrideID uuid.UUID) error {
	_, err := withTx(ctx, s.pool, func(tx pgx.Tx) (struct{}, error) {
		if err := lockMutableTenant(ctx, tx, tenantID); err != nil {
			return struct{}{}, err
		}
		result, err := tx.Exec(ctx, `DELETE FROM cfg_tenant_feature_override WHERE id = $1 AND tenant_id = $2`, overrideID, tenantID)
		if err != nil {
			return struct{}{}, err
		}
		if result.RowsAffected() == 0 {
			return struct{}{}, domain.ErrFeatureNotFound
		}
		return struct{}{}, insertOutbox(ctx, tx, tenantID, "tenant.feature.changed", map[string]any{"override_id": overrideID, "removed": true})
	})
	return err
}
