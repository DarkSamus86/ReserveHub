package grpc

import (
	"context"
	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Server) GetEffectiveLimit(ctx context.Context, req *tenantv1.GetEffectiveLimitRequest) (*tenantv1.EffectiveLimit, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.GetEffectiveLimit(ctx, id, req.GetLimitCode())
	if err != nil {
		return nil, grpcError(err)
	}
	return &tenantv1.EffectiveLimit{TenantId: result.TenantID.String(), Code: result.Code, Value: result.Value, Overridden: result.Overridden}, nil
}

func requestPeriod(starts, ends *timestamppb.Timestamp) (domain.Period, error) {
	start, err := requestTime(starts)
	if err != nil {
		return domain.Period{}, err
	}
	end, err := requestTime(ends)
	if err != nil {
		return domain.Period{}, err
	}
	period := domain.Period{StartsAt: start}
	if !end.IsZero() {
		period.EndsAt = &end
	}
	return period, nil
}

func (s *Server) SetTenantLimitOverride(ctx context.Context, req *tenantv1.SetTenantLimitOverrideRequest) (*tenantv1.TenantLimitOverride, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	period, err := requestPeriod(req.GetStartsAt(), req.GetEndsAt())
	if err != nil {
		return nil, err
	}
	result, err := s.service.SetLimitOverride(ctx, id, req.GetLimitCode(), req.GetValue(), period)
	if err != nil {
		return nil, grpcError(err)
	}
	return limitOverrideMessage(result), nil
}

func (s *Server) RemoveTenantLimitOverride(ctx context.Context, req *tenantv1.RemoveTenantLimitOverrideRequest) (*emptypb.Empty, error) {
	tenantID, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	overrideID, err := parseID(req.GetOverrideId(), "override_id")
	if err != nil {
		return nil, err
	}
	if err := s.service.RemoveLimitOverride(ctx, tenantID, overrideID); err != nil {
		return nil, grpcError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) HasFeature(ctx context.Context, req *tenantv1.HasFeatureRequest) (*tenantv1.EffectiveFeature, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.HasFeature(ctx, id, req.GetFeatureCode())
	if err != nil {
		return nil, grpcError(err)
	}
	return featureMessage(result), nil
}

func (s *Server) GetEffectiveFeatures(ctx context.Context, req *tenantv1.GetEffectiveFeaturesRequest) (*tenantv1.EffectiveFeatures, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	items, err := s.service.GetEffectiveFeatures(ctx, id)
	if err != nil {
		return nil, grpcError(err)
	}
	result := &tenantv1.EffectiveFeatures{TenantId: id.String(), Features: make([]*tenantv1.EffectiveFeature, 0, len(items))}
	for _, item := range items {
		result.Features = append(result.Features, featureMessage(item))
	}
	return result, nil
}

func (s *Server) SetTenantFeatureOverride(ctx context.Context, req *tenantv1.SetTenantFeatureOverrideRequest) (*tenantv1.TenantFeatureOverride, error) {
	id, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	period, err := requestPeriod(req.GetStartsAt(), req.GetEndsAt())
	if err != nil {
		return nil, err
	}
	result, err := s.service.SetFeatureOverride(ctx, id, req.GetFeatureCode(), req.GetEnabled(), period)
	if err != nil {
		return nil, grpcError(err)
	}
	return featureOverrideMessage(result), nil
}

func (s *Server) RemoveTenantFeatureOverride(ctx context.Context, req *tenantv1.RemoveTenantFeatureOverrideRequest) (*emptypb.Empty, error) {
	tenantID, err := parseID(req.GetTenantId(), "tenant_id")
	if err != nil {
		return nil, err
	}
	overrideID, err := parseID(req.GetOverrideId(), "override_id")
	if err != nil {
		return nil, err
	}
	if err := s.service.RemoveFeatureOverride(ctx, tenantID, overrideID); err != nil {
		return nil, grpcError(err)
	}
	return &emptypb.Empty{}, nil
}
