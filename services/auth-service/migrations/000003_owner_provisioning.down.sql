DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM auth_service.provision_requests)
        OR EXISTS (SELECT 1 FROM auth_service.invitations)
        OR EXISTS (SELECT 1 FROM auth_service.tenant_memberships) THEN
        RAISE EXCEPTION 'cannot revert owner provisioning while provisioning data exists';
    END IF;
    IF EXISTS (SELECT 1 FROM auth_service.users WHERE status = 'INVITED') THEN
        RAISE EXCEPTION 'cannot revert owner provisioning while invited users exist';
    END IF;
    IF EXISTS (SELECT 1 FROM auth_service.roles WHERE scope = 'tenant' AND code !~ '^[a-z][a-z0-9-]{0,62}$') THEN
        RAISE EXCEPTION 'cannot revert owner provisioning while tenant role codes exceed old format';
    END IF;
END $$;

DROP TABLE auth_service.provision_requests;
DROP TABLE auth_service.invitations;
DROP TABLE auth_service.tenant_memberships;
ALTER TABLE auth_service.users DROP CONSTRAINT fk_users_platform_role;
ALTER TABLE auth_service.users DROP CONSTRAINT ck_users_password_state;
ALTER TABLE auth_service.users DROP CONSTRAINT ck_users_normalized_email;
DROP INDEX auth_service.uq_users_email_folded;
ALTER TABLE auth_service.users DROP CONSTRAINT ck_users_status;
ALTER TABLE auth_service.users DROP CONSTRAINT ck_users_platform_role;
ALTER TABLE auth_service.users ALTER COLUMN password_hash SET NOT NULL;
ALTER TABLE auth_service.users DROP COLUMN status;
ALTER TABLE auth_service.users DROP COLUMN role_scope;
ALTER TABLE auth_service.users ADD CONSTRAINT fk_users_role FOREIGN KEY (role_id) REFERENCES auth_service.roles (id);
DROP INDEX auth_service.uq_roles_one_per_tenant;
ALTER TABLE auth_service.roles DROP CONSTRAINT uq_roles_id_tenant;
ALTER TABLE auth_service.roles DROP CONSTRAINT uq_roles_id_scope;
ALTER TABLE auth_service.roles DROP CONSTRAINT ck_roles_code_by_scope;
ALTER TABLE auth_service.roles ADD CONSTRAINT roles_code_check CHECK (code ~ '^[a-z][a-z0-9-]{0,62}$');
