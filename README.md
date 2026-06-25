# pgloop

CLI local-first para o loop diário do desenvolvedor com PostgreSQL. Detecta padrões perigosos de lock em migrations SQL antes que causem problemas em produção — sem conectar ao banco, sem dependências externas.

```
$ pgloop lint migration.sql

✖  CRITICAL
   →  linha 3
   CREATE INDEX idx_orders_user_id ON orders(user_id)
   Lock:  SHARE
   CREATE INDEX sem CONCURRENTLY bloqueia escritas durante o build. Use CREATE INDEX CONCURRENTLY.

   Sugestão:
   CREATE INDEX CONCURRENTLY idx_name ON t(col);

⚠  WARN
   Lock:  NONE
   Migration sem lock_timeout ou statement_timeout.

Total: 2 problema(s)  1 CRITICAL  1 WARN
```

---

## Instalação

```bash
# Build a partir do código-fonte (requer Go 1.22+ e gcc)
git clone https://github.com/liciomatos/pgloop
cd pgloop
make install
```

---

## Uso

### Verificação básica

```bash
pgloop lint migration.sql
```

### Integração com CI (saída JSON)

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
      "message": "CREATE INDEX sem CONCURRENTLY bloqueia escritas...",
      "suggestion": "CREATE INDEX CONCURRENTLY idx_name ON t(col);"
    }
  ]
}
```

### GitHub Actions (annotations inline no PR)

```bash
pgloop lint migration.sql --format github
# ::error file=migration.sql,line=3::[pgloop P2] CREATE INDEX sem CONCURRENTLY...
```

### Flags disponíveis

| Flag | Padrão | Descrição |
|---|---|---|
| `--format` | `terminal` | Formato de saída: `terminal`, `json`, `github` |
| `--fail-on` | `CRITICAL` | Nível mínimo para exit code 2: `CRITICAL` ou `WARN` |
| `--ignore` | — | Suprime padrões por ID: `--ignore P2,P9` |
| `--suggestions` | `true` | Exibe receitas de reescrita segura no terminal |

### Exit codes

| Código | Significado |
|---|---|
| `0` | Nenhum problema encontrado |
| `1` | Apenas WARNings |
| `2` | Pelo menos um CRITICAL (ou WARN se `--fail-on WARN`) |

---

## Padrões Detectados

| ID | Padrão SQL | Lock Adquirido | Risco |
|---|---|---|---|
| P1 | `ADD COLUMN ... DEFAULT valor` | ACCESS EXCLUSIVE | CRITICAL |
| P2 | `CREATE INDEX` sem `CONCURRENTLY` | SHARE | CRITICAL |
| P3 | `ADD CONSTRAINT` sem `NOT VALID` | ACCESS EXCLUSIVE | CRITICAL |
| P4 | `DROP COLUMN` | ACCESS EXCLUSIVE | CRITICAL |
| P5 | `SET NOT NULL` sem check constraint prévia | ACCESS EXCLUSIVE | CRITICAL |
| P6 | `RENAME TABLE` / `RENAME COLUMN` | ACCESS EXCLUSIVE | WARN |
| P7 | `ALTER COLUMN TYPE` | ACCESS EXCLUSIVE | CRITICAL |
| P8 | `TRUNCATE` | ACCESS EXCLUSIVE | CRITICAL |
| P9 | Migration sem `lock_timeout` / `statement_timeout` | — | WARN |
| P10 | Múltiplas operações `ACCESS EXCLUSIVE` na mesma migration | — | WARN |

Para cada problema detectado, o pgloop exibe o lock adquirido e uma receita de reescrita segura.

---

## Exemplos de Reescrita Segura

### P1 — ADD COLUMN com DEFAULT

```sql
-- Perigoso
ALTER TABLE users ADD COLUMN status TEXT DEFAULT 'active';

-- Seguro
ALTER TABLE users ADD COLUMN status TEXT;
UPDATE users SET status = 'active' WHERE id BETWEEN 1 AND 10000; -- em batches
ALTER TABLE users ALTER COLUMN status SET DEFAULT 'active';
```

### P2 — CREATE INDEX sem CONCURRENTLY

```sql
-- Perigoso (bloqueia escritas durante o build)
CREATE INDEX idx_orders_user ON orders(user_id);

-- Seguro (não pode estar dentro de BEGIN/COMMIT)
CREATE INDEX CONCURRENTLY idx_orders_user ON orders(user_id);
```

### P3 — ADD CONSTRAINT sem NOT VALID

```sql
-- Perigoso (escaneia a tabela inteira com lock)
ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id);

-- Seguro (dois deploys separados)
ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;
-- deploy seguinte:
ALTER TABLE orders VALIDATE CONSTRAINT fk_user;
```

### P5 — SET NOT NULL sem check constraint

```sql
-- Perigoso
ALTER TABLE users ALTER COLUMN email SET NOT NULL;

-- Seguro
ALTER TABLE users ADD CONSTRAINT chk_email_nn CHECK (email IS NOT NULL) NOT VALID;
ALTER TABLE users VALIDATE CONSTRAINT chk_email_nn; -- deploy separado, sem full-scan com lock
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users DROP CONSTRAINT chk_email_nn;
```

---

## Configuração

Crie `.pgloop.yaml` na raiz do projeto ou em `~/.config/pgloop/pgloop.yaml`:

```yaml
default_profile: dev

profiles:
  dev:
    host: localhost
    port: 5432
    database: myapp_dev
    user: postgres
  staging:
    host: staging-db.internal
    port: 5432
    database: myapp
    user: myapp_readonly
    ssl_mode: require
  prod:
    host: prod-db.internal
    port: 5432
    database: myapp
    user: myapp_readonly
    ssl_mode: verify-full
    read_only: true      # bloqueia writes acidentais

lint:
  fail_on: [ACCESS_EXCLUSIVE]
  warn_on: [SHARE]
  ignore: []             # ex: [P9] para ignorar aviso de timeout globalmente

apply:
  lock_timeout: 3s
  statement_timeout: 30s
  dry_run: false
```

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

| Versão | Feature |
|---|---|
| **v0.1** (atual) | `pgloop lint` — análise estática dos 10 padrões |
| v0.2 | `pgloop lint --trace` — executa em container PostgreSQL efêmero (Docker), captura locks reais |
| v0.3 | `pgloop explain` — EXPLAIN ANALYZE renderizado no terminal, sem serviços externos |
| v0.4 | `pgloop seed` — seed schema-aware respeitando foreign keys e constraints |
| v1.0 | `pgloop apply`, MCP server, Homebrew tap |
