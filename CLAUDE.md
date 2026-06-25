# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

`pgloop` is a Go CLI tool that performs static analysis on PostgreSQL migration SQL files to detect dangerous lock patterns — no database connection required. Currently implements the `lint` subcommand; future subcommands (`apply`, `explain`, `seed`) are planned (see README roadmap).

## Commands

```bash
make build      # go build with -ldflags "-X main.version=$(VERSION)"
make test       # go test ./... -v
make bench      # benchmark lockmapper (target: <200ms for 100 statements)
make lint       # golangci-lint run ./...
make install    # go install with ldflags
make demo       # build + run demo/run.sh
make release    # goreleaser release --clean (requires a git tag)
```

Run a single test:
```bash
go test ./internal/lockmapper/... -run TestPatterns/01_add_column_with_default.sql_pg11 -v
```

The binary requires `gcc` to build (cgo dependency from `pg_query_go`).

## Architecture

The pipeline for `pgloop lint <file>` is linear:

```
SQL file
  → parser.ParseStatements()              # AST via pg_query_go (cgo)
  → lockmapper.Analyze(stmts, sql, opts)  # detects patterns, returns []LintResult
  → cmd: applyIgnore()                    # filters by --ignore
  → cmd: enrichWithSuggestions()          # populates LintResult.Suggestion via rewriter
  → output.NewRenderer().Render()         # formats: terminal / JSON / GitHub Annotations
```

**`internal/parser`** — wrapper around `pg_query_go/v6`. Returns `[]Statement{Raw, Node, Position}`. `Raw` is canonical SQL (deparsed); `Position` is a byte offset for line number calculation.

**`internal/lockmapper`** — analysis engine. `Analyze(stmts, sql, AnalyzeOptions)` dispatches to `analyzeStatement` → `analyzeAlterTable` (returns `[]LintResult`, since one ALTER TABLE can contain multiple dangerous commands). P9 and P10 are appended as synthetic results at the end.

**`internal/rewriter`** — maps `PatternID → string` with a safe rewrite recipe. Called exclusively from `cmd/lint.go` via `enrichWithSuggestions()`.

**`internal/output`** — three unexported renderers (`terminalRenderer`, `jsonRenderer`, `gitHubRenderer`) behind the `Renderer` interface. Created via `output.NewRenderer(format, showSuggestions)`. `riskToLevel()` lives in `level.go` — never duplicate per renderer.

**`cmd/`** — Cobra commands. `lint.go` orchestrates the pipeline. `patterns.go` lists all patterns. Exit codes via `ExitError` returned from `RunE`; `root.go` calls `os.Exit` — nowhere else. Viper loads `.pgloop.yaml` from the project root or `~/.config/pgloop/`.

## Code Conventions

### Naming
- Struct fields: full words — `Message` not `Msg`, `Statement` not `Stmt`
- Loop variables: `result` not `r`, `stmt` not `s`, `pattern` not `p` or `i`
- Predicate functions prefixed by the subject: `columnHasDefault`, `hasTimeoutSet`

### LintResult
Central type. Critical fields:
- `Synthetic bool` — `true` for P9/P10 (file-level diagnostic). The renderer uses `!result.Synthetic` to decide whether to display the `Statement`. **Never use sentinel strings for this.**
- `Suggestion string` — populated in `cmd/lint.go` after `Analyze()`, never inside `lockmapper` or `output`.
- `Message string` — issue description, may vary by `PGVersion` (e.g. P1).

### AnalyzeOptions
Pass `AnalyzeOptions` to `Analyze()` — never add loose parameters. New analysis options go into this struct.

### analyzeAlterTable returns []LintResult
A single `ALTER TABLE` can contain multiple commands (ADD COLUMN, DROP COLUMN, etc.). `analyzeAlterTable` must return **all** issues found, not just the first one.

### Output layer
`output` must not import `rewriter`. Suggestions arrive pre-populated in `LintResult.Suggestion`. `riskToLevel()` (in `level.go`) and `countByLevel()` (in `terminal.go`) must not be duplicated across renderers.

### Exit codes
`os.Exit` is called only in `cmd/root.go`'s `Execute()`. `RunE` returns `*ExitError{Code: N}`; `Execute()` detects it with `errors.As`.

### Adding a new output format
Create an unexported struct implementing `Renderer`, add a `case` in `NewRenderer()` in `renderer.go`. Never add a format `switch` anywhere else.

## Patterns (P1–P10)

`PatternID` constants in `lockmapper/mapper.go`. `AllPatterns()` is the source of truth for metadata (code, name, lock, risk, version note) — used by `pgloop patterns`.

To add a new pattern:
1. Add a `PatternID` constant
2. Add an entry to `AllPatterns()` with a `VersionNote` if behavior varies by PG version
3. Add detection logic in `analyzeStatement` or `analyzeAlterTable`
4. Add a recipe in `rewriter/rewriter.go`
5. Add a fixture in `testdata/migrations/NN_name.sql` and a case in `mapper_test.go`

**P1 (ADD COLUMN with DEFAULT)** is version-aware: CRITICAL on PG≤10, WARN on PG≥11. The logic lives in `addColumnWithDefaultResult(raw, line, pgVersion)`. Follow the same pattern for new version-aware rules.

P9 and P10 are synthetic (`Synthetic: true`) — no statement is associated with them.

## PG Version
`AnalyzeOptions.PGVersion = 0` means "unspecified" → conservative behavior (assumes PG10). CLI flags (`--pg-version`) and config (`.pgloop.yaml: lint.pg_version`) override this. Tests that validate version-aware behavior use `AnalyzeOptions{PGVersion: N}` explicitly.

## Test Fixtures

`testdata/migrations/` has two types:
- `NN_name.sql` — dangerous migrations, each triggering one specific pattern
- `safe_*.sql` — safe migrations, verified to produce zero CRITICAL issues (false-positive guard)

Tests in `internal/lockmapper/mapper_test.go` (table-driven, file-driven) and `internal/parser/parser_test.go` (edge cases: empty, invalid syntax, multi-statement, positions).

## Config (.pgloop.yaml)

Loaded automatically if present in the project root or at `~/.config/pgloop/`. Supports only lint settings (no connection profiles — those are v0.2+). Example in `config/pgloop.yaml`. CLI flags always take precedence over the file.

## Dependencies

- `pg_query_go/v6` — cgo binding to libpg_query; requires gcc
- `charmbracelet/lipgloss` — terminal styling; named ANSI color constants in `terminal.go` (`colorRed`, `colorYellow`, etc.) — never use numeric strings directly
- `spf13/cobra` + `spf13/viper` — CLI + config
