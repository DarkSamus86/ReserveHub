BEGIN;

ALTER TABLE auth_service.tr_tenant_invitation
    DROP CONSTRAINT invitations_status_check,
    DROP CONSTRAINT ck_invitation_secret;

ALTER TABLE auth_service.tr_tenant_invitation
    ADD CONSTRAINT ck_invitation_status CHECK (status IN ('PENDING_DELIVERY', 'READY_TO_SEND', 'SENT', 'ACCEPTED', 'REVOKED')),
    ADD CONSTRAINT ck_invitation_secret CHECK (
        (status = 'PENDING_DELIVERY' AND token_hash IS NULL AND expires_at IS NULL AND sent_at IS NULL AND accepted_at IS NULL AND revoked_at IS NULL) OR
        (status = 'READY_TO_SEND' AND token_hash IS NOT NULL AND expires_at IS NOT NULL AND sent_at IS NULL AND accepted_at IS NULL AND revoked_at IS NULL) OR
        (status = 'SENT' AND token_hash IS NOT NULL AND expires_at IS NOT NULL AND sent_at IS NOT NULL AND accepted_at IS NULL AND revoked_at IS NULL) OR
        (status = 'ACCEPTED' AND token_hash IS NOT NULL AND expires_at IS NOT NULL AND sent_at IS NOT NULL AND accepted_at IS NOT NULL AND revoked_at IS NULL) OR
        (status = 'REVOKED' AND revoked_at IS NOT NULL AND accepted_at IS NULL)
    );

COMMIT;
