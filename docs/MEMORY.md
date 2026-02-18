# PicoClaw — AI Context Anchor (MEMORY.md)

> **Purpose:** Load this file at the start of any AI session to instantly orient to the project.  
> **Last Audited:** 2026-02-18 | All 184 tests passing | Build clean  
> **Detailed docs:** [CODEBASE_MAP.md](CODEBASE_MAP.md) | [ARCHITECTURE.md](ARCHITECTURE.md) | [BUGS_AND_ISSUES.md](BUGS_AND_ISSUES.md) | [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md)

---

## What Is PicoClaw?

PicoClaw is an ultra-lightweight self-hostable AI agent framework targeting $10 hardware (Sipeed AX620E-NANO, 256 MB RAM). It is a full Go rewrite of OpenClaw with:
- **<10 MB RAM** (99% less than OpenClaw)
- **<1s boot** (400× faster)
- **Single binary** — compile and copy to device

Primary use: personal AI agent accessible via multiple messaging channels, with a web dashboard.

---

## Critical Facts

| Key | Value |
|-----|-------|
| Go module | `github.com/sipeed/picoclaw` |
| Go version | 1.24.6 |
| Default API port | **18790** |
| Default model | **`glm-4.7`** (Zhipu GLM) |
| Default max tokens | 8192 |
| Default temperature | 0.7 |
| Config file | `~/.picoclaw/config.json` (or `./config.json`) |
| Workspace | `~/.picoclaw/workspace/` |
| Auth file | `~/.picoclaw/auth.json` |
| Sessions | `~/.picoclaw/workspace/sessions/*.json` |
| Memory | `~/.picoclaw/workspace/memory/MEMORY.md` |
| Daily notes | `~/.picoclaw/workspace/memory/YYYYMM/YYYYMMDD.md` |
| Kanban DB (Go) | `./kanban.db` (CWD at startup) |
| Kanban DB (Python) | `/var/lib/picoclaw/kanban.db` (→ WRONG, BUG-014) |

---

## Package Quick Reference

```
cmd/picoclaw/main.go        CLI, gateway startup, all cobra commands (1,548 lines)
pkg/config/config.go        Config struct, env override (PICOCLAW_*), defaults
pkg/agent/loop.go           AgentLoop: core Run() loop, max 20 tool iterations
pkg/agent/context.go        SystemPrompt builder (identity + skills + memory)
pkg/agent/memory.go         Read/write MEMORY.md + daily notes
pkg/api/server.go           25+ REST routes + WebSocket + embedded SPA
pkg/api/ws.go               WebSocket hub, broadcast to dashboard clients
pkg/api/webhooks.go         Webhook ingestion (BUG: route pattern wrong)
pkg/api/workflow_events.go  WorkflowEvent from ide-monitor → bus + kanban
pkg/api/vscode.go           VS Code extension API (todos, ask, diff, tasks)
pkg/api/kanban.go           Kanban CRUD + proxy to Python port 3000
pkg/bus/bus.go              MessageBus: inbound/outbound/system channels
pkg/channels/manager.go     ChannelManager: load/start/stop all channels
pkg/channels/base.go        BaseChannel: common allow-list, send helper
pkg/tools/registry.go       ToolRegistry + ToolDefinition (thread-safe)
pkg/tools/filesystem.go     read_file, write_file, list_dir, edit_file (UNSAFE: BUG-007)
pkg/tools/shell.go          exec tool with deny-list regex
pkg/tools/web.go            web_search + web_fetch
pkg/tools/message.go        message tool (send to channel)
pkg/tools/spawn.go          spawn tool (subagent goroutine)
pkg/providers/types.go      LLMProvider interface, Message, ToolDefinition types
pkg/heartbeat/service.go    HeartbeatService (BUG: nil callback, double-Stop panic)
pkg/cron/service.go         Cron scheduler, gronx expressions, JSON persistence
pkg/session/manager.go      Session load/save/list, JSON files
pkg/skills/loader.go        Skills: workspace-local, global, GitHub install
pkg/integration/registry.go Auto-registration via init() pattern
pkg/integration/kanban/     SQLite kanban (repo-root kanban.db) — BUG-005
pkg/orchestration/          Swarm brain, task assignment (category routing stub)
pkg/codex/diff.go           StructuredDiff format, SHA-256 checksums
pkg/auth/oauth.go           OAuth2 PKCE for OpenAI; token-paste for Anthropic
pkg/migrate/                OpenClaw → PicoClaw migration (8 test cases, all pass)
pkg/domain/                 DDD domain layer (events, aggregates, value objects — NOT wired)
pkg/app/container.go        DI Container — defined but NEVER called (BUG-004)
```

---

## Top 5 Active Bugs (Must Fix First)

