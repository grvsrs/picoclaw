# PicoClaw — Comprehensive Development Plan

> Created: 2026-02-18  
> Base: all 184 tests passing, clean build, 20,500+ lines Go

This document is the authoritative roadmap. It is organized by **priority phase**. Bugs from [BUGS_AND_ISSUES.md](BUGS_AND_ISSUES.md) are incorporated with their BUG-NNN references.

---

## Current State Summary

**What works:**
- Full gateway with agent loop, multi-channel messaging (Telegram, Discord, Slack, WhatsApp, Feishu, DingTalk, QQ, MaixCam)
- 10 built-in tools: exec, read/write/list/edit file, web search, web fetch, message, spawn, ops_monitor, cron
- Session persistence with automatic summarization
- Cron scheduling (cron expressions + interval)
- Skills system (workspace-local, global, GitHub install)
- REST API + WebSocket dashboard + embedded SPA
- Bot template system
- VS Code extension API endpoints
- Kanban task tracking (Go + Python, separate DBs — see BUG-005)
- ide-monitor daemon (Copilot/Antigravity/Git event tracking)
- Telegram bot swarm (dev, ops, monitor bots)
- OAuth2/PKCE auth flow
- OpenClaw → PicoClaw migration tool

**What is non-functional or incomplete:**
- Heartbeat callback (BUG-001) — fires but does nothing
- Filesystem tool path restriction (BUG-007) — unrestricted
- DI Container (BUG-004) — defined but never wired
- Two unsynchronized kanban databases (BUG-005)
- VSCode integration auto-start (BUG-021)
- InboxBot (BUG-019) — skeleton only
- Orchestrator category routing (BUG-011) — stub

---

## Phase 1 — Critical Fixes (Do First)

These are bugs that cause silent failures or security gaps. Fix before adding new features.

### P1.1 — Fix Filesystem Tool Path Restriction (BUG-007) 🔴 Security

**Time estimate:** 30 minutes  
**Files:** `cmd/picoclaw/main.go`, `pkg/agent/loop.go`, `pkg/tools/filesystem.go`

Call `tools.SetFSAllowedDir()` inside `NewAgentLoop`:
```go
// pkg/agent/loop.go: NewAgentLoop(), after setting up workspace
tools.SetFSAllowedDir(workspace)
```

This prevents the AI agent from reading `/etc/passwd`, `~/.picoclaw/auth.json`, SSH keys, etc.

---

### P1.2 — Fix Heartbeat Callback (BUG-001) 🟡

**Time estimate:** 20 minutes  
**Files:** `cmd/picoclaw/main.go:667-672`

Wire the callback in `gatewayCmd()`:
```go
heartbeatService := heartbeat.NewHeartbeatService(
    cfg.WorkspacePath(),
    func(prompt string) (string, error) {
        return agentLoop.ProcessDirectWithChannel(
            context.Background(), prompt,
            "heartbeat:system", "system", "heartbeat",
        )
    },
    30*60,
    true,
)
```

---

### P1.3 — Fix LLM Params Respecting Config (BUG-006) 🟡

**Time estimate:** 30 minutes  
**Files:** `pkg/agent/loop.go`

1. Add `temperature float64` field to `AgentLoop`
2. Read from `cfg.Agents.Defaults.Temperature` in `NewAgentLoop()`
3. Use `al.contextWindow` (already set from `cfg.Agents.Defaults.MaxTokens`) in the Chat call

---

### P1.4 — Fix Webhook Route Pattern (BUG-002) 🟡

**Time estimate:** 15 minutes  
**Files:** `pkg/api/server.go:133`, `pkg/api/webhooks.go:46-54`

Change route registration:
```go
mux.HandleFunc("/api/webhook/{source}", s.handleWebhook)
```
Remove the fallback manual trim from `handleWebhook`.

---

### P1.5 — Fix HeartbeatService Double-Stop Panic (BUG-013) 🟡

**Time estimate:** 20 minutes  
**Files:** `pkg/heartbeat/service.go`

```go
type HeartbeatService struct {
    ...
    closeOnce sync.Once
}

func (hs *HeartbeatService) Stop() {
    hs.closeOnce.Do(func() { close(hs.stopChan) })
}
```

---

### P1.6 — Fix Python KanbanStore Default Path (BUG-014) 🟡

**Time estimate:** 15 minutes  
**Files:** `telegram-bots/pkg/kanban/store.py`

```python
import os
DEFAULT_DB = os.path.expanduser("~/.local/share/picoclaw/kanban.db")

def __init__(self, db_path: str = DEFAULT_DB):
```

