BEGIN;

-- The catalog is global to Provider. Stable IDs keep API references reproducible.
CREATE TABLE provider.md_country (
    id UUID PRIMARY KEY,
    code VARCHAR(2) NOT NULL UNIQUE,
    name_ru VARCHAR(128) NOT NULL,
    name_en VARCHAR(128) NOT NULL,
    CONSTRAINT ck_md_country_code CHECK (code ~ '^[A-Z]{2}$'),
    CONSTRAINT ck_md_country_names CHECK (
        name_ru = btrim(name_ru) AND length(name_ru) > 0 AND
        name_en = btrim(name_en) AND length(name_en) > 0
    )
);

CREATE TABLE provider.md_city (
    id UUID PRIMARY KEY,
    country_id UUID NOT NULL REFERENCES provider.md_country (id) ON DELETE RESTRICT,
    name_ru VARCHAR(160) NOT NULL,
    name_en VARCHAR(160) NOT NULL,
    CONSTRAINT ck_md_city_names CHECK (
        name_ru = btrim(name_ru) AND length(name_ru) > 0 AND
        name_en = btrim(name_en) AND length(name_en) > 0
    )
);
CREATE UNIQUE INDEX uq_md_city_country_name_ru ON provider.md_city (country_id, lower(name_ru));
CREATE UNIQUE INDEX uq_md_city_country_name_en ON provider.md_city (country_id, lower(name_en));
CREATE INDEX ix_md_city_country_name ON provider.md_city (country_id, name_ru, id);

INSERT INTO provider.md_country (id, code, name_ru, name_en) VALUES
    ('30000000-0000-0000-0000-000000000001', 'KZ', 'Казахстан', 'Kazakhstan');
INSERT INTO provider.md_city (id, country_id, name_ru, name_en) VALUES
    ('31000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 'Астана', 'Astana'),
    ('31000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001', 'Алматы', 'Almaty');

-- Keep free-text city for old binaries and unresolved historic values.
ALTER TABLE provider.md_provider ADD COLUMN city_id UUID;
ALTER TABLE provider.md_provider ADD CONSTRAINT fk_md_provider_city
    FOREIGN KEY (city_id) REFERENCES provider.md_city (id) ON DELETE RESTRICT;
CREATE INDEX ix_md_provider_city_id ON provider.md_provider (city_id) WHERE city_id IS NOT NULL;

-- Only recognized exact names are safe to map without a country on old records.
UPDATE provider.md_provider AS p
SET city_id = c.id
FROM provider.md_city AS c
WHERE lower(p.city) IN (lower(c.name_ru), lower(c.name_en));

-- During a rolling deploy, an old binary may change city without setting city_id.
-- Clear an association that no longer matches the new text.
CREATE FUNCTION provider.validate_provider_city_id()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
    IF TG_OP = 'UPDATE'
       AND NEW.city IS DISTINCT FROM OLD.city
       AND NEW.city_id IS NOT DISTINCT FROM OLD.city_id
       AND NEW.city_id IS NOT NULL
       AND NOT EXISTS (
           SELECT 1 FROM provider.md_city AS c
           WHERE c.id = NEW.city_id
             AND lower(NEW.city) IN (lower(c.name_ru), lower(c.name_en))
       ) THEN
        NEW.city_id := NULL;
    END IF;
    IF NEW.city_id IS NOT NULL
       AND NOT EXISTS (
           SELECT 1 FROM provider.md_city AS c
           WHERE c.id = NEW.city_id
             AND lower(NEW.city) IN (lower(c.name_ru), lower(c.name_en))
       ) THEN
        RAISE EXCEPTION 'provider city does not match city_id' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END
$function$;
CREATE TRIGGER tr_md_provider_validate_city_id
    BEFORE INSERT OR UPDATE OF city, city_id ON provider.md_provider
    FOR EACH ROW EXECUTE FUNCTION provider.validate_provider_city_id();

COMMIT;
