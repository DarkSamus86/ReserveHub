# Примерная архитектура кода ReserveHub

Статус: предложение backend-security, frontend-security и analyst. Документ описывает организацию исходного кода приложений, сервисов и их контрактов. Сетевые связи описаны в [communication-topology.md](communication-topology.md); продуктовые функции — в [Full-Spec](ReserveHub-FULL-SPEC.md) и [MVP Scope](ReserveHub-MVP.md).

Примеры каталогов ниже не означают, что все компоненты уже реализованы. Пакет появляется вместе с реальной ответственностью, а не ради шаблона.

## 1. Текущее состояние

| Компонент | Что реализовано |
| --- | --- |
| `services/auth-service` | Отдельный Go-модуль с регистрацией, входом и `/me` по HTTP; пакет `internal/auth` объединяет сценарии, модели, порт репозитория и JWT. Это часть функций `identity-service` из спецификаций. Вопрос о будущем имени сервиса не решён. |
| `services/tenant-service` | Отдельный Go-модуль с внутренним gRPC API, слоями `domain`, `application`, `repository`, `transport/grpc`, исходным proto и сгенерированным Go-кодом. |
| `services/api-gateway` | Отдельный Go-модуль. Сейчас публикует только три маршрута Auth и health endpoints; маршруты, proxy, JWT и limiter находятся в `internal/gateway`. |
| Frontend | Приложений, JS/TS workspace и публичных OpenAPI-спецификаций пока нет. |

MVP также предусматривает provider, catalog, venue, schedule, inventory, pricing, reservation, order, payment, fulfillment, notification и audit services. Это целевой объём MVP, а не список существующих директорий. Travel, loyalty и большая часть AI/ML-возможностей относятся к более поздним этапам.

## 2. Структура репозитория

```text
ReserveHub/
├── apps/                         # будущие frontend-приложения
│   ├── customer-web/
│   ├── provider-admin/
│   └── platform-admin/
├── packages/                     # только реально общий frontend-код
│   ├── ui/
│   └── api-client/
├── services/
│   ├── api-gateway/               # существует
│   ├── auth-service/              # существует; часть identity-функций
│   ├── tenant-service/            # существует
│   └── <new-domain-service>/      # добавлять по конкретному MVP-сценарию
├── docs/
└── compose.yaml
```

Каждый Go-сервис остаётся самостоятельным модулем, образом и единицей развёртывания. `go.work` возможен для локальной разработки, но сборка Docker/CI не должна зависеть от workspace или исходников соседнего сервиса. Не импортировать `internal` другого сервиса и не создавать общий пакет со всеми доменными моделями.

## 3. Шаблон Go-сервиса

```text
services/example-service/
├── api/openapi/v1/                # если есть публичный HTTP API
├── proto/example/v1/             # если есть внутренний gRPC API
├── gen/                          # только воспроизводимый generated code
├── cmd/example-service/main.go    # composition root и lifecycle
├── internal/
│   ├── domain/                   # модели, инварианты, доменные ошибки
│   ├── application/              # сценарии и объявленные ими порты
│   ├── transport/http/           # маршруты, DTO и HTTP error mapping
│   ├── transport/grpc/           # protobuf mapping и gRPC status
│   ├── adapters/postgres/        # реализация портов хранения
│   ├── adapters/nats/            # outbox publisher/consumer при необходимости
│   ├── adapters/external/        # клиенты внешних систем при необходимости
│   ├── security/                 # проверка principal и политика доступа
│   ├── config/                   # типизированный конфиг этого сервиса
│   └── observability/            # logging, metrics, tracing
├── migrations/                   # собственная схема сервиса
├── Dockerfile
├── go.mod
└── README.md
```

Каталоги `http`, `grpc`, `nats` и `external` нужны только сервису, который реально использует соответствующий адаптер. Для небольшого сервиса допустимо меньше пакетов, если направление зависимостей сохраняется.

### Правило зависимостей

```text
cmd → transport → application → domain
  └→ adapters ────────┘
```

- `cmd` собирает сервер, конфигурацию, порты и адаптеры; бизнес-правила в нём не размещаются.
- `domain` не импортирует HTTP, protobuf, PostgreSQL, NATS и переменные окружения.
- `application` реализует use cases и объявляет только необходимые интерфейсы: repository, transaction, clock, publisher или клиент другого сервиса.
- `transport` проверяет wire-формат, переводит DTO в application input и возвращает безопасный HTTP/gRPC ответ. DTO не являются доменными сущностями.
- `adapters` реализуют application ports. SQL и protobuf-типы не проходят внутрь domain.
- `security` устанавливает проверенные user principal и service principal. Сервис-владелец данных сам проверяет tenant membership и владение ресурсом.

