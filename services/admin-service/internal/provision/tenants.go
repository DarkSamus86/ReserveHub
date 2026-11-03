package provision

import (
	"context"
	"errors"
	"fmt"
	"time"

	"darksamus.com/ReserveHub/admin-service/internal/repository"
	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrOwnerInvitationNotAccepted = errors.New("owner invitation has not been accepted")
	ErrTenantNotFound             = errors.New("tenant not found")
	ErrInvalidArgument            = errors.New("invalid argument")
	ErrInvalidStateTransition     = errors.New("invalid state transition")
	ErrAlreadyExists              = errors.New("already exists")
	ErrPermissionDenied           = errors.New("permission denied")
	ErrResourceExhausted          = errors.New("resource exhausted")
)

type TenantDTO struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Status      string     `json:"status"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
	SuspendedAt *time.Time `json:"suspended_at,omitempty"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
}

type TenantSettingsDTO struct {
	TenantID        string     `json:"tenant_id"`
	Timezone        string     `json:"timezone"`
	CurrencyCode    string     `json:"currency_code"`
	LocaleCode      string     `json:"locale_code"`
	DefaultLanguage string     `json:"default_language"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

type TenantBrandingDTO struct {
	TenantID       string     `json:"tenant_id"`
	Status         string     `json:"status"`
	DisplayName    string     `json:"display_name,omitempty"`
	LogoURL        string     `json:"logo_url,omitempty"`
	FaviconURL     string     `json:"favicon_url,omitempty"`
	PrimaryColor   string     `json:"primary_color,omitempty"`
	SecondaryColor string     `json:"secondary_color,omitempty"`
	AccentColor    string     `json:"accent_color,omitempty"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
}

type TenantDomainDTO struct {
	ID                string     `json:"id"`
	TenantID          string     `json:"tenant_id"`
	Domain            string     `json:"domain"`
	Status            string     `json:"status"`
	IsPrimary         bool       `json:"is_primary"`
	VerificationToken string     `json:"verification_token,omitempty"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	CreatedAt         *time.Time `json:"created_at,omitempty"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
}

type TenantListResult struct {
	Tenants       []TenantDTO `json:"tenants"`
	NextPageToken string      `json:"next_page_token,omitempty"`
}

type TenantDetailResult struct {
	Tenant       TenantDTO          `json:"tenant"`
	Settings     *TenantSettingsDTO `json:"settings,omitempty"`
	Provisioning *Result            `json:"provisioning,omitempty"`
}

func protoTime(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil || !ts.IsValid() {
		return nil
	}
	t := ts.AsTime().UTC()
	return &t
}

func tenantFromProto(pb *tenantv1.Tenant) TenantDTO {
	if pb == nil {
		return TenantDTO{}
	}
	return TenantDTO{
		ID:          pb.GetId(),
		Name:        pb.GetName(),
		Slug:        pb.GetSlug(),
		Status:      pb.GetStatus(),
		CreatedAt:   protoTime(pb.GetCreatedAt()),
		UpdatedAt:   protoTime(pb.GetUpdatedAt()),
		ActivatedAt: protoTime(pb.GetActivatedAt()),
		SuspendedAt: protoTime(pb.GetSuspendedAt()),
		ArchivedAt:  protoTime(pb.GetArchivedAt()),
	}
}

func settingsFromProto(pb *tenantv1.TenantSettings) TenantSettingsDTO {
	if pb == nil {
		return TenantSettingsDTO{}
	}
	return TenantSettingsDTO{
		TenantID:        pb.GetTenantId(),
		Timezone:        pb.GetTimezone(),
		CurrencyCode:    pb.GetCurrencyCode(),
		LocaleCode:      pb.GetLocaleCode(),
		DefaultLanguage: pb.GetDefaultLanguage(),
		CreatedAt:       protoTime(pb.GetCreatedAt()),
		UpdatedAt:       protoTime(pb.GetUpdatedAt()),
	}
}

