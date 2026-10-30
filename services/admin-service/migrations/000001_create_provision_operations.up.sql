CREATE SCHEMA IF NOT EXISTS admin_service;

CREATE TABLE admin_service.provision_operations (
    operation_id UUID PRIMARY KEY,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    actor_id UUID NOT NULL,
    owner_email TEXT NOT NULL,
    tenant_request JSONB NOT NULL,
    tenant_id UUID,
    status TEXT NOT NULL CHECK (status IN ('requested', 'retryable_failure', 'failed', 'completed')),
    invite_status TEXT,
    safe_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
