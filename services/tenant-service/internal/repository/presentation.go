package repository

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"strings"
	"time"

	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const domainSelect = `
	SELECT td.id, td.tenant_id, td.domain, ds.code, td.is_primary,
	       COALESCE(td.verification_token, ''), td.verified_at, td.created_at, td.updated_at
	FROM md_tenant_domain td
	JOIN di_domain_status ds ON ds.id = td.status_id`

func scanDomain(row pgx.Row) (domain.TenantDomain, error) {
	var result domain.TenantDomain
	err := row.Scan(
		&result.ID, &result.TenantID, &result.Domain, &result.Status, &result.Primary,
		&result.VerificationToken, &result.VerifiedAt, &result.CreatedAt, &result.UpdatedAt,
	)
	return result, err
}

func effectiveFeatureWith(ctx context.Context, q rowQuerier, tenantID uuid.UUID, code string) (bool, error) {
	var overrideValue, planValue *bool
	err := q.QueryRow(ctx, effectiveFeatureQuery()+` WHERE f.code = $2`, tenantID, code).Scan(new(string), &overrideValue, &planValue)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.ErrFeatureNotFound
	}
	if err != nil {
		return false, err
	}
	if overrideValue != nil {
		return *overrideValue, nil
	}
	return planValue != nil && *planValue, nil
}

func effectiveLimitWith(ctx context.Context, q rowQuerier, tenantID uuid.UUID, code string) (int64, error) {
	var value *int64
	err := q.QueryRow(ctx, `
		SELECT COALESCE(
			(SELECT limit_value FROM cfg_tenant_limit_override
			 WHERE tenant_id = $1 AND limit_type_id = lt.id
			   AND starts_at <= now() AND (ends_at IS NULL OR ends_at > now())
			 ORDER BY starts_at DESC LIMIT 1),
			(SELECT pl.limit_value
			 FROM hs_tenant_plan_assignment tpa
			 JOIN ln_plan_limit pl ON pl.plan_id = tpa.plan_id AND pl.limit_type_id = lt.id
			 WHERE tpa.tenant_id = $1 AND tpa.starts_at <= now()
			   AND (tpa.ends_at IS NULL OR tpa.ends_at > now())
			 ORDER BY tpa.starts_at DESC LIMIT 1)
		) FROM di_limit_type lt WHERE lt.code = $2`, tenantID, code).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrLimitTypeNotFound
	}
	if err != nil {
		return 0, err
	}
	if value == nil {
		return 0, domain.ErrEffectiveLimitNotFound
	}
	return *value, nil
}

