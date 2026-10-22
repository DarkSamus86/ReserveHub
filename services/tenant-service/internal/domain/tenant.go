package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
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
