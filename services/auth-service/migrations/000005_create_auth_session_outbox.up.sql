BEGIN;

CREATE TABLE auth_service.tr_auth_session (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth_service.md_user (id) ON DELETE RESTRICT,
    current_refresh_token_hash BYTEA NOT NULL,
    rotation_counter BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    rotated_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT uq_tr_auth_session_refresh_hash UNIQUE (current_refresh_token_hash),
    CONSTRAINT ck_tr_auth_session_hash_size CHECK (octet_length(current_refresh_token_hash) = 32),
    CONSTRAINT ck_tr_auth_session_rotation_counter CHECK (rotation_counter >= 0),
    CONSTRAINT ck_tr_auth_session_expiry CHECK (expires_at > created_at),
    CONSTRAINT ck_tr_auth_session_rotated_at CHECK (rotated_at IS NULL OR rotated_at >= created_at),
    CONSTRAINT ck_tr_auth_session_revoked_at CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX ix_tr_auth_session_user_unrevoked
    ON auth_service.tr_auth_session (user_id)
    WHERE revoked_at IS NULL;

-- id is the event_id used by consumers to deduplicate at-least-once delivery.
-- Producers must put only safe metadata (or ciphertext under a separately managed
-- key) in payload; never plaintext invite/refresh secrets, passwords, or hashes.
CREATE TABLE auth_service.sys_outbox_event (
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

CREATE INDEX ix_auth_sys_outbox_event_pending
    ON auth_service.sys_outbox_event (next_attempt_at, created_at, id)
    WHERE published_at IS NULL;

COMMIT;
