---
name: change-verification
description: Verify ReserveHub application, test, build, or generated-code changes with checks for the affected Go modules and frontend; report integration-test coverage accurately.
---

# Change verification

1. Inspect the diff and identify affected modules under `services/` and whether `frontend/` changed. Include consumers of a changed wire contract, not only its owner.
2. In each affected Go module, run `go test ./...` and `go vet ./...` when relevant to the change. From `frontend/`, run `npm run build` for TypeScript, UI, or frontend build changes. Do not assume a repository-root `go test ./...` covers the separate modules.
3. PostgreSQL integration tests use `TEST_DATABASE_URL` and may skip without it. For tenant integration tests, consult `services/tenant-service/README.md` for the `-tags=integration` command. Run database-backed checks when a suitable isolated test database is available; do not point tests at production or an unverified shared database.
4. If a command fails, identify whether the change or the environment caused it. Report commands, pass/fail/skip results, and any untested behavior. Stop once the checks needed for the change have passed or the remaining limitation is clear.