func brandingFromProto(pb *tenantv1.TenantBranding) TenantBrandingDTO {
	if pb == nil {
		return TenantBrandingDTO{}
	}
	return TenantBrandingDTO{
		TenantID:       pb.GetTenantId(),
		Status:         pb.GetStatus(),
		DisplayName:    pb.GetDisplayName(),
		LogoURL:        pb.GetLogoUrl(),
		FaviconURL:     pb.GetFaviconUrl(),
		PrimaryColor:   pb.GetPrimaryColor(),
		SecondaryColor: pb.GetSecondaryColor(),
		AccentColor:    pb.GetAccentColor(),
		CreatedAt:      protoTime(pb.GetCreatedAt()),
		UpdatedAt:      protoTime(pb.GetUpdatedAt()),
	}
}

func domainFromProto(pb *tenantv1.TenantDomain) TenantDomainDTO {
	if pb == nil {
		return TenantDomainDTO{}
	}
	return TenantDomainDTO{
		ID:                pb.GetId(),
		TenantID:          pb.GetTenantId(),
		Domain:            pb.GetDomain(),
		Status:            pb.GetStatus(),
		IsPrimary:         pb.GetIsPrimary(),
		VerificationToken: pb.GetVerificationToken(),
		VerifiedAt:        protoTime(pb.GetVerifiedAt()),
		CreatedAt:         protoTime(pb.GetCreatedAt()),
		UpdatedAt:         protoTime(pb.GetUpdatedAt()),
	}
}

func mapGRPCError(err error) error {
	if err == nil {
		return nil
	}
	switch status.Code(err) {
	case codes.NotFound:
		return fmt.Errorf("%w: %s", ErrTenantNotFound, status.Convert(err).Message())
	case codes.InvalidArgument:
		return fmt.Errorf("%w: %s", ErrInvalidArgument, status.Convert(err).Message())
	case codes.FailedPrecondition:
		return fmt.Errorf("%w: %s", ErrInvalidStateTransition, status.Convert(err).Message())
	case codes.AlreadyExists:
		return fmt.Errorf("%w: %s", ErrAlreadyExists, status.Convert(err).Message())
	case codes.PermissionDenied:
		return fmt.Errorf("%w: %s", ErrPermissionDenied, status.Convert(err).Message())
	case codes.ResourceExhausted:
		return fmt.Errorf("%w: %s", ErrResourceExhausted, status.Convert(err).Message())
	default:
		return err
	}
}

func (s *Service) withOutgoingAuth(ctx context.Context, bearer string) (context.Context, context.CancelFunc) {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	callCtx = metadata.AppendToOutgoingContext(callCtx, "authorization", "Bearer "+bearer, "x-service-token", s.serviceToken)
	return callCtx, cancel
}

func (s *Service) ListTenants(ctx context.Context, bearer string, pageSize int32, pageToken, statusFilter string) (TenantListResult, error) {
	callCtx, cancel := s.withOutgoingAuth(ctx, bearer)
	defer cancel()
	res, err := s.tenant.ListTenants(callCtx, &tenantv1.ListTenantsRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Status:    statusFilter,
	})
	if err != nil {
		return TenantListResult{}, mapGRPCError(err)
	}
	items := make([]TenantDTO, 0, len(res.GetTenants()))
	for _, t := range res.GetTenants() {
		items = append(items, tenantFromProto(t))
	}
	return TenantListResult{Tenants: items, NextPageToken: res.GetNextPageToken()}, nil
}

