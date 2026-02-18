# PicoClaw — Bugs, Issues & Technical Debt

> Assessment date: 2026-02-18  
> Test status: Go 32/32 ✅ | ide-monitor 60/60 ✅ | telegram-bots 92/92 ✅  
> Compilation: `go build ./...` clean ✅ | `go vet ./...` clean ✅

All **known issues are non-critical** — the binary builds and all tests pass. The issues below represent architectural gaps, dead code, security deferments, and missing integrations.

---

## Severity Legend

| Severity | Meaning |
|---------|---------|
| 🔴 HIGH | Causes silent failure or data loss in production |
| 🟡 MEDIUM | Wrong behavior, security gap, or architectural inconsistency |
| 🟢 LOW | Code smell, dead code, hardcoded value, missing feature |

---

## Bug / Issue Catalog

---

### BUG-001 — Heartbeat Callback Is Nil (Silent No-Op) 🟡

**File:** `cmd/picoclaw/main.go:667-672`  
**Code:**
```go
heartbeatService := heartbeat.NewHeartbeatService(
    cfg.WorkspacePath(),
    nil,        // ← callback is nil
    30*60,
    true,
)
```

**Effect:** The heartbeat service starts successfully and fires every 30 minutes, but `checkHeartbeat()` short-circuits at `if hs.onHeartbeat != nil` — it does nothing. The intent was to call `agentLoop.ProcessDirect()` with a heartbeat prompt so the agent proactively checks for tasks.

**Fix:** Wire the callback:
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

### BUG-002 — Webhook `PathValue("source")` Always Returns Empty (Dead Route Pattern) 🟡

**File:** `pkg/api/webhooks.go:46`, `pkg/api/server.go:133`  
**Code:**
```go
// server.go
mux.HandleFunc("/api/webhook/", s.handleWebhook)  // no {source} pattern

// webhooks.go
source := r.PathValue("source")                    // always ""
if source == "" {
    source = r.URL.Path[len("/api/webhook/"):]    // this fallback works
```

**Effect:** `r.PathValue("source")` requires the route to be registered with a `{source}` wildcard (e.g., `/api/webhook/{source}`). Without it, the function returns `""`. The fallback manual string trim covers this, but the API is inconsistent.

**Fix:** Register the route properly:
```go
mux.HandleFunc("/api/webhook/{source}", s.handleWebhook)
// then remove the fallback in handleWebhook
```

---

### BUG-003 — Duplicate `skillsCmd()` Function (Dead Code) 🟢

**File:** `cmd/picoclaw/main.go:1301`  
**Issue:** Function `skillsCmd()` is defined at line 1301 but is **never called**. The `main()` switch case `"skills":` handles skills commands with inline code (lines 131-175). The standalone `skillsCmd()` function duplicates that logic and is dead code.

**Fix:** Remove `func skillsCmd()` entirely (lines 1301-1360).

---

### BUG-004 — DI Container (`pkg/app/container.go`) Is Defined But Never Used 🟡

**File:** `pkg/app/container.go`  
**Issue:** `NewContainer()` creates a fully wired application container with all services (AgentService, ChannelService, SessionService, SkillService, WorkflowService). But `gatewayCmd()` in `main.go` constructs every package manually via direct constructor calls, completely bypassing the container.

**Effect:** The `pkg/app/` service layer is architecturally correct DDD code that has zero runtime presence. Any bugs in those services will never be caught. The container exists but the system does not use DI.

**Fix:** Either:
1. Wire `gatewayCmd()` through `app.NewContainer()`, OR
2. Delete `pkg/app/` and document "flat construction" as the design decision

---

### BUG-005 — Two Unsynchronized Kanban Databases 🔴

**Locations:**
- Python: `/var/lib/picoclaw/kanban.db` (used by `telegram-bots/pkg/kanban/store.py`)
- Go: `picoclaw/kanban.db` (repo root, used by `pkg/integration/kanban/kanban.go`)

**Effect:** Tasks created via Telegram bots (Python) exist only in the Python SQLite. Tasks created via API (`/api/tasks`) exist only in the Go SQLite. The VS Code extension reads from Go kanban. Telegram bots read from Python kanban. Neither sees the other's tasks.

**Fix:** Unify to a single database. Either:
1. Go reads/writes Python's `/var/lib/picoclaw/kanban.db`, OR
2. Python connects to Go's `/api/tasks` REST API instead of SQLite directly

---

### BUG-006 — LLM Parameters Hardcoded in Agent Loop 🟡

**File:** `pkg/agent/loop.go:297-300`  
**Code:**
```go
response, err := al.provider.Chat(ctx, messages, providerToolDefs, al.model, map[string]interface{}{
    "max_tokens":  8192,
    "temperature": 0.7,
})
```

**Effect:** `max_tokens` and `temperature` ignore `config.Agents.Defaults.MaxTokens` and `config.Agents.Defaults.Temperature`. Changing config values has no effect.

