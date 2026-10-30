DROP INDEX IF EXISTS admin_service.ix_admin_provision_invitation_id;

ALTER TABLE admin_service.tr_tenant_provision_operation
    DROP COLUMN invitation_id;
