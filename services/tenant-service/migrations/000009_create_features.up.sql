BEGIN;

SET LOCAL search_path = tenant_service, public;

CREATE TABLE features (
    id UUID PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT
);

INSERT INTO features (id, code, name) VALUES
    ('30000000-0000-0000-0000-000000000001', 'PROMO_CODES', 'Promo codes'),
    ('30000000-0000-0000-0000-000000000002', 'ADVANCED_ANALYTICS', 'Advanced analytics'),
    ('30000000-0000-0000-0000-000000000003', 'CUSTOM_BRANDING', 'Custom branding'),
    ('30000000-0000-0000-0000-000000000004', 'CUSTOM_DOMAIN', 'Custom domain'),
    ('30000000-0000-0000-0000-000000000005', 'LOYALTY', 'Loyalty'),
    ('30000000-0000-0000-0000-000000000006', 'DYNAMIC_PRICING', 'Dynamic pricing'),
    ('30000000-0000-0000-0000-000000000007', 'AI_RECOMMENDATIONS', 'AI recommendations'),
    ('30000000-0000-0000-0000-000000000008', 'EXTERNAL_API', 'External API');

CREATE TABLE plan_features (
    plan_id UUID NOT NULL,
    feature_id UUID NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,

    PRIMARY KEY (plan_id, feature_id),
    CONSTRAINT fk_plan_features_plan
        FOREIGN KEY (plan_id)
        REFERENCES subscription_plans(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_plan_features_feature
        FOREIGN KEY (feature_id)
        REFERENCES features(id)
        ON DELETE CASCADE
);

CREATE TABLE tenant_feature_overrides (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    feature_id UUID NOT NULL,
    enabled BOOLEAN NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_tenant_feature_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES tenants(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_tenant_feature_feature
        FOREIGN KEY (feature_id)
        REFERENCES features(id),
    CONSTRAINT chk_tenant_feature_period
        CHECK (ends_at IS NULL OR ends_at > starts_at)
);

CREATE INDEX idx_tenant_feature_overrides_lookup
    ON tenant_feature_overrides (tenant_id, feature_id, starts_at, ends_at);

COMMIT;
