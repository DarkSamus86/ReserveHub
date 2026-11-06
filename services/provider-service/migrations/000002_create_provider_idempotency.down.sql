BEGIN;

LOCK TABLE provider.sys_create_provider_request IN ACCESS EXCLUSIVE MODE;

DO $block$
BEGIN
    IF EXISTS (SELECT 1 FROM provider.sys_create_provider_request) THEN
        RAISE EXCEPTION 'cannot drop provider idempotency table while requests exist';
    END IF;
END
$block$;

DROP TABLE provider.sys_create_provider_request;

COMMIT;
