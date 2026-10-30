---
name: tenant-provisioning-flow
description: Maintain the ReserveHub platform tenant creation and owner invitation journey across Gateway, Admin, Tenant, Auth, and the platform UI.
---

# Tenant provisioning flow

1. Read the current `services/admin-service/README.md` and the relevant handlers, service code, and tests in Gateway, Admin, Tenant, Auth, and `frontend/`. Confirm the specific state transition being changed before editing.
2. Preserve the operation's UUID `Idempotency-Key` behavior: the same key and body return the saved result; changed data with the same key conflicts; a retryable downstream failure can be retried with the same key and body. Tenant creation and owner invitation are separate stages. A completed provisioning operation does not imply delivery or acceptance.
3. Keep Tenant pending until the authorized backend activation flow. Reissue only under the backend's current eligibility rule. Treat the backend as authoritative for status and permissions; never expose an invitation secret in a public response, browser state, logs, or audit payload.
4. Cover affected success, retry, partial failure, expired invitation, permission, and duplicate-request paths with focused tests or a concrete manual check. Use `$api-contract-change` for wire changes and `$change-verification` for final checks. Report any stage whose behavior remains unverified.
