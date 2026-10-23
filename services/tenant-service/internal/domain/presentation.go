package domain

import (
	"fmt"
	"github.com/google/uuid"
	"net"
	"strings"
	"time"
)

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
