-- OK: ADD COLUMN without default (no table rewrite)
SET lock_timeout = '3s';
ALTER TABLE users ADD COLUMN score INT;
