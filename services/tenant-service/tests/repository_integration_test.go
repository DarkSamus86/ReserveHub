package tests

import (
	"context"
	"errors"
	"net"
	"os"
	"reflect"
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

	proofs := make(map[string]string)
	var resolverErr error
	store, err := repository.NewPostgresWithTXTResolver(ctx, databaseURL, func(_ context.Context, name string) ([]string, error) {
		if resolverErr != nil {
			return nil, resolverErr
		}
		if value := proofs[name]; value != "" {
			return []string{value}, nil
		}
		return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
	})
	if err != nil {
		t.Fatalf("NewPostgres() error = %v", err)
	}
	defer store.Close()

	createInput := domain.CreateTenantInput{
		OperationID:     uuid.New(),
		Name:            "Integration Tenant",
		Slug:            "integration-" + uuid.NewString(),
		Timezone:        "Asia/Almaty",
		CurrencyCode:    "KZT",
		LocaleCode:      "ru-KZ",
		DefaultLanguage: "ru",
		PlanID:          uuid.MustParse("10000000-0000-0000-0000-000000000001"),
	}
	created, err := store.CreateTenant(ctx, createInput)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if created.Status != domain.StatusPending {
		t.Fatalf("created status = %s, want %s", created.Status, domain.StatusPending)
	}
	replayed, err := store.CreateTenant(ctx, createInput)
	if err != nil || replayed.ID != created.ID {
		t.Fatalf("idempotent create returned tenant %v, error %v; want %v", replayed.ID, err, created.ID)
	}
	conflicting := createInput
	conflicting.Name = "Different company"
	if _, err := store.CreateTenant(ctx, conflicting); !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("same operation with different data: error = %v", err)
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
	customDomain, err := store.AddDomain(ctx, created.ID, "integration-"+uuid.NewString()+".example.com")
	if err != nil {
		t.Fatalf("AddDomain() error = %v", err)
	}
	resolverErr = errors.New("DNS temporarily unavailable")
	if _, err := store.VerifyDomain(ctx, created.ID, customDomain.ID, customDomain.VerificationToken); !errors.Is(err, domain.ErrDomainProofUnavailable) {
		t.Fatalf("transient DNS failure: error = %v", err)
	}
	domainsAfterResolverError, err := store.GetDomains(ctx, created.ID)
	if err != nil || len(domainsAfterResolverError) != 1 || domainsAfterResolverError[0].Status != "PENDING" {
		t.Fatalf("transient DNS failure changed domain: domains = %+v, error = %v", domainsAfterResolverError, err)
	}
	eventsAfterResolverError, err := store.FetchOutbox(ctx, 1000)
	if err != nil || countTenantEvents(eventsAfterResolverError, created.ID, "tenant.domain.verification_failed") != 0 {
		t.Fatalf("transient DNS failure emitted event: error = %v", err)
	}
	resolverErr = nil
	noProof, err := store.VerifyDomain(ctx, created.ID, customDomain.ID, customDomain.VerificationToken)
	if err != nil || noProof.Status != "FAILED" {
		t.Fatalf("token without DNS proof: domain = %+v, error = %v", noProof, err)
	}
	proofs["_reservehub-verification."+customDomain.Domain] = "wrong-token"
	wrongProof, err := store.VerifyDomain(ctx, created.ID, customDomain.ID, customDomain.VerificationToken)
	if err != nil || wrongProof.Status != "FAILED" {
		t.Fatalf("mismatched DNS proof: domain = %+v, error = %v", wrongProof, err)
	}
	wrongToken, err := store.VerifyDomain(ctx, created.ID, customDomain.ID, "wrong-client-token")
	if err != nil || wrongToken.Status != "FAILED" {
		t.Fatalf("mismatched client token: domain = %+v, error = %v", wrongToken, err)
	}
	domainsAfterRejectedProof, err := store.GetDomains(ctx, created.ID)
	if err != nil || len(domainsAfterRejectedProof) != 1 || domainsAfterRejectedProof[0].Status != "FAILED" || domainsAfterRejectedProof[0].VerifiedAt != nil {
		t.Fatalf("rejected proof changed domain: domains = %+v, error = %v", domainsAfterRejectedProof, err)
	}
	proofEvents, err := store.FetchOutbox(ctx, 1000)
	if err != nil || countTenantEvents(proofEvents, created.ID, "tenant.domain.verification_failed") != 3 {
		t.Fatalf("failed proofs did not emit three events: error = %v", err)
	}
	proofs["_reservehub-verification."+customDomain.Domain] = customDomain.VerificationToken
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

	futureT2 := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Microsecond)
	if _, err := store.ChangePlan(ctx, created.ID, uuid.MustParse("10000000-0000-0000-0000-000000000002"), futureT2); err != nil {
		t.Fatalf("schedule future plan: %v", err)
	}
	historyBefore, err := store.GetPlanHistory(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetPlanHistory(): %v", err)
	}
	if len(historyBefore) != 2 || historyBefore[0].EndsAt != nil || !historyBefore[0].StartsAt.Equal(futureT2) || historyBefore[1].EndsAt == nil || !historyBefore[1].EndsAt.Equal(futureT2) {
		t.Fatalf("future plan intervals overlap or have a gap: %+v", historyBefore)
	}
	eventsBeforeRejectedSchedule, err := store.FetchOutbox(ctx, 1000)
	if err != nil {
		t.Fatalf("FetchOutbox() before rejected schedule: %v", err)
	}
	if _, err := store.ChangePlan(ctx, created.ID, uuid.MustParse("10000000-0000-0000-0000-000000000003"), futureT2.Add(-time.Hour)); !errors.Is(err, domain.ErrInvalidStateTransition) {
		t.Fatalf("overlapping plan schedule: error = %v", err)
	}
	historyAfter, err := store.GetPlanHistory(ctx, created.ID)
	if err != nil || !reflect.DeepEqual(historyAfter, historyBefore) {
		t.Fatalf("rejected schedule changed history: before = %+v, after = %+v, error = %v", historyBefore, historyAfter, err)
	}
	eventsAfterRejectedSchedule, err := store.FetchOutbox(ctx, 1000)
	if err != nil || len(eventsAfterRejectedSchedule) != len(eventsBeforeRejectedSchedule) {
		t.Fatalf("rejected schedule changed outbox: before = %d, after = %d, error = %v", len(eventsBeforeRejectedSchedule), len(eventsAfterRejectedSchedule), err)
	}

	featureOverride, err := store.SetFeatureOverride(ctx, created.ID, "CUSTOM_DOMAIN", true, domain.Period{StartsAt: time.Now().Add(-time.Second)})
	if err != nil {
		t.Fatalf("SetFeatureOverride(): %v", err)
	}
	suspended, err := store.ChangeTenantState(ctx, created.ID, domain.StatusSuspended)
	if err != nil || suspended.Status != domain.StatusSuspended {
		t.Fatalf("suspend tenant: %+v, %v", suspended, err)
	}
	eventsBeforeWrites, err := store.FetchOutbox(ctx, 1000)
	if err != nil {
		t.Fatalf("FetchOutbox() before suspended writes: %v", err)
	}
	name := "Blocked while suspended"
	writes := []struct {
		name string
		run  func() error
	}{
		{"UpdateTenant", func() error {
			_, err := store.UpdateTenant(ctx, domain.UpdateTenantInput{TenantID: created.ID, Name: &name})
			return err
		}},
		{"UpdateSettings", func() error {
			_, err := store.UpdateSettings(ctx, domain.UpdateSettingsInput{TenantID: created.ID, Timezone: "UTC", CurrencyCode: "USD", LocaleCode: "en-US", DefaultLanguage: "en"})
			return err
		}},
		{"ChangePlan", func() error {
			_, err := store.ChangePlan(ctx, created.ID, uuid.MustParse("10000000-0000-0000-0000-000000000003"), futureT2.Add(time.Hour))
			return err
		}},
		{"SetLimitOverride", func() error {
			_, err := store.SetLimitOverride(ctx, created.ID, "MAX_DOMAINS", 3, domain.Period{StartsAt: time.Now()})
			return err
		}},
		{"RemoveLimitOverride", func() error { return store.RemoveLimitOverride(ctx, created.ID, override.ID) }},
		{"SetFeatureOverride", func() error {
			_, err := store.SetFeatureOverride(ctx, created.ID, "CUSTOM_DOMAIN", false, domain.Period{StartsAt: time.Now()})
			return err
		}},
		{"RemoveFeatureOverride", func() error { return store.RemoveFeatureOverride(ctx, created.ID, featureOverride.ID) }},
		{"AddDomain", func() error { _, err := store.AddDomain(ctx, created.ID, "other.example.com"); return err }},
		{"VerifyDomain", func() error {
			_, err := store.VerifyDomain(ctx, created.ID, customDomain.ID, customDomain.VerificationToken)
			return err
		}},
		{"SetPrimaryDomain", func() error { _, err := store.SetPrimaryDomain(ctx, created.ID, customDomain.ID); return err }},
		{"DisableDomain", func() error { _, err := store.DisableDomain(ctx, created.ID, customDomain.ID); return err }},
		{"UpdateBranding", func() error {
			_, err := store.UpdateBranding(ctx, domain.Branding{TenantID: created.ID, DisplayName: name})
			return err
		}},
	}
	for _, write := range writes {
		if err := write.run(); !errors.Is(err, domain.ErrInvalidStateTransition) {
			t.Errorf("%s while suspended: error = %v", write.name, err)
		}
	}
	unchangedTenant, err := store.GetTenant(ctx, created.ID)
	if err != nil || unchangedTenant.Name != created.Name || unchangedTenant.Status != domain.StatusSuspended {
		t.Fatalf("suspended writes changed tenant: %+v, error = %v", unchangedTenant, err)
	}
	unchangedSettings, err := store.GetSettings(ctx, created.ID)
	if err != nil || unchangedSettings.Timezone != "Asia/Almaty" {
		t.Fatalf("suspended writes changed settings: %+v, error = %v", unchangedSettings, err)
	}
	unchangedDomains, err := store.GetDomains(ctx, created.ID)
	if err != nil || len(unchangedDomains) != 1 || unchangedDomains[0].Status != "VERIFIED" || !unchangedDomains[0].Primary {
		t.Fatalf("suspended writes changed domains: %+v, error = %v", unchangedDomains, err)
	}
	unchangedBranding, err := store.GetBranding(ctx, created.ID)
	if err != nil || unchangedBranding.DisplayName != "Integration" {
		t.Fatalf("suspended writes changed branding: %+v, error = %v", unchangedBranding, err)
	}
	eventsAfterWrites, err := store.FetchOutbox(ctx, 1000)
	if err != nil || len(eventsAfterWrites) != len(eventsBeforeWrites) {
		t.Fatalf("suspended writes changed outbox: before = %d, after = %d, error = %v", len(eventsBeforeWrites), len(eventsAfterWrites), err)
	}

	transitions := []string{domain.StatusActive, domain.StatusArchived}
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