func (s *PostgresStore) AddDomain(ctx context.Context, tenantID uuid.UUID, name string) (domain.TenantDomain, error) {
	result, err := withTx(ctx, s.pool, func(tx pgx.Tx) (domain.TenantDomain, error) {
		if err := lockMutableTenant(ctx, tx, tenantID); err != nil {
			return domain.TenantDomain{}, err
		}
		enabled, err := effectiveFeatureWith(ctx, tx, tenantID, "CUSTOM_DOMAIN")
		if err != nil {
			return domain.TenantDomain{}, err
		}
		if !enabled {
			return domain.TenantDomain{}, domain.ErrFeatureDisabled
		}
		limit, err := effectiveLimitWith(ctx, tx, tenantID, "MAX_DOMAINS")
		if err != nil {
			return domain.TenantDomain{}, err
		}
		var count int64
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM md_tenant_domain td
			JOIN di_domain_status ds ON ds.id = td.status_id
			WHERE td.tenant_id = $1 AND ds.code != 'DISABLED'`, tenantID).Scan(&count); err != nil {
			return domain.TenantDomain{}, err
		}
		if count >= limit {
			return domain.TenantDomain{}, domain.ErrDomainLimitExceeded
		}

		domainID, token := uuid.New(), uuid.NewString()
		_, err = tx.Exec(ctx, `
			INSERT INTO md_tenant_domain (id, tenant_id, domain, status_id, verification_token)
			VALUES ($1, $2, $3, (SELECT id FROM di_domain_status WHERE code = 'PENDING'), $4)`,
			domainID, tenantID, name, token)
		if err != nil {
			return domain.TenantDomain{}, translateError(err)
		}
		if err := insertOutbox(ctx, tx, tenantID, "tenant.domain.created", map[string]any{"domain_id": domainID, "domain": name}); err != nil {
			return domain.TenantDomain{}, err
		}
		return scanDomain(tx.QueryRow(ctx, domainSelect+` WHERE td.id = $1`, domainID))
	})
	return result, translateError(err)
}

func (s *PostgresStore) GetDomains(ctx context.Context, tenantID uuid.UUID) ([]domain.TenantDomain, error) {
	if _, err := s.GetTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, domainSelect+` WHERE td.tenant_id = $1 ORDER BY td.created_at, td.id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.TenantDomain, 0)
	for rows.Next() {
		item, err := scanDomain(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) VerifyDomain(ctx context.Context, tenantID, domainID uuid.UUID, token string) (domain.TenantDomain, error) {
	var name, challenge string
	if err := s.pool.QueryRow(ctx, `
		SELECT domain, COALESCE(verification_token, '') FROM md_tenant_domain
		WHERE id = $1 AND tenant_id = $2`, domainID, tenantID).Scan(&name, &challenge); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.TenantDomain{}, domain.ErrDomainNotFound
		}
		return domain.TenantDomain{}, err
	}
	proved := false
	if token != "" && challenge != "" && subtle.ConstantTimeCompare([]byte(token), []byte(challenge)) == 1 {
		lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		records, err := s.lookupTXT(lookupCtx, "_reservehub-verification."+name)
		if err != nil {
			var dnsErr *net.DNSError
			if !errors.As(err, &dnsErr) || !dnsErr.IsNotFound {
				return domain.TenantDomain{}, domain.ErrDomainProofUnavailable
			}
		}
		for _, record := range records {
			if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(record)), []byte(challenge)) == 1 {
				proved = true
				break
			}
		}
	}
	return withTx(ctx, s.pool, func(tx pgx.Tx) (domain.TenantDomain, error) {
		if err := lockMutableTenant(ctx, tx, tenantID); err != nil {
			return domain.TenantDomain{}, err
		}
		var storedToken, status string
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(td.verification_token, ''), ds.code FROM md_tenant_domain td
			JOIN di_domain_status ds ON ds.id = td.status_id
			WHERE td.id = $1 AND td.tenant_id = $2 FOR UPDATE OF td`, domainID, tenantID).Scan(&storedToken, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.TenantDomain{}, domain.ErrDomainNotFound
			}
			return domain.TenantDomain{}, err
		}
		if storedToken != challenge || status == "DISABLED" {
			return domain.TenantDomain{}, domain.ErrDomainNotVerified
		}
		if status == "VERIFIED" {
			if proved {
				return scanDomain(tx.QueryRow(ctx, domainSelect+` WHERE td.id = $1`, domainID))
			}
		}
		newStatus, event := "FAILED", "tenant.domain.verification_failed"
		if proved {
			newStatus, event = "VERIFIED", "tenant.domain.verified"
		}
		_, err := tx.Exec(ctx, `
			UPDATE md_tenant_domain
			SET status_id = (SELECT id FROM di_domain_status WHERE code = $3),
			    verified_at = CASE WHEN $3 = 'VERIFIED' THEN now() ELSE NULL END,
			    is_primary = CASE WHEN $3 = 'VERIFIED' THEN is_primary ELSE false END,
			    updated_at = now()
			WHERE id = $1 AND tenant_id = $2`, domainID, tenantID, newStatus)
		if err != nil {
			return domain.TenantDomain{}, err
		}
		if err := insertOutbox(ctx, tx, tenantID, event, map[string]any{"domain_id": domainID, "status": newStatus}); err != nil {
			return domain.TenantDomain{}, err
		}
		return scanDomain(tx.QueryRow(ctx, domainSelect+` WHERE td.id = $1`, domainID))
	})
}

func (s *PostgresStore) SetPrimaryDomain(ctx context.Context, tenantID, domainID uuid.UUID) (domain.TenantDomain, error) {
	return withTx(ctx, s.pool, func(tx pgx.Tx) (domain.TenantDomain, error) {
		if err := lockMutableTenant(ctx, tx, tenantID); err != nil {
			return domain.TenantDomain{}, err
		}
		var status string
		if err := tx.QueryRow(ctx, `
			SELECT ds.code FROM md_tenant_domain td JOIN di_domain_status ds ON ds.id = td.status_id
			WHERE td.id = $1 AND td.tenant_id = $2 FOR UPDATE OF td`, domainID, tenantID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.TenantDomain{}, domain.ErrDomainNotFound
			}
			return domain.TenantDomain{}, err
		}
		if status != "VERIFIED" {
			return domain.TenantDomain{}, domain.ErrDomainNotVerified
		}
		if _, err := tx.Exec(ctx, `UPDATE md_tenant_domain SET is_primary = false, updated_at = now() WHERE tenant_id = $1 AND is_primary`, tenantID); err != nil {
			return domain.TenantDomain{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE md_tenant_domain SET is_primary = true, updated_at = now() WHERE id = $1`, domainID); err != nil {
			return domain.TenantDomain{}, err
		}
		if err := insertOutbox(ctx, tx, tenantID, "tenant.domain.primary_changed", map[string]any{"domain_id": domainID}); err != nil {
			return domain.TenantDomain{}, err
		}
		return scanDomain(tx.QueryRow(ctx, domainSelect+` WHERE td.id = $1`, domainID))
	})
}

