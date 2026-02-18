# PicoClaw — Complete Codebase Map

> Generated: 2026-02-18  
> Go LOC: ~20,500 | Python LOC: ~3,500  
> Status: Compiles clean, all tests pass (32/32)

---

## Repository Root Layout

```
picoclaw/
├── cmd/picoclaw/main.go       # CLI entry point (1,548 lines) — ALL commands + startup logic
├── pkg/                       # Go packages — the core application
│   ├── agent/                 # AI agent loop, context building, memory
│   ├── api/                   # HTTP REST API + WebSocket dashboard server
│   ├── app/                   # Application services (DI container, service wrappers)
│   ├── auth/                  # OAuth2 / PKCE / token-paste auth flows
│   ├── bus/                   # In-process message bus (fan-out pub/sub)
│   ├── channels/              # Chat channel adapters (Telegram, Discord, Slack, etc.)
│   ├── codex/                 # Structured diff format for coding bots
│   ├── config/                # Config loading (JSON + env override)
│   ├── cron/                  # Cron job scheduler
│   ├── domain/                # DDD domain types, events, value objects
│   ├── events/                # Domain event fan-out (eventbus adapter)
│   ├── heartbeat/             # Periodic health check and proactive agent loop
│   ├── infrastructure/        # Infrastructure adapters (DB persistence, eventbus)
│   ├── integration/           # Plugin registry for external service integrations
│   ├── logger/                # Structured leveled logger
│   ├── migrate/               # OpenClaw → PicoClaw migration tool
│   ├── orchestration/         # Task assignment, lease locking, retry policies
│   ├── providers/             # LLM provider adapters (HTTP, Claude, Codex, Moonshot)
│   ├── session/               # Conversation session manager (disk-backed)
│   ├── skills/                # Skill loader + installer
│   ├── tools/                 # AI tool implementations (exec, fs, web, message, spawn)
│   ├── utils/                 # Utilities (string truncation, media helpers)
│   └── voice/                 # Groq voice transcription
├── ide-monitor/               # Python daemon: watches VS Code + Antigravity + Git → emits WorkflowEvents
├── telegram-bots/             # Python: Telegram bot swarm (dev, ops, monitor bots) + Kanban server
├── skills/                    # Built-in skill definitions (SKILL.md files)
│   ├── github/
│   ├── skill-creator/
│   ├── summarize/
│   ├── tmux/
│   └── weather/
├── templates/                 # Bot configuration templates
├── web/                       # Embedded Vue/React dashboard (web/dist embedded in binary)
├── assets/                    # Images for README
├── Caddyfile                  # Caddy reverse proxy config
├── Makefile                   # Build, test, run targets
├── go.mod                     # Go module: github.com/sipeed/picoclaw (Go 1.24.6)
├── config.json                # Active runtime config (git-ignored in prod)
├── config.example.json        # Reference config with all options
├── config.moonshot.json       # Moonshot-specific config example
├── config.telegram.json       # Telegram-specific config example
└── kanban.db                  # SQLite: native Go kanban task store
```

---

## Package-by-Package Reference

### `cmd/picoclaw/main.go` (1,548 lines)

**The CLI shell — ALL commands defined here.**

| Command | Function | Description |
|---------|----------|-------------|
| `onboard` | `onboard()` | Initialize workspace, write template files |
| `agent` | `agentCmd()` | Start interactive CLI agent or one-shot `-m` mode |
| `gateway` | `gatewayCmd()` | Start full gateway: agent + API server + channels + cron + integrations |
| `status` | `statusCmd()` | Show running config, provider status, OAuth credentials |
| `auth login/logout/status` | `authCmd()` | OAuth2/token auth management |
| `cron list/add/remove` | `cronCmd()` | Manage scheduled cron jobs from CLI |
| `skills list/install/remove` | `skillsCmd()` | Manage skills from CLI |
| `migrate` | `migrateCmd()` | Migrate from OpenClaw |

