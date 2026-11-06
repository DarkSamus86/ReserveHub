package authn

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func signedTestToken(t *testing.T, key ed25519.PrivateKey, role Role, scope string) string {
	t.Helper()
	return signedTenantTestToken(t, key, role, scope, "", "")
}

func signedTenantTestToken(t *testing.T, key ed25519.PrivateKey, role Role, scope, tenantID, profile string) string {
	t.Helper()
	now := time.Now().UTC()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": "auth-ed25519-v1"})
	claims, _ := json.Marshal(Claims{
		Subject:           "user-1",
		Role:              role,
		Scope:             scope,
		TenantID:          tenantID,
		PermissionProfile: profile,
		Audience:          []string{"reservehub-tenant"},
		Issuer:            "reservehub-auth",
		IssuedAt:          now.Unix(),
		Expires:           now.Add(time.Minute).Unix(),
	})
	encode := base64.RawURLEncoding.EncodeToString
	unsigned := encode(header) + "." + encode(claims)
	return unsigned + "." + encode(ed25519.Sign(key, []byte(unsigned)))
}

func TestUnaryServerInterceptorRolePolicy(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	interceptor := UnaryServerInterceptor(publicKey)
	called := false
	handler := func(context.Context, any) (any, error) { called = true; return "ok", nil }
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+signedTestToken(t, privateKey, RoleUser, "platform")))
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/GetTenant"}, handler)
	if status.Code(err) != codes.PermissionDenied || called {
		t.Fatalf("unassigned user read request: called=%v err=%v", called, err)
	}
	adminCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+signedTestToken(t, privateKey, RoleAdmin, "platform")))
	_, err = interceptor(adminCtx, nil, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/CreateTenant"}, handler)
	if err != nil {
		t.Fatalf("platform admin request: %v", err)
	}
	called = false
	tenantAdminCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+signedTestToken(t, privateKey, RoleAdmin, "tenant")))
	_, err = interceptor(tenantAdminCtx, nil, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/CreateTenant"}, handler)
	if status.Code(err) != codes.PermissionDenied || called {
		t.Fatalf("tenant scoped admin request: called=%v err=%v", called, err)
	}
}

func TestUnaryServerInterceptorTenantMemberACLAndBOLA(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	interceptor := UnaryServerInterceptor(publicKey)
	tenantID := "11111111-1111-1111-1111-111111111111"
	otherTenantID := "22222222-2222-2222-2222-222222222222"
	ownerCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer "+signedTenantTestToken(t, privateKey, RoleUser, "tenant", tenantID, ProfileTenantOwner),
	))
	viewerCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer "+signedTenantTestToken(t, privateKey, RoleUser, "tenant", tenantID, ProfileTenantViewer),
	))
	handler := func(ctx context.Context, _ any) (any, error) {
		if _, ok := ClaimsFromContext(ctx); !ok {
			t.Fatal("expected claims in handler context")
		}
		return "ok", nil
	}

	// Viewer and Owner can read own tenant
	for _, ctx := range []context.Context{ownerCtx, viewerCtx} {
		_, err := interceptor(ctx, &tenantv1.GetTenantRequest{TenantId: tenantID}, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/GetTenant"}, handler)
		if err != nil {
			t.Fatalf("expected own tenant read to succeed, got %v", err)
		}
	}

	// Cross-tenant read blocked (BOLA)
	_, err = interceptor(ownerCtx, &tenantv1.GetTenantRequest{TenantId: otherTenantID}, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/GetTenant"}, handler)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected cross-tenant read PermissionDenied, got %v", err)
	}

	// Owner can update own tenant; Viewer cannot
	_, err = interceptor(ownerCtx, &tenantv1.UpdateTenantRequest{TenantId: tenantID}, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/UpdateTenant"}, handler)
	if err != nil {
		t.Fatalf("expected owner UpdateTenant to succeed, got %v", err)
	}
	_, err = interceptor(viewerCtx, &tenantv1.UpdateTenantRequest{TenantId: tenantID}, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/UpdateTenant"}, handler)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected viewer UpdateTenant PermissionDenied, got %v", err)
	}

	// Owner cannot call platform-only lifecycle RPCs
	_, err = interceptor(ownerCtx, &tenantv1.ChangeTenantStateRequest{TenantId: tenantID}, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/ActivateTenant"}, handler)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected owner ActivateTenant PermissionDenied, got %v", err)
	}
}

func TestUnaryServerInterceptorRejectsMissingToken(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	interceptor := UnaryServerInterceptor(publicKey)
	_, err = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/GetTenant"}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %s, want Unauthenticated", status.Code(err))
	}
}

func TestCreateTenantRequiresServiceIdentity(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	interceptor := UnaryServerInterceptor(publicKey, []byte("0123456789abcdef0123456789abcdef"))
	called := false
	handler := func(context.Context, any) (any, error) { called = true; return nil, nil }
	method := &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/CreateTenant"}
	jwt := signedTestToken(t, privateKey, RoleAdmin, "platform")
	withoutService := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+jwt))
	_, err = interceptor(withoutService, nil, method, handler)
	if status.Code(err) != codes.Unauthenticated || called {
		t.Fatalf("missing service identity: called=%v err=%v", called, err)
	}
	withService := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+jwt, "x-service-token", "0123456789abcdef0123456789abcdef"))
	_, err = interceptor(withService, nil, method, handler)
	if err != nil || !called {
		t.Fatalf("authorized service: called=%v err=%v", called, err)
	}
}
