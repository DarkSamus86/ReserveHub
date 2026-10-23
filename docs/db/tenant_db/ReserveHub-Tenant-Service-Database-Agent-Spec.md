# ReserveHub Tenant Service — Database Implementation Specification

## 1. Назначение документа

Этот документ является технической спецификацией для реализации базы данных микросервиса `tenant-service` проекта **ReserveHub**.

Документ предназначен для разработчика или AI-агента, который должен:

1. создать PostgreSQL-схему;
2. подготовить SQL migrations;
3. реализовать все таблицы;
4. реализовать связи;
5. реализовать constraints;
6. реализовать indexes;
7. добавить базовые dictionary values;
8. не добавлять сущности, которые не описаны в данном документе;
9. соблюдать naming convention ReserveHub.

---

# 2. Общий database naming convention ReserveHub

Во всех микросервисах ReserveHub таблицы называются с префиксом, обозначающим их назначение.

```text
di_  -> dictionary / справочник
md_  -> master data / основная бизнес-сущность
tr_  -> transaction / транзакционные данные
ln_  -> link / таблица связи
hs_  -> history / исторические данные
cfg_ -> configuration / конфигурация
sys_ -> system / инфраструктурные таблицы
aud_ -> audit / аудит
agg_ -> aggregate / агрегированные данные
```

Правила:

- не использовать `de_`;
- не использовать `dict_`;
- не использовать названия без префикса;
- не добавлять имя сервиса в имя каждой таблицы;
- использовать `snake_case`.

Пример:

```text
di_tenant_status
md_tenant
cfg_tenant_settings
hs_tenant_plan_assignment
ln_plan_feature
sys_outbox_event
```

---

# 3. Граница ответственности tenant-service

`tenant-service` управляет tenant-ами ReserveHub.

Tenant — организация-клиент платформы.

Примеры:

```text
Cinema Network
Concert Organizer
Theatre
Hotel Group
Tour Operator
Railway Provider
Airline
```

`tenant-service` отвечает только за:

- tenant lifecycle;
- tenant settings;
- tenant domains;
- tenant branding;
- subscription plan assignment;
- feature availability;
- tenant limits;
- tenant contacts;
- tenant-related outbox events.

Не хранить здесь:

- users;
- identities;
- providers business data;
- orders;
- reservations;
- payments;
- tickets;
- inventory;
- catalog items;
- analytics facts.

---

# 4. Требования к СУБД

Использовать PostgreSQL.

Все timestamps:

```text
TIMESTAMPTZ
```

Все основные идентификаторы:

```text
UUID
```

Если UUID генерируется в БД:

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
```

---

# 5. Normalization requirements

Схема должна соответствовать как минимум 1NF, 2NF и 3NF.

Запрещено хранить списки в одной колонке, например:

```text
domains = "a.com,b.com,c.com"
features = "A,B,C"
contacts = JSON array
```

Повторяющиеся или связанные сущности должны быть вынесены в отдельные таблицы.

---

# 6. Полный список таблиц tenant-service

```text
di_tenant_status
md_tenant
cfg_tenant_settings

di_domain_status
md_tenant_domain

di_branding_status
cfg_tenant_branding

md_subscription_plan
hs_tenant_plan_assignment

di_limit_type
ln_plan_limit
cfg_tenant_limit_override

di_feature
ln_plan_feature
cfg_tenant_feature_override

md_tenant_contact