**Gateway startup sequence** (in `gatewayCmd()`):
1. Load config
2. Create LLM provider
3. Create `MessageBus`
4. Create `AgentLoop` (registers 9 tools)
5. Setup `CronTool` + `CronService`
6. Create `HeartbeatService`
7. Create `ChannelManager` (Telegram, Discord, Slack, etc.)
8. Attach Groq voice transcriber (if configured)
9. Initialize and start `IntegrationRegistry` (Kanban, VSCode)
10. Start all channels
11. Start `AgentLoop.Run()` goroutine
12. Start `API Server` (dashboard + REST + WebSocket)
13. Wait for SIGINT

---

### `pkg/config/config.go` (325 lines)

Central configuration struct with JSON + env-var override.

**Key types:**
- `Config` — top-level, thread-safe with `sync.RWMutex`
- `AgentDefaults` — model, max_tokens (8192), temperature (0.7), max_tool_iterations (20), workspace
- `ChannelsConfig` — Telegram, Discord, Slack, WhatsApp, Feishu, DingTalk, QQ, MaixCam
- `ProvidersConfig` — Anthropic, OpenAI, OpenRouter, Groq, Zhipu, VLLM, Gemini, Moonshot
- `GatewayConfig` — host (0.0.0.0), port (18790), api_key
- `ToolsConfig` — Brave web search API key + max results

**Env var pattern:** `PICOCLAW_AGENTS_DEFAULTS_MODEL`, `PICOCLAW_API_KEY`, etc.

**Default model:** `glm-4.7` (Zhipu GLM)

---

### `pkg/agent/` (1,082 lines total)

The **core AI reasoning engine**.

**`loop.go`** (664 lines) — `AgentLoop` struct:

```
AgentLoop
├── bus: *bus.MessageBus          // Receives inbound, publishes outbound
├── provider: LLMProvider         // The LLM backend
├── workspace: string             // Disk path for all workspace files
├── model: string                 // Active model name
├── contextWindow: int            // For summarization threshold
├── maxIterations: int            // Tool call loop cap (default: 20)
├── sessions: *SessionManager     // Disk-backed conversation history
├── contextBuilder: *ContextBuilder
└── tools: *ToolRegistry
```

**Key methods:**
- `NewAgentLoop()` — registers 9 built-in tools: `read_file`, `write_file`, `list_dir`, `exec`, `web_search`, `web_fetch`, `message`, `spawn`, `edit_file`, `ops_monitor`
- `Run(ctx)` — event loop: consume inbound bus messages → `processMessage()`
- `ProcessDirect()` / `ProcessDirectWithChannel()` — used by CLI and API
- `runAgentLoop()` — core: build context → call LLM → handle tool calls → repeat up to maxIterations
- `maybeSummarize()` — triggers async session summarization when >20 messages or >75% context window
- `summarizeSession()` — multi-part summarization, oversized-message guard, merge summaries

**`context.go`** (257 lines) — `ContextBuilder`:
- Builds the system prompt from: identity block (time, runtime, workspace, tools list), bootstrap files (AGENTS.md, SOUL.md, USER.md, IDENTITY.md), skills summary, memory context
- `BuildMessages()` — assembles the `[]providers.Message` array for LLM calls
- `buildToolsSection()` — dynamically enumerates tools into system prompt

**`memory.go`** (161 lines) — `MemoryStore`:
- Long-term: `workspace/memory/MEMORY.md`
- Daily notes: `workspace/memory/YYYYMM/YYYYMMDD.md`
- `GetMemoryContext()` — returns last 3 days of notes + long-term memory

---

### `pkg/providers/` (LLM Adapters)

All providers implement the single interface:
```go
type LLMProvider interface {
    Chat(ctx, messages, tools, model, options) (*LLMResponse, error)
    GetDefaultModel() string
}
```

| File | Provider | Notes |
|------|----------|-------|
| `http_provider.go` | Generic HTTP/OpenAI-compat | Used for OpenRouter, Zhipu, VLLM, Gemini |
| `claude_provider.go` | Anthropic Claude | Uses official Anthropic SDK |
| `codex_provider.go` | OpenAI Codex/GPT | Uses official OpenAI SDK |
| `moonshot_provider.go` | Moonshot AI | Custom HTTP client |

