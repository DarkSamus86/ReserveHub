package authn

import (
	"context"
	"crypto/ed25519"
	"crypto/subtle"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type claimsContextKey struct{}

type tenantScopedRequest interface {
	GetTenantId() string
}

var (
	tenantReadMethods = map[string]bool{
		"/reservehub.tenant.v1.TenantService/GetTenant":            true,
		"/reservehub.tenant.v1.TenantService/GetTenantSettings":    true,
		"/reservehub.tenant.v1.TenantService/GetCurrentPlan":       true,
		"/reservehub.tenant.v1.TenantService/GetPlanHistory":       true,
		"/reservehub.tenant.v1.TenantService/GetEffectiveLimit":    true,
		"/reservehub.tenant.v1.TenantService/HasFeature":           true,
		"/reservehub.tenant.v1.TenantService/GetEffectiveFeatures": true,
		"/reservehub.tenant.v1.TenantService/GetTenantDomains":     true,
		"/reservehub.tenant.v1.TenantService/GetTenantBranding":    true,
	}
	tenantOwnerWriteMethods = map[string]bool{
		"/reservehub.tenant.v1.TenantService/UpdateTenant":            true,
		"/reservehub.tenant.v1.TenantService/UpdateTenantSettings":    true,
		"/reservehub.tenant.v1.TenantService/AddTenantDomain":         true,
		"/reservehub.tenant.v1.TenantService/VerifyTenantDomain":      true,
		"/reservehub.tenant.v1.TenantService/SetPrimaryTenantDomain":  true,
		"/reservehub.tenant.v1.TenantService/DisableTenantDomain":     true,
		"/reservehub.tenant.v1.TenantService/UpdateTenantBranding":    true,
	}
)

func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(Claims)
	return claims, ok
}

func WithClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

func UnaryServerInterceptor(publicKey ed25519.PublicKey, platformServiceToken ...[]byte) grpc.UnaryServerInterceptor {
	key := append(ed25519.PublicKey(nil), publicKey...)
	var serviceToken []byte
	if len(platformServiceToken) > 0 {
		serviceToken = append([]byte(nil), platformServiceToken[0]...)
	}
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == "/grpc.health.v1.Health/Check" || info.FullMethod == "/grpc.health.v1.Health/Watch" {
			return handler(ctx, request)
		}
		token, err := bearerToken(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "valid bearer token required")
		}
		claims, err := VerifyToken(key, token, time.Now())
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired bearer token")
		}
		if err := authorizeMethod(info.FullMethod, claims, request); err != nil {
			return nil, err
		}
		if len(serviceToken) > 0 {
			md, _ := metadata.FromIncomingContext(ctx)
			values := md.Get("x-service-token")
			if len(values) != 1 || subtle.ConstantTimeCompare([]byte(values[0]), serviceToken) != 1 {
				return nil, status.Error(codes.Unauthenticated, "authorized service required")
			}
		}
		return handler(WithClaims(ctx, claims), request)
	}
}

func authorizeMethod(fullMethod string, claims Claims, request any) error {
	if claims.Scope == "platform" && claims.Role == RoleAdmin {
		return nil
	}
	if claims.Scope != "tenant" || claims.Role != RoleUser || claims.TenantID == "" {
		return status.Error(codes.PermissionDenied, "user has no tenant access assigned")
	}
	isRead := tenantReadMethods[fullMethod]
	isOwnerWrite := tenantOwnerWriteMethods[fullMethod]
	if !isRead && !isOwnerWrite {
		return status.Error(codes.PermissionDenied, "platform administrator required")
	}
	if isOwnerWrite && claims.PermissionProfile != ProfileTenantOwner {
		return status.Error(codes.PermissionDenied, "tenant owner permission required")
	}
	if isRead && claims.PermissionProfile != ProfileTenantOwner && claims.PermissionProfile != ProfileTenantViewer {
		return status.Error(codes.PermissionDenied, "user has no tenant access assigned")
	}
	scopedReq, ok := request.(tenantScopedRequest)
	if !ok {
		return status.Error(codes.PermissionDenied, "tenant context required")
	}
	targetID, err := uuid.Parse(scopedReq.GetTenantId())
	if err != nil || targetID.String() != claims.TenantID {
		return status.Error(codes.PermissionDenied, "cross-tenant access denied")
	}
	return nil
}

func bearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("authorization")
	if len(values) != 1 {
		return "", status.Error(codes.Unauthenticated, "missing authorization metadata")
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", status.Error(codes.Unauthenticated, "invalid authorization metadata")
	}
	return parts[1], nil
}
