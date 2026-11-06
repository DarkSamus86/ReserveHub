package httpapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	providerv1 "darksamus.com/ReserveHub/provider-service/gen/provider/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeProviderClient struct {
	providerv1.ProviderServiceClient
	calls int
	t     *testing.T
}

func (f *fakeProviderClient) CreateProvider(ctx context.Context, req *providerv1.CreateProviderRequest, _ ...grpc.CallOption) (*providerv1.CreateProviderResponse, error) {
	f.calls++
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok || len(md.Get("authorization")) != 1 || len(md.Get("x-service-token")) != 1 || md.Get("x-service-token")[0] != "admin-secret" {
		f.t.Error("missing Provider credentials")
	}
	if req.GetTenantId() == "" || req.GetOperationId() == "" {
		f.t.Error("missing Provider identifiers")
	}
	return &providerv1.CreateProviderResponse{Provider: &providerv1.Provider{Id: uuid.NewString(), TenantId: req.GetTenantId(), Name: req.GetName(), City: req.GetCity(), Status: "ACTIVE"}, Replayed: f.calls > 1}, nil
}

func TestAdminProviderCreatePreservesReplayStatus(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeProviderClient{t: t}
	h := NewWithProvider(nil, nil, pub, client, "admin-secret")
	token := func(role string) string {
		now := time.Now().Unix()
		header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": "auth-ed25519-v1"})
		claims, _ := json.Marshal(map[string]any{"sub": uuid.NewString(), "scope": "platform", "role": role, "aud": []string{"reservehub-api", "reservehub-tenant"}, "iss": "reservehub-auth", "iat": now, "exp": now + 60})
		enc := base64.RawURLEncoding.EncodeToString
		unsigned := enc(header) + "." + enc(claims)
		return unsigned + "." + enc(ed25519.Sign(priv, []byte(unsigned)))
	}
	path := "/v1/admin/tenants/" + uuid.NewString() + "/providers"
	key := uuid.NewString()
	call := func(role string) int {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"name":"Kinopark Astana","city":"Astana"}`))
		req.Header.Set("Authorization", "Bearer "+token(role))
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		return res.Code
	}
	if got := call("user"); got != 403 {
		t.Fatalf("non-admin status=%d", got)
	}
	if got := call("admin"); got != 201 {
		t.Fatalf("create status=%d", got)
	}
	if got := call("admin"); got != 200 {
		t.Fatalf("replay status=%d", got)
	}
	if client.calls != 2 {
		t.Fatalf("Provider called %d times", client.calls)
	}
}
