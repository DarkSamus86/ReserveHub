package application

import (
	"context"
	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"fmt"
	"github.com/google/uuid"
	"strings"
)

func (s *Service) GetEffectiveLimit(ctx context.Context, tenantID uuid.UUID, code string) (domain.EffectiveLimit, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return domain.EffectiveLimit{}, fmt.Errorf("%w: limit_code is required", domain.ErrInvalidArgument)
	}
	return s.store.GetEffectiveLimit(ctx, tenantID, code)
}

func (s *Service) SetLimitOverride(ctx context.Context, tenantID uuid.UUID, code string, value int64, period domain.Period) (domain.LimitOverride, error) {
	if err := s.ensureMutable(ctx, tenantID); err != nil {
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
	if err := s.ensureMutable(ctx, tenantID); err != nil {
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
	if err := s.ensureMutable(ctx, tenantID); err != nil {
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
	if err := s.ensureMutable(ctx, tenantID); err != nil {
		return err
	}
	return s.store.RemoveFeatureOverride(ctx, tenantID, overrideID)
}
