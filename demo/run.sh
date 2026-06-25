#!/usr/bin/env bash
set -euo pipefail

PGLOOP="${PGLOOP_BIN:-$(dirname "$0")/../pgloop}"

if [[ ! -x "$PGLOOP" ]]; then
    echo "pgloop não encontrado. Rode 'make build' primeiro."
    exit 1
fi

DEMO_DIR="$(dirname "$0")"

separator() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

# ── Demo 1: migration perigosa ──────────────────────────────────────────────
separator
echo "  Demo 1 — Migration com problemas (como seria escrita normalmente)"
echo ""
echo "  \$ cat demo/dangerous.sql"
echo ""
cat "$DEMO_DIR/dangerous.sql"
separator
echo "  \$ pgloop lint demo/dangerous.sql"
echo ""
"$PGLOOP" lint "$DEMO_DIR/dangerous.sql" || true

# ── Demo 2: migration segura ────────────────────────────────────────────────
separator
echo "  Demo 2 — A mesma migration reescrita de forma segura"
echo ""
echo "  \$ pgloop lint demo/safe.sql"
echo ""
"$PGLOOP" lint "$DEMO_DIR/safe.sql" || true

# ── Demo 3: saída JSON ──────────────────────────────────────────────────────
separator
echo "  Demo 3 — Saída JSON (para CI/pipelines)"
echo ""
echo "  \$ pgloop lint demo/dangerous.sql --format json"
echo ""
"$PGLOOP" lint "$DEMO_DIR/dangerous.sql" --format json || true

# ── Demo 4: GitHub Annotations ──────────────────────────────────────────────
separator
echo "  Demo 4 — GitHub Annotations (para uso em Actions)"
echo ""
echo "  \$ pgloop lint demo/dangerous.sql --format github"
echo ""
"$PGLOOP" lint "$DEMO_DIR/dangerous.sql" --format github || true

# ── Demo 5: ignorar um padrão ───────────────────────────────────────────────
separator
echo "  Demo 5 — Ignorando P9 (timeout warning)"
echo ""
echo "  \$ pgloop lint demo/dangerous.sql --ignore P9 --suggestions=false"
echo ""
"$PGLOOP" lint "$DEMO_DIR/dangerous.sql" --ignore P9 --suggestions=false || true

separator
echo "  Pronto. Rode 'pgloop lint --help' para ver todas as opções."
echo ""
