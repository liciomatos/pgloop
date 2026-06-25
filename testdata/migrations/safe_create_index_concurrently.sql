-- OK: CREATE INDEX CONCURRENTLY (seguro)
SET lock_timeout = '3s';
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
