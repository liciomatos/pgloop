-- CRITICAL: SET NOT NULL sem check constraint prévia
SET lock_timeout = '3s';
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
