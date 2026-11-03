# admin-service

This internal platform backend coordinates company creation, platform tenant lifecycle management, and tenant-scoped configuration access. It is the only caller of `tenant-service` gRPC in the Compose flow. Gateway exposes its routes but does not join Tenant's private network.

## Platform Admin Endpoints (`scope: "platform"`, `role: "admin"`)

- `POST /v1/admin/tenants` — requires a platform admin bearer token and a UUID `Idempotency-Key` header. The JSON body contains `name`, `slug`, `timezone`, `currency_code`, `locale_code`, `default_language`, `plan_id`, and `owner_email`. A successful response contains `operation_id`, `tenant_id`, `status: completed`, and `invite_status: pending_delivery` (never the invitation secret or owner email).
- `GET /v1/admin/tenants` — lists tenants from `tenant-service` (`page_size`, `page_token`, `status`) enriched with provisioning and live invitation status metadata (`operation_id`, `provisioning_status`, `invite_status`, `invite_expires_at`, `invite_status_unavailable`).
- `GET /v1/admin/tenants/{tenant_id}` — returns the full administrative tenant card (`tenant`, `settings`, `branding`, `domains`, `provisioning`) without exposing `owner_email`.
- `POST /v1/admin/tenants/{tenant_id}/activate` — transitions a `PENDING` tenant to `ACTIVE` after enforcing the **Activation Gate**: the tenant's provisioning operation must be `completed` and Auth must report `invite_status: accepted` (otherwise returns `409 Conflict` with `code: "owner_invitation_not_accepted"`, or `503 Service Unavailable` if Auth is unreachable).
- `POST /v1/admin/tenants/{tenant_id}/suspend` — suspends an `ACTIVE` tenant.
- `POST /v1/admin/tenants/{tenant_id}/resume` — resumes a `SUSPENDED` tenant to `ACTIVE`.
- `POST /v1/admin/tenants/{tenant_id}/archive` — archives a tenant (terminal state).
- `GET /v1/admin/tenant-provisioning/{operation_id}` — reads the persisted provisioning result and asks Auth for the current invitation status and expiry. If Auth is unavailable, returns the saved status with `invite_status_unavailable: true`.
- `POST /v1/admin/tenant-provisioning/{operation_id}/reissue-invitation` — reissues an expired `ready_to_send` or `sent` owner invitation (`202 Accepted`).

## Tenant Member Endpoints (`scope: "tenant"`, matching `tenant_id`)

- `GET /v1/tenants/{tenant_id}` (`tenant_owner`, `tenant_viewer`)
- `PATCH /v1/tenants/{tenant_id}` (`tenant_owner`)
- `GET /v1/tenants/{tenant_id}/settings` (`tenant_owner`, `tenant_viewer`)
- `PUT /v1/tenants/{tenant_id}/settings` (`tenant_owner`)
- `GET /v1/tenants/{tenant_id}/branding` (`tenant_owner`, `tenant_viewer`)
- `PUT /v1/tenants/{tenant_id}/branding` (`tenant_owner`)
- `GET /v1/tenants/{tenant_id}/domains` (`tenant_owner`, `tenant_viewer`)

Code lives in `internal/config` (configuration and secrets), `internal/authn` (admin and tenant member JWT verification), `internal/httpapi` (routes and validation), `internal/provision` (Tenant/Auth orchestration and lifecycle), `internal/repository` (operation and outbox persistence), and `internal/messaging` (audit publication). `cmd/admin-service/main.go` wires these components and owns process startup and shutdown.

The service stores owner email in its own `admin_service.tr_tenant_provision_operation` table. It never adds email to Tenant's outbox event or logs request bodies. It calls Tenant with the caller's bearer token plus a shared service token, and Auth with the service token. Configure `PLATFORM_SERVICE_TOKEN_FILE` with the same random secret for all three services.

Each persisted provisioning result, invitation reissue attempt, and tenant lifecycle transition (`admin.tenant.lifecycle_changed.v1`) creates an `admin_service.sys_outbox_event` row. The publisher claims batches of ten rows with a two-minute lease and sends an event envelope to JetStream subject `admin.audit.v1` with `Nats-Msg-Id` equal to the outbox ID.
