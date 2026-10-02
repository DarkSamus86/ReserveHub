# ReserveHub — Tenant Service
## Полная функциональная и бизнес-спецификация

**Сервис:** `tenant-service`<br>
**Система:** ReserveHub<br>
**Хранилище:** PostgreSQL<br>
**Основная бизнес-сущность:** `Tenant`

---

# 1. Назначение

`tenant-service` — центральный сервис управления tenant-ами ReserveHub.

Tenant — отдельная организация/клиент/изолированное пространство платформы. Остальные сервисы работают в контексте `tenant_id`.

Сервис является source of truth для:

- существования tenant;
- текущего состояния tenant;
- базовых настроек;
- тарифного плана;
- лимитов;
- feature flags;
- доменов;
- branding;
- контактных данных;
- tenant-related integration events.

Он не владеет заказами, платежами, бронированиями, мероприятиями, пользователями и другой предметной бизнес-логикой.

---

# 2. Основные бизнес-процессы

## BP-01. Создание tenant
Создание tenant и его минимальной конфигурации.

## BP-02. Lifecycle tenant
Смена состояний `PENDING`, `ACTIVE`, `SUSPENDED`, `ARCHIVED`.

Операции:
- activate;
- suspend;
- resume;
- archive.

## BP-03. Tenant Settings
Управление timezone, currency, locale и default language.

## BP-04. Subscription Plan
Назначение, смена, история и определение текущего тарифного плана.

## BP-05. Limits
Каталог лимитов, лимиты тарифа, tenant overrides и вычисление effective limit.

## BP-06. Feature Flags
Каталог features, plan features, tenant overrides и вычисление effective feature.

## BP-07. Domains
Добавление, verification, primary domain, disable.

## BP-08. Branding
Display name, logo, favicon, colors и branding status.

## BP-09. Contacts
Technical, billing, business и security contacts.

## BP-10. Integration Events
Transactional outbox и публикация изменений другим сервисам.

---

# 3. Бизнес-сущность Tenant

```text
Tenant
├── id
├── name
├── slug
├── status
├── settings
├── domains[]
├── branding
├── plan history[]
├── effective limits
├── effective features
└── contacts[]
```

`Tenant` является aggregate root.

Все tenant-specific сущности должны ссылаться на `tenant_id`.

---

# 4. Границы ответственности

## Tenant Service отвечает за

- `tenants`;
- `tenant_statuses`;
- `tenant_settings`;
- `tenant_domains`;
- `domain_statuses`;
- `tenant_branding`;
- `branding_statuses`;
- `subscription_plans`;
- `tenant_plan_assignments`;
- `limit_types`;
- `plan_limits`;
- `tenant_limit_overrides`;
- `features`;
- `plan_features`;
- `tenant_feature_overrides`;
- `tenant_contacts`;
- `outbox_events`.

## Tenant Service не отвечает за

- authentication;
- users/passwords;
- orders;
- payments;
- bookings;
- events;
- sessions;
- seats;
- inventory;
- notifications;
- sales analytics;
- billing transactions.

---

# 5. Таблицы БД

## 5.1. `tenant_statuses`

Справочник состояний tenant.

Хранить:

```text
PENDING
ACTIVE
SUSPENDED
ARCHIVED
```

Заполняется migrations/seed. Обычные use cases не должны создавать новые статусы.

Читается при:
- CreateTenant;
- ActivateTenant;
- SuspendTenant;
- ResumeTenant;
- ArchiveTenant;
- GetTenant;
- ListTenants.

---

## 5.2. `tenants`

Главная таблица tenant.

Хранит:

- `id`;
- `name`;
- `slug`;
- `status_id`;
- timestamps lifecycle.

### Create

```text
INSERT tenants
```

Начальный status:

```text
PENDING
```

### Activate

```text
status_id = ACTIVE
activated_at = now()
updated_at = now()
```

### Suspend

```text
status_id = SUSPENDED
suspended_at = now()
updated_at = now()
```

### Resume

```text
status_id = ACTIVE
updated_at = now()
```

### Archive

```text
status_id = ARCHIVED
archived_at = now()
updated_at = now()
```

### В `tenants` НЕ хранить

- current plan;
- plan name;
- limits;
- features;
- domains;
- branding;
- timezone;
- currency.

