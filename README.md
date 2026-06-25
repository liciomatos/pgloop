# pgloop

Local-first CLI for the daily developer loop with PostgreSQL. Detects dangerous lock patterns in SQL migrations before they cause problems in production — no database connection, no external dependencies.

```
$ pgloop lint migration.sql

✖  CRITICAL
   →  line 3
   CREATE INDEX idx_orders_user_id ON orders(user_id)
   Lock:  SHARE
   CREATE INDEX without CONCURRENTLY blocks writes during the build. Use CREATE INDEX CONCURRENTLY.

   Suggestion:
   CREATE INDEX CONCURRENTLY idx_name ON t(col);
   -- Note: cannot run inside an explicit transaction (BEGIN/COMMIT)

⚠  WARN
   Lock:  NONE
   Migration has no lock_timeout or statement_timeout.

Total: 2 issue(s)  1 CRITICAL  1 WARN
```

---

## Installation

```bash
# Build from source (requires Go 1.22+ and gcc)
git clone https://github.com/liciomatos/pgloop
cd pgloop
make install
```

---

## Usage

### Basic check

```bash
pgloop lint migration.sql
```

### Directory or multiple files

```bash
pgloop lint migrations/               # all .sql files, alphabetical order
pgloop lint 001.sql 002.sql 003.sql   # explicit list
pgloop lint migrations/ hotfix.sql    # mix of directory and file
```

### CI integration (JSON output)

```bash
pgloop lint migration.sql --format json
```

```json
{
  "file": "migration.sql",
  "total_issues": 2,
  "critical": 1,
  "warn": 1,
  "issues": [
    {
      "file": "migration.sql",
      "line": 3,
      "level": "error",
      "lock_mode": "SHARE",
      "pattern": 2,
      "message": "CREATE INDEX without CONCURRENTLY blocks writes during the build...",
      "suggestion": "CREATE INDEX CONCURRENTLY idx_name ON t(col);"
    }
  ]
}
```

### GitHub Actions (inline PR annotations)

```bash
pgloop lint migration.sql --format github
# ::error file=migration.sql,line=3::[pgloop P2] CREATE INDEX without CONCURRENTLY...
```

### Available flags

| Flag | Default | Description |
|---|---|---|
| `--format` | `terminal` | Output format: `terminal`, `json`, `github` |
| `--fail-on` | `CRITICAL` | Minimum level for exit code 2: `CRITICAL` or `WARN` |
| `--ignore` | — | Suppress patterns by ID: `--ignore P2,P9` |
| `--suggestions` | `true` | Show safe rewrite recipes in terminal output |
| `--pg-version` | `0` | Target PostgreSQL major version (e.g. `14`) — affects P1 diagnosis |

### Exit codes

| Code | Meaning |
|---|---|
| `0` | No issues found |
| `1` | Warnings only |
| `2` | At least one CRITICAL (or WARN if `--fail-on WARN`) |

---

## Detected Patterns

| ID | SQL Pattern | Lock Acquired | Risk |
|---|---|---|---|
| P1 | `ADD COLUMN ... DEFAULT value` | ACCESS EXCLUSIVE | CRITICAL |
| P2 | `CREATE INDEX` without `CONCURRENTLY` | SHARE | CRITICAL |
| P3 | `ADD CONSTRAINT` without `NOT VALID` | ACCESS EXCLUSIVE | CRITICAL |
| P4 | `DROP COLUMN` | ACCESS EXCLUSIVE | CRITICAL |
| P5 | `SET NOT NULL` without prior check constraint | ACCESS EXCLUSIVE | CRITICAL |
| P6 | `RENAME TABLE` / `RENAME COLUMN` | ACCESS EXCLUSIVE | WARN |
| P7 | `ALTER COLUMN TYPE` | ACCESS EXCLUSIVE | CRITICAL |
| P8 | `TRUNCATE` | ACCESS EXCLUSIVE | CRITICAL |
| P9 | Migration without `lock_timeout` / `statement_timeout` | — | WARN |
| P10 | Multiple `ACCESS EXCLUSIVE` operations in the same migration | — | WARN |

For each detected issue, pgloop shows the lock acquired and a safe rewrite recipe.

Run `pgloop patterns` to see the full table directly in the terminal.

---

## Safe Rewrite Examples

### P1 — ADD COLUMN with DEFAULT

```sql
-- Dangerous
ALTER TABLE users ADD COLUMN status TEXT DEFAULT 'active';

-- Safe
ALTER TABLE users ADD COLUMN status TEXT;
UPDATE users SET status = 'active' WHERE id BETWEEN 1 AND 10000; -- in batches
ALTER TABLE users ALTER COLUMN status SET DEFAULT 'active';
```

