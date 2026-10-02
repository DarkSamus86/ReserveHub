BEGIN;

SET LOCAL search_path = tenant_service, public;

CREATE TABLE tenant_settings (
    tenant_id UUID PRIMARY KEY,
    timezone VARCHAR(64) NOT NULL,
    currency_code CHAR(3) NOT NULL,
    locale_code VARCHAR(16) NOT NULL,
    default_language VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_tenant_settings_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES tenants(id)
        ON DELETE CASCADE
);

COMMIT;
