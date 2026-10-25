# Топология взаимодействия микросервисов ReserveHub

Документ описывает, как внешние клиенты и внутренние сервисы взаимодействуют в ReserveHub. Основан на [ReserveHub-FULL-SPEC.md](ReserveHub-FULL-SPEC.md). Спецификация задаёт обязанности сервисов и список событий NATS, но не фиксирует протокол для каждой пары сервисов. Поэтому документ разделяет требования Full-Spec и рекомендуемые архитектурные решения.

## Коротко

- Внешние приложения обращаются к публичному API только через `api-gateway` по HTTP(S).
- Gateway проверяет входящий запрос и направляет его владельцу соответствующей функции. Сервисы не публикуют свои пользовательские API напрямую.
- Синхронные внутренние запросы, которым нужен немедленный ответ, выполняются по gRPC.
- NATS JetStream распространяет события о фактах, которые уже произошли, между независимыми подписчиками.
- Tenant и прочие внутренние сервисы не должны быть доступны из интернета. Gateway не должен становиться транспортом для внутренних вызовов между сервисами.

## Поток клиентского запроса

```mermaid
flowchart LR
    Customer[customer-web]
    Provider[provider-admin]
    Platform[platform-admin]
    Gateway[api-gateway<br/>public HTTP(S)]
    Backend[Domain services<br/>private gRPC / internal APIs]
    Tenant[tenant-service<br/>private gRPC]
    Admin[admin-service / platform backend]
    Events[NATS JetStream]

    Customer --> Gateway
    Provider --> Gateway
    Platform --> Gateway
    Gateway --> Backend
    Backend <--> |sync gRPC when immediate result is needed| Backend
    Admin --> |authorized gRPC| Tenant
    Backend --> Events
    Events --> Backend
```

Схема показывает рекомендуемые границы. Full-Spec перечисляет `api-gateway`, но не описывает `admin-service`; этот сервис или аналогичный platform-admin backend предлагается как изолированный посредник между административным API и Tenant.

## Клиентские API через Gateway

Все три приложения из Full-Spec используют Gateway как публичную точку входа. Доступ к конкретным операциям ограничивается ролью пользователя, его tenant и правами на ресурс.

| Клиент | Функции, доступные через Gateway | Сервисы-владельцы функций |
| --- | --- | --- |
| `customer-web` | Регистрация и вход, профиль, поиск и каталог, расписание и доступность, бронь, checkout, оплата, заказы, билеты и подтверждения | `identity-service`, `user-service`, `search-service`, `catalog-service`, `schedule-service`, `inventory-service`, `reservation-service`, `pricing-service`, `promotion-service`, `order-service`, `payment-service`, `fulfillment-service`, `recommendation-service` |
| `provider-admin` | Управление провайдером и сотрудниками, каталогом, площадками, расписанием, inventory, ценами, заказами, акциями, интеграциями, аналитикой и billing | `provider-service`, `user-service`, `catalog-service`, `venue-service`, `route-service`, `schedule-service`, `inventory-service`, `pricing-service`, `order-service`, `promotion-service`, `integration-service`, `analytics-service`, `billing-service` |
| `platform-admin` | Управление tenants и providers, платформенные аналитика и billing, feature flags, аудит и системные операции | административный backend, `tenant-service`, `provider-service`, `analytics-service`, `billing-service`, `audit-service` и сервисы конфигурации |

Таблица описывает функциональную маршрутизацию, а не требование предоставить отдельный публичный маршрут каждому сервису. Например, UI может вызвать один маршрут Gateway для checkout, после чего backend координирует pricing, promotions, reservation, order и payment внутри приватной сети.

Gateway отвечает за общие edge-функции: проверку аутентификации и RBAC, rate limiting, валидацию входных запросов, request/correlation ID, tenant context и при необходимости агрегацию ответов. Бизнес-правила и окончательная авторизация на ресурсах остаются в сервисах-владельцах данных.

## Внутренние синхронные вызовы по gRPC

Используйте gRPC, если вызывающий сервис не может принять решение или ответить клиенту без актуального результата от другого сервиса.

| Вызывающий | Получатель | Пример запроса |
| --- | --- | --- |
| `reservation-service` | `inventory-service` | Удержать места или capacity, подтвердить, освободить hold; нужна защита от двойного бронирования. |
| Checkout / `order-service` | `pricing-service` | Получить серверную котировку с итоговой ценой и сроком действия. Клиентская цена не является доверенной. |
| Checkout / `order-service` | `promotion-service` | Проверить промокод и зарезервировать ограниченное число применений, если это нужно для атомарного checkout. |
| `order-service` | `reservation-service` | Проверить или подтвердить reservation при переходе заказа к оплате. |
| `payment-service` | `order-service` | Проверить состояние заказа при подтверждении оплаты или возврате. |
| `catalog-service` | `venue-service`, `schedule-service` | Проверить актуальные связи площадки и расписания при создании или публикации предложения. |
| `provider-service` и бизнес-сервисы | Tenant-контекст или контролируемый внутренний API | Проверить статус tenant, план или флаг, если без свежих данных нельзя выполнить операцию. Вызов должен быть ограничен по полномочиям и сетевой доступности. |
| `search-service` | `ranking-service` | Получить синхронный score, если ранжирование входит в критический путь поискового ответа. |
| Backend сервиса | `fraud-service` | Получить риск-сигнал перед критическим действием. ML-результат остаётся сигналом; решение принимается серверными правилами. |