### Данные и миграции

Миграции находятся рядом с сервисом-владельцем данных. Auth и Tenant уже используют отдельные схемы PostgreSQL и отдельные таблицы версий миграций, хотя физически работают с одним сервером БД. Другой сервис получает данные через API или событие, а не читает таблицы владельца напрямую. Уже применённые миграции не меняются: новые инварианты добавляются следующей миграцией с планом совместимого выката.

Для критических reservation, order, payment, refund и fulfillment событий Full-Spec требует transactional outbox. Изменение бизнес-состояния и запись события фиксируются одной локальной транзакцией; отдельный адаптер публикует событие в NATS. Потребитель выдерживает повторную доставку и выполняет обработку идемпотентно.

## 4. Применение к существующим сервисам

### Auth / identity

Сегодня `internal/auth` содержит `Service`, `User`, `Session`, интерфейс хранения и JWT. PostgreSQL-адаптер импортирует этот широкий пакет, а HTTP transport сериализует `User` и `Session` напрямую. При изменении публичного JSON это связывает transport с application и доменной моделью.

Рекомендуемый путь без изменения существующего HTTP-поведения:

```text
services/auth-service/
├── api/openapi/v1/auth.yaml       # добавить как источник HTTP-контракта
├── cmd/auth-service/
├── internal/domain/               # User, Role, инварианты
├── internal/application/          # Register/Login/Me/Bootstrap и ports
├── internal/transport/httpapi/    # request/response DTO, handlers, errors
├── internal/adapters/postgres/
├── internal/security/token/       # signer/verifier adapter
├── internal/config/
└── migrations/
```

Пароли и credential material не становятся response DTO. Full-Spec описывает sessions, refresh rotation и logout, однако текущая реализация выдаёт короткоживущий bearer JWT. Browser session contract — отдельное решение до расширения frontend.

### Tenant

Tenant уже близок к рекомендуемой структуре: `domain`, `application` с repository port, `repository` и `transport/grpc`. `proto/tenant/v1/tenant.proto` — исходный внутренний контракт; `gen/tenant/v1` не редактируется вручную. Переименование `repository` в `adapters/postgres` и `messaging` в `adapters/nats` допустимо, когда это упрощает навигацию, но не является самостоятельной целью.

Текущий interceptor проверяет пользовательский JWT и platform-admin scope для большинства RPC. При появлении внутренних клиентов Tenant потребуется отдельно устанавливать identity вызывающего сервиса и проверять разрешённые RPC-методы. Пользовательский actor context сохраняется для authorization и аудита. Ни изоляция сети, ни один пользовательский JWT не подтверждают полномочия workload.

### API Gateway

Когда появятся новые реальные upstream API, разделить `internal/gateway` по обязанностям:

```text
services/api-gateway/
├── cmd/api-gateway/
├── internal/transport/http/       # маршруты, middleware, ошибки
├── internal/adapters/authproxy/   # allowlist маршрутов Auth и upstream client
├── internal/security/token/       # проверка edge JWT
├── internal/ratelimit/
├── internal/config/
└── internal/observability/
```

Gateway владеет публичным маршрутом, edge-политикой, ограничениями запросов и transport mapping. Он не копирует доменные DTO и не владеет состоянием заказа, брони или оплаты. Таблица маршрутов Gateway должна сверяться с HTTP-контрактом upstream. Выборочная композиция данных для экрана допустима, если бизнес-решение остаётся у доменного сервиса.

JWT claims и правила верификации сейчас вручную реализованы отдельно в Auth, Tenant и Gateway. Это риск расхождения `alg`, `kid`, `issuer`, `audience`, времени жизни и scope. Нужно закрепить единый контракт и совместимость проверяющих. Варианты: маленький версионируемый security-модуль с владельцем либо поддерживаемая JWT-библиотека и общие контрактные фикстуры. Большой `shared/common` ради этого не нужен.

### Будущие владельцы MVP-доменов

