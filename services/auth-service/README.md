# auth-service

Basic authentication service for ReserveHub. The detailed target requirements are in [Auth Service STPO](../../docs/services/auth-service/ReserveHub-Auth-Service-STPO.md). Logs follow the [common ReserveHub logging convention](../../docs/observability/logging.md). It exposes JSON over HTTP:

- `POST /v1/auth/register` — public registration; always creates a `user` role.
- `POST /v1/auth/login` — verifies credentials and returns a 15-minute EdDSA access token, a refresh token, and the user's active tenant `memberships`.
- `POST /v1/auth/refresh` — accepts `{"refresh_token":"..."}` and rotates it once; returns the login response shape.
- `POST /v1/auth/logout` — accepts `{"refresh_token":"..."}`, revokes the session, and returns 204.
- `POST /v1/auth/invitations/accept` — accepts `{"token":"...","password":"..."}` for a new identity or `{"token":"..."}` for an existing account; returns an active membership without a session.
- `GET /v1/auth/me` — returns the current account (including active tenant `memberships`) for a valid bearer token.
- `GET /v1/auth/tenants` — returns all active tenant memberships (`{"items":[...]}`) for the authenticated user.
- `POST /v1/auth/tenant-session` — exchanges a valid bearer token and `{"tenant_id":"..."}` for a 15-minute tenant-scoped EdDSA JWT containing `scope: "tenant"`, `tenant_id`, and `permission_profile` (`tenant_owner` or `tenant_viewer`). Returns 403 if the user has no `ACTIVE` membership in that tenant.
- `GET /v1/auth/tenants/{tenant_id}/members` — lists members and invitations for `{tenant_id}` (`tenant_owner` or `tenant_viewer` of that tenant).
- `POST /v1/auth/tenants/{tenant_id}/invitations` — invites a user by `email` and `permission_profile` (`tenant_viewer` or `tenant_owner`) into `{tenant_id}` (`tenant_owner` only); queues SMTP delivery via the Auth invitation worker and records `auth.invitation.created.v1`.
- `POST /v1/auth/tenants/{tenant_id}/members/{membership_id}/revoke` — revokes a membership and any pending invitation in `{tenant_id}` (`tenant_owner` only; blocks revoking the last active owner) and records `auth.membership.revoked.v1`.
- `GET /health/live` and `GET /health/ready` — liveness and PostgreSQL readiness.

The first owner-provisioning step is available only to a trusted internal caller at
`POST /internal/v1/tenant-memberships/provision-owner`. The caller sends
`X-Service-Token` and JSON containing canonical UUIDs `tenant_id` and
`operation_id`, the initial Tenant slug, and the owner's email. Configure
`AUTH_PROVISION_SERVICE_TOKEN_FILE` to a file containing a random secret of at
least 32 bytes; without it the internal operation returns 503. The same
operation ID and identical normalized input return the same IDs (200); a new
operation returns 201; reusing an operation ID or tenant ID for different data
returns 409. The response contains only tenant, user, membership, and invitation
IDs plus `pending_delivery` status. It never returns an invitation secret.

Provisioning creates a tenant-scoped role whose code is the initial slug and a
`PENDING` owner membership. A new identity is `INVITED` without a password and
cannot log in. An existing account stays unchanged and receives no tenant
access. Without SMTP configuration the invitation stays `PENDING_DELIVERY`.
With SMTP, the worker stores an AES-GCM encrypted delivery command in the Auth
outbox and moves the invite to `READY_TO_SEND`. Successful STARTTLS delivery
moves it to `SENT`. Only a valid, unexpired `SENT` token can atomically
activate membership; new users set their own password in that transaction.
After accepting the invitation, the user can exchange their bearer token via
`POST /v1/auth/tenant-session` for a tenant-scoped JWT once the platform
administrator activates the tenant.

