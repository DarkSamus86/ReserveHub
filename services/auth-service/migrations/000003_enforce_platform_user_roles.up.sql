BEGIN;

-- Keep the audit and constraint change in one transaction: no user assignment
-- or role scope can change between the preflight check and FK validation.
LOCK TABLE auth_service.roles, auth_service.users IN SHARE ROW EXCLUSIVE MODE;

-- Investigate and resolve affected identities explicitly before retrying.
-- In particular, never map a tenant-scoped "admin" to a platform admin here.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM auth_service.users AS users
        LEFT JOIN auth_service.roles AS roles ON roles.id = users.role_id
        WHERE roles.id IS NULL
           OR roles.scope <> 'platform'
           OR roles.tenant_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'users contain non-platform or missing role assignments; audit auth_service.users.role_id before migrating';
    END IF;
END;
$$;

-- The constant default preserves existing INSERTs that specify only role_id.
-- PostgreSQL 17 can add this constant default without rewriting users.
ALTER TABLE auth_service.users
    ADD COLUMN role_scope TEXT NOT NULL DEFAULT 'platform',
    ADD CONSTRAINT ck_users_role_scope_platform CHECK (role_scope = 'platform');

-- The composite FK needs a unique referenced key even though roles.id is PK.
ALTER TABLE auth_service.roles
    ADD CONSTRAINT uq_roles_id_scope UNIQUE (id, scope);

ALTER TABLE auth_service.users
    ADD CONSTRAINT fk_users_platform_role
        FOREIGN KEY (role_id, role_scope)
        REFERENCES auth_service.roles (id, scope)
        ON UPDATE RESTRICT
        NOT VALID;

ALTER TABLE auth_service.users VALIDATE CONSTRAINT fk_users_platform_role;

-- The stronger FK now covers existence as well as the platform scope.
ALTER TABLE auth_service.users DROP CONSTRAINT fk_users_role;

COMMIT;