Для этого существуют отдельные нормализованные таблицы.

---

## 5.3. `tenant_settings`

Связь:

```text
tenant 1 : 1 tenant_settings
```

Хранит:

```text
timezone
currency_code
locale_code
default_language
```

Пример:

```text
Asia/Almaty
KZT
ru-KZ
ru
```

Создавать рекомендуется в одной транзакции вместе с tenant.

---

## 5.4. `domain_statuses`

Справочник:

```text
PENDING
VERIFIED
FAILED
DISABLED
```

---

## 5.5. `tenant_domains`

Связь:

```text
tenant 1 : N domain
```

Хранит:

- domain;
- tenant_id;
- status;
- `is_primary`;
- verification token;
- `verified_at`.

Правила:

- domain глобально уникален;
- один tenant имеет максимум один primary domain;
- custom domain начинается с `PENDING`;
- primary domain должен быть `VERIFIED`.

---

## 5.6. `branding_statuses`

Справочник:

```text
DEFAULT
DRAFT
ACTIVE
DISABLED
```

---

## 5.7. `tenant_branding`

Связь:

```text
tenant 1 : 1 tenant_branding
```

Хранит:

```text
display_name
logo_url
favicon_url
primary_color
secondary_color
accent_color
status_id
```

Файлы логотипов/иконок хранить в S3/MinIO/object storage. В PostgreSQL хранится URL или object key.

---

## 5.8. `subscription_plans`

Каталог тарифов:

```text
FREE
BASIC
BUSINESS
ENTERPRISE
```

Хранит описание plan, но не факт владения tenant этим планом.

Не хранить здесь:

- `tenant_id`;
- payment;
- invoice;
- subscription charge.

Billing остается отдельным доменом.

---

## 5.9. `tenant_plan_assignments`

История назначения тарифов tenant.

Пример:

```text
01.01 → 31.03 BASIC
01.04 → 31.08 BUSINESS
01.09 → NULL ENTERPRISE
```

Current plan определяется:

```text
starts_at <= now()
AND (ends_at IS NULL OR ends_at > now())
```

При смене plan:

1. закрыть текущий assignment;
2. создать новый assignment.

Нельзя просто перезаписать историю.

---

## 5.10. `limit_types`

Каталог типов лимитов.

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

## 5.11. `plan_limits`

Стандартные лимиты тарифов.

Пример:

```text
BASIC:
MAX_DOMAINS = 1

BUSINESS:
MAX_DOMAINS = 5
```

---

## 5.12. `tenant_limit_overrides`

Индивидуальные overrides.

Пример:

```text
BUSINESS: MAX_DOMAINS = 5
tenant override: MAX_DOMAINS = 10
```

Effective limit:

```text
active tenant override
↓ если нет
current plan
↓
plan limit
```

---

## 5.13. `features`

Каталог возможностей ReserveHub.

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

## 5.14. `plan_features`

Feature availability для тарифного плана.

Пример:

```text
BASIC:
CUSTOM_DOMAIN = false

BUSINESS:
CUSTOM_DOMAIN = true
```

---

## 5.15. `tenant_feature_overrides`

Индивидуальное включение/выключение функции.

Effective feature:

```text
tenant override
↓ если нет
plan feature
↓ если нет
false
```

---

## 5.16. `tenant_contacts`

Контакты tenant.

Типы:

```text
TECHNICAL
BILLING
BUSINESS
SECURITY
```

Это не user accounts. Контакт может существовать без пользователя в auth-service.

---

## 5.17. `outbox_events`

Transactional Outbox.

Правильная запись:

```text
BEGIN

изменение business state
INSERT outbox_events

COMMIT
```

Publisher отдельно:

```text
read unpublished
↓
publish to broker
↓
success
↓
published_at = now()
```

---

# 6. BP-01 — Create Tenant

## Вход

```text
CreateTenant
{
    name
    slug
    timezone
    currency_code
    locale_code
    default_language
    plan_id
}
```

Опционально:

```text
branding
primary_domain
contacts
```

## Проверки

1. name заполнен;
2. slug валиден;
3. slug уникален;
4. timezone валиден;
5. currency валиден;
6. locale валиден;
7. plan существует;
8. plan активен.

## Транзакция

