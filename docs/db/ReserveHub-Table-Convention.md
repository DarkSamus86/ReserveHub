# Таблицы Tenant, Auth и Admin: единая конвенция

**Срез:** рабочее дерево ветки `feat/auth-tenant-provisioning`, 2026-10-28.
**Согласовано:** `analyst` и `database-security`.
**Основа:** [конвенция Tenant DB](tenant_db/ReserveHub-Tenant-Service-Database-Agent-Spec.md), [Auth owner provisioning](../../services/auth-service/migrations/000003_owner_provisioning.up.sql), [Auth rename](../../services/auth-service/migrations/000004_standardize_table_names.up.sql), [Auth sessions/outbox](../../services/auth-service/migrations/000005_create_auth_session_outbox.up.sql), [Admin rename](../../services/admin-service/migrations/000002_standardize_table_names.up.sql), [Admin outbox](../../services/admin-service/migrations/000003_create_admin_outbox.up.sql), [Tenant idempotency](../../services/tenant-service/migrations/000021_create_tenant_idempotency.up.sql).

Документ перечисляет **фактические таблицы после применения всех миграций этой ветки** и имена до переименования. Это инвентаризация текущего provisioning-среза, **не полный состав Auth и Admin для MVP**. Требуемые следующие таблицы и решения перечислены ниже. Более ранние [планы Auth](auth/README.md), [планы Tenant](tenant/README.md) и [описание интеграции](../auth-tenant-integration.md) местами описывают состояние до появления миграций owner provisioning и idempotency в этой ветке.

## Границы владения

Один PostgreSQL содержит независимые схемы `tenant_service`, `auth_service` и `admin_service`. Каждый сервис пишет только в свою схему. `tenant_id` в Auth и Admin — идентификатор из контракта Tenant, а не основание для межсхемного FK, JOIN или общей SQL-транзакции. `sys_schema_migration` создаёт `migrate` отдельно в каждой схеме; это служебная таблица, её имя уже соответствует конвенции.

## Правило имён

Во всех схемах: `snake_case`, без повторения имени сервиса в названии таблицы. Имена сущностей по возможности даны в единственном числе, но это не жёсткое правило существующей конвенции (`cfg_tenant_settings` уже во множественном). Префикс задаётся **ролью таблицы**, а не тем, сколько в ней строк или насколько часто они меняются.

| Префикс | Назначение | Пример |
| --- | --- | --- |
| `di_` | Справочник значений, которыми управляет сервис | `di_tenant_status` |
| `md_` | Основная бизнес-сущность или каталог | `md_tenant`, `md_user` |
| `tr_` | Запись бизнес-операции или процесса с жизненным циклом | `tr_tenant_invitation` |
| `ln_` | Связь сущностей, в том числе со статусом и профилем прав | `ln_user_tenant_membership` |
| `hs_` | История состояний или назначений, сохраняемая для чтения прошлого | `hs_tenant_plan_assignment` |
| `cfg_` | Настройка или override, действующий для сущности | `cfg_tenant_settings` |
| `sys_` | Инфраструктура: дедупликация, outbox, миграции | `sys_create_tenant_request` |
| `aud_` | Неизменяемый аудит действий и результата | пока нет |
| `agg_` | Производный агрегат/read model | пока нет |

Новые таблицы не получают имя без префикса. Не использовать `de_` и `dict_`. Статусы `PENDING`, `ACTIVE` и `REVOKED` сами по себе не требуют отдельного `di_`: справочник нужен, когда значениями действительно управляют как данными.

## Tenant: фактическая схема, переименование не требуется

