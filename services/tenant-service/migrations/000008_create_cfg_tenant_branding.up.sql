CREATE TABLE cfg_tenant_branding (
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

    CONSTRAINT fk_cfg_tenant_branding_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_cfg_tenant_branding_status
        FOREIGN KEY (status_id)
        REFERENCES di_branding_status(id),
    CONSTRAINT chk_cfg_tenant_branding_primary_color
        CHECK (primary_color IS NULL OR primary_color ~ '^#[0-9A-Fa-f]{6}$'),
    CONSTRAINT chk_cfg_tenant_branding_secondary_color
        CHECK (secondary_color IS NULL OR secondary_color ~ '^#[0-9A-Fa-f]{6}$'),
    CONSTRAINT chk_cfg_tenant_branding_accent_color
        CHECK (accent_color IS NULL OR accent_color ~ '^#[0-9A-Fa-f]{6}$')
);
