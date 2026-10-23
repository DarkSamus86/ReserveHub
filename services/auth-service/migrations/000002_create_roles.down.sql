ALTER TABLE auth_service.users ADD COLUMN role TEXT CHECK (role IN ('admin', 'user'));

UPDATE auth_service.users AS users
SET role = roles.code
FROM auth_service.roles AS roles
WHERE roles.id = users.role_id;

ALTER TABLE auth_service.users ALTER COLUMN role SET NOT NULL;
ALTER TABLE auth_service.users DROP CONSTRAINT fk_users_role;
ALTER TABLE auth_service.users DROP COLUMN role_id;
DROP TABLE auth_service.roles;
