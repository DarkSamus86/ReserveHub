package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain valid email and password JSON")
		return
	}
	session, err := h.service.Register(r.Context(), input.Email, input.Password)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, session)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain valid email and password JSON")
		return
	}
	session, err := h.service.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "valid bearer token required")
		return
	}
	user, err := h.service.Me(r.Context(), parts[1])
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "email is already registered")
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "invalid email, password, or access token")
	case errors.Is(err, auth.ErrInvalidSecret):
		writeError(w, http.StatusUnauthorized, "invalid_secret", "secret is invalid or expired")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "request could not be completed")
	}
}
