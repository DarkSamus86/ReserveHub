package application

import (
	"context"
	"time"

	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
)

// Repository is the persistence port required by application use cases.
type Repository interface {
	CreateTenant(context.Context, domain.CreateTenantInput) (domain.Tenant, error)
	GetTenant(context.Context, uuid.UUID) (domain.Tenant, error)
	ListTenants(context.Context, int, int, string) ([]domain.Tenant, bool, error)
	UpdateTenant(context.Context, domain.UpdateTenantInput) (domain.Tenant, error)
	ChangeTenantState(context.Context, uuid.UUID, string) (domain.Tenant, error)

	GetSettings(context.Context, uuid.UUID) (domain.Settings, error)
	UpdateSettings(context.Context, domain.UpdateSettingsInput) (domain.Settings, error)

	GetCurrentPlan(context.Context, uuid.UUID) (domain.TenantPlan, error)
	GetPlanHistory(context.Context, uuid.UUID) ([]domain.TenantPlan, error)
	ChangePlan(context.Context, uuid.UUID, uuid.UUID, time.Time) (domain.TenantPlan, error)

	GetEffectiveLimit(context.Context, uuid.UUID, string) (domain.EffectiveLimit, error)
	SetLimitOverride(context.Context, uuid.UUID, string, int64, domain.Period) (domain.LimitOverride, error)
	RemoveLimitOverride(context.Context, uuid.UUID, uuid.UUID) error

	HasFeature(context.Context, uuid.UUID, string) (domain.EffectiveFeature, error)
	GetEffectiveFeatures(context.Context, uuid.UUID) ([]domain.EffectiveFeature, error)
	SetFeatureOverride(context.Context, uuid.UUID, string, bool, domain.Period) (domain.FeatureOverride, error)
	RemoveFeatureOverride(context.Context, uuid.UUID, uuid.UUID) error

	AddDomain(context.Context, uuid.UUID, string) (domain.TenantDomain, error)
	GetDomains(context.Context, uuid.UUID) ([]domain.TenantDomain, error)
	VerifyDomain(context.Context, uuid.UUID, uuid.UUID, string) (domain.TenantDomain, error)
	SetPrimaryDomain(context.Context, uuid.UUID, uuid.UUID) (domain.TenantDomain, error)
	DisableDomain(context.Context, uuid.UUID, uuid.UUID) (domain.TenantDomain, error)

	GetBranding(context.Context, uuid.UUID) (domain.Branding, error)
	UpdateBranding(context.Context, domain.Branding) (domain.Branding, error)
}