| Таблица `tenant_service` | Категория и назначение |
| --- | --- |
| `di_tenant_status` | Справочник состояний компании. |
| `di_domain_status` | Справочник состояний домена. |
| `di_branding_status` | Справочник состояний branding. |
| `di_limit_type` | Справочник типов лимитов. |
| `di_feature` | Справочник feature flags. |
| `md_tenant` | Компания и её жизненный цикл. |
| `md_tenant_domain` | Домены компании. |
| `md_subscription_plan` | Каталог тарифов. |
| `md_tenant_contact` | Контакты компании. |
| `cfg_tenant_settings` | Настройки компании. |
| `cfg_tenant_branding` | Настройки branding. |
| `cfg_tenant_limit_override` | Переопределения лимитов. |
| `cfg_tenant_feature_override` | Переопределения feature flags. |
| `ln_plan_limit` | Связь тарифа и типов лимитов с величиной. |
| `ln_plan_feature` | Связь тарифа и features. |
| `hs_tenant_plan_assignment` | История назначений тарифов. |
| `sys_outbox_event` | Надёжная публикация событий Tenant. |
| `sys_create_tenant_request` | Дедупликация `CreateTenant` по `operation_id` и сохранённый результат. |

**Итого: 18 предметных/инфраструктурных таблиц**, плюс `tenant_service.sys_schema_migration`. `tr_`, `aud_` и `agg_` в Tenant сейчас не нужны только ради заполнения категорий. `md_subscription_plan` остаётся `md_`: это тарифный каталог, а не индивидуальная настройка компании. `hs_tenant_plan_assignment` остаётся `hs_`, потому что хранит историю назначения.

## Auth: имена до и после миграции `000004`

| До `000004` | После `000004` в `auth_service` | Почему | Нужна сейчас |
| --- | --- | --- | --- |
| `users` | `md_user` | Глобальная identity и состояние приглашённого пользователя. | Да |
| `roles` | `md_role` | Platform и динамические tenant-scoped роли; tenant-роль не является статическим справочником. | Да |
| `tenant_memberships` | `ln_user_tenant_membership` | Связь пользователя с `tenant_id`, профиль прав и статус. | Да |
| `invitations` | `tr_tenant_invitation` | Одноразовый процесс приглашения с собственным состоянием и сроком. | Да |
| `provision_requests` | `sys_owner_provision_request` | Техническая дедупликация защищённого `ProvisionOwner` по операции и tenant. | Да |

Миграция `000005` добавляет ещё две таблицы:

| Таблица `auth_service` | Назначение | Состояние кода |
| --- | --- | --- |
| `tr_auth_session` | Сессия с хэшем текущего refresh-секрета, сроком, ротацией и отзывом. | Таблица готова; refresh/logout ещё не подключены. |
| `sys_outbox_event` | Атомарная с Auth-записью постановка безопасного события для доставки приглашений и аудита. | Таблица готова; producer/publisher и формат защищённой доставки ещё не подключены. |

**После `000005`: 7 таблиц Auth**, плюс `auth_service.sys_schema_migration`. Это ещё не завершённый Auth MVP: DDL не выдаёт refresh token и не доставляет приглашение. `md_user.role_id` продолжает ссылаться только на platform-роль; tenant-права живут в membership. `md_role.code` фиксирует исходный slug и связан с неизменным `tenant_id`. `tr_tenant_invitation` сейчас остаётся `PENDING_DELIVERY` без токена: доставка и принятие ещё не реализованы. Для этапа C проверка членства использует уже существующую `ln_user_tenant_membership`; отдельная таблица для самого tenant в Auth не нужна.

## Admin: имена до и после миграции `000002`

| До `000002` | После `000002` в `admin_service` | Почему | Нужна сейчас |
| --- | --- | --- | --- |
| `provision_operations` | `tr_tenant_provision_operation` | Административная бизнес-операция: actor, email владельца в защищённом контексте, ключ, `tenant_id`, статус и безопасная ошибка. | Да |

Миграция `000003` добавляет `admin_service.sys_outbox_event` для безопасных административных событий; producer/publisher ещё не подключены. **После `000003`: 2 таблицы Admin**, плюс `admin_service.sys_schema_migration`. `tr_tenant_provision_operation` хранит состояние оркестрации, но не копирует пользователя, членство или компанию как собственные сущности. `sys_owner_provision_request` в Auth и `sys_create_tenant_request` в Tenant решают другую задачу — локальную дедупликацию межсервисных шагов. Строка Admin обновляется при повторе и **не является неизменяемым аудитом**.

