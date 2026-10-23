CREATE TABLE cfg_tenant_limit_override (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    limit_type_id UUID NOT NULL,
    limit_value BIGINT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_cfg_tenant_limit_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_cfg_tenant_limit_type
        FOREIGN KEY (limit_type_id)
        REFERENCES di_limit_type(id),
    CONSTRAINT chk_cfg_tenant_limit_value
        CHECK (limit_value >= 0),
    CONSTRAINT chk_cfg_tenant_limit_period
        CHECK (ends_at IS NULL OR ends_at > starts_at)
);
