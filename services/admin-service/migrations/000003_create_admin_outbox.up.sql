BEGIN;

-- id is the event_id used by consumers to deduplicate at-least-once delivery.
-- Administrative audit payload must exclude owner email and credentials.
CREATE TABLE admin_service.sys_outbox_event (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(64) NOT NULL CHECK (length(btrim(aggregate_type)) > 0),
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(128) NOT NULL CHECK (length(btrim(event_type)) > 0),
    schema_version INTEGER NOT NULL DEFAULT 1 CHECK (schema_version > 0),
    payload JSONB NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_sys_outbox_event_published_at CHECK (published_at IS NULL OR published_at >= created_at)
);

CREATE INDEX ix_admin_sys_outbox_event_pending
    ON admin_service.sys_outbox_event (next_attempt_at, created_at, id)
    WHERE published_at IS NULL;

COMMIT;
