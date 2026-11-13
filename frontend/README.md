# ReserveHub Platform & Workspace UI

React interface for platform administration, invitation onboarding, and tenant-scoped workspace management exposed through `api-gateway`.

## Local development

With the API gateway running at `http://localhost:8080`, install dependencies and start Vite:

```sh
cd frontend
npm ci
npm run dev
```

Open `http://127.0.0.1:5173`. Vite proxies `/v1/*` and `/health/*` to the fixed local gateway, so browser requests use the UI's own origin and work with CORS disabled. The proxy is for local development only; serve assets through a trusted same-origin reverse proxy in deployment, or configure a precise gateway CORS allowlist if separate origins are required. Do not enable wildcard CORS for authenticated requests.

To start the UI with the full stack, run `docker compose up --build` from the repository root and open `http://127.0.0.1:5173`. Compose waits for the API Gateway health check and routes UI API requests to `api-gateway` over the internal Compose network. Set `FRONTEND_PORT` to change the host port.

To test invitation emails locally, run `bash scripts/dev/setup-local-smtp.sh` once before starting Compose. This creates untracked development secrets and enables the Mailpit SMTP catcher with STARTTLS. Open the inbox at `http://localhost:8025`; invitation links use `https://localhost:8443/invitations/accept`. The generated certificate is self-signed, so the browser will require a one-time trust exception for the local HTTPS frontend. Keep the same certificate and invitation encryption key while invitations are queued. The Mailpit SMTP port is only available inside the Compose network, and its inbox is bound to localhost.

Frontend requests always use the page's own origin; there is no user-editable API origin override. `npm run build` runs strict TypeScript checking and creates the production bundle in `dist/`. Do not put database credentials, service tokens, JWT signing keys, or SMTP secrets from `.env`/`.env.example` into browser code.

## Browser routes

- `/login` — sign in.
- `/invitations/accept` — accept a one-time invitation; a `token` query parameter is removed from the address bar immediately.
- `/platform/tenants/new` — create a company (platform administrator).
- `/platform/tenants` — company directory (platform administrator).
- `/platform/tenants/{tenant_id}` — company card and providers (platform administrator).
- `/workspace/{tenant_id}` — selected company workspace; `/workspace` shows the no-access state when the account has no active memberships.

The browser URL tracks navigation and supports direct links, refresh, and Back/Forward. In deployment, the same-origin reverse proxy must serve `index.html` for these UI paths while forwarding `/v1/*` and health/API paths to Gateway.

## Supported flows (Stages B–D)

- **Public Invitation Acceptance (`/invitations/accept?token=...`)**:
  - Reads the one-time `token` from the query parameter and immediately strips `?token=...` from the address bar via `history.replaceState` without storing it in `localStorage` or `sessionStorage`.
  - Supports both new identities (setting and confirming a 12–72 character password) and existing accounts (`POST /v1/auth/invitations/accept`).
- **Authentication & Context Routing**:
  - Sign in with `POST /v1/auth/login` and verify identity and active memberships with `GET /v1/auth/me` / `GET /v1/auth/tenants`.
  - Platform administrators (`role: "admin"`) enter the **Platform Admin Console**; tenant members (`tenant_owner`, `tenant_viewer`) exchange their token for a tenant-scoped JWT via `POST /v1/auth/tenant-session` and enter the **Tenant Workspace** with a header **Tenant Switcher**.
- **Platform Admin Console**:
  - Create a tenant with `POST /v1/admin/tenants` and a UUID `Idempotency-Key`.
  - Track provisioning and invitation status (`GET /v1/admin/tenant-provisioning/{operation_id}`) and reissue expired invitations (`POST /v1/admin/tenant-provisioning/{operation_id}/reissue-invitation`).
  - Browse the **Tenant Directory** (`GET /v1/admin/tenants` with status filter and pagination), inspect the **Tenant Card** (`GET /v1/admin/tenants/{tenant_id}`), and transition lifecycle states (`POST /v1/admin/tenants/{tenant_id}/{activate|suspend|resume|archive}`) with the **Activation Gate** enforcing `invite_status: accepted` before activation.
- **Tenant Workspace (`tenant_owner` & `tenant_viewer`)**:
  - View and update company profile (`GET/PATCH /v1/tenants/{tenant_id}`), regional settings (`GET/PUT /v1/tenants/{tenant_id}/settings`), branding (`GET/PUT /v1/tenants/{tenant_id}/branding`), and domains (`GET /v1/tenants/{tenant_id}/domains`).
  - Read-only enforcement with explanatory banners when `permission_profile === "tenant_viewer"` or when the tenant status is `PENDING` or `SUSPENDED`.
  - **Team & Memberships**: list members (`GET /v1/auth/tenants/{tenant_id}/members`), invite members (`POST /v1/auth/tenants/{tenant_id}/invitations`), and revoke memberships (`POST /v1/auth/tenants/{tenant_id}/members/{membership_id}/revoke`).
- **Providers (Stage D)**:
  - In the selected company card, a platform administrator can list and inspect city-level providers, create one with an `Idempotency-Key`, and switch its `ACTIVE`/`SUSPENDED` status while the company is active.
  - A `tenant_owner` can list and inspect providers of the current company and update their name and city while the company is active. Provider profile updates remain available when that provider is suspended. `tenant_viewer` does not call Provider APIs.
  - Provider lists remain readable while the company is `PENDING` or `SUSPENDED`. Owner access is blocked when the company is `ARCHIVED`; platform administrators can still read the archived provider directory.
  - When a tenant JWT expires, the client refreshes the base session and exchanges it for a new tenant-scoped JWT before retrying the tenant request.
- **Sign out**: revokes the refresh session through `POST /v1/auth/logout` and clears `sessionStorage`.