| ID | Severity | Summary | File |
|----|----------|---------|------|
| BUG-005 | 🔴 CRITICAL | Two kanban SQLite DBs not synced | `pkg/integration/kanban/` + `telegram-bots/pkg/kanban/` |
| BUG-007 | 🔴 HIGH | Filesystem tools unrestricted (security hole) | `pkg/tools/filesystem.go` |
| BUG-001 | 🟡 HIGH | Heartbeat fires but callback is nil (no-op) | `cmd/picoclaw/main.go:670` |
| BUG-006 | 🟡 HIGH | max_tokens/temperature hardcoded, ignore config | `pkg/agent/loop.go:316-317` |
| BUG-013 | 🟡 HIGH | HeartbeatService.Stop() can panic on 2nd call | `pkg/heartbeat/service.go` |

Full list: 22 bugs in [BUGS_AND_ISSUES.md](BUGS_AND_ISSUES.md)

---

## Architecture in One View

```
User/Bot                   HTTP/WS                  Agent                LLM
──────────    ─────────────────────────────────   ─────────────   ─────────────
Telegram  ──→ ChannelManager ──→ MessageBus ──→  AgentLoop ──→  Zhipu/Anthropic/
Discord   ──→        │                              │             OpenAI/OpenRouter
Slack     ──→        │                           10 tools:        /Moonshot/Gemini
WhatsApp  ──→        │                            exec, file,     /Groq/VLLM
Feishu    ──→        │                            web, message,
DingTalk  ──→        ↓                            spawn, cron,
Web SPA   ──→ REST API (18790) ← WebSocket hub   ops_monitor
VS Code   ──→                                         │
                                                      ↓
                                                 Session JSON
                                                 MEMORY.md
                                                 Kanban SQLite
```

---

## Python Daemons

### ide-monitor (separate process)
```
watcher.py → parsers (Copilot/Antigravity/Git) → correlator → emitter → POST /api/events
```
- Tracks IDE coding events
- Emits `WorkflowEvent` structs with confidence scores
- If confidence ≥ threshold → picoclaw creates kanban card

### telegram-bots (separate process)
```
dev_bot.py   → /run_tests /deploy /scaffold /git_status
ops_bot.py   → /status /logs /stop /restart_service
monitor_bot.py → monitoring alerts
inbox_bot.py → STUB - not implemented (BUG-019)
executor.py  → inotify watcher for YAML task files
```
All bots use `bot_base.py` for auth, audit logging, and kanban.

---

## How to Run

```bash
# Build
go build -o picoclaw ./cmd/picoclaw

# Start gateway
./picoclaw gateway --config config.json

# Or with wrapper script
./start.sh

# API health check
curl http://localhost:18790/api/health
```

---

## Important Conventions

1. **Tool registration:** New tools call `registry.Register(tool)` in `NewAgentLoop()`. Use snake_case names.
2. **Channel registration:** New channels: implement `channels.Channel` interface, register in `ChannelManager.loadChannel()`.
3. **Integration registration:** Use `init()` + `integration.Register(&MyIntegration{})` for auto-start.
4. **Domain events:** Use `domain.EventType` constants. Don't add raw strings.
5. **Config access:** Always use `cfg.WorkspacePath()` — never hardcode `~/.picoclaw/`.
6. **Bus messages:** Inbound = user → agent. Outbound = agent → user. System = internal events.
7. **Session key:** Format is `"{channel}:{channelID}:{userID}"`.
8. **Auth file:** `~/.picoclaw/auth.json` — plaintext (BUG-008, fix pending).
9. **Skills:** Discovered from `workspace/skills/` (local) and `~/.picoclaw/skills/` (global).
10. **URL pattern:** PathValue requires `{param}` in route — currently broken in webhooks (BUG-002).

---

## What to Do Next (Priority Order)

1. `pkg/tools/filesystem.go` — call `SetFSAllowedDir(workspace)` in `NewAgentLoop()`
2. `cmd/picoclaw/main.go:670` — wire heartbeat callback to agentLoop.ProcessDirect
3. `pkg/agent/loop.go:316-317` — read max_tokens/temperature from config
4. Kanban unification — Python bots use REST API, not separate SQLite
5. `pkg/integration/vscode/vscode.go` — add `init()` for auto-registration
6. `pkg/heartbeat/service.go` — `sync.Once` for Stop()
7. Implement `telegram-bots/bots/inbox_bot.py`

See [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) for full phased roadmap.

---

## LLM Provider Selection Logic

```
GetAPIKey() priority: OpenRouter → Anthropic → OpenAI → Moonshot → Gemini → Zhipu → Groq → VLLM

Provider constructors (all in pkg/providers/):
  anthropic.go, openai.go, openrouter.go, zhipu.go,
  moonshot.go, gemini.go, groq.go, vllm.go
```

---

## Known Missing Pieces (Not Yet Built)

- [ ] Credential encryption (BUG-008) — auth.json is plaintext
- [ ] Streaming LLM responses (blocking today)
- [ ] Rate limiting on /api/* (no IP-level rate limit)
- [ ] InboxBot Telegram commands (BUG-019)
- [ ] Orchestrator category routing (BUG-011 — stub only)
- [ ] Cross-session memory consolidation
- [ ] Sovereign Agent dashboard (picoclaw_ui_plan.md design → web/dist/)
- [ ] `pkg/app/container.go` DI wiring (BUG-004)
