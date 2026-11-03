package httpapi

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"darksamus.com/ReserveHub/admin-service/internal/authn"
	"darksamus.com/ReserveHub/admin-service/internal/provision"
	"darksamus.com/ReserveHub/admin-service/internal/repository"
	"github.com/google/uuid"
)

type Handler struct {
	store     *repository.Store
	service   *provision.Service
	publicKey ed25519.PublicKey
}

func New(store *repository.Store, service *provision.Service, publicKey ed25519.PublicKey) http.Handler {
	h := &Handler{store: store, service: service, publicKey: append(ed25519.PublicKey(nil), publicKey...)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health/ready", h.ready)
	mux.HandleFunc("POST /v1/admin/tenants", h.createTenant)
	mux.HandleFunc("GET /v1/admin/tenants", h.listTenants)
	mux.HandleFunc("GET /v1/admin/tenants/{tenant_id}", h.getTenantDetail)
	mux.HandleFunc("POST /v1/admin/tenants/{tenant_id}/activate", h.changeTenantState("activate"))
	mux.HandleFunc("POST /v1/admin/tenants/{tenant_id}/suspend", h.changeTenantState("suspend"))
	mux.HandleFunc("POST /v1/admin/tenants/{tenant_id}/resume", h.changeTenantState("resume"))
	mux.HandleFunc("POST /v1/admin/tenants/{tenant_id}/archive", h.changeTenantState("archive"))
	mux.HandleFunc("GET /v1/admin/tenant-provisioning/{operation_id}", h.getOperation)
	mux.HandleFunc("POST /v1/admin/tenant-provisioning/{operation_id}/reissue-invitation", h.reissueInvitation)

	mux.HandleFunc("GET /v1/tenants/{tenant_id}", h.getMemberTenant)
	mux.HandleFunc("PATCH /v1/tenants/{tenant_id}", h.updateMemberTenant)
	mux.HandleFunc("GET /v1/tenants/{tenant_id}/settings", h.getMemberTenantSettings)
	mux.HandleFunc("PUT /v1/tenants/{tenant_id}/settings", h.updateMemberTenantSettings)
	mux.HandleFunc("GET /v1/tenants/{tenant_id}/branding", h.getMemberTenantBranding)
	mux.HandleFunc("PUT /v1/tenants/{tenant_id}/branding", h.updateMemberTenantBranding)
	mux.HandleFunc("GET /v1/tenants/{tenant_id}/domains", h.getMemberTenantDomains)
	return mux
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if h.store.Ping(ctx) != nil {
		apiError(w, http.StatusServiceUnavailable, "not_ready")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) createTenant(w http.ResponseWriter, r *http.Request) {
	actor, bearer, ok := authn.PlatformAdminFromRequest(r, h.publicKey, time.Now())
	if !ok {
		apiError(w, http.StatusForbidden, "platform_admin_required")
		return
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 {
		apiError(w, http.StatusBadRequest, "idempotency_key_must_be_uuid")
		return
	}
	opID, err := uuid.Parse(keys[0])
	if err != nil || opID == uuid.Nil {
		apiError(w, http.StatusBadRequest, "idempotency_key_must_be_uuid")
		return
	}
	var input provision.Input
	if err := decodeJSON(w, r, &input); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = strings.ToLower(strings.TrimSpace(input.Slug))
	input.OwnerEmail = strings.ToLower(strings.TrimSpace(input.OwnerEmail))
	mailbox, err := mail.ParseAddress(input.OwnerEmail)
	if err != nil || mailbox.Address != input.OwnerEmail || len(input.OwnerEmail) > 254 {
		apiError(w, http.StatusBadRequest, "invalid_owner_email")
		return
	}
	if _, err := uuid.Parse(input.PlanID); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_plan_id")
		return
	}
	requestJSON, _ := json.Marshal(input)
	hash := sha256.Sum256(requestJSON)
	op, resultCode, err := h.service.Run(r.Context(), opID, actor, hash[:], requestJSON, input, bearer)
	if err != nil {
		slog.Error("tenant_provision_attempt", "actor_id", actor, "operation_id", opID, "result", "operation_unavailable")
		apiError(w, http.StatusServiceUnavailable, "operation_unavailable")
		return
	}
	slog.Info("tenant_provision_attempt", "actor_id", actor, "operation_id", opID, "tenant_id", op.TenantID, "result", op.Status)
	jsonResponse(w, resultCode, op)
}

func (h *Handler) listTenants(w http.ResponseWriter, r *http.Request) {
	_, bearer, ok := authn.PlatformAdminFromRequest(r, h.publicKey, time.Now())
	if !ok {
		apiError(w, http.StatusForbidden, "platform_admin_required")
		return
	}
	query := r.URL.Query()
	for key := range query {
		if key != "page_size" && key != "page_token" && key != "status" {
			apiError(w, http.StatusBadRequest, "invalid_query_parameter")
			return
		}
	}
	var pageSize int32
	if rawSize := query.Get("page_size"); rawSize != "" {
		parsed, err := strconv.Atoi(rawSize)
		if err != nil || parsed <= 0 || parsed > 200 {
			apiError(w, http.StatusBadRequest, "invalid_page_size")
			return
		}
		pageSize = int32(parsed)
	}
	result, err := h.service.ListTenants(r.Context(), bearer, pageSize, query.Get("page_token"), query.Get("status"))
	if err != nil {
		writeProvisionError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (h *Handler) getTenantDetail(w http.ResponseWriter, r *http.Request) {
	_, bearer, ok := authn.PlatformAdminFromRequest(r, h.publicKey, time.Now())
	if !ok {
		apiError(w, http.StatusForbidden, "platform_admin_required")
		return
	}
	tenantID, err := uuid.Parse(r.PathValue("tenant_id"))
	if err != nil || tenantID == uuid.Nil {
		apiError(w, http.StatusBadRequest, "invalid_tenant_id")
		return
	}
	result, err := h.service.GetTenantDetail(r.Context(), bearer, tenantID)
	if err != nil {
		writeProvisionError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (h *Handler) changeTenantState(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, bearer, ok := authn.PlatformAdminFromRequest(r, h.publicKey, time.Now())
		if !ok {
			apiError(w, http.StatusForbidden, "platform_admin_required")
			return
		}
		tenantID, err := uuid.Parse(r.PathValue("tenant_id"))
		if err != nil || tenantID == uuid.Nil {
			apiError(w, http.StatusBadRequest, "invalid_tenant_id")
			return
		}
		var body struct {
			Reason string `json:"reason"`
		}
		if r.ContentLength != 0 {
			if err := decodeJSON(w, r, &body); err != nil {
				apiError(w, http.StatusBadRequest, "invalid_request")
				return
			}
		}
		updated, err := h.service.ChangeTenantState(r.Context(), bearer, actor, tenantID, action)
		if err != nil {
			writeProvisionError(w, err)
			return
		}
		jsonResponse(w, http.StatusOK, updated)
	}
}

func (h *Handler) getOperation(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := authn.PlatformAdminFromRequest(r, h.publicKey, time.Now()); !ok {
		apiError(w, http.StatusForbidden, "platform_admin_required")
		return
	}
	opID, err := uuid.Parse(r.PathValue("operation_id"))
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid_operation_id")
		return
	}
	op, err := h.service.Get(r.Context(), opID)
	if errors.Is(err, repository.ErrNotFound) {
		apiError(w, http.StatusNotFound, "operation_not_found")
		return
	}
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "operation_unavailable")
		return
	}
	jsonResponse(w, http.StatusOK, op)
}

func (h *Handler) reissueInvitation(w http.ResponseWriter, r *http.Request) {
	actor, _, ok := authn.PlatformAdminFromRequest(r, h.publicKey, time.Now())
	if !ok {
		apiError(w, http.StatusForbidden, "platform_admin_required")
		return
	}
	opID, err := uuid.Parse(r.PathValue("operation_id"))
	if err != nil || opID == uuid.Nil {
		apiError(w, http.StatusBadRequest, "invalid_operation_id")
		return
	}
	err = h.service.ReissueInvitation(r.Context(), opID, actor)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusAccepted)
	case errors.Is(err, repository.ErrNotFound):
		apiError(w, http.StatusNotFound, "operation_not_found")
	case errors.Is(err, provision.ErrReissueUnavailable):
		apiError(w, http.StatusConflict, "reissue_unavailable")
	default:
		apiError(w, http.StatusServiceUnavailable, "operation_unavailable")
	}
}

