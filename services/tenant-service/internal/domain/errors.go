package domain

import (
	"errors"
)

var (
	ErrTenantNotFound         = errors.New("tenant not found")
	ErrSlugAlreadyExists      = errors.New("tenant slug already exists")
	ErrIdempotencyConflict    = errors.New("operation id already used for different tenant data")
	ErrInvalidStateTransition = errors.New("invalid tenant status transition")
	ErrPlanNotFound           = errors.New("plan not found")
	ErrPlanInactive           = errors.New("plan is inactive")
	ErrTenantPlanNotFound     = errors.New("tenant plan not found")
	ErrLimitTypeNotFound      = errors.New("limit type not found")
	ErrEffectiveLimitNotFound = errors.New("effective limit not found")
	ErrFeatureNotFound        = errors.New("feature not found")
	ErrDomainAlreadyExists    = errors.New("domain already exists")
	ErrDomainNotFound         = errors.New("domain not found")
	ErrDomainNotVerified      = errors.New("domain is not verified")
	ErrDomainProofUnavailable = errors.New("domain ownership proof is temporarily unavailable")
	ErrDomainLimitExceeded    = errors.New("domain limit exceeded")
	ErrFeatureDisabled        = errors.New("required feature is disabled")
	ErrBrandingNotAllowed     = errors.New("custom branding is not allowed")
	ErrInvalidArgument        = errors.New("invalid argument")
)