```text
BEGIN
```

1. `INSERT tenants` со status `PENDING`;
2. `INSERT tenant_settings`;
3. `INSERT tenant_branding` со status `DEFAULT`;
4. `INSERT tenant_plan_assignments`;
5. опционально `INSERT tenant_domains`;
6. опционально `INSERT tenant_contacts`;
7. `INSERT outbox_events` → `tenant.created`;

```text
COMMIT
```

Если любой шаг падает — rollback всей операции.

## Результат

```text
tenant_id
name
slug
status
created_at
```

---

# 7. BP-02 — Activate Tenant

Команда:

```text
ActivateTenant(tenant_id)
```

Предусловия:

- tenant существует;
- status = PENDING;
- settings существуют;
- current plan существует;
- plan активен.

Изменения:

```text
UPDATE tenants
SET status = ACTIVE,
    activated_at = now(),
    updated_at = now()
```

В той же транзакции:

```text
INSERT outbox_events
event_type = tenant.activated
```

---

# 8. BP-03 — Suspend Tenant

Команда:

```text
SuspendTenant(tenant_id)
```

Переход:

```text
ACTIVE -> SUSPENDED
```

Запись:

```text
UPDATE tenants
SET status = SUSPENDED,
    suspended_at = now(),
    updated_at = now()
```

Event:

```text
tenant.suspended
```

После события другие сервисы должны прекратить tenant-specific write operations, но не удалять данные.

---

# 9. BP-04 — Resume Tenant

Переход:

```text
SUSPENDED -> ACTIVE
```

Запись:

```text
UPDATE tenants
SET status = ACTIVE,
    updated_at = now()
```

Event:

```text
tenant.resumed
```

---

# 10. BP-05 — Archive Tenant

Логическое удаление.

Разрешенные переходы:

```text
PENDING -> ARCHIVED
ACTIVE -> ARCHIVED
SUSPENDED -> ARCHIVED
```

Запись:

```text
UPDATE tenants
SET status = ARCHIVED,
    archived_at = now(),
    updated_at = now()
```

Event:

```text
tenant.archived
```

Обычная операция удаления не выполняет `DELETE FROM tenants`.

---

# 11. BP-06 — Get / Update Tenant

## GetTenant

Основная таблица:

```text
tenants
```

Для details дополнительно:

```text
tenant_settings
tenant_branding
tenant_domains
tenant_plan_assignments
subscription_plans
```

## UpdateTenant

Разрешить изменение базовых полей:

```text
name
slug
```

Не использовать этот command для:

- plan;
- lifecycle status;
- limits;
- features;
- branding;
- domains;
- settings.

Для них отдельные use cases.

---

# 12. BP-07 — Tenant Settings

Команда:

```text
UpdateTenantSettings
```

Пишет:

```text
tenant_settings
```

Изменяемые значения:

```text
timezone
currency_code
locale_code
default_language
```

Flow:

```text
validate
↓
UPDATE tenant_settings
↓
UPDATE tenants.updated_at
↓
INSERT outbox tenant.settings.changed
```

---

# 13. BP-08 — Change Tenant Plan

Вход:

```text
tenant_id
new_plan_id
effective_from
```

Алгоритм:

1. tenant exists;
2. new plan exists;
3. new plan active;
4. найти current assignment;
5. закрыть его `ends_at = effective_from`;
6. создать новый assignment;
7. outbox event.

Транзакция обязательна.

Event payload:

```text
tenant_id
old_plan_id
new_plan_id
effective_from
```

Event:

```text
tenant.plan.changed
```

---

# 14. BP-09 — Get Current Plan

Читать:

```text
tenant_plan_assignments
JOIN subscription_plans
```

Условие:

```text
starts_at <= now()
AND (ends_at IS NULL OR ends_at > now())
ORDER BY starts_at DESC
LIMIT 1
```

---

# 15. BP-10 — Limits

## Platform configuration

`plan_limits` — лимиты тарифов.

## Tenant override

Command:

```text
SetTenantLimitOverride
```

Пишет:

```text
tenant_limit_overrides
```

Проверки:

- tenant exists;
- limit type exists;
- value >= 0;
- period valid.

Event:

```text
tenant.limit.changed
```

## Effective Limit

```text
GetEffectiveLimit(tenant_id, limit_code)
```

