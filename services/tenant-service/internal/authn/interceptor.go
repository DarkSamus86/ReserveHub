package authn

import (
	"context"
	"crypto/ed25519"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func UnaryServerInterceptor(publicKey ed25519.PublicKey) grpc.UnaryServerInterceptor {
	key := append(ed25519.PublicKey(nil), publicKey...)
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
		if claims.Scope != "platform" || claims.Role != RoleAdmin {
			return nil, status.Error(codes.PermissionDenied, "user has no tenant access assigned")
		}
		return handler(ctx, request)
	}
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
