package application

import (
	"context"
	"fmt"
	"strings"

	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
)

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
	if err := s.ensureMutable(ctx, in.TenantID); err != nil {
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
	current, err := s.store.GetTenant(ctx, id)
	if err != nil {
		return domain.Tenant{}, err
	}
	if err := domain.ValidateTransition(current.Status, target); err != nil {
		return domain.Tenant{}, err
	}
	return s.store.ChangeTenantState(ctx, id, target)
}

func (s *Service) GetSettings(ctx context.Context, id uuid.UUID) (domain.Settings, error) {
	return s.store.GetSettings(ctx, id)
}

func (s *Service) UpdateSettings(ctx context.Context, in domain.UpdateSettingsInput) (domain.Settings, error) {
	if err := s.ensureMutable(ctx, in.TenantID); err != nil {
		return domain.Settings{}, err
	}
	if err := domain.ValidateSettings(in.Timezone, in.CurrencyCode, in.LocaleCode, in.DefaultLanguage); err != nil {
		return domain.Settings{}, err
	}
	return s.store.UpdateSettings(ctx, in)
}

func (s *Service) ensureMutable(ctx context.Context, tenantID uuid.UUID) error {
	tenant, err := s.store.GetTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	if tenant.Status == domain.StatusArchived || tenant.Status == domain.StatusSuspended {
		return fmt.Errorf("%w: %s tenant is immutable", domain.ErrInvalidStateTransition, tenant.Status)
	}
	return nil
}