Алгоритм:

```text
active tenant override?
YES → override value
NO
↓
current plan
↓
plan_limits
↓
value
```

### Важная граница

Tenant Service хранит **разрешенный максимум**, но не usage чужого домена.

Пример:

```text
Tenant Service:
MAX_EVENTS = 100

Event Service:
current events = 99
```

Решение создать 100-е мероприятие принимает `event-service` на основании лимита от tenant-service.

---

# 16. BP-11 — Feature Flags

## Plan Feature

Пишется в:

```text
plan_features
```

## Tenant Override

Command:

```text
SetTenantFeatureOverride
```

Пишет:

```text
tenant_feature_overrides
```

Event:

```text
tenant.feature.changed
```

## Effective Feature

```text
HasFeature(tenant_id, feature_code)
```

Алгоритм:

```text
tenant override
↓
plan feature
↓
false
```

---

# 17. BP-12 — Add Domain

Вход:

```text
tenant_id
domain
```

Проверки:

1. tenant exists;
2. tenant не ARCHIVED;
3. domain format valid;
4. domain globally unique;
5. если custom domain — feature `CUSTOM_DOMAIN = true`;
6. effective `MAX_DOMAINS` не превышен.

Запись:

```text
INSERT tenant_domains
status = PENDING
is_primary = false
verification_token = generated
```

Event:

```text
tenant.domain.created
```

---

# 18. BP-13 — Verify Domain

Алгоритм:

```text
load tenant_domains
↓
read verification_token
↓
perform/check ownership verification
↓
success?
```

Success:

```text
status = VERIFIED
verified_at = now()
```

Failure:

```text
status = FAILED
```

Events:

```text
tenant.domain.verified
tenant.domain.verification_failed
```

---

# 19. BP-14 — Set Primary Domain

Проверки:

- domain belongs to tenant;
- status = VERIFIED;
- domain not DISABLED.

Транзакция:

```text
BEGIN

UPDATE tenant_domains
SET is_primary = false
WHERE tenant_id = ...

UPDATE tenant_domains
SET is_primary = true
WHERE id = ...

INSERT outbox event

COMMIT
```

Event:

```text
tenant.domain.primary_changed
```

---

# 20. BP-15 — Branding

Command:

```text
UpdateTenantBranding
```

Пишет:

```text
tenant_branding
```

Поля:

```text
display_name
logo_url
favicon_url
primary_color
secondary_color
accent_color
```

Цвет:

```text
#RRGGBB
```

Если custom branding является платной возможностью:

```text
HasFeature(CUSTOM_BRANDING) == true
```

Event:

```text
tenant.branding.changed
```

---

# 21. BP-16 — Contacts

Commands:

```text
AddTenantContact
UpdateTenantContact
RemoveTenantContact
ListTenantContacts
```

Таблица:

```text
tenant_contacts
```

Контактная сущность не должна смешиваться с auth/user entity.

---

# 22. Интеграция с Auth Service

Tenant Service не хранит passwords или user identity.

Ожидаемый flow:

```text
auth-service
↓
authenticate
↓
JWT / trusted context
{
    user_id
    tenant_id
    roles
}
```

Клиент не должен иметь возможность произвольно подменить tenant.

Плохо:

```text
GET /orders?tenant_id=other-tenant
```

Trusted `tenant_id` должен приходить из authenticated context / gateway metadata.

---

# 23. Интеграция с Billing Service

Tenant Service хранит:

```text
какой plan назначен tenant
```

Billing Service хранит:

```text
price
invoice
payment
debt
billing subscription
transactions
```

Пример:

```text
billing-service
payment overdue
↓
SuspendTenant
↓
tenant-service
↓
tenant.suspended
```

После восстановления оплаты:

```text
billing-service
↓
ResumeTenant
```

---

# 24. Интеграция с бизнес-сервисами

Другим сервисам нужны:

```text
tenant_id
tenant status
effective limits
effective features
```

Два режима:

## Синхронный

```text
business-service
↓
tenant-service gRPC
↓
response
```

## Event-driven

Tenant Service публикует изменения, а другие сервисы поддерживают local projection/cache.

Рекомендуется комбинировать оба подхода.

---

# 25. Integration Events

Рекомендуемый набор:

