# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

`pgloop` is a Go CLI tool that performs static analysis on PostgreSQL migration SQL files to detect dangerous lock patterns — no database connection required. Currently implements the `lint` subcommand; future subcommands (`apply`, `explain`, `seed`) are planned (see README roadmap).

## Commands

```bash
make build      # go build com -ldflags "-X main.version=$(VERSION)"
make test       # go test ./... -v
make bench      # benchmark lockmapper (target: <200ms para 100 statements)
make lint       # golangci-lint run ./...
make install    # go install com ldflags
make demo       # build + run demo/run.sh
make release    # goreleaser release --clean (requer tag git)
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
  → lockmapper.Analyze(stmts, sql, opts)  # detecta padrões, retorna []LintResult
  → cmd: applyIgnore()                    # filtra por --ignore
  → cmd: enrichWithSuggestions()          # popula LintResult.Suggestion via rewriter
  → output.NewRenderer().Render()         # formata: terminal / JSON / GitHub Annotations
```

**`internal/parser`** — wrapper sobre `pg_query_go/v6`. Retorna `[]Statement{Raw, Node, Position}`. `Raw` é SQL canônico (deparsado); `Position` é byte offset para cálculo de linha.

**`internal/lockmapper`** — motor de análise. `Analyze(stmts, sql, AnalyzeOptions)` despacha para `analyzeStatement` → `analyzeAlterTable` (retorna `[]LintResult`, pois um ALTER TABLE pode ter múltiplos comandos perigosos). Appenda P9 e P10 como sintéticos ao final.

**`internal/rewriter`** — mapeia `PatternID → string` com receita de reescrita segura. Chamado exclusivamente em `cmd/lint.go` via `enrichWithSuggestions()`.

**`internal/output`** — três renderers unexported (`terminalRenderer`, `jsonRenderer`, `gitHubRenderer`) atrás da interface `Renderer`. Criados via `output.NewRenderer(format, showSuggestions)`. `riskToLevel()` em `level.go` — nunca duplicar por renderer.

**`cmd/`** — comandos Cobra. `lint.go` orquestra o pipeline. `patterns.go` lista todos os padrões. Exit codes via `ExitError` retornado de `RunE`; `root.go` chama `os.Exit` — em nenhum outro lugar. Viper carrega `.pgloop.yaml` da raiz do projeto ou `~/.config/pgloop/`.

## Code Conventions

### Naming
- Campos de struct: palavras completas — `Message` não `Msg`, `Statement` não `Stmt`
- Variáveis de loop: `result` não `r`, `stmt` não `s`, `pattern` não `p` ou `i`
- Funções de verificação prefixadas pelo sujeito: `columnHasDefault`, `hasTimeoutSet`

### LintResult
Tipo central. Campos críticos:
- `Synthetic bool` — `true` para P9/P10 (diagnóstico de arquivo). O renderer usa `!result.Synthetic` para decidir se exibe o `Statement`. **Nunca usar strings sentinela para isso.**
- `Suggestion string` — populado em `cmd/lint.go` após `Analyze()`, nunca dentro de `lockmapper` ou `output`.
- `Message string` — descrição do problema, pode variar por `PGVersion` (ex: P1).

### AnalyzeOptions
Passar `AnalyzeOptions` para `Analyze()` — nunca adicionar parâmetros avulsos. Novas opções de análise entram nessa struct.

### analyzeAlterTable retorna []LintResult
Um `ALTER TABLE` pode ter múltiplos comandos (ADD COLUMN, DROP COLUMN, etc.) na mesma instrução. `analyzeAlterTable` deve retornar **todos** os problemas encontrados, não apenas o primeiro.

### Output layer
`output` não deve importar `rewriter`. As sugestões chegam pré-populadas em `LintResult.Suggestion`. `riskToLevel()` (em `level.go`) e `countByLevel()` (em `terminal.go`) não devem ser duplicadas entre renderers.

### Exit codes
`os.Exit` é chamado apenas em `cmd/root.go`'s `Execute()`. `RunE` retorna `*ExitError{Code: N}`; `Execute()` detecta com `errors.As`.

### Adicionar um novo output format
Criar struct unexported implementando `Renderer`, adicionar `case` em `NewRenderer()` em `renderer.go`. Nunca adicionar `switch` de formato em outro lugar.

## Patterns (P1–P10)

`PatternID` constants em `lockmapper/mapper.go`. `AllPatterns()` é a fonte de verdade para metadata (código, nome, lock, risco, nota de versão) — usada por `pgloop patterns`.

Adicionar novo padrão:
1. Constante `PatternID`
2. Entrada em `AllPatterns()` com `VersionNote` se comportamento varia por PG version
3. Detecção em `analyzeStatement` ou `analyzeAlterTable`
4. Receita em `rewriter/rewriter.go`
5. Fixture em `testdata/migrations/NN_name.sql` + caso em `mapper_test.go`

**P1 (ADD COLUMN com DEFAULT)** é version-aware: CRITICAL em PG≤10, WARN em PG≥11. A lógica está em `addColumnWithDefaultResult(raw, line, pgVersion)`. Se adicionar padrões version-aware, seguir o mesmo padrão de função separada.

P9 e P10 são sintéticos (`Synthetic: true`) — sem statement associado.

## PG Version
`AnalyzeOptions.PGVersion = 0` significa "não especificada" → comportamento conservador (assume PG10). Flags da CLI (`--pg-version`) e config (`.pgloop.yaml: lint.pg_version`) sobrescrevem. Testes que validam comportamento version-aware usam `AnalyzeOptions{PGVersion: N}` explicitamente.

## Test Fixtures

`testdata/migrations/` tem dois tipos:
- `NN_name.sql` — migrations perigosas, cada uma dispara um padrão específico
- `safe_*.sql` — migrations seguras, verificadas para zero issues CRITICAL (guard de falso positivo)

Testes em `internal/lockmapper/mapper_test.go` (table-driven, file-driven) e `internal/parser/parser_test.go` (edge cases: vazio, sintaxe inválida, multi-statement, posições).

## Config (.pgloop.yaml)

Carregado automaticamente se presente na raiz do projeto ou em `~/.config/pgloop/`. Suporta apenas configurações de lint (não profiles de conexão — esses são v0.2+). Exemplo em `config/pgloop.yaml`. Flags da CLI sempre têm precedência sobre o arquivo.

## Dependencies

- `pg_query_go/v6` — cgo binding to libpg_query; requer gcc
- `charmbracelet/lipgloss` — terminal styling; cores ANSI nomeadas em `terminal.go` (`colorRed`, `colorYellow`, etc.) — nunca usar strings numéricas diretamente
- `spf13/cobra` + `spf13/viper` — CLI + config