func (s *Service) GetTenantDetail(ctx context.Context, bearer string, tenantID uuid.UUID) (TenantDetailResult, error) {
	callCtx, cancel := s.withOutgoingAuth(ctx, bearer)
	defer cancel()
	tenantPB, err := s.tenant.GetTenant(callCtx, &tenantv1.GetTenantRequest{TenantId: tenantID.String()})
	if err != nil {
		return TenantDetailResult{}, mapGRPCError(err)
	}
	detail := TenantDetailResult{Tenant: tenantFromProto(tenantPB)}
	if settingsPB, sErr := s.tenant.GetTenantSettings(callCtx, &tenantv1.GetTenantSettingsRequest{TenantId: tenantID.String()}); sErr == nil {
		settings := settingsFromProto(settingsPB)
		detail.Settings = &settings
	}
	if op, opErr := s.store.GetByTenantID(ctx, tenantID); opErr == nil {
		if live, lErr := s.liveResult(ctx, op); lErr == nil {
			detail.Provisioning = &live
		}
	}
	return detail, nil
}

func (s *Service) ChangeTenantState(ctx context.Context, bearer string, actor, tenantID uuid.UUID, action string) (TenantDTO, error) {
	if action == "activate" {
		op, err := s.store.GetByTenantID(ctx, tenantID)
		if errors.Is(err, repository.ErrNotFound) {
			_ = s.store.RecordTenantLifecycleAction(ctx, tenantID, actor, action, "", "rejected")
			return TenantDTO{}, ErrOwnerInvitationNotAccepted
		}
		if err != nil {
			return TenantDTO{}, err
		}
		live, err := s.liveResult(ctx, op)
		if err != nil {
			return TenantDTO{}, err
		}
		if live.InviteStatusUnavailable {
			_ = s.store.RecordTenantLifecycleAction(ctx, tenantID, actor, action, "", "unavailable")
			return TenantDTO{}, fmt.Errorf("owner invitation status is temporarily unavailable")
		}
		if live.Status != "completed" || live.InviteStatus != "accepted" {
			_ = s.store.RecordTenantLifecycleAction(ctx, tenantID, actor, action, "", "rejected")
			return TenantDTO{}, ErrOwnerInvitationNotAccepted
		}
	}

	callCtx, cancel := s.withOutgoingAuth(ctx, bearer)
	defer cancel()
	req := &tenantv1.ChangeTenantStateRequest{TenantId: tenantID.String()}
	var (
		updated *tenantv1.Tenant
		callErr error
	)
	switch action {
	case "activate":
		updated, callErr = s.tenant.ActivateTenant(callCtx, req)
	case "suspend":
		updated, callErr = s.tenant.SuspendTenant(callCtx, req)
	case "resume":
		updated, callErr = s.tenant.ResumeTenant(callCtx, req)
	case "archive":
		updated, callErr = s.tenant.ArchiveTenant(callCtx, req)
	default:
		return TenantDTO{}, ErrInvalidArgument
	}
	if callErr != nil {
		mapped := mapGRPCError(callErr)
		auditResult := "unavailable"
		if errors.Is(mapped, ErrInvalidStateTransition) || errors.Is(mapped, ErrTenantNotFound) || errors.Is(mapped, ErrInvalidArgument) {
			auditResult = "rejected"
		}
		_ = s.store.RecordTenantLifecycleAction(ctx, tenantID, actor, action, "", auditResult)
		return TenantDTO{}, mapped
	}
	dto := tenantFromProto(updated)
	if err := s.store.RecordTenantLifecycleAction(ctx, tenantID, actor, action, dto.Status, "completed"); err != nil {
		return TenantDTO{}, err
	}
	return dto, nil
}

func (s *Service) GetMemberTenant(ctx context.Context, bearer string, tenantID uuid.UUID) (TenantDTO, error) {
	callCtx, cancel := s.withOutgoingAuth(ctx, bearer)
	defer cancel()
	pb, err := s.tenant.GetTenant(callCtx, &tenantv1.GetTenantRequest{TenantId: tenantID.String()})
	if err != nil {
		return TenantDTO{}, mapGRPCError(err)
	}
	return tenantFromProto(pb), nil
}

