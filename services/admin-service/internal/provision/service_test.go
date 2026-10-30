package provision

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"darksamus.com/ReserveHub/admin-service/internal/repository"
	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeTenant struct {
	tenantv1.UnimplementedTenantServiceServer
	mu       sync.Mutex
	calls    int
	failNext bool
}

func (f *fakeTenant) CreateTenant(_ context.Context, req *tenantv1.CreateTenantRequest) (*tenantv1.Tenant, error) {
	f.mu.Lock()
	f.calls++
	if f.failNext {
		f.failNext = false
		f.mu.Unlock()
		return nil, status.Error(codes.Unavailable, "test tenant unavailable")
	}
	f.mu.Unlock()
	return &tenantv1.Tenant{Id: uuid.NewSHA1(uuid.NameSpaceOID, []byte(req.GetOperationId())).String(), Slug: req.GetSlug(), Status: "PENDING"}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestProvisionCreateRetryAndAuthFailure(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := repository.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	listener := bufconn.Listen(1 << 20)
	grpcServer := grpc.NewServer()
	fake := &fakeTenant{}
	tenantv1.RegisterTenantServiceServer(grpcServer, fake)
	go grpcServer.Serve(listener)
	defer grpcServer.Stop()
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithInsecure(), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	authCalls := 0
	failNext := false
	failStatus := false
	inviteStatuses := make(map[string]string)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("X-Service-Token") != "0123456789abcdef0123456789abcdef" {
			t.Errorf("missing service token")
		}
		if req.Method == http.MethodGet {
			if failStatus {
				return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
			}
			id := strings.TrimPrefix(req.URL.Path, "/internal/v1/tenant-invitations/")
			body, _ := json.Marshal(map[string]string{"invitation_id": id, "status": inviteStatuses[id]})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
		}
		if strings.HasSuffix(req.URL.Path, "/reissue") {
			return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
		}
		authCalls++
		code := http.StatusCreated
		if failNext {
			code = http.StatusServiceUnavailable
			failNext = false
		}
		var provisionRequest struct {
			OperationID string `json:"operation_id"`
		}
		if err := json.NewDecoder(req.Body).Decode(&provisionRequest); err != nil {
			t.Error(err)
		}
		inviteID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(provisionRequest.OperationID+":invite")).String()
		inviteStatuses[inviteID] = "pending_delivery"
		body, _ := json.Marshal(map[string]string{"status": "pending_delivery", "invitation_id": inviteID})
		return &http.Response{StatusCode: code, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})}
	s := &Service{store: store, tenant: tenantv1.NewTenantServiceClient(conn), authURL: "http://auth.invalid", client: client, serviceToken: "0123456789abcdef0123456789abcdef"}
	actor := uuid.New()
	input := Input{Name: "Company", Slug: "company-" + uuid.NewString(), Timezone: "Asia/Almaty", CurrencyCode: "KZT", LocaleCode: "ru-KZ", DefaultLanguage: "ru", PlanID: "10000000-0000-0000-0000-000000000001", OwnerEmail: "owner@example.com"}
	encoded, _ := json.Marshal(input)
	hash := sha256.Sum256(encoded)
	opID := uuid.New()
	firstOperationID := opID
	first, code, err := s.Run(ctx, opID, actor, hash[:], encoded, input, "admin-token")
	if err != nil || code != 201 || first.Status != "completed" || first.InviteStatus != "pending_delivery" {
		t.Fatalf("first: %+v %d %v", first, code, err)
	}
	replayed, code, err := s.Run(ctx, opID, actor, hash[:], encoded, input, "admin-token")
	if err != nil || code != 200 || replayed.TenantID != first.TenantID || authCalls != 1 || fake.calls != 1 {
		t.Fatalf("replay: %+v %d %v, auth=%d tenant=%d", replayed, code, err, authCalls, fake.calls)
	}
	firstInviteID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(firstOperationID.String()+":invite")).String()
	inviteStatuses[firstInviteID] = "SENT"
	live, err := s.Get(ctx, firstOperationID)
	if err != nil || live.InviteStatus != "sent" || live.InviteStatusUnavailable {
		t.Fatalf("live invitation status: %+v %v", live, err)
	}
	failStatus = true
	stale, err := s.Get(ctx, firstOperationID)
	if err != nil || stale.Status != "completed" || stale.InviteStatus != "pending_delivery" || !stale.InviteStatusUnavailable {
		t.Fatalf("saved status while Auth unavailable: %+v %v", stale, err)
	}
	failStatus = false
	if err := s.ReissueInvitation(ctx, firstOperationID, actor); err != nil {
		t.Fatalf("reissue invitation: %v", err)
	}
	changed := input
	changed.Name = "Different"
	changedJSON, _ := json.Marshal(changed)
	changedHash := sha256.Sum256(changedJSON)
	_, code, err = s.Run(ctx, opID, actor, changedHash[:], changedJSON, changed, "admin-token")
	if err != nil || code != 409 {
		t.Fatalf("changed request: code=%d err=%v", code, err)
	}

	failNext = true
	opID = uuid.New()
	failed, code, err := s.Run(ctx, opID, actor, hash[:], encoded, input, "admin-token")
	if err != nil || code != 503 || failed.Status != "retryable_failure" || failed.TenantID == "" {
		t.Fatalf("auth failure: %+v %d %v", failed, code, err)
	}
	retried, code, err := s.Run(ctx, opID, actor, hash[:], encoded, input, "admin-token")
	if err != nil || code != 201 || retried.Status != "completed" || retried.TenantID != failed.TenantID || fake.calls != 2 {
		t.Fatalf("retry: %+v %d %v tenant=%d", retried, code, err, fake.calls)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	assertAudit := func(operation uuid.UUID, expectedStatuses []string, tenantPresent bool) {
		t.Helper()
		rows, err := pool.Query(ctx, `
			SELECT payload, aggregate_id, event_type FROM admin_service.sys_outbox_event
			WHERE aggregate_id=$1 AND event_type='admin.tenant_provision.status_changed'
			ORDER BY created_at, id`, operation)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		statuses := make(map[string]bool)
		count := 0
		for rows.Next() {
			var payload []byte
			var aggregate uuid.UUID
			var eventType string
			if err := rows.Scan(&payload, &aggregate, &eventType); err != nil {
				t.Fatal(err)
			}
			var event struct {
				OperationID string  `json:"operation_id"`
				ActorID     string  `json:"actor_id"`
				TenantID    *string `json:"tenant_id"`
				Status      string  `json:"status"`
			}
			if err := json.Unmarshal(payload, &event); err != nil {
				t.Fatal(err)
			}
			if aggregate != operation || eventType != "admin.tenant_provision.status_changed" ||
				event.OperationID != operation.String() || event.ActorID != actor.String() ||
				(event.TenantID != nil) != tenantPresent || bytes.Contains(payload, []byte("owner_email")) ||
				bytes.Contains(payload, []byte("tenant_request")) || bytes.Contains(payload, []byte("owner@example.com")) {
				t.Fatalf("unsafe or incorrect audit event: %s", payload)
			}
			statuses[event.Status] = true
			count++
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if count != len(expectedStatuses) {
			t.Fatalf("audit count for %s = %d, want %d", operation, count, len(expectedStatuses))
		}
		for _, expected := range expectedStatuses {
			if !statuses[expected] {
				t.Fatalf("missing audit status %s for %s", expected, operation)
			}
		}
	}
	assertAudit(firstOperationID, []string{"completed"}, true)
	assertAudit(opID, []string{"retryable_failure", "completed"}, true)

	fake.mu.Lock()
	fake.failNext = true
	fake.mu.Unlock()
	failedTenantOp := uuid.New()
	failedTenant, code, err := s.Run(ctx, failedTenantOp, actor, hash[:], encoded, input, "admin-token")
	if err != nil || code != 503 || failedTenant.Status != "retryable_failure" || failedTenant.TenantID != "" {
		t.Fatalf("tenant failure: %+v %d %v", failedTenant, code, err)
	}
	assertAudit(failedTenantOp, []string{"retryable_failure"}, false)
}
