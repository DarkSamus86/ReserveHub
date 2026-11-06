package httpapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	providerv1 "darksamus.com/ReserveHub/provider-service/gen/provider/v1"
	"darksamus.com/ReserveHub/workspace-service/internal/authn"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Handler struct {
	Provider      providerv1.ProviderServiceClient
	ProviderToken string
	PublicKey     ed25519.PublicKey
	Ready         func(context.Context) error
}

func (h Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) { jsonResponse(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if h.Ready != nil && h.Ready(r.Context()) != nil {
			writeError(w, 503, "unavailable", "provider service unavailable")
			return
		}
		jsonResponse(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /v1/tenants/{tenant_id}/providers", h.list)
	mux.HandleFunc("GET /v1/tenants/{tenant_id}/providers/{provider_id}", h.get)
	mux.HandleFunc("PATCH /v1/tenants/{tenant_id}/providers/{provider_id}", h.update)
	return mux
}

func (h Handler) context(w http.ResponseWriter, r *http.Request) (context.Context, context.CancelFunc, string, bool) {
	member, bearer, ok := authn.TenantMemberFromRequest(r, h.PublicKey, time.Now())
	if !ok {
		writeError(w, 401, "unauthenticated", "invalid token")
		return nil, nil, "", false
	}
	if member.PermissionProfile != authn.ProfileTenantOwner {
		writeError(w, 403, "forbidden", "tenant owner required")
		return nil, nil, "", false
	}
	tenantID, ok := canonicalID(r.PathValue("tenant_id"))
	if !ok {
		writeError(w, 400, "invalid_request", "tenant_id must be a canonical UUID")
		return nil, nil, "", false
	}
	if member.TenantID != tenantID {
		writeError(w, 403, "forbidden", "cross-tenant access denied")
		return nil, nil, "", false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+bearer, "x-service-token", h.ProviderToken))
	return ctx, cancel, tenantID.String(), true
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, tenant, ok := h.context(w, r)
	if !ok {
		return
	}
	defer cancel()
	query := r.URL.Query()
	for key := range query {
		if key != "page_size" && key != "page_token" {
			writeError(w, 400, "invalid_request", "unsupported query parameter")
			return
		}
	}
	var size int32
	if raw := query.Get("page_size"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || n < 1 || n > 100 {
			writeError(w, 400, "invalid_request", "invalid page_size")
			return
		}
		size = int32(n)
	}
	result, err := h.Provider.ListProviders(ctx, &providerv1.ListProvidersRequest{TenantId: tenant, PageSize: size, PageToken: query.Get("page_token")})
	if err != nil {
		rpcError(w, err)
		return
	}
	items := make([]providerResponse, 0, len(result.GetProviders()))
	for _, p := range result.GetProviders() {
		items = append(items, mapProvider(p))
	}
	jsonResponse(w, 200, map[string]any{"providers": items, "next_page_token": result.GetNextPageToken()})
}

func (h Handler) get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, tenant, ok := h.context(w, r)
	if !ok {
		return
	}
	defer cancel()
	id, ok := canonicalID(r.PathValue("provider_id"))
	if !ok {
		writeError(w, 400, "invalid_request", "provider_id must be a canonical UUID")
		return
	}
	result, err := h.Provider.GetProvider(ctx, &providerv1.GetProviderRequest{TenantId: tenant, ProviderId: id.String()})
	if err != nil {
		rpcError(w, err)
		return
	}
	jsonResponse(w, 200, mapProvider(result))
}

func (h Handler) update(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, tenant, ok := h.context(w, r)
	if !ok {
		return
	}
	defer cancel()
	id, ok := canonicalID(r.PathValue("provider_id"))
	if !ok {
		writeError(w, 400, "invalid_request", "provider_id must be a canonical UUID")
		return
	}
	var body struct {
		Name *string `json:"name"`
		City *string `json:"city"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, 400, "invalid_request", "invalid JSON body")
		return
	}
	result, err := h.Provider.UpdateProviderProfile(ctx, &providerv1.UpdateProviderProfileRequest{TenantId: tenant, ProviderId: id.String(), Name: body.Name, City: body.City})
	if err != nil {
		rpcError(w, err)
		return
	}
	jsonResponse(w, 200, mapProvider(result))
}

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
	v := providerResponse{ID: p.GetId(), TenantID: p.GetTenantId(), Name: p.GetName(), City: p.GetCity(), Status: p.GetStatus()}
	if p.GetCreatedAt() != nil {
		v.CreatedAt = p.GetCreatedAt().AsTime()
	}
	if p.GetUpdatedAt() != nil {
		v.UpdatedAt = p.GetUpdatedAt().AsTime()
	}
	return v
}
func canonicalID(raw string) (uuid.UUID, bool) {
	id, err := uuid.Parse(raw)
	return id, err == nil && id != uuid.Nil && id.String() == raw
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
func rpcError(w http.ResponseWriter, err error) {
	code := 503
	key := "unavailable"
	switch status.Code(err) {
	case codes.InvalidArgument:
		code, key = 400, "invalid_argument"
	case codes.Unauthenticated:
		code, key = 401, "unauthenticated"
	case codes.PermissionDenied, codes.FailedPrecondition:
		code, key = 403, "permission_denied"
	case codes.NotFound:
		code, key = 404, "not_found"
	case codes.AlreadyExists:
		code, key = 409, "conflict"
	}
	writeError(w, code, key, http.StatusText(code))
}
func writeError(w http.ResponseWriter, code int, key, message string) {
	jsonResponse(w, code, map[string]string{"code": key, "message": message})
}
func jsonResponse(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
