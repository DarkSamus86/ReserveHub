# Workspace service

The Workspace facade serves tenant-owner Provider operations. Browser requests enter through Gateway; Workspace checks the signed tenant session and tenant ID, then calls Provider over private gRPC with the user's bearer token and its own workload credential. Provider repeats authorization and tenant lifecycle checks. Workspace does not own Provider data or user credentials.

Routes: `GET /v1/tenants/{tenant_id}/providers`, `GET /v1/tenants/{tenant_id}/providers/{provider_id}`, and `PATCH /v1/tenants/{tenant_id}/providers/{provider_id}`. The list accepts `page_size` and `page_token`; profile PATCH accepts optional `name` and `city`. The current stage D contract permits `tenant_owner` only. Provider and venue staff grants are planned separately in the [access-management STPO](<../../docs/99 - Источники/services/access-management/ReserveHub-Access-Management-STPO.md>).

Set `PROVIDER_GRPC_ADDRESS`, `AUTH_JWT_PUBLIC_KEY_FILE`, and `PROVIDER_WORKSPACE_SERVICE_TOKEN_FILE`; `HTTP_ADDRESS` defaults to `:8085`. No host port is published for this service in Compose.
