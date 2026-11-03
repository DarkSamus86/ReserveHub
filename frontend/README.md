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

Frontend requests always use the page's own origin; there is no user-editable API origin override. `npm run build` runs strict TypeScript checking and creates the production bundle in `dist/`. Do not put database credentials, service tokens, JWT signing keys, or SMTP secrets from `.env`/`.env.example` into browser code.

## Supported flows (Stage B & Stage C)

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
- **Sign out**: revokes the refresh session through `POST /v1/auth/logout` and clears `sessionStorage`.
