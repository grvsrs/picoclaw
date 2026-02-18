#!/usr/bin/env bash
# =============================================================================
# qmd-daemon.sh — Start the QMD HTTP MCP daemon for picoclaw
#
# The daemon pre-loads the ~2GB of ML models once and keeps them warm so
# agents can run hybrid searches (BM25 + vector + reranking) without the
# 20-30s cold-start penalty on every request.
#
# The daemon listens on http://localhost:8181/mcp
#
# Usage:
#   bash scripts/qmd-daemon.sh         # Start in background, log to /tmp/qmd.log
#   bash scripts/qmd-daemon.sh --fg    # Start in foreground (for debugging)
#   bash scripts/qmd-daemon.sh --stop  # Stop the background daemon
# =============================================================================

set -euo pipefail

PORT="${QMD_PORT:-8181}"
LOGFILE="${QMD_LOG:-/tmp/qmd-daemon.log}"
PIDFILE="/tmp/qmd-daemon.pid"

case "${1:-}" in

# ---------------------------------------------------------------------------
# Stop
# ---------------------------------------------------------------------------
--stop)
    if [ -f "$PIDFILE" ]; then
        PID=$(cat "$PIDFILE")
        if kill -0 "$PID" 2>/dev/null; then
            echo "Stopping QMD daemon (PID $PID)..."
            kill "$PID"
            rm -f "$PIDFILE"
            echo "✓ Stopped"
        else
            echo "QMD daemon not running (stale pidfile removed)"
            rm -f "$PIDFILE"
        fi
    else
        echo "QMD daemon not running (no pidfile found)"
    fi
    ;;

# ---------------------------------------------------------------------------
# Foreground
# ---------------------------------------------------------------------------
--fg)
    echo "Starting QMD HTTP daemon on port $PORT (foreground)..."
    echo "Ctrl+C to stop."
    qmd mcp --http --port "$PORT"
    ;;

# ---------------------------------------------------------------------------
# Background (default)
# ---------------------------------------------------------------------------
*)
    # Check if already running
    if [ -f "$PIDFILE" ]; then
        PID=$(cat "$PIDFILE")
        if kill -0 "$PID" 2>/dev/null; then
            echo "QMD daemon already running on port $PORT (PID $PID)"
            echo "Health: $(curl -sf http://localhost:$PORT/health || echo 'not reachable')"
            exit 0
        fi
        rm -f "$PIDFILE"
    fi

    # Memory check — warn but don't block
    AVAIL_MB=$(awk '/^MemAvailable/ {print int($2/1024)}' /proc/meminfo 2>/dev/null || echo 9999)
    if [ "$AVAIL_MB" -lt 2000 ]; then
        echo "⚠  WARNING: Only ${AVAIL_MB}MB free RAM. QMD models need ~2GB."
        echo "   Daemon will start but may cause memory pressure."
        echo "   Consider using BM25-only mode (skip daemon) on this machine."
        echo ""
    fi

    echo "Starting QMD HTTP daemon on port $PORT..."
    echo "Log: $LOGFILE"
    echo "Models will load on first request (~20-30s cold start)."

    nohup qmd mcp --http --port "$PORT" >> "$LOGFILE" 2>&1 &
    DAEMON_PID=$!
    echo "$DAEMON_PID" > "$PIDFILE"

    # Wait for daemon to become ready (up to 60s)
    echo -n "Waiting for daemon to be ready"
    for i in $(seq 1 60); do
        if curl -sf "http://localhost:$PORT/health" >/dev/null 2>&1; then
            echo ""
            echo "✓ QMD daemon is ready (PID $DAEMON_PID)"
            echo "  Health: $(curl -sf http://localhost:$PORT/health)"
            echo "  MCP endpoint: http://localhost:$PORT/mcp"
            echo "  Log: tail -f $LOGFILE"
            echo ""
            echo "To stop: bash scripts/qmd-daemon.sh --stop"
            exit 0
        fi
        echo -n "."
        sleep 1
    done

    echo ""
    echo "⚠  Daemon did not become healthy within 60s."
    echo "   Check log: tail -20 $LOGFILE"
    echo "   The daemon may still be loading models — try again in a moment."
    ;;
esac
