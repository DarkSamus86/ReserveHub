-- A user keeps a platform role even when they belong to one or more tenants.
ALTER TABLE auth_service.roles DROP CONSTRAINT roles_code_check;
ALTER TABLE auth_service.roles ADD CONSTRAINT ck_roles_code_by_scope CHECK (
    (scope = 'platform' AND code ~ '^[a-z][a-z0-9-]{0,62}$') OR
    (scope = 'tenant' AND length(code) <= 100 AND code ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);
ALTER TABLE auth_service.roles ADD CONSTRAINT uq_roles_id_scope UNIQUE (id, scope);
ALTER TABLE auth_service.roles ADD CONSTRAINT uq_roles_id_tenant UNIQUE (id, tenant_id);
CREATE UNIQUE INDEX uq_roles_one_per_tenant ON auth_service.roles (tenant_id) WHERE scope = 'tenant';

ALTER TABLE auth_service.users ADD COLUMN role_scope TEXT NOT NULL DEFAULT 'platform';
ALTER TABLE auth_service.users ADD CONSTRAINT ck_users_platform_role CHECK (role_scope = 'platform');
ALTER TABLE auth_service.users ADD COLUMN status TEXT NOT NULL DEFAULT 'ACTIVE';
ALTER TABLE auth_service.users ADD CONSTRAINT ck_users_status CHECK (status IN ('ACTIVE', 'INVITED'));
ALTER TABLE auth_service.users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE auth_service.users ADD CONSTRAINT ck_users_password_state CHECK (
    (status = 'ACTIVE' AND password_hash IS NOT NULL) OR
    (status = 'INVITED' AND password_hash IS NULL)
);
ALTER TABLE auth_service.users ADD CONSTRAINT ck_users_normalized_email CHECK (
    email = lower(btrim(email)) AND length(email) <= 254
);
CREATE UNIQUE INDEX uq_users_email_folded ON auth_service.users (lower(email));
ALTER TABLE auth_service.users DROP CONSTRAINT fk_users_role;
ALTER TABLE auth_service.users ADD CONSTRAINT fk_users_platform_role
    FOREIGN KEY (role_id, role_scope) REFERENCES auth_service.roles (id, scope);

CREATE TABLE auth_service.tenant_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth_service.users (id),
    tenant_id UUID NOT NULL,
    tenant_role_id UUID NOT NULL,
    permission_profile TEXT NOT NULL CHECK (permission_profile IN ('tenant_owner', 'tenant_viewer')),
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'ACTIVE', 'REVOKED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    activated_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT uq_tenant_memberships_user_tenant UNIQUE (user_id, tenant_id),
    CONSTRAINT uq_tenant_memberships_id_tenant UNIQUE (id, tenant_id),
    CONSTRAINT fk_membership_tenant_role FOREIGN KEY (tenant_role_id, tenant_id)
        REFERENCES auth_service.roles (id, tenant_id),
    CONSTRAINT ck_membership_timestamps CHECK (
        (status = 'PENDING' AND activated_at IS NULL AND revoked_at IS NULL) OR
        (status = 'ACTIVE' AND activated_at IS NOT NULL AND revoked_at IS NULL) OR
        (status = 'REVOKED' AND revoked_at IS NOT NULL)
    )
);
CREATE INDEX ix_tenant_memberships_tenant ON auth_service.tenant_memberships (tenant_id);

CREATE TABLE auth_service.invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    membership_id UUID NOT NULL UNIQUE REFERENCES auth_service.tenant_memberships (id),
    email TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PENDING_DELIVERY', 'SENT', 'ACCEPTED', 'REVOKED')),
    token_hash BYTEA,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT ck_invitation_secret CHECK (
        (status = 'PENDING_DELIVERY' AND token_hash IS NULL AND expires_at IS NULL AND sent_at IS NULL) OR
        (status = 'SENT' AND token_hash IS NOT NULL AND expires_at IS NOT NULL AND sent_at IS NOT NULL) OR
        (status = 'ACCEPTED' AND accepted_at IS NOT NULL) OR
        (status = 'REVOKED' AND revoked_at IS NOT NULL)
    ),
    CONSTRAINT ck_invitation_hash_size CHECK (token_hash IS NULL OR octet_length(token_hash) = 32)
);
CREATE UNIQUE INDEX uq_invitations_token_hash ON auth_service.invitations (token_hash)
    WHERE token_hash IS NOT NULL;
CREATE INDEX ix_invitations_pending_delivery ON auth_service.invitations (created_at, id)
    WHERE status = 'PENDING_DELIVERY';

CREATE TABLE auth_service.provision_requests (
    operation_id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL UNIQUE,
    slug TEXT NOT NULL,
    owner_email TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES auth_service.users (id),
    membership_id UUID NOT NULL UNIQUE REFERENCES auth_service.tenant_memberships (id),
    invitation_id UUID NOT NULL UNIQUE REFERENCES auth_service.invitations (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_provision_slug CHECK (length(slug) <= 100 AND slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    CONSTRAINT fk_provision_membership_tenant FOREIGN KEY (membership_id, tenant_id)
        REFERENCES auth_service.tenant_memberships (id, tenant_id)
);
