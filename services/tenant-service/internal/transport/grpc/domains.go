package grpc

import (
	"context"
	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
)

func (s *Server) AddTenantDomain(ctx context.Context, req *tenantv1.AddTenantDomainRequest) (*tenantv1.TenantDomain, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.AddDomain(ctx, id, req.GetDomain())
	if err != nil {
		return nil, grpcError(err)
	}
	return domainMessage(result), nil
}

func (s *Server) GetTenantDomains(ctx context.Context, req *tenantv1.GetTenantDomainsRequest) (*tenantv1.GetTenantDomainsResponse, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	items, err := s.service.GetDomains(ctx, id)
	if err != nil {
		return nil, grpcError(err)
	}
	result := &tenantv1.GetTenantDomainsResponse{Domains: make([]*tenantv1.TenantDomain, 0, len(items))}
	for _, item := range items {
		result.Domains = append(result.Domains, domainMessage(item))
	}
	return result, nil
}

func (s *Server) VerifyTenantDomain(ctx context.Context, req *tenantv1.VerifyTenantDomainRequest) (*tenantv1.TenantDomain, error) {
	tenantID, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	domainID, err := parseID(req.GetDomainId(), "domain_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.VerifyDomain(ctx, tenantID, domainID, req.GetVerificationToken())
	if err != nil {
		return nil, grpcError(err)
	}
	return domainMessage(result), nil
}

func (s *Server) SetPrimaryTenantDomain(ctx context.Context, req *tenantv1.SetPrimaryTenantDomainRequest) (*tenantv1.TenantDomain, error) {
	tenantID, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	domainID, err := parseID(req.GetDomainId(), "domain_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.SetPrimaryDomain(ctx, tenantID, domainID)
	if err != nil {
		return nil, grpcError(err)
	}
	return domainMessage(result), nil
}

func (s *Server) DisableTenantDomain(ctx context.Context, req *tenantv1.DisableTenantDomainRequest) (*tenantv1.TenantDomain, error) {
	tenantID, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	domainID, err := parseID(req.GetDomainId(), "domain_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.DisableDomain(ctx, tenantID, domainID)
	if err != nil {
		return nil, grpcError(err)
	}
	return domainMessage(result), nil
}
