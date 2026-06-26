-- CRITICAL: LOCK TABLE explicit
SET lock_timeout = '3s';
LOCK TABLE orders IN EXCLUSIVE MODE;
ALTER TABLE orders ADD COLUMN processed BOOLEAN;