```text
tenant.created
tenant.updated

tenant.activated
tenant.suspended
tenant.resumed
tenant.archived

tenant.settings.changed

tenant.plan.changed

tenant.limit.changed

tenant.feature.changed

tenant.domain.created
tenant.domain.verified
tenant.domain.verification_failed
tenant.domain.disabled
tenant.domain.primary_changed

tenant.branding.changed

tenant.contact.changed
```

---

# 26. Transactional Outbox

Любое изменение, которое должно попасть в другие сервисы:

```text
BEGIN

business changes
INSERT outbox_events

COMMIT
```

Publisher:

```text
SELECT WHERE published_at IS NULL
↓
publish to NATS/Kafka
↓
success
↓
UPDATE published_at = now()
```

Пример payload:

```json
{
  "event_id": "uuid",
  "event_type": "tenant.suspended",
  "occurred_at": "2026-10-02T12:00:00Z",
  "tenant_id": "uuid",
  "data": {
    "status": "SUSPENDED"
  }
}
```

---

# 27. Commands

```text
CreateTenant
UpdateTenant

ActivateTenant
SuspendTenant
ResumeTenant
ArchiveTenant

UpdateTenantSettings

AssignTenantPlan
ChangeTenantPlan

SetTenantLimitOverride
RemoveTenantLimitOverride

SetTenantFeatureOverride
RemoveTenantFeatureOverride

AddTenantDomain
VerifyTenantDomain
SetPrimaryTenantDomain
DisableTenantDomain

UpdateTenantBranding

AddTenantContact
UpdateTenantContact
RemoveTenantContact
```

---

# 28. Queries

```text
GetTenant
ListTenants
GetTenantDetails

GetTenantSettings

GetCurrentPlan
GetPlanHistory

GetEffectiveLimit
GetEffectiveLimits

HasFeature
GetEffectiveFeatures

GetTenantDomains
GetPrimaryDomain

GetTenantBranding

GetTenantContacts
```

---

# 29. Рекомендуемый gRPC API

```proto
service TenantService {
  rpc CreateTenant(CreateTenantRequest) returns (Tenant);
  rpc GetTenant(GetTenantRequest) returns (Tenant);
  rpc ListTenants(ListTenantsRequest) returns (ListTenantsResponse);
  rpc UpdateTenant(UpdateTenantRequest) returns (Tenant);

  rpc ActivateTenant(ChangeTenantStateRequest) returns (Tenant);
  rpc SuspendTenant(ChangeTenantStateRequest) returns (Tenant);
  rpc ResumeTenant(ChangeTenantStateRequest) returns (Tenant);
  rpc ArchiveTenant(ChangeTenantStateRequest) returns (Tenant);

  rpc UpdateTenantSettings(UpdateTenantSettingsRequest)
      returns (TenantSettings);

  rpc ChangeTenantPlan(ChangeTenantPlanRequest)
      returns (TenantPlan);

  rpc GetEffectiveLimit(GetEffectiveLimitRequest)
      returns (EffectiveLimit);

  rpc GetEffectiveFeatures(GetEffectiveFeaturesRequest)
      returns (EffectiveFeatures);

  rpc AddTenantDomain(AddTenantDomainRequest)
      returns (TenantDomain);

  rpc VerifyTenantDomain(VerifyTenantDomainRequest)
      returns (TenantDomain);

  rpc SetPrimaryTenantDomain(SetPrimaryDomainRequest)
      returns (TenantDomain);

  rpc UpdateTenantBranding(UpdateTenantBrandingRequest)
      returns (TenantBranding);
}
```

---

# 30. Бизнес-правила

## BR-001
`tenant.slug` уникален глобально.

## BR-002
Tenant всегда имеет один текущий status.

## BR-003
`ARCHIVED` tenant не выполняет обычные business operations.

## BR-004
`SUSPENDED` tenant сохраняет данные, но tenant-specific writes должны блокироваться.

## BR-005
Current plan определяется через `tenant_plan_assignments`.

## BR-006
История plan не удаляется при обычной смене тарифа.

## BR-007
Effective limit:

```text
tenant override > plan limit
```

## BR-008
Effective feature:

```text
tenant override > plan feature > false
```

## BR-009
Domain уникален на всю платформу.

