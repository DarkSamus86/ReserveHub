CREATE TABLE md_tenant_contact (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    contact_type VARCHAR(32) NOT NULL,
    name VARCHAR(255),
    email VARCHAR(320),
    phone VARCHAR(32),
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_md_tenant_contact_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE
);
