package application

import (
	"context"
	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
)

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