## BR-010
Один tenant имеет максимум один primary domain.

## BR-011
Primary domain должен быть VERIFIED.

## BR-012
Branding files хранятся вне PostgreSQL.

## BR-013
Важные tenant changes порождают outbox event.

## BR-014
Tenant Service не считает usage ресурсов других сервисов.

## BR-015
Tenant Service — source of truth для tenant configuration.

---

# 31. Domain Errors

```text
TENANT_NOT_FOUND
TENANT_SLUG_ALREADY_EXISTS
INVALID_TENANT_STATUS_TRANSITION

PLAN_NOT_FOUND
PLAN_INACTIVE
TENANT_PLAN_NOT_FOUND

LIMIT_TYPE_NOT_FOUND
INVALID_LIMIT_VALUE

FEATURE_NOT_FOUND

DOMAIN_ALREADY_EXISTS
DOMAIN_NOT_FOUND
DOMAIN_NOT_VERIFIED
DOMAIN_LIMIT_EXCEEDED

BRANDING_NOT_ALLOWED
INVALID_BRANDING_COLOR

CONTACT_NOT_FOUND
```

---

# 32. Полный Create Tenant Flow

```text
Platform Admin
      │
      ▼
API Gateway
      │
      ▼
Tenant Service
      │
      ├── validate request
      ├── check unique slug
      ├── load active plan
      │
      ├── BEGIN
      ├── INSERT tenants
      ├── INSERT tenant_settings
      ├── INSERT tenant_branding
      ├── INSERT tenant_plan_assignments
      ├── INSERT tenant_domains (optional)
      ├── INSERT tenant_contacts (optional)
      ├── INSERT outbox_events
      └── COMMIT
      │
      ▼
Tenant response
      │
      ▼
Outbox Publisher
      │
      ▼
Message Broker
      │
      ├── Auth/BFF
      ├── Event Service
      ├── Booking Service
      ├── Billing Service
      └── other subscribers
```

---

# 33. Limit Check Flow

```text
event-service
↓
current event count = 99
↓
GetEffectiveLimit(tenant_id, MAX_EVENTS)
↓
tenant-service returns 100
↓
99 < 100
↓
event-service creates event
```

Ответственность:

```text
Tenant Service:
"максимум 100"

Event Service:
"сейчас 99"
```

---

# 34. Feature Check Flow

```text
Business Service
↓
HasFeature(tenant_id, CUSTOM_DOMAIN)
↓
Tenant Service
↓
tenant_feature_overrides
↓ если нет
plan_features(current plan)
↓ если нет
false
```

---

# 35. Tenant Status как platform guard

```text
PENDING
→ setup/configuration

ACTIVE
→ normal operations

SUSPENDED
→ tenant business writes denied

ARCHIVED
→ tenant unavailable
```

Frontend не является защитным слоем. Критичные backend services должны учитывать tenant state через trusted context, API или local projection.

---

# 36. Рекомендуемая архитектура проекта

```text
tenant-service/
├── cmd/
│   └── tenant-service/
│
├── internal/
│   ├── domain/
│   │   ├── tenant/
│   │   ├── plan/
│   │   ├── limit/
│   │   ├── feature/
│   │   ├── domain/
│   │   └── branding/
│   │
│   ├── application/
│   │   ├── commands/
│   │   └── queries/
│   │
│   ├── repository/
│   ├── transport/
│   │   └── grpc/
│   ├── messaging/
│   └── infrastructure/
│
├── migrations/
├── proto/
└── config/
```

---

# 37. Domain Layer

Domain layer содержит правила, а не SQL.

Пример методов:

```text
Tenant.Activate()
Tenant.Suspend()
Tenant.Resume()
Tenant.Archive()
```

Именно domain entity должна запрещать:

```text
ARCHIVED -> SUSPENDED
```

а не PostgreSQL handler или gRPC transport.

---

# 38. Application Layer

Application layer координирует use case.

Пример `ChangeTenantPlanHandler`:

```text
load tenant
↓
load current plan
↓
validate new plan
↓
begin transaction
↓
close current assignment
↓
create new assignment
↓
create outbox event
↓
commit
```

---

# 39. Repository Layer

Пример интерфейсов:

