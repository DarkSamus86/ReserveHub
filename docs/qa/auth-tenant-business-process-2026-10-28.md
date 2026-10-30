# QA: создание Tenant, приглашение владельца и сессия Auth

Дата: 2026-10-28. Объём: этап B из `docs/auth-tenant-integration.md`, включая Admin provisioning, Auth delivery/accept, refresh/logout, статус операции и безопасный audit outbox. Этап C (tenant-scoped доступ и gate активации Tenant) не входит в этот результат.

## Среда и запуск

- Временная PostgreSQL 18.6, отдельная БД `reservehub_qa_test`, миграции Auth `000001`–`000006` и Admin `000001`–`000004`. Все тестовые email и UUID синтетические.
- Auth: `GOCACHE=/tmp/reservehub-go-cache TEST_DATABASE_URL='postgres://darksamus@localhost:55432/reservehub_qa_test?host=/tmp/reservehub-integration-pg&sslmode=disable' go test ./... -count=1` из `services/auth-service` — **PASS**.
- Admin: та же команда из `services/admin-service` — **PASS**.
- Gateway: `GOCACHE=/tmp/reservehub-go-cache go test ./... -count=1` из `services/api-gateway` — **PASS**.
- Проверены QA тесты `services/auth-service/tests/business_process_integration_test.go` (пять сценариев) и `services/admin-service/internal/provision/qa_live_test.go` (один сценарий). Повторный прогон Auth QA на БД с ранее оставленными приглашениями — **PASS**.

## Сценарии и результаты

### AT-B-01 — новый владелец принимает одноразовое приглашение

Цель: владелец получает identity и членство только после подтверждения email и собственного пароля. Приоритет P0: преждевременная активация даёт доступ постороннему.

Предусловия: новый email, новый `tenant_id` и `operation_id`; Auth миграции применены; отправитель в тесте захватывает письмо в памяти.

1. Выполнить `ProvisionOwner`. Ожидается `pending_delivery`, `INVITED` user, `PENDING` membership, `PENDING_DELIVERY` invitation, отсутствие token hash и отказ входа.
2. Запустить delivery worker. Ожидается одно письмо, состояние `SENT`, токен только в тестовом отправителе; plaintext email/токен/пароль отсутствуют в outbox payload.
3. Вызвать accept без пароля. Ожидается отказ.
4. Отправить два одновременных accept с тем же токеном и выбранным паролем. Ожидается ровно один успех, один отказ; user/membership/invitation переходят в `ACTIVE`/`ACTIVE`/`ACCEPTED` в одной операции.
5. Повторить accept и войти с выбранным паролем. Ожидается отказ повтора и успешный вход.

Результат: **PASS** (`TestQANewOwnerInvitationAcceptanceAndReplay`). Очистка: тестовые записи остаются только во временной QA БД; каждый прогон использует новые UUID/email.

### AT-B-02 — существующий пользователь становится владельцем другой компании

Цель: сохранить прежнюю identity и пароль, не активируя новое членство до подтверждения. Приоритет P0: изменение пароля или автоматическая активация нарушают границу tenant.

Предусловия: зарегистрированный пользователь с синтетическим email и паролем, новый tenant.

1. Выполнить `ProvisionOwner` и проверить `PENDING` membership при `ACTIVE` user.
2. Доставить письмо; попытаться принять с новым паролем. Ожидается отказ и прежний `password_hash`.
3. Принять с токеном без нового пароля. Ожидается `ACTIVE` membership и неизменённый `password_hash`; вход со старым паролем успешен.

Результат: **PASS** (`TestQAExistingOwnerKeepsPasswordAndRefreshExpiresAbsolutely`). Очистка: синтетические записи в QA БД.

### AT-B-03 — SMTP отказ, повтор доставки и запрет раннего accept

Цель: ссылка не активирует владельца до SMTP ACK; повтор использует тот же токен до expiry. Приоритет P0: ранний accept обходит подтверждение доставки.

Предусловия: новый invite; тестовый отправитель на первом вызове возвращает ошибку, на втором — успех.

1. Запустить worker и дождаться первого отказа отправителя. Ожидается `READY_TO_SEND`, зашифрованный command в outbox без plaintext email/токена.
2. Попробовать accept известным тестовому отправителю токеном. Ожидается отказ.
3. Сдвинуть `next_attempt_at` тестовой записи на текущее время и дождаться retry. Ожидается тот же токен, затем `SENT`.
4. Принять токен. Ожидается успешная активация.

Результат: **PASS** (`TestQAInvitationCannotBeAcceptedBeforeSMTPAcknowledgement`). Очистка: только временная QA БД.

### AT-B-04 — срок приглашения и перевыдача

