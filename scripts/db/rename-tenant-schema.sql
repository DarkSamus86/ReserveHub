DO $schema$
BEGIN
    IF to_regnamespace('tenant') IS NOT NULL THEN
        IF to_regnamespace('tenant_service') IS NOT NULL THEN
            RAISE EXCEPTION 'both tenant and tenant_service schemas exist';
        END IF;
        IF NOT EXISTS (SELECT 1 FROM tenant.sys_schema_migration WHERE version >= 22 AND NOT dirty) THEN
            RAISE EXCEPTION 'tenant schema migration state is incomplete';
        END IF;
    ELSIF to_regnamespace('tenant_service') IS NULL THEN
        RAISE EXCEPTION 'neither tenant nor tenant_service schema exists';
    ELSE
        IF NOT EXISTS (SELECT 1 FROM tenant_service.sys_schema_migration WHERE version = 22 AND NOT dirty) THEN
            RAISE EXCEPTION 'tenant schema migrations have not reached version 22';
        END IF;
        EXECUTE 'ALTER SCHEMA tenant_service RENAME TO tenant';
    END IF;
END
$schema$;