---

### P1.7 — Remove Dead Code (BUG-003, BUG-018) 🟢

**Time estimate:** 20 minutes  
**Files:** `cmd/picoclaw/main.go`

- Remove duplicate `func skillsCmd()` (lines 1301-1360)
- Remove second template creation loop in `onboard()` (lines 385-393)

---

## Phase 2 — Kanban Unification (BUG-005) 🔴

This is the most architecturally important fix. Two separate SQLite databases break the entire task tracking story.

### P2.1 — Decision: Choose One Canonical Database

**Option A (recommended):** Go API is the source of truth. Python uses `/api/tasks` REST.
- Python `KanbanStore` becomes a thin HTTP client to `http://localhost:18790/api/tasks`
- Go `pkg/integration/kanban/kanban.go` remains canonical, writing to `~/.picoclaw/kanban.db`
- Telegram bots call `POST /api/tasks` to create cards
- kanban_server.py proxies through Go API instead of reading SQLite directly

**Option B:** Python SQLite remains canonical. Go reads from it.
- Go uses `mattn/go-sqlite3` to open the Python SQLite directly
- Simpler migration (no HTTP changes in Python bots)
- Breaks if Python moves the DB or changes schema

### P2.2 — Implement Option A

**Time estimate:** 3-4 hours

1. Create `telegram-bots/pkg/kanban/api_client.py` — HTTP client for `/api/tasks`
2. Add API-compatible schema methods to match `KanbanStore` interface  
3. Update `bot_base.py` to use `APIKanbanClient` instead of `KanbanStore`
4. Update `kanban_server.py` to proxy all operations through Go API
5. Add config key `kanban_api_url` to `picoclaw.yaml`

---

## Phase 3 — Architecture Cleanup

### P3.1 — Wire DI Container or Delete It (BUG-004)

**Decision needed:** Is `pkg/app/container.go` the future or a mistake?  
**Recommendation:** Keep it and wire it.

**Time estimate:** 2-3 hours  
**Steps:**
1. Move `gatewayCmd()` wire-up to `app.NewContainer(cfg, msgBus, provider)`
2. Remove duplicate construction from `main.go`
3. `Container` returns: agentLoop, channelManager, cronService, heartbeatService, integrationRegistry, apiServer

Notes:
- Container construction should remain in `main.go` (not in app/) to keep `cmd/` as the composition root
- `pkg/app/` services should be thin wrappers that hold references, not constructors

---

### P3.2 — Wire VSCode Integration Auto-Start (BUG-021)

**Time estimate:** 30 minutes  
**Files:** `pkg/integration/vscode/vscode.go`

Add auto-registration:
```go
func init() {
    integration.Register(&VSCodeIntegration{})
}
```

Ensure it is imported in `main.go` (blank import):
```go
import _ "github.com/sipeed/picoclaw/pkg/integration/vscode"
```

---

### P3.3 — System Prompt Budget (BUG-020)

**Time estimate:** 2 hours  
**Files:** `pkg/agent/context.go`

Implement token budget in `BuildSystemPrompt()`:
```go
type ContextBudget struct {
    Identity    int // tokens
    Bootstrap   int
    Skills      int
    Memory      int
    Total       int
}

func DefaultBudget() ContextBudget {
    return ContextBudget{
        Identity:  500,
        Bootstrap: 2000,
        Skills:    1000,
        Memory:    2000,
        Total:     6000,
    }
}
```

Truncate each section to its budget before joining.

---

### P3.4 — Fix Domain Layer Panic (BUG-012)

**Time estimate:** 10 minutes  
**Files:** `pkg/domain/identity.go`

Replace `panic()` with error return:
```go
func NewEntityID() (EntityID, error) {
    id, err := uuid.NewRandom()
    if err != nil {
        return EntityID(""), fmt.Errorf("domain: failed to generate ID: %w", err)
    }
    return EntityID(id.String()), nil
}
```

---

## Phase 4 — Security Hardening

### P4.1 — Encrypt Credentials at Rest (BUG-008)

**Time estimate:** 3 hours

Option: Use OS keyring (`zalando/go-keyring`) for API keys and OAuth tokens.
```go
// pkg/auth/store.go
import "github.com/zalando/go-keyring"

func SetCredential(provider string, cred *AuthCredential) error {
    data, _ := json.Marshal(cred)
    return keyring.Set("picoclaw", provider, string(data))
}
```

Fallback: If keyring unavailable (headless), write encrypted JSON using AES-256-GCM with key derived from machine-unique data (hostname + user).

---

### P4.2 — Rate Limiting for Gateway API

