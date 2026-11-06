BEGIN;

-- A create request is reserved before inserting the provider, then linked to the
-- committed provider in the same transaction. The hash includes tenant and body.
CREATE TABLE provider.sys_create_provider_request (
    operation_id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    provider_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_create_provider_request_provider
        FOREIGN KEY (tenant_id, provider_id)
        REFERENCES provider.md_provider (tenant_id, id)
        ON DELETE RESTRICT
);

COMMIT;