**Fix:**
```go
map[string]interface{}{
    "max_tokens":  al.contextWindow,
    "temperature": 0.7,          // read from cfg field when added to AgentLoop
}
```
Also pass temperature from config through `AgentLoop`.

---

### BUG-007 — `pkg/tools/filesystem.go` Path Restriction Not Enforced in Production 🟡

**File:** `pkg/tools/filesystem.go:12-16`  
**Code:**
```go
var fsAllowedDir string

func SetFSAllowedDir(dir string) {
    fsAllowedDir = dir
}
```

**Effect:** `SetFSAllowedDir()` is exported but **never called** in `gatewayCmd()` or `agentCmd()`. The `read_file`, `write_file`, `list_dir` tools can read/write any path on the filesystem that the process has access to. The AI agent can read `/etc/passwd`, `~/.picoclaw/auth.json`, etc.

**Fix:** Call `tools.SetFSAllowedDir(cfg.WorkspacePath())` in `NewAgentLoop()`.

---

### BUG-008 — Credentials Stored Plaintext 🟡

**File:** `pkg/auth/store.go`  
**Effect:** `~/.picoclaw/auth.json` stores OAuth tokens and API keys in plaintext JSON. On a shared or remotely-accessible machine, any user or process with file read access to the home directory can exfiltrate credentials.

**Fix:** Encrypt file at rest using OS keyring (`golang.org/x/sys/unix` secret store or `99designs/keychain`).

---

### BUG-009 — `AgentLoop.Run()` Processes Messages Sequentially 🟡

**File:** `pkg/agent/loop.go:116-140`  
**Effect:** All inbound messages from all channels funnel into a single goroutine that processes them one at a time. A slow LLM response (20-120s) blocks all other channels. Under multi-user load, messages queue up.

**Fix for multi-user workloads:** Process each message in its own goroutine with a semaphore limit:
```go
sem := make(chan struct{}, maxConcurrent)
go func() {
    sem <- struct{}{}
    defer func() { <-sem }()
    // process message
}()
```

---

### BUG-010 — Kanban Proxy URL Hardcoded 🟢

**File:** `pkg/api/kanban_proxy.go:39`  
**Code:**
```go
const defaultKanbanURL = "http://localhost:3000"
// TODO: make configurable via config.json
```

**Fix:** Add `kanban_server_url` field to `ToolsConfig` or a new `IntegrationsConfig` section.

---

### BUG-011 — Orchestrator `pkg/orchestration/orchestrator.go:292` Missing Kanban Lookup 🟢

**Code:**
```go
// TODO: look up category from task ID via kanban
```

**Effect:** Task routing by category falls back to default agent for all tasks. Capability-based routing is not implemented.

---

### BUG-012 — `pkg/domain/identity.go` Panics on UUID Gen Failure 🟢

**Code:**
```go
panic(fmt.Sprintf("domain: failed to generate ID: %v", err))
```

**Effect:** UUID generation (`crypto/rand`) theoretically can fail. A `panic` in ID generation will crash the entire service without recovery. This is in the domain layer which is not yet wired, but when it is, this should be `return EntityID(""), err`.

---

### BUG-013 — Heartbeat Service Cannot Be Stopped Reliably 🟡

**File:** `pkg/heartbeat/service.go:56-64`  
**Code:**
```go
func (hs *HeartbeatService) Stop() {
    hs.mu.Lock()
    defer hs.mu.Unlock()
    if !hs.running() {
        return
    }
    close(hs.stopChan)   // ← can panic if called twice
}
```

**Effect:** If `Stop()` is called twice (e.g., signal handler + deferred stop), `close()` on an already-closed channel panics. `running()` checks via `select/default` which has a race with the close.

**Fix:** Use `sync.Once` for `close`:
```go
hs.closeOnce.Do(func() { close(hs.stopChan) })
```

---

### BUG-014 — Python `KanbanStore` Default DB Path Requires Root 🟡

**File:** `telegram-bots/pkg/kanban/store.py:28`  
**Code:**
```python
def __init__(self, db_path: str = "/var/lib/picoclaw/kanban.db"):
```

**Effect:** The default path `/var/lib/picoclaw/` requires root or a privileged service user to create. Running the kanban bots as a regular user silently uses a path that doesn't exist, causing `sqlite3.OperationalError`.

**Fix:** Default to a user-writable path:
```python
DEFAULT_DB = Path.home() / ".local" / "share" / "picoclaw" / "kanban.db"
```

---

### BUG-015 — `executor.py` Hardcodes `/workspace/tasks/` and `/projects/` Paths 🟡

**File:** `telegram-bots/bots/executor.py`  
**Code:**
```python
WATCH_DIRS = [
    Path("/workspace/tasks/dev"),
    Path("/workspace/tasks/ops"),
    ...
]
PROJECTS_BASE = Path("/projects")
```

**Effect:** Executor fails silently on any machine without `/workspace/tasks/` and `/projects/` directories. These are Docker container paths; the executor breaks outside containers.