The protected `GET /internal/v1/tenant-invitations/{id}` returns only status
and optional expiry. `POST /internal/v1/tenant-invitations/{id}/reissue` uses
the same `X-Service-Token` as provisioning and returns 202 only for an expired
`READY_TO_SEND` or `SENT` invite. It clears the old token hash and encrypted
payload; the worker then prepares a new token. Accepted or revoked invites
cannot be reissued by this operation.

SMTP delivery is opt-in. Set `AUTH_SMTP_ADDRESS` (host:port),
`AUTH_SMTP_FROM`, `AUTH_SMTP_USERNAME`, `AUTH_SMTP_PASSWORD_FILE`,
`AUTH_INVITATION_LINK_BASE_URL` (HTTPS page URL), and
`AUTH_INVITATION_ENCRYPTION_KEY_FILE` (64 hex digits for a random 32-byte key).
STARTTLS with certificate verification is required. Keep the encryption key
available while queued commands exist. The outbox removes ciphertext after
successful or expired delivery. `NATS_URL` is required for the Auth audit
publisher, which sends safe metadata to `auth.audit.v1` and never publishes
invitation delivery commands.

MVP timing assumptions: `AUTH_INVITATION_TTL` defaults to 48 hours from token
creation (configurable from 1 minute to 30 days). `AUTH_REFRESH_TTL` defaults
to a fixed 30 days from login (configurable up to 90 days); refresh does not
extend absolute expiry. Access tokens expire within 15 minutes and may remain
valid until then after logout. Refresh tokens are returned in JSON; browser
cookie and CSRF handling is outside this contract. Token responses use
`Cache-Control: no-store`.

Passwords are hashed with bcrypt. Roles are stored in `auth_service.md_role`; `auth_service.md_user.role_id` references the assigned platform role. The schema seeds the platform roles `admin` and `user` and supports tenant-scoped role rows for membership provisioning. Clients cannot choose a role during registration. The initial administrator is created by the one-shot `auth-service-bootstrap` job from `AUTH_BOOTSTRAP_ADMIN_EMAIL` and `AUTH_BOOTSTRAP_ADMIN_PASSWORD` after the schema migrations complete.

## Local Compose

Create an Ed25519 signing key pair before starting Compose:

```sh
mkdir -p secrets
openssl genpkey -algorithm ED25519 -out secrets/auth-jwt-private.pem
openssl pkey -in secrets/auth-jwt-private.pem -pubout -out secrets/auth-jwt-public.pem
chmod 600 secrets/auth-jwt-private.pem
```

Set `AUTH_JWT_PRIVATE_KEY_FILE` and `AUTH_JWT_PUBLIC_KEY_FILE` in `.env` to those files (the `.env.example` paths work as-is). Only Auth receives the private signing key; Gateway and Tenant receive the public verification key. The `secrets/*.pem` files are ignored by Git. On the first startup, also set `AUTH_BOOTSTRAP_ADMIN_EMAIL` and a unique `AUTH_BOOTSTRAP_ADMIN_PASSWORD` (12–72 bytes) in your local `.env` (do not commit it). The migrations seed the `admin` role; the one-shot `auth-service-bootstrap` job creates the account with a bcrypt hash before the server starts. There is deliberately no default admin password in SQL: without credentials on a fresh database, the bootstrap job fails with an actionable error and the server does not start. After successful bootstrap, remove both bootstrap variables from `.env`; subsequent runs skip account creation and never reset its password. To change the password, use a dedicated password-rotation process rather than rerunning bootstrap. Do not use development keys or passwords in production.

Then run:

```sh
docker compose up --build -d
```

The public Auth endpoint is `http://localhost:8080` through `api-gateway`. The Auth service itself is internal to the Compose network. Register a regular account and log in with:

```sh
curl -sS http://localhost:8080/v1/auth/register -H 'content-type: application/json' -d '{"email":"user@example.com","password":"use-a-long-password"}'
curl -sS http://localhost:8080/v1/auth/login -H 'content-type: application/json' -d '{"email":"user@example.com","password":"use-a-long-password"}'
```
