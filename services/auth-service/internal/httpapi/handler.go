package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
)

type Handler struct {
	service *auth.Service
}

func New(service *auth.Service, logger *slog.Logger) http.Handler {
	h := &Handler{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/auth/register", h.register)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("GET /v1/auth/me", h.me)
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health/ready", h.ready)
	return LoggingMiddleware(logger, mux)
}

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

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Ready(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "email is already registered")
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "invalid email, password, or access token")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "request could not be completed")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON value")
	}
	return nil
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiError{Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
