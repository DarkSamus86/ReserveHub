CREATE TABLE tenant_service.sys_create_tenant_request (
    operation_id UUID PRIMARY KEY,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    tenant_id UUID REFERENCES tenant_service.md_tenant(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
