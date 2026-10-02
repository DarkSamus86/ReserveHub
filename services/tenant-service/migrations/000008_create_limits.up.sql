BEGIN;

SET LOCAL search_path = tenant_service, public;

CREATE TABLE limit_types (
    id UUID PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    unit VARCHAR(32) NOT NULL
);

INSERT INTO limit_types (id, code, name, unit) VALUES
    ('20000000-0000-0000-0000-000000000001', 'MAX_PROVIDERS', 'Maximum providers', 'count'),
    ('20000000-0000-0000-0000-000000000002', 'MAX_EMPLOYEES', 'Maximum employees', 'count'),
    ('20000000-0000-0000-0000-000000000003', 'MAX_EVENTS', 'Maximum events', 'count'),
    ('20000000-0000-0000-0000-000000000004', 'MAX_DOMAINS', 'Maximum domains', 'count'),
    ('20000000-0000-0000-0000-000000000005', 'MAX_API_KEYS', 'Maximum API keys', 'count'),
    ('20000000-0000-0000-0000-000000000006', 'MAX_MONTHLY_ORDERS', 'Maximum monthly orders', 'count/month');

CREATE TABLE plan_limits (
    plan_id UUID NOT NULL,
    limit_type_id UUID NOT NULL,
    limit_value BIGINT NOT NULL,

    PRIMARY KEY (plan_id, limit_type_id),
    CONSTRAINT fk_plan_limits_plan
        FOREIGN KEY (plan_id)
        REFERENCES subscription_plans(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_plan_limits_type
        FOREIGN KEY (limit_type_id)
        REFERENCES limit_types(id),
    CONSTRAINT chk_plan_limits_value
        CHECK (limit_value >= 0)
);

CREATE TABLE tenant_limit_overrides (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    limit_type_id UUID NOT NULL,
    limit_value BIGINT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_tenant_limit_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES tenants(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_tenant_limit_type
        FOREIGN KEY (limit_type_id)
        REFERENCES limit_types(id),
    CONSTRAINT chk_tenant_limit_value
        CHECK (limit_value >= 0),
    CONSTRAINT chk_tenant_limit_period
        CHECK (ends_at IS NULL OR ends_at > starts_at)
);

CREATE INDEX idx_tenant_limit_overrides_lookup
    ON tenant_limit_overrides (tenant_id, limit_type_id, starts_at, ends_at);

COMMIT;
