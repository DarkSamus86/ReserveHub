package grpc

import (
	"errors"
	"fmt"
	"time"

	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"darksamus.com/ReserveHub/tenant-service/internal/application"
	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	tenantv1.UnimplementedTenantServiceServer
	service *application.Service
}

func New(service *application.Service) *Server { return &Server{service: service} }

func parseID(value, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%s must be a UUID", field))
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

var _ tenantv1.TenantServiceServer = (*Server)(nil)
