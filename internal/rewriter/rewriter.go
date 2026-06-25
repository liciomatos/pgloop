package rewriter

import "github.com/liciomatos/pgloop/internal/lockmapper"

var recipes = map[lockmapper.PatternID]string{
	lockmapper.PatternAddColumnWithDefault: `-- Opção segura:
ALTER TABLE t ADD COLUMN col TEXT;          -- sem default, sem lock de reescrita
UPDATE t SET col = 'valor' WHERE id BETWEEN ? AND ?;  -- em batches
ALTER TABLE t ALTER COLUMN col SET DEFAULT 'valor';`,

	lockmapper.PatternCreateIndexNoConcurrently: `-- Opção segura:
CREATE INDEX CONCURRENTLY idx_name ON t(col);
-- Atenção: não rode dentro de uma transação explícita (BEGIN/COMMIT)`,

	lockmapper.PatternAddConstraintNoNotValid: `-- Opção segura:
ALTER TABLE t ADD CONSTRAINT chk_name CHECK (col IS NOT NULL) NOT VALID;
-- Em deploy separado:
ALTER TABLE t VALIDATE CONSTRAINT chk_name;`,

	lockmapper.PatternDropColumn: `-- Opção segura (expand/contract):
-- 1. Remova referências à coluna no código
-- 2. Renomeie: ALTER TABLE t RENAME COLUMN col TO _col_unused;
-- 3. Em deploy futuro: ALTER TABLE t DROP COLUMN _col_unused;`,

	lockmapper.PatternSetNotNull: `-- Opção segura:
ALTER TABLE t ADD CONSTRAINT chk_nn CHECK (col IS NOT NULL) NOT VALID;
ALTER TABLE t VALIDATE CONSTRAINT chk_nn;  -- em deploy separado, sem lock
ALTER TABLE t SET NOT NULL col;            -- após validação, o scan é pulado
ALTER TABLE t DROP CONSTRAINT chk_nn;`,

	lockmapper.PatternRenameTableColumn: `-- Opção segura (expand/contract):
-- 1. Adicione a nova coluna/tabela
-- 2. Sincronize com trigger
-- 3. Migre leituras/escritas no código
-- 4. Remova o trigger e a coluna antiga em deploy futuro`,

	lockmapper.PatternAlterColumnType: `-- Opção segura:
ALTER TABLE t ADD COLUMN col_new NEW_TYPE;
-- Sincronize com trigger durante transição
-- Migre código para usar col_new
-- DROP COLUMN col_old em deploy futuro`,

	lockmapper.PatternTruncate: `-- Opção segura:
SET lock_timeout = '2s';
DELETE FROM t WHERE created_at < now() - interval '1 year';  -- com lock_timeout
-- Ou, se precisar de TRUNCATE:
TRUNCATE t CASCADE;  -- documente as tabelas afetadas explicitamente`,

	lockmapper.PatternNoTimeout: `-- Adicione no início da migration:
SET lock_timeout = '3s';
SET statement_timeout = '30s';
-- Ou use: pgloop apply migration.sql  (injeta automaticamente)`,

	lockmapper.PatternMultipleExclusive: `-- Quebre em migrations separadas:
-- migration_001_add_column.sql
-- migration_002_create_index.sql
-- Execute com deploys graduais para reduzir janela de risco`,
}

func Suggestion(pattern lockmapper.PatternID) string {
	if s, ok := recipes[pattern]; ok {
		return s
	}
	return ""
}
