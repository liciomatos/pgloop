package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/liciomatos/pgloop/internal/lockmapper"
	"github.com/liciomatos/pgloop/internal/output"
	"github.com/liciomatos/pgloop/internal/parser"
	"github.com/liciomatos/pgloop/internal/rewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagFormat      string
	flagIgnore      []string
	flagFailOn      string
	flagSuggestions bool
	flagPGVersion   int
)

var lintCmd = &cobra.Command{
	Use:   "lint <file.sql|directory> [...]",
	Short: "Analyze SQL migrations and detect dangerous lock patterns",
	Long: `Statically analyzes one or more SQL migrations and maps each DDL to its exact lock mode.

Accepts individual files, multiple files, or a directory (reads all .sql files alphabetically).

  pgloop lint migration.sql
  pgloop lint migrations/
  pgloop lint migrations/001.sql migrations/002.sql

No database connection required — analysis is performed via AST (same engine as PostgreSQL).
Each detected issue is assigned a code P1–P10. To see the full list:
  pgloop patterns

Exit codes:
  0  no issues
  1  warnings only
  2  at least one CRITICAL (or WARN with --fail-on WARN)`,
	Args: cobra.MinimumNArgs(1),
	RunE: runLint,
}

func init() {
	lintCmd.Flags().StringVar(&flagFormat, "format", "terminal", "output format: terminal, json, github")
	lintCmd.Flags().StringSliceVar(&flagIgnore, "ignore", nil, "suppress patterns by code (e.g. --ignore P2,P9) — see 'pgloop patterns'")
	lintCmd.Flags().StringVar(&flagFailOn, "fail-on", "CRITICAL", "minimum level for exit code 2: CRITICAL or WARN")
	lintCmd.Flags().BoolVar(&flagSuggestions, "suggestions", true, "show safe rewrite recipes in terminal output")
	lintCmd.Flags().IntVar(&flagPGVersion, "pg-version", 0, "target PostgreSQL major version (e.g. 14) — affects P1 diagnosis")

	viper.BindPFlag("lint.format", lintCmd.Flags().Lookup("format"))
	viper.BindPFlag("lint.ignore", lintCmd.Flags().Lookup("ignore"))
	viper.BindPFlag("lint.fail_on", lintCmd.Flags().Lookup("fail-on"))
	viper.BindPFlag("lint.suggestions", lintCmd.Flags().Lookup("suggestions"))
	viper.BindPFlag("lint.pg_version", lintCmd.Flags().Lookup("pg-version"))
}

func runLint(cmd *cobra.Command, args []string) error {
	files, err := resolveInputFiles(args)
	if err != nil {
		return err
	}

	opts := lockmapper.AnalyzeOptions{
		PGVersion: viper.GetInt("lint.pg_version"),
	}

	renderer, err := output.NewRenderer(viper.GetString("lint.format"), viper.GetBool("lint.suggestions"))
	if err != nil {
		return err
	}

	var allResults []lockmapper.LintResult
	for _, file := range files {
		results, err := lintFile(file, opts)
		if err != nil {
			return err
		}
		if err := renderer.Render(file, results); err != nil {
			return err
		}
		allResults = append(allResults, results...)
	}

	if len(files) > 1 {
		printMultiFileSummary(len(files), allResults)
	}

	if code := exitCode(allResults, viper.GetString("lint.fail_on")); code != 0 {
		return &ExitError{Code: code}
	}
	return nil
}

// lintFile analyzes a single SQL file and returns enriched results.
func lintFile(file string, opts lockmapper.AnalyzeOptions) ([]lockmapper.LintResult, error) {
	sql, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", file, err)
	}

	stmts, err := parser.ParseStatements(string(sql))
	if err != nil {
		return nil, fmt.Errorf("%s: SQL parse error: %w", file, err)
	}

	results := lockmapper.Analyze(stmts, string(sql), opts)
	results = applyIgnore(results, viper.GetStringSlice("lint.ignore"))
	results = enrichWithSuggestions(results)
	return results, nil
}

// resolveInputFiles expands args into a flat list of .sql files.
// Directories are expanded to their .sql files sorted alphabetically.
func resolveInputFiles(args []string) ([]string, error) {
	var files []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("not found: %s", arg)
		}
		if info.IsDir() {
			dirFiles, err := sqlFilesInDir(arg)
			if err != nil {
				return nil, err
			}
			files = append(files, dirFiles...)
		} else {
			files = append(files, arg)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .sql files found in the provided arguments")
	}
	return files, nil
}

// sqlFilesInDir returns all .sql files in a directory, sorted alphabetically.
// Non-recursive — only the immediate directory level.
func sqlFilesInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %w", dir, err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

// printMultiFileSummary prints an aggregate summary after analyzing multiple files.
// Only prints for terminal format — JSON and github are self-contained per file.
func printMultiFileSummary(fileCount int, results []lockmapper.LintResult) {
	format := strings.ToLower(viper.GetString("lint.format"))
	if format != "terminal" && format != "" {
		return
	}

	critical, warn := countByRisk(results)
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Printf("Summary: %d file(s) analyzed", fileCount)
	if len(results) == 0 {
		fmt.Println("  ✓ no issues found")
		return
	}
	fmt.Printf("  |  Total: %d issue(s)", len(results))
	if critical > 0 {
		fmt.Printf("  %d CRITICAL", critical)
	}
	if warn > 0 {
		fmt.Printf("  %d WARN", warn)
	}
	fmt.Println()
}

func countByRisk(results []lockmapper.LintResult) (critical, warn int) {
	for _, result := range results {
		switch result.Risk {
		case lockmapper.RiskCritical:
			critical++
		case lockmapper.RiskWarn:
			warn++
		}
	}
	return
}

// enrichWithSuggestions populates the Suggestion field of each result via the rewriter.
func enrichWithSuggestions(results []lockmapper.LintResult) []lockmapper.LintResult {
	for i := range results {
		results[i].Suggestion = rewriter.Suggestion(results[i].Pattern)
	}
	return results
}

func applyIgnore(results []lockmapper.LintResult, ignore []string) []lockmapper.LintResult {
	if len(ignore) == 0 {
		return results
	}
	ignoreSet := make(map[string]bool)
	for _, pattern := range ignore {
		ignoreSet[strings.ToUpper(strings.TrimSpace(pattern))] = true
	}

	filtered := results[:0]
	for _, result := range results {
		patternKey := fmt.Sprintf("P%d", result.Pattern)
		if !ignoreSet[patternKey] {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func exitCode(results []lockmapper.LintResult, failOn string) int {
	hasCritical := false
	hasWarn := false
	for _, result := range results {
		switch result.Risk {
		case lockmapper.RiskCritical:
			hasCritical = true
		case lockmapper.RiskWarn:
			hasWarn = true
		}
	}

	if hasCritical {
		return 2
	}
	if hasWarn && strings.ToUpper(failOn) == "WARN" {
		return 2
	}
	if hasWarn {
		return 1
	}
	return 0
}
