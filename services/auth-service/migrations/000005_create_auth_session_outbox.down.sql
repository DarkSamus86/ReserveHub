BEGIN;
LOCK TABLE auth_service.tr_auth_session, auth_service.sys_outbox_event IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM auth_service.tr_auth_session)
        OR EXISTS (SELECT 1 FROM auth_service.sys_outbox_event) THEN
        RAISE EXCEPTION 'cannot revert Auth session/outbox migration while data exists';
    END IF;
END $$;

DROP TABLE auth_service.sys_outbox_event;
DROP TABLE auth_service.tr_auth_session;
COMMIT;
