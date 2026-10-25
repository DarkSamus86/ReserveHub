# auth-service

Basic authentication service for ReserveHub. The detailed target requirements are in [Auth Service STPO](../../docs/services/auth-service/ReserveHub-Auth-Service-STPO.md). Logs follow the [common ReserveHub logging convention](../../docs/observability/logging.md). It exposes JSON over HTTP:

- `POST /v1/auth/register` — public registration; always creates a `user` role.
- `POST /v1/auth/login` — verifies credentials and returns a 15-minute EdDSA bearer token.
- `GET /v1/auth/me` — returns the current account for a valid bearer token.
- `GET /health/live` and `GET /health/ready` — liveness and PostgreSQL readiness.

Passwords are hashed with bcrypt. Roles are stored in `auth_service.roles`; `auth_service.users.role_id` references the assigned role. The schema seeds the platform roles `admin` and `user` and supports tenant-scoped role rows for later membership provisioning. Clients cannot choose a role during registration. The initial administrator is created by the one-shot `auth-service-bootstrap` job from `AUTH_BOOTSTRAP_ADMIN_EMAIL` and `AUTH_BOOTSTRAP_ADMIN_PASSWORD` after the schema migrations complete. Admins can call all tenant RPCs. Regular users currently have no tenant access: until user-to-tenant membership and scoped read queries exist, tenant RPCs are denied rather than exposing every tenant to every account.

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

Registration always creates the `user` role. Use the configured bootstrap account to access tenant RPCs; user accounts currently cannot access tenant data until ownership-scoped reads are implemented. The service is intended to run behind a TLS-terminating gateway when exposed outside the trusted Docker network.