**Provider selection** (`providers.CreateProvider(cfg)`):  
Priority: Anthropic > OpenAI > OpenRouter > Zhipu > Moonshot > Gemini > Groq > VLLM  
Falls back to HTTP provider for most.

---

### `pkg/tools/` (AI Tool Implementations)

| Tool | File | What it does |
|------|------|-------------|
| `read_file` | `filesystem.go` | Read file contents (path-restricted unless fsAllowedDir unset) |
| `write_file` | `filesystem.go` | Write/create file |
| `list_dir` | `filesystem.go` | List directory entries |
| `exec` | `shell.go` | Shell command execution with deny-list patterns (rm -rf, shutdown, fork bomb, etc.) |
| `web_search` | `web.go` | Brave Search API — returns titles + URLs + snippets |
| `web_fetch` | `web.go` | HTTP GET + HTML-to-text stripping, 50KB cap |
| `message` | `message.go` | Send message back via channel bus |
| `spawn` | `spawn.go` | Spawn async subagent (via `SubagentManager`) |
| `edit_file` | `edit.go` | Targeted file edit (find-and-replace a block) |
| `ops_monitor` | `ops_monitor.go` | Remote control of picoclaw ops-monitor bot via HTTP |
| `cron` | `cron.go` | Schedule/manage recurring agent tasks |

**Security:** `ExecTool` blocks: `rm -rf`, `del /f`, `rmdir /s`, `format/mkfs/diskpart`, `dd if=`, `> /dev/sd*`, `shutdown/reboot/poweroff`, fork bombs

---

### `pkg/channels/` (Chat Channel Adapters)

**`base.go`** — `Channel` interface + `BaseChannel` (allow-list enforcement)  
**`manager.go`** — creates, starts, stops all enabled channels; routes outbound messages

| Channel | File | Library |
|---------|------|---------|
| Telegram | `telegram.go` | `mymmrac/telego` |
| Discord | `discord.go` | `bwmarrin/discordgo` |
| Slack | `slack.go` | `slack-go/slack` |
| WhatsApp | `whatsapp.go` | Custom (bridge over WebSocket) |
| Feishu (Lark) | `feishu.go` | `larksuite/oapi-sdk-go` |
| DingTalk | `dingtalk.go` | `open-dingtalk/dingtalk-stream-sdk-go` |
| QQ | `qq.go` | `tencent-connect/botgo` |
| MaixCam | `maixcam.go` | Custom TCP server (for Sipeed MaixCam hardware) |

**Voice:** Groq Whisper transcription attached to Telegram/Discord/Slack channels.

---

### `pkg/api/` (Dashboard HTTP API)

Serves on `cfg.Gateway.Host:cfg.Gateway.Port` (default `0.0.0.0:18790`).

**Middleware:** CORS (localhost-only origins) + API key auth (Bearer/X-API-Key header)

**Routes:**