func (h *Handler) authorizeTenantMemberOrAdmin(w http.ResponseWriter, r *http.Request, requireOwner bool) (uuid.UUID, string, bool) {
	tenantID, err := uuid.Parse(r.PathValue("tenant_id"))
	if err != nil || tenantID == uuid.Nil {
		apiError(w, http.StatusBadRequest, "invalid_tenant_id")
		return uuid.Nil, "", false
	}
	now := time.Now()
	if _, bearer, ok := authn.PlatformAdminFromRequest(r, h.publicKey, now); ok {
		return tenantID, bearer, true
	}
	member, bearer, ok := authn.TenantMemberFromRequest(r, h.publicKey, now)
	if !ok {
		apiError(w, http.StatusForbidden, "tenant_access_denied")
		return uuid.Nil, "", false
	}
	if member.TenantID != tenantID {
		apiError(w, http.StatusForbidden, "cross_tenant_access_denied")
		return uuid.Nil, "", false
	}
	if requireOwner && member.PermissionProfile != authn.ProfileTenantOwner {
		apiError(w, http.StatusForbidden, "tenant_owner_required")
		return uuid.Nil, "", false
	}
	return tenantID, bearer, true
}

func (h *Handler) getMemberTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, bearer, ok := h.authorizeTenantMemberOrAdmin(w, r, false)
	if !ok {
		return
	}
	result, err := h.service.GetMemberTenant(r.Context(), bearer, tenantID)
	if err != nil {
		writeProvisionError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (h *Handler) updateMemberTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, bearer, ok := h.authorizeTenantMemberOrAdmin(w, r, true)
	if !ok {
		return
	}
	var input struct {
		Name *string `json:"name"`
		Slug *string `json:"slug"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	result, err := h.service.UpdateMemberTenant(r.Context(), bearer, tenantID, input.Name, input.Slug)
	if err != nil {
		writeProvisionError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (h *Handler) getMemberTenantSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, bearer, ok := h.authorizeTenantMemberOrAdmin(w, r, false)
	if !ok {
		return
	}
	result, err := h.service.GetMemberTenantSettings(r.Context(), bearer, tenantID)
	if err != nil {
		writeProvisionError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (h *Handler) updateMemberTenantSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, bearer, ok := h.authorizeTenantMemberOrAdmin(w, r, true)
	if !ok {
		return
	}
	var input struct {
		Timezone        string `json:"timezone"`
		CurrencyCode    string `json:"currency_code"`
		LocaleCode      string `json:"locale_code"`
		DefaultLanguage string `json:"default_language"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	result, err := h.service.UpdateMemberTenantSettings(r.Context(), bearer, tenantID, input.Timezone, input.CurrencyCode, input.LocaleCode, input.DefaultLanguage)
	if err != nil {
		writeProvisionError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (h *Handler) getMemberTenantBranding(w http.ResponseWriter, r *http.Request) {
	tenantID, bearer, ok := h.authorizeTenantMemberOrAdmin(w, r, false)
	if !ok {
		return
	}
	result, err := h.service.GetMemberTenantBranding(r.Context(), bearer, tenantID)
	if err != nil {
		writeProvisionError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (h *Handler) updateMemberTenantBranding(w http.ResponseWriter, r *http.Request) {
	tenantID, bearer, ok := h.authorizeTenantMemberOrAdmin(w, r, true)
	if !ok {
		return
	}
	var input struct {
		DisplayName    string `json:"display_name"`
		LogoURL        string `json:"logo_url"`
		FaviconURL     string `json:"favicon_url"`
		PrimaryColor   string `json:"primary_color"`
		SecondaryColor string `json:"secondary_color"`
		AccentColor    string `json:"accent_color"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	result, err := h.service.UpdateMemberTenantBranding(r.Context(), bearer, tenantID, provision.TenantBrandingDTO{
		DisplayName:    input.DisplayName,
		LogoURL:        input.LogoURL,
		FaviconURL:     input.FaviconURL,
		PrimaryColor:   input.PrimaryColor,
		SecondaryColor: input.SecondaryColor,
		AccentColor:    input.AccentColor,
	})
	if err != nil {
		writeProvisionError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (h *Handler) getMemberTenantDomains(w http.ResponseWriter, r *http.Request) {
	tenantID, bearer, ok := h.authorizeTenantMemberOrAdmin(w, r, false)
	if !ok {
		return
	}
	domains, err := h.service.GetMemberTenantDomains(r.Context(), bearer, tenantID)
	if err != nil {
		writeProvisionError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"domains": domains})
}

func writeProvisionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, provision.ErrOwnerInvitationNotAccepted):
		apiError(w, http.StatusConflict, "owner_invitation_not_accepted")
	case errors.Is(err, provision.ErrInvalidStateTransition):
		apiError(w, http.StatusConflict, "invalid_state_transition")
	case errors.Is(err, provision.ErrAlreadyExists):
		apiError(w, http.StatusConflict, "already_exists")
	case errors.Is(err, provision.ErrTenantNotFound):
		apiError(w, http.StatusNotFound, "tenant_not_found")
	case errors.Is(err, provision.ErrInvalidArgument):
		apiError(w, http.StatusBadRequest, "invalid_argument")
	case errors.Is(err, provision.ErrPermissionDenied):
		apiError(w, http.StatusForbidden, "permission_denied")
	case errors.Is(err, provision.ErrResourceExhausted):
		apiError(w, http.StatusConflict, "limit_exceeded")
	default:
		apiError(w, http.StatusServiceUnavailable, "operation_unavailable")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, into any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("extra JSON")
	}
	return nil
}

func jsonResponse(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func apiError(w http.ResponseWriter, code int, value string) {
	jsonResponse(w, code, map[string]string{"code": value})
}
