BEGIN;

-- Refuse to discard catalog records added after the initial KZ seed.
DO $guard$
BEGIN
    IF EXISTS (SELECT 1 FROM provider.md_country WHERE id <> '30000000-0000-0000-0000-000000000001')
       OR EXISTS (SELECT 1 FROM provider.md_city WHERE id NOT IN (
           '31000000-0000-0000-0000-000000000001',
           '31000000-0000-0000-0000-000000000002'
       )) THEN
        RAISE EXCEPTION 'cannot roll back geo catalog with added countries or cities';
    END IF;
END
$guard$;

DROP TRIGGER tr_md_provider_validate_city_id ON provider.md_provider;
DROP FUNCTION provider.validate_provider_city_id();
DROP INDEX provider.ix_md_provider_city_id;
ALTER TABLE provider.md_provider DROP CONSTRAINT fk_md_provider_city;
ALTER TABLE provider.md_provider DROP COLUMN city_id;
DROP TABLE provider.md_city;
DROP TABLE provider.md_country;

-- md_provider.city text remains intact, but its catalog association is lost.
COMMIT;
