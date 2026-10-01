# ReserveHub — MVP Scope

## 1. Цель MVP

MVP должен доказать основной end-to-end flow ReserveHub:

```text
Provider creates event
        ↓
Customer finds event
        ↓
Customer selects inventory
        ↓
Reservation is created
        ↓
Customer pays using test provider
        ↓
Order becomes paid
        ↓
QR ticket is generated
        ↓
Provider sees sale in admin
```

MVP не должен пытаться реализовать все будущие verticals.

---

# 2. MVP Vertical

Первая версия:

```text
Event Ticketing
```

Поддержать:

- cinema;
- concerts;
- theatre;
- sport;
- shows.

Реально достаточно иметь 2-3 demo categories, но core не должен быть cinema-specific.

---

# 3. MVP Roles

## Customer

- register;
- login;
- browse;
- reserve;
- pay;
- receive ticket;
- view orders.

## Provider Admin

- manage catalog;
- manage venues;
- manage schedules;
- manage inventory;
- manage pricing;
- view orders;
- view basic analytics.

## Platform Admin

- create tenant;
- create/activate provider;
- view basic platform metrics.

---

# 4. MVP Frontends

## customer-web

Required:

- login/register;
- home/catalog;
- search;
- event page;
- schedule selection;
- seat/zone selection;
- reservation timer;
- checkout;
- test payment;
- QR ticket;
- order history.

## provider-admin

Required:

- login;
- dashboard;
- catalog CRUD;
- venue CRUD;
- seat/zone layout;
- schedule CRUD;
- pricing;
- inventory;
- orders;
- basic analytics.

## platform-admin

Required:

- login;
- tenant list;
- create tenant;
- provider list;
- activate/deactivate;
- basic metrics.

---

# 5. MVP Microservices

Do not start with every planned microservice.

Required services:

## 5.1 api-gateway

- HTTP/JSON;
- routing;
- auth context;
- tenant context;
- rate limit basic;
- request ID.

## 5.2 identity-service

- register;
- login;
- logout;
- access token;
- refresh token;
- RBAC.

## 5.3 tenant-service

- create tenant;
- get tenant;
- list tenants;
- activate;
- deactivate.

## 5.4 provider-service

- create provider;
- provider profile;
- provider status;
- provider-tenant relation.

## 5.5 catalog-service

- create event;
- edit;
- publish;
- archive;
- categories;
- basic media URL.

## 5.6 venue-service

- venue;
- hall;
- zone;
- seat;
- layout.

## 5.7 schedule-service

- occurrence/session;
- start/end;
- status;
- cancellation.

## 5.8 inventory-service

- availability;
- hold;
- release;
- confirm;
- seat inventory;
- zone/capacity inventory;
- concurrency protection.

## 5.9 pricing-service

- base price;
- seat/zone price;
- price quote;
- server-side total.

## 5.10 reservation-service

- create reservation;
- expiration;
- cancellation;
- confirmation;
- idempotency.

## 5.11 order-service

- order;
- order items;
- order totals;
- status;
- paid/cancelled.

## 5.12 payment-service

Only test/mock payment provider.

Required:

- create payment;
- mark success;
- mark failure;
- webhook simulation;
- idempotency.

## 5.13 fulfillment-service

- QR ticket;
- ticket status;
- validation endpoint.

## 5.14 notification-service

MVP:

- email abstraction;
- log/mock provider acceptable initially;
- payment success;
- ticket issued.

## 5.15 audit-service

MVP:

- admin actions;
- price changes;
- schedule changes;
- tenant/provider changes.

---

# 6. MVP Infrastructure

Required:

- PostgreSQL;
- Redis;
- NATS JetStream;
- Docker;
- Docker Compose.

Not required initially:

- Kubernetes;
- ClickHouse;
- OpenSearch.

---

# 7. MVP Events

Implement only important events:

```text
tenant.created
provider.created
catalog.item.published
schedule.created
inventory.held
inventory.released
reservation.created
reservation.expired
reservation.confirmed
order.created
payment.completed
payment.failed
order.paid
fulfillment.created
```

---

# 8. MVP Security

Required:

