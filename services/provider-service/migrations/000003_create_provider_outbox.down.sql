BEGIN;

LOCK TABLE provider.sys_outbox_event IN ACCESS EXCLUSIVE MODE;

DO $block$
BEGIN
    IF EXISTS (SELECT 1 FROM provider.sys_outbox_event) THEN
        RAISE EXCEPTION 'cannot drop provider outbox while events exist';
    END IF;
END
$block$;

DROP TABLE provider.sys_outbox_event;

COMMIT;
