package authn

import (
	"context"
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func testToken(role Role) string {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Now().UTC()
	encode := base64.RawURLEncoding.EncodeToString
	header := encode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := encode([]byte(`{"sub":"user-1","role":"` + string(role) + `","iss":"reservehub-auth","iat":` + strconv.FormatInt(now.Unix(), 10) + `,"exp":` + strconv.FormatInt(now.Add(time.Minute).Unix(), 10) + `}`))
	unsigned := header + "." + claims
	return unsigned + "." + encode(sign(secret, unsigned))
}

func TestUnaryServerInterceptorRolePolicy(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	interceptor := UnaryServerInterceptor(secret)
	called := false
	handler := func(context.Context, any) (any, error) { called = true; return "ok", nil }

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+testToken(RoleUser)))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/GetTenant"}, handler)
	if status.Code(err) != codes.PermissionDenied || called {
		t.Fatalf("unassigned user read request: called=%v err=%v", called, err)
	}

	adminCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+testToken(RoleAdmin)))
	_, err = interceptor(adminCtx, nil, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/CreateTenant"}, handler)
	if err != nil {
		t.Fatalf("admin write request: %v", err)
	}
}

func TestUnaryServerInterceptorRejectsMissingToken(t *testing.T) {
	interceptor := UnaryServerInterceptor([]byte("0123456789abcdef0123456789abcdef"))
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/reservehub.tenant.v1.TenantService/GetTenant"}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %s, want Unauthenticated", status.Code(err))
	}
}
