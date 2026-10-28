BEGIN;
LOCK TABLE admin_service.sys_outbox_event IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM admin_service.sys_outbox_event) THEN
        RAISE EXCEPTION 'cannot revert Admin outbox migration while data exists';
    END IF;
END $$;

DROP TABLE admin_service.sys_outbox_event;
COMMIT;
