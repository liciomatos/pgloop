-- The same migration rewritten the safe way

SET lock_timeout = '3s';
SET statement_timeout = '30s';

-- P1: column without default (no table rewrite)
ALTER TABLE orders ADD COLUMN status TEXT;
-- apply the default later, in separate batches
ALTER TABLE orders ALTER COLUMN status SET DEFAULT 'pending';

-- P2: index with CONCURRENTLY (does not block writes)
CREATE INDEX CONCURRENTLY idx_orders_status ON orders(status);

-- P3: constraint with NOT VALID (does not scan the table)
ALTER TABLE orders ADD CONSTRAINT chk_valid_status
    CHECK (status IN ('pending', 'paid', 'cancelled')) NOT VALID;
-- validate in a separate deploy:
-- ALTER TABLE orders VALIDATE CONSTRAINT chk_valid_status;

-- P5: NOT NULL via check constraint (no full-scan with lock)
ALTER TABLE users ADD CONSTRAINT chk_email_nn
    CHECK (email IS NOT NULL) NOT VALID;
-- validate in a separate deploy, then:
-- ALTER TABLE users ALTER COLUMN email SET NOT NULL;
-- ALTER TABLE users DROP CONSTRAINT chk_email_nn;