### P2 — CREATE INDEX without CONCURRENTLY

```sql
-- Dangerous (blocks writes during the build)
CREATE INDEX idx_orders_user ON orders(user_id);

-- Safe (cannot run inside BEGIN/COMMIT)
CREATE INDEX CONCURRENTLY idx_orders_user ON orders(user_id);
```

### P3 — ADD CONSTRAINT without NOT VALID

```sql
-- Dangerous (scans the entire table with a lock)
ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id);

-- Safe (two separate deploys)
ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;
-- next deploy:
ALTER TABLE orders VALIDATE CONSTRAINT fk_user;
```

### P5 — SET NOT NULL without check constraint

```sql
-- Dangerous
ALTER TABLE users ALTER COLUMN email SET NOT NULL;

-- Safe
ALTER TABLE users ADD CONSTRAINT chk_email_nn CHECK (email IS NOT NULL) NOT VALID;
ALTER TABLE users VALIDATE CONSTRAINT chk_email_nn; -- separate deploy, no full-scan with lock
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users DROP CONSTRAINT chk_email_nn;
```

---

## Configuration

pgloop reads a config file automatically — no flag required. It searches in two locations, in order:

1. `.pgloop.yaml` in the **current working directory** (project-level config — commit this to your repo)
2. `~/.config/pgloop/.pgloop.yaml` (user-level config — applies to all projects on your machine)

CLI flags always take precedence over the config file, so you can override any setting per-run without editing the file.

### Full annotated config

```yaml
# ──────────────────────────────────────────────────────────────────────────────
# pgloop.yaml — project or user-level configuration
# Place at the root of your project (.pgloop.yaml) or at ~/.config/pgloop/.pgloop.yaml
# CLI flags always override these values.
# ──────────────────────────────────────────────────────────────────────────────


# ── lint ──────────────────────────────────────────────────────────────────────
# Settings for `pgloop lint`. All of these map 1:1 to CLI flags.
lint:
  # Target PostgreSQL major version.
  # This affects how version-sensitive patterns are classified. Currently impacts P1:
  #   PG10 or earlier → ADD COLUMN with DEFAULT is CRITICAL (causes a full table rewrite)
  #   PG11+           → ADD COLUMN with DEFAULT is WARN    (fast column add, no rewrite,
  #                                                          but still acquires ACCESS EXCLUSIVE briefly)
  # Set this to the major version you actually deploy against.
  # 0 = unspecified: conservative mode — assumes PG10 (safest default).
  # Equivalent CLI flag: --pg-version
  pg_version: 14

  # Minimum risk level that causes `pgloop lint` to exit with code 2 (build failure).
  # CRITICAL  → exit 2 only when at least one CRITICAL issue is found (default)
  # WARN      → exit 2 for any issue, including warnings
  # In both cases, exit code 1 means only warnings were found; 0 means clean.
  # Equivalent CLI flag: --fail-on
  fail_on: CRITICAL

  # Whether to print safe rewrite recipes below each issue in terminal output.
  # Recipes show the step-by-step SQL to fix the pattern without causing downtime.
  # Set to false for compact output (e.g. in log pipelines or noisy CI environments).
  # Note: JSON and github formats always include suggestions regardless of this setting.
  # Equivalent CLI flag: --suggestions
  suggestions: true

  # Patterns to suppress globally across all migrations in this project.
  # Accepts a list of pattern codes (P1–P10). Run `pgloop patterns` to see all codes.
  # Use this when a pattern is intentionally accepted project-wide — for example:
  #   [P9]      → your deploy tool (Flyway, Liquibase, etc.) already injects lock_timeout
  #   [P6]      → your team has a documented RENAME strategy and accepts the warning
  #   [P9, P10] → suppress both timeout and multi-exclusive warnings
  # To suppress a pattern for a single run only, prefer the CLI flag: --ignore P9
  # Default: [] (nothing suppressed)
  ignore: []


# ── profiles (coming in v0.2) ─────────────────────────────────────────────────
# Connection profiles for commands that require a live database (e.g. `pgloop apply`,
# `pgloop explain`, `pgloop seed`). Not used by `pgloop lint`, which is fully static.
#
# default_profile: dev
#
# profiles:
#   dev:
#     host: localhost
#     port: 5432
#     database: myapp_dev
#     user: postgres
#     # password: "" — prefer PGPASSWORD env var or .pgpass file
#     ssl_mode: disable   # disable | require | verify-ca | verify-full
#
#   staging:
#     host: staging-db.internal
#     port: 5432
#     database: myapp
#     user: myapp_readonly
#     ssl_mode: require
#
#   prod:
#     host: prod-db.internal
#     port: 5432
#     database: myapp
#     user: myapp_readonly
#     ssl_mode: verify-full
#     read_only: true     # blocks accidental writes when using pgloop against prod


# ── apply (coming in v0.2) ────────────────────────────────────────────────────
# Settings for `pgloop apply` — runs a migration safely against a live database,
# injecting timeouts automatically and rolling back on any lock acquisition failure.
#
# apply:
#   lock_timeout: 3s        # abort if a lock cannot be acquired within this duration
#   statement_timeout: 30s  # abort if any single statement takes longer than this
#   dry_run: false          # when true, prints what would run without executing it
#   retries: 3              # number of retry attempts on lock timeout before giving up
#   retry_delay: 5s         # wait between retries
```

