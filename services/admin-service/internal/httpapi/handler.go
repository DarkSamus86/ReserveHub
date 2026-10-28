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
	mux.HandleFunc("GET /v1/admin/tenant-provisioning/{operation_id}", h.getOperation)
	mux.HandleFunc("POST /v1/admin/tenant-provisioning/{operation_id}/reissue-invitation", h.reissueInvitation)
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
