# ReserveHub Platform UI

React admin interface for the platform tenant provisioning flow currently exposed by `api-gateway`.

## Local development

With the API gateway running at `http://localhost:8080`, install dependencies and start Vite:

```sh
cd frontend
npm ci
npm run dev
```

Open `http://127.0.0.1:5173`. Vite proxies `/v1/*` and `/health/*` to the fixed local gateway, so browser requests use the UI's own origin and work with CORS disabled. The proxy is for local development only; serve assets through a trusted same-origin reverse proxy in deployment, or configure a precise gateway CORS allowlist if separate origins are required. Do not enable wildcard CORS for authenticated admin requests.

Frontend requests always use the page's own origin; there is no user-editable API origin override. `npm run build` runs strict TypeScript checking and creates the production bundle in `dist/`. Do not put database credentials, service tokens, JWT signing keys, or SMTP secrets from `.env`/`.env.example` into browser code.

## Supported flow

- Sign in with `POST /v1/auth/login`, then verify identity with `GET /v1/auth/me`.
- Create a tenant with `POST /v1/admin/tenants` and a UUID `Idempotency-Key`. Retrying an unchanged form reuses the same key.
- Refresh provisioning and invitation state with `GET /v1/admin/tenant-provisioning/{operation_id}`.
- Reissue the invitation only when the operation is complete and the `ready_to_send` or `sent` invitation has expired. The backend remains authoritative and can reject the request.
- Sign out through `POST /v1/auth/logout`.

Access and refresh tokens are held in `sessionStorage` for the current tab and removed on sign out. The invitation secret is never returned by the backend and is not rendered by this interface. The tenant remains pending after invitation acceptance; activation and tenant-scoped business workflows are outside the current API surface.
