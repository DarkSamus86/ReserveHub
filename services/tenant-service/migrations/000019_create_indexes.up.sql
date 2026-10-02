CREATE INDEX idx_md_tenant_status
    ON md_tenant (status_id);

CREATE UNIQUE INDEX uq_md_tenant_domain_primary
    ON md_tenant_domain (tenant_id)
    WHERE is_primary = true;

CREATE INDEX idx_md_tenant_domain_tenant
    ON md_tenant_domain (tenant_id);

CREATE INDEX idx_md_tenant_domain_status
    ON md_tenant_domain (status_id);

CREATE INDEX idx_hs_tenant_plan_assignment_lookup
    ON hs_tenant_plan_assignment (tenant_id, starts_at, ends_at);

CREATE INDEX idx_cfg_tenant_limit_override_lookup
    ON cfg_tenant_limit_override (tenant_id, limit_type_id, starts_at, ends_at);

CREATE INDEX idx_cfg_tenant_feature_override_lookup
    ON cfg_tenant_feature_override (tenant_id, feature_id, starts_at, ends_at);

CREATE INDEX idx_sys_outbox_event_unpublished
    ON sys_outbox_event (created_at)
    WHERE published_at IS NULL;