### Common setups

**PG14 project, fail on any issue:**
```yaml
lint:
  pg_version: 14
  fail_on: WARN
```

**Suppress P9 — deploy tool already injects timeouts:**
```yaml
lint:
  pg_version: 15
  ignore: [P9]
```

**Compact CI output, no rewrite recipes:**
```yaml
lint:
  suggestions: false
  fail_on: CRITICAL
```

**Full team config — multiple environments:**
```yaml
lint:
  pg_version: 16
  fail_on: CRITICAL
  suggestions: true
  ignore: [P9]       # Flyway injects lock_timeout via beforeMigrate callback

# profiles:         # uncomment when pgloop apply ships (v0.2)
#   dev:
#     host: localhost
#     port: 5432
#     database: myapp_dev
#     user: postgres
#   prod:
#     host: prod-db.internal
#     port: 5432
#     database: myapp
#     user: myapp_readonly
#     ssl_mode: verify-full
#     read_only: true
```

### Parameter reference

#### `lint`

| Key | Type | Default | CLI flag | Description |
|---|---|---|---|---|
| `lint.pg_version` | integer | `0` | `--pg-version` | Target PostgreSQL major version. `0` = unspecified (assumes PG10). |
| `lint.fail_on` | string | `CRITICAL` | `--fail-on` | Exit code 2 threshold: `CRITICAL` or `WARN`. |
| `lint.suggestions` | bool | `true` | `--suggestions` | Show rewrite recipes in terminal output. |
| `lint.ignore` | list | `[]` | `--ignore` | Pattern codes to suppress globally (e.g. `[P9, P10]`). |

#### `profiles` _(v0.2)_

| Key | Type | Description |
|---|---|---|
| `default_profile` | string | Profile used when no `--profile` flag is given. |
| `profiles.<name>.host` | string | Database host. |
| `profiles.<name>.port` | integer | Database port (default: `5432`). |
| `profiles.<name>.database` | string | Database name. |
| `profiles.<name>.user` | string | Database user. |
| `profiles.<name>.ssl_mode` | string | `disable`, `require`, `verify-ca`, or `verify-full`. |
| `profiles.<name>.read_only` | bool | Block accidental writes when connecting to this profile. |

#### `apply` _(v0.2)_

| Key | Type | Default | Description |
|---|---|---|---|
| `apply.lock_timeout` | duration | `3s` | Abort if a lock cannot be acquired within this duration. |
| `apply.statement_timeout` | duration | `30s` | Abort if any single statement exceeds this duration. |
| `apply.dry_run` | bool | `false` | Print what would run without executing it. |
| `apply.retries` | integer | `3` | Retry attempts on lock timeout before giving up. |
| `apply.retry_delay` | duration | `5s` | Wait between retries. |

---

## GitHub Actions

```yaml
# .github/workflows/db-lint.yml
name: pgloop lint

on:
  pull_request:
    paths:
      - 'migrations/**'

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go install github.com/liciomatos/pgloop@latest
      - run: pgloop lint migrations/ --format github
```

---

## Roadmap

| Version | Feature |
|---|---|
| **v0.1** (current) | `pgloop lint` — static analysis of 10 patterns |
| v0.2 | `pgloop lint --trace` — runs in an ephemeral PostgreSQL container (Docker), captures real locks |
| v0.3 | `pgloop explain` — EXPLAIN ANALYZE rendered in the terminal, no external services |
| v0.4 | `pgloop seed` — schema-aware seed respecting foreign keys and constraints |
| v1.0 | `pgloop apply`, MCP server, Homebrew tap |
