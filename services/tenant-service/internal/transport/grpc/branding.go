package grpc

import (
	"context"
	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"darksamus.com/ReserveHub/tenant-service/internal/domain"
)

func (s *Server) GetTenantBranding(ctx context.Context, req *tenantv1.GetTenantBrandingRequest) (*tenantv1.TenantBranding, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.GetBranding(ctx, id)
	if err != nil {
		return nil, grpcError(err)
	}
	return brandingMessage(result), nil
}

func (s *Server) UpdateTenantBranding(ctx context.Context, req *tenantv1.UpdateTenantBrandingRequest) (*tenantv1.TenantBranding, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.UpdateBranding(ctx, domain.Branding{
		TenantID: id, DisplayName: req.GetDisplayName(), LogoURL: req.GetLogoUrl(), FaviconURL: req.GetFaviconUrl(),
		PrimaryColor: req.GetPrimaryColor(), SecondaryColor: req.GetSecondaryColor(), AccentColor: req.GetAccentColor(),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return brandingMessage(result), nil
}
