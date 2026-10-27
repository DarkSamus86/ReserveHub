# tenant-service

Internal gRPC service that owns tenant lifecycle and configuration for ReserveHub.

Logs follow the [ReserveHub JSON logging convention](../../docs/observability/logging.md).

## Local run

From the repository root, generate the Ed25519 key pair using the commands in `.env.example`, then run:

```bash
docker compose up --build tenant-service
```

Tenant gRPC requests require a bearer JWT signed by Auth, with the tenant audience and a platform-scoped admin role; health checks are public. Tenant-scoped admins and regular users are denied until tenant membership and scoped RPC authorization are implemented.

Domain verification requires a DNS TXT record at `_reservehub-verification.<domain>` whose value exactly matches the challenge returned by `AddTenantDomain`. After publishing the record, call `VerifyTenantDomain`; the service resolves DNS itself. A missing or mismatched proof returns a domain with status `FAILED` and emits `tenant.domain.verification_failed`. A resolver outage returns gRPC `Unavailable` without changing the domain or emitting an event, so the request can be retried. The challenge sent back to the API is not proof of ownership by itself.

Plan changes may be scheduled only after the latest assignment. Conflicting or retroactive changes are rejected so completed plan history is retained. Tenant-specific writes are rejected while a tenant is suspended; resuming or archiving it remains possible.

Internal endpoints:

- gRPC: `tenant-service:8081` on the private `tenant-internal` Compose network.
- Prometheus metrics and health checks listen on port `9091` inside the container and are not published to the host.

gRPC reflection is enabled only when `APP_ENV=local`.

Tenant is not exposed through the public API gateway. A trusted internal caller such as a future `admin-service` should call it over gRPC and pass the caller's bearer token for Tenant's own authentication and authorization checks. Attach only authorized internal services to `tenant-internal`.

PostgreSQL migrations run before the application container. Tenant tables and the
`sys_schema_migration` version table live in the `tenant_service` schema of the
shared `reservehub` database.

The initial data includes the plan rules explicitly defined by the product
specification:

- `BASIC`: `MAX_DOMAINS=1`, `CUSTOM_DOMAIN=false`;
- `BUSINESS`: `MAX_DOMAINS=5`, `CUSTOM_DOMAIN=true`.

Other plan limits and features must be configured by later platform
configuration migrations or by tenant overrides.

## Development checks

```bash
go test ./...
go vet ./...
```

The PostgreSQL integration test is opt-in:

```bash
TEST_DATABASE_URL='postgres://reservehub:reservehub@localhost:5432/reservehub_test?sslmode=disable&search_path=tenant_service' \
  go test -tags=integration ./tests
```

## Protobuf generation

Install `protoc-gen-go` and `protoc-gen-go-grpc`, then run:

```bash
protoc -I proto \
  --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
  proto/tenant/v1/tenant.proto
```
