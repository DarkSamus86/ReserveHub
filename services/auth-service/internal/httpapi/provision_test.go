package httpapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
)

const (
	testTenantID     = "11111111-1111-4111-8111-111111111111"
	testOperationID  = "22222222-2222-4222-8222-222222222222"
	testServiceToken = "a-private-service-token-with-at-least-32-bytes"
)

type provisionMemoryRepo struct {
	auth.Repository
	provisionResult auth.ProvisionOwnerResult
	provisionErr    error
	provisionCalls  int
}

func (r *provisionMemoryRepo) ProvisionOwner(_ context.Context, _ auth.ProvisionOwnerInput) (auth.ProvisionOwnerResult, error) {
	r.provisionCalls++
	return r.provisionResult, r.provisionErr
}

func TestProvisionInternalHTTPAuthorizationAndResponse(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	repo := &provisionMemoryRepo{provisionResult: auth.ProvisionOwnerResult{
		TenantID: testTenantID, UserID: "user-id", MembershipID: "membership-id",
		InvitationID: "invitation-id", Status: "pending_delivery", Created: true,
	}}
	service := auth.NewService(repo, privateKey, time.Minute)
	handler := New(service, slog.New(slog.NewTextHandler(io.Discard, nil)), testServiceToken)
	body, err := json.Marshal(auth.ProvisionOwnerInput{
		TenantID: testTenantID, Slug: "9-" + strings.Repeat("a", 98),
		Email: " Owner@Example.com ", OperationID: testOperationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := func(token string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/internal/v1/tenant-memberships/provision-owner", bytes.NewReader(body))
		if token != "" {
			r.Header.Set("X-Service-Token", token)
		}
		return r
	}
	for _, token := range []string{"", "wrong"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, request(token))
		if w.Code != http.StatusUnauthorized || repo.provisionCalls != 0 {
			t.Fatalf("token %q: status=%d, calls=%d", token, w.Code, repo.provisionCalls)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request(testServiceToken))
	if w.Code != http.StatusCreated || repo.provisionCalls != 1 {
		t.Fatalf("provision status=%d, calls=%d, body=%s", w.Code, repo.provisionCalls, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "email") || strings.Contains(w.Body.String(), "token") || strings.Contains(w.Body.String(), "created") {
		t.Fatalf("provision response exposes private/internal values: %s", w.Body.String())
	}
	repo.provisionResult.Created = false
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request(testServiceToken))
	if w.Code != http.StatusOK {
		t.Fatalf("replay status=%d", w.Code)
	}
	repo.provisionErr = auth.ErrProvisionConflict
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request(testServiceToken))
	if w.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d", w.Code)
	}
}
