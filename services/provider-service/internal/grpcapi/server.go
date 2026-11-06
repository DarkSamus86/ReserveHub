package grpcapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	providerv1 "darksamus.com/ReserveHub/provider-service/gen/provider/v1"
	"darksamus.com/ReserveHub/provider-service/internal/application"
	"darksamus.com/ReserveHub/provider-service/internal/domain"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	providerv1.UnimplementedProviderServiceServer
	Service application.Service
}

func (s Server) CreateProvider(ctx context.Context, req *providerv1.CreateProviderRequest) (*providerv1.CreateProviderResponse, error) {
	p, err := principalFrom(ctx)
	if err != nil {
		return nil, err
	}
	tenant, err := parseID(req.GetTenantId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant_id")
	}
	operation, err := parseID(req.GetOperationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid operation_id")
	}
	name, city := strings.TrimSpace(req.GetName()), strings.TrimSpace(req.GetCity())
	if !validText(name, 255) || !validText(city, 160) {
		return nil, status.Error(codes.InvalidArgument, "invalid name or city")
	}
	if err = s.Service.CheckTenant(ctx, tenant, p.bearer, true, true); err != nil {
		return nil, mapError(err)
	}
	item, replayed, err := s.Service.Create(ctx, operation, tenant, p.actor, name, city)
	if err != nil {
		return nil, mapError(err)
	}
	return &providerv1.CreateProviderResponse{Provider: providerMessage(item), Replayed: replayed}, nil
}

func (s Server) ListProviders(ctx context.Context, req *providerv1.ListProvidersRequest) (*providerv1.ListProvidersResponse, error) {
	p, err := principalFrom(ctx)
	if err != nil {
		return nil, err
	}
	tenant, err := parseID(req.GetTenantId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant_id")
	}
	limit := int(req.GetPageSize())
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return nil, status.Error(codes.InvalidArgument, "invalid page_size")
	}
	before, beforeID, err := decodeCursor(req.GetPageToken(), tenant)
	if err != nil {
		return nil, err
	}
	if err = s.Service.CheckTenant(ctx, tenant, p.bearer, p.caller == callerAdmin, false); err != nil {
		return nil, mapError(err)
	}
	items, err := s.Service.List(ctx, tenant, limit+1, before, beforeID)
	if err != nil {
		return nil, mapError(err)
	}
	result := &providerv1.ListProvidersResponse{Providers: make([]*providerv1.Provider, 0, len(items))}
	if len(items) > limit {
		items = items[:limit]
		result.NextPageToken = encodeCursor(items[len(items)-1], tenant)
	}
	for _, item := range items {
		result.Providers = append(result.Providers, providerMessage(item))
	}
	return result, nil
}

func (s Server) GetProvider(ctx context.Context, req *providerv1.GetProviderRequest) (*providerv1.Provider, error) {
	p, err := principalFrom(ctx)
	if err != nil {
		return nil, err
	}
	tenant, id, err := parseResourceIDs(req.GetTenantId(), req.GetProviderId())
	if err != nil {
		return nil, err
	}
	if err = s.Service.CheckTenant(ctx, tenant, p.bearer, p.caller == callerAdmin, false); err != nil {
		return nil, mapError(err)
	}
	item, err := s.Service.Get(ctx, tenant, id)
	if err != nil {
		return nil, mapError(err)
	}
	return providerMessage(item), nil
}

func (s Server) UpdateProviderProfile(ctx context.Context, req *providerv1.UpdateProviderProfileRequest) (*providerv1.Provider, error) {
	p, err := principalFrom(ctx)
	if err != nil {
		return nil, err
	}
	tenant, id, err := parseResourceIDs(req.GetTenantId(), req.GetProviderId())
	if err != nil {
		return nil, err
	}
	if req.Name == nil && req.City == nil {
		return nil, status.Error(codes.InvalidArgument, "name or city required")
	}
	var name, city *string
	if req.Name != nil {
		v := strings.TrimSpace(req.GetName())
		if !validText(v, 255) {
			return nil, status.Error(codes.InvalidArgument, "invalid name")
		}
		name = &v
	}
	if req.City != nil {
		v := strings.TrimSpace(req.GetCity())
		if !validText(v, 160) {
			return nil, status.Error(codes.InvalidArgument, "invalid city")
		}
		city = &v
	}
	if err = s.Service.CheckTenant(ctx, tenant, p.bearer, false, true); err != nil {
		return nil, mapError(err)
	}
	item, err := s.Service.UpdateProfile(ctx, tenant, id, p.actor, name, city)
	if err != nil {
		return nil, mapError(err)
	}
	return providerMessage(item), nil
}

