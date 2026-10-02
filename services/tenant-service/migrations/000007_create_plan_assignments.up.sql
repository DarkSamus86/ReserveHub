BEGIN;

SET LOCAL search_path = tenant_service, public;

CREATE TABLE tenant_plan_assignments (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    plan_id UUID NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_tenant_plan_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES tenants(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_tenant_plan_plan
        FOREIGN KEY (plan_id)
        REFERENCES subscription_plans(id),
    CONSTRAINT chk_tenant_plan_period
        CHECK (ends_at IS NULL OR ends_at > starts_at)
);

CREATE INDEX idx_tenant_plan_assignments_lookup
    ON tenant_plan_assignments (tenant_id, starts_at, ends_at);

COMMIT;
