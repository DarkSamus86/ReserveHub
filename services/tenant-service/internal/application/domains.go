package application

import (
	"context"
	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
	"strings"
)

func (s *Service) AddDomain(ctx context.Context, tenantID uuid.UUID, name string) (domain.TenantDomain, error) {
	if err := s.ensureMutable(ctx, tenantID); err != nil {
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
	if err := s.ensureMutable(ctx, tenantID); err != nil {
		return domain.TenantDomain{}, err
	}
	return s.store.VerifyDomain(ctx, tenantID, domainID, strings.TrimSpace(token))
}

func (s *Service) SetPrimaryDomain(ctx context.Context, tenantID, domainID uuid.UUID) (domain.TenantDomain, error) {
	if err := s.ensureMutable(ctx, tenantID); err != nil {
		return domain.TenantDomain{}, err
	}
	return s.store.SetPrimaryDomain(ctx, tenantID, domainID)
}

func (s *Service) DisableDomain(ctx context.Context, tenantID, domainID uuid.UUID) (domain.TenantDomain, error) {
	if err := s.ensureMutable(ctx, tenantID); err != nil {
		return domain.TenantDomain{}, err
	}
	return s.store.DisableDomain(ctx, tenantID, domainID)
}
