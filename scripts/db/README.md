# PostgreSQL schema names

ReserveHub uses one PostgreSQL database (`reservehub` by default) with three service-owned schemas: `auth`, `tenant`, and `admin`. The service process names, Go modules, and Compose service names retain their `-service` suffixes.

## Upgrade from the old schema names

Stop the API and application services and take a database backup before deploying this change. The old application code queries `auth_service`, `tenant_service`, and `admin_service`; it cannot run concurrently with the new code while schemas are being renamed.

Each service runs its existing migrations in the old schema, records its new transition version, then runs the corresponding `rename-*-schema.sql` job. The rename moves all tables, indexes, sequences, and the `sys_schema_migration` table together without copying rows. On later Compose starts, the preparation job detects the new schema, migration tracking uses it, and the rename job verifies it. The preparation job fails if both old and new schemas exist for one service, so an ambiguous partial rollout cannot silently choose one.

The transition migrations (`auth` 000007, `tenant` 000022, `admin` 000005) contain only a marker because `golang-migrate` resolves the migration table schema when it opens the connection. Renaming that schema inside a migration would prevent the runner from writing the new version. The Compose jobs perform the rename only after the version is recorded.

For rollback, stop the new application services, rename each schema back (`ALTER SCHEMA auth RENAME TO auth_service`, and likewise for `tenant` and `admin`), then restore the prior application and Compose configuration. If rolling migration versions back, do so only after the schemas have their old names. A rollback requires a coordinated maintenance window; do not use the transition `.down.sql` marker alone as a schema rollback.
