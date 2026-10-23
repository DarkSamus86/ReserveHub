package domain

import (
	"github.com/google/uuid"
	"time"
)

type Plan struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	Active      bool
}

type TenantPlan struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Plan      Plan
	StartsAt  time.Time
	EndsAt    *time.Time
	CreatedAt time.Time
}

type Period struct {
	StartsAt time.Time
	EndsAt   *time.Time
}
