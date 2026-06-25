package lockmapper_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/liciomatos/pgloop/internal/lockmapper"
	"github.com/liciomatos/pgloop/internal/parser"
)

var defaultOpts = lockmapper.AnalyzeOptions{}

// patternCase verifies that a migration triggers a specific pattern.
type patternCase struct {
	file          string
	opts          lockmapper.AnalyzeOptions
	wantPattern   lockmapper.PatternID
	wantRisk      lockmapper.RiskLevel
	wantMinIssues int
}

var patternCases = []patternCase{
	// Version-independent patterns
	{
		file: "02_create_index_no_concurrently.sql", opts: defaultOpts,
		wantPattern: lockmapper.PatternCreateIndexNoConcurrently, wantRisk: lockmapper.RiskCritical, wantMinIssues: 1,
	},
	{
		file: "03_add_constraint_no_not_valid.sql", opts: defaultOpts,
		wantPattern: lockmapper.PatternAddConstraintNoNotValid, wantRisk: lockmapper.RiskCritical, wantMinIssues: 1,
	},
	{
		file: "04_drop_column.sql", opts: defaultOpts,
		wantPattern: lockmapper.PatternDropColumn, wantRisk: lockmapper.RiskCritical, wantMinIssues: 1,
	},
	{
		file: "05_set_not_null.sql", opts: defaultOpts,
		wantPattern: lockmapper.PatternSetNotNull, wantRisk: lockmapper.RiskCritical, wantMinIssues: 1,
	},
	{
		file: "06_rename_column.sql", opts: defaultOpts,
		wantPattern: lockmapper.PatternRenameTableColumn, wantRisk: lockmapper.RiskWarn, wantMinIssues: 1,
	},
	{
		file: "07_alter_column_type.sql", opts: defaultOpts,
		wantPattern: lockmapper.PatternAlterColumnType, wantRisk: lockmapper.RiskCritical, wantMinIssues: 1,
	},
	{
		file: "08_truncate.sql", opts: defaultOpts,
		wantPattern: lockmapper.PatternTruncate, wantRisk: lockmapper.RiskCritical, wantMinIssues: 1,
	},
	{
		file: "09_no_timeout.sql", opts: defaultOpts,
		wantPattern: lockmapper.PatternNoTimeout, wantRisk: lockmapper.RiskWarn, wantMinIssues: 1,
	},
	{
		file: "10_multiple_exclusive.sql", opts: defaultOpts,
		wantPattern: lockmapper.PatternMultipleExclusive, wantRisk: lockmapper.RiskWarn, wantMinIssues: 1,
	},
	// P1 — ADD COLUMN with DEFAULT: behavior changes by PG version
	{
		file: "01_add_column_with_default.sql", opts: lockmapper.AnalyzeOptions{PGVersion: 0},
		wantPattern: lockmapper.PatternAddColumnWithDefault, wantRisk: lockmapper.RiskCritical, wantMinIssues: 1,
	},
	{
		file: "01_add_column_with_default.sql", opts: lockmapper.AnalyzeOptions{PGVersion: 10},
		wantPattern: lockmapper.PatternAddColumnWithDefault, wantRisk: lockmapper.RiskCritical, wantMinIssues: 1,
	},
	{
		file: "01_add_column_with_default.sql", opts: lockmapper.AnalyzeOptions{PGVersion: 11},
		wantPattern: lockmapper.PatternAddColumnWithDefault, wantRisk: lockmapper.RiskWarn, wantMinIssues: 1,
	},
	{
		file: "01_add_column_with_default.sql", opts: lockmapper.AnalyzeOptions{PGVersion: 16},
		wantPattern: lockmapper.PatternAddColumnWithDefault, wantRisk: lockmapper.RiskWarn, wantMinIssues: 1,
	},
}

