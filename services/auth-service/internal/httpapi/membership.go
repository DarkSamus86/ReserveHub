package httpapi

import (
	"net/http"
	"strings"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
)

type tenantSessionRequest struct {
	TenantID string `json:"tenant_id"`
}

func bearerFromRequest(r *http.Request) (string, bool) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func (h *Handler) listMemberships(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "valid bearer token required")
		return
	}
	memberships, err := h.service.ListMemberships(r.Context(), token)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"memberships": memberships})
}

func (h *Handler) exchangeTenantSession(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "valid bearer token required")
		return
	}
	var input tenantSessionRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "valid JSON required")
		return
	}
	session, err := h.service.ExchangeTenantSession(r.Context(), token, input.TenantID)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) listTenantMembers(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "valid bearer token required")
		return
	}
	members, err := h.service.ListTenantMembers(r.Context(), token, r.PathValue("tenant_id"))
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (h *Handler) inviteTenantMember(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "valid bearer token required")
		return
	}
	var input auth.InviteMemberInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "valid JSON required")
		return
	}
	result, err := h.service.InviteTenantMember(r.Context(), token, r.PathValue("tenant_id"), input)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) revokeTenantMember(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "valid bearer token required")
		return
	}
	var empty struct{}
	if err := decodeJSON(w, r, &empty); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "valid JSON required")
		return
	}
	if err := h.service.RevokeTenantMember(r.Context(), token, r.PathValue("tenant_id"), r.PathValue("membership_id")); err != nil {
		h.writeAuthError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
