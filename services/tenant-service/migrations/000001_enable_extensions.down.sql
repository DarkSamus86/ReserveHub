-- pgcrypto is database-wide and may be shared by other service schemas.
-- It is intentionally retained when tenant-service migrations are rolled back.
SELECT 1;