**Time estimate:** 2 hours  
**Files:** `pkg/api/server.go`

Add rate limiting middleware per IP:
```go
// Use golang.org/x/time/rate
type rateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.Mutex
    r        rate.Limit  // requests per second
    b        int          // burst
}
```

Apply at API level: 10 req/s per IP, 100 burst.

---

## Phase 5 — Performance & Reliability

### P5.1 — Concurrent Message Processing (BUG-009)

**Time estimate:** 2 hours  
**Files:** `pkg/agent/loop.go`

Replace sequential processing with per-session goroutines:
```go
type AgentLoop struct {
    ...
    sessionLocks sync.Map  // sessionKey → chan struct{} (mutex per session)
    maxConcurrent int       // default 5
}

func (al *AgentLoop) Run(ctx context.Context) {
    sem := make(chan struct{}, al.maxConcurrent)
    for {
        msg := al.bus.ConsumeInbound(ctx)
        sem <- struct{}{}
        go func() {
            defer func() { <-sem }()
            al.processMessageSerialized(ctx, msg)  // each session still serialized
        }()
    }
}
```

Benefit: Messages from different sessions run concurrently; same session remains sequential.

---

### P5.2 — ide-monitor Retry Queue (BUG-016)

**Time estimate:** 1 hour  
**Files:** `ide-monitor/emitter.py`

```python
import threading
from collections import deque

_retry_queue = deque(maxlen=1000)
_retry_thread = None

def emit_with_retry(event: WorkflowEvent):
    payload = event.to_json()
    if not _try_post(payload):
        _retry_queue.append((payload, time.time()))
    _flush_retry_queue()
```

---

### P5.3 — Executor Path Configuration (BUG-015)

**Time estimate:** 1 hour  
**Files:** `telegram-bots/bots/executor.py`, `telegram-bots/config/picoclaw.yaml`

Add to `picoclaw.yaml`:
```yaml
executor:
  watch_dirs:
    - /workspace/tasks/dev
    - /workspace/tasks/ops
  projects_base: /projects
  scripts_base: /scripts
  task_timeout: 300
```

Read in executor:
```python
cfg = yaml.safe_load(open(CONFIG_PATH))
WATCH_DIRS = [Path(d) for d in cfg.get("executor", {}).get("watch_dirs", [])]
```

---

## Phase 6 — Feature Development

### P6.1 — Implement InboxBot (BUG-019)

**Time estimate:** 4 hours  
**Files:** `telegram-bots/bots/inbox_bot.py`

Commands:
- `/inbox` — list all Inbox state tasks
- `/task add <text>` — create new task in inbox
- `/task done <id>` — mark task done
- `/task plan <id>` — move to planned
- `/task list [state=all|inbox|planned|running|done]` — filtered list

---

### P6.2 — Orchestrator Category Routing (BUG-011)

**Time estimate:** 3-4 hours  
**Files:** `pkg/orchestration/orchestrator.go`

Implement `AssignByCapability(task)`:
1. Read task category from kanban
2. Filter agents by `AgentCapability.Categories`
3. Select agent with lowest active task count and highest priority
4. Return assignment or fallback to default agent

---

### P6.3 — Configurable Kanban Proxy URL

**Time estimate:** 30 minutes (BUG-010)

Add to config:
```json
{
  "integrations": {
    "kanban_server_url": "http://localhost:3000"
  }
}
```

Pass through to `Server` and use in `handleKanbanProxy()`.

---

### P6.4 — Build the "Sovereign Agent" Dashboard (picoclaw_ui_plan.md)

The file `picoclaw_ui_plan.md` contains a complete HTML prototype of the next-generation dashboard UI:
- Dark theme, "Sovereign Agent System" branding
- Sections: Overview, Agents, Pipelines, Logs, Chat
- Material Symbols icons, Tailwind CSS
- Kanban board integration

**Time estimate:** 8-12 hours  
**Steps:**
1. Extract HTML from `picoclaw_ui_plan.md`
2. Move to `web/dist/index.html` (replace current Vanilla JS dashboard)
3. Wire all existing API endpoints
4. Add WebSocket-driven real-time updates
5. Connect Kanban board to `/api/tasks`
6. Add agent chat section

---

### P6.5 — Rate Limiting Mode for Remote Bot (BUG-017)

**Time estimate:** 2 hours  
**Files:** `telegram-bots/pkg/kanban/mode.py`