| Path | Handler | Notes |
|------|---------|-------|
| `GET /api/health` | `handleHealth` | Liveness probe |
| `GET /api/system/status` | `handleSystemStatus` | Uptime, agent status, channel status, cron |
| `GET /api/system/info` | `handleSystemInfo` | Memory, goroutines, hostname, arch |
| `GET /api/channels` | `handleChannels` | Channel status map |
| `GET /api/sessions` | `handleSessions` | List conversation sessions |
| `GET/DELETE /api/sessions/{key}` | `handleSessionDetail` | Session history + delete |
| `GET /api/tools` | `handleTools` | Tool schema definitions |
| `GET /api/cron/jobs` | `handleCronJobs` | All cron jobs |
| `GET /api/cron/status` | `handleCronStatus` | Cron service status |
| `POST /api/agent/chat` | `handleAgentChat` | Chat with agent via API |
| `GET /api/agent/status` | `handleAgentStatus` | Agent startup info |
| `GET/POST /api/bots` | `handleBots` | List/create bots |
| `GET/PUT/DELETE /api/bots/{id}` | `handleBotByID` | Bot lifecycle |
| `POST /api/bots/{id}/start|stop` | `handleStartBot/Stop` | Bot control |
| `GET /api/bot-templates` | `handleListBotTemplates` | Template library |
| `GET /api/kanban/*` | `handleKanbanProxy` | Proxy to Python kanban server |
| `GET/POST /api/tasks` | `handleTasks` | Native Go task store |
| `GET/PUT/DELETE /api/tasks/{id}` | `handleTaskByID` | Individual task |
| `GET /api/vscode/status` | `handleVSCodeStatus` | VS Code extension status bar |
| `POST /api/vscode/todo` | `handleVSCodeTodo` | Push TODO → kanban |
| `POST /api/vscode/ask` | `handleVSCodeAsk` | Coding question → agent |
| `POST /api/vscode/diff/apply` | `handleVSCodeDiffApply` | Apply structured diff |
| `POST /api/vscode/diff/preview` | `handleVSCodeDiffPreview` | Validate diff |
| `GET /api/vscode/tasks` | `handleVSCodeTasks` | Tasks for VS Code extension |
| `POST /api/vscode/tasks/{id}/claim` | `handleVSCodeClaimTask` | Claim task |
| `POST /api/webhook/{source}` | `handleWebhook` | Accept events from local programs |
| `POST /api/events` | `handleWorkflowEvent` | Receive WorkflowEvent from ide-monitor |
| `GET /api/ws` | `wsHub.HandleWebSocket` | Live events WebSocket |
| `GET /` | `handleStaticFiles` | Serve embedded dashboard UI (SPA fallback) |

**Security:** Auto-generates random API key per session if not set in config (printed once at startup, Jupyter-pattern).

---

### `pkg/bus/` (Message Bus)

In-process pub/sub with channel-based fan-out.

```
MessageBus
├── inbound: chan InboundMessage  (buf=100)
├── outbound: chan OutboundMessage (buf=100)
├── inboundSubs: []*Subscriber   — fan-out taps (buf=64 each)
├── outboundSubs: []*Subscriber  — fan-out taps
└── systemSubs: []*Subscriber    — SystemEvent taps (for WS broadcasting)
```

**Types:** `InboundMessage`, `OutboundMessage`, `SystemEvent`  
**Fan-out subscriptions:** WebSocket hub subscribes as a tap; channels subscribe as outbound taps.

---

### `pkg/cron/` (Cron Scheduler)

- Supports `"every"` (interval in ms) and `"cron"` (cron expression via `gronx`)
- Jobs stored in `workspace/cron/jobs.json`
- `SetOnJob()` callback → agent processes job messages
- API: `AddJob`, `RemoveJob`, `EnableJob`, `ListJobs`, `Status`

---

### `pkg/session/` (Conversation Sessions)

- Disk-backed at `workspace/sessions/{key}.json`
- Per-session: message history (`[]providers.Message`), summary string, created/updated timestamps
- `AddMessage()`, `AddFullMessage()`, `TruncateHistory()`, `SetSummary()`
- Sessions include tool call messages (assistant + tool result pairs)

---

### `pkg/auth/` (Authentication)

- `oauth.go` — OAuth2 PKCE browser/device-code flow for OpenAI
- `pkce.go` — PKCE challenge/verifier generation
- `store.go` — credential persistence (`~/.picoclaw/auth.json`)
- `token.go` — `AuthCredential` struct with `IsExpired()`, `NeedsRefresh()`

---

### `pkg/domain/` (DDD Domain Layer)

Pure domain types with no infrastructure dependencies.

- `events.go` — 30+ typed `EventType` constants, `Event` interface, `BaseEvent`, `EventBus` interface
- `values.go` — `ChannelType`, `ProviderType`, `MessageRole` value objects
- `identity.go` — `EntityID` (string UUID), `AggregateRoot`, `TimeRange`
- `repository.go` — generic repository interfaces
- `domain/agent/agent.go` — Agent aggregate
- `domain/channel/channel.go` — Channel aggregate  
- `domain/session/session.go` — Session aggregate
- `domain/skill/skill.go` — Skill aggregate
- `domain/workflow/workflow.go` — Workflow aggregate
- `domain/provider/provider.go` — Provider aggregate

---

### `pkg/orchestration/` (Swarm Brain)