func (s *PostgresStore) DisableDomain(ctx context.Context, tenantID, domainID uuid.UUID) (domain.TenantDomain, error) {
	return withTx(ctx, s.pool, func(tx pgx.Tx) (domain.TenantDomain, error) {
		if err := lockMutableTenant(ctx, tx, tenantID); err != nil {
			return domain.TenantDomain{}, err
		}
		result, err := tx.Exec(ctx, `
			UPDATE md_tenant_domain
			SET status_id = (SELECT id FROM di_domain_status WHERE code = 'DISABLED'),
			    is_primary = false, updated_at = now()
			WHERE id = $1 AND tenant_id = $2`, domainID, tenantID)
		if err != nil {
			return domain.TenantDomain{}, err
		}
		if result.RowsAffected() == 0 {
			return domain.TenantDomain{}, domain.ErrDomainNotFound
		}
		if err := insertOutbox(ctx, tx, tenantID, "tenant.domain.disabled", map[string]any{"domain_id": domainID}); err != nil {
			return domain.TenantDomain{}, err
		}
		return scanDomain(tx.QueryRow(ctx, domainSelect+` WHERE td.id = $1`, domainID))
	})
}

const brandingSelect = `
	SELECT tb.tenant_id, bs.code, COALESCE(tb.display_name, ''), COALESCE(tb.logo_url, ''),
	       COALESCE(tb.favicon_url, ''), COALESCE(tb.primary_color, ''),
	       COALESCE(tb.secondary_color, ''), COALESCE(tb.accent_color, ''),
	       tb.created_at, tb.updated_at
	FROM cfg_tenant_branding tb JOIN di_branding_status bs ON bs.id = tb.status_id`

func scanBranding(row pgx.Row) (domain.Branding, error) {
	var result domain.Branding
	err := row.Scan(
		&result.TenantID, &result.Status, &result.DisplayName, &result.LogoURL,
		&result.FaviconURL, &result.PrimaryColor, &result.SecondaryColor,
		&result.AccentColor, &result.CreatedAt, &result.UpdatedAt,
	)
	return result, err
}

func (s *PostgresStore) GetBranding(ctx context.Context, tenantID uuid.UUID) (domain.Branding, error) {
	result, err := scanBranding(s.pool.QueryRow(ctx, brandingSelect+` WHERE tb.tenant_id = $1`, tenantID))
	return result, translateError(err)
}

func (s *PostgresStore) UpdateBranding(ctx context.Context, branding domain.Branding) (domain.Branding, error) {
	return withTx(ctx, s.pool, func(tx pgx.Tx) (domain.Branding, error) {
		if err := lockMutableTenant(ctx, tx, branding.TenantID); err != nil {
			return domain.Branding{}, err
		}
		enabled, err := effectiveFeatureWith(ctx, tx, branding.TenantID, "CUSTOM_BRANDING")
		if err != nil {
			return domain.Branding{}, err
		}
		if !enabled {
			return domain.Branding{}, domain.ErrBrandingNotAllowed
		}
		result, err := tx.Exec(ctx, `
			UPDATE cfg_tenant_branding
			SET status_id = (SELECT id FROM di_branding_status WHERE code = 'ACTIVE'),
			    display_name = NULLIF($2, ''), logo_url = NULLIF($3, ''), favicon_url = NULLIF($4, ''),
			    primary_color = NULLIF($5, ''), secondary_color = NULLIF($6, ''),
			    accent_color = NULLIF($7, ''), updated_at = now()
			WHERE tenant_id = $1`, branding.TenantID, branding.DisplayName, branding.LogoURL,
			branding.FaviconURL, branding.PrimaryColor, branding.SecondaryColor, branding.AccentColor)
		if err != nil {
			return domain.Branding{}, err
		}
		if result.RowsAffected() == 0 {
			return domain.Branding{}, domain.ErrTenantNotFound
		}
		if err := insertOutbox(ctx, tx, branding.TenantID, "tenant.branding.changed", map[string]any{}); err != nil {
			return domain.Branding{}, err
		}
		return scanBranding(tx.QueryRow(ctx, brandingSelect+` WHERE tb.tenant_id = $1`, branding.TenantID))
	})
}
