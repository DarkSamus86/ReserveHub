# ReserveHub — Full Product Specification

## 1. Назначение продукта

**ReserveHub** — multi-tenant booking, reservation and commerce platform.

Платформа объединяет:

- marketplace для покупателей;
- SaaS-панель для поставщиков;
- управление предложениями;
- управление доступностью;
- бронирование;
- оплату;
- fulfillment;
- аналитику;
- billing;
- integrations;
- security;
- AI/ML.

Цель ReserveHub — поддерживать разные типы бронируемых сущностей через единый core.

---

# 2. Поддерживаемые verticals

## 2.1 Events

- Cinema
- Concerts
- Theatre
- Sport
- Shows
- Festivals
- Conferences
- Exhibitions
- Other live events

## 2.2 Travel

- Hotels
- Apartments
- Tours
- Activities
- Transfers
- Rail
- Flights

## 2.3 Future

- Vehicle rentals
- Equipment rentals
- Workspaces
- Time-slot services
- Other reservable resources

---

# 3. Основные типы пользователей

## 3.1 Customer

Возможности:

- регистрация;
- вход;
- восстановление пароля;
- управление профилем;
- поиск предложений;
- фильтрация;
- просмотр карточки предложения;
- просмотр расписания;
- просмотр доступности;
- выбор места / зоны / тарифа;
- бронирование;
- оплата;
- получение билета / voucher / confirmation;
- просмотр истории заказов;
- возвраты;
- отмены;
- уведомления;
- сохраненные предложения;
- персональные рекомендации;
- reviews позже;
- loyalty позже.

---

## 3.2 Provider Owner

Владелец tenant/provider.

Возможности:

- полный доступ к provider;
- управление сотрудниками;
- управление billing;
- управление интеграциями;
- управление настройками;
- управление branding;
- управление всеми предложениями;
- управление выплатами;
- просмотр всей аналитики.

---

## 3.3 Provider Admin

Возможности:

- создание и редактирование предложений;
- расписание;
- площадки;
- inventory;
- pricing;
- orders;
- refunds;
- сотрудники;
- promotions;
- аналитика;
- настройки.

---

## 3.4 Provider Manager

Возможности:

- управление каталогом;
- расписание;
- inventory;
- цены;
- bookings;
- orders.

---

## 3.5 Cashier

Возможности:

- продажа;
- поиск заказа;
- возврат по разрешенным правилам;
- проверка билета.

---

## 3.6 Analyst

Возможности:

- dashboard;
- отчеты;
- экспорт;
- агрегированная аналитика.

---

## 3.7 Support

Возможности:

- просмотр customer/order/reservation;
- помощь клиенту;
- переотправка fulfillment;
- ограниченные refund operations.

---

## 3.8 Platform Super Admin

Возможности:

- создание tenant;
- управление tenants;
- управление providers;
- platform billing;
- platform analytics;
- feature flags;
- audit;
- security overview;
- system health;
- support tooling;
- блокировка providers;
- управление platform-wide config.

---

# 4. Customer Marketplace

## 4.1 Home

- featured offers;
- popular categories;
- trending events;
- personalized recommendations;
- nearby later;
- promo blocks;
- search.

## 4.2 Search

Поддержка:

- free text;
- category;
- city;
- date;
- price;
- provider;
- availability;
- rating later;
- tags;
- transport route later.

## 4.3 Catalog Item Page

- title;
- description;
- media;
- provider;
- location;
- schedule;
- price range;
- available inventory;
- policies;
- metadata;
- related offers;
- recommendations.

## 4.4 Availability

Поддержка:

- seat map;
- zones;
- general admission;
- capacity;
- room nights;
- fare classes;
- time slots;
- packages.

## 4.5 Reservation

- hold inventory;
- TTL;
- expiration timer;
- release;
- retry;
- idempotency;
- double-booking protection.

## 4.6 Checkout

- customer details;
- order summary;
- server-side pricing;
- promo code;
- fees;
- taxes;
- payment method;
- confirmation.

## 4.7 Orders

- order history;
- order details;
- status;
- invoices later;
- refunds;
- cancellation;
- re-delivery of fulfillment.

## 4.8 Fulfillment

Possible types:

- QR ticket;
- barcode;
- voucher;
- booking confirmation;
- hotel confirmation;
- external reservation reference;
- travel confirmation.

---

