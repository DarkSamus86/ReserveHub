# tenant-service

Internal gRPC service that owns tenant lifecycle and configuration for ReserveHub.

Logs follow the [ReserveHub JSON logging convention](../../docs/observability/logging.md).

## Local run

From the repository root, set `AUTH_JWT_SECRET` in `.env` to a random secret of at least 32 bytes (for example, `openssl rand -hex 32`), then run:

```bash
docker compose up --build tenant-service
```

Tenant gRPC requests require a bearer JWT from `auth-service`; health checks are public. Admin tokens can call all tenant RPCs. Regular user tokens are currently denied tenant RPC access until tenant membership and scoped read queries are implemented.

Endpoints:

- gRPC: `localhost:8081`
- Prometheus metrics: `http://localhost:9091/metrics`
- liveness: `http://localhost:9091/health/live`
- readiness: `http://localhost:9091/health/ready`

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
