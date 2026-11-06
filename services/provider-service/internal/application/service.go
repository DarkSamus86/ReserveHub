package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"darksamus.com/ReserveHub/provider-service/internal/domain"
	"github.com/google/uuid"
)

var (
	ErrTenantUnavailable = errors.New("tenant service unavailable")
	ErrTenantArchived    = errors.New("tenant archived")
	ErrTenantInactive    = errors.New("tenant inactive")
)

type TenantLookup func(context.Context, uuid.UUID, string) (string, error)

type Service struct {
	Store  domain.Store
	Tenant TenantLookup
}

func (s Service) CheckTenant(ctx context.Context, id uuid.UUID, bearer string, platformAdmin, mutation bool) error {
	if s.Tenant == nil {
		return ErrTenantUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	status, err := s.Tenant(ctx, id, bearer)
	if err != nil {
		return err
	}
	if !platformAdmin && status == "ARCHIVED" {
		return ErrTenantArchived
	}
	if mutation && status != "ACTIVE" {
		return ErrTenantInactive
	}
	return nil
}

func (s Service) Create(ctx context.Context, operation, tenant, actor uuid.UUID, name, city string) (domain.Provider, bool, error) {
	digest := sha256.Sum256([]byte(tenant.String() + "\x00" + name + "\x00" + city))
	return s.Store.Create(ctx, operation, tenant, actor, name, city, digest)
}

func (s Service) List(ctx context.Context, tenant uuid.UUID, limit int, before time.Time, beforeID uuid.UUID) ([]domain.Provider, error) {
	return s.Store.List(ctx, tenant, limit, before, beforeID)
}
func (s Service) Get(ctx context.Context, tenant, id uuid.UUID) (domain.Provider, error) {
	return s.Store.Get(ctx, tenant, id)
}
func (s Service) UpdateProfile(ctx context.Context, tenant, id, actor uuid.UUID, name, city *string) (domain.Provider, error) {
	return s.Store.UpdateProfile(ctx, tenant, id, actor, name, city)
}
func (s Service) UpdateStatus(ctx context.Context, tenant, id, actor uuid.UUID, status string) (domain.Provider, error) {
	return s.Store.UpdateStatus(ctx, tenant, id, actor, status)
}
