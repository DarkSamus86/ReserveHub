ALTER TABLE auth_service.users RENAME TO md_user;
ALTER TABLE auth_service.roles RENAME TO md_role;
ALTER TABLE auth_service.tenant_memberships RENAME TO ln_user_tenant_membership;
ALTER TABLE auth_service.invitations RENAME TO tr_tenant_invitation;
ALTER TABLE auth_service.provision_requests RENAME TO sys_owner_provision_request;
