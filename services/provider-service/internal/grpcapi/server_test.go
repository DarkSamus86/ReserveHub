package grpcapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net"
	"sync/atomic"
	"testing"
	"time"

	providerv1 "darksamus.com/ReserveHub/provider-service/gen/provider/v1"
	"darksamus.com/ReserveHub/provider-service/internal/application"
	"darksamus.com/ReserveHub/provider-service/internal/domain"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeStore struct {
	item    domain.Provider
	creates int
}

func (s *fakeStore) Create(_ context.Context, _, tenant, _ uuid.UUID, name, city string, _ [32]byte) (domain.Provider, bool, error) {
	s.creates++
	s.item = domain.Provider{ID: uuid.New(), TenantID: tenant, Name: name, City: city, Status: "ACTIVE", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	return s.item, false, nil
}
func (s *fakeStore) List(_ context.Context, tenant uuid.UUID, _ int, _ time.Time, _ uuid.UUID) ([]domain.Provider, error) {
	if s.item.TenantID != tenant {
		return []domain.Provider{}, nil
	}
	return []domain.Provider{s.item}, nil
}
func (s *fakeStore) Get(_ context.Context, tenant, id uuid.UUID) (domain.Provider, error) {
	if s.item.TenantID != tenant || s.item.ID != id {
		return domain.Provider{}, domain.ErrNotFound
	}
	return s.item, nil
}
func (s *fakeStore) UpdateProfile(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *string, *string) (domain.Provider, error) {
	return s.item, nil
}
func (s *fakeStore) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (domain.Provider, error) {
	return s.item, nil
}

func TestProviderRPCRequiresWorkloadAndActorAndTenant(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tenantID := uuid.New()
	otherID := uuid.New()
	store := &fakeStore{}
	var state atomic.Value
	state.Store("ACTIVE")
	service := application.Service{Store: store, Tenant: func(_ context.Context, id uuid.UUID, bearer string) (string, error) {
		if bearer == "" {
			t.Fatal("missing actor token")
		}
		if id == otherID {
			return "ACTIVE", nil
		}
		return state.Load().(string), nil
	}}
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.UnaryInterceptor(UnaryAuth(pub, "admin-secret-32-bytes-aaaaaaaaaaaa", "workspace-secret-32-bytes-bbbbbbbb")))
	providerv1.RegisterProviderServiceServer(server, Server{Service: service})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := providerv1.NewProviderServiceClient(conn)
	token := func(scope, role, profile string, tid uuid.UUID) string {
		now := time.Now().Unix()
		header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": "auth-ed25519-v1"})
		claims, _ := json.Marshal(map[string]any{"sub": uuid.NewString(), "scope": scope, "role": role, "tenant_id": tid.String(), "permission_profile": profile, "aud": []string{"reservehub-api", "reservehub-tenant"}, "iss": "reservehub-auth", "iat": now, "exp": now + 60})
		encode := base64.RawURLEncoding.EncodeToString
		unsigned := encode(header) + "." + encode(claims)
		return unsigned + "." + encode(ed25519.Sign(priv, []byte(unsigned)))
	}
	admin := token("platform", "admin", "", uuid.Nil)
	owner := token("tenant", "user", "tenant_owner", tenantID)
	viewer := token("tenant", "user", "tenant_viewer", tenantID)
	withAuth := func(secret, jwt string) context.Context {
		return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-service-token", secret, "authorization", "Bearer "+jwt))
	}
	create := &providerv1.CreateProviderRequest{TenantId: tenantID.String(), OperationId: uuid.NewString(), Name: "Astana", City: "Astana"}
	if _, err = client.CreateProvider(context.Background(), create); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing credentials: %v", err)
	}
	if _, err = client.CreateProvider(withAuth("workspace-secret-32-bytes-bbbbbbbb", admin), create); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("admin via workspace credential: %v", err)
	}
	if _, err = client.CreateProvider(withAuth("admin-secret-32-bytes-aaaaaaaaaaaa", owner), create); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("owner via admin credential: %v", err)
	}
	created, err := client.CreateProvider(withAuth("admin-secret-32-bytes-aaaaaaaaaaaa", admin), create)
	if err != nil || created.Replayed || created.Provider.GetTenantId() != tenantID.String() {
		t.Fatalf("admin create: %v %+v", err, created)
	}
	if _, err = client.ListProviders(withAuth("workspace-secret-32-bytes-bbbbbbbb", viewer), &providerv1.ListProvidersRequest{TenantId: tenantID.String()}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("viewer list: %v", err)
	}
	if _, err = client.ListProviders(withAuth("workspace-secret-32-bytes-bbbbbbbb", owner), &providerv1.ListProvidersRequest{TenantId: otherID.String()}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-tenant list: %v", err)
	}
	got, err := client.GetProvider(withAuth("workspace-secret-32-bytes-bbbbbbbb", owner), &providerv1.GetProviderRequest{TenantId: tenantID.String(), ProviderId: created.Provider.GetId()})
	if err != nil || got.GetId() != created.Provider.GetId() {
		t.Fatalf("owner get: %v %+v", err, got)
	}
	if _, err = client.GetProvider(withAuth("admin-secret-32-bytes-aaaaaaaaaaaa", admin), &providerv1.GetProviderRequest{TenantId: otherID.String(), ProviderId: created.Provider.GetId()}); status.Code(err) != codes.NotFound {
		t.Fatalf("provider BOLA: %v", err)
	}
	state.Store("SUSPENDED")
	if _, err = client.ChangeProviderStatus(withAuth("admin-secret-32-bytes-aaaaaaaaaaaa", admin), &providerv1.ChangeProviderStatusRequest{TenantId: tenantID.String(), ProviderId: created.Provider.GetId(), Status: "SUSPENDED"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("inactive tenant write: %v", err)
	}
	if store.creates != 1 {
		t.Fatalf("create count %d", store.creates)
	}
}

func TestTenantDependencyErrorsAreSanitized(t *testing.T) {
	err := mapError(status.Error(codes.FailedPrecondition, "internal tenant details"))
	if status.Code(err) != codes.PermissionDenied || status.Convert(err).Message() != "access denied" {
		t.Fatalf("unsafe dependency error: %v", err)
	}
}
