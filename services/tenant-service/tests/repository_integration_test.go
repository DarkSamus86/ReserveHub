//go:build integration

package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"darksamus.com/ReserveHub/tenant-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTenantLifecycleIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	seedMVPEntitlements(t, ctx, databaseURL)

	store, err := repository.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgres() error = %v", err)
	}
	defer store.Close()

	created, err := store.CreateTenant(ctx, domain.CreateTenantInput{
		Name:            "Integration Tenant",
		Slug:            "integration-" + uuid.NewString(),
		Timezone:        "Asia/Almaty",
		CurrencyCode:    "KZT",
		LocaleCode:      "ru-KZ",
		DefaultLanguage: "ru",
		PlanID:          uuid.MustParse("10000000-0000-0000-0000-000000000001"),
	})
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if created.Status != domain.StatusPending {
		t.Fatalf("created status = %s, want %s", created.Status, domain.StatusPending)
	}

	active, err := store.ChangeTenantState(ctx, created.ID, domain.StatusActive)
	if err != nil {
		t.Fatalf("activate tenant: %v", err)
	}
	if active.Status != domain.StatusActive {
		t.Fatalf("active status = %s", active.Status)
	}

	limit, err := store.GetEffectiveLimit(ctx, created.ID, "MAX_DOMAINS")
	if err != nil || limit.Value != 1 || limit.Overridden {
		t.Fatalf("plan limit = %+v, error = %v", limit, err)
	}
	override, err := store.SetLimitOverride(ctx, created.ID, "MAX_DOMAINS", 2, domain.Period{StartsAt: time.Now().Add(-time.Second)})
	if err != nil {
		t.Fatalf("SetLimitOverride() error = %v", err)
	}
	limit, err = store.GetEffectiveLimit(ctx, created.ID, "MAX_DOMAINS")
	if err != nil || limit.Value != 2 || !limit.Overridden {
		t.Fatalf("override limit = %+v, error = %v", limit, err)
	}
	if err := store.RemoveLimitOverride(ctx, created.ID, override.ID); err != nil {
		t.Fatalf("RemoveLimitOverride() error = %v", err)
	}

	feature, err := store.HasFeature(ctx, created.ID, "CUSTOM_DOMAIN")
	if err != nil || !feature.Enabled {
		t.Fatalf("CUSTOM_DOMAIN = %+v, error = %v", feature, err)
	}
	customDomain, err := store.AddDomain(ctx, created.ID, "integration.example.com")
	if err != nil {
		t.Fatalf("AddDomain() error = %v", err)
	}
	verified, err := store.VerifyDomain(ctx, created.ID, customDomain.ID, customDomain.VerificationToken)
	if err != nil || verified.Status != "VERIFIED" {
		t.Fatalf("VerifyDomain() = %+v, error = %v", verified, err)
	}
	primary, err := store.SetPrimaryDomain(ctx, created.ID, customDomain.ID)
	if err != nil || !primary.Primary {
		t.Fatalf("SetPrimaryDomain() = %+v, error = %v", primary, err)
	}

	branding, err := store.UpdateBranding(ctx, domain.Branding{TenantID: created.ID, DisplayName: "Integration", PrimaryColor: "#112233"})
	if err != nil || branding.Status != "ACTIVE" {
		t.Fatalf("UpdateBranding() = %+v, error = %v", branding, err)
	}

	transitions := []string{domain.StatusSuspended, domain.StatusActive, domain.StatusArchived}
	for _, target := range transitions {
		updated, err := store.ChangeTenantState(ctx, created.ID, target)
		if err != nil {
			t.Fatalf("ChangeTenantState(%s) error = %v", target, err)
		}
		if updated.Status != target {
			t.Fatalf("status = %s, want %s", updated.Status, target)
		}
	}

	events, err := store.FetchOutbox(ctx, 100)
	if err != nil {
		t.Fatalf("FetchOutbox() error = %v", err)
	}
	if len(events) < 10 {
		t.Fatalf("outbox events = %d, want at least 10", len(events))
	}
}

func seedMVPEntitlements(t *testing.T, ctx context.Context, databaseURL string) {
	t.Helper()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create seed pool: %v", err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `
		INSERT INTO ln_plan_limit (plan_id, limit_type_id, limit_value)
		SELECT '10000000-0000-0000-0000-000000000001', id, 1
		FROM di_limit_type WHERE code = 'MAX_DOMAINS';

		INSERT INTO ln_plan_feature (plan_id, feature_id, enabled)
		SELECT '10000000-0000-0000-0000-000000000001', id, true
		FROM di_feature WHERE code IN ('CUSTOM_DOMAIN', 'CUSTOM_BRANDING');`)
	if err != nil {
		t.Fatalf("seed entitlements: %v", err)
	}
}
