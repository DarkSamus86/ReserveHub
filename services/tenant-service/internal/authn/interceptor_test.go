package authn

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func signedTestToken(t *testing.T, key ed25519.PrivateKey, role Role, scope string) string {
	t.Helper()
	now := time.Now().UTC()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": verificationKey})
	claims, _ := json.Marshal(Claims{Subject: "user-1", Role: role, Scope: scope, Audience: []string{tenantAudience}, Issuer: issuer, IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix()})
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