# 5. Provider Admin

## 5.1 Dashboard

- revenue;
- GMV;
- orders;
- reservations;
- conversion;
- refund rate;
- occupancy;
- average order value;
- top products;
- peak hours;
- recent activity.

## 5.2 Catalog Management

- create offer;
- edit offer;
- draft;
- publish;
- unpublish;
- archive;
- media;
- categories;
- tags;
- localization;
- metadata.

## 5.3 Venue Management

- create venue;
- halls;
- sectors;
- zones;
- seats;
- layouts;
- seat types;
- capacity.

## 5.4 Route Management

Future:

- stations;
- airports;
- stops;
- segments;
- routes;
- transport metadata.

## 5.5 Schedule Management

- create occurrence;
- recurring schedules;
- cancel occurrence;
- reschedule;
- capacity;
- cutoff rules;
- timezone support.

## 5.6 Inventory Management

- seat inventory;
- zone inventory;
- capacity inventory;
- room inventory;
- fare inventory;
- availability overrides;
- manual block;
- release.

## 5.7 Pricing Management

- base price;
- seat/zone price;
- schedule price;
- weekday/weekend;
- promo price;
- provider fee;
- taxes;
- dynamic pricing later;
- currency.

## 5.8 Orders

- list;
- search;
- filters;
- status;
- customer;
- payment status;
- refund;
- cancellation;
- fulfillment status.

## 5.9 Sales

- online sales;
- cashier sales;
- channel breakdown;
- settlements;
- exports.

## 5.10 Employees

- invite;
- deactivate;
- roles;
- permissions;
- organization units later.

## 5.11 Promotions

- promo codes;
- discounts;
- campaigns;
- usage limits;
- date constraints;
- category constraints;
- customer constraints.

## 5.12 Branding

- logo;
- colors;
- public provider page;
- custom domain later;
- email templates;
- ticket templates.

## 5.13 Integrations

- payment provider;
- email provider;
- SMS later;
- external inventory;
- webhooks;
- API credentials.

---

# 6. Platform Admin

## 6.1 Tenant Management

- create;
- activate;
- suspend;
- delete/archive;
- plan;
- limits;
- feature flags;
- domains;
- branding status.

## 6.2 Provider Management

- providers by tenant;
- verification;
- status;
- integration status;
- sales status.

## 6.3 Platform Analytics

- total GMV;
- platform revenue;
- active tenants;
- active providers;
- users;
- orders;
- reservations;
- payment success;
- refunds;
- revenue by category;
- revenue by provider;
- growth;
- conversion.

## 6.4 Platform Billing

- subscriptions;
- commissions;
- invoices;
- provider fees;
- billing history;
- plan limits.

## 6.5 Feature Flags

- provider-specific;
- tenant-specific;
- global;
- staged rollout.

## 6.6 Audit

- actor;
- action;
- resource;
- old/new;
- timestamp;
- tenant;
- request ID;
- IP.

## 6.7 System Health

- service health;
- DB health;
- NATS health;
- Redis health;
- API latency;
- 5xx rate;
- payment failures;
- queue lag.

---

# 7. Core Microservices

## 7.1 api-gateway

Responsibilities:

- public API;
- routing;
- auth middleware;
- RBAC;
- request validation;
- rate limiting;
- tenant context;
- request ID;
- correlation ID;
- response aggregation;
- WebSocket/SSE.

---

## 7.2 identity-service

Responsibilities:

- registration;
- login;
- logout;
- password reset;
- email verification;
- access tokens;
- refresh token rotation;
- sessions;
- RBAC;
- MFA later;
- service identity.

Entities:

- Identity
- Credential
- Session
- RefreshToken
- Role
- Permission
- UserRole

---

## 7.3 user-service

Responsibilities:

- customer profile;
- contact data;
- preferences;
- notification settings;
- localization;
- saved items later.

---

## 7.4 tenant-service

Responsibilities:

- tenant lifecycle;
- plans;
- status;
- settings;
- feature flags;
- domains;
- timezone;
- currency;
- locale.

---

## 7.5 provider-service

Responsibilities:

- provider profile;
- brand;
- verification status;
- provider settings;
- provider staff relation;
- provider type.

---

## 7.6 catalog-service

Responsibilities:

- catalog item;
- category;
- tags;
- media metadata;
- publish lifecycle;
- vertical-specific metadata.

