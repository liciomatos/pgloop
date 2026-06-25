-- OK: ADD CONSTRAINT with NOT VALID (safe)
SET lock_timeout = '3s';
ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;
ALTER TABLE orders VALIDATE CONSTRAINT fk_user;
