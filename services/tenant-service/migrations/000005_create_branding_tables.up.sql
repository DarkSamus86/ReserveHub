BEGIN;

SET LOCAL search_path = tenant_service, public;

CREATE TABLE branding_statuses (
    id SMALLINT PRIMARY KEY,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(64) NOT NULL
);

INSERT INTO branding_statuses (id, code, name) VALUES
    (1, 'DEFAULT', 'Default'),
    (2, 'DRAFT', 'Draft'),
    (3, 'ACTIVE', 'Active'),
    (4, 'DISABLED', 'Disabled');

CREATE TABLE tenant_branding (
    tenant_id UUID PRIMARY KEY,
    status_id SMALLINT NOT NULL,
    display_name VARCHAR(255),
    logo_url TEXT,
    favicon_url TEXT,
    primary_color VARCHAR(7),
    secondary_color VARCHAR(7),
    accent_color VARCHAR(7),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_tenant_branding_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES tenants(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_tenant_branding_status
        FOREIGN KEY (status_id)
        REFERENCES branding_statuses(id),
    CONSTRAINT chk_branding_primary_color
        CHECK (primary_color IS NULL OR primary_color ~ '^#[0-9A-Fa-f]{6}$'),
    CONSTRAINT chk_branding_secondary_color
        CHECK (secondary_color IS NULL OR secondary_color ~ '^#[0-9A-Fa-f]{6}$'),
    CONSTRAINT chk_branding_accent_color
        CHECK (accent_color IS NULL OR accent_color ~ '^#[0-9A-Fa-f]{6}$')
);

COMMIT;
