package domain

import (
	"github.com/google/uuid"
	"time"
)

type EffectiveLimit struct {
	TenantID   uuid.UUID
	Code       string
	Value      int64
	Overridden bool
}

type LimitOverride struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Code      string
	Value     int64
	StartsAt  time.Time
	EndsAt    *time.Time
	CreatedAt time.Time
}

type EffectiveFeature struct {
	TenantID   uuid.UUID
	Code       string
	Enabled    bool
	Overridden bool
}

type FeatureOverride struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Code      string
	Enabled   bool
	StartsAt  time.Time
	EndsAt    *time.Time
	CreatedAt time.Time
}
