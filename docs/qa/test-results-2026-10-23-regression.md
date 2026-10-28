# QA: полный текущий регресс auth-service и tenant-service — 2026-10-23

## Область и среда

Проверены существующие тесты обоих Go-модулей, миграции с нуля, реальные HTTP/gRPC ответы и бизнес-правила из [auth STPO](../services/auth-service/ReserveHub-Auth-Service-STPO.md) и [tenant specification](../services/tenant-service/ReserveHub-Tenant-Service-Full-Specification.md). Новые Go-тесты для этого прогона не создавались. Подробные сценарии: [auth](./auth-service-test-cases.md), [tenant](./tenant-service-test-cases.md).

Код проверялся в рабочем дереве ветки `dev` **включая незакоммиченные изменения backend-security**: JWT `scope`, запрет записи для `SUSPENDED` в application и удаление будущих тарифных назначений в `ChangePlan`. Это не регресс последнего коммита `dev`. Использовались временный PostgreSQL 17 на `127.0.0.1:55432`, две отдельные БД с исходными SQL-миграциями и запущенные локально auth/tenant на тестовых портах. Существующий tenant-интеграционный тест запускался в изолированной tenant-БД. NATS publisher не запускался; записи outbox проверены существующим тестом, доставка событий не проверялась. Секреты и JWT в отчёт не помещались. [Предыдущий отчёт](./test-results-2026-10-23.md) отражает состояние **до** последних исправлений.

## Автоматический прогон и актуальность тестов

| Проверка | Итог | Ограничение |
| --- | --- | --- |
| `go test -count=1 ./...` в auth-service | PASS | Unit-тесты bcrypt, bootstrap, JWT, HTTP logging; SQL-репозиторий сам по себе ими не покрыт. |
| `go vet ./...` в auth-service | PASS | Статический анализ Go. |
| `go test -count=1 ./...` в tenant-service | PASS, один штатный SKIP | `TestTenantLifecycleIntegration` пропускается без `TEST_DATABASE_URL`. |
| `go vet ./...` в tenant-service | PASS | Статический анализ Go. |
| Существующий `TestTenantLifecycleIntegration` с `TEST_DATABASE_URL`, `-count=1` | PASS | Проходит напрямую через repository, минуя application guard для `SUSPENDED`; подтверждает текущий небезопасный способ domain verification через возвращённый токен. |
| SQL-миграции обоих сервисов на пустых БД | PASS | Применены все имеющиеся `*.up.sql` по порядку. Down-миграции не прогонялись. |

Тесты JWT обновлены под новый `scope`; добавленный в предыдущей задаче `TestUnaryServerInterceptorRejectsTenantScopedAdmin` запущен вместе с остальными. Тесты остаются актуальны для позитивного lifecycle, но **не доказывают** защиту домена, сохранение истории тарифа и запрет записи во всех слоях. В частности, интеграционный тест передаёт в `VerifyDomain` токен, полученный из `AddDomain`, и ожидает `VERIFIED` — это подтверждает дефект TEN-09, а не требование владения доменом.

## Результаты по auth-service

| Кейс | Статус | Доказательство / предел проверки |
| --- | --- | --- |
| AUTH-01 | PASS | На чистой БД созданы роли и platform admin; HTTP login и `/me` вернули 200 и `admin`. |
| AUTH-02 | ЧАСТИЧНО | Повторный bootstrap без credentials прошёл. Отдельно не выполнялась смена пароля с повторным входом; существующий unit-тест проверяет отсутствие перезаписи hash. |
| AUTH-03 | PASS | На пустой БД bootstrap без credentials завершился ошибкой, admin не создан. |
| AUTH-04 | PASS в unit | `TestBootstrapCannotElevateExistingUser`; PostgreSQL-конфликт email отдельно не моделировался. |
| AUTH-05 | PASS | HTTP register 201 с email в смешанном регистре, login 200 с нормализованным email, `/me` 200; bcrypt проверен существующим unit-тестом. |
| AUTH-06 | PASS | Повторная регистрация 409, некорректный email 400. |
| AUTH-07 | PASS с ограничением | Неверный пароль и отсутствующий email — 401; `/me` без bearer — 401; повреждённый JWT на tenant RPC — `Unauthenticated`; истечение срока проверено существующим unit-тестом. |
| AUTH-08 | PASS в unit | Текущие тесты logging/request ID прошли; сторонние логи инфраструктуры не исследовались. |
| AUTH-09 | PASS | Без токена tenant RPC — `Unauthenticated`; user — `PermissionDenied`; platform admin — `NotFound` для неизвестного tenant, то есть допущен до бизнес-слоя. |
| AUTH-10 | PASS | SQL-фикстурой user назначена роль `scope=tenant, code=admin` во временной БД. Login 200; запрос чужого tenant вернул `PermissionDenied`. Прежний воспроизведённый обход устранён в этом сценарии. |
| AUTH-11 | НЕ РЕАЛИЗОВАНО | Owner, tenant-role и membership provisioning — следующий этап требований. |
| AUTH-12 | НЕ РЕАЛИЗОВАНО | Досрочный отзыв токена/актуализация роли не реализованы; действующий JWT живёт до истечения TTL. |
| AUTH-13 | НЕ РЕАЛИЗОВАНО | Refresh/logout/reset/rate limiting отсутствуют в текущем этапе. |

