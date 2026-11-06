# API Gateway

The public HTTP entry point for Auth, platform administration, and tenant-scoped workspace operations. It does not proxy the Tenant gRPC service directly.

## Routes

| Route | Access | Upstream |
| --- | --- | --- |
| `POST /v1/auth/register` | Public, rate limited | `auth-service` |
| `POST /v1/auth/login` | Public, rate limited | `auth-service` |
| `POST /v1/auth/invitations/accept` | Public, rate limited; one-time token | `auth-service` |
| `POST /v1/auth/refresh` | Public, rate limited; refresh token in JSON body | `auth-service` |
| `POST /v1/auth/logout` | Public, rate limited; refresh token in JSON body | `auth-service` |
| `GET /v1/auth/me` | Valid EdDSA bearer token with API audience required | `auth-service` |
| `GET /v1/auth/tenants` | Valid EdDSA bearer token | `auth-service` |
| `POST /v1/auth/tenant-session` | Valid EdDSA bearer token; exchanges for tenant-scoped JWT | `auth-service` |
| `GET /v1/auth/tenants/{tenant_id}/members` | Tenant member (`tenant_owner` or `tenant_viewer`) for `{tenant_id}` | `auth-service` |
| `POST /v1/auth/tenants/{tenant_id}/invitations` | Tenant owner (`tenant_owner`) for `{tenant_id}` | `auth-service` |
| `POST /v1/auth/tenants/{tenant_id}/members/{membership_id}/revoke` | Tenant owner (`tenant_owner`) for `{tenant_id}` | `auth-service` |
| `POST /v1/admin/tenants` | Platform admin bearer token and UUID `Idempotency-Key` | `admin-service` |
| `GET /v1/admin/tenants` | Platform admin bearer token (`page_size`, `page_token`, `status`) | `admin-service` |
| `GET /v1/admin/tenants/{tenant_id}` | Platform admin bearer token | `admin-service` |
| `POST /v1/admin/tenants/{tenant_id}/{activate\|suspend\|resume\|archive}` | Platform admin bearer token | `admin-service` |
| `GET /v1/admin/tenant-provisioning/{operation_id}` | Platform admin bearer token | `admin-service` |
| `POST /v1/admin/tenant-provisioning/{operation_id}/reissue-invitation` | Platform admin bearer token; expired invitation only | `admin-service` |
| `GET /v1/tenants/{tenant_id}` | Tenant member (`tenant_owner` or `tenant_viewer`) for `{tenant_id}` | `admin-service` |
| `PATCH /v1/tenants/{tenant_id}` | Tenant owner (`tenant_owner`) for `{tenant_id}` | `admin-service` |
| `GET /v1/tenants/{tenant_id}/settings` | Tenant member (`tenant_owner` or `tenant_viewer`) for `{tenant_id}` | `admin-service` |
| `PUT /v1/tenants/{tenant_id}/settings` | Tenant owner (`tenant_owner`) for `{tenant_id}` | `admin-service` |
| `GET /v1/tenants/{tenant_id}/branding` | Tenant member (`tenant_owner` or `tenant_viewer`) for `{tenant_id}` | `admin-service` |
| `PUT /v1/tenants/{tenant_id}/branding` | Tenant owner (`tenant_owner`) for `{tenant_id}` | `admin-service` |
| `GET /v1/tenants/{tenant_id}/domains` | Tenant member (`tenant_owner` or `tenant_viewer`) for `{tenant_id}` | `admin-service` |
| `POST /v1/admin/tenants/{tenant_id}/providers` | Platform admin bearer token and UUID `Idempotency-Key` | `admin-service` → Provider gRPC |
| `GET /v1/admin/tenants/{tenant_id}/providers` | Platform admin bearer token; `page_size`, `page_token` | `admin-service` → Provider gRPC |
| `GET /v1/admin/tenants/{tenant_id}/providers/{provider_id}` | Platform admin bearer token | `admin-service` → Provider gRPC |
| `PATCH /v1/admin/tenants/{tenant_id}/providers/{provider_id}/status` | Platform admin bearer token | `admin-service` → Provider gRPC |
| `GET /v1/tenants/{tenant_id}/providers` | Tenant owner for `{tenant_id}`; `page_size`, `page_token` | `workspace-service` → Provider gRPC |
| `GET /v1/tenants/{tenant_id}/providers/{provider_id}` | Tenant owner for `{tenant_id}` | `workspace-service` → Provider gRPC |
| `PATCH /v1/tenants/{tenant_id}/providers/{provider_id}` | Tenant owner for `{tenant_id}` | `workspace-service` → Provider gRPC |
| `GET /openapi.json` | OpenAPI 3.0 specification; only when `APP_ENV` is non-production (404 in production) | Gateway |
| `GET /swagger` | Swagger UI; only when `APP_ENV` is non-production (404 in production) | Gateway |
| `GET /health/live` | Public liveness | Gateway |
| `GET /health/ready` | Dependency readiness | Gateway, Auth, Admin, and Workspace |

Every other method and path returns 404 or 405. Tenant and Provider RPCs are internal service calls. In Compose, neither `tenant-service` nor `provider-service` publishes host ports. Gateway reaches Admin and Workspace over the application network; those facades reach Provider on `provider-internal`. Provider reaches Tenant on `tenant-internal`. Gateway is attached to neither private network.

## Local run

From the repository root, create `.env` using `.env.example`, then start the stack:

```sh
docker compose up --build -d
```

Use `http://localhost:8080` for Auth. Generate an Ed25519 key pair as shown in the repository `.env.example`. Auth receives the private key; Gateway and Tenant receive only the public key through Compose secrets.

## Security behavior

- JWT verification accepts only `EdDSA`, key ID `auth-ed25519-v1`, issuer `reservehub-auth`, and audience `reservehub-api`; it checks signature, subject, role, scope, optional `tenant_id` and `permission_profile`, issue time, expiry, and the 15-minute maximum token lifetime. Tenant-scoped routes enforce BOLA (`claims.TenantID == pathTenantID`) and RBAC (`tenant_owner` vs `tenant_viewer`) at the gateway edge and again in the upstream service.
- Login and registration are rate limited by the socket peer IP; all API traffic has an IP bucket, and authenticated calls also have a subject bucket. The in-memory limiter has a fixed maximum key count.
- Request bodies are limited to 1 MiB. Paths and HTTP methods (`POST`, `PUT`, `PATCH`, `GET`) are explicit. The gateway rejects unexpected query parameters (allowing `page_size`, `page_token`, and `status` on `GET /v1/admin/tenants`, and only `page_size` and `page_token` on Provider list routes) and non-JSON write requests.
- Client-supplied forwarded and identity headers are removed. The gateway derives forwarding metadata from the TCP peer; it does not trust `X-Forwarded-For` from clients.
- HTTP server and upstream timeouts, bounded header size, request IDs, safe upstream errors, and graceful shutdown are configured. Logs omit query strings, bodies, bearer tokens, and email addresses.
- CORS is disabled. Add an explicit origin allowlist before serving browser clients from another origin.

The limiter is process-local. If the gateway is scaled to multiple replicas, use a shared rate-limit store and retain an IP-based pre-authentication limit. For external production traffic, terminate TLS at a trusted ingress and configure trusted proxy handling explicitly before relying on proxy-provided client addresses.
