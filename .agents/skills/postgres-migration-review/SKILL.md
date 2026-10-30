---
name: postgres-migration-review
description: Review ReserveHub PostgreSQL migrations and persistence invariants for service ownership, safe rollout, rollback, and transactional outbox or idempotency behavior.
---

# PostgreSQL migration review

1. Locate the table owner in `services/{auth,tenant,admin}-service/migrations/` and inspect the latest migrations and repository queries. A service must not migrate another service's schema. Add a new numbered `up.sql` and `down.sql`; do not rewrite an applied migration.
2. Check existing rows, nullability, defaults, uniqueness, foreign keys, indexes, and lock or backfill cost. For incompatible data changes, describe the required rollout order instead of assuming a fresh database.
3. Check that idempotency records and outbox events remain consistent with the business write in one transaction where the feature requires them. Avoid putting invitation secrets, access tokens, or personal data into outbox and audit payloads unless the contract requires them.
4. Validate SQL and repository behavior with an isolated database when available. If rollback is destructive or cannot reconstruct prior data, state that limit plainly. Invoke `$change-verification` for affected code.
