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

Create `.pgloop.yaml` in the project root or at `~/.config/pgloop/.pgloop.yaml`:

```yaml
lint:
  pg_version: 14        # target PostgreSQL major version (affects P1 diagnosis)
  fail_on: CRITICAL     # CRITICAL or WARN
  suggestions: true     # show safe rewrite recipes in terminal output
  ignore: []            # e.g. [P9] to suppress timeout warnings globally
```

CLI flags always take precedence over the config file.

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
