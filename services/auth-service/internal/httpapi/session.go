package httpapi

import "net/http"

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}
type acceptRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var input refreshRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "valid JSON required")
		return
	}
	result, err := h.service.Refresh(r.Context(), input.RefreshToken)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var input refreshRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "valid JSON required")
		return
	}
	if err := h.service.Logout(r.Context(), input.RefreshToken); err != nil {
		h.writeAuthError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	var input acceptRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "valid JSON required")
		return
	}
	result, err := h.service.AcceptInvitation(r.Context(), input.Token, input.Password)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
