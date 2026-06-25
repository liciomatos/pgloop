-- WARN: múltiplas ops EXCLUSIVE na mesma migration
SET lock_timeout = '3s';
ALTER TABLE users ADD COLUMN score INT DEFAULT 0;
ALTER TABLE users DROP COLUMN old_score;
