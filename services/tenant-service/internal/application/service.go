package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"darksamus.com/ReserveHub/tenant-service/internal/repository"
	"github.com/google/uuid"
)

type Service struct {
	store repository.Store
	now   func() time.Time
}

func New(store repository.Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) CreateTenant(ctx context.Context, in domain.CreateTenantInput) (domain.Tenant, error) {
	if err := domain.ValidateCreateTenant(&in); err != nil {
		return domain.Tenant{}, err
	}
	return s.store.CreateTenant(ctx, in)
}

func (s *Service) GetTenant(ctx context.Context, id uuid.UUID) (domain.Tenant, error) {
	return s.store.GetTenant(ctx, id)
}

func (s *Service) ListTenants(ctx context.Context, pageSize, offset int, status string) ([]domain.Tenant, bool, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	if offset < 0 {
		return nil, false, fmt.Errorf("%w: invalid page token", domain.ErrInvalidArgument)
	}
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != "" && status != domain.StatusPending && status != domain.StatusActive && status != domain.StatusSuspended && status != domain.StatusArchived {
		return nil, false, fmt.Errorf("%w: invalid status", domain.ErrInvalidArgument)
	}
	return s.store.ListTenants(ctx, pageSize, offset, status)
}

func (s *Service) UpdateTenant(ctx context.Context, in domain.UpdateTenantInput) (domain.Tenant, error) {
	if err := s.ensureNotArchived(ctx, in.TenantID); err != nil {
		return domain.Tenant{}, err
	}
	if in.Name == nil && in.Slug == nil {
		return domain.Tenant{}, fmt.Errorf("%w: at least one field is required", domain.ErrInvalidArgument)
	}
	if in.Name != nil {
		value := strings.TrimSpace(*in.Name)
		if value == "" || len(value) > 255 {
			return domain.Tenant{}, fmt.Errorf("%w: invalid name", domain.ErrInvalidArgument)
		}
		in.Name = &value
	}
	if in.Slug != nil {
		value, err := domain.NormalizeSlug(*in.Slug)
		if err != nil {
			return domain.Tenant{}, err
		}
		in.Slug = &value
	}
	return s.store.UpdateTenant(ctx, in)
}

func (s *Service) ChangeState(ctx context.Context, id uuid.UUID, target string) (domain.Tenant, error) {
	return s.store.ChangeTenantState(ctx, id, target)
}

func (s *Service) GetSettings(ctx context.Context, id uuid.UUID) (domain.Settings, error) {
	return s.store.GetSettings(ctx, id)
}

func (s *Service) UpdateSettings(ctx context.Context, in domain.UpdateSettingsInput) (domain.Settings, error) {
	if err := s.ensureNotArchived(ctx, in.TenantID); err != nil {
		return domain.Settings{}, err
	}
	if err := domain.ValidateSettings(in.Timezone, in.CurrencyCode, in.LocaleCode, in.DefaultLanguage); err != nil {
		return domain.Settings{}, err
	}
	return s.store.UpdateSettings(ctx, in)
}

func (s *Service) GetCurrentPlan(ctx context.Context, id uuid.UUID) (domain.TenantPlan, error) {
	return s.store.GetCurrentPlan(ctx, id)
}

func (s *Service) GetPlanHistory(ctx context.Context, id uuid.UUID) ([]domain.TenantPlan, error) {
	return s.store.GetPlanHistory(ctx, id)
}

func (s *Service) ChangePlan(ctx context.Context, tenantID, planID uuid.UUID, effectiveFrom time.Time) (domain.TenantPlan, error) {
	if err := s.ensureNotArchived(ctx, tenantID); err != nil {
		return domain.TenantPlan{}, err
	}
	if effectiveFrom.IsZero() {
		effectiveFrom = s.now().UTC()
	}
	return s.store.ChangePlan(ctx, tenantID, planID, effectiveFrom)
}

func (s *Service) GetEffectiveLimit(ctx context.Context, tenantID uuid.UUID, code string) (domain.EffectiveLimit, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return domain.EffectiveLimit{}, fmt.Errorf("%w: limit_code is required", domain.ErrInvalidArgument)
	}
	return s.store.GetEffectiveLimit(ctx, tenantID, code)
}

