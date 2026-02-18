# Execution Feedback Loop

This document describes the ExecutionEvent system that closes the loop between terminal/VS Code execution and Kanban state updates.

## Overview

**Before:** Commands ran → results shown → Kanban unchangedafter:** Commands ran → ExecutionEvents emitted → Kanban updated → Telegram notifies

## Architecture

```
Terminal / VS Code
    ↓ (picoclaw-run wrapper)
ExecutionEvent (started/completed/failed)
    ↓
Kanban Store (state transitions)
    ↓
Telegram Bot (notifications)
```

## ExecutionEvent Schema

Events are append-only facts stored in the `execution_events` table:

```sql
CREATE TABLE execution_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT,                    -- KAN-xxx or NULL for ad-hoc
    source TEXT NOT NULL,            -- executor | terminal | vscode | bot
    event_type TEXT NOT NULL,        -- started | progress | completed | failed
    summary TEXT NOT NULL,           -- short human-readable line
    details TEXT,                    -- optional extended info
    exit_code INTEGER,               -- for completed/failed
    artifact_path TEXT,              -- path to log file
    created_at TEXT NOT NULL
);
```

## State Transition Rules

| Event Type | Kanban Reaction |
|-----------|----------------|
| `started` | card moves to **RUNNING** |
| `completed` | card moves to **REVIEW** |
| `failed` | card moves to **BLOCKED** |
| `progress` | no state change (info only) |

## Using picoclaw-run

The `bin/picoclaw-run` wrapper emits ExecutionEvents for any shell command.

### Basic Usage

```bash
# With task ID (links to Kanban card)
picoclaw-run KAN-001 pytest

# Ad-hoc (no Kanban linkage)
picoclaw-run --adhoc ls -la
```

### What it does

1. **Emit started event** → Kanban moves card to RUNNING
2. **Run command** (capture output to temp log)
3. **Emit terminal event:**
   - `completed` (exit code 0) → Kanban moves to REVIEW
   - `failed` (exit code != 0) → Kanban moves to BLOCKED
4. **Exit with same code** (preserves CI/script behavior)

### Example: Run tests for KAN-001

```bash
$ picoclaw-run KAN-001 pytest tests/
[picoclaw-run] Starting: pytest tests/
[picoclaw-run] Task ID: KAN-001
[picoclaw-run] Output logged to: /tmp/tmpxxx.log
======================== test session starts =========================
...
======================== 16 passed in 0.42s ==========================

[picoclaw-run] ✅ Completed successfully
```

**Result:**
- Card moved from INBOX → RUNNING → REVIEW
- 2 ExecutionEvents created (started, completed)
- State history updated with timestamps
- Telegram bot can notify on completion

### Example: Failed command

```bash
$ picoclaw-run KAN-002 ./deploy.sh production
[picoclaw-run] Starting: ./deploy.sh production
[picoclaw-run] Task ID: KAN-002
Error: production requires approval
[picoclaw-run] ❌ Failed with exit code 1
$ echo $?
1
```

**Result:**
- Card moved from INBOX → RUNNING → BLOCKED
- `failed` event with exit_code=1
- Log saved to artifact_path

## VS Code Integration

Add to `.vscode/tasks.json`:

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Run Tests (Kanban KAN-001)",
      "type": "shell",
      "command": "${workspaceFolder}/telegram-bots/bin/picoclaw-run",
      "args": ["KAN-001", "pytest", "tests/"],
      "problemMatcher": [],
      "group": {
        "kind": "test",
        "isDefault": true
      }
    }
  ]
}
```

**Usage:** Run task → Kanban updates → Telegram notifies

No extension required.

## Notes Table

The `notes` table supports quick capture without creating tasks:

```sql
CREATE TABLE notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,           -- telegram | vscode | cli
    telegram_message_id TEXT,
    telegram_user_id TEXT,
    content TEXT NOT NULL,
    tags TEXT,                      -- comma-separated
    linked_task_id TEXT,            -- nullable KAN-xxx
    created_at TEXT NOT NULL
);
```

### Telegram Bot Commands (Future)

```
/note Fix flaky test in auth module
→ Creates note (not a task)

/idea Add retry logic to auth tests
→ Creates note + Kanban card

/notes today
→ Shows notes from last 24h

/notes search auth
→ Full-text search

/promote 42
→ Convert note ID 42 to Kanban card
```

## API: Emit Events Programmatically

```python
from pkg.kanban.store import KanbanStore
from pkg.kanban.events import emit_event

store = KanbanStore("/path/to/kanban.db")

# Emit started event
emit_event(
    store,
    task_id="KAN-001",
    source="terminal",
    event_type="started",
    summary="Running pytest",
)

# Emit completed event
emit_event(
    store,
    task_id="KAN-001",
    source="terminal",
    event_type="completed",
    summary="Tests passed",
    exit_code=0,
    artifact_path="/tmp/test.log",
)
```

**Side effect:** Kanban card state updated automatically.

## Query Events

```python
from pkg.kanban.events import get_recent_events, get_task_events

# Last 50 events (all tasks)
events = get_recent_events(store, limit=50)

# All events for KAN-001
task_events = get_task_events(store, "KAN-001")
```

## Telegram Notifications (Next Step)

Create a notifier bot that:

1. Watches `execution_events` table
2. Filters by:
   - mode (personal / remote)
   - event_type (only terminal events)
   - task ownership
3. Sends Telegram notification:
   - ✅ Task KAN-001 completed
   - ❌ Task KAN-002 failed (exit code 1)
   - [View log](artifact_path)

**Design rule:** Notifier bot is read-only, never writes to Kanban.

## Testing

```bash
# Create a test task
curl -X POST localhost:3000/api/task \
  -H "Content-Type: application/json" \
  -d '{"title": "Test execution loop"}'

# Run command with picoclaw-run
./bin/picoclaw-run KAN-003 echo "test"

# Verify events
sqlite3 /home/g/picoclaw/kanban.db \
  "SELECT * FROM execution_events WHERE task_id='KAN-003'"

# Verify state transition
sqlite3 /home/g/picoclaw/kanban.db \
  "SELECT card_id, state FROM kanban_cards WHERE card_id='KAN-003'"
```

## What's Next

1. **Notifier Bot** — watches events, sends Telegram alerts
2. **VS Code Extension** — task picker + quick commands
3. **Notes Bot** — `/note`, `/idea`, `/search` commands
4. **Event Log UI** — timeline view in Kanban web interface

---

**Status:** ✅ All contracts implemented and tested (2026-02-16)
