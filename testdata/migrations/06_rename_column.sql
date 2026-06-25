-- WARN: RENAME COLUMN
SET lock_timeout = '3s';
ALTER TABLE users RENAME COLUMN name TO full_name;