В Auth и Admin сейчас нет `cfg_`-таблиц: адреса сервисов, ключи и сервисные секреты относятся к конфигурации запуска, а не к данным PostgreSQL. Добавлять таблицу настроек только ради одинакового набора префиксов не нужно.

## Сколько таблиц требуется дальше

**Пять исходных таблиц Auth и одна Admin достаточны только для подготовки владельца.** Дополнительные DDL-таблицы этой ветки подготавливают следующий срез, но не завершают этап B из [Auth STPO §11](../services/auth-service/ReserveHub-Auth-Service-STPO.md) и полный [MVP §5.2, §5.15 и §8](../architecture/ReserveHub-MVP.md). Следующая матрица отделяет наличие таблицы от работающего сценария.

| Этап | Схема и таблица | Зачем и минимальные данные | Статус решения |
| --- | --- | --- | --- |
| Завершение B | существующая `auth_service.tr_tenant_invitation` | Хэш одноразового токена, срок, `accepted_at` и статус уже предусмотрены. Реализовать выдачу, доставку и атомарное принятие с активацией membership; отдельная таблица принятия не нужна. | Таблица есть, сценарий не завершён. |
| Завершение B | `auth_service.sys_outbox_event` | Если приглашение отправляется асинхронно, сохранить команду доставки в той же Auth-транзакции, что токен и состояние invite. Повторять доставку по `event_id`. | DDL добавлен в `000005`; транспорт и формат защищённой команды ещё утвердить. |
| Полный MVP | `auth_service.tr_auth_session` | Учётная сессия с refresh rotation и logout. Нужна для прямо требуемых MVP refresh/logout. | DDL добавлен в `000005`; flow ещё не реализован. |
| Полный MVP | `auth_service.sys_outbox_event`, `admin_service.sys_outbox_event` | Надёжно передавать события выдачи/отзыва прав и административных действий в центральный аудит; Admin пишет событие вместе с обновлением операции. | DDL добавлен в Auth `000005` и Admin `000003`; producer/publisher ещё не реализованы. Если выбрать синхронный аудит, нужна явная политика отказа при недоступности audit-service. |
| Полный MVP | `audit_service.aud_event` | Неизменяемая история административных и чувствительных действий. | Владелец — отдельный audit-service по MVP и [Full-Spec §7.20](../architecture/ReserveHub-FULL-SPEC.md), не Auth/Admin. |

Для `tr_auth_session` минимальны `id`, `user_id` с FK на `md_user`, **хэш** текущего refresh-секрета (`BYTEA`, 32 байта, уникальный), `created_at`, `expires_at`, `rotated_at`, `revoked_at`, счётчик/версия ротации. `expires_at` позже `created_at`. Ротация должна условно заменить текущий хэш только для неотозванной и неистёкшей сессии; logout проставляет `revoked_at`. Refresh также проверяет статус `md_user`. История каждого отдельного токена не обязательна, если refresh token содержит проверяемый идентификатор сессии: тогда повторное предъявление старого секрета можно отнести к конкретной сессии и отозвать её.

Для outbox в каждой схеме нужны `id`/`event_id`, тип и версия события, `aggregate_id`, безопасный payload, `created_at`, `published_at`, число попыток и время следующей попытки, индекс неопубликованных записей. Потребитель дедуплицирует `event_id`. **Секрет приглашения не помещать в обычный outbox, широковещательное событие или лог**: для повторяемой доставки нужен шифрованный payload с ключом вне БД либо иной защищённый notification-контракт. Данные owner email и токенов исключить из административного audit payload. После подтверждённой доставки нужна политика retention.

После новых миграций при асинхронной доставке получается **7 таблиц Auth и 2 Admin** плюс служебные таблицы мигратора; неизменяемый аудит хранится отдельно. Новые таблицы пока не используются приложением — в MVP приёмку они войдут вместе с refresh/logout, приглашениями и публикацией аудита.

## Требования без новых таблиц и дальнейшие возможности

