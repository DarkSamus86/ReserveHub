package provision_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"darksamus.com/ReserveHub/admin-service/internal/httpapi"
	"darksamus.com/ReserveHub/admin-service/internal/provision"
	"darksamus.com/ReserveHub/admin-service/internal/repository"
	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type qaTenant struct {
	tenantv1.UnimplementedTenantServiceServer
}

func (qaTenant) CreateTenant(_ context.Context, request *tenantv1.CreateTenantRequest) (*tenantv1.Tenant, error) {
	return &tenantv1.Tenant{Id: uuid.NewSHA1(uuid.NameSpaceOID, []byte(request.GetOperationId())).String(), Status: "PENDING"}, nil
}

type qaAuthState struct {
	mu       sync.Mutex
	inviteID string
	status   string
	expires  *time.Time
	down     bool
}

func (s *qaAuthState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Service-Token") != "0123456789abcdef0123456789abcdef" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/internal/v1/tenant-memberships/provision-owner" {
		s.status = "PENDING_DELIVERY"
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "pending_delivery", "invitation_id": s.inviteID})
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/internal/v1/tenant-invitations/"+s.inviteID {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"invitation_id": s.inviteID, "status": s.status, "expires_at": s.expires})
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/internal/v1/tenant-invitations/"+s.inviteID+"/reissue" {
		if s.status != "SENT" || s.expires == nil || s.expires.After(time.Now()) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		s.status, s.expires = "PENDING_DELIVERY", nil
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func qaToken(t *testing.T, key ed25519.PrivateKey, actor uuid.UUID, role string) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": "auth-ed25519-v1"})
	now := time.Now().UTC()
	claims, _ := json.Marshal(map[string]any{
		"sub": actor.String(), "role": role, "scope": "platform",
		"aud": []string{"reservehub-api", "reservehub-tenant"}, "iss": "reservehub-auth",
		"iat": now.Unix(), "exp": now.Add(time.Minute).Unix(),
	})
	encode := base64.RawURLEncoding.EncodeToString
	unsigned := encode(header) + "." + encode(claims)
	return unsigned + "." + encode(ed25519.Sign(key, []byte(unsigned)))
}

func TestQALiveInvitationStatusReissueAndAudit(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := repository.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	listener := bufconn.Listen(1 << 20)
	grpcServer := grpc.NewServer()
	tenantv1.RegisterTenantServiceServer(grpcServer, qaTenant{})
	go grpcServer.Serve(listener)
	defer grpcServer.Stop()
	conn, err := grpc.NewClient("passthrough:///qa-tenant", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	state := &qaAuthState{inviteID: uuid.NewString()}
	authServer := httptest.NewServer(state)
	defer authServer.Close()
	service := provision.New(store, tenantv1.NewTenantServiceClient(conn), authServer.URL, "0123456789abcdef0123456789abcdef")
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(store, service, publicKey)
	actor := uuid.New()
	adminToken := qaToken(t, privateKey, actor, "admin")
	userToken := qaToken(t, privateKey, actor, "user")
	opID := uuid.New()
	requestBody := fmt.Sprintf(`{"name":"QA Company","slug":"qa-%s","timezone":"Asia/Almaty","currency_code":"KZT","locale_code":"ru-KZ","default_language":"ru","plan_id":"10000000-0000-0000-0000-000000000001","owner_email":"qa@example.com"}`, uuid.NewString())
	request := httptest.NewRequest(http.MethodPost, "/v1/admin/tenants", strings.NewReader(requestBody))
	request.Header.Set("Authorization", "Bearer "+adminToken)
	request.Header.Set("Idempotency-Key", opID.String())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("provision returned %d: %s", response.Code, response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), []byte("qa@example.com")) {
		t.Fatal("admin response exposed owner email")
	}
	get := func(bearer string) (int, provision.Result) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/tenant-provisioning/"+opID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+bearer)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		var result provision.Result
		_ = json.Unmarshal(res.Body.Bytes(), &result)
		return res.Code, result
	}
	if code, _ := get(userToken); code != http.StatusForbidden {
		t.Fatalf("platform user read Admin operation: %d", code)
	}
	for _, status := range []string{"PENDING_DELIVERY", "READY_TO_SEND", "SENT", "ACCEPTED"} {
		state.mu.Lock()
		state.status = status
		expiry := time.Now().Add(time.Hour).UTC()
		state.expires = &expiry
		state.mu.Unlock()
		code, result := get(adminToken)
		if code != http.StatusOK || result.InviteStatus != strings.ToLower(status) || result.InviteStatusUnavailable || result.InviteExpiresAt == nil {
			t.Fatalf("live status %s: code=%d result=%+v", status, code, result)
		}
	}
	state.mu.Lock()
	state.down = true
	state.mu.Unlock()
	code, unavailable := get(adminToken)
	if code != http.StatusOK || !unavailable.InviteStatusUnavailable || unavailable.InviteStatus != "pending_delivery" {
		t.Fatalf("Auth outage fallback: code=%d result=%+v", code, unavailable)
	}
	state.mu.Lock()
	state.down = false
	state.status = "SENT"
	expired := time.Now().Add(-time.Minute).UTC()
	state.expires = &expired
	state.mu.Unlock()
	reissue := func(bearer string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/admin/tenant-provisioning/"+opID.String()+"/reissue-invitation", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+bearer)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res.Code
	}
	if code := reissue(userToken); code != http.StatusForbidden {
		t.Fatalf("platform user reissued invitation: %d", code)
	}
	if code := reissue(adminToken); code != http.StatusAccepted {
		t.Fatalf("admin reissue returned %d", code)
	}
	if code := reissue(adminToken); code != http.StatusConflict {
		t.Fatalf("immediate repeated reissue returned %d", code)
	}
	if code, result := get(adminToken); code != http.StatusOK || result.InviteStatus != "pending_delivery" {
		t.Fatalf("status after reissue: code=%d result=%+v", code, result)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `SELECT payload::text FROM admin_service.sys_outbox_event WHERE aggregate_id=$1 AND event_type='admin.invitation.reissue_requested.v1' ORDER BY created_at`, opID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	results := map[string]bool{}
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload, "qa@example.com") || strings.Contains(payload, "token") {
			t.Fatalf("unsafe reissue audit payload: %s", payload)
		}
		var value struct {
			ActorID string `json:"actor_id"`
			Result  string `json:"result"`
		}
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			t.Fatal(err)
		}
		if value.ActorID != actor.String() {
			t.Fatalf("audit actor mismatch: %s", payload)
		}
		results[value.Result] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !results["accepted"] || !results["rejected"] {
		t.Fatalf("missing reissue audit results: %+v", results)
	}
}