Types:

- CINEMA
- CONCERT
- THEATRE
- SPORT
- SHOW
- FESTIVAL
- HOTEL
- TOUR
- ACTIVITY
- RAIL
- FLIGHT
- TRANSFER
- OTHER

---

## 7.7 venue-service

Responsibilities:

- venues;
- halls;
- sectors;
- zones;
- seats;
- layouts;
- seat types;
- capacity.

---

## 7.8 route-service

Future travel service.

Responsibilities:

- routes;
- stops;
- stations;
- airports;
- segments;
- route metadata.

---

## 7.9 schedule-service

Responsibilities:

- occurrences;
- sessions;
- departures;
- time slots;
- start/end time;
- timezone;
- cancellation;
- reschedule;
- recurrence later.

---

## 7.10 inventory-service

Responsibilities:

- availability;
- hold;
- release;
- confirm;
- sold/reserved state;
- concurrency protection;
- source-of-truth rules.

Inventory types:

- SEAT
- ZONE
- GENERAL_ADMISSION
- CAPACITY
- ROOM_NIGHT
- FARE_CLASS
- VEHICLE
- TIME_SLOT
- PACKAGE

---

## 7.11 pricing-service

Responsibilities:

- base price;
- price rules;
- provider fee;
- platform fee;
- taxes;
- promo adjustments;
- dynamic pricing later;
- price quote;
- quote expiration.

Rule:

Frontend never defines trusted final price.

---

## 7.12 search-service

Responsibilities:

- search;
- filters;
- autocomplete;
- sorting;
- indexing;
- ranking;
- category search;
- location search;
- route search later.

Storage:

- PostgreSQL initially;
- OpenSearch later.

---

## 7.13 reservation-service

Responsibilities:

- create reservation;
- hold inventory;
- expiration;
- release;
- confirmation;
- cancellation;
- idempotency;
- double-booking prevention.

States:

- CREATED
- HELD
- CONFIRMED
- EXPIRED
- CANCELLED

---

## 7.14 order-service

Responsibilities:

- commercial order;
- order items;
- totals;
- status;
- customer snapshot;
- payment relation;
- refund state.

States:

- CREATED
- WAITING_PAYMENT
- PAID
- CANCELLED
- REFUNDED
- PARTIALLY_REFUNDED

---

## 7.15 payment-service

Responsibilities:

- create payment;
- payment provider abstraction;
- payment attempt;
- webhook;
- signature verification;
- idempotency;
- refund;
- reconciliation;
- payment audit.

---

## 7.16 fulfillment-service

Responsibilities:

- ticket;
- QR;
- barcode;
- voucher;
- booking confirmation;
- external booking reference;
- validation;
- invalidation;
- use/scan history.

---

## 7.17 notification-service

Channels:

- email;
- push;
- Telegram;
- SMS later.

Triggers:

- registration;
- reservation;
- expiration;
- payment;
- refund;
- cancellation;
- fulfillment;
- schedule change.

---

## 7.18 analytics-service

Responsibilities:

- provider analytics;
- platform analytics;
- customer behavior aggregates;
- conversion;
- revenue;
- GMV;
- occupancy;
- sales;
- refund metrics.

Storage:

- PostgreSQL initially;
- ClickHouse later.

---

## 7.19 billing-service

Responsibilities:

- provider subscription;
- commission;
- platform fee;
- plan;
- invoices;
- billing history;
- provider balance later.

---

## 7.20 audit-service

Responsibilities:

- administrative audit;
- security-sensitive actions;
- immutable action history;
- investigation support.

---

## 7.21 promotion-service

Responsibilities:

- promo codes;
- campaigns;
- discounts;
- conditions;
- limits;
- redemption tracking.

---

## 7.22 loyalty-service

Future.

Responsibilities:

- points;
- cashback;
- tiers;
- accrual;
- redemption;
- expiration.

---

## 7.23 integration-service

Responsibilities:

- external booking providers;
- external inventory;
- external travel APIs;
- synchronization;
- adapters;
- retries;
- provider health;
- reconciliation.

---

# 8. AI / ML Services

Python is used for ML/AI.

## 8.1 recommendation-service

Functions:

- personalized offers;
- similar items;
- related categories;
- provider recommendations.

Signals:

- views;
- searches;
- reservations;
- purchases;
- preferences.

---

## 8.2 ranking-service

