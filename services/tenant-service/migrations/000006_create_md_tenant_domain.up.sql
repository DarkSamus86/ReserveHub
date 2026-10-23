CREATE TABLE md_tenant_domain (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    domain VARCHAR(253) NOT NULL,
    status_id SMALLINT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    verification_token VARCHAR(255),
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_md_tenant_domain_domain UNIQUE (domain),
    CONSTRAINT fk_md_tenant_domain_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_md_tenant_domain_status
        FOREIGN KEY (status_id)
        REFERENCES di_domain_status(id)
);
