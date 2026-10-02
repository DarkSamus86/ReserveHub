BEGIN;

SET LOCAL search_path = tenant_service, public;

CREATE TABLE domain_statuses (
    id SMALLINT PRIMARY KEY,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(64) NOT NULL
);

INSERT INTO domain_statuses (id, code, name) VALUES
    (1, 'PENDING', 'Pending'),
    (2, 'VERIFIED', 'Verified'),
    (3, 'FAILED', 'Failed'),
    (4, 'DISABLED', 'Disabled');

CREATE TABLE tenant_domains (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    domain VARCHAR(253) NOT NULL,
    status_id SMALLINT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    verification_token VARCHAR(255),
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_tenant_domains_domain UNIQUE (domain),
    CONSTRAINT fk_tenant_domains_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES tenants(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_tenant_domains_status
        FOREIGN KEY (status_id)
        REFERENCES domain_statuses(id)
);

CREATE UNIQUE INDEX uq_tenant_domains_primary
    ON tenant_domains (tenant_id)
    WHERE is_primary = true;

CREATE INDEX idx_tenant_domains_tenant ON tenant_domains (tenant_id);
CREATE INDEX idx_tenant_domains_status ON tenant_domains (status_id);

COMMIT;