Handles multi-agent task routing:
- `TaskAssignment` — claim leases with expiry (prevents duplicate execution)
- `AgentCapability` — categories, tools, max_concurrent, priority
- `RetryPolicy` — max_attempts (3), exponential backoff (5s→60s), escalate to human

---

### `pkg/integration/` (Plugin Registry)

- `registry.go` — global `IntegrationRegistry`, `Register()`, `InitAll()`, `StartAll()`
- Interfaces: `Integration`, `APIIntegration` (routes), `EventConsumer` (bus subscriptions)
- `kanban/kanban.go` — SQLite-backed kanban (auto-registered via `init()`)
- `vscode/vscode.go` — VS Code integration (activity tracking)

---

### `pkg/codex/` (Structured Diff)

Typed coding bot output format — prevents free-text file edits.

```
FileChange {
    Op: create|modify|delete|rename|insert
    Path: string
    OldPath: string (rename)
    OldContent: string (modify — content to replace)
    NewContent: string
    InsertLine: int
    Checksum: string (SHA-256 of NewContent)
}

StructuredDiff {
    ID, TaskID, AgentID, Timestamp
    Changes: []FileChange
    Metadata: map[string]string
}
```

- `diff.go` — create, apply, serialize, deserialize diffs
- `verify.go` — pre/post apply verification (checksum, existence checks)

---

### `pkg/migrate/`

Migrates from **OpenClaw** (`~/.openclaw`) to **PicoClaw** (`~/.picoclaw`):
- Config field mapping (`openrouter_api_key` → `providers.openrouter.api_key`)
- Workspace files: `AGENTS.md`, `SOUL.md`, `USER.md`, `TOOLS.md` (skipped if missing)
- `--dry-run`, `--config-only`, `--workspace-only`, `--refresh`, `--force` flags

---

### `pkg/heartbeat/`

Periodic proactive agent trigger (default: every 30 minutes):
- Reads `workspace/memory/HEARTBEAT.md` for agenda instructions
- Calls `onHeartbeat(prompt)` — in gateway, this is unused (callback is `nil`)
- **NOTE:** In `gatewayCmd()`, heartbeat callback is `nil` — heartbeat fires but does nothing. **This is a known gap.**

---

### `pkg/logger/`

Structured, leveled logger (DEBUG/INFO/WARN/ERROR). Methods: `InfoC`, `InfoCF`, `DebugC`, `DebugCF`, `ErrorCF`, `WarnCF`. Output to stdout.

---

## `ide-monitor/` — Python Daemon (3,500 lines)

Watches the developer's environment and emits `WorkflowEvent v1` NDJSON to picoclaw or a local log file.

**Architecture:**
```
watcher.py (main)
├── UnifiedHandler (watchdog FileSystemEventHandler)
│   ├── parsers/antigravity.py  → AG_TASK_* events
│   ├── parsers/copilot.py      → COPILOT_* events
│   └── parsers/git.py          → GIT_COMMIT events
├── burst_detector.py
│   ├── FileBurstDetector        → filesystem.batch_modified
│   └── CopilotBurstDetector     → copilot.burst_start/end
├── correlator.py (TemporalCorrelator)
│   └── git.commit_linked_to_task, workflow.activity_clustered
├── confidence.py (ConfidenceScorer)
│   └── confidence 0.0–1.0 per event
└── emitter.py
    ├── → POST /api/events (picoclaw) if PICOCLAW_URL set
    └── → ~/.local/share/ide-monitor/events.jsonl (fallback)
```

**Event taxonomy** (closed set in `normalizer.py`):
- `copilot.prompt/completion/error/burst_start/burst_end`
- `antigravity.task.created/updated/progress/completed/failed/walkthrough_added/plan_ready/iterated/skill.updated/artifact.unknown`
- `git.commit/commit_linked_to_task`
- `filesystem.batch_modified`
- `workflow.task_inferred/activity_clustered/conflict_detected`

**Parsers:**
- `parsers/antigravity.py` — reads Antigravity AI brain `.md` artifacts
- `parsers/copilot.py` — reads VS Code Copilot telemetry log files
- `parsers/git.py` — reads `.git/COMMIT_EDITMSG` on post-commit

