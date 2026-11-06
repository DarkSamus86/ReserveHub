package httpapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	providerv1 "darksamus.com/ReserveHub/provider-service/gen/provider/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeProvider struct {
	providerv1.ProviderServiceClient
	calls int
	t     *testing.T
}

func (f *fakeProvider) ListProviders(ctx context.Context, req *providerv1.ListProvidersRequest, _ ...grpc.CallOption) (*providerv1.ListProvidersResponse, error) {
	f.calls++
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok || len(md.Get("x-service-token")) != 1 || md.Get("x-service-token")[0] != "workspace-secret" || len(md.Get("authorization")) != 1 {
		f.t.Error("missing Provider credentials")
	}
	return &providerv1.ListProvidersResponse{Providers: []*providerv1.Provider{{Id: uuid.NewString(), TenantId: req.GetTenantId(), Name: "Kinopark Astana", City: "Astana", Status: "ACTIVE"}}}, nil
}

func TestWorkspaceProviderListRequiresOwnerAndTenantMatch(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	other := uuid.NewString()
	client := &fakeProvider{t: t}
	h := Handler{Provider: client, ProviderToken: "workspace-secret", PublicKey: pub}.Routes()
	token := func(profile string) string {
		now := time.Now().Unix()
		header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": "auth-ed25519-v1"})
		claims, _ := json.Marshal(map[string]any{"sub": uuid.NewString(), "scope": "tenant", "role": "user", "tenant_id": id, "permission_profile": profile, "aud": []string{"reservehub-api", "reservehub-tenant"}, "iss": "reservehub-auth", "iat": now, "exp": now + 60})
		enc := base64.RawURLEncoding.EncodeToString
		unsigned := enc(header) + "." + enc(claims)
		return unsigned + "." + enc(ed25519.Sign(priv, []byte(unsigned)))
	}
	call := func(path, bearer string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+bearer)
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		return res.Code
	}
	if code := call("/v1/tenants/"+id+"/providers", token("tenant_viewer")); code != 403 {
		t.Fatalf("viewer status=%d", code)
	}
	if code := call("/v1/tenants/"+other+"/providers", token("tenant_owner")); code != 403 {
		t.Fatalf("cross-tenant status=%d", code)
	}
	if code := call("/v1/tenants/"+id+"/providers?page_size=5", token("tenant_owner")); code != 200 {
		t.Fatalf("owner status=%d", code)
	}
	if client.calls != 1 {
		t.Fatalf("Provider called %d times", client.calls)
	}
}
