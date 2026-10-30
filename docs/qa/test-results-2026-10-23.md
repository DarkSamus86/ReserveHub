# QA: auth-service и tenant-service — 2026-10-23

## Область и метод

Сверены реализованные бизнес-процессы с [auth STPO](../services/auth-service/ReserveHub-Auth-Service-STPO.md) и [tenant specification](../services/tenant-service/ReserveHub-Tenant-Service-Full-Specification.md). Перечень сценариев: [auth](./auth-service-test-cases.md), [tenant](./tenant-service-test-cases.md). Проверены код, Go-тесты без кеша, локальный Compose и HTTP/gRPC-запросы. Изменяющие тесты выполнялись в **двух отдельных временных БД** (auth и tenant), миграции применялись заново; обе БД удалены. Тестовые секреты и JWT не сохранялись в отчёте. Production, внешние сети, TLS, конкурентная нагрузка и реальные DNS-проверки не тестировались.

## Фактически выполнено

| Кейс | Метод | Результат |
| --- | --- | --- |
| AUTH-01, AUTH-02 | Временная auth-БД: миграции, первый и повторный `bootstrap-admin` (второй без credentials), HTTP login и `/me`. | **PASS**: login и `/me` 200, роль admin; повторный запуск без credentials успешен. Не проверялась отдельно смена пароля с последующим входом, она покрыта unit `TestBootstrapDoesNotResetExistingAdmin`. |
| AUTH-03, AUTH-04 | `go test -count=1 -v ./...` в auth-service. | **PASS (unit)**: пустые credentials при отсутствии admin отвергаются; существующий user не повышается до admin. Отдельный PostgreSQL конфликт email не воспроизводился. |
| AUTH-05, AUTH-06 | Временная auth-БД: регистрация email в другом регистре, login, `/me`, повторная регистрация; unit bcrypt/валидации. | **PASS**: 201, затем login 200, `/me` 200 роль user; повторный email 409. Bcrypt и короткий пароль проверены unit-тестами, сохранённый hash в этой БД вручную не читался. |
| AUTH-07 | Локальный HTTP: login с несуществующим email, `/me` без токена; unit `TestTokenRoundTripAndExpiry` и `TestTokenRejectsTamperingAndShortSecret`. | **PASS**: оба HTTP 401, неверный/истёкший JWT отклонён unit-тестами. |
| AUTH-08 | Unit-тесты HTTP logging и request ID. | **PASS (unit)**: schema и замена некорректного request ID, секреты не записаны тестовым handler. Не проводился аудит сторонней инфраструктуры логов. |
| AUTH-09, TEN-01, TEN-02 | `grpcurl` к работающему tenant-service: без токена, с user и platform admin токенами из временного auth-service; запрос несуществующего tenant. | **PASS**: без токена `Unauthenticated`, user `PermissionDenied`, admin доходит до бизнес-слоя и получает `NotFound`. gRPC health `SERVING`. |
| AUTH-10 | Временная auth-БД: SQL-фикстурой user назначена tenant-scoped роль `admin`; после login выполнен `GetTenant` для UUID вне этой роли. | **FAIL (изоляция ролей)**: получен `NotFound` бизнес-слоя, а не `PermissionDenied`, значит токен был принят как платформенный admin. Публичное API не умеет создавать/назначать такую роль; риск возникает при будущих membership/provisioning или доверенной записи в БД. |
| TEN-03–TEN-05, TEN-07, TEN-10, TEN-13 | Отдельная tenant-БД с применёнными миграциями: `TEST_DATABASE_URL=… go test -run ^TestTenantLifecycleIntegration$ -count=1 -v ./tests`. | **PASS в объёме существующего теста**: create/activate, plan entitlement, limit override, feature, add/verify/primary domain, branding, suspend/resume/archive и наличие outbox. Тест не покрывает уникальный slug, неверные переходы после архивации, rollback и параллельность целиком. |
| TEN-01 | Живой Compose, HTTP `/health/live` и `/health/ready`, gRPC health. | **PASS**: HTTP 200/200; gRPC `SERVING`. |
| Прочие unit | `go test -count=1 -v ./...` и `go vet ./...` отдельно в обоих сервисах. | **PASS**, кроме штатного SKIP интеграционного tenant-теста в общем прогоне без `TEST_DATABASE_URL`; затем он запущен отдельно на временной БД и прошёл. |

