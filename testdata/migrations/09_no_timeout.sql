-- WARN: migration without timeout
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