Functions:

- search ranking;
- personalization;
- ranking score;
- contextual ordering.

---

## 8.3 forecasting-service

Functions:

- demand forecast;
- expected sales;
- occupancy forecast;
- peak prediction;
- provider planning.

---

## 8.4 fraud-service

Functions:

- anomaly detection;
- suspicious reservation patterns;
- payment anomaly signal;
- promo abuse signal;
- account abuse signal.

ML output is advisory unless explicitly governed by backend rules.

---

## 8.5 ai-assistant-service

Future.

Functions:

- natural language search;
- customer support assistant;
- provider assistant;
- analytics summaries;
- description generation.

Example:

"Найди концерт в Астане на выходных дешевле 20000"

AI converts request to structured search criteria.

---

# 9. Event-Driven System

Broker:

- NATS JetStream

Important events:

- tenant.created
- provider.created
- user.registered
- catalog.item.created
- catalog.item.published
- schedule.created
- schedule.changed
- schedule.cancelled
- inventory.held
- inventory.released
- inventory.confirmed
- reservation.created
- reservation.expired
- reservation.confirmed
- reservation.cancelled
- order.created
- order.paid
- order.cancelled
- order.refunded
- payment.created
- payment.completed
- payment.failed
- payment.refunded
- fulfillment.created
- fulfillment.used
- fulfillment.cancelled
- provider.sync.started
- provider.sync.completed
- provider.sync.failed

---

# 10. Transactional Outbox

Critical business events use transactional outbox.

Required for:

- reservation;
- order;
- payment;
- refund;
- fulfillment.

---

# 11. Security Functionality

ReserveHub is also a practical web security project.

## 11.1 Authentication Security

- password hashing;
- token rotation;
- session invalidation;
- brute-force protection;
- MFA later;
- secure recovery flow.

## 11.2 Authorization

- RBAC;
- resource ownership;
- tenant isolation;
- BOLA/IDOR protection;
- permission checks.

## 11.3 API Security

- rate limiting;
- schema validation;
- secure CORS;
- request size limits;
- security headers;
- TLS;
- anti-abuse controls.

## 11.4 Payment Security

- signed webhooks;
- replay protection;
- idempotency;
- server-side amount validation;
- provider transaction reconciliation.

## 11.5 Business Logic Security

- double-booking protection;
- price tampering protection;
- promo abuse protection;
- refund rules;
- fulfillment replay protection.

## 11.6 Infrastructure Security

- secret management;
- least privilege;
- container hardening;
- dependency scanning;
- image scanning;
- backup encryption later;
- isolated environments.

---

# 12. Security Testing

Pipeline includes:

- SAST;
- dependency scanning;
- container scanning;
- secret scanning;
- API tests;
- authorization tests;
- tenant isolation tests;
- manual pentest;
- regression tests for every fixed vulnerability.

Vulnerability docs:

- RH-SEC-001
- RH-SEC-002
- ...

---

# 13. Observability

## Logs

Все микросервисы используют единый ReserveHub logging contract: одна JSON-запись на строку в stdout, общий формат timestamp/уровня/имени события и одинаковые правила безопасного логирования. Подробная схема обязательна для всех сервисов и описана в [Logging Convention](../observability/logging.md).

Обязательные базовые поля:

- `timestamp` — UTC RFC 3339;
- `level` — `DEBUG`, `INFO`, `WARN`, `ERROR`;
- `message` — стабильное имя события в `snake_case`;
- `service` — имя сервиса, совпадающее с Compose;
- `environment` — значение `APP_ENV`.

Для запросов дополнительно логируются `request_id`, длительность и транспортные поля: HTTP `http_method`, route template и `http_status`; gRPC `grpc_method` и `grpc_code`. `trace_id`, `span_id`, `tenant_id`, `user_id` и resource ID включаются, когда достоверно известны и их логирование разрешено.

Каждый сервис принимает `X-Request-ID`/`x-request-id` либо генерирует UUID и возвращает ID в ответе. Пароли, токены, секреты, credentials и полные тела запросов/ответов не логируются. Для auth-запросов допускаются маршрут, статус и длительность, но не email, пароль или bearer token.

## Metrics

- RPS;
- latency;
- error rate;
- DB connections;
- NATS lag;
- reservation conflicts;
- payment failures;
- refund failures;
- inventory holds;
- Redis health.

