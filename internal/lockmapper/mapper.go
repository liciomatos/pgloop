package lockmapper

import (
	"fmt"
	"strings"

	"github.com/liciomatos/pgloop/internal/parser"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type RiskLevel string
type LockMode string
type PatternID int

const (
	RiskCritical RiskLevel = "CRITICAL"
	RiskWarn     RiskLevel = "WARN"
	RiskOK       RiskLevel = "OK"

	LockAccessExclusive      LockMode = "ACCESS EXCLUSIVE"
	LockShare                LockMode = "SHARE"
	LockShareUpdateExclusive LockMode = "SHARE UPDATE EXCLUSIVE"
	LockNone                 LockMode = "NONE"
)

const (
	PatternAddColumnWithDefault      PatternID = 1
	PatternCreateIndexNoConcurrently PatternID = 2
	PatternAddConstraintNoNotValid   PatternID = 3
	PatternDropColumn                PatternID = 4
	PatternSetNotNull                PatternID = 5
	PatternRenameTableColumn         PatternID = 6
	PatternAlterColumnType           PatternID = 7
	PatternTruncate                  PatternID = 8
	PatternNoTimeout                 PatternID = 9
	PatternMultipleExclusive         PatternID = 10
	PatternDropIndexNoConcurrently   PatternID = 11
	PatternLockTable                 PatternID = 12
)

// AnalyzeOptions controls analysis behavior.
type AnalyzeOptions struct {
	// PGVersion is the target PostgreSQL major version (e.g. 14).
	// 0 = unspecified: uses conservative behavior (assumes PG10).
	PGVersion int
}

// PatternInfo describes a pattern detectable by the linter.
type PatternInfo struct {
	ID          PatternID
	Code        string   // e.g. "P1" — used with --ignore
	Name        string   // short pattern name
	LockMode    LockMode // lock acquired by the statement
	Risk        RiskLevel
	VersionNote string // note about version-specific behavior
}

// AllPatterns returns all supported patterns in ID order.
func AllPatterns() []PatternInfo {
	return []PatternInfo{
		{PatternAddColumnWithDefault, "P1", "ADD COLUMN with DEFAULT", LockAccessExclusive, RiskCritical, "WARN on PG11+"},
		{PatternCreateIndexNoConcurrently, "P2", "CREATE INDEX without CONCURRENTLY", LockShare, RiskCritical, ""},
		{PatternAddConstraintNoNotValid, "P3", "ADD CONSTRAINT without NOT VALID", LockAccessExclusive, RiskCritical, ""},
		{PatternDropColumn, "P4", "DROP COLUMN", LockAccessExclusive, RiskCritical, ""},
		{PatternSetNotNull, "P5", "SET NOT NULL without prior check constraint", LockAccessExclusive, RiskCritical, ""},
		{PatternRenameTableColumn, "P6", "RENAME TABLE / RENAME COLUMN", LockAccessExclusive, RiskWarn, ""},
		{PatternAlterColumnType, "P7", "ALTER COLUMN TYPE", LockAccessExclusive, RiskCritical, ""},
		{PatternTruncate, "P8", "TRUNCATE", LockAccessExclusive, RiskCritical, ""},
		{PatternNoTimeout, "P9", "Migration without lock_timeout / statement_timeout", LockNone, RiskWarn, ""},
		{PatternMultipleExclusive, "P10", "Multiple ACCESS EXCLUSIVE operations in the same migration", LockAccessExclusive, RiskWarn, ""},
		{PatternDropIndexNoConcurrently, "P11", "DROP INDEX without CONCURRENTLY", LockAccessExclusive, RiskCritical, ""},
		{PatternLockTable, "P12", "LOCK TABLE explicit", LockAccessExclusive, RiskCritical, ""},
	}
}

type LintResult struct {
	Statement  string
	LockMode   LockMode
	Risk       RiskLevel
	Pattern    PatternID
	Message    string // issue description
	Suggestion string // safe rewrite recipe (populated by cmd via rewriter)
	Line       int
	Synthetic  bool // true for P9/P10: file-level diagnostic, not tied to a specific statement
}

func Analyze(stmts []parser.Statement, sql string, opts AnalyzeOptions) []LintResult {
	var results []LintResult

	exclusiveCount := 0
	for _, stmt := range stmts {
		for _, result := range analyzeStatement(stmt, sql, opts) {
			results = append(results, result)
			if result.LockMode == LockAccessExclusive {
				exclusiveCount++
			}
		}
	}

	if !hasTimeoutSet(stmts) && len(stmts) > 0 {
		results = append(results, LintResult{
			LockMode:  LockNone,
			Risk:      RiskWarn,
			Pattern:   PatternNoTimeout,
			Message:   "Migration has no lock_timeout or statement_timeout. Use pgloop apply to inject them automatically.",
			Line:      1,
			Synthetic: true,
		})
	}

	if exclusiveCount >= 2 {
		results = append(results, LintResult{
			LockMode:  LockAccessExclusive,
			Risk:      RiskWarn,
			Pattern:   PatternMultipleExclusive,
			Message:   "Multiple ACCESS EXCLUSIVE operations in the same migration. Split into separate migrations.",
			Synthetic: true,
		})
	}

	return results
}

// analyzeStatement returns zero or more issues for a single SQL statement.
// ALTER TABLE can contain multiple commands (ADD COLUMN, DROP COLUMN, etc.) in one statement,
// so it delegates to analyzeAlterTable which returns a complete list.
func analyzeStatement(stmt parser.Statement, fullSQL string, opts AnalyzeOptions) []LintResult {
	node := stmt.Node
	line := lineOf(fullSQL, stmt.Position)

	if alter := node.GetAlterTableStmt(); alter != nil {
		return analyzeAlterTable(alter, stmt.Raw, line, opts)
	}
	if index := node.GetIndexStmt(); index != nil {
		if result := analyzeCreateIndex(index, stmt.Raw, line); result != nil {
			return []LintResult{*result}
		}
		return nil
	}
	if node.GetRenameStmt() != nil {
		return []LintResult{{
			Statement: stmt.Raw,
			LockMode:  LockAccessExclusive,
			Risk:      RiskWarn,
			Pattern:   PatternRenameTableColumn,
			Message:   "RENAME locks the schema. Use expand/contract: add a compatibility view and migrate gradually.",
			Line:      line,
		}}
	}
	if node.GetTruncateStmt() != nil {
		return []LintResult{{
			Statement: stmt.Raw,
			LockMode:  LockAccessExclusive,
			Risk:      RiskCritical,
			Pattern:   PatternTruncate,
			Message:   "TRUNCATE acquires ACCESS EXCLUSIVE. Prefer DELETE with lock_timeout or explicit TRUNCATE ... CASCADE.",
			Line:      line,
		}}
	}
	if drop := node.GetDropStmt(); drop != nil {
		if result := analyzeDropIndex(drop, stmt.Raw, line); result != nil {
			return []LintResult{*result}
		}
		return nil
	}
	if node.GetLockStmt() != nil {
		return []LintResult{{
			Statement: stmt.Raw,
			LockMode:  LockAccessExclusive,
			Risk:      RiskCritical,
			Pattern:   PatternLockTable,
			Message:   "Explicit LOCK TABLE holds the lock for the entire transaction. Avoid in migrations; prefer advisory locks or redesign the flow.",
			Line:      line,
		}}
	}
	return nil
}

// analyzeAlterTable detects all issues in the commands of an ALTER TABLE statement.
// A single ALTER TABLE can contain multiple dangerous commands.
func analyzeAlterTable(alter *pg_query.AlterTableStmt, raw string, line int, opts AnalyzeOptions) []LintResult {
	var results []LintResult

	for _, cmdNode := range alter.Cmds {
		cmd := cmdNode.GetAlterTableCmd()
		if cmd == nil {
			continue
		}
		switch cmd.Subtype {
		case pg_query.AlterTableType_AT_AddColumn:
			col := cmd.Def.GetColumnDef()
			if col != nil && columnHasDefault(col) {
				results = append(results, *addColumnWithDefaultResult(raw, line, opts.PGVersion))
			}
		case pg_query.AlterTableType_AT_DropColumn:
			results = append(results, LintResult{
				Statement: raw,
				LockMode:  LockAccessExclusive,
				Risk:      RiskCritical,
				Pattern:   PatternDropColumn,
				Message:   "DROP COLUMN acquires ACCESS EXCLUSIVE and rewrites the catalog. Use expand/contract: rename to _unused first, remove in a future deploy.",
				Line:      line,
			})
		case pg_query.AlterTableType_AT_SetNotNull:
			results = append(results, LintResult{
				Statement: raw,
				LockMode:  LockAccessExclusive,
				Risk:      RiskCritical,
				Pattern:   PatternSetNotNull,
				Message:   "SET NOT NULL scans the entire table. Add CHECK (col IS NOT NULL) NOT VALID, validate with VALIDATE CONSTRAINT, then SET NOT NULL.",
				Line:      line,
			})
		case pg_query.AlterTableType_AT_AlterColumnType:
			results = append(results, LintResult{
				Statement: raw,
				LockMode:  LockAccessExclusive,
				Risk:      RiskCritical,
				Pattern:   PatternAlterColumnType,
				Message:   "ALTER COLUMN TYPE rewrites the table. Use a new column + sync trigger + gradual swap.",
				Line:      line,
			})
		case pg_query.AlterTableType_AT_AddConstraint:
			constr := cmd.Def.GetConstraint()
			if constr != nil && !constr.SkipValidation {
				results = append(results, LintResult{
					Statement: raw,
					LockMode:  LockAccessExclusive,
					Risk:      RiskCritical,
					Pattern:   PatternAddConstraintNoNotValid,
					Message:   "ADD CONSTRAINT without NOT VALID scans the entire table. Use ADD CONSTRAINT ... NOT VALID; then VALIDATE CONSTRAINT ... in a separate deploy.",
					Line:      line,
				})
			}
		}
	}
	return results
}

// addColumnWithDefaultResult returns the LintResult for P1, adjusted by PG version.
// In PG11+, ADD COLUMN with DEFAULT does not rewrite the table (fast column add),
// but still acquires ACCESS EXCLUSIVE briefly to update the catalog.
func addColumnWithDefaultResult(raw string, line int, pgVersion int) *LintResult {
	switch {
	case pgVersion == 0:
		// Version unspecified: conservative diagnosis.
		return &LintResult{
			Statement: raw,
			LockMode:  LockAccessExclusive,
			Risk:      RiskCritical,
			Pattern:   PatternAddColumnWithDefault,
			Message:   "ADD COLUMN with DEFAULT rewrites the entire table (PG10 and earlier). If you are on PG11+, use --pg-version 11 for accurate diagnosis.",
			Line:      line,
		}
	case pgVersion < 11:
		return &LintResult{
			Statement: raw,
			LockMode:  LockAccessExclusive,
			Risk:      RiskCritical,
			Pattern:   PatternAddColumnWithDefault,
			Message:   fmt.Sprintf("ADD COLUMN with DEFAULT rewrites the entire table on PG%d. Add the column without a default, then UPDATE in batches.", pgVersion),
			Line:      line,
		}
	default:
		// PG11+: no table rewrite, but catalog lock still exists.
		return &LintResult{
			Statement: raw,
			LockMode:  LockAccessExclusive,
			Risk:      RiskWarn,
			Pattern:   PatternAddColumnWithDefault,
			Message:   fmt.Sprintf("On PG%d, ADD COLUMN with DEFAULT does not rewrite the table, but still acquires ACCESS EXCLUSIVE briefly to update the catalog.", pgVersion),
			Line:      line,
		}
	}
}

func analyzeDropIndex(drop *pg_query.DropStmt, raw string, line int) *LintResult {
	if drop.RemoveType == pg_query.ObjectType_OBJECT_INDEX && !drop.Concurrent {
		return &LintResult{
			Statement: raw,
			LockMode:  LockAccessExclusive,
			Risk:      RiskCritical,
			Pattern:   PatternDropIndexNoConcurrently,
			Message:   "DROP INDEX without CONCURRENTLY acquires ACCESS EXCLUSIVE, blocking reads and writes during the drop. Use DROP INDEX CONCURRENTLY.",
			Line:      line,
		}
	}
	return nil
}

func analyzeCreateIndex(index *pg_query.IndexStmt, raw string, line int) *LintResult {
	if !index.Concurrent {
		return &LintResult{
			Statement: raw,
			LockMode:  LockShare,
			Risk:      RiskCritical,
			Pattern:   PatternCreateIndexNoConcurrently,
			Message:   "CREATE INDEX without CONCURRENTLY blocks writes during the build. Use CREATE INDEX CONCURRENTLY.",
			Line:      line,
		}
	}
	return nil
}

func columnHasDefault(col *pg_query.ColumnDef) bool {
	for _, constraint := range col.Constraints {
		constr := constraint.GetConstraint()
		if constr != nil && constr.Contype == pg_query.ConstrType_CONSTR_DEFAULT {
			return true
		}
	}
	return false
}

func hasTimeoutSet(stmts []parser.Statement) bool {
	for _, stmt := range stmts {
		varStmt := stmt.Node.GetVariableSetStmt()
		if varStmt == nil {
			continue
		}
		name := strings.ToLower(varStmt.Name)
		if name == "lock_timeout" || name == "statement_timeout" {
			return true
		}
	}
	return false
}

func lineOf(sql string, pos int) int {
	if pos <= 0 {
		return 1
	}
	if pos > len(sql) {
		pos = len(sql)
	}
	return strings.Count(sql[:pos], "\n") + 1
}
