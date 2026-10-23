package grpc

import (
	"context"
	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
)

func (s *Server) GetCurrentPlan(ctx context.Context, req *tenantv1.GetCurrentPlanRequest) (*tenantv1.TenantPlan, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.GetCurrentPlan(ctx, id)
	if err != nil {
		return nil, grpcError(err)
	}
	return planMessage(result), nil
}

func (s *Server) GetPlanHistory(ctx context.Context, req *tenantv1.GetPlanHistoryRequest) (*tenantv1.GetPlanHistoryResponse, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	items, err := s.service.GetPlanHistory(ctx, id)
	if err != nil {
		return nil, grpcError(err)
	}
	result := &tenantv1.GetPlanHistoryResponse{Plans: make([]*tenantv1.TenantPlan, 0, len(items))}
	for _, item := range items {
		result.Plans = append(result.Plans, planMessage(item))
	}
	return result, nil
}

func (s *Server) ChangeTenantPlan(ctx context.Context, req *tenantv1.ChangeTenantPlanRequest) (*tenantv1.TenantPlan, error) {
	tenantID, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	planID, err := parseID(req.GetPlanId(), "plan_id")
	if err != nil {
		return nil, err
	}
	effective, err := requestTime(req.GetEffectiveFrom())
	if err != nil {
		return nil, err
	}
	result, err := s.service.ChangePlan(ctx, tenantID, planID, effective)
	if err != nil {
		return nil, grpcError(err)
	}
	return planMessage(result), nil
}