## Tracing

- OpenTelemetry.

## Stack

- Prometheus;
- Grafana;
- Loki;
- Tempo/Jaeger later.

---

# 14. Frontend Applications

## 14.1 customer-web

TypeScript + React/Next.js.

Features:

- homepage;
- search;
- categories;
- product details;
- availability;
- reservation;
- checkout;
- payment;
- fulfillment;
- profile;
- order history;
- recommendations.

## 14.2 provider-admin

Features:

- dashboard;
- catalog;
- venues;
- routes;
- schedules;
- inventory;
- pricing;
- orders;
- sales;
- employees;
- promotions;
- analytics;
- billing;
- integrations;
- settings.

## 14.3 platform-admin

Features:

- tenants;
- providers;
- platform analytics;
- billing;
- feature flags;
- audit;
- security overview;
- system health.

---

# 15. Data Stores

## PostgreSQL

Primary transactional database.

## Redis

Use cases:

- reservation TTL;
- cache;
- rate limiting;
- ephemeral state.

## ClickHouse

Future analytics.

## OpenSearch

Future search.

## Object Storage

Future:

- S3-compatible storage;
- media;
- generated documents;
- exports.

---

# 16. Environments

- local;
- staging;
- production.

Each environment has separate:

- databases;
- Redis;
- NATS;
- secrets;
- integrations;
- payment config.

---

# 17. CI/CD

## Go

- format;
- vet;
- lint;
- tests;
- integration tests;
- build.

## TypeScript

- lint;
- typecheck;
- tests;
- build.

## Python

- ruff;
- mypy;
- pytest;
- model checks later.

## Security

- dependency scan;
- secret scan;
- container scan.

---

# 18. Deployment

Initial:

- VPS;
- Docker;
- Docker Compose;
- reverse proxy;
- PostgreSQL;
- Redis;
- NATS;
- Prometheus;
- Grafana;
- Loki.

Later:

- Kubernetes;
- HA;
- autoscaling;
- service discovery;
- managed databases if required.

---

# 19. Business Models

ReserveHub may support:

- monthly subscription;
- commission;
- hybrid;
- enterprise plan.

Possible plans:

- Free
- Basic
- Business
- Enterprise

---

# 20. Future Hotel Functionality

Entities:

- Property;
- Room;
- RoomType;
- RatePlan;
- RoomAvailability;
- Stay;
- Guest;
- CancellationPolicy;
- Amenity;
- PropertyPolicy.

Features:

- date-range availability;
- occupancy;
- room pricing;
- cancellation;
- guest details;
- confirmation;
- provider integrations.

---

# 21. Future Tours / Activities

Entities:

- Tour;
- Activity;
- Departure;
- Capacity;
- Guide;
- PickupPoint;
- Package;
- Voucher.

Features:

- group capacity;
- time slots;
- participant details;
- tour options;
- meeting point;
- voucher.

---

# 22. Future Rail

Entities:

- Train;
- Route;
- Segment;
- Station;
- Carriage;
- Seat;
- Fare;
- Passenger;
- External Reservation.

Features:

- route search;
- seat availability;
- segment-aware inventory;
- passenger data;
- external provider sync.

---

# 23. Future Flights

Entities:

- Flight;
- Airport;
- Route;
- Segment;
- Fare;
- FareClass;
- Passenger;
- Baggage;
- PNR;
- ExternalBooking.

Features:

- flight search;
- fare rules;
- baggage;
- rebooking;
- cancellation;
- schedule changes;
- external provider integrations.

---

# 24. Product Evolution

ReserveHub is not considered "finished" after v1.

Expected lifecycle:

- feature development;
- security fixes;
- performance improvements;
- new providers;
- new verticals;
- analytics improvements;
- ML improvements;
- infrastructure evolution;
- UI improvements;
- documentation maintenance.

---

# 25. Final Product Vision

ReserveHub should eventually provide a common platform for:

```text
DISCOVER
  ↓
SEARCH
  ↓
CHECK AVAILABILITY
  ↓
PRICE
  ↓
RESERVE
  ↓
ORDER
  ↓
PAY
  ↓
FULFILL
  ↓
NOTIFY
  ↓
ANALYZE
```

while supporting:

- multiple tenants;
- multiple providers;
- multiple verticals;
- secure isolation;
- external integrations;
- AI/ML;
- production operations.
