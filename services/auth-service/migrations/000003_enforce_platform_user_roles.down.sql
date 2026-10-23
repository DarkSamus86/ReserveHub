BEGIN;

-- Restore the original existence FK before removing the scoped FK.
ALTER TABLE auth_service.users
    ADD CONSTRAINT fk_users_role
        FOREIGN KEY (role_id)
        REFERENCES auth_service.roles (id)
        NOT VALID;

ALTER TABLE auth_service.users VALIDATE CONSTRAINT fk_users_role;

ALTER TABLE auth_service.users
    DROP CONSTRAINT fk_users_platform_role,
    DROP COLUMN role_scope;

ALTER TABLE auth_service.roles DROP CONSTRAINT uq_roles_id_scope;

COMMIT;