func TestTenantWriteWaitsForConcurrentSuspensionIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := repository.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgres(): %v", err)
	}
	defer store.Close()
	created, err := store.CreateTenant(ctx, domain.CreateTenantInput{
		Name: "Concurrent Tenant", Slug: "concurrent-" + uuid.NewString(),
		Timezone: "Asia/Almaty", CurrencyCode: "KZT", LocaleCode: "ru-KZ", DefaultLanguage: "ru",
		PlanID: uuid.MustParse("10000000-0000-0000-0000-000000000002"),
	})
	if err != nil {
		t.Fatalf("CreateTenant(): %v", err)
	}
	if _, err := store.ChangeTenantState(ctx, created.ID, domain.StatusActive); err != nil {
		t.Fatalf("activate: %v", err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New(): %v", err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin(): %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var locked uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM md_tenant WHERE id = $1 FOR UPDATE`, created.ID).Scan(&locked); err != nil {
		t.Fatalf("lock tenant: %v", err)
	}
	started := make(chan struct{})
	writeResult := make(chan error, 1)
	go func() {
		close(started)
		_, err := store.UpdateSettings(ctx, domain.UpdateSettingsInput{
			TenantID: created.ID, Timezone: "UTC", CurrencyCode: "USD", LocaleCode: "en-US", DefaultLanguage: "en",
		})
		writeResult <- err
	}()
	<-started
	select {
	case err := <-writeResult:
		t.Fatalf("write completed while tenant row was locked: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if _, err := tx.Exec(ctx, `
		UPDATE md_tenant SET status_id = (SELECT id FROM di_tenant_status WHERE code = 'SUSPENDED')
		WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("suspend under lock: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit suspension: %v", err)
	}
	select {
	case err := <-writeResult:
		if !errors.Is(err, domain.ErrInvalidStateTransition) {
			t.Fatalf("write after suspension: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("write did not finish after suspension")
	}
	settings, err := store.GetSettings(ctx, created.ID)
	if err != nil || settings.Timezone != "Asia/Almaty" {
		t.Fatalf("concurrent write changed settings: %+v, error = %v", settings, err)
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
		FROM di_limit_type WHERE code = 'MAX_DOMAINS'
		ON CONFLICT DO NOTHING;

		INSERT INTO ln_plan_feature (plan_id, feature_id, enabled)
		SELECT '10000000-0000-0000-0000-000000000001', id, true
		FROM di_feature WHERE code IN ('CUSTOM_DOMAIN', 'CUSTOM_BRANDING')
		ON CONFLICT DO NOTHING;`)
	if err != nil {
		t.Fatalf("seed entitlements: %v", err)
	}
}

func countTenantEvents(events []domain.OutboxEvent, tenantID uuid.UUID, eventType string) int {
	count := 0
	for _, event := range events {
		if event.AggregateID == tenantID && event.EventType == eventType {
			count++
		}
	}
	return count
}
