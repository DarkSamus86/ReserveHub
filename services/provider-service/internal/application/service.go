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

func (s Service) Create(ctx context.Context, operation, tenant, actor uuid.UUID, name, city string, cityID *uuid.UUID) (domain.Provider, bool, error) {
	fingerprint := tenant.String() + "\x00" + name + "\x00" + city
	if cityID != nil {
		fingerprint += "\x00" + cityID.String()
	}
	digest := sha256.Sum256([]byte(fingerprint))
	return s.Store.Create(ctx, operation, tenant, actor, name, city, cityID, digest)
}

func (s Service) List(ctx context.Context, tenant uuid.UUID, limit int, before time.Time, beforeID uuid.UUID) ([]domain.Provider, error) {
	return s.Store.List(ctx, tenant, limit, before, beforeID)
}
func (s Service) Get(ctx context.Context, tenant, id uuid.UUID) (domain.Provider, error) {
	return s.Store.Get(ctx, tenant, id)
}
func (s Service) UpdateProfile(ctx context.Context, tenant, id, actor uuid.UUID, name, city *string, cityID *uuid.UUID) (domain.Provider, error) {
	return s.Store.UpdateProfile(ctx, tenant, id, actor, name, city, cityID)
}

func (s Service) ListCountries(ctx context.Context) ([]domain.Country, error) {
	return s.Store.ListCountries(ctx)
}

func (s Service) ListCities(ctx context.Context, countryID uuid.UUID, query string, limit int) ([]domain.City, error) {
	return s.Store.ListCities(ctx, countryID, query, limit)
}
func (s Service) UpdateStatus(ctx context.Context, tenant, id, actor uuid.UUID, status string) (domain.Provider, error) {
	return s.Store.UpdateStatus(ctx, tenant, id, actor, status)
}