## Выявленные проблемы / расхождения

1. **P0 до внедрения tenant-ролей — AUTH-10: смешение scopes `admin` (воспроизведено).** Схема разрешает `scope=tenant, code=admin` (`services/auth-service/migrations/000002_create_roles.up.sql`), auth при login выбирает только `roles.code` (`services/auth-service/internal/repository/postgres.go`), а JWT и tenant interceptor трактуют `admin` как глобальный (`services/auth-service/internal/auth/service.go`, `services/tenant-service/internal/authn/interceptor.go`). Требуется отличать платформенную роль от tenant-роли в claims и проверках, не принимать совпадение code за полномочие.
2. **P1 — TEN-09: нет доказательства владения доменом (анализ кода + подтверждающий существующий интеграционный путь).** `AddDomain` возвращает verification token; `VerifyDomain` сверяет предоставленный клиентом token с сохранённым, без DNS/HTTP проверки (`services/tenant-service/internal/repository/presentation.go`). Интеграционный тест переводит домен в `VERIFIED`, просто передав возвращённый token. Это не проверка собственности на домен; TEN-09 как отдельный отрицательный тест пока не запускался.
3. **P1 — TEN-08: возможны перекрывающиеся будущие тарифы (анализ кода, не воспроизводилось динамически).** `ChangePlan` закрывает только интервал, действующий в указанную дату, и добавляет новый без конечной даты (`services/tenant-service/internal/repository/plan.go`). При назначении B с T2, затем C с T1 < T2 оба будут действовать после T2. Нужен интеграционный тест с планами B/C и правило непересечения интервалов.
4. **P1 — TEN-06: запись в suspended tenant (анализ кода, не запускалось отдельно).** BR-004 требует блокировать tenant-specific writes, но `ensureNotArchived` запрещает только `ARCHIVED` (`services/tenant-service/internal/application/tenant.go`); смена плана и другие операции аналогично пропускают suspended. Если platform admin должен сохранять право обслуживания, исключение необходимо описать и проверить отдельно.
5. **Этап B (не реализован, не регрессия этапа A) — AUTH-11/TEN-12.** Создание/активация tenant не создаёт владельца, membership или роль `{tenant_slug}`; auth сам отмечает это как будущий этап ([STPO §9, §11](../services/auth-service/ReserveHub-Auth-Service-STPO.md)). Доступ user к tenant пока правильно закрыт. Полный бизнес-сценарий владельца компании и кабинета поэтому не готов.

## Что осталось проверить

- AUTH-07/TEN-02: реальные HTTP/gRPC запросы с повреждённым и истёкшим токеном (unit-тесты уже есть), отзыв токена и изменение роли — AUTH-12 (пока нет механизма отзыва).
- TEN-03/TEN-08/TEN-09/TEN-06: отдельные интеграционные негативные сценарии на временной БД; конкурентная смена планов, конфликт slug, запрет записей suspended и DNS/HTTP proof.
- TEN-11/TEN-12, AUTH-11/AUTH-13: owner provisioning, межtenantная изоляция, приглашения, refresh/logout/rate limiting после реализации этапов B–D.
- Отдельно остаются риски [backend security review](../security/backend-security-report.md): TLS/открытые порты, общий DB-аккаунт, общий HS256 secret, rate limiting и gRPC reflection. Тестирование на локальном стенде не подтверждает пригодность к production.

**Вывод:** базовый этап A auth и основные happy-path операции tenant работают на локальном стенде. Полный tenant-owner workflow пока не реализован; перед внедрением tenant-ролей критично исправить смешение platform/tenant `admin`.