func (s Server) ChangeProviderStatus(ctx context.Context, req *providerv1.ChangeProviderStatusRequest) (*providerv1.Provider, error) {
	p, err := principalFrom(ctx)
	if err != nil {
		return nil, err
	}
	tenant, id, err := parseResourceIDs(req.GetTenantId(), req.GetProviderId())
	if err != nil {
		return nil, err
	}
	if req.GetStatus() != "ACTIVE" && req.GetStatus() != "SUSPENDED" {
		return nil, status.Error(codes.InvalidArgument, "invalid status")
	}
	if err = s.Service.CheckTenant(ctx, tenant, p.bearer, true, true); err != nil {
		return nil, mapError(err)
	}
	item, err := s.Service.UpdateStatus(ctx, tenant, id, p.actor, req.GetStatus())
	if err != nil {
		return nil, mapError(err)
	}
	return providerMessage(item), nil
}

func parseResourceIDs(tenantText, idText string) (uuid.UUID, uuid.UUID, error) {
	tenant, err := parseID(tenantText)
	if err != nil {
		return uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "invalid tenant_id")
	}
	id, err := parseID(idText)
	if err != nil {
		return uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "invalid provider_id")
	}
	return tenant, id, nil
}

func validText(s string, max int) bool {
	return len(s) > 0 && len(s) <= max && !strings.ContainsAny(s, "\x00\r\n")
}

func providerMessage(p domain.Provider) *providerv1.Provider {
	return &providerv1.Provider{Id: p.ID.String(), TenantId: p.TenantID.String(), Name: p.Name, City: p.City, Status: p.Status, CreatedAt: timestamppb.New(p.CreatedAt), UpdatedAt: timestamppb.New(p.UpdatedAt)}
}

type cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
	Tenant    uuid.UUID `json:"tenant_id"`
}

func decodeCursor(token string, tenant uuid.UUID) (time.Time, uuid.UUID, error) {
	if token == "" {
		return time.Time{}, uuid.Nil, nil
	}
	if len(token) > 512 {
		return time.Time{}, uuid.Nil, status.Error(codes.InvalidArgument, "invalid page_token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, uuid.Nil, status.Error(codes.InvalidArgument, "invalid page_token")
	}
	var c cursor
	if json.Unmarshal(raw, &c) != nil || c.Tenant != tenant || c.ID == uuid.Nil || c.CreatedAt.IsZero() {
		return time.Time{}, uuid.Nil, status.Error(codes.InvalidArgument, "invalid page_token")
	}
	return c.CreatedAt, c.ID, nil
}
func encodeCursor(p domain.Provider, tenant uuid.UUID) string {
	raw, _ := json.Marshal(cursor{p.CreatedAt, p.ID, tenant})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, domain.ErrConflict):
		return status.Error(codes.AlreadyExists, "idempotency key already used")
	case errors.Is(err, application.ErrTenantArchived):
		return status.Error(codes.PermissionDenied, "access denied")
	case errors.Is(err, application.ErrTenantInactive):
		return status.Error(codes.FailedPrecondition, "tenant is not active")
	case errors.Is(err, application.ErrTenantUnavailable):
		return status.Error(codes.Unavailable, "tenant service unavailable")
	}
	switch status.Code(err) {
	case codes.NotFound:
		return status.Error(codes.NotFound, "not found")
	case codes.Unauthenticated:
		return status.Error(codes.Unauthenticated, "invalid token")
	case codes.PermissionDenied, codes.FailedPrecondition:
		return status.Error(codes.PermissionDenied, "access denied")
	default:
		return status.Error(codes.Unavailable, "service unavailable")
	}
}
