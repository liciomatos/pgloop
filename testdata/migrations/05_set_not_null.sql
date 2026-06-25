-- CRITICAL: SET NOT NULL without a prior check constraint
SET lock_timeout = '3s';
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
