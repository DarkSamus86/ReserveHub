package tests

import (
	"context"
	"errors"
	"testing"

	"darksamus.com/ReserveHub/tenant-service/internal/application"
	"darksamus.com/ReserveHub/tenant-service/internal/domain"

	"github.com/google/uuid"
)

type applicationStore struct {
	application.Repository
	tenant        domain.Tenant
	settings      domain.Settings
	settingsCalls int
	stateCalls    int
}

func (s *applicationStore) GetTenant(context.Context, uuid.UUID) (domain.Tenant, error) {
	return s.tenant, nil
}

func (s *applicationStore) ChangeTenantState(context.Context, uuid.UUID, string) (domain.Tenant, error) {
	s.stateCalls++
	return s.tenant, nil
}

func (s *applicationStore) UpdateSettings(_ context.Context, in domain.UpdateSettingsInput) (domain.Settings, error) {
	s.settingsCalls++
	s.settings = domain.Settings{
		TenantID:        in.TenantID,
		Timezone:        in.Timezone,
		CurrencyCode:    in.CurrencyCode,
		LocaleCode:      in.LocaleCode,
		DefaultLanguage: in.DefaultLanguage,
	}
	return s.settings, nil
}

func TestApplicationRejectsInvalidStateTransitionBeforeRepository(t *testing.T) {
	t.Parallel()

	store := &applicationStore{tenant: domain.Tenant{ID: uuid.New(), Status: domain.StatusArchived}}
	service := application.New(store)
	_, err := service.ChangeState(context.Background(), store.tenant.ID, domain.StatusActive)
	if !errors.Is(err, domain.ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
	if store.stateCalls != 0 {
		t.Fatalf("repository was called %d times", store.stateCalls)
	}
}

func TestApplicationRejectsArchivedTenantMutation(t *testing.T) {
	t.Parallel()

	store := &applicationStore{tenant: domain.Tenant{ID: uuid.New(), Status: domain.StatusArchived}}
	service := application.New(store)
	_, err := service.UpdateSettings(context.Background(), domain.UpdateSettingsInput{
		TenantID: store.tenant.ID, Timezone: "UTC", CurrencyCode: "USD",
		LocaleCode: "en-US", DefaultLanguage: "en",
	})
	if !errors.Is(err, domain.ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
	if store.settingsCalls != 0 {
		t.Fatalf("repository was called %d times", store.settingsCalls)
	}
}

func TestApplicationRejectsSuspendedTenantMutation(t *testing.T) {
	t.Parallel()

	store := &applicationStore{tenant: domain.Tenant{ID: uuid.New(), Status: domain.StatusSuspended}}
	service := application.New(store)
	_, err := service.UpdateSettings(context.Background(), domain.UpdateSettingsInput{
		TenantID: store.tenant.ID, Timezone: "UTC", CurrencyCode: "USD",
		LocaleCode: "en-US", DefaultLanguage: "en",
	})
	if !errors.Is(err, domain.ErrInvalidStateTransition) || store.settingsCalls != 0 {
		t.Fatalf("suspended write: calls = %d, error = %v", store.settingsCalls, err)
	}
}

func TestApplicationValidatesSettingsBeforeRepository(t *testing.T) {
	t.Parallel()

	store := &applicationStore{tenant: domain.Tenant{ID: uuid.New(), Status: domain.StatusActive}}
	service := application.New(store)
	_, err := service.UpdateSettings(context.Background(), domain.UpdateSettingsInput{
		TenantID: store.tenant.ID, Timezone: "Unknown/Timezone", CurrencyCode: "USD",
		LocaleCode: "en-US", DefaultLanguage: "en",
	})
	if !errors.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got %v", err)
	}
	if store.settingsCalls != 0 {
		t.Fatalf("repository was called %d times", store.settingsCalls)
	}
}

func TestApplicationUpdatesValidSettings(t *testing.T) {
	t.Parallel()

	store := &applicationStore{tenant: domain.Tenant{ID: uuid.New(), Status: domain.StatusActive}}
	service := application.New(store)
	result, err := service.UpdateSettings(context.Background(), domain.UpdateSettingsInput{
		TenantID: store.tenant.ID, Timezone: "Asia/Almaty", CurrencyCode: "KZT",
		LocaleCode: "ru-KZ", DefaultLanguage: "ru",
	})
	if err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if result.Timezone != "Asia/Almaty" || store.settingsCalls != 1 {
		t.Fatalf("unexpected result = %+v, calls = %d", result, store.settingsCalls)
	}
}