func TestPatterns(t *testing.T) {
	testdataDir := filepath.Join("..", "..", "testdata", "migrations")

	for _, tc := range patternCases {
		name := tc.file
		if tc.opts.PGVersion != 0 {
			name += "_pg" + itoa(tc.opts.PGVersion)
		}
		t.Run(name, func(t *testing.T) {
			sql := readFile(t, filepath.Join(testdataDir, tc.file))
			stmts, err := parser.ParseStatements(sql)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			results := lockmapper.Analyze(stmts, sql, tc.opts)
			if len(results) < tc.wantMinIssues {
				t.Errorf("expected at least %d issue(s), got %d", tc.wantMinIssues, len(results))
			}

			found := false
			for _, r := range results {
				if r.Pattern == tc.wantPattern && r.Risk == tc.wantRisk {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("pattern %d with risk %s not found in results: %+v", tc.wantPattern, tc.wantRisk, results)
			}
		})
	}
}

var safeCases = []string{
	"safe_add_column.sql",
	"safe_create_index_concurrently.sql",
	"safe_add_constraint_not_valid.sql",
}

func TestSafeMigrationsNoFalsePositives(t *testing.T) {
	testdataDir := filepath.Join("..", "..", "testdata", "migrations")

	for _, file := range safeCases {
		t.Run(file, func(t *testing.T) {
			sql := readFile(t, filepath.Join(testdataDir, file))
			stmts, err := parser.ParseStatements(sql)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			results := lockmapper.Analyze(stmts, sql, defaultOpts)
			for _, r := range results {
				if r.Risk == lockmapper.RiskCritical {
					t.Errorf("false positive CRITICAL (P%d) in %s: %s", r.Pattern, file, r.Message)
				}
			}
		})
	}
}

// TestAddColumnWithDefaultPG11IsWarnNotCritical explicitly validates the P1 fix.
func TestAddColumnWithDefaultPG11IsWarnNotCritical(t *testing.T) {
	sql := "ALTER TABLE users ADD COLUMN status TEXT DEFAULT 'active';"
	stmts, _ := parser.ParseStatements(sql)

	pg11Results := lockmapper.Analyze(stmts, sql, lockmapper.AnalyzeOptions{PGVersion: 11})
	pg10Results := lockmapper.Analyze(stmts, sql, lockmapper.AnalyzeOptions{PGVersion: 10})

	for _, r := range pg11Results {
		if r.Pattern == lockmapper.PatternAddColumnWithDefault && r.Risk == lockmapper.RiskCritical {
			t.Error("PG11: ADD COLUMN with DEFAULT must be WARN, not CRITICAL")
		}
	}
	for _, r := range pg10Results {
		if r.Pattern == lockmapper.PatternAddColumnWithDefault && r.Risk == lockmapper.RiskWarn {
			t.Error("PG10: ADD COLUMN with DEFAULT must be CRITICAL, not WARN")
		}
	}
}

func TestEmptySQL(t *testing.T) {
	stmts, err := parser.ParseStatements("")
	if err != nil {
		t.Fatalf("empty SQL must not produce a parse error: %v", err)
	}
	results := lockmapper.Analyze(stmts, "", defaultOpts)
	if len(results) != 0 {
		t.Errorf("empty SQL must produce no results, got %d", len(results))
	}
}

func TestMultiCmdAlterTable(t *testing.T) {
	// A single ALTER TABLE statement with two dangerous commands.
	sql := `SET lock_timeout = '3s';
ALTER TABLE orders
    ADD COLUMN status TEXT DEFAULT 'pending',
    DROP COLUMN legacy_field;`

	stmts, err := parser.ParseStatements(sql)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	results := lockmapper.Analyze(stmts, sql, lockmapper.AnalyzeOptions{PGVersion: 10})
	patterns := make(map[lockmapper.PatternID]bool)
	for _, r := range results {
		patterns[r.Pattern] = true
	}

	if !patterns[lockmapper.PatternAddColumnWithDefault] {
		t.Error("expected P1 (ADD COLUMN with DEFAULT) in multi-cmd ALTER TABLE")
	}
	if !patterns[lockmapper.PatternDropColumn] {
		t.Error("expected P4 (DROP COLUMN) in multi-cmd ALTER TABLE")
	}
}

func BenchmarkAnalyze100Statements(b *testing.B) {
	sql := ""
	for i := 0; i < 100; i++ {
		sql += "CREATE INDEX CONCURRENTLY idx_bench_" + itoa(i) + " ON t(col);\n"
	}
	stmts, _ := parser.ParseStatements(sql)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lockmapper.Analyze(stmts, sql, defaultOpts)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func itoa(n int) string {
	return fmt.Sprint(n)
}
