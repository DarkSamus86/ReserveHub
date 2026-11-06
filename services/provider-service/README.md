# Provider service

Provider owns a tenant's city-level operating units. Its business API is private gRPC for the platform Admin and tenant Workspace backends; browsers use the public Gateway and those facades. Provider checks every operation against Tenant's `GetTenant` gRPC API using the caller's bearer token and an internal service credential. Its database schema is `provider`; it does not own venues, seats, sales, or tenant records.

## Code structure

| Package | Responsibility |
| --- | --- |
| `cmd/provider-service` | Process startup, dependency wiring, and shutdown |
| `internal/domain` | Provider model, storage port, and domain errors |
| `internal/application` | Tenant lifecycle policy, create idempotency input, and provider use cases |
| `proto/provider/v1`, `gen/provider/v1` | Source gRPC contract and generated Go code |
| `internal/grpcapi` | gRPC authorization, request validation, response mapping |
| `internal/authn` | JWT validation and caller identity |
| `internal/repository` | PostgreSQL transactions, idempotency records, and outbox persistence |
| `internal/tenantclient` | Tenant gRPC adapter with workload and user credentials |
| `internal/outbox` | Audit event and publisher storage port |
| `internal/messaging` | NATS JetStream publisher |
| `internal/config` | Environment and secret-file loading |

Dependencies point inward from transport and adapters to application ports and domain types. `cmd/provider-service` composes them; packages under `internal` do not import the command package.

## API

The private `ProviderService` gRPC contract is defined in `proto/provider/v1/provider.proto`. Platform administrators can call `CreateProvider`, `ListProviders`, `GetProvider`, and `ChangeProviderStatus` through the Admin backend. Creation requires a UUID `operation_id`. The response has a `replayed` flag so the HTTP facade can return 201 for the first create and 200 for an identical replay; conflicting reuse maps to 409.

Tenant owners can call `ListProviders`, `GetProvider`, and `UpdateProviderProfile` through the Workspace backend for their own tenant. List accepts `page_size` (1–100) and `page_token`. Provider responses contain `id`, `tenant_id`, `name`, `city`, `status`, `created_at`, and `updated_at`. Status can be `ACTIVE` or `SUSPENDED`. Tenant viewers and unassigned staff have no Provider access in the current stage D contract.

Every business RPC requires `authorization: Bearer <Auth JWT>` and `x-service-token`. Admin and Workspace use distinct service credentials. Provider verifies both credentials and the caller role, checks method and tenant scope, and derives the audit actor from the verified JWT. Admin and Workspace tokens cannot invoke each other's write operations. No Provider business port is published to the host.

All writes require an ACTIVE tenant. A tenant owner can read providers when the tenant is PENDING or SUSPENDED, but not ARCHIVED. Platform administrators may read an archived tenant. Updating a provider's profile while that provider is SUSPENDED is allowed if its tenant is ACTIVE, so the owner can correct its details. Provider status changes to the current state are no-ops. Tenant ID cannot be changed after creation.

Create and status changes write audit outbox events in the same database transaction. A background publisher sends them to the JetStream subject `provider.audit.v1`; consumers should deduplicate by event ID. Audit payloads contain identifiers and lifecycle state, not personal data.

## Runtime

Set `DATABASE_URL`, `TENANT_GRPC_ADDRESS`, `NATS_URL`, `AUTH_JWT_PUBLIC_KEY_FILE`, `PLATFORM_SERVICE_TOKEN_FILE` (outbound Tenant credential), `PROVIDER_ADMIN_SERVICE_TOKEN_FILE`, and `PROVIDER_WORKSPACE_SERVICE_TOKEN_FILE`. All three token files must have distinct values of at least 32 bytes. Optional settings are `GRPC_ADDRESS` (default `:8084`), `METRICS_ADDRESS` (default `:9094`), and `OUTBOX_INTERVAL` (default `1s`). Compose runs schema setup and migrations before starting the service. HTTP `/health/live` and `/health/ready` on the metrics listener are internal health endpoints only; standard gRPC health is also registered.

Regenerate the gRPC bindings from the service module directory:

```sh
protoc -I proto --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative proto/provider/v1/provider.proto
```
