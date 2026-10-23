package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"darksamus.com/ReserveHub/auth-service/internal/httpapi"
	"darksamus.com/ReserveHub/auth-service/internal/observability"
)

func TestHTTPLoggingUsesReserveHubSchema(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger(&output, "auth-service", "test", "debug")
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"secret@example.com","password":"do-not-log-this"}`))
	request.Header.Set("X-Request-ID", "auth-request-123")
	response := httptest.NewRecorder()
	httpapi.LoggingMiddleware(logger, mux).ServeHTTP(response, request)

	if got := response.Header().Get("X-Request-ID"); got != "auth-request-123" {
		t.Fatalf("response request ID = %q", got)
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("invalid JSON log: %v", err)
	}
	for key, want := range map[string]any{
		"level": "WARN", "message": "http_request", "service": "auth-service", "environment": "test",
		"request_id": "auth-request-123", "http_method": "POST", "http_route": "POST /v1/auth/login", "http_status": float64(http.StatusUnauthorized),
	} {
		if got := record[key]; got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
	if _, ok := record["duration_ms"].(float64); !ok {
		t.Error("duration_ms missing")
	}
	if _, ok := record["timestamp"].(string); !ok {
		t.Error("timestamp missing")
	}
	if _, exists := record["time"]; exists {
		t.Error("nonstandard time field found")
	}
	if _, exists := record["msg"]; exists {
		t.Error("nonstandard msg field found")
	}
	if strings.Contains(output.String(), "secret@example.com") || strings.Contains(output.String(), "do-not-log-this") {
		t.Error("authentication credentials leaked into logs")
	}
}

func TestHTTPLoggingReplacesInvalidRequestID(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger(&output, "auth-service", "test", "info")
	handler := httpapi.LoggingMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	request.Header.Set("X-Request-ID", "not a safe id")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if got := response.Header().Get("X-Request-ID"); got == "not a safe id" || got == "" {
		t.Fatalf("unsafe request ID was not replaced: %q", got)
	}
}
