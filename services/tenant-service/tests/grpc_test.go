package tests

import (
	"context"
	"testing"

	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	transportgrpc "darksamus.com/ReserveHub/tenant-service/internal/transport/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetTenantRejectsInvalidUUID(t *testing.T) {
	t.Parallel()

	server := transportgrpc.New(nil)
	_, err := server.GetTenant(context.Background(), &tenantv1.GetTenantRequest{TenantId: "not-a-uuid"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status code = %s, want %s; error = %v", status.Code(err), codes.InvalidArgument, err)
	}
}

func TestListTenantsRejectsInvalidPageToken(t *testing.T) {
	t.Parallel()

	server := transportgrpc.New(nil)
	_, err := server.ListTenants(context.Background(), &tenantv1.ListTenantsRequest{PageToken: "%%%"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status code = %s, want %s; error = %v", status.Code(err), codes.InvalidArgument, err)
	}
}

func TestGRPCContractContainsMVPMethods(t *testing.T) {
	t.Parallel()

	service := tenantv1.File_tenant_v1_tenant_proto.Services().ByName("TenantService")
	if service == nil {
		t.Fatal("TenantService descriptor is missing")
	}
	if got, want := service.Methods().Len(), 27; got != want {
		t.Fatalf("method count = %d, want %d", got, want)
	}
}
