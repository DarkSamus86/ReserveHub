package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("provider not found")
	ErrConflict = errors.New("idempotency key conflict")
)

type Provider struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	City      string    `json:"city"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store interface {
	Create(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, [32]byte) (Provider, bool, error)
	List(context.Context, uuid.UUID, int, time.Time, uuid.UUID) ([]Provider, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (Provider, error)
	UpdateProfile(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *string, *string) (Provider, error)
	UpdateStatus(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (Provider, error)
}
