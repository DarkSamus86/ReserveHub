BEGIN;

LOCK TABLE provider.md_provider IN ACCESS EXCLUSIVE MODE;

DO $block$
BEGIN
    IF EXISTS (SELECT 1 FROM provider.md_provider) THEN
        RAISE EXCEPTION 'cannot drop provider table while provider rows exist';
    END IF;
END
$block$;

DROP TABLE provider.md_provider;
DROP FUNCTION provider.reject_provider_tenant_reassignment();

COMMIT;