| Сервис | Ответственность за данные и правила |
| --- | --- |
| `provider-service` | Профиль, статус provider и связь сотрудников с provider. |
| `catalog-service` | Предложение и его публикация. |
| `venue-service` | Площадка, зал, зона, место и схема. |
| `schedule-service` | Сеанс, время, перенос и отмена. |
| `inventory-service` | Атомарная доступность, hold, release и confirm. |
| `pricing-service` | Серверная котировка и правила цены. |
| `reservation-service` | Жизненный цикл брони, TTL и координация hold. |
| `order-service` | Коммерческий заказ, итоговый snapshot и статус. |
| `payment-service` | Платёжная попытка, provider webhook, refund и сверка. |
| `fulfillment-service` | Билет/QR/voucher, выдача и проверка использования. |
| `notification-service` | Доставка уведомлений по каналам. |
| `audit-service` | История административных и чувствительных действий. |

`admin-service` для platform-admin сценариев — возможный внутренний фасад перед Tenant. Его нет в Full-Spec/MVP как самостоятельного сервиса. Выделять его следует при появлении конкретных use cases и политики доступа, не создавая публичный маршрут к Tenant.

## 5. Владение контрактами

### Публичный HTTP

Исходный OpenAPI хранится рядом с сервисом-владельцем HTTP API, например `services/auth-service/api/openapi/v1/auth.yaml`. Он описывает маршруты, request/response DTO, статусы, ошибки, заголовки и требования к аутентификации. Gateway хранит соответствие публичных маршрутов upstream и edge-политику, но не второй независимый набор доменных DTO.

Когда появится несколько публичных API, можно собирать service-owned OpenAPI в общий опубликованный bundle для документации и TypeScript codegen. Текущий формат ошибки `{code, message}` и `X-Request-ID` следует закрепить в контракте; формат ошибок полей и карта статусов пока не утверждены.

### Внутренний gRPC

Сервис-владелец публикует `.proto` как источник контракта. Generated code воспроизводится из него и используется только в transport/client adapters. Protobuf message не становится application/domain моделью. Для RPC фиксируются caller service identity, actor context, method ACL, deadline, допустимые повторы и идемпотентность. Конкретные пары RPC из topology-документа пока являются рекомендациями.

### События NATS

Producer владеет схемой, значением и версией события. Событие описывает уже зафиксированный факт, имеет идентификатор для дедупликации и достаточный контекст для подписчика без прямого чтения чужой БД. Consumers строят локальные read models и обрабатывают повторную доставку. Имена и payload событий утверждаются отдельным контрактом, а не выводятся из Go-структуры.

## 6. Пример frontend-архитектуры

Full-Spec задаёт TypeScript и React/Next.js для `customer-web`. Framework для двух admin-приложений не зафиксирован. Структуру ниже нужно применить при появлении frontend-кода и подстроить под выбранный framework.

```text
apps/
├── customer-web/
│   └── src/
│       ├── app/                  # маршруты, layout, app providers
│       ├── features/             # search, offers, reservation, checkout, orders
│       ├── entities/             # представления данных для этого UI
│       └── shared/               # локальные компоненты и утилиты
├── provider-admin/
│   └── src/
│       ├── app/
│       ├── features/             # catalog, venues, schedule, inventory, pricing, orders
│       ├── entities/
│       └── shared/
└── platform-admin/
    └── src/
        ├── app/
        ├── features/             # tenants, providers, billing, audit, health
        ├── entities/
        └── shared/
packages/
├── ui/                           # доступные визуальные примитивы
└── api-client/                   # HTTP transport и generated clients
```

У каждого приложения свои маршруты, actor-specific состояние сессии, активный provider/tenant и feature-модули. Направление импортов: `app → features → shared packages`. Приложения не импортируют feature-код друг друга. `packages/ui` не содержит ролей, правил checkout и доступа к tenant. Общий код выносится после появления реального повторного использования.

### Клиент API и экранные состояния

`packages/api-client` содержит общий HTTP transport: base URL, сериализацию, отмену запроса, разбор ошибок и доступ к `X-Request-ID`. Типы и методы для конкретных API генерируются из публичного OpenAPI. App feature переводит ответ в состояние экрана: loading, empty, forbidden, validation error, unavailable или retry. Wire DTO не поддерживаются вручную параллельно в каждом приложении. Внутренний Tenant proto не экспортируется в браузер.

Выбранный в UI provider/tenant является контекстом навигации. Заголовок или параметр, присланный браузером, не доказывает право доступа; backend проверяет membership, scope и владение ресурсом на каждой операции. Проверка роли во frontend управляет отображением кнопок, но не заменяет серверную авторизацию. Матрица прав owner/admin/manager/cashier/analyst требует отдельного продуктового контракта.

### Сессии и browser security

