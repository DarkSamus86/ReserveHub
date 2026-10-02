INSERT INTO di_tenant_status (id, code, name, description) VALUES
    (1, 'PENDING', 'Pending', 'Tenant is waiting for activation'),
    (2, 'ACTIVE', 'Active', 'Tenant is active'),
    (3, 'SUSPENDED', 'Suspended', 'Tenant is temporarily suspended'),
    (4, 'ARCHIVED', 'Archived', 'Tenant is archived');

INSERT INTO di_domain_status (id, code, name) VALUES
    (1, 'PENDING', 'Pending'),
    (2, 'VERIFIED', 'Verified'),
    (3, 'FAILED', 'Failed'),
    (4, 'DISABLED', 'Disabled');

INSERT INTO di_branding_status (id, code, name) VALUES
    (1, 'DEFAULT', 'Default'),
    (2, 'DRAFT', 'Draft'),
    (3, 'ACTIVE', 'Active'),
    (4, 'DISABLED', 'Disabled');

INSERT INTO md_subscription_plan (id, code, name, description) VALUES
    ('10000000-0000-0000-0000-000000000001', 'FREE', 'Free', 'Free plan'),
    ('10000000-0000-0000-0000-000000000002', 'BASIC', 'Basic', 'Basic plan'),
    ('10000000-0000-0000-0000-000000000003', 'BUSINESS', 'Business', 'Business plan'),
    ('10000000-0000-0000-0000-000000000004', 'ENTERPRISE', 'Enterprise', 'Enterprise plan');

INSERT INTO di_limit_type (id, code, name, unit) VALUES
    ('20000000-0000-0000-0000-000000000001', 'MAX_PROVIDERS', 'Maximum providers', 'count'),
    ('20000000-0000-0000-0000-000000000002', 'MAX_EMPLOYEES', 'Maximum employees', 'count'),
    ('20000000-0000-0000-0000-000000000003', 'MAX_EVENTS', 'Maximum events', 'count'),
    ('20000000-0000-0000-0000-000000000004', 'MAX_DOMAINS', 'Maximum domains', 'count'),
    ('20000000-0000-0000-0000-000000000005', 'MAX_API_KEYS', 'Maximum API keys', 'count'),
    ('20000000-0000-0000-0000-000000000006', 'MAX_MONTHLY_ORDERS', 'Maximum monthly orders', 'count/month');

INSERT INTO di_feature (id, code, name) VALUES
    ('30000000-0000-0000-0000-000000000001', 'PROMO_CODES', 'Promo codes'),
    ('30000000-0000-0000-0000-000000000002', 'ADVANCED_ANALYTICS', 'Advanced analytics'),
    ('30000000-0000-0000-0000-000000000003', 'CUSTOM_BRANDING', 'Custom branding'),
    ('30000000-0000-0000-0000-000000000004', 'CUSTOM_DOMAIN', 'Custom domain'),
    ('30000000-0000-0000-0000-000000000005', 'LOYALTY', 'Loyalty'),
    ('30000000-0000-0000-0000-000000000006', 'DYNAMIC_PRICING', 'Dynamic pricing'),
    ('30000000-0000-0000-0000-000000000007', 'AI_RECOMMENDATIONS', 'AI recommendations'),
    ('30000000-0000-0000-0000-000000000008', 'EXTERNAL_API', 'External API');

INSERT INTO ln_plan_limit (plan_id, limit_type_id, limit_value) VALUES
    ('10000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000004', 1),
    ('10000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000004', 5);

INSERT INTO ln_plan_feature (plan_id, feature_id, enabled) VALUES
    ('10000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000004', false),
    ('10000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000004', true);
