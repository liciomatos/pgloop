#!/usr/bin/env bash
set -euo pipefail

PGLOOP="${PGLOOP_BIN:-$(dirname "$0")/../pgloop}"

if [[ ! -x "$PGLOOP" ]]; then
    echo "pgloop not found. Run 'make build' first."
    exit 1
fi

DEMO_DIR="$(dirname "$0")"

separator() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

# ── Demo 1: dangerous migration ─────────────────────────────────────────────
separator
echo "  Demo 1 — Migration with issues (written the obvious way)"
echo ""
echo "  \$ cat demo/dangerous.sql"
echo ""
cat "$DEMO_DIR/dangerous.sql"
separator
echo "  \$ pgloop lint demo/dangerous.sql"
echo ""
"$PGLOOP" lint "$DEMO_DIR/dangerous.sql" || true

# ── Demo 2: safe migration ──────────────────────────────────────────────────
separator
echo "  Demo 2 — The same migration rewritten safely"
echo ""
echo "  \$ pgloop lint demo/safe.sql"
echo ""
"$PGLOOP" lint "$DEMO_DIR/safe.sql" || true

# ── Demo 3: JSON output ─────────────────────────────────────────────────────
separator
echo "  Demo 3 — JSON output (for CI/pipelines)"
echo ""
echo "  \$ pgloop lint demo/dangerous.sql --format json"
echo ""
"$PGLOOP" lint "$DEMO_DIR/dangerous.sql" --format json || true

# ── Demo 4: GitHub Annotations ──────────────────────────────────────────────
separator
echo "  Demo 4 — GitHub Annotations (for use in Actions)"
echo ""
echo "  \$ pgloop lint demo/dangerous.sql --format github"
echo ""
"$PGLOOP" lint "$DEMO_DIR/dangerous.sql" --format github || true

# ── Demo 5: ignore a pattern ────────────────────────────────────────────────
separator
echo "  Demo 5 — Suppressing P9 (timeout warning)"
echo ""
echo "  \$ pgloop lint demo/dangerous.sql --ignore P9 --suggestions=false"
echo ""
"$PGLOOP" lint "$DEMO_DIR/dangerous.sql" --ignore P9 --suggestions=false || true

separator
echo "  Done. Run 'pgloop lint --help' to see all options."
echo ""
