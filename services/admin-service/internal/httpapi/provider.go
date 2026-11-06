package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"darksamus.com/ReserveHub/admin-service/internal/authn"
	providerv1 "darksamus.com/ReserveHub/provider-service/gen/provider/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type providerResponse struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	City      string    `json:"city"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func mapProvider(p *providerv1.Provider) providerResponse {
	response := providerResponse{ID: p.GetId(), TenantID: p.GetTenantId(), Name: p.GetName(), City: p.GetCity(), Status: p.GetStatus()}
	if p.GetCreatedAt() != nil {
		response.CreatedAt = p.GetCreatedAt().AsTime()
	}
	if p.GetUpdatedAt() != nil {
		response.UpdatedAt = p.GetUpdatedAt().AsTime()
	}
	return response
}

func providerError(w http.ResponseWriter, err error) {
	code := http.StatusServiceUnavailable
	key := "unavailable"
	switch status.Code(err) {
	case codes.InvalidArgument:
		code, key = http.StatusBadRequest, "invalid_argument"
	case codes.Unauthenticated:
		code, key = http.StatusUnauthorized, "unauthenticated"
	case codes.PermissionDenied, codes.FailedPrecondition:
		code, key = http.StatusForbidden, "permission_denied"
	case codes.NotFound:
		code, key = http.StatusNotFound, "not_found"
	case codes.AlreadyExists:
		code, key = http.StatusConflict, "conflict"
	}
	jsonResponse(w, code, map[string]string{"code": key, "message": http.StatusText(code)})
}

func (h *Handler) providerContext(w http.ResponseWriter, r *http.Request) (context.Context, context.CancelFunc, string, bool) {
	if h.provider == nil {
		jsonResponse(w, 503, map[string]string{"code": "unavailable", "message": "provider service unavailable"})
		return nil, nil, "", false
	}
	_, bearer, ok := authn.PlatformAdminFromRequest(r, h.publicKey, time.Now())
	if !ok {
		jsonResponse(w, 403, map[string]string{"code": "forbidden", "message": "platform administrator required"})
		return nil, nil, "", false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+bearer, "x-service-token", h.providerToken))
	return ctx, cancel, bearer, true
}

func providerID(w http.ResponseWriter, raw string) (string, bool) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		jsonResponse(w, 400, map[string]string{"code": "invalid_request", "message": "canonical UUID required"})
		return "", false
	}
	return raw, true
}

func (h *Handler) createProvider(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, _, ok := h.providerContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	tenant, ok := providerID(w, r.PathValue("tenant_id"))
	if !ok {
		return
	}
	values := r.Header.Values("Idempotency-Key")
	if len(values) != 1 {
		jsonResponse(w, 400, map[string]string{"code": "invalid_request", "message": "UUID Idempotency-Key required"})
		return
	}
	operation, ok := providerID(w, values[0])
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
		City string `json:"city"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		jsonResponse(w, 400, map[string]string{"code": "invalid_request", "message": "invalid JSON body"})
		return
	}
	result, err := h.provider.CreateProvider(ctx, &providerv1.CreateProviderRequest{TenantId: tenant, OperationId: operation, Name: body.Name, City: body.City})
	if err != nil {
		providerError(w, err)
		return
	}
	code := 201
	if result.GetReplayed() {
		code = 200
	} else {
		w.Header().Set("Location", "/v1/admin/tenants/"+tenant+"/providers/"+result.GetProvider().GetId())
	}
	jsonResponse(w, code, mapProvider(result.GetProvider()))
}

func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, _, ok := h.providerContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	tenant, ok := providerID(w, r.PathValue("tenant_id"))
	if !ok {
		return
	}
	query := r.URL.Query()
	for key := range query {
		if key != "page_size" && key != "page_token" {
			jsonResponse(w, 400, map[string]string{"code": "invalid_request", "message": "unsupported query parameter"})
			return
		}
	}
	var pageSize int32
	if raw := query.Get("page_size"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || n < 1 || n > 100 {
			jsonResponse(w, 400, map[string]string{"code": "invalid_request", "message": "invalid page_size"})
			return
		}
		pageSize = int32(n)
	}
	result, err := h.provider.ListProviders(ctx, &providerv1.ListProvidersRequest{TenantId: tenant, PageSize: pageSize, PageToken: query.Get("page_token")})
	if err != nil {
		providerError(w, err)
		return
	}
	items := make([]providerResponse, 0, len(result.GetProviders()))
	for _, p := range result.GetProviders() {
		items = append(items, mapProvider(p))
	}
	jsonResponse(w, 200, map[string]any{"providers": items, "next_page_token": result.GetNextPageToken()})
}

func (h *Handler) getProvider(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, _, ok := h.providerContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	tenant, ok := providerID(w, r.PathValue("tenant_id"))
	if !ok {
		return
	}
	id, ok := providerID(w, r.PathValue("provider_id"))
	if !ok {
		return
	}
	result, err := h.provider.GetProvider(ctx, &providerv1.GetProviderRequest{TenantId: tenant, ProviderId: id})
	if err != nil {
		providerError(w, err)
		return
	}
	jsonResponse(w, 200, mapProvider(result))
}

func (h *Handler) changeProviderStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, _, ok := h.providerContext(w, r)
	if !ok {
		return
	}
	defer cancel()
	tenant, ok := providerID(w, r.PathValue("tenant_id"))
	if !ok {
		return
	}
	id, ok := providerID(w, r.PathValue("provider_id"))
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		jsonResponse(w, 400, map[string]string{"code": "invalid_request", "message": "invalid JSON body"})
		return
	}
	result, err := h.provider.ChangeProviderStatus(ctx, &providerv1.ChangeProviderStatusRequest{TenantId: tenant, ProviderId: id, Status: strings.TrimSpace(body.Status)})
	if err != nil {
		providerError(w, err)
		return
	}
	jsonResponse(w, 200, mapProvider(result))
}
