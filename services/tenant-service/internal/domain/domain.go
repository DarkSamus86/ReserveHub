package domain

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTenantNotFound         = errors.New("tenant not found")
	ErrSlugAlreadyExists      = errors.New("tenant slug already exists")
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
	ErrDomainLimitExceeded    = errors.New("domain limit exceeded")
	ErrFeatureDisabled        = errors.New("required feature is disabled")
	ErrBrandingNotAllowed     = errors.New("custom branding is not allowed")
	ErrInvalidArgument        = errors.New("invalid argument")
)

const (
	StatusPending   = "PENDING"
	StatusActive    = "ACTIVE"
	StatusSuspended = "SUSPENDED"
	StatusArchived  = "ARCHIVED"
)

var (
	slugPattern     = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	localePattern   = regexp.MustCompile(`^[a-z]{2,3}(?:-[A-Z]{2})?$`)
	languagePattern = regexp.MustCompile(`^[a-z]{2,3}$`)
	colorPattern    = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

type Tenant struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ActivatedAt *time.Time
	SuspendedAt *time.Time
	ArchivedAt  *time.Time
}

type Settings struct {
	TenantID        uuid.UUID
	Timezone        string
	CurrencyCode    string
	LocaleCode      string
	DefaultLanguage string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

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

type TenantDomain struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	Domain            string
	Status            string
	Primary           bool
	VerificationToken string
	VerifiedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Branding struct {
	TenantID       uuid.UUID
	Status         string
	DisplayName    string
	LogoURL        string
	FaviconURL     string
	PrimaryColor   string
	SecondaryColor string
	AccentColor    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OutboxEvent struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       []byte
	CreatedAt     time.Time
}

type CreateTenantInput struct {
	Name            string
	Slug            string
	Timezone        string
	CurrencyCode    string
	LocaleCode      string
	DefaultLanguage string
	PlanID          uuid.UUID
}

type UpdateTenantInput struct {
	TenantID uuid.UUID
	Name     *string
	Slug     *string
}

type UpdateSettingsInput struct {
	TenantID        uuid.UUID
	Timezone        string
	CurrencyCode    string
	LocaleCode      string
	DefaultLanguage string
}

type Period struct {
	StartsAt time.Time
	EndsAt   *time.Time
}

func ParseID(value, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %s must be a UUID", ErrInvalidArgument, field)
	}
	return id, nil
}

func ValidateCreateTenant(in *CreateTenantInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 255 {
		return fmt.Errorf("%w: name is required and must not exceed 255 characters", ErrInvalidArgument)
	}
	var err error
	if in.Slug, err = NormalizeSlug(in.Slug); err != nil {
		return err
	}
	if err := ValidateSettings(in.Timezone, in.CurrencyCode, in.LocaleCode, in.DefaultLanguage); err != nil {
		return err
	}
	if in.PlanID == uuid.Nil {
		return fmt.Errorf("%w: plan_id is required", ErrInvalidArgument)
	}
	return nil
}

func NormalizeSlug(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !slugPattern.MatchString(value) || len(value) > 100 {
		return "", fmt.Errorf("%w: slug must contain lowercase letters, digits and single hyphens", ErrInvalidArgument)
	}
	return value, nil
}

func ValidateSettings(timezone, currency, locale, language string) error {
	if _, err := time.LoadLocation(timezone); err != nil {
		return fmt.Errorf("%w: unknown timezone", ErrInvalidArgument)
	}
	if !currencyPattern.MatchString(currency) {
		return fmt.Errorf("%w: currency_code must be an uppercase ISO 4217 code", ErrInvalidArgument)
	}
	if !localePattern.MatchString(locale) {
		return fmt.Errorf("%w: locale_code is invalid", ErrInvalidArgument)
	}
	if !languagePattern.MatchString(language) {
		return fmt.Errorf("%w: default_language is invalid", ErrInvalidArgument)
	}
	return nil
}

func ValidateTransition(from, to string) error {
	allowed := map[string]map[string]bool{
		StatusPending:   {StatusActive: true, StatusArchived: true},
		StatusActive:    {StatusSuspended: true, StatusArchived: true},
		StatusSuspended: {StatusActive: true, StatusArchived: true},
	}
	if !allowed[from][to] {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidStateTransition, from, to)
	}
	return nil
}

func ValidatePeriod(period Period) error {
	if period.StartsAt.IsZero() {
		return fmt.Errorf("%w: starts_at is required", ErrInvalidArgument)
	}
	if period.EndsAt != nil && !period.EndsAt.After(period.StartsAt) {
		return fmt.Errorf("%w: ends_at must be after starts_at", ErrInvalidArgument)
	}
	return nil
}

func NormalizeDomain(value string) (string, error) {
	value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if value == "" || len(value) > 253 || net.ParseIP(value) != nil || strings.Contains(value, ":") {
		return "", fmt.Errorf("%w: invalid domain", ErrInvalidArgument)
	}
	labels := strings.Split(value, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("%w: domain must contain a public suffix", ErrInvalidArgument)
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || !slugPattern.MatchString(label) {
			return "", fmt.Errorf("%w: invalid domain", ErrInvalidArgument)
		}
	}
	return value, nil
}

func ValidateBranding(branding Branding) error {
	for name, value := range map[string]string{
		"primary_color":   branding.PrimaryColor,
		"secondary_color": branding.SecondaryColor,
		"accent_color":    branding.AccentColor,
	} {
		if value != "" && !colorPattern.MatchString(value) {
			return fmt.Errorf("%w: %s must use #RRGGBB format", ErrInvalidArgument, name)
		}
	}
	return nil
}
