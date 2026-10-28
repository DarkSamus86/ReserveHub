BEGIN;
LOCK TABLE auth_service.tr_tenant_invitation IN ACCESS EXCLUSIVE MODE;

DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM auth_service.tr_tenant_invitation WHERE status = 'READY_TO_SEND') THEN
        RAISE EXCEPTION 'cannot revert invitation delivery state while READY_TO_SEND invitations exist';
    END IF;
END $$;

ALTER TABLE auth_service.tr_tenant_invitation
    DROP CONSTRAINT ck_invitation_status,
    DROP CONSTRAINT ck_invitation_secret;

ALTER TABLE auth_service.tr_tenant_invitation
    ADD CONSTRAINT invitations_status_check CHECK (status IN ('PENDING_DELIVERY', 'SENT', 'ACCEPTED', 'REVOKED')),
    ADD CONSTRAINT ck_invitation_secret CHECK (
        (status = 'PENDING_DELIVERY' AND token_hash IS NULL AND expires_at IS NULL AND sent_at IS NULL) OR
        (status = 'SENT' AND token_hash IS NOT NULL AND expires_at IS NOT NULL AND sent_at IS NOT NULL) OR
        (status = 'ACCEPTED' AND accepted_at IS NOT NULL) OR
        (status = 'REVOKED' AND revoked_at IS NOT NULL)
    );

COMMIT;
