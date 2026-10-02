CREATE TABLE hs_tenant_plan_assignment (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    plan_id UUID NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_hs_tenant_plan_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_hs_tenant_plan_plan
        FOREIGN KEY (plan_id)
        REFERENCES md_subscription_plan(id),
    CONSTRAINT chk_hs_tenant_plan_period
        CHECK (ends_at IS NULL OR ends_at > starts_at)
);