func (s *Service) UpdateMemberTenant(ctx context.Context, bearer string, tenantID uuid.UUID, name, slug *string) (TenantDTO, error) {
	callCtx, cancel := s.withOutgoingAuth(ctx, bearer)
	defer cancel()
	pb, err := s.tenant.UpdateTenant(callCtx, &tenantv1.UpdateTenantRequest{
		TenantId: tenantID.String(),
		Name:     name,
		Slug:     slug,
	})
	if err != nil {
		return TenantDTO{}, mapGRPCError(err)
	}
	return tenantFromProto(pb), nil
}

func (s *Service) GetMemberTenantSettings(ctx context.Context, bearer string, tenantID uuid.UUID) (TenantSettingsDTO, error) {
	callCtx, cancel := s.withOutgoingAuth(ctx, bearer)
	defer cancel()
	pb, err := s.tenant.GetTenantSettings(callCtx, &tenantv1.GetTenantSettingsRequest{TenantId: tenantID.String()})
	if err != nil {
		return TenantSettingsDTO{}, mapGRPCError(err)
	}
	return settingsFromProto(pb), nil
}

func (s *Service) UpdateMemberTenantSettings(ctx context.Context, bearer string, tenantID uuid.UUID, timezone, currency, locale, defaultLang string) (TenantSettingsDTO, error) {
	callCtx, cancel := s.withOutgoingAuth(ctx, bearer)
	defer cancel()
	pb, err := s.tenant.UpdateTenantSettings(callCtx, &tenantv1.UpdateTenantSettingsRequest{
		TenantId:        tenantID.String(),
		Timezone:        timezone,
		CurrencyCode:    currency,
		LocaleCode:      locale,
		DefaultLanguage: defaultLang,
	})
	if err != nil {
		return TenantSettingsDTO{}, mapGRPCError(err)
	}
	return settingsFromProto(pb), nil
}

func (s *Service) GetMemberTenantBranding(ctx context.Context, bearer string, tenantID uuid.UUID) (TenantBrandingDTO, error) {
	callCtx, cancel := s.withOutgoingAuth(ctx, bearer)
	defer cancel()
	pb, err := s.tenant.GetTenantBranding(callCtx, &tenantv1.GetTenantBrandingRequest{TenantId: tenantID.String()})
	if err != nil {
		return TenantBrandingDTO{}, mapGRPCError(err)
	}
	return brandingFromProto(pb), nil
}

func (s *Service) UpdateMemberTenantBranding(ctx context.Context, bearer string, tenantID uuid.UUID, in TenantBrandingDTO) (TenantBrandingDTO, error) {
	callCtx, cancel := s.withOutgoingAuth(ctx, bearer)
	defer cancel()
	pb, err := s.tenant.UpdateTenantBranding(callCtx, &tenantv1.UpdateTenantBrandingRequest{
		TenantId:       tenantID.String(),
		DisplayName:    in.DisplayName,
		LogoUrl:        in.LogoURL,
		FaviconUrl:     in.FaviconURL,
		PrimaryColor:   in.PrimaryColor,
		SecondaryColor: in.SecondaryColor,
		AccentColor:    in.AccentColor,
	})
	if err != nil {
		return TenantBrandingDTO{}, mapGRPCError(err)
	}
	return brandingFromProto(pb), nil
}

func (s *Service) GetMemberTenantDomains(ctx context.Context, bearer string, tenantID uuid.UUID) ([]TenantDomainDTO, error) {
	callCtx, cancel := s.withOutgoingAuth(ctx, bearer)
	defer cancel()
	pb, err := s.tenant.GetTenantDomains(callCtx, &tenantv1.GetTenantDomainsRequest{TenantId: tenantID.String()})
	if err != nil {
		return nil, mapGRPCError(err)
	}
	items := make([]TenantDomainDTO, 0, len(pb.GetDomains()))
	for _, d := range pb.GetDomains() {
		items = append(items, domainFromProto(d))
	}
	return items, nil
}
