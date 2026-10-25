# API Gateway

The public HTTP entry point for the current services. It exposes only the Auth API; it does not proxy the Tenant gRPC service.

## Routes

| Route | Access | Upstream |
| --- | --- | --- |
| `POST /v1/auth/register` | Public, rate limited | `auth-service` |
| `POST /v1/auth/login` | Public, rate limited | `auth-service` |
| `GET /v1/auth/me` | Valid EdDSA bearer token with API audience required | `auth-service` |
| `GET /health/live` | Public liveness | Gateway |
| `GET /health/ready` | Dependency readiness | Gateway and Auth |

Every other method and path returns 404 or 405. Tenant RPCs are internal service calls. In Compose, `tenant-service` has no published host ports and is attached only to `tenant-internal`; a future `admin-service` can join that network and call Tenant over gRPC. The gateway is not attached to that network.

## Local run

From the repository root, create `.env` using `.env.example`, then start the stack:

```sh
docker compose up --build -d
```

Use `http://localhost:8080` for Auth. Generate an Ed25519 key pair as shown in the repository `.env.example`. Auth receives the private key; Gateway and Tenant receive only the public key through Compose secrets.

## Security behavior

- JWT verification accepts only `EdDSA`, key ID `auth-ed25519-v1`, issuer `reservehub-auth`, and audience `reservehub-api`; it checks signature, subject, role, scope, issue time, expiry, and the 15-minute maximum token lifetime. `/v1/auth/me` is checked at the gateway and again by Auth.
- Login and registration are rate limited by the socket peer IP; all API traffic has an IP bucket, and authenticated `/me` calls also have a subject bucket. The in-memory limiter has a fixed maximum key count.
- Request bodies are limited to 1 MiB. Auth paths and HTTP methods are explicit. The gateway rejects query parameters and non-JSON auth requests.
- Client-supplied forwarded and identity headers are removed. The gateway derives forwarding metadata from the TCP peer; it does not trust `X-Forwarded-For` from clients.
- HTTP server and upstream timeouts, bounded header size, request IDs, safe upstream errors, and graceful shutdown are configured. Logs omit query strings, bodies, bearer tokens, and email addresses.
- CORS is disabled. Add an explicit origin allowlist before serving browser clients from another origin.

The limiter is process-local. If the gateway is scaled to multiple replicas, use a shared rate-limit store and retain an IP-based pre-authentication limit. For external production traffic, terminate TLS at a trusted ingress and configure trusted proxy handling explicitly before relying on proxy-provided client addresses.
