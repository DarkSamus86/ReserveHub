CREATE TABLE auth_service.roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID,
    code TEXT NOT NULL CHECK (code ~ '^[a-z][a-z0-9-]{0,62}$'),
    name TEXT NOT NULL,
    scope TEXT NOT NULL CHECK (scope IN ('platform', 'tenant')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_roles_scope_tenant CHECK (
        (scope = 'platform' AND tenant_id IS NULL)
        OR (scope = 'tenant' AND tenant_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX uq_roles_platform_code
    ON auth_service.roles (code)
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX uq_roles_tenant_code
    ON auth_service.roles (tenant_id, code)
    WHERE tenant_id IS NOT NULL;

INSERT INTO auth_service.roles (code, name, scope)
VALUES
    ('admin', 'Platform administrator', 'platform'),
    ('user', 'User', 'platform');

ALTER TABLE auth_service.users ADD COLUMN role_id UUID;

UPDATE auth_service.users AS users
SET role_id = roles.id
FROM auth_service.roles AS roles
WHERE roles.scope = 'platform'
  AND roles.tenant_id IS NULL
  AND roles.code = users.role;

ALTER TABLE auth_service.users
    ALTER COLUMN role_id SET NOT NULL,
    ADD CONSTRAINT fk_users_role FOREIGN KEY (role_id) REFERENCES auth_service.roles (id);

ALTER TABLE auth_service.users DROP COLUMN role;

CREATE INDEX ix_users_role_id ON auth_service.users (role_id);
