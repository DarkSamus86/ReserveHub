package grpc

import (
	"context"
	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"encoding/base64"
	"strconv"

	"github.com/google/uuid"
)

func (s *Server) CreateTenant(ctx context.Context, req *tenantv1.CreateTenantRequest) (*tenantv1.Tenant, error) {
	var operationID uuid.UUID
	if req.GetOperationId() != "" {
		var err error
		operationID, err = parseID(req.GetOperationId(), "operation_id")
		if err != nil {
			return nil, err
		}
	}
	planID, err := parseID(req.GetPlanId(), "plan_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.CreateTenant(ctx, domain.CreateTenantInput{
		OperationID: operationID,
		Name:        req.GetName(), Slug: req.GetSlug(), Timezone: req.GetTimezone(), CurrencyCode: req.GetCurrencyCode(),
		LocaleCode: req.GetLocaleCode(), DefaultLanguage: req.GetDefaultLanguage(), PlanID: planID,
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return tenantMessage(result), nil
}

func (s *Server) GetTenant(ctx context.Context, req *tenantv1.GetTenantRequest) (*tenantv1.Tenant, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.GetTenant(ctx, id)
	if err != nil {
		return nil, grpcError(err)
	}
	return tenantMessage(result), nil
}

func decodePageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, grpcError(domain.ErrInvalidArgument)
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil || offset < 0 {
		return 0, grpcError(domain.ErrInvalidArgument)
	}
	return offset, nil
}

func (s *Server) ListTenants(ctx context.Context, req *tenantv1.ListTenantsRequest) (*tenantv1.ListTenantsResponse, error) {
	offset, err := decodePageToken(req.GetPageToken())
	if err != nil {
		return nil, err
	}
	items, more, err := s.service.ListTenants(ctx, int(req.GetPageSize()), offset, req.GetStatus())
	if err != nil {
		return nil, grpcError(err)
	}
	result := &tenantv1.ListTenantsResponse{Tenants: make([]*tenantv1.Tenant, 0, len(items))}
	for _, item := range items {
		result.Tenants = append(result.Tenants, tenantMessage(item))
	}
	if more {
		result.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset + len(items))))
	}
	return result, nil
}

func (s *Server) UpdateTenant(ctx context.Context, req *tenantv1.UpdateTenantRequest) (*tenantv1.Tenant, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.UpdateTenant(ctx, domain.UpdateTenantInput{TenantID: id, Name: req.Name, Slug: req.Slug})
	if err != nil {
		return nil, grpcError(err)
	}
	return tenantMessage(result), nil
}

func (s *Server) changeState(ctx context.Context, rawID, target string) (*tenantv1.Tenant, error) {
	id, err := parseID(rawID, "tenant_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.ChangeState(ctx, id, target)
	if err != nil {
		return nil, grpcError(err)
	}
	return tenantMessage(result), nil
}

func (s *Server) ActivateTenant(ctx context.Context, req *tenantv1.ChangeTenantStateRequest) (*tenantv1.Tenant, error) {
	return s.changeState(ctx, req.GetTenantId(), domain.StatusActive)
}

func (s *Server) SuspendTenant(ctx context.Context, req *tenantv1.ChangeTenantStateRequest) (*tenantv1.Tenant, error) {
	return s.changeState(ctx, req.GetTenantId(), domain.StatusSuspended)
}

func (s *Server) ResumeTenant(ctx context.Context, req *tenantv1.ChangeTenantStateRequest) (*tenantv1.Tenant, error) {
	return s.changeState(ctx, req.GetTenantId(), domain.StatusActive)
}

func (s *Server) ArchiveTenant(ctx context.Context, req *tenantv1.ChangeTenantStateRequest) (*tenantv1.Tenant, error) {
	return s.changeState(ctx, req.GetTenantId(), domain.StatusArchived)
}

func (s *Server) GetTenantSettings(ctx context.Context, req *tenantv1.GetTenantSettingsRequest) (*tenantv1.TenantSettings, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.GetSettings(ctx, id)
	if err != nil {
		return nil, grpcError(err)
	}
	return settingsMessage(result), nil
}

func (s *Server) UpdateTenantSettings(ctx context.Context, req *tenantv1.UpdateTenantSettingsRequest) (*tenantv1.TenantSettings, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.UpdateSettings(ctx, domain.UpdateSettingsInput{
		TenantID: id, Timezone: req.GetTimezone(), CurrencyCode: req.GetCurrencyCode(),
		LocaleCode: req.GetLocaleCode(), DefaultLanguage: req.GetDefaultLanguage(),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return settingsMessage(result), nil
}
