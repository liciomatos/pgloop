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
	Use:   "lint <arquivo.sql|diretório> [...]",
	Short: "Analisa migrations SQL e detecta padrões perigosos de lock",
	Long: `Analisa estaticamente uma ou mais migrations SQL e mapeia cada DDL ao seu lock mode exato.

Aceita arquivos individuais, múltiplos arquivos ou um diretório (lê todos os .sql em ordem alfabética).

  pgloop lint migration.sql
  pgloop lint migrations/
  pgloop lint migrations/001.sql migrations/002.sql

Não conecta ao banco — a análise é feita via AST (mesma engine do PostgreSQL).
Cada problema detectado recebe um código P1–P10. Para ver a lista completa:
  pgloop patterns

Exit codes:
  0  sem problemas
  1  apenas WARNings
  2  pelo menos um CRITICAL (ou WARN com --fail-on WARN)`,
	Args: cobra.MinimumNArgs(1),
	RunE: runLint,
}

func init() {
	lintCmd.Flags().StringVar(&flagFormat, "format", "terminal", "formato de saída: terminal, json, github")
	lintCmd.Flags().StringSliceVar(&flagIgnore, "ignore", nil, "padrões a ignorar por código (ex: --ignore P2,P9) — veja 'pgloop patterns'")
	lintCmd.Flags().StringVar(&flagFailOn, "fail-on", "CRITICAL", "nível mínimo para exit code 2: CRITICAL ou WARN")
	lintCmd.Flags().BoolVar(&flagSuggestions, "suggestions", true, "exibe sugestões de reescrita no terminal")
	lintCmd.Flags().IntVar(&flagPGVersion, "pg-version", 0, "versão major do PostgreSQL alvo (ex: 14) — afeta diagnóstico do P1")

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

// lintFile analisa um único arquivo SQL e retorna os resultados enriquecidos.
func lintFile(file string, opts lockmapper.AnalyzeOptions) ([]lockmapper.LintResult, error) {
	sql, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler %s: %w", file, err)
	}

	stmts, err := parser.ParseStatements(string(sql))
	if err != nil {
		return nil, fmt.Errorf("%s: erro ao parsear SQL: %w", file, err)
	}

	results := lockmapper.Analyze(stmts, string(sql), opts)
	results = applyIgnore(results, viper.GetStringSlice("lint.ignore"))
	results = enrichWithSuggestions(results)
	return results, nil
}

// resolveInputFiles expande args em lista de arquivos .sql.
// Diretórios são expandidos para seus arquivos .sql em ordem alfabética.
func resolveInputFiles(args []string) ([]string, error) {
	var files []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("não encontrado: %s", arg)
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
		return nil, fmt.Errorf("nenhum arquivo .sql encontrado nos argumentos fornecidos")
	}
	return files, nil
}

// sqlFilesInDir retorna todos os arquivos .sql de um diretório, em ordem alfabética.
// Não é recursivo — apenas o nível imediato do diretório.
func sqlFilesInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler diretório %s: %w", dir, err)
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

// printMultiFileSummary exibe o resumo consolidado após análise de múltiplos arquivos.
// Só imprime para o formato terminal — JSON e github já são auto-suficientes por arquivo.
func printMultiFileSummary(fileCount int, results []lockmapper.LintResult) {
	format := strings.ToLower(viper.GetString("lint.format"))
	if format != "terminal" && format != "" {
		return
	}

	critical, warn := countByRisk(results)
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Printf("Resumo: %d arquivo(s) analisado(s)", fileCount)
	if len(results) == 0 {
		fmt.Println("  ✓ nenhum problema encontrado")
		return
	}
	fmt.Printf("  |  Total: %d problema(s)", len(results))
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

// enrichWithSuggestions popula o campo Suggestion de cada resultado via rewriter.
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
