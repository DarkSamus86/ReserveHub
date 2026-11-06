DO $schema$
BEGIN
    IF to_regnamespace('admin') IS NOT NULL THEN
        IF to_regnamespace('admin_service') IS NOT NULL THEN
            RAISE EXCEPTION 'both admin and admin_service schemas exist';
        END IF;
        IF NOT EXISTS (SELECT 1 FROM admin.sys_schema_migration WHERE version >= 5 AND NOT dirty) THEN
            RAISE EXCEPTION 'admin schema migration state is incomplete';
        END IF;
    ELSIF to_regnamespace('admin_service') IS NULL THEN
        RAISE EXCEPTION 'neither admin nor admin_service schema exists';
    ELSE
        IF NOT EXISTS (SELECT 1 FROM admin_service.sys_schema_migration WHERE version = 5 AND NOT dirty) THEN
            RAISE EXCEPTION 'admin schema migrations have not reached version 5';
        END IF;
        EXECUTE 'ALTER SCHEMA admin_service RENAME TO admin';
    END IF;
END
$schema$;
