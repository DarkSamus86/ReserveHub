package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"darksamus.com/ReserveHub/tenant-service/internal/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type tenantRequest struct{ id string }

func (r tenantRequest) GetTenantId() string { return r.id }

func TestUnaryInterceptorLogSchema(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger(&output, "tenant-service", "test", "debug")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "request-123"))
	interceptor := observability.UnaryInterceptor(observability.NewMetrics(), logger)
	_, err := interceptor(ctx, tenantRequest{id: "tenant-123"}, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Get"},
		func(context.Context, any) (any, error) { return nil, status.Error(codes.InvalidArgument, "bad input") })
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unexpected error: %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("invalid JSON log: %v", err)
	}
	for key, want := range map[string]any{
		"level": "WARN", "message": "grpc_request", "service": "tenant-service",
		"environment": "test", "request_id": "request-123", "tenant_id": "tenant-123",
		"grpc_method": "/test.Service/Get", "grpc_code": "InvalidArgument",
	} {
		if got := record[key]; got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
	if timestamp, ok := record["timestamp"].(string); !ok || !strings.HasSuffix(timestamp, "Z") {
		t.Error("timestamp must be UTC")
	}
	if _, ok := record["duration_ms"].(float64); !ok {
		t.Error("duration_ms is missing")
	}
}

func TestLoggerFiltersDebug(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger(&output, "tenant-service", "", "info")
	logger.Debug("ignored")
	if output.Len() != 0 {
		t.Error("debug record passed info filter")
	}
	logger.LogAttrs(context.Background(), slog.LevelError, "failed", slog.Any("error", errors.New("failure")))
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("invalid JSON log: %v", err)
	}
	if record["environment"] != "local" || record["error"] != "failure" {
		t.Fatalf("unexpected record: %v", record)
	}
}
