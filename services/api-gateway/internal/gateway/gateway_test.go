package gateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"darksamus.com/ReserveHub/api-gateway/internal/config"
)

func TestPublicAuthRoutesStripCallerCredentials(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Service-Token") != "" || r.Header.Get("X-User-ID") != "" {
			t.Errorf("caller credentials reached Auth on %s", r.URL.Path)
		}
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("incorrect upstream request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	parsed, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	handler, closeGateway, err := New(config.Config{AuthServiceURL: parsed, AdminServiceURL: parsed, WorkspaceServiceURL: parsed}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeGateway()
	for _, path := range []string{"/v1/auth/invitations/accept", "/v1/auth/refresh", "/v1/auth/logout"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"refresh_token":"secret"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer caller")
		req.Header.Set("X-Service-Token", "caller")
		req.Header.Set("X-User-ID", "caller")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusNoContent {
			t.Errorf("%s returned %d: %s", path, res.Code, res.Body.String())
		}
	}
}

func TestAdminReissueRouteRequiresAdminAndProxies(t *testing.T) {
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/admin/tenant-provisioning/test-operation/reissue-invitation" {
			t.Errorf("unexpected admin proxy target: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Service-Token") != "" || r.Header.Get("Authorization") == "" {
			t.Error("admin proxy forwarded spoofed service token or removed admin bearer")
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	handler, closeGateway, err := New(config.Config{AuthServiceURL: parsed, AdminServiceURL: parsed, WorkspaceServiceURL: parsed, JWTVerifyKey: publicKey}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeGateway()
	makeToken := func(role string) string {
		now := time.Now().Unix()
		header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": tokenKeyID})
		claims, _ := json.Marshal(map[string]any{
			"sub": "test-actor", "role": role, "scope": "platform", "aud": []string{tokenAudience},
			"iss": tokenIssuer, "iat": now, "exp": now + 60,
		})
		encode := base64.RawURLEncoding.EncodeToString
		unsigned := encode(header) + "." + encode(claims)
		return unsigned + "." + encode(ed25519.Sign(privateKey, []byte(unsigned)))
	}
	call := func(role string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/admin/tenant-provisioning/test-operation/reissue-invitation", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+makeToken(role))
		req.Header.Set("X-Service-Token", "spoofed")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res.Code
	}
	if code := call("user"); code != http.StatusForbidden || calls != 0 {
		t.Fatalf("user reissue: code=%d calls=%d", code, calls)
	}
	if code := call("admin"); code != http.StatusAccepted || calls != 1 {
		t.Fatalf("admin reissue: code=%d calls=%d", code, calls)
	}
}

func TestTenantMemberRoutesEnforceBOLAAndRoleAtEdge(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	handler, closeGateway, err := New(config.Config{AuthServiceURL: parsed, AdminServiceURL: parsed, WorkspaceServiceURL: parsed, JWTVerifyKey: publicKey}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeGateway()

	tenantID := "11111111-1111-1111-1111-111111111111"
	otherTenantID := "22222222-2222-2222-2222-222222222222"
	makeTenantToken := func(tid, profile string) string {
		now := time.Now().Unix()
		header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": tokenKeyID})
		claims, _ := json.Marshal(map[string]any{
			"sub": "user-1", "role": "user", "scope": "tenant",
			"tenant_id": tid, "permission_profile": profile,
			"aud": []string{tokenAudience}, "iss": tokenIssuer,
			"iat": now, "exp": now + 60,
		})
		encode := base64.RawURLEncoding.EncodeToString
		unsigned := encode(header) + "." + encode(claims)
		return unsigned + "." + encode(ed25519.Sign(privateKey, []byte(unsigned)))
	}
	ownerTok := makeTenantToken(tenantID, "tenant_owner")
	viewerTok := makeTenantToken(tenantID, "tenant_viewer")

	do := func(method, path, token, body string) int {
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, reader)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res.Code
	}

	if code := do(http.MethodGet, "/v1/tenants/"+tenantID, viewerTok, ""); code != http.StatusOK {
		t.Fatalf("viewer read own tenant: got %d, want 200", code)
	}
	if code := do(http.MethodGet, "/v1/tenants/"+otherTenantID, ownerTok, ""); code != http.StatusForbidden {
		t.Fatalf("cross-tenant read (BOLA): got %d, want 403", code)
	}
	if code := do(http.MethodPatch, "/v1/tenants/"+tenantID, viewerTok, `{"name":"New"}`); code != http.StatusForbidden {
		t.Fatalf("viewer patch tenant: got %d, want 403", code)
	}
	if code := do(http.MethodPatch, "/v1/tenants/"+tenantID, ownerTok, `{"name":"New"}`); code != http.StatusOK {
		t.Fatalf("owner patch tenant: got %d, want 200", code)
	}
	if code := do(http.MethodPost, "/v1/auth/tenants/"+tenantID+"/invitations", viewerTok, `{"email":"a@b.c","permission_profile":"tenant_viewer"}`); code != http.StatusForbidden {
		t.Fatalf("viewer invite member: got %d, want 403", code)
	}
	if code := do(http.MethodPost, "/v1/auth/tenants/"+tenantID+"/invitations", ownerTok, `{"email":"a@b.c","permission_profile":"tenant_viewer"}`); code != http.StatusOK {
		t.Fatalf("owner invite member: got %d, want 200", code)
	}
}

func TestPublicAuthRateLimitReturns429(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	handler, closeGateway, err := New(config.Config{AuthServiceURL: parsed, AdminServiceURL: parsed, WorkspaceServiceURL: parsed}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeGateway()
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/invitations/accept", strings.NewReader(`{"token":"invalid"}`))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if i == 10 && res.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 after invite bucket exhaustion, got %d", res.Code)
		}
	}
}

