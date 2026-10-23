CREATE TABLE md_tenant (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    status_id SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    activated_at TIMESTAMPTZ,
    suspended_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,

    CONSTRAINT uq_md_tenant_slug UNIQUE (slug),
    CONSTRAINT fk_md_tenant_status
        FOREIGN KEY (status_id)
        REFERENCES di_tenant_status(id)
);
