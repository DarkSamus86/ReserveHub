package application

import (
	"context"
	"errors"
	"testing"

	"darksamus.com/ReserveHub/tenant-service/internal/authn"
	"darksamus.com/ReserveHub/tenant-service/internal/domain"

	"github.com/google/uuid"
)

type applicationStore struct {
	Repository
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
	service := New(store)
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
	service := New(store)
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
	service := New(store)
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
	service := New(store)
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
	service := New(store)
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

func TestApplicationRejectsArchivedMemberReadAndPendingMemberWrite(t *testing.T) {
	t.Parallel()

	tid := uuid.New()
	memberCtx := authn.WithClaims(context.Background(), authn.Claims{
		Subject:           "user-1",
		Role:              authn.RoleUser,
		Scope:             "tenant",
		TenantID:          tid.String(),
		PermissionProfile: authn.ProfileTenantOwner,
	})
	adminCtx := authn.WithClaims(context.Background(), authn.Claims{
		Subject: "admin-1",
		Role:    authn.RoleAdmin,
		Scope:   "platform",
	})

	archivedStore := &applicationStore{tenant: domain.Tenant{ID: tid, Status: domain.StatusArchived}}
	archivedSvc := New(archivedStore)
	if _, err := archivedSvc.GetTenant(memberCtx, tid); !errors.Is(err, domain.ErrInvalidStateTransition) {
		t.Fatalf("expected archived tenant member read to fail with ErrInvalidStateTransition, got %v", err)
	}
	if _, err := archivedSvc.GetTenant(adminCtx, tid); err != nil {
		t.Fatalf("expected platform admin read on archived tenant to succeed, got %v", err)
	}

	pendingStore := &applicationStore{tenant: domain.Tenant{ID: tid, Status: domain.StatusPending}}
	pendingSvc := New(pendingStore)
	_, err := pendingSvc.UpdateSettings(memberCtx, domain.UpdateSettingsInput{
		TenantID: tid, Timezone: "Asia/Almaty", CurrencyCode: "KZT", LocaleCode: "ru-KZ", DefaultLanguage: "ru",
	})
	if !errors.Is(err, domain.ErrInvalidStateTransition) || pendingStore.settingsCalls != 0 {
		t.Fatalf("expected pending tenant member write to fail before repository, err=%v calls=%d", err, pendingStore.settingsCalls)
	}
}