Full-Spec предусматривает sessions, refresh rotation и logout, которых текущий Auth пока не реализует. Способ браузерной сессии остаётся открытым. Рекомендация backend-security — identity-owned session через `Secure`, `HttpOnly`, `SameSite` cookie и CSRF-защиту для запросов, меняющих состояние. Если первый релиз оставит bearer, access token следует держать в памяти браузера и не помещать в `localStorage`, URL или аналитику.

Размещение UI и Gateway под одним origin упрощает браузерный API. Если origin различаются, понадобятся точный CORS allowlist и согласованные cookie/CSRF-настройки. Клиентская сборка содержит только публичные URL и разрешённые feature flags; секреты остаются на backend. Для SSE/WebSocket потребуется контракт авторизации подписки, проверки прав при смене scope и обработки переподключения.

## 7. Сквозные правила кода

- **Валидация:** transport проверяет формат и размер запроса, application/domain — предметный инвариант. DTO не передаётся напрямую в SQL или событие.
- **Ошибки:** application/domain возвращают типизированные ошибки; HTTP/gRPC transport сопоставляет их безопасным статусам и сообщениям. Внутренние детали не отдаются клиенту.
- **Идентичность:** user principal и service principal устанавливаются отдельно. При действиях от лица пользователя сервис проверяет обоих и сохраняет actor для аудита.
- **Наблюдаемость:** `request_id` и trace context проходят через Gateway, RPC и события. Сервисы соблюдают [logging convention](../observability/logging.md) и не пишут токены, пароли, payment secrets и тела чувствительных запросов в логи.
- **Конфигурация:** каждый сервис валидирует собственный typed config при старте. Приватный ключ получает только signer, публичный ключ — verifier-сервисы. Секреты не попадают в frontend bundle.
- **Зависимости:** общий модуль допустим для узкого стабильного контракта с владельцем и версией; доменные модели сервисов не складываются в общий `common`.

## 8. Сборка, проверки и выпуск

Go-сервисы собираются и развёртываются независимо. Проверки привязываются к границам: unit для domain/application, интеграционные для PostgreSQL/NATS adapters, HTTP/gRPC contract, авторизация/tenant isolation и миграции. Генерация protobuf/OpenAPI должна быть воспроизводимой; CI обнаруживает несовпадение generated code с исходным контрактом. Root `go.work` может служить удобству, но не становится обязательным входом для Docker build.

При старте frontend-разработки выбираются JS workspace и package manager. У каждого приложения независимые lint, typecheck, build, unit/component и browser проверки ключевых потоков. Изменения публичного API согласуются между Go-сервисом-владельцем и frontend-потребителями. Frontend workspace не должен требоваться для сборки Go-сервисов.

Compose сейчас служит локальным окружением. При другом способе развёртывания сохраняются границы сервисов, секретов и контрактов. Изменения БД выпускаются совместимо: расширение схемы, выкладка кода, затем удаление старого контракта после перехода потребителей; автоматический rollback данных не предполагается безопасным.

## 9. Порядок внедрения

1. Описать текущий Auth HTTP API и формат ошибок в OpenAPI; отдельно выбрать browser session contract.
2. Разделить Auth на domain, application, HTTP DTO и PostgreSQL adapter без изменения внешнего поведения.
3. По мере появления upstream API выделить в Gateway маршруты, auth proxy, JWT verifier и rate limiter. Сверять маршруты с контрактами владельцев.
4. Закрепить общий JWT contract и проверить совместимость Auth, Gateway и Tenant. Для новых внутренних клиентов Tenant добавить service identity и политику по gRPC-методам.
5. Создавать сервисы MVP и их миграции только вместе с конкретным сценарием и владельцем данных.
6. При начале frontend-разработки добавить три приложения, минимальные shared packages и генерацию TypeScript-клиента из OpenAPI.

## 10. Открытые решения

- Browser session: cookie с CSRF или bearer flow, refresh/logout, origin и CORS.
- Технология service identity для gRPC и формат передачи actor context.
- Нужен ли отдельный `admin-service`, какие RPC Tenant доступны ему и другим внутренним клиентам.
- Публичные HTTP-маршруты, DTO, ошибки и версия API для новых сервисов.
- Активный tenant/provider, матрица прав и поведение при блокировке tenant во время сессии.
- Статусы и компенсации checkout/hold/payment, idempotency keys и политика повторов.
- Какие экраны требуют BFF-композиции, polling или SSE/WebSocket.
- Схемы и версии событий NATS, допустимая задержка локальных read models.

Эти решения следует утверждать по конкретным сценариям; примерная структура каталогов сама по себе их не определяет.
