ALTER TABLE admin_service.tr_tenant_provision_operation
    ADD COLUMN invitation_id UUID;

CREATE INDEX ix_admin_provision_invitation_id
    ON admin_service.tr_tenant_provision_operation (invitation_id)
    WHERE invitation_id IS NOT NULL;