func TestProviderRoutesEnforceRolesAndStripInternalCredentials(t *testing.T) {
	var adminCalls, workspaceCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminCalls++
		if r.Header.Get("X-Service-Token") != "" || r.Header.Get("X-User-ID") != "" || r.Header.Get("Authorization") == "" {
			t.Error("admin proxy forwarded a forged internal credential or removed bearer token")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	workspace := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workspaceCalls++
		if r.Header.Get("X-Service-Token") != "" || r.Header.Get("X-User-ID") != "" || r.Header.Get("Authorization") == "" {
			t.Error("workspace proxy forwarded a forged internal credential or removed bearer token")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer workspace.Close()
	parsedWorkspace, _ := url.Parse(workspace.URL)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	handler, closeGateway, err := New(config.Config{AuthServiceURL: parsed, AdminServiceURL: parsed, WorkspaceServiceURL: parsedWorkspace, JWTVerifyKey: publicKey}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeGateway()
	tenantID := "11111111-1111-1111-1111-111111111111"
	otherID := "22222222-2222-2222-2222-222222222222"
	token := func(scope, role, profile, tenant string) string {
		now := time.Now().Unix()
		header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": tokenKeyID})
		claims, _ := json.Marshal(map[string]any{"sub": "test-actor", "role": role, "scope": scope, "permission_profile": profile, "tenant_id": tenant, "aud": []string{tokenAudience}, "iss": tokenIssuer, "iat": now, "exp": now + 60})
		encode := base64.RawURLEncoding.EncodeToString
		unsigned := encode(header) + "." + encode(claims)
		return unsigned + "." + encode(ed25519.Sign(privateKey, []byte(unsigned)))
	}
	do := func(method, path, bearer string) int {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+bearer)
		req.Header.Set("X-Service-Token", "forged")
		req.Header.Set("X-User-ID", "forged")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res.Code
	}
	adminPath := "/v1/admin/tenants/" + tenantID + "/providers"
	tenantPath := "/v1/tenants/" + tenantID + "/providers"
	admin := token("platform", "admin", "", "")
	owner := token("tenant", "user", "tenant_owner", tenantID)
	viewer := token("tenant", "user", "tenant_viewer", tenantID)
	if code := do("GET", adminPath+"?page_size=5", admin); code != 200 {
		t.Fatalf("admin list: %d", code)
	}
	if code := do("GET", tenantPath+"?page_size=5", owner); code != 200 {
		t.Fatalf("owner list: %d", code)
	}
	if code := do("GET", tenantPath, viewer); code != 403 {
		t.Fatalf("viewer list: %d", code)
	}
	if code := do("GET", "/v1/tenants/"+otherID+"/providers", owner); code != 403 {
		t.Fatalf("cross tenant: %d", code)
	}
	if code := do("GET", adminPath+"?status=ACTIVE", admin); code != 400 {
		t.Fatalf("unsupported query: %d", code)
	}
	if adminCalls != 1 || workspaceCalls != 1 {
		t.Fatalf("unexpected upstream calls: admin=%d workspace=%d", adminCalls, workspaceCalls)
	}
}

func TestOpenAPISpecAndSwaggerUI(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	handler, closeGateway, err := New(config.Config{AuthServiceURL: parsed, AdminServiceURL: parsed, WorkspaceServiceURL: parsed, SwaggerEnabled: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeGateway()

	specReq := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	specRes := httptest.NewRecorder()
	handler.ServeHTTP(specRes, specReq)
	if specRes.Code != http.StatusOK {
		t.Fatalf("GET /openapi.json returned %d", specRes.Code)
	}
	var spec map[string]any
	if err := json.Unmarshal(specRes.Body.Bytes(), &spec); err != nil || spec["openapi"] != "3.0.3" {
		t.Fatalf("invalid openapi.json payload: %v", err)
	}

	uiReq := httptest.NewRequest(http.MethodGet, "/swagger", nil)
	uiRes := httptest.NewRecorder()
	handler.ServeHTTP(uiRes, uiReq)
	if uiRes.Code != http.StatusOK || !strings.Contains(uiRes.Body.String(), "SwaggerUIBundle") {
		t.Fatalf("GET /swagger returned %d", uiRes.Code)
	}
}

func TestSwaggerDisabledByDefault(t *testing.T) {
	parsed, _ := url.Parse("http://127.0.0.1:1")
	handler, closeGateway, err := New(config.Config{AuthServiceURL: parsed, AdminServiceURL: parsed, WorkspaceServiceURL: parsed}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeGateway()
	for _, path := range []string{"/openapi.json", "/swagger", "/swagger/"} {
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != http.StatusNotFound {
			t.Fatalf("GET %s returned %d, want 404", path, res.Code)
		}
	}
}
