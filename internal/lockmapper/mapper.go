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
)

// AnalyzeOptions controla o comportamento da análise.
type AnalyzeOptions struct {
	// PGVersion é a versão major do PostgreSQL alvo (ex: 14).
	// 0 = não especificada: usa comportamento conservador (assume PG10).
	PGVersion int
}

// PatternInfo descreve um padrão detectável pelo lint.
type PatternInfo struct {
	ID          PatternID
	Code        string   // ex: "P1" — usado em --ignore
	Name        string   // nome curto do padrão
	LockMode    LockMode // lock adquirido pelo statement
	Risk        RiskLevel
	VersionNote string // nota sobre comportamento diferente por versão do PG
}

// AllPatterns retorna todos os padrões suportados, na ordem de ID.
func AllPatterns() []PatternInfo {
	return []PatternInfo{
		{PatternAddColumnWithDefault, "P1", "ADD COLUMN com DEFAULT", LockAccessExclusive, RiskCritical, "WARN em PG11+"},
		{PatternCreateIndexNoConcurrently, "P2", "CREATE INDEX sem CONCURRENTLY", LockShare, RiskCritical, ""},
		{PatternAddConstraintNoNotValid, "P3", "ADD CONSTRAINT sem NOT VALID", LockAccessExclusive, RiskCritical, ""},
		{PatternDropColumn, "P4", "DROP COLUMN", LockAccessExclusive, RiskCritical, ""},
		{PatternSetNotNull, "P5", "SET NOT NULL sem check constraint prévia", LockAccessExclusive, RiskCritical, ""},
		{PatternRenameTableColumn, "P6", "RENAME TABLE / RENAME COLUMN", LockAccessExclusive, RiskWarn, ""},
		{PatternAlterColumnType, "P7", "ALTER COLUMN TYPE", LockAccessExclusive, RiskCritical, ""},
		{PatternTruncate, "P8", "TRUNCATE", LockAccessExclusive, RiskCritical, ""},
		{PatternNoTimeout, "P9", "Migration sem lock_timeout / statement_timeout", LockNone, RiskWarn, ""},
		{PatternMultipleExclusive, "P10", "Múltiplas operações EXCLUSIVE na mesma migration", LockAccessExclusive, RiskWarn, ""},
	}
}

type LintResult struct {
	Statement  string
	LockMode   LockMode
	Risk       RiskLevel
	Pattern    PatternID
	Message    string // descrição do problema
	Suggestion string // receita de reescrita segura (preenchida pelo cmd via rewriter)
	Line       int
	Synthetic  bool // true para P9/P10: diagnóstico de arquivo, não de um statement específico
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
			Message:   "Migration sem lock_timeout ou statement_timeout. Use pgloop apply para injetar automaticamente.",
			Line:      1,
			Synthetic: true,
		})
	}

	if exclusiveCount >= 2 {
		results = append(results, LintResult{
			LockMode:  LockAccessExclusive,
			Risk:      RiskWarn,
			Pattern:   PatternMultipleExclusive,
			Message:   "Múltiplas operações ACCESS EXCLUSIVE na mesma migration. Quebre em migrations separadas.",
			Synthetic: true,
		})
	}

	return results
}

// analyzeStatement retorna zero ou mais problemas para um único statement SQL.
// ALTER TABLE pode ter múltiplos comandos (ADD COLUMN, DROP COLUMN, etc.) na mesma instrução,
// então delega para analyzeAlterTable que retorna uma lista completa.
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
			Message:   "RENAME bloqueia o schema. Use expand/contract: adicione uma view de compatibilidade e migre gradualmente.",
			Line:      line,
		}}
	}
	if node.GetTruncateStmt() != nil {
		return []LintResult{{
			Statement: stmt.Raw,
			LockMode:  LockAccessExclusive,
			Risk:      RiskCritical,
			Pattern:   PatternTruncate,
			Message:   "TRUNCATE adquire ACCESS EXCLUSIVE. Prefira DELETE com lock_timeout ou TRUNCATE ... CASCADE explícito.",
			Line:      line,
		}}
	}
	return nil
}

