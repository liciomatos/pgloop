-- Migration de exemplo: adiciona coluna de status, índice e constraint
-- Escrita da forma "óbvia" — mas cheia de armadilhas em produção

ALTER TABLE orders ADD COLUMN status TEXT DEFAULT 'pending';

CREATE INDEX idx_orders_status ON orders(status);

ALTER TABLE orders ADD CONSTRAINT chk_valid_status
    CHECK (status IN ('pending', 'paid', 'cancelled'));

ALTER TABLE users ALTER COLUMN email SET NOT NULL;