Это примеры для проектирования, а не утверждённые Full-Spec RPC-контракты. Для каждого вызова следует определить владельца данных, deadline, таймаут, retry policy, поведение при недоступности downstream и правила идемпотентности. Не повторяйте автоматически неидемпотентные команды.

### Tenant и платформенное администрирование

`tenant-service` предоставляет внутренние операции жизненного цикла tenant, плана, статуса, настроек и feature flags. Он не должен иметь публичного HTTP/gRPC-маршрута для браузера или обычного пользователя.

Рекомендуемая граница для платформенных административных команд:

```text
platform-admin UI -> api-gateway -> admin-service -> tenant-service (gRPC)
```

`admin-service` в Full-Spec пока не перечислен и должен считаться предлагаемым компонентом. Он даёт возможность не подключать публичный Gateway к приватной сети Tenant и концентрирует платформенную авторизацию и правила администрирования. Если другим сервисам нужны tenant-настройки в критическом пути, нужно отдельно утвердить список разрешённых внутренних клиентов и узкий RPC-контракт. Альтернатива для некритичных данных — распространение ограниченного tenant-контекста через события и локальный read model.

При любом варианте нельзя полагаться только на сетевую изоляцию: Tenant должен проверять service identity/caller authorization, а пользовательские права на tenant и ресурс должны быть проверены на backend-границе.

## Асинхронные события через NATS JetStream

NATS подходит для публикации уже принятого факта, когда несколько независимых сервисов должны обновить собственные данные или выполнить побочную работу. Событие не должно подменять синхронное решение, необходимое для завершения текущей команды.

| Событие / источник | Типичные подписчики |
| --- | --- |
| `tenant.created` и изменения tenant | `provider-service`, billing, audit и потребители конфигурации |
| `user.registered` | `notification-service`, analytics |
| `catalog.item.published` и изменения каталога | `search-service`, analytics, recommendation/ranking |
| Изменения расписания и доступности | `search-service`, `notification-service`, reservation-процессы |
| `reservation.*` | `order-service`, notification, analytics и audit |
| `order.*` и `payment.*` | fulfillment, billing, analytics, notification и audit |
| `fulfillment.*` | notification, analytics и audit |
| Изменения интеграций | `integration-service`, monitoring/analytics и audit |

Список подписчиков — рекомендация, основанная на обязанностях сервисов в Full-Spec. Точные имена событий, схемы payload, версии, гарантии доставки и владельцы публикации должны быть описаны отдельными контрактами.

Для критических событий, связанных с reservation, order, payment, refund и fulfillment, Full-Spec требует transactional outbox. Это связывает фиксацию бизнес-изменения в PostgreSQL с надёжной публикацией в NATS. Потребители должны учитывать повторную доставку и быть идемпотентными.

## Внешние webhooks и интеграции

### Webhook платёжного провайдера

Платёжный провайдер — внешний отправитель, поэтому webhook является входящим публичным запросом. Его можно провести через выделенный маршрут Gateway к `payment-service` либо через отдельный webhook ingress. Это отдельный режим аутентификации: нужно проверить подпись провайдера, защититься от повторной отправки и обеспечить идемпотентную обработку. Пользовательский JWT сам по себе не аутентифицирует провайдера.

### Webhooks внешних интеграций

Callbacks интеграций маршрутизируются к `integration-service` через ограниченный маршрут Gateway или специализированный ingress. Проверяйте credentials/signature конкретной интеграции, применяйте replay protection и tenant-aware authorization. Внутренняя синхронизация после приёма callback выполняется gRPC или NATS в зависимости от того, нужен ли немедленный ответ.

### Вызовы наружу

Вызовы payment provider, email/SMS провайдерам, внешним системам inventory и travel API выполняет соответствующий владеющий сервис (`payment-service`, `notification-service`, `integration-service`). Эти запросы не идут через публичный API Gateway.

## Правило выбора транспорта

| Нужное взаимодействие | Транспорт |
| --- | --- |
| Браузер или внешний клиент вызывает функцию ReserveHub | HTTP(S) через `api-gateway` |
| Другому сервису нужен немедленный ответ или команда должна завершиться синхронно | Внутренний gRPC |
| Нужно уведомить несколько независимых сервисов о зафиксированном факте | NATS JetStream |
| Внешний платёжный или интеграционный сервис присылает callback | Ограниченный webhook ingress; подпись провайдера, replay protection, идемпотентность |
| Внутренний сервис вызывает другой сервис через публичный Gateway | Не использовать; вызывать владельца API напрямую по внутреннему контракту |

## Требует отдельного решения

Перед массовым расширением маршрутов Gateway необходимо определить:

1. Каталог публичных HTTP-маршрутов и соответствующие backend-владельцы.
2. Внутренние gRPC API, service identity, RBAC и сетевые ACL для каждого вызывающего.
3. Tenant context: какие данные идут в токене, какие проверяются синхронно и какие могут быть локально реплицированы.
4. Схемы, версии, владельцев и гарантии доставки событий NATS.
5. Таймауты, retry, circuit breaking, идемпотентность и fallback для синхронных цепочек.
6. Отдельные правила входа для платёжных и интеграционных webhooks.
