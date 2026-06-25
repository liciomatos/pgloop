-- CRITICAL: DROP COLUMN
SET lock_timeout = '3s';
ALTER TABLE users DROP COLUMN legacy_field;
