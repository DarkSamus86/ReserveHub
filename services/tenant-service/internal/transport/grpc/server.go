package grpc

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"time"

	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"darksamus.com/ReserveHub/tenant-service/internal/application"
	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	tenantv1.UnimplementedTenantServiceServer
	service *application.Service
}

func New(service *application.Service) *Server { return &Server{service: service} }

func parseID(value, field string) (uuid.UUID, error) {
	id, err := domain.ParseID(value, field)
	if err != nil {
		return uuid.Nil, grpcError(err)
	}
	return id, nil
}

func grpcError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrTenantNotFound),
		errors.Is(err, domain.ErrPlanNotFound),
		errors.Is(err, domain.ErrTenantPlanNotFound),
		errors.Is(err, domain.ErrLimitTypeNotFound),
		errors.Is(err, domain.ErrEffectiveLimitNotFound),
		errors.Is(err, domain.ErrFeatureNotFound),
		errors.Is(err, domain.ErrDomainNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrSlugAlreadyExists), errors.Is(err, domain.ErrDomainAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrDomainLimitExceeded):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, domain.ErrBrandingNotAllowed), errors.Is(err, domain.ErrFeatureDisabled):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrInvalidStateTransition),
		errors.Is(err, domain.ErrPlanInactive),
		errors.Is(err, domain.ErrDomainNotVerified):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "internal tenant service error")
	}
}

func timestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func optionalTimestamp(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamppb.New(*value)
}

func requestTime(value *timestamppb.Timestamp) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}
	if err := value.CheckValid(); err != nil {
		return time.Time{}, status.Error(codes.InvalidArgument, "invalid timestamp")
	}
	return value.AsTime(), nil
}

func tenantMessage(value domain.Tenant) *tenantv1.Tenant {
	return &tenantv1.Tenant{
		Id: value.ID.String(), Name: value.Name, Slug: value.Slug, Status: value.Status,
		CreatedAt: timestamp(value.CreatedAt), UpdatedAt: timestamp(value.UpdatedAt),
		ActivatedAt: optionalTimestamp(value.ActivatedAt), SuspendedAt: optionalTimestamp(value.SuspendedAt),
		ArchivedAt: optionalTimestamp(value.ArchivedAt),
	}
}

func settingsMessage(value domain.Settings) *tenantv1.TenantSettings {
	return &tenantv1.TenantSettings{
		TenantId: value.TenantID.String(), Timezone: value.Timezone, CurrencyCode: value.CurrencyCode,
		LocaleCode: value.LocaleCode, DefaultLanguage: value.DefaultLanguage,
		CreatedAt: timestamp(value.CreatedAt), UpdatedAt: timestamp(value.UpdatedAt),
	}
}

func planMessage(value domain.TenantPlan) *tenantv1.TenantPlan {
	return &tenantv1.TenantPlan{
		Id: value.ID.String(), TenantId: value.TenantID.String(),
		Plan: &tenantv1.SubscriptionPlan{
			Id: value.Plan.ID.String(), Code: value.Plan.Code, Name: value.Plan.Name,
			Description: value.Plan.Description, IsActive: value.Plan.Active,
		},
		StartsAt: timestamp(value.StartsAt), EndsAt: optionalTimestamp(value.EndsAt), CreatedAt: timestamp(value.CreatedAt),
	}
}

func limitOverrideMessage(value domain.LimitOverride) *tenantv1.TenantLimitOverride {
	return &tenantv1.TenantLimitOverride{
		Id: value.ID.String(), TenantId: value.TenantID.String(), LimitCode: value.Code, Value: value.Value,
		StartsAt: timestamp(value.StartsAt), EndsAt: optionalTimestamp(value.EndsAt), CreatedAt: timestamp(value.CreatedAt),
	}
}

func featureMessage(value domain.EffectiveFeature) *tenantv1.EffectiveFeature {
	return &tenantv1.EffectiveFeature{TenantId: value.TenantID.String(), Code: value.Code, Enabled: value.Enabled, Overridden: value.Overridden}
}

func featureOverrideMessage(value domain.FeatureOverride) *tenantv1.TenantFeatureOverride {
	return &tenantv1.TenantFeatureOverride{
		Id: value.ID.String(), TenantId: value.TenantID.String(), FeatureCode: value.Code, Enabled: value.Enabled,
		StartsAt: timestamp(value.StartsAt), EndsAt: optionalTimestamp(value.EndsAt), CreatedAt: timestamp(value.CreatedAt),
	}
}

func domainMessage(value domain.TenantDomain) *tenantv1.TenantDomain {
	return &tenantv1.TenantDomain{
		Id: value.ID.String(), TenantId: value.TenantID.String(), Domain: value.Domain, Status: value.Status,
		IsPrimary: value.Primary, VerificationToken: value.VerificationToken,
		VerifiedAt: optionalTimestamp(value.VerifiedAt), CreatedAt: timestamp(value.CreatedAt), UpdatedAt: timestamp(value.UpdatedAt),
	}
}

func brandingMessage(value domain.Branding) *tenantv1.TenantBranding {
	return &tenantv1.TenantBranding{
		TenantId: value.TenantID.String(), Status: value.Status, DisplayName: value.DisplayName,
		LogoUrl: value.LogoURL, FaviconUrl: value.FaviconURL, PrimaryColor: value.PrimaryColor,
		SecondaryColor: value.SecondaryColor, AccentColor: value.AccentColor,
		CreatedAt: timestamp(value.CreatedAt), UpdatedAt: timestamp(value.UpdatedAt),
	}
}

func (s *Server) CreateTenant(ctx context.Context, req *tenantv1.CreateTenantRequest) (*tenantv1.Tenant, error) {
	planID, err := parseID(req.GetPlanId(), "plan_id")
	if err != nil {
		return nil, err
	}
	result, err := s.service.CreateTenant(ctx, domain.CreateTenantInput{
		Name: req.GetName(), Slug: req.GetSlug(), Timezone: req.GetTimezone(), CurrencyCode: req.GetCurrencyCode(),
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

var _ tenantv1.TenantServiceServer = (*Server)(nil)
