# tenant-service

PostgreSQL tables use the `tenant` schema in the shared `reservehub` database. For upgrades from `tenant_service`, follow the [schema rename procedure](../../scripts/db/README.md).

Internal gRPC service that owns tenant lifecycle and configuration for ReserveHub.

Logs follow the [ReserveHub JSON logging convention](<../../docs/99 - Источники/observability/logging.md>).

## Local run

From the repository root, generate the Ed25519 key pair using the commands in `.env.example`, then run:

```bash
docker compose up --build tenant-service
```

Tenant gRPC requests require a bearer JWT signed by Auth with the `reservehub-tenant` audience (`health` checks are public):
- **Platform Admin** (`scope: "platform"`, `role: "admin"`): full access to all RPCs (`CreateTenant`, `ListTenants`, lifecycle transitions, plan/entitlement management, and tenant reads/writes).
- **Tenant Owner** (`scope: "tenant"`, `role: "user"`, `permission_profile: "tenant_owner"`): read and write access (`GetTenant`, `UpdateTenant`, `GetTenantSettings`, `UpdateTenantSettings`, `GetTenantBranding`, `UpdateTenantBranding`, `GetTenantDomains`, `AddTenantDomain`, `VerifyTenantDomain`, `RemoveTenantDomain`, `GetActiveTenantPlan`, `GetTenantEntitlements`) strictly when `req.GetTenantId() == claims.TenantID`.
- **Tenant Viewer** (`scope: "tenant"`, `role: "user"`, `permission_profile: "tenant_viewer"`): read-only access (`GetTenant`, `GetTenantSettings`, `GetTenantBranding`, `GetTenantDomains`, `GetActiveTenantPlan`, `GetTenantEntitlements`) strictly when `req.GetTenantId() == claims.TenantID`.

Tenant RPCs additionally require `x-service-token` from `PLATFORM_SERVICE_TOKEN_FILE` in the deployed service. `CreateTenant` accepts an optional UUID `operation_id`. Repeating the same operation and tenant data returns the original Tenant, including after a lost response; changing the data for that operation returns `AlreadyExists`.

Domain verification requires a DNS TXT record at `_reservehub-verification.<domain>` whose value exactly matches the challenge returned by `AddTenantDomain`. After publishing the record, call `VerifyTenantDomain`; the service resolves DNS itself. A missing or mismatched proof returns a domain with status `FAILED` and emits `tenant.domain.verification_failed`. A resolver outage returns gRPC `Unavailable` without changing the domain or emitting an event, so the request can be retried. The challenge sent back to the API is not proof of ownership by itself.

Plan changes may be scheduled only after the latest assignment. Conflicting or retroactive changes are rejected so completed plan history is retained.
Lifecycle enforcement:
- `PENDING`: tenant-scoped member writes are rejected (`FailedPrecondition`) until the tenant is activated; member reads are allowed.
- `SUSPENDED`: all tenant configuration writes are rejected (`FailedPrecondition`); member reads and platform `ResumeTenant`/`ArchiveTenant` remain allowed.
- `ARCHIVED`: terminal state; all writes and all tenant-scoped member reads are rejected (`FailedPrecondition`).

Internal endpoints:

- gRPC: `tenant-service:8081` on the private `tenant-internal` Compose network.
- Prometheus metrics and health checks listen on port `9091` inside the container and are not published to the host.

gRPC reflection is enabled only when `APP_ENV=local`.

Tenant is not exposed directly through the public API gateway. `admin-service` proxies authorized platform admin and tenant member HTTP requests to `tenant-service` over gRPC with the caller's bearer token and its internal service credential. `provider-service` calls `GetTenant` over gRPC with the same credential pattern to check tenant existence and lifecycle state before provider operations. Attach only authorized internal services to `tenant-internal`.

PostgreSQL migrations run before the application container. Tenant tables and the
`sys_schema_migration` version table live in the `tenant` schema of the
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
TEST_DATABASE_URL='postgres://reservehub:reservehub@localhost:5432/reservehub_test?sslmode=disable&search_path=tenant' \
  go test -tags=integration ./internal/repository
```

## Protobuf generation

Install `protoc-gen-go` and `protoc-gen-go-grpc`, then run:

```bash
protoc -I proto \
  --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
  proto/tenant/v1/tenant.proto
```
