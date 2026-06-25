-- CRITICAL: CREATE INDEX without CONCURRENTLY
SET lock_timeout = '3s';
CREATE INDEX idx_orders_user_id ON orders(user_id);