| Сценарий | Что требуется |
| --- | --- |
| Блокировка пользователя | [AUTH-FR-005](../services/auth-service/ReserveHub-Auth-Service-STPO.md): расширить оба ограничения `md_user` — `ck_users_status` и `ck_users_password_state` — для `SUSPENDED`, определить переходы, запрет выдачи/обновления токенов и действие уже выданного access JWT в пределах его 15 минут. Новая таблица не нужна. |
| Tenant-scoped RBAC этапа C | Для фиксированных `tenant_owner`/`tenant_viewer` использовать существующие `ln_user_tenant_membership` и `tr_tenant_invitation` для приглашения, выдачи и отзыва доступа сотрудника; отдельная staff-invitation таблица не нужна. Для `tenant_manager` расширить `CHECK permission_profile`; `di_permission` и `ln_profile_permission` нужны лишь если утверждена управляемая в БД матрица. |
| Reset и подтверждение email | [Auth STPO §11, этап D](../services/auth-service/ReserveHub-Auth-Service-STPO.md) и [Full-Spec §7.2](../architecture/ReserveHub-FULL-SPEC.md) предусматривают эти сценарии, но явный минимум MVP их не перечисляет. При реализации добавить `tr_password_reset_request` и `tr_email_verification_request` либо согласованную общую challenge-модель: `user_id`, уникальный хэш секрета, срок, `used_at`/`revoked_at`; для email — проверяемый адрес и `md_user.email_verified_at`. |
| Расширенная история refresh | Отдельная `tr_refresh_token` нужна только при утверждённой истории всех ротаций, расследовании reuse или нескольких параллельных токенах в сессии. MVP refresh/logout можно реализовать через `tr_auth_session`. |
| Несколько credential types и DB-managed service identities | `md_credential`/`md_service_principal` добавлять только при таких контрактах. Сейчас пароль принадлежит `md_user`, а сервисный секрет и ключи — конфигурации запуска. |
| Данные платформенного Admin | Tenant/provider/метрики принадлежат Tenant, Provider и Analytics. Новые `md_`-таблицы Admin не следуют из одного экрана платформенного администратора. |

[Auth STPO §7](../services/auth-service/ReserveHub-Auth-Service-STPO.md) называет Auth источником истины для аудита доступа, тогда как MVP и Full-Spec выделяют `audit-service` для неизменяемой истории. Рабочая граница: Auth/Admin владеют фактами действий и публикуют их, audit-service хранит общую историю. Перед реализацией зафиксировать контракт доставки и окончательное владение историей.

Таблицы заказов, билетов, платежей и других доменов не размещать в этих трёх схемах по принципу удобства JOIN; ими должны владеть соответствующие сервисы.

## Переход и развёртывание

1. В этой ветке добавлены versioned migrations: `auth_service` `000004` и `admin_service` `000002` с `ALTER TABLE ... RENAME TO ...`, затем Auth `000005` и Admin `000003` для session/outbox. Ранее существовавшие миграции не переписаны; Tenant менять не требуется. SQL-ссылки в Auth и Admin repository и интеграционном тесте Auth обновлены вместе с rename-миграциями.
2. PostgreSQL сохраняет данные, OID таблиц, FK и grants при `RENAME`, но имена существующих FK, индексов и constraints автоматически не меняются. Их переименование не требуется для работы приложения и может быть отдельным согласованным шагом.
3. Перед развёртыванием проверить переход на копии БД с данными и последовательность `up`/`down`. `ALTER TABLE ... RENAME` берёт `ACCESS EXCLUSIVE`; после него старые экземпляры приложения обращаются к прежним именам. Нужен координированный rollout с остановкой старого кода либо отдельно проверенная совместимость. Простые views не считать достаточной совместимостью для `ON CONFLICT` и всех DML-запросов. Для рабочей БД оценить долгие транзакции, `lock_timeout` и восстановимый backup.
4. После развёртывания проверить фактический список таблиц во всех трёх схемах, владельцев и права runtime/migration ролей, отсутствие межсхемных FK/JOIN, повтор `CreateTenant`/`ProvisionOwner` и чтение статуса Admin. `sys_schema_migration` не переименовывать без одновременного изменения настройки migrator в Compose.
5. Для Auth `000005` и Admin `000003` откат с данными намеренно запрещён: нельзя молча удалить действующие сессии или события в очереди. Согласовать retention и безопасное удаление данных до попытки `down`.
