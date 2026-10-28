package httpapi

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
)

func (h *Handler) provisionOwner(w http.ResponseWriter, r *http.Request) {
	if h.provisionServiceToken == "" {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "provisioning service is unavailable")
		return
	}
	values := r.Header.Values("X-Service-Token")
	if len(values) != 1 || subtle.ConstantTimeCompare([]byte(values[0]), []byte(h.provisionServiceToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "valid service credentials required")
		return
	}
	var input auth.ProvisionOwnerInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain valid provisioning JSON")
		return
	}
	result, err := h.service.ProvisionOwner(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		case errors.Is(err, auth.ErrProvisionConflict):
			writeError(w, http.StatusConflict, "provision_conflict", "provisioning operation conflicts with an existing request")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "provisioning could not be completed")
		}
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, result)
}

func (h *Handler) reissueInvitation(w http.ResponseWriter, r *http.Request) {
	if h.provisionServiceToken == "" {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "provisioning service is unavailable")
		return
	}
	values := r.Header.Values("X-Service-Token")
	if len(values) != 1 || subtle.ConstantTimeCompare([]byte(values[0]), []byte(h.provisionServiceToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "valid service credentials required")
		return
	}
	if err := h.service.ReissueInvitation(r.Context(), r.PathValue("id")); err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "invalid_input", "invalid invitation ID")
		case errors.Is(err, auth.ErrProvisionConflict), errors.Is(err, auth.ErrInvalidSecret):
			writeError(w, http.StatusConflict, "reissue_unavailable", "invitation cannot be reissued")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "reissue could not be completed")
		}
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) invitationStatus(w http.ResponseWriter, r *http.Request) {
	if h.provisionServiceToken == "" {
		writeError(w, http.StatusServiceUnavailable, "not_configured", "provisioning service is unavailable")
		return
	}
	values := r.Header.Values("X-Service-Token")
	if len(values) != 1 || subtle.ConstantTimeCompare([]byte(values[0]), []byte(h.provisionServiceToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "valid service credentials required")
		return
	}
	result, err := h.service.InvitationStatus(r.Context(), r.PathValue("id"))
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "invalid_input", "invalid invitation ID")
		case errors.Is(err, auth.ErrInvitationNotFound):
			writeError(w, http.StatusNotFound, "not_found", "invitation not found")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "status could not be loaded")
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}
