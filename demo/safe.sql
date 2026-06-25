-- A mesma migration reescrita de forma segura

SET lock_timeout = '3s';
SET statement_timeout = '30s';

-- P1: coluna sem default (sem reescrita de tabela)
ALTER TABLE orders ADD COLUMN status TEXT;
-- aplicar o default depois, em batches separados
ALTER TABLE orders ALTER COLUMN status SET DEFAULT 'pending';

-- P2: índice com CONCURRENTLY (não bloqueia escritas)
CREATE INDEX CONCURRENTLY idx_orders_status ON orders(status);

-- P3: constraint com NOT VALID (não escaneia a tabela)
ALTER TABLE orders ADD CONSTRAINT chk_valid_status
    CHECK (status IN ('pending', 'paid', 'cancelled')) NOT VALID;
-- validar em deploy separado:
-- ALTER TABLE orders VALIDATE CONSTRAINT chk_valid_status;

-- P5: NOT NULL via check constraint (sem full-scan com lock)
ALTER TABLE users ADD CONSTRAINT chk_email_nn
    CHECK (email IS NOT NULL) NOT VALID;
-- validar em deploy separado, depois:
-- ALTER TABLE users ALTER COLUMN email SET NOT NULL;
-- ALTER TABLE users DROP CONSTRAINT chk_email_nn;
