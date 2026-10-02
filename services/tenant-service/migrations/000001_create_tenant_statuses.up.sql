BEGIN;

SET LOCAL search_path = tenant_service, public;

CREATE TABLE tenant_statuses (
    id SMALLINT PRIMARY KEY,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(64) NOT NULL,
    description TEXT
);

INSERT INTO tenant_statuses (id, code, name, description) VALUES
    (1, 'PENDING', 'Pending', 'Tenant is waiting for activation'),
    (2, 'ACTIVE', 'Active', 'Tenant is active'),
    (3, 'SUSPENDED', 'Suspended', 'Tenant is temporarily suspended'),
    (4, 'ARCHIVED', 'Archived', 'Tenant is archived');

COMMIT;
