package tests

import (
	"errors"
	"testing"
	"time"

	. "darksamus.com/ReserveHub/tenant-service/internal/domain"
)

func TestValidateTransition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{name: "pending to active", from: StatusPending, to: StatusActive},
		{name: "active to suspended", from: StatusActive, to: StatusSuspended},
		{name: "suspended to active", from: StatusSuspended, to: StatusActive},
		{name: "active to archived", from: StatusActive, to: StatusArchived},
		{name: "archived is terminal", from: StatusArchived, to: StatusActive, wantErr: true},
		{name: "cannot suspend pending", from: StatusPending, to: StatusSuspended, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateTransition() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidStateTransition) {
				t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
			}
		})
	}
}

func TestValidateCreateTenant(t *testing.T) {
	t.Parallel()

	in := CreateTenantInput{
		Name:            "Example Tenant",
		Slug:            "Example-Tenant",
		Timezone:        "Asia/Almaty",
		CurrencyCode:    "KZT",
		LocaleCode:      "ru-KZ",
		DefaultLanguage: "ru",
		PlanID:          [16]byte{1},
	}
	if err := ValidateCreateTenant(&in); err != nil {
		t.Fatalf("ValidateCreateTenant() error = %v", err)
	}
	if in.Slug != "example-tenant" {
		t.Fatalf("slug = %q, want normalized slug", in.Slug)
	}
}

func TestValidatePeriod(t *testing.T) {
	t.Parallel()

	start := time.Now()
	end := start.Add(-time.Minute)
	err := ValidatePeriod(Period{StartsAt: start, EndsAt: &end})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestNormalizeDomain(t *testing.T) {
	t.Parallel()

	got, err := NormalizeDomain(" Events.Example.COM. ")
	if err != nil {
		t.Fatalf("NormalizeDomain() error = %v", err)
	}
	if got != "events.example.com" {
		t.Fatalf("NormalizeDomain() = %q", got)
	}
}
