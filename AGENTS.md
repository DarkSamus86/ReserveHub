# ReserveHub agent instructions

ReserveHub has independent Go modules in `services/auth-service`, `services/tenant-service`, `services/admin-service`, and `services/api-gateway`, plus the React application in `frontend`. Run Go commands from the affected module directory. `tenant-service` owns its gRPC API and data; browser requests go through Gateway. Read the current code and relevant service README before changing a contract; architecture documents may describe proposed rather than implemented components.

## Use project skills

- Use `$change-verification` after changing application code, tests, build configuration, or generated code. Select checks for the affected modules and report skipped integration tests.
- Use `$api-contract-change` when changing an HTTP or gRPC request, response, route, authorization rule, or error mapping across components.
- Use `$postgres-migration-review` when adding or changing a PostgreSQL migration or a persistence invariant.
- Use `$tenant-provisioning-flow` when changing tenant creation, owner invitation, operation status, retry, or reissue behavior across services or the platform UI.

The skills live in `.agents/skills/`. Apply multiple skills when their triggers overlap. Keep existing user changes intact and scope edits to the requested task.
