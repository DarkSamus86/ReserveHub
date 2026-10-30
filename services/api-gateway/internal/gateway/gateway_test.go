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
	handler, closeGateway, err := New(config.Config{AuthServiceURL: parsed, AdminServiceURL: parsed}, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
	handler, closeGateway, err := New(config.Config{AuthServiceURL: parsed, AdminServiceURL: parsed, JWTVerifyKey: publicKey}, slog.New(slog.NewTextHandler(io.Discard, nil)))
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

func TestPublicAuthRateLimitReturns429(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	handler, closeGateway, err := New(config.Config{AuthServiceURL: parsed, AdminServiceURL: parsed}, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
