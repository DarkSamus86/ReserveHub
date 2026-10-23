package application

import (
	"context"
	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
	"time"
)

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
