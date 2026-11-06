BEGIN;

CREATE TABLE provider.md_provider (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    city VARCHAR(160) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_md_provider_name CHECK (name = btrim(name) AND length(name) > 0),
    CONSTRAINT ck_md_provider_city CHECK (city = btrim(city) AND length(city) > 0),
    CONSTRAINT ck_md_provider_status CHECK (status IN ('ACTIVE', 'SUSPENDED')),
    CONSTRAINT uq_md_provider_tenant_id_id UNIQUE (tenant_id, id)
);

-- Tenant-first ordering supports bounded directory reads without an index per status.
CREATE INDEX ix_md_provider_tenant_created
    ON provider.md_provider (tenant_id, created_at DESC, id DESC);

CREATE FUNCTION provider.reject_provider_tenant_reassignment()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id THEN
        RAISE EXCEPTION 'provider tenant_id is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END
$function$;

CREATE TRIGGER tr_md_provider_tenant_immutable
    BEFORE UPDATE ON provider.md_provider
    FOR EACH ROW EXECUTE FUNCTION provider.reject_provider_tenant_reassignment();

COMMIT;
