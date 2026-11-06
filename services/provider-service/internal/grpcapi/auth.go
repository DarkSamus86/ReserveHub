package grpcapi

import (
	"context"
	"crypto/ed25519"
	"crypto/subtle"
	"strings"
	"time"

	providerv1 "darksamus.com/ReserveHub/provider-service/gen/provider/v1"
	"darksamus.com/ReserveHub/provider-service/internal/authn"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type callerKind uint8

const (
	callerAdmin callerKind = iota + 1
	callerWorkspace
)

type principal struct {
	actor  uuid.UUID
	tenant uuid.UUID
	bearer string
	caller callerKind
}
type principalKey struct{}

type tenantRequest interface{ GetTenantId() string }

// UnaryAuth authenticates both the calling workload and the user on every RPC.
// Distinct credentials make it impossible for Workspace to invoke platform methods.
func UnaryAuth(key ed25519.PublicKey, adminToken, workspaceToken string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(ctx, request)
		}
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "caller credentials required")
		}
		serviceValues := md.Get("x-service-token")
		bearerValues := md.Get("authorization")
		if len(serviceValues) != 1 || len(bearerValues) != 1 {
			return nil, status.Error(codes.Unauthenticated, "caller credentials required")
		}
		var caller callerKind
		switch {
		case subtle.ConstantTimeCompare([]byte(serviceValues[0]), []byte(adminToken)) == 1:
			caller = callerAdmin
		case subtle.ConstantTimeCompare([]byte(serviceValues[0]), []byte(workspaceToken)) == 1:
			caller = callerWorkspace
		default:
			return nil, status.Error(codes.Unauthenticated, "authorized service required")
		}
		parts := strings.Fields(bearerValues[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return nil, status.Error(codes.Unauthenticated, "valid bearer token required")
		}
		p := principal{bearer: parts[1], caller: caller}
		now := time.Now()
		if caller == callerAdmin {
			actor, valid := authn.VerifyPlatformAdmin(key, parts[1], now)
			if !valid {
				return nil, status.Error(codes.Unauthenticated, "valid platform admin token required")
			}
			p.actor = actor
		} else {
			member, valid := authn.VerifyTenantMember(key, parts[1], now)
			if !valid {
				return nil, status.Error(codes.Unauthenticated, "valid tenant token required")
			}
			if member.PermissionProfile != authn.ProfileTenantOwner {
				return nil, status.Error(codes.PermissionDenied, "tenant owner required")
			}
			p.actor = member.UserID
			p.tenant = member.TenantID
		}
		var methodAllowed bool
		switch info.FullMethod {
		case providerv1.ProviderService_CreateProvider_FullMethodName, providerv1.ProviderService_ChangeProviderStatus_FullMethodName:
			methodAllowed = caller == callerAdmin
		case providerv1.ProviderService_UpdateProviderProfile_FullMethodName:
			methodAllowed = caller == callerWorkspace
		case providerv1.ProviderService_GetProvider_FullMethodName, providerv1.ProviderService_ListProviders_FullMethodName:
			methodAllowed = true
		}
		if !methodAllowed {
			return nil, status.Error(codes.PermissionDenied, "operation not allowed for caller")
		}
		scoped, ok := request.(tenantRequest)
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "tenant_id required")
		}
		tenantID, err := parseID(scoped.GetTenantId())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid tenant_id")
		}
		if caller == callerWorkspace && tenantID != p.tenant {
			return nil, status.Error(codes.PermissionDenied, "tenant access denied")
		}
		return handler(context.WithValue(ctx, principalKey{}, p), request)
	}
}

func principalFrom(ctx context.Context) (principal, error) {
	p, ok := ctx.Value(principalKey{}).(principal)
	if !ok {
		return p, status.Error(codes.Unauthenticated, "caller credentials required")
	}
	return p, nil
}

func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, status.Error(codes.InvalidArgument, "invalid UUID")
	}
	return id, nil
}
