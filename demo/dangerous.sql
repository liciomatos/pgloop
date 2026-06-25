-- Example migration: adds a status column, index, and constraint
-- Written the "obvious" way — but full of traps in production

ALTER TABLE orders ADD COLUMN status TEXT DEFAULT 'pending';

CREATE INDEX idx_orders_status ON orders(status);

ALTER TABLE orders ADD CONSTRAINT chk_valid_status
    CHECK (status IN ('pending', 'paid', 'cancelled'));

ALTER TABLE users ALTER COLUMN email SET NOT NULL;