## Результаты по tenant-service

| Кейс | Статус | Доказательство / предел проверки |
| --- | --- | --- |
| TEN-01 | PASS | gRPC health `SERVING`; анонимный tenant RPC `Unauthenticated`, user `PermissionDenied`. |
| TEN-02 | PASS с ограничением | Platform admin допущен; повреждённый JWT `Unauthenticated`. Истёкший JWT проверен unit-логикой auth; отдельного live gRPC запроса с истёкшим токеном не было. |
| TEN-03 | PASS с ограничением | Создание вернуло `PENDING`; `GetTenant` и `ListTenants` видят созданный tenant; дубликат slug — `AlreadyExists`. Атомарность rollback проверена только существующим тестом/outbox косвенно. |
| TEN-04 | ЧАСТИЧНО | Tenant с валидными настройками и планом активирован (`ACTIVE`). Негативные сценарии отсутствующих настроек/плана отдельно не воспроизводились. |
| TEN-05 | ЧАСТИЧНО | На живом gRPC пройдены `ACTIVE → SUSPENDED → ACTIVE`; архивирование и часть недопустимых переходов покрыты существующими тестами repository/domain. |
| TEN-06 | PASS на gRPC; ОСТАТОЧНЫЙ РИСК | При `SUSPENDED` семь операций (`UpdateTenant`, settings, plan, limit, feature, domain, branding) вернули `FailedPrecondition`. Guard стоит до транзакции в application; прямые вызовы repository и гонка с одновременным suspend отдельно не проверялись. |
| TEN-07 | PASS в существующем интеграционном тесте | Лимит из плана, установка/снятие override, фича и домен проверены. Параллельные изменения и все тарифы не покрыты. |
| TEN-08 | **FAIL: потеря истории тарифов** | BASIC назначен на T2, затем ENTERPRISE на T1 < T2: `GetPlanHistory` содержит ENTERPRISE/BUSINESS, BASIC исчез. Дополнительно ретроактивные смены на уже прошедшие T+10, T+20, затем T+15 секунд удалили состоявшееся назначение ENTERPRISE из истории. Это нарушает BR-006. Политика отмены ещё будущего BASIC требует решения продукта; удаление уже прошедшей записи не зависит от этого решения. Конкурентность не проверялась. |
| TEN-09 | **FAIL: домен подтверждается без владения** | `AddTenantDomain` вернул токен; `VerifyTenantDomain` с этим токеном перевёл домен в `VERIFIED` без DNS/HTTP proof. |
| TEN-10 | ЧАСТИЧНО | Существующий интеграционный тест проходит verified/primary/disable, но verification использует дефектный путь из TEN-09. Сценарий с настоящим proof ещё невозможен. |
| TEN-11, TEN-12 | НЕ РЕАЛИЗОВАНО | Membership, tenant isolation для user и owner provisioning отсутствуют по плану следующего этапа. |
| TEN-13 | ЧАСТИЧНО | Существующий интеграционный тест проверяет наличие outbox после успешных операций; публикация в NATS, rollback и идемпотентная повторная доставка не проверялись. |
| TEN-14 | **FAIL для закрытого окружения** | Анонимный gRPC reflection вернул описание `TenantService`; reflection включён безусловно и не проходит через unary JWT interceptor. |

## Дефекты и приоритет

1. **P1, TEN-08, BR-006:** `ChangePlan` физически удаляет назначения начиная с `effective_from`; ретроактивный запрос удаляет уже состоявшуюся историю. `GetPlanHistory` больше её не возвращает. Нет события отмены или ограничителя на непересекающиеся интервалы. Исправление должно сохранить аудит; правило для будущих назначений следует согласовать отдельно.
2. **P1, TEN-09:** verification token доступен через API и принимается тем же API как достаточное доказательство. Нужна независимая проверка владения DNS TXT или HTTP challenge до `VERIFIED`.
3. **P1, TEN-06, остаточный риск:** application guard покрывает обычный gRPC путь, но repository пишет без проверки `SUSPENDED`; проверка статуса до транзакции допускает гонку с приостановкой. Динамический конкурентный обход не воспроизводился.
4. **P2, TEN-14:** анонимный reflection раскрывает схему gRPC и вне локальной среды, потому что регистрация безусловна.

Отдельно остаются известные security-находки из [backend security review](../security/backend-security-report.md): общий HS256 secret, отсутствие отзыва JWT и rate limiting, вопросы TLS/портов/прав БД. Этот прогон проверял бизнес-поведение, а не внешний периметр. Исправление AUTH-10 подтверждено live запросом, но не решает отзыв ранее выданного platform admin токена.

## Вывод и границы

Регресс текущего этапа A auth и основных tenant happy paths прошёл. Бизнес-процесс домена и сохранение истории тарифа **не соответствуют** требованиям, поэтому весь tenant модуль нельзя пометить как PASS. Полный сценарий owner/membership остаётся запланированным этапом B. Использовались только локальные временные БД и порты; production, внешние сети, TLS, нагрузка, гонки, настоящий DNS proof и доставка NATS не проверялись.
