#!/usr/bin/env bash
# =============================================================================
# qmd-setup.sh — Install QMD and configure local knowledge base collections
#
# Run once after cloning bot_memory to set up the picoclaw memory integration.
#
# Usage:
#   bash scripts/qmd-setup.sh [--bm25-only]
#
# --bm25-only  Skip embedding step (no ML models needed — good for low-memory
#              machines).  You can always run `qmd embed` later.
# =============================================================================

set -euo pipefail

BM25_ONLY="${1:-}"
PICOCLAW_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
QMD_DIR="$PICOCLAW_ROOT/bot_memory"

echo "=== QMD Setup for picoclaw ==="
echo "Workspace root: $PICOCLAW_ROOT"
echo ""

# ---------------------------------------------------------------------------
# 1. Check / install QMD
# ---------------------------------------------------------------------------
if command -v qmd &>/dev/null; then
    echo "✓ qmd is already installed: $(qmd --version 2>/dev/null || echo 'version unknown')"
else
    echo "Installing QMD globally..."
    if command -v bun &>/dev/null; then
        bun install -g "$QMD_DIR"
        echo "✓ Installed via bun (local bot_memory)"
    elif command -v npm &>/dev/null; then
        npm install -g "$QMD_DIR"
        echo "✓ Installed via npm (local bot_memory)"
    else
        echo "ERROR: Neither bun nor npm found. Install one first:"
        echo "  curl -fsSL https://bun.sh/install | bash"
        exit 1
    fi
fi

# ---------------------------------------------------------------------------
# 2. Check memory available (inform the user before models load)
# ---------------------------------------------------------------------------
echo ""
echo "=== Memory Check ==="
free -h || true
AVAIL_MB=$(awk '/^MemAvailable/ {print int($2/1024)}' /proc/meminfo 2>/dev/null || echo 0)
if [ "$AVAIL_MB" -lt 2000 ]; then
    echo ""
    echo "⚠  WARNING: Only ${AVAIL_MB}MB free RAM detected."
    echo "   QMD's ML models need ~2GB combined."
    echo "   Recommending BM25-only mode (fast, no models, still very useful)."
    BM25_ONLY="--bm25-only"
fi

# ---------------------------------------------------------------------------
# 3. Set up collections
# ---------------------------------------------------------------------------
echo ""
echo "=== Setting Up Collections ==="

WORKSPACE_DIR="${PICOCLAW_AGENT_WORKSPACE:-$HOME/.picoclaw/workspace}"
MEMORY_DIR="$WORKSPACE_DIR/memory"

# picoclaw memory (agent notes and decisions)
if [ -d "$MEMORY_DIR" ]; then
    echo "Adding picoclaw memory collection..."
    qmd-run collection add "$MEMORY_DIR" --name picoclaw 2>/dev/null || \
        echo "  (collection already exists)"
    qmd-run context add qmd://picoclaw "picoclaw agent memory, long-term notes, and daily decisions" 2>/dev/null || true
else
    echo "  Skipping picoclaw memory (dir not found: $MEMORY_DIR)"
fi

# picoclaw workspace docs (Markdown + config files, excluding binary/build)
echo "Adding workspace docs collection..."
qmd-run collection add "$PICOCLAW_ROOT" --name workspace \
    --glob '**/*.{md,go,json,yaml,yml,toml,txt}' 2>/dev/null || \
    echo "  (collection already exists)"
qmd-run context add qmd://workspace "picoclaw source code, configuration, and documentation" 2>/dev/null || true

# Kanban / Telegram-bots logs
KANBAN_DIR="$PICOCLAW_ROOT/telegram-bots/logs"
if [ -d "$KANBAN_DIR" ]; then
    echo "Adding kanban task log collection..."
    qmd-run collection add "$KANBAN_DIR" --name kanban 2>/dev/null || \
        echo "  (collection already exists)"
    qmd-run context add qmd://kanban "Telegram kanban bot task history and status updates" 2>/dev/null || true
fi

# IDE monitor output (if present)
IDE_MON_DIR="$PICOCLAW_ROOT/ide-monitor"
if [ -d "$IDE_MON_DIR" ]; then
    echo "Adding ide-monitor collection..."
    qmd-run collection add "$IDE_MON_DIR" --name ide-monitor \
        --glob '**/*.{md,txt,log,json}' 2>/dev/null || \
        echo "  (collection already exists)"
    qmd-run context add qmd://ide-monitor "IDE activity event logs and parsed coding telemetry" 2>/dev/null || true
fi

echo ""
qmd-run status || true

# ---------------------------------------------------------------------------
# 4. Embed (optional)
# ---------------------------------------------------------------------------
if [ "$BM25_ONLY" = "--bm25-only" ]; then
    echo ""
    echo "Skipping embedding (BM25-only mode)."
    echo "BM25 keyword search is still fast and useful."
    echo "Run 'qmd embed' later when you have ~2GB of free RAM to enable hybrid search."
else
    echo ""
    echo "=== Generating Embeddings ==="
    echo "(This downloads ~300MB embedding model on first run and indexes documents)"
    echo "Memory before:"
    free -h || true
    qmd-run embed
    echo ""
    echo "Memory after:"
    free -h || true
fi

# ---------------------------------------------------------------------------
# 5. Enable in picoclaw config
# ---------------------------------------------------------------------------
echo ""
echo "=== Enable in picoclaw config ==="
echo "Add or update this section in config.json:"
cat <<'EOF'
  "tools": {
    "qmd": {
      "enabled": true,
      "mcp_endpoint": "http://localhost:8181/mcp",
      "mode": "auto"
    }
  }
EOF

echo ""
echo "Or set environment variables:"
echo "  export PICOCLAW_TOOLS_QMD_ENABLED=true"
echo "  export PICOCLAW_TOOLS_QMD_MODE=auto"
echo ""
echo "✓ QMD setup complete!"
echo ""
echo "Next steps:"
echo "  1. Enable in config.json (see above)"
echo "  2. Restart picoclaw"
echo "  3. Optionally start the daemon for hybrid search: bash scripts/qmd-daemon.sh"
