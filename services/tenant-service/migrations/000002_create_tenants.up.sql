BEGIN;

SET LOCAL search_path = tenant_service, public;

CREATE TABLE tenants (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    status_id SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    activated_at TIMESTAMPTZ,
    suspended_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,

    CONSTRAINT uq_tenants_slug UNIQUE (slug),
    CONSTRAINT fk_tenants_status
        FOREIGN KEY (status_id)
        REFERENCES tenant_statuses(id)
);

CREATE INDEX idx_tenants_status ON tenants (status_id);

COMMIT;