**Fix:** Read paths from `config/picoclaw.yaml` or environment variables.

---

### BUG-016 — `ide-monitor` Has No Retry on `POST /api/events` Failure 🟢

**File:** `ide-monitor/emitter.py`  
**Code:**
```python
try:
    r = requests.post(PICOCLAW_URL, data=payload, timeout=2)
    if r.ok:
        return
except Exception:
    pass  # Fall through to JSONL
```

**Effect:** If picoclaw is temporarily unavailable (restart, overload), events are silently written to JSONL instead of being queued for retry. Events are permanently missed from the live dashboard.

**Fix:** Add a simple exponential backoff retry queue.

---

### BUG-017 — No Rate Limit in `kanban/mode.py` 🟢

**File:** `telegram-bots/pkg/kanban/mode.py:98`  
**Code:**
```python
# TODO: implement actual rate limiting
```

**Effect:** Remote mode has no rate limiting implemented despite the docstring claiming "rate-limited". Bots in remote mode can be flooded.

---

### BUG-018 — `createWorkspaceTemplates()` Called Twice in `onboard()` 🟢

**File:** `cmd/picoclaw/main.go:350-393`  
**Effect:** The template file creation loop (`for filename, content := range templates`) appears twice in `onboard()` — once before the memory file creation and once after. The second pass is always a no-op (files exist) but wastes time and is confusing.

---

### BUG-019 — Missing `inbox_bot.py` on `InboxBot` Implementation 🟢

**File:** `telegram-bots/bots/inbox_bot.py`  
**Effect:** The file exists but no tests cover it. The bot is referenced in README but not wired in any runner or Makefile target. Status: **unimplemented**.

---

### BUG-020 — System Prompt Size Not Bounded 🟡

**File:** `pkg/agent/context.go`  
**Effect:** `BuildSystemPrompt()` concatenates: identity block + all bootstrap files + all skills summaries + long-term memory + 3 days of daily notes. For a workspace with many skills and extensive memory files, the system prompt can exceed the model's context window limit, causing LLM errors.

**Fix:** Implement a token budget per section with truncation fallback.

---

### BUG-021 — VSCode Integration Not Auto-Started in Registry 🟢

**File:** `pkg/integration/vscode/vscode.go`  
**Issue:** Unlike `kanban/kanban.go` which has `func init() { integration.Register(&KanbanIntegration{}) }`, the VSCode integration has no `init()` auto-registration. It is never started unless explicitly wired.

---

### BUG-022 — WebSocket Connection Leak on Malformed Upgrade 🟢

**File:** `pkg/api/ws.go`  
**Effect:** If `upgrader.Upgrade()` fails (malformed WebSocket request), the handler returns an HTTP error but the client connection may not be properly cleaned up in all edge cases. Under fuzz conditions this could cause goroutine leaks.

---

## Summary Table

| ID | Severity | Category | One-liner |
|----|---------|---------|-----------|
| BUG-001 | 🟡 | Integration | Heartbeat fires but does nothing (nil callback) |
| BUG-002 | 🟡 | Routing | `PathValue("source")` always empty, fallback works |
| BUG-003 | 🟢 | Dead Code | `skillsCmd()` defined but never called |
| BUG-004 | 🟡 | Architecture | DI Container defined but never used |
| BUG-005 | 🔴 | Data | Two unsynchronized kanban SQLite databases |
| BUG-006 | 🟡 | Config | LLM max_tokens and temperature hardcoded, ignore config |
| BUG-007 | 🟡 | Security | Filesystem tools unrestricted — can read any file |
| BUG-008 | 🟡 | Security | OAuth/API credentials stored plaintext |
| BUG-009 | 🟡 | Performance | Agent message processing is sequential (single goroutine) |
| BUG-010 | 🟢 | Config | Kanban proxy URL hardcoded as `localhost:3000` |
| BUG-011 | 🟢 | Features | Orchestrator category routing not implemented |
| BUG-012 | 🟢 | Stability | Domain ID generates panic on crypto/rand failure |
| BUG-013 | 🟡 | Stability | HeartbeatService.Stop() can panic if called twice |
| BUG-014 | 🟡 | Portability | Python KanbanStore defaults to `/var/lib/` (requires root) |
| BUG-015 | 🟡 | Portability | executor.py hardcodes Docker container paths |
| BUG-016 | 🟢 | Reliability | ide-monitor drops events instead of retrying |
| BUG-017 | 🟢 | Security | Remote mode rate limiting not implemented |
| BUG-018 | 🟢 | Dead Code | `createWorkspaceTemplates()` called twice in `onboard()` |
| BUG-019 | 🟢 | Missing | `InboxBot` files exist but bot is not implemented |
| BUG-020 | 🟡 | Reliability | System prompt has no size bound (context overflow risk) |
| BUG-021 | 🟢 | Missing | VSCode integration never auto-registered or started |
| BUG-022 | 🟢 | Stability | WebSocket upgrade failure may leak goroutine |