// analyzeAlterTable detecta todos os problemas nos comandos de um ALTER TABLE.
// Uma única instrução ALTER TABLE pode conter múltiplos comandos perigosos.
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
				Message:   "DROP COLUMN adquire ACCESS EXCLUSIVE e reescreve o catálogo. Use expand/contract: renomeie para _unused primeiro e remova em deploy futuro.",
				Line:      line,
			})
		case pg_query.AlterTableType_AT_SetNotNull:
			results = append(results, LintResult{
				Statement: raw,
				LockMode:  LockAccessExclusive,
				Risk:      RiskCritical,
				Pattern:   PatternSetNotNull,
				Message:   "SET NOT NULL escaneia a tabela inteira. Adicione CHECK (col IS NOT NULL) NOT VALID, valide com VALIDATE CONSTRAINT, depois SET NOT NULL.",
				Line:      line,
			})
		case pg_query.AlterTableType_AT_AlterColumnType:
			results = append(results, LintResult{
				Statement: raw,
				LockMode:  LockAccessExclusive,
				Risk:      RiskCritical,
				Pattern:   PatternAlterColumnType,
				Message:   "ALTER COLUMN TYPE reescreve a tabela. Use nova coluna + trigger de sync + swap gradual.",
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
					Message:   "ADD CONSTRAINT sem NOT VALID escaneia a tabela inteira. Use ADD CONSTRAINT ... NOT VALID; VALIDATE CONSTRAINT ... em seguida.",
					Line:      line,
				})
			}
		}
	}
	return results
}

// addColumnWithDefaultResult retorna o LintResult para P1, ajustado pela versão do PG.
// Em PG11+, ADD COLUMN com DEFAULT não reescreve a tabela (fast column add),
// mas ainda adquire ACCESS EXCLUSIVE brevemente para atualizar o catálogo.
func addColumnWithDefaultResult(raw string, line int, pgVersion int) *LintResult {
	switch {
	case pgVersion == 0:
		// Versão não especificada: diagnóstico conservador.
		return &LintResult{
			Statement: raw,
			LockMode:  LockAccessExclusive,
			Risk:      RiskCritical,
			Pattern:   PatternAddColumnWithDefault,
			Message:   "ADD COLUMN com DEFAULT reescreve a tabela inteira (PG10 e anteriores). Se estiver no PG11+, use --pg-version 11 para diagnóstico preciso.",
			Line:      line,
		}
	case pgVersion < 11:
		return &LintResult{
			Statement: raw,
			LockMode:  LockAccessExclusive,
			Risk:      RiskCritical,
			Pattern:   PatternAddColumnWithDefault,
			Message:   fmt.Sprintf("ADD COLUMN com DEFAULT reescreve a tabela inteira no PG%d. Adicione a coluna sem default, depois faça UPDATE em batches.", pgVersion),
			Line:      line,
		}
	default:
		// PG11+: sem reescrita de tabela, mas lock de catálogo ainda existe.
		return &LintResult{
			Statement: raw,
			LockMode:  LockAccessExclusive,
			Risk:      RiskWarn,
			Pattern:   PatternAddColumnWithDefault,
			Message:   fmt.Sprintf("Em PG%d, ADD COLUMN com DEFAULT não reescreve a tabela, mas ainda adquire ACCESS EXCLUSIVE brevemente para atualizar o catálogo.", pgVersion),
			Line:      line,
		}
	}
}

func analyzeCreateIndex(index *pg_query.IndexStmt, raw string, line int) *LintResult {
	if !index.Concurrent {
		return &LintResult{
			Statement: raw,
			LockMode:  LockShare,
			Risk:      RiskCritical,
			Pattern:   PatternCreateIndexNoConcurrently,
			Message:   "CREATE INDEX sem CONCURRENTLY bloqueia escritas durante o build. Use CREATE INDEX CONCURRENTLY.",
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