Цель: просроченный токен не действует, перевыдача аннулирует его и создаёт новый. Приоритет P1: восстановление после длительного отказа доставки.

Предусловия: приглашение `SENT`; в тестовой БД expiry сдвигается в прошлое.

1. Попробовать accept старым токеном. Ожидается отказ.
2. Вызвать защищённую операцию `ReissueInvitation`. Ожидается `PENDING_DELIVERY`, удалённый token hash и очищенный ciphertext предыдущей доставки.
3. Выполнить новую доставку. Ожидается новый токен; старый отвергается, новый принимается.

Результат: **PASS** (`TestQAExpiredInvitationCanBeReissuedWithoutOldTokenReuse`). Для `READY_TO_SEND` после истечения действует тот же repository метод; отдельный HTTP путь этого состояния не прогонялся.

### AT-B-05 — refresh и logout

Цель: refresh token одноразовый, хранится только hash, срок сессии абсолютный; logout запрещает новый refresh. Приоритет P0: повтор токена продлевает или восстанавливает доступ.

Предусловия: активный пользователь и выданная сессия.

1. Передать некорректный refresh. Ожидается отказ.
2. Обновить сессию. Ожидается новый refresh token, старый отвергается, `refresh_expires_at` не меняется, БД хранит SHA-256 hash.
3. Вызвать два refresh одновременно с одним ещё действительным токеном. Ожидается один успех и один отказ.
4. Вызвать logout текущим токеном. Ожидается отзыв; refresh и повтор logout с ним отвергаются.
5. Для отдельной тестовой сессии сдвинуть expiry в прошлое. Ожидается отказ refresh.

Результат: **PASS** (`TestQAExistingOwnerKeepsPasswordAndRefreshExpiresAbsolutely`). Ранее выданный access JWT не отзывается онлайн и остаётся действительным до своего 15-минутного срока по принятому MVP правилу.

### AT-B-06 — HTTP контракт и Admin статус/перевыдача

Цель: публичный Auth и административный HTTP показывают верный статус и требуют нужную роль. Приоритет P1: администратор должен видеть прогресс без доступа к секрету.

Предусловия: Auth тест использует настоящий HTTP handler с PostgreSQL; Admin тест — настоящий Admin HTTP handler и БД, fake Auth HTTP и fake Tenant gRPC, platform admin и platform user JWT.

1. Проверить internal Auth GET invitation status без сервисного токена и с ним. Ожидается 401 и 200; ответ содержит статус, но не invite token.
2. Вызвать публичные Auth accept, повтор accept, refresh, logout, refresh после logout. Ожидаются соответственно 200, 401, 200, 204, 401 и согласованный JSON.
3. Создать Admin operation. Ожидается 201 `completed` при `pending_delivery`, без email владельца в ответе.
4. Установить в fake Auth по очереди `PENDING_DELIVERY`, `READY_TO_SEND`, `SENT`, `ACCEPTED` и вызвать Admin GET. Ожидается live status/expiry; platform user получает 403.
5. Симулировать Auth 503. Ожидается сохранённый snapshot с `invite_status_unavailable=true`.
6. Сделать invite просроченным и вызвать Admin POST reissue: platform user получает 403, admin — 202, немедленный повтор — 409. Ожидается safe audit с actor и результатами `accepted`/`rejected`, без email/секрета.
7. Проверить Gateway reissue route: platform user получает 403 без обращения к upstream, admin получает 202, подставленный клиентом `X-Service-Token` не проходит в Admin.

Результат: **PASS** (`TestQAAuthHTTPInvitationAndSessionContracts`, `TestQALiveInvitationStatusReissueAndAudit`, `TestAdminReissueRouteRequiresAdminAndProxies`). Единый запущенный путь Gateway→Admin→Auth→Tenant не поднимался.

## Ограничения проверки

- Реальный SMTP-сервер, STARTTLS/TLS verification, inbox и фактическая доставка письма не проверялись; использован контролируемый sender, сохраняющий токен только в памяти теста.
- NATS JetStream publication/consumer deduplication не проверялись на работающем broker; проверены persisted outbox payload и сборка publisher. Потребитель audit-service ещё отсутствует.
- Tenant остаётся `PENDING` после accept. Tenant activation gate, выбор tenant context, permission checks и tenant isolation относятся к этапу C и здесь не подтверждены.
- Admin тест live status/reissue использует fake Auth; Auth endpoint и бизнес-логика проверены отдельно. Полный стек через Gateway в одном процессе не запускался.

Подтверждённых дефектов в проверенных сценариях нет. Риск этапа B: при недоступности Auth Admin GET показывает старый snapshot с явным `invite_status_unavailable=true`; оператор должен учитывать этот флаг. Долгий SMTP outage требует явного Admin reissue после expiry.
