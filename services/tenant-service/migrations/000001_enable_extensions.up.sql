-- pgcrypto belongs to the shared database, not the tenant-service schema.
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;
