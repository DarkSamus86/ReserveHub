DO $schema$
BEGIN
    IF to_regnamespace('auth') IS NOT NULL THEN
        IF to_regnamespace('auth_service') IS NOT NULL THEN
            RAISE EXCEPTION 'both auth and auth_service schemas exist';
        END IF;
        IF NOT EXISTS (SELECT 1 FROM auth.sys_schema_migration WHERE version >= 7 AND NOT dirty) THEN
            RAISE EXCEPTION 'auth schema migration state is incomplete';
        END IF;
    ELSIF to_regnamespace('auth_service') IS NULL THEN
        RAISE EXCEPTION 'neither auth nor auth_service schema exists';
    ELSE
        IF NOT EXISTS (SELECT 1 FROM auth_service.sys_schema_migration WHERE version = 7 AND NOT dirty) THEN
            RAISE EXCEPTION 'auth schema migrations have not reached version 7';
        END IF;
        EXECUTE 'ALTER SCHEMA auth_service RENAME TO auth';
    END IF;
END
$schema$;