**Tests:** 5 test files, ~80 test cases covering burst, confidence, correlator, normalizer, parsers.

---

## `telegram-bots/` — Python Telegram Bot Swarm

Separate Python project with its own `.venv` and `requirements.txt`.

### Bots

| Bot | File | Commands |
|-----|------|---------|
| `DevBot` | `bots/dev_bot.py` | `/run_tests`, `/deploy`, `/scaffold`, `/git_status` |
| `OpsBot` | `bots/ops_bot.py` | `/status`, `/logs`, `/stop`, `/restart_service`, `/disk_usage`, `/process_info` |
| `MonitorBot` | `bots/monitor_bot.py` | Monitoring alerts and dashboards |
| `InboxBot` | `bots/inbox_bot.py` | Task inbox management |

**Base class** (`bots/bot_base.py`): auth (allow-list), command parsing, confirmation flow (`AWAITING_CONFIRM` state), audit logging, Kanban integration.

### Kanban System

```
pkg/kanban/
├── schema.py     — TaskState (inbox→planned→running→blocked→review→done), TaskCategory, TaskMode
├── store.py      — SQLite CRUD with WAL mode, state_history table, migration
├── telegram_bridge.py — Create/transition cards from Telegram interactions
├── categorizer.py — LLM-based task categorization
├── mode.py       — personal/remote mode switching
└── events.py     — event emission for kanban state changes
```

**Storage:** SQLite at `/var/lib/picoclaw/kanban.db` (configurable)  
**Tables:** `kanban_cards`, `state_history`, `create_indexes`

### Kanban Server

`kanban_server.py` — Flask HTTP server:
- `GET /` → Kanban HTML UI (`kanban_ui.html`)
- `GET /api/board` → JSON board state (cards, events, mode, stats)
- `GET /api/mode` → current mode
- `POST /api/mode` → switch personal/remote mode

**Executor** (`bots/executor.py`): polls for `planned` tasks and executes them.

### Config

`config/picoclaw.yaml` — bot configuration with commands, schemas, executor paths.

---

## Go Module Dependencies

| Dependency | Purpose |
|-----------|---------|
| `mymmrac/telego` | Telegram bot |
| `bwmarrin/discordgo` | Discord bot |
| `slack-go/slack` | Slack bot |
| `larksuite/oapi-sdk-go` | Feishu/Lark |
| `open-dingtalk/dingtalk-stream-sdk-go` | DingTalk |
| `tencent-connect/botgo` | QQ |
| `anthropics/anthropic-sdk-go` | Claude provider |
| `openai/openai-go` | Codex/GPT provider |
| `mattn/go-sqlite3` | Kanban SQLite |
| `gorilla/websocket` | WebSocket hub |
| `adhocore/gronx` | Cron expression parsing |
| `caarlos0/env` | Env var → struct binding |
| `chzyer/readline` | Interactive CLI history |
| `google/uuid` | UUID generation |
| `golang.org/x/oauth2` | OAuth2 flows |

---

## Runtime Data Paths

| Path | Contents |
|------|---------|
| `~/.picoclaw/config.json` | Main config |
| `~/.picoclaw/auth.json` | OAuth credentials |
| `~/.picoclaw/workspace/` | Default workspace root |
| `workspace/AGENTS.md` | Agent instructions |
| `workspace/SOUL.md` | Agent personality |
| `workspace/USER.md` | User profile |
| `workspace/IDENTITY.md` | Agent identity |
| `workspace/HEARTBEAT.md` | Heartbeat agenda |
| `workspace/memory/MEMORY.md` | Long-term memory |
| `workspace/memory/YYYYMM/YYYYMMDD.md` | Daily notes |
| `workspace/sessions/{key}.json` | Conversation history |
| `workspace/cron/jobs.json` | Cron job definitions |
| `workspace/skills/{name}/SKILL.md` | Installed skills |
| `/var/lib/picoclaw/kanban.db` | Kanban SQLite (Python side) |
| `picoclaw/kanban.db` (repo root) | Kanban SQLite (Go side, dev) |
| `~/.local/share/ide-monitor/events.jsonl` | ide-monitor fallback log |
