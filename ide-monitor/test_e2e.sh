#!/bin/bash
# ─────────────────────────────────────────────────────────────────
# IDE Monitor — End-to-End Test Script
#
# Simulates Antigravity + Copilot activity and verifies the
# ide-monitor detects, parses, and emits correct WorkflowEvents.
#
# Usage:
#   ./test_e2e.sh              # standalone test (no picoclaw needed)
#   ./test_e2e.sh --picoclaw   # also tests POST to picoclaw
# ─────────────────────────────────────────────────────────────────

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BRAIN_DIR="$HOME/.gemini/antigravity/brain"
TEST_GUID="test-$(date +%s)"
TEST_BRAIN="$BRAIN_DIR/$TEST_GUID"
JSONL_PATH="$HOME/.local/share/ide-monitor/events.jsonl"

MODE="standalone"
[[ "${1:-}" == "--picoclaw" ]] && MODE="picoclaw"

echo "═══════════════════════════════════════════════"
echo " IDE Monitor — End-to-End Test"
echo " Mode: $MODE"
echo " Test GUID: $TEST_GUID"
echo "═══════════════════════════════════════════════"
echo

# ── Setup ───────────────────────────────────────────

echo "[1/7] Creating test brain directory..."
mkdir -p "$TEST_BRAIN"

# Clean old JSONL
> "$JSONL_PATH" 2>/dev/null || true

echo "[2/7] Starting ide-monitor in background..."
cd "$SCRIPT_DIR"

if [[ "$MODE" == "picoclaw" ]]; then
    python3 watcher.py --workspace "$HOME" --picoclaw http://localhost:8080 &
else
    python3 watcher.py --workspace "$HOME" --picoclaw off &
fi
MONITOR_PID=$!
sleep 2

# Verify monitor is running
if ! kill -0 $MONITOR_PID 2>/dev/null; then
    echo "  [FAIL] Monitor failed to start"
    exit 1
fi
echo "  Monitor PID: $MONITOR_PID"
echo

# ── Simulate Antigravity Activity ───────────────────

echo "[3/7] Simulating: task.created (task.md)..."
cat > "$TEST_BRAIN/task.md" << 'EOF'
# Refactor authentication middleware

- [ ] Extract token validation into separate module
- [ ] Add refresh token rotation
- [ ] Write integration tests
EOF
sleep 2

echo "[4/7] Simulating: task.plan_ready (implementation_plan.md)..."
cat > "$TEST_BRAIN/implementation_plan.md" << 'EOF'
# Implementation Plan

1. Create pkg/auth/validator.go with TokenValidator interface
2. Move validation logic from middleware.go
3. Implement refresh token rotation in token.go
4. Add tests in auth_test.go
EOF
sleep 2

echo "[5/7] Simulating: task.completed (walkthrough.md)..."
cat > "$TEST_BRAIN/walkthrough.md" << 'EOF'
# Walkthrough

Successfully refactored authentication middleware:
- Extracted TokenValidator interface
- Moved validation to dedicated module
- Added refresh token rotation
- 4 new integration tests passing
EOF
sleep 2

# ── Verify ──────────────────────────────────────────

echo "[6/7] Checking captured events..."
echo

if [[ -f "$JSONL_PATH" ]]; then
    EVENT_COUNT=$(wc -l < "$JSONL_PATH")
    echo "  Events captured: $EVENT_COUNT"
    echo

    # Check for expected event types
    for EXPECTED in "antigravity.task.created" "antigravity.task.plan_ready" "antigravity.task.completed"; do
        if grep -q "$EXPECTED" "$JSONL_PATH"; then
            echo "  ✓ $EXPECTED"
        else
            echo "  ✗ $EXPECTED (NOT FOUND)"
        fi
    done

    echo
    echo "  Last 3 events:"
    tail -3 "$JSONL_PATH" | python3 -m json.tool 2>/dev/null || tail -3 "$JSONL_PATH"
else
    echo "  [WARN] No JSONL file found at $JSONL_PATH"
fi

# ── Test Picoclaw Integration ───────────────────────

if [[ "$MODE" == "picoclaw" ]]; then
    echo
    echo "[6b] Testing direct POST to Picoclaw..."
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/api/events \
        -H "Content-Type: application/json" \
        -d "{
            \"id\": \"test-direct-001\",
            \"spec_version\": \"1.0\",
            \"source\": \"antigravity\",
            \"event_type\": \"antigravity.task.completed\",
            \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
            \"task_id\": \"$TEST_GUID\",
            \"task_title\": \"E2E Test Task\",
            \"task_status\": \"complete\",
            \"summary\": \"End-to-end test completed successfully\"
        }")

    if [[ "$RESPONSE" == "202" ]]; then
        echo "  ✓ Picoclaw accepted event (HTTP 202)"
    else
        echo "  ✗ Picoclaw response: HTTP $RESPONSE"
    fi
fi

# ── Cleanup ─────────────────────────────────────────

echo
echo "[7/7] Cleaning up..."
kill $MONITOR_PID 2>/dev/null || true
wait $MONITOR_PID 2>/dev/null || true

# Remove test artifacts
rm -rf "$TEST_BRAIN"

echo
echo "═══════════════════════════════════════════════"
echo " Test complete."
echo "═══════════════════════════════════════════════"