```text
TenantRepository
PlanRepository
LimitRepository
FeatureRepository
DomainRepository
BrandingRepository
ContactRepository
OutboxRepository
```

Raw SQL не должен находиться в transport layer.

---

# 40. Transport Layer

Для межсервисного API — gRPC.

Transport отвечает за:

- parse request;
- primitive validation;
- auth/context extraction;
- вызов application handler;
- mapping domain error -> gRPC status;
- mapping domain response -> protobuf.

Transport не должен реализовывать бизнес-правила.

---

# 41. Карта записи данных

| Операция | Основная таблица | Дополнительно | Event |
|---|---|---|---|
| Create tenant | `tenants` | `tenant_settings`, `tenant_branding`, `tenant_plan_assignments` | `tenant.created` |
| Update tenant | `tenants` | — | `tenant.updated` |
| Activate | `tenants` | — | `tenant.activated` |
| Suspend | `tenants` | — | `tenant.suspended` |
| Resume | `tenants` | — | `tenant.resumed` |
| Archive | `tenants` | — | `tenant.archived` |
| Update settings | `tenant_settings` | `tenants.updated_at` | `tenant.settings.changed` |
| Change plan | `tenant_plan_assignments` | — | `tenant.plan.changed` |
| Set limit override | `tenant_limit_overrides` | — | `tenant.limit.changed` |
| Set feature override | `tenant_feature_overrides` | — | `tenant.feature.changed` |
| Add domain | `tenant_domains` | — | `tenant.domain.created` |
| Verify domain | `tenant_domains` | — | `tenant.domain.verified` |
| Primary domain | `tenant_domains` | — | `tenant.domain.primary_changed` |
| Update branding | `tenant_branding` | — | `tenant.branding.changed` |
| Contact change | `tenant_contacts` | — | `tenant.contact.changed` |

---

# 42. Карта чтения данных

| Query | Таблицы |
|---|---|
| GetTenant | `tenants`, `tenant_statuses` |
| GetTenantDetails | `tenants`, `tenant_settings`, `tenant_branding`, `tenant_domains` |
| GetCurrentPlan | `tenant_plan_assignments`, `subscription_plans` |
| GetPlanHistory | `tenant_plan_assignments`, `subscription_plans` |
| GetEffectiveLimit | `tenant_limit_overrides`, `tenant_plan_assignments`, `plan_limits`, `limit_types` |
| GetEffectiveFeatures | `tenant_feature_overrides`, `tenant_plan_assignments`, `plan_features`, `features` |
| GetDomains | `tenant_domains`, `domain_statuses` |
| GetBranding | `tenant_branding`, `branding_statuses` |
| GetContacts | `tenant_contacts` |

---

# 43. MVP

## Lifecycle
- CreateTenant;
- GetTenant;
- ListTenants;
- UpdateTenant;
- ActivateTenant;
- SuspendTenant;
- ResumeTenant;
- ArchiveTenant.

## Settings
- GetTenantSettings;
- UpdateTenantSettings.

## Plans
- GetCurrentPlan;
- ChangeTenantPlan;
- GetPlanHistory.

## Limits
- GetEffectiveLimit;
- SetTenantLimitOverride;
- RemoveTenantLimitOverride.

## Features
- HasFeature;
- GetEffectiveFeatures;
- SetTenantFeatureOverride;
- RemoveTenantFeatureOverride.

## Domains
- AddTenantDomain;
- GetTenantDomains;
- VerifyTenantDomain;
- SetPrimaryTenantDomain;
- DisableTenantDomain.

## Branding
- GetTenantBranding;
- UpdateTenantBranding.

## Infrastructure
- PostgreSQL;
- migrations;
- transactional outbox;
- broker publisher;
- gRPC;
- structured logging;
- tracing;
- metrics.

---

# 44. Итог

Tenant Service отвечает на пять фундаментальных вопросов ReserveHub:

```text
1. Кто это?
   → Tenant

2. Может ли он сейчас работать?
   → Tenant Status

3. Какой у него тариф?
   → Current Plan

4. Что ему разрешено?
   → Features + Limits

5. Как он представлен наружу?
   → Domains + Branding + Settings
```

`tenant-service` должен оставаться сервисом lifecycle/configuration и не забирать бизнес-логику бронирований, мероприятий, платежей и других доменов.
