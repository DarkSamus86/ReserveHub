package main

import (
	"context"
	"testing"

	providerv1 "darksamus.com/ReserveHub/provider-service/gen/provider/v1"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestProviderNamedHealthStatus(t *testing.T) {
	h := health.NewServer()
	name := providerv1.ProviderService_ServiceDesc.ServiceName
	for _, want := range []grpc_health_v1.HealthCheckResponse_ServingStatus{grpc_health_v1.HealthCheckResponse_SERVING, grpc_health_v1.HealthCheckResponse_NOT_SERVING} {
		setHealthStatus(h, want)
		response, err := h.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: name})
		if err != nil || response.GetStatus() != want {
			t.Fatalf("named health: got %v, err %v; want %v", response, err, want)
		}
	}
}
