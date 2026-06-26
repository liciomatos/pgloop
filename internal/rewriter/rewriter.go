package rewriter

import "github.com/liciomatos/pgloop/internal/lockmapper"

var recipes = map[lockmapper.PatternID]string{
	lockmapper.PatternAddColumnWithDefault: `-- Safe alternative:
ALTER TABLE t ADD COLUMN col TEXT;          -- no default, no table rewrite lock
UPDATE t SET col = 'value' WHERE id BETWEEN ? AND ?;  -- in batches
ALTER TABLE t ALTER COLUMN col SET DEFAULT 'value';`,

	lockmapper.PatternCreateIndexNoConcurrently: `-- Safe alternative:
CREATE INDEX CONCURRENTLY idx_name ON t(col);
-- Note: cannot run inside an explicit transaction (BEGIN/COMMIT)`,

	lockmapper.PatternAddConstraintNoNotValid: `-- Safe alternative:
ALTER TABLE t ADD CONSTRAINT chk_name CHECK (col IS NOT NULL) NOT VALID;
-- In a separate deploy:
ALTER TABLE t VALIDATE CONSTRAINT chk_name;`,

	lockmapper.PatternDropColumn: `-- Safe alternative (expand/contract):
-- 1. Remove all references to the column from application code
-- 2. Rename: ALTER TABLE t RENAME COLUMN col TO _col_unused;
-- 3. In a future deploy: ALTER TABLE t DROP COLUMN _col_unused;`,

	lockmapper.PatternSetNotNull: `-- Safe alternative:
ALTER TABLE t ADD CONSTRAINT chk_nn CHECK (col IS NOT NULL) NOT VALID;
ALTER TABLE t VALIDATE CONSTRAINT chk_nn;  -- separate deploy, no lock
ALTER TABLE t ALTER COLUMN col SET NOT NULL;  -- scan is skipped after validation
ALTER TABLE t DROP CONSTRAINT chk_nn;`,

	lockmapper.PatternRenameTableColumn: `-- Safe alternative (expand/contract):
-- 1. Add the new column/table
-- 2. Sync with a trigger
-- 3. Migrate reads/writes in application code
-- 4. Remove the trigger and old column in a future deploy`,

	lockmapper.PatternAlterColumnType: `-- Safe alternative:
ALTER TABLE t ADD COLUMN col_new NEW_TYPE;
-- Sync with a trigger during the transition
-- Migrate code to use col_new
-- DROP COLUMN col_old in a future deploy`,

	lockmapper.PatternTruncate: `-- Safe alternative:
SET lock_timeout = '2s';
DELETE FROM t WHERE created_at < now() - interval '1 year';  -- with lock_timeout
-- Or, if TRUNCATE is required:
TRUNCATE t CASCADE;  -- explicitly document the affected tables`,

	lockmapper.PatternNoTimeout: `-- Add at the beginning of the migration:
SET lock_timeout = '3s';
SET statement_timeout = '30s';
-- Or use: pgloop apply migration.sql  (injects them automatically)`,

	lockmapper.PatternMultipleExclusive: `-- Split into separate migrations:
-- migration_001_add_column.sql
-- migration_002_create_index.sql
-- Deploy gradually to reduce the risk window`,

	lockmapper.PatternDropIndexNoConcurrently: `-- Safe alternative:
DROP INDEX CONCURRENTLY idx_name;
-- Restrictions: cannot run inside BEGIN/COMMIT; does not support CASCADE.
-- For indexes backing UNIQUE or PRIMARY KEY constraints, drop the constraint instead:
ALTER TABLE t DROP CONSTRAINT constraint_name;  -- drops the index automatically`,

	lockmapper.PatternLockTable: `-- Avoid LOCK TABLE in migrations.
-- If serialization is needed, consider pg advisory locks:
SELECT pg_advisory_xact_lock(hashtext('my_migration_lock'));`,
}

func Suggestion(pattern lockmapper.PatternID) string {
	if s, ok := recipes[pattern]; ok {
		return s
	}
	return ""
}