Implement token bucket rate limiter:
```python
import time
from threading import Lock

class RateLimiter:
    def __init__(self, rate, burst):
        self.rate = rate        # tokens per second
        self.burst = burst      # max burst
        self.tokens = burst
        self.last = time.monotonic()
        self._lock = Lock()

    def allow(self) -> bool:
        with self._lock:
            now = time.monotonic()
            elapsed = now - self.last
            self.tokens = min(self.burst, self.tokens + elapsed * self.rate)
            self.last = now
            if self.tokens >= 1:
                self.tokens -= 1
                return True
            return False
```

---

## Phase 7 — Testing & Documentation

### P7.1 — Add Missing Go Tests

Missing coverage (no test files):
- `pkg/session/` — SessionManager CRUD
- `pkg/tools/` — exec deny-list, filesystem restriction, web tools
- `pkg/channels/` — base channel allow-list
- `pkg/cron/` — cron job scheduling
- `pkg/heartbeat/` — heartbeat loop
- `pkg/integration/kanban/` — SQLite kanban store

**Time estimate:** 8 hours to add comprehensive tests

### P7.2 — Integration Test: Gateway Smoke Test

Add `cmd/picoclaw/gateway_test.go`:
```go
func TestGatewaySmoke(t *testing.T) {
    // Start gateway with test config (mock LLM provider)
    // POST /api/agent/chat → expect non-empty response
    // GET /api/system/status → expect running: true
    // Shutdown cleanly
}
```

### P7.3 — Continuous Documentation Sync

`docs/` directory created with this assessment. Keep updated:
- `docs/CODEBASE_MAP.md` — update when adding packages
- `docs/ARCHITECTURE.md` — update when wiring changes
- `docs/BUGS_AND_ISSUES.md` — mark fixed, add new
- `docs/DEVELOPMENT_PLAN.md` — this doc, mark phases complete

---

## Phase 8 — Multi-Agent & Advanced Features

### P8.1 — Subagent Result Reporting

Currently `spawn` tool creates a subagent goroutine but results are lost if the parent session ends. Add persistent subagent result storage:
```
workspace/subagents/{id}/status.json
workspace/subagents/{id}/result.md
```

### P8.2 — Streaming LLM Responses

Current `Chat()` interface is blocking (wait for full response). Add streaming for large responses:
```go
type StreamingLLMProvider interface {
    LLMProvider
    ChatStream(ctx, messages, tools, model, options) (<-chan LLMChunk, error)
}
```

Streaming would reduce perceived latency for long agent responses, especially on Telegram.

### P8.3 — Agent Memory Consolidation

Currently each channel has its own session with separate summaries. Add cross-session memory consolidation:
- Weekly: merge daily notes into monthly summary
- Monthly: merge into long-term MEMORY.md section
- Triggered by cron job

### P8.4 — File Diffing & Code Review Tool

The `pkg/codex/` implementation is complete but no tool exposes it to the agent. Add a `code_review` tool that:
1. Reads `StructuredDiff` from workspace
2. Validates with `codex.Validate()`
3. Shows diff summary with risks
4. Applies with `codex.Apply()` on confirmation

### P8.5 — Antigravity AI Deep Integration

The ide-monitor tracks Antigravity brain artifacts. When brain events fire:
1. Create kanban card with task context
2. Notify agent via system message
3. Agent can pick up and assist with the AI coding session

---

## Priority Order Summary

```
IMMEDIATE (This Week)
├── P1.1  Fix filesystem restriction (SECURITY)
├── P1.2  Fix heartbeat callback
├── P1.3  Fix LLM params from config
├── P1.4  Fix webhook route pattern
├── P1.5  Fix heartbeat double-stop panic
├── P1.6  Fix Python KanbanStore default path
└── P1.7  Remove dead code

SHORT TERM (2 Weeks)
├── P2    Kanban unification (BUG-005)
├── P3.1  DI container decision + implementation
├── P3.2  VSCode integration auto-start
└── P3.3  System prompt budget

MEDIUM TERM (1 Month)
├── P4    Security hardening (credentials, rate limits)
├── P5    Performance (concurrent processing, retry queue)
└── P6.1  InboxBot implementation

LONG TERM (Ongoing)
├── P6.4  Sovereign Agent dashboard
├── P7    Test coverage expansion
└── P8    Multi-agent capabilities
```

---

## Metrics to Track

| Metric | Current | Target |
|--------|---------|--------|
| Go test coverage | ~35% | >70% |
| Python test coverage | ~60% | >80% |
| Open bugs (HIGH/MEDIUM) | 12 | 0 |
| Kanban unification | 2 DBs | 1 DB |
| Heartbeat working | No | Yes |
| File restriction active | No | Yes |
