---
name: api-contract-change
description: Trace and update ReserveHub HTTP or gRPC contracts across their owner, Gateway, callers, and platform UI when routes, payloads, auth, or errors change.
---

# API contract change

1. Find the implemented contract and its owner. For Tenant gRPC, inspect `services/tenant-service/proto/tenant/v1/tenant.proto`, its transport, application layer, and callers. For public HTTP, inspect the owning handler and `services/api-gateway/internal/gateway/`; inspect `frontend/src/` when the browser consumes it.
2. Record the request and response fields, status/error mapping, and principal required at each boundary before editing. Keep Tenant internal to service callers; Gateway and the owning service both enforce their respective authorization checks. Client-side role checks are presentation only.
3. Update the owner and affected consumers together. Keep existing clients working or explicitly document the compatibility break. When editing proto, regenerate `services/tenant-service/gen/` using the command in its README; never hand-edit generated code.
4. Add focused contract tests at changed boundaries. Invoke `$change-verification` for the affected modules and frontend. Report any caller that could not be checked.
