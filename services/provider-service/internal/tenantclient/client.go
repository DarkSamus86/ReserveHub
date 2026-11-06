package tenantclient

import (
	"context"

	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

func NewLookup(client tenantv1.TenantServiceClient, serviceToken string) func(context.Context, uuid.UUID, string) (string, error) {
	return func(ctx context.Context, id uuid.UUID, bearer string) (string, error) {
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+bearer, "x-service-token", serviceToken))
		tenant, err := client.GetTenant(ctx, &tenantv1.GetTenantRequest{TenantId: id.String()})
		if err != nil {
			return "", err
		}
		return tenant.GetStatus(), nil
	}
}