- password hashing;
- JWT/refresh;
- RBAC;
- tenant isolation;
- BOLA/IDOR tests;
- rate limiting for auth;
- SQL parameterization;
- CORS config;
- secure headers;
- webhook idempotency;
- server-side pricing;
- double-booking tests;
- secret scanning;
- dependency scanning.

---

# 9. MVP Observability

Required:

- structured logs;
- request ID;
- correlation ID;
- Prometheus metrics;
- Grafana dashboard;
- basic service health.

Optional for MVP:

- Loki;
- OpenTelemetry tracing.

---

# 10. MVP Environments

Required:

## Local

Docker Compose.

## Staging

Public or private staging environment.

## Production Demo

Live demo for portfolio.

Staging and production must use separate DB/config.

---

# 11. MVP CI/CD

PR pipeline:

- Go format;
- vet;
- lint;
- unit tests;
- integration tests;
- TypeScript checks;
- container build;
- dependency scan;
- secret scan.

Staging:

- deploy from develop.

Production:

- deploy from main/release tag.

---

# 12. MVP Analytics

Provider:

- sales count;
- revenue;
- paid orders;
- reservations;
- occupancy;
- top events.

Platform:

- tenant count;
- provider count;
- user count;
- total orders;
- GMV;
- basic platform revenue placeholder.

Do not add ClickHouse yet.

---

# 13. MVP Demo Data

Create:

- 2 tenants;
- 2-3 providers;
- several venues;
- several events;
- several schedules;
- seat and zone inventory;
- demo customers;
- demo orders.

---

# 14. MVP Demo Flow

Customer:

```text
Open ReserveHub
  ↓
Register
  ↓
Find event
  ↓
Select session
  ↓
Select seat
  ↓
Reserve
  ↓
Test payment
  ↓
Receive QR ticket
```

Provider:

```text
Login
  ↓
Create event
  ↓
Create schedule
  ↓
Configure inventory
  ↓
Set price
  ↓
Publish
  ↓
See sale
```

Platform Admin:

```text
Login
  ↓
Create tenant
  ↓
Create/enable provider
  ↓
View platform metrics
```

---

# 15. What Is NOT MVP

Do not implement yet:

- hotels;
- apartments;
- tours;
- rail;
- flights;
- transfers;
- loyalty;
- dynamic pricing ML;
- recommendations;
- fraud ML;
- LLM assistant;
- OpenSearch;
- ClickHouse;
- Kubernetes;
- advanced billing;
- custom domains;
- advanced promo engine;
- real payment provider;
- external travel providers.

---

# 16. MVP Definition of Done

MVP is complete when:

- local deployment works;
- staging works;
- production demo works;
- customer flow works end-to-end;
- provider admin works;
- platform admin works;
- reservation prevents double booking;
- payment mock is idempotent;
- QR ticket is generated;
- tenant isolation is tested;
- basic audit exists;
- logs and metrics exist;
- CI/CD works;
- README exists;
- architecture docs exist;
- demo accounts exist;
- security checklist exists;
- main flows have integration tests.

---

# 17. MVP Suggested Milestones

## Milestone 1 — Foundation

- repo;
- Docker;
- PostgreSQL;
- NATS;
- Redis;
- gateway;
- identity;
- tenant;
- provider.

## Milestone 2 — Catalog

- catalog;
- venue;
- schedule.

## Milestone 3 — Inventory

- seats;
- zones;
- availability;
- pricing.

## Milestone 4 — Reservation

- holds;
- TTL;
- concurrency;
- expiration.

## Milestone 5 — Commerce

- order;
- test payment;
- idempotency.

## Milestone 6 — Fulfillment

- QR;
- validation.

## Milestone 7 — Admin

- provider admin;
- platform admin.

## Milestone 8 — Production Readiness

- CI/CD;
- staging;
- monitoring;
- security tests;
- production demo.

---

# 18. MVP Stack

Backend:

```text
Go
gRPC
PostgreSQL
pgx
sqlc
NATS JetStream
Redis
```

Frontend:

```text
TypeScript
React / Next.js
```

Infrastructure:

```text
Docker
Docker Compose
Prometheus
Grafana
```

AI/ML:

```text
Not required for MVP
```

Python services are added after core product flow is stable.