func (s *Service) SetLimitOverride(ctx context.Context, tenantID uuid.UUID, code string, value int64, period domain.Period) (domain.LimitOverride, error) {
	if err := s.ensureNotArchived(ctx, tenantID); err != nil {
		return domain.LimitOverride{}, err
	}
	if value < 0 {
		return domain.LimitOverride{}, fmt.Errorf("%w: value must be non-negative", domain.ErrInvalidArgument)
	}
	if period.StartsAt.IsZero() {
		period.StartsAt = s.now().UTC()
	}
	if err := domain.ValidatePeriod(period); err != nil {
		return domain.LimitOverride{}, err
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	return s.store.SetLimitOverride(ctx, tenantID, code, value, period)
}

func (s *Service) RemoveLimitOverride(ctx context.Context, tenantID, overrideID uuid.UUID) error {
	if err := s.ensureNotArchived(ctx, tenantID); err != nil {
		return err
	}
	return s.store.RemoveLimitOverride(ctx, tenantID, overrideID)
}

func (s *Service) HasFeature(ctx context.Context, tenantID uuid.UUID, code string) (domain.EffectiveFeature, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return domain.EffectiveFeature{}, fmt.Errorf("%w: feature_code is required", domain.ErrInvalidArgument)
	}
	return s.store.HasFeature(ctx, tenantID, code)
}

func (s *Service) GetEffectiveFeatures(ctx context.Context, tenantID uuid.UUID) ([]domain.EffectiveFeature, error) {
	return s.store.GetEffectiveFeatures(ctx, tenantID)
}

func (s *Service) SetFeatureOverride(ctx context.Context, tenantID uuid.UUID, code string, enabled bool, period domain.Period) (domain.FeatureOverride, error) {
	if err := s.ensureNotArchived(ctx, tenantID); err != nil {
		return domain.FeatureOverride{}, err
	}
	if period.StartsAt.IsZero() {
		period.StartsAt = s.now().UTC()
	}
	if err := domain.ValidatePeriod(period); err != nil {
		return domain.FeatureOverride{}, err
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	return s.store.SetFeatureOverride(ctx, tenantID, code, enabled, period)
}

func (s *Service) RemoveFeatureOverride(ctx context.Context, tenantID, overrideID uuid.UUID) error {
	if err := s.ensureNotArchived(ctx, tenantID); err != nil {
		return err
	}
	return s.store.RemoveFeatureOverride(ctx, tenantID, overrideID)
}

func (s *Service) AddDomain(ctx context.Context, tenantID uuid.UUID, name string) (domain.TenantDomain, error) {
	if err := s.ensureNotArchived(ctx, tenantID); err != nil {
		return domain.TenantDomain{}, err
	}
	normalized, err := domain.NormalizeDomain(name)
	if err != nil {
		return domain.TenantDomain{}, err
	}
	return s.store.AddDomain(ctx, tenantID, normalized)
}

func (s *Service) GetDomains(ctx context.Context, tenantID uuid.UUID) ([]domain.TenantDomain, error) {
	return s.store.GetDomains(ctx, tenantID)
}

func (s *Service) VerifyDomain(ctx context.Context, tenantID, domainID uuid.UUID, token string) (domain.TenantDomain, error) {
	if err := s.ensureNotArchived(ctx, tenantID); err != nil {
		return domain.TenantDomain{}, err
	}
	return s.store.VerifyDomain(ctx, tenantID, domainID, strings.TrimSpace(token))
}

func (s *Service) SetPrimaryDomain(ctx context.Context, tenantID, domainID uuid.UUID) (domain.TenantDomain, error) {
	if err := s.ensureNotArchived(ctx, tenantID); err != nil {
		return domain.TenantDomain{}, err
	}
	return s.store.SetPrimaryDomain(ctx, tenantID, domainID)
}

func (s *Service) DisableDomain(ctx context.Context, tenantID, domainID uuid.UUID) (domain.TenantDomain, error) {
	if err := s.ensureNotArchived(ctx, tenantID); err != nil {
		return domain.TenantDomain{}, err
	}
	return s.store.DisableDomain(ctx, tenantID, domainID)
}

func (s *Service) GetBranding(ctx context.Context, tenantID uuid.UUID) (domain.Branding, error) {
	return s.store.GetBranding(ctx, tenantID)
}

func (s *Service) UpdateBranding(ctx context.Context, branding domain.Branding) (domain.Branding, error) {
	if err := s.ensureNotArchived(ctx, branding.TenantID); err != nil {
		return domain.Branding{}, err
	}
	if err := domain.ValidateBranding(branding); err != nil {
		return domain.Branding{}, err
	}
	return s.store.UpdateBranding(ctx, branding)
}

func (s *Service) ensureNotArchived(ctx context.Context, tenantID uuid.UUID) error {
	tenant, err := s.store.GetTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	if tenant.Status == domain.StatusArchived {
		return fmt.Errorf("%w: archived tenant is immutable", domain.ErrInvalidStateTransition)
	}
	return nil
}
