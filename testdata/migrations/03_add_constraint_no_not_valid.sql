-- CRITICAL: ADD CONSTRAINT without NOT VALID
SET lock_timeout = '3s';
ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id);
