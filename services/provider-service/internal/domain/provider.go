package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("provider not found")
	ErrConflict        = errors.New("idempotency key conflict")
	ErrInvalidCity     = errors.New("invalid provider city")
	ErrCountryNotFound = errors.New("country not found")
)

type Provider struct {
	ID        uuid.UUID  `json:"id"`
	TenantID  uuid.UUID  `json:"tenant_id"`
	Name      string     `json:"name"`
	City      string     `json:"city"`
	CityID    *uuid.UUID `json:"city_id,omitempty"`
	CountryID *uuid.UUID `json:"country_id,omitempty"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Country struct {
	ID     uuid.UUID
	Code   string
	NameRU string
	NameEN string
}

type City struct {
	ID        uuid.UUID
	CountryID uuid.UUID
	NameRU    string
	NameEN    string
}

type Store interface {
	Create(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, *uuid.UUID, [32]byte) (Provider, bool, error)
	List(context.Context, uuid.UUID, int, time.Time, uuid.UUID) ([]Provider, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (Provider, error)
	UpdateProfile(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *string, *string, *uuid.UUID) (Provider, error)
	UpdateStatus(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (Provider, error)
	ListCountries(context.Context) ([]Country, error)
	ListCities(context.Context, uuid.UUID, string, int) ([]City, error)
}
