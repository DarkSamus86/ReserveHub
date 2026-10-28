package httpapi

import (
	"log/slog"
	"net/http"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
)

type Handler struct {
	service               *auth.Service
	provisionServiceToken string
}

func New(service *auth.Service, logger *slog.Logger, provisionServiceToken string) http.Handler {
	h := &Handler{service: service, provisionServiceToken: provisionServiceToken}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/auth/register", h.register)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("GET /v1/auth/me", h.me)
	mux.HandleFunc("POST /v1/auth/invitations/accept", h.acceptInvitation)
	mux.HandleFunc("POST /v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)
	mux.HandleFunc("POST /internal/v1/tenant-memberships/provision-owner", h.provisionOwner)
	mux.HandleFunc("POST /internal/v1/tenant-invitations/{id}/reissue", h.reissueInvitation)
	mux.HandleFunc("GET /internal/v1/tenant-invitations/{id}", h.invitationStatus)
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health/ready", h.ready)
	return LoggingMiddleware(logger, mux)
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Ready(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
