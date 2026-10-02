BEGIN;

SET LOCAL search_path = tenant_service, public;

CREATE TABLE subscription_plans (
    id UUID PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO subscription_plans (id, code, name, description) VALUES
    ('10000000-0000-0000-0000-000000000001', 'FREE', 'Free', 'Free plan'),
    ('10000000-0000-0000-0000-000000000002', 'BASIC', 'Basic', 'Basic plan'),
    ('10000000-0000-0000-0000-000000000003', 'BUSINESS', 'Business', 'Business plan'),
    ('10000000-0000-0000-0000-000000000004', 'ENTERPRISE', 'Enterprise', 'Enterprise plan');

COMMIT;
