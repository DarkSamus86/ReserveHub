CREATE TABLE cfg_tenant_feature_override (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    feature_id UUID NOT NULL,
    enabled BOOLEAN NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_cfg_tenant_feature_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_cfg_tenant_feature_feature
        FOREIGN KEY (feature_id)
        REFERENCES di_feature(id),
    CONSTRAINT chk_cfg_tenant_feature_period
        CHECK (ends_at IS NULL OR ends_at > starts_at)
);