sys_outbox_event
```

---

# 7. di_tenant_status

```sql
CREATE TABLE di_tenant_status (
    id SMALLINT PRIMARY KEY,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(64) NOT NULL,
    description TEXT
);
```

Seed:

```text
1 | PENDING   | Pending
2 | ACTIVE    | Active
3 | SUSPENDED | Suspended
4 | ARCHIVED  | Archived
```

---

# 8. md_tenant

```sql
CREATE TABLE md_tenant (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    status_id SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    activated_at TIMESTAMPTZ,
    suspended_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,

    CONSTRAINT uq_md_tenant_slug UNIQUE (slug),

    CONSTRAINT fk_md_tenant_status
        FOREIGN KEY (status_id)
        REFERENCES di_tenant_status(id)
);
```

Требования:

- `slug` уникален;
- hard delete не использовать в бизнес-логике;
- архивирование через `ARCHIVED`;
- `activated_at`, `suspended_at`, `archived_at` заполняются соответствующими операциями.

---

# 9. cfg_tenant_settings

```sql
CREATE TABLE cfg_tenant_settings (
    tenant_id UUID PRIMARY KEY,
    timezone VARCHAR(64) NOT NULL,
    currency_code CHAR(3) NOT NULL,
    locale_code VARCHAR(16) NOT NULL,
    default_language VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_cfg_tenant_settings_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE
);
```

Связь:

```text
md_tenant 1 -> 1 cfg_tenant_settings
```

---

# 10. di_domain_status

```sql
CREATE TABLE di_domain_status (
    id SMALLINT PRIMARY KEY,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(64) NOT NULL
);
```

Seed:

```text
1 | PENDING
2 | VERIFIED
3 | FAILED
4 | DISABLED
```

---

# 11. md_tenant_domain

```sql
CREATE TABLE md_tenant_domain (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    domain VARCHAR(253) NOT NULL,
    status_id SMALLINT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    verification_token VARCHAR(255),
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_md_tenant_domain_domain UNIQUE (domain),

    CONSTRAINT fk_md_tenant_domain_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_md_tenant_domain_status
        FOREIGN KEY (status_id)
        REFERENCES di_domain_status(id)
);
```

У одного tenant может быть максимум один primary domain:

```sql
CREATE UNIQUE INDEX uq_md_tenant_domain_primary
ON md_tenant_domain (tenant_id)
WHERE is_primary = true;
```

---

# 12. di_branding_status

```sql
CREATE TABLE di_branding_status (
    id SMALLINT PRIMARY KEY,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(64) NOT NULL
);
```

Seed:

```text
1 | DEFAULT
2 | DRAFT
3 | ACTIVE
4 | DISABLED
```

---

# 13. cfg_tenant_branding

```sql
CREATE TABLE cfg_tenant_branding (
    tenant_id UUID PRIMARY KEY,
    status_id SMALLINT NOT NULL,
    display_name VARCHAR(255),
    logo_url TEXT,
    favicon_url TEXT,
    primary_color VARCHAR(7),
    secondary_color VARCHAR(7),
    accent_color VARCHAR(7),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_cfg_tenant_branding_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_cfg_tenant_branding_status
        FOREIGN KEY (status_id)
        REFERENCES di_branding_status(id),

    CONSTRAINT chk_cfg_tenant_branding_primary_color
        CHECK (primary_color IS NULL OR primary_color ~ '^#[0-9A-Fa-f]{6}$'),

    CONSTRAINT chk_cfg_tenant_branding_secondary_color
        CHECK (secondary_color IS NULL OR secondary_color ~ '^#[0-9A-Fa-f]{6}$'),

    CONSTRAINT chk_cfg_tenant_branding_accent_color
        CHECK (accent_color IS NULL OR accent_color ~ '^#[0-9A-Fa-f]{6}$')
);
```

---

# 14. md_subscription_plan

```sql
CREATE TABLE md_subscription_plan (
    id UUID PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Примеры:

```text
FREE
BASIC
BUSINESS
ENTERPRISE
```

Billing остается в `billing-service`.

---

# 15. hs_tenant_plan_assignment

```sql
CREATE TABLE hs_tenant_plan_assignment (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    plan_id UUID NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_hs_tenant_plan_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_hs_tenant_plan_plan
        FOREIGN KEY (plan_id)
        REFERENCES md_subscription_plan(id),

    CONSTRAINT chk_hs_tenant_plan_period
        CHECK (ends_at IS NULL OR ends_at > starts_at)
);
```

Не добавлять `current_plan_id` в `md_tenant`.

---

# 16. di_limit_type

```sql
CREATE TABLE di_limit_type (
    id UUID PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    unit VARCHAR(32) NOT NULL
);
```

Примеры:

```text
MAX_PROVIDERS
MAX_EMPLOYEES
MAX_EVENTS
MAX_DOMAINS
MAX_API_KEYS
MAX_MONTHLY_ORDERS
```

---

# 17. ln_plan_limit

```sql
CREATE TABLE ln_plan_limit (
    plan_id UUID NOT NULL,
    limit_type_id UUID NOT NULL,
    limit_value BIGINT NOT NULL,

    PRIMARY KEY (plan_id, limit_type_id),

    CONSTRAINT fk_ln_plan_limit_plan
        FOREIGN KEY (plan_id)
        REFERENCES md_subscription_plan(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_ln_plan_limit_type
        FOREIGN KEY (limit_type_id)
        REFERENCES di_limit_type(id),

    CONSTRAINT chk_ln_plan_limit_value
        CHECK (limit_value >= 0)
);
```

---

# 18. cfg_tenant_limit_override

```sql
CREATE TABLE cfg_tenant_limit_override (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    limit_type_id UUID NOT NULL,
    limit_value BIGINT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_cfg_tenant_limit_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_cfg_tenant_limit_type
        FOREIGN KEY (limit_type_id)
        REFERENCES di_limit_type(id),

    CONSTRAINT chk_cfg_tenant_limit_value
        CHECK (limit_value >= 0),

    CONSTRAINT chk_cfg_tenant_limit_period
        CHECK (ends_at IS NULL OR ends_at > starts_at)
);
```

Effective limit:

```text
tenant override
    ↓
если нет
    ↓
plan limit
```

---

# 19. di_feature

```sql
CREATE TABLE di_feature (
    id UUID PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT
);
```

Примеры:

```text
PROMO_CODES
ADVANCED_ANALYTICS
CUSTOM_BRANDING
CUSTOM_DOMAIN
LOYALTY
DYNAMIC_PRICING
AI_RECOMMENDATIONS
EXTERNAL_API
```

---

# 20. ln_plan_feature

```sql
CREATE TABLE ln_plan_feature (
    plan_id UUID NOT NULL,
    feature_id UUID NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,

    PRIMARY KEY (plan_id, feature_id),

    CONSTRAINT fk_ln_plan_feature_plan
        FOREIGN KEY (plan_id)
        REFERENCES md_subscription_plan(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_ln_plan_feature_feature
        FOREIGN KEY (feature_id)
        REFERENCES di_feature(id)
        ON DELETE CASCADE
);
```

---

# 21. cfg_tenant_feature_override

```sql
CREATE TABLE cfg_tenant_feature_override (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    feature_id UUID NOT NULL,
    enabled BOOLEAN NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_cfg_tenant_feature_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_cfg_tenant_feature_feature
        FOREIGN KEY (feature_id)
        REFERENCES di_feature(id),

    CONSTRAINT chk_cfg_tenant_feature_period
        CHECK (ends_at IS NULL OR ends_at > starts_at)
);
```

Effective feature:

```text
tenant override
    ↓
если нет
    ↓
plan feature
    ↓
если нет
    ↓
false
```

---

# 22. md_tenant_contact

```sql
CREATE TABLE md_tenant_contact (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    contact_type VARCHAR(32) NOT NULL,
    name VARCHAR(255),
    email VARCHAR(320),
    phone VARCHAR(32),
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_md_tenant_contact_tenant
        FOREIGN KEY (tenant_id)
        REFERENCES md_tenant(id)
        ON DELETE CASCADE
);
```

Initial contact types на application layer:

```text
TECHNICAL
BILLING
BUSINESS
SECURITY
```

---

# 23. sys_outbox_event

```sql
CREATE TABLE sys_outbox_event (
    id UUID PRIMARY KEY,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
```

События:

```text
tenant.created
tenant.activated
tenant.suspended
tenant.archived
tenant.plan.changed
tenant.feature.changed
tenant.limit.changed
tenant.domain.added
tenant.domain.verified
```

---

# 24. Required indexes

```sql
CREATE INDEX idx_md_tenant_status
ON md_tenant (status_id);

CREATE INDEX idx_md_tenant_domain_tenant
ON md_tenant_domain (tenant_id);

CREATE INDEX idx_md_tenant_domain_status
ON md_tenant_domain (status_id);

CREATE INDEX idx_hs_tenant_plan_assignment_lookup
ON hs_tenant_plan_assignment (
    tenant_id,
    starts_at,
    ends_at
);

CREATE INDEX idx_cfg_tenant_limit_override_lookup
ON cfg_tenant_limit_override (
    tenant_id,
    limit_type_id,
    starts_at,
    ends_at
);

CREATE INDEX idx_cfg_tenant_feature_override_lookup
ON cfg_tenant_feature_override (
    tenant_id,
    feature_id,
    starts_at,
    ends_at
);

CREATE INDEX idx_sys_outbox_event_unpublished
ON sys_outbox_event (created_at)
WHERE published_at IS NULL;
```

---

# 25. Required relationships

```text
di_tenant_status
    1
    │
    N
md_tenant
    │
    ├── 1:1 cfg_tenant_settings
    ├── 1:N md_tenant_domain
    ├── 1:1 cfg_tenant_branding
    ├── 1:N hs_tenant_plan_assignment
    ├── 1:N cfg_tenant_limit_override
    ├── 1:N cfg_tenant_feature_override
    └── 1:N md_tenant_contact
```

```text
md_subscription_plan
    ├── 1:N hs_tenant_plan_assignment
    ├── N:M di_limit_type via ln_plan_limit
    └── N:M di_feature via ln_plan_feature
```

---

# 26. Create Tenant transaction

Операция создания tenant должна быть атомарной.

```text
BEGIN

INSERT md_tenant
INSERT cfg_tenant_settings
INSERT cfg_tenant_branding
INSERT hs_tenant_plan_assignment
INSERT sys_outbox_event (tenant.created)

COMMIT
```

При ошибке:

```text
ROLLBACK
```

---

# 27. Required business behavior

## Create Tenant

- validate name;
- validate slug;
- ensure slug uniqueness;
- set initial status;
- create settings;
- create default branding;
- assign initial plan;
- emit `tenant.created`.

## Activate Tenant

```text
status = ACTIVE
activated_at = now()
updated_at = now()
```

Emit `tenant.activated`.

## Suspend Tenant

```text
status = SUSPENDED
suspended_at = now()
updated_at = now()
```

Emit `tenant.suspended`.

## Archive Tenant

```text
status = ARCHIVED
archived_at = now()
updated_at = now()
```

Emit `tenant.archived`.

Не использовать hard delete в бизнес-операции.

---

# 28. Current plan query

```sql
SELECT
    p.id,
    p.code,
    p.name,
    p.description
FROM hs_tenant_plan_assignment h
JOIN md_subscription_plan p
    ON p.id = h.plan_id
WHERE h.tenant_id = $1
  AND h.starts_at <= now()
  AND (
      h.ends_at IS NULL
      OR h.ends_at > now()
  )
ORDER BY h.starts_at DESC
LIMIT 1;
```

---

# 29. Effective limit algorithm

```text
1. Find active cfg_tenant_limit_override.
2. If found -> return override.
3. Resolve current plan.
4. Find ln_plan_limit.
5. Return plan limit.
6. If none -> no configured limit.
```

---

# 30. Effective feature algorithm

```text
1. Find active cfg_tenant_feature_override.
2. If found -> return enabled.
3. Resolve current plan.
4. Find ln_plan_feature.
5. If found -> return enabled.
6. Otherwise -> false.
```

Не добавлять boolean columns в `md_tenant` для каждой feature.

---

# 31. Migration order

```text
000001_enable_extensions
000002_create_di_tenant_status
000003_create_md_tenant
000004_create_cfg_tenant_settings
000005_create_di_domain_status
000006_create_md_tenant_domain
000007_create_di_branding_status
000008_create_cfg_tenant_branding
000009_create_md_subscription_plan
000010_create_hs_tenant_plan_assignment
000011_create_di_limit_type
000012_create_ln_plan_limit
000013_create_cfg_tenant_limit_override
000014_create_di_feature
000015_create_ln_plan_feature
000016_create_cfg_tenant_feature_override
000017_create_md_tenant_contact
000018_create_sys_outbox_event
000019_create_indexes
000020_seed_dictionaries
```

Если migration tool поддерживает `up/down`, создать обе версии.

---

# 32. Migration directory

```text
services/
└── tenant-service/
    └── migrations/
        ├── 000001_enable_extensions.up.sql
        ├── 000001_enable_extensions.down.sql
        ├── 000002_create_di_tenant_status.up.sql
        ├── 000002_create_di_tenant_status.down.sql
        ├── ...
        └── 000020_seed_dictionaries.up.sql
```

---

# 33. Seed data

Обязательно seed:

```text
di_tenant_status
di_domain_status
di_branding_status
```

Допустимый initial seed:

```text
md_subscription_plan
di_limit_type
di_feature
ln_plan_limit
ln_plan_feature
```

Не добавлять demo tenant-ов в production migrations.

---

# 34. Database constraints

БД должна обеспечивать:

```text
unique tenant slug
unique domain
one primary domain per tenant
non-negative limits
valid date ranges
valid HEX branding colors
foreign-key integrity
```

Application layer дополнительно валидирует:

```text
slug format
timezone
ISO 4217 currency
locale
domain syntax
email
phone
```

---

# 35. Do not implement

Не создавать:

- user tables;
- provider business tables;
- order/payment tables;
- reservation tables;
- analytics tables;
- cross-database foreign keys;
- generic JSONB вместо нормализованных core-таблиц;
- shared tables для нескольких микросервисов.

---

# 36. Outbox requirements

Изменение бизнес-сущности и запись outbox event выполняются в одной транзакции.

Пример:

```text
BEGIN
UPDATE md_tenant
INSERT sys_outbox_event
COMMIT
```

Outbox publisher позже публикует события в NATS JetStream.

---

# 37. Expected final structure

```text
tenant_db
│
├── di_tenant_status
├── md_tenant
├── cfg_tenant_settings
│
├── di_domain_status
├── md_tenant_domain
│
├── di_branding_status
├── cfg_tenant_branding
│
├── md_subscription_plan
├── hs_tenant_plan_assignment
│
├── di_limit_type
├── ln_plan_limit
├── cfg_tenant_limit_override
│
├── di_feature
├── ln_plan_feature
├── cfg_tenant_feature_override
│
├── md_tenant_contact
│
└── sys_outbox_event
```

---

# 38. Acceptance criteria

Implementation считается готовой, когда:

- все таблицы из спецификации созданы;
- названия таблиц совпадают со спецификацией;
- PK/FK реализованы;
- UNIQUE реализованы;
- CHECK constraints реализованы;
- required indexes созданы;
- dictionary seed присутствует;
- migrations применяются к пустой PostgreSQL;
- down migrations корректно откатываются;
- create tenant выполняется одной транзакцией;
- current plan определяется из history;
- effective limits вычисляются;
- effective features вычисляются;
- primary domain uniqueness enforced;
- tenant не hard-delete-ится бизнес-операцией;
- outbox работает атомарно;
- в схеме нет таблиц других микросервисов.

---

# 39. Implementation priority

```text
1. Core tenant
2. Settings
3. Plans
4. Features
5. Limits
6. Domains
7. Branding
8. Contacts
9. Outbox
10. Indexes
11. Seed
12. Repository queries
13. Integration tests
```

---

# 40. Architectural rule

`tenant-service` полностью владеет `tenant_db`.

Другие сервисы могут хранить `tenant_id`, но не должны напрямую читать таблицы `tenant_db`.

Межсервисное взаимодействие:

```text
gRPC
NATS JetStream
```

Не использовать прямой SQL-доступ к БД другого микросервиса.

---

# 41. Final instruction to implementation agent

Реализовать БД строго по этой спецификации.

Если обнаружена неоднозначность:

1. сохранить нормализацию;
2. сохранить ownership границы `tenant-service`;
3. предпочесть явную реляционную модель вместо JSONB;
4. не добавлять несвязанные сущности;
5. не переименовывать таблицы молча;
6. задокументировать любое необходимое отклонение до реализации.
