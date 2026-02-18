# PicoClaw — System Architecture

> Last updated: 2026-02-18

---

## 1. Overview

PicoClaw is a **multi-modal personal AI agent** with the following design principles:

- **Ultra-light:** single Go binary <10MB, <50MB RAM in idle gateway mode
- **Multi-channel:** Telegram, Discord, Slack, WhatsApp, Feishu, DingTalk, QQ, MaixCam hardware, and web dashboard
- **Multi-provider:** OpenRouter, Anthropic Claude, OpenAI/Codex, Moonshot, Zhipu GLM, Groq, VLLM (local)
- **Tool-first:** agent must execute tools; it cannot pretend. Tool calls are logged and audited.
- **Memory-persistent:** long-term memory (MEMORY.md) + daily notes across sessions
- **Skills-extensible:** drop a SKILL.md in the workspace or install from GitHub

---

## 2. High-Level System Diagram

```
┌────────────────────────────────────────────────────────────────────────┐
│                        picoclaw gateway process                        │
│                                                                        │
│  ┌──────────────┐   ┌─────────────────────────────────────────────┐   │
│  │ ChannelMgr   │   │              AgentLoop                       │   │
│  │              │   │  ┌─────────────┐  ┌──────────────────────┐  │   │
│  │ Telegram ─┐  │   │  │ContextBuild │  │   ToolRegistry       │  │   │
│  │ Discord  ─┤  │   │  │  - AGENTS   │  │  read_file           │  │   │
│  │ Slack    ─┤──┼──▶│  │  - SOUL.md  │  │  write_file          │  │   │
│  │ WhatsApp ─┤  │   │  │  - USER.md  │  │  list_dir            │  │   │
│  │ Feishu   ─┤  │   │  │  - skills   │  │  exec                │  │   │
│  │ DingTalk ─┤  │   │  │  - memory   │  │  web_search          │  │   │
│  │ MaixCam  ─┘  │   │  └──────┬──────┘  │  web_fetch           │  │   │
│  └──────┬───────┘   │         │         │  message             │  │   │
│         │           │         ▼         │  spawn               │  │   │
│         │           │  ┌─────────────┐  │  edit_file           │  │   │
│  ┌──────▼───────┐   │  │ LLMProvider │  │  ops_monitor         │  │   │
│  │  MessageBus  │◀──┤  │  (HTTP/     │  │  cron                │  │   │
│  │  (in-proc    │   │  │  Claude/    │  └──────────────────────┘  │   │
│  │  pub/sub)    │──▶│  │  Codex/     │                            │   │
│  └──────┬───────┘   │  │  Moonshot)  │  ┌──────────────────────┐  │   │
│         │           │  └─────────────┘  │   SessionManager     │  │   │
│         │           │                   │  disk-backed JSON    │  │   │
│  ┌──────▼───────┐   │                   └──────────────────────┘  │   │
│  │  CronService │   └─────────────────────────────────────────────┘   │
│  │  (gronx)     │                                                      │
│  └──────────────┘   ┌─────────────────────────────────────────────┐   │
│                      │          API Server (HTTP)                   │   │
│  ┌──────────────┐   │  REST + WebSocket + SPA (Vue/React)          │   │
│  │  Heartbeat   │   │  Auth: API key (auto-gen or configured)      │   │
│  │  Service     │   │  CORS: localhost-only origins                │   │
│  └──────────────┘   └──────────────────────┬────────────────────┘   │
│                                            │                          │
│  ┌──────────────┐   ┌────────────────────▼─────────────────────┐   │
│  │ Integrations │   │           WebSocket Hub                   │   │
│  │  - Kanban    │   │  Broadcasts: inbound/outbound/system      │   │
│  │  - VSCode    │   │  events to connected dashboard clients    │   │
│  └──────────────┘   └──────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────────────────┘
            ▲                          ▲
            │                          │
  ┌─────────┴──────────┐    ┌──────────┴──────────┐
  │   ide-monitor       │    │  telegram-bots       │
  │   (Python daemon)   │    │  (Python Telegram)   │
  │                     │    │                      │
  │  POST /api/events   │    │  bots: dev, ops,     │
  │  WorkflowEvent v1   │    │  monitor, inbox      │
  └─────────────────────┘    │  kanban_server.py    │
                              └──────────────────────┘
```

---

## 3. Message Flow

### 3.1 Inbound Message (Telegram → Agent → Response)

```
User sends Telegram message
    │
    ▼
TelegramChannel.handleUpdate()
    │ IsAllowed(senderID) check
    ▼
BaseChannel.HandleMessage()
    │ assembles InboundMessage{Channel, SenderID, ChatID, Content, SessionKey}
    ▼
MessageBus.PublishInbound(msg)
    │ copies to all inboundSubs taps (fan-out)
    ▼
AgentLoop.Run() ← consuming from bus
    │
    ▼
processMessage(ctx, msg)
    │ if msg.Channel == "system" → processSystemMessage()
    │ else                       → runAgentLoop()
    ▼
runAgentLoop(opts)
    ├── updateToolContexts(channel, chatID)
    ├── sessions.GetHistory(sessionKey) + GetSummary()
    ├── contextBuilder.BuildMessages(history, summary, userMsg)
    │     ├── BuildSystemPrompt()
    │     │     ├── getIdentity() [time, runtime, tools list]
    │     │     ├── LoadBootstrapFiles() [AGENTS/SOUL/USER/IDENTITY.md]
    │     │     ├── skillsLoader.BuildSkillsSummary()
    │     │     └── memory.GetMemoryContext() [MEMORY.md + 3 days notes]
    │     └── append history + current message
    ├── sessions.AddMessage(sessionKey, "user", content)
    └── runLLMIteration(ctx, messages, opts)
            │
            ▼  [loop up to maxIterations=20]
        provider.Chat(ctx, messages, toolDefs, model, options)
            │
            ▼
        if response.ToolCalls → execute tools
            │ tools.ExecuteWithContext(ctx, name, args, channel, chatID)
            │ append assistantMsg + toolResultMsgs to messages
            │ save to session
            └── continue loop
            │
        if no ToolCalls → finalContent = response.Content
            │
            ▼
    sessions.AddMessage("assistant", finalContent)
    sessions.Save()
    maybeSummarize() [async if >20 msgs or >75% context window]
    │
    ▼
bus.PublishOutbound(OutboundMessage{Channel, ChatID, Content})
    │
    ▼
ChannelManager dispatch loop → TelegramChannel.Send()
    │
    ▼
User receives reply
```

### 3.2 Cron Job Execution

```
CronService timer fires
    │
    ▼
CronTool.ExecuteJob(ctx, job)
    │
    ▼
agentLoop.ProcessDirectWithChannel(ctx, job.Message, sessionKey, channel, chatID)
    │ (if job.Deliver=true → sends response via bus to target channel)
    │ (if job.Deliver=false → response is logged only)
    │
    ▼
[same runAgentLoop flow as above]
```

### 3.3 WorkflowEvent Flow (ide-monitor → picoclaw)

```
ide-monitor watches: Copilot logs, Antigravity brain, Git commits, filesystem
    │
    ▼
emitter.py: POST /api/events with WorkflowEvent JSON
    │
    ▼
handleWorkflowEvent()
    ├── unmarshals WorkflowEvent struct
    ├── wsHub.Broadcast(WSEvent) → all dashboard WebSocket clients
    ├── PublishSystem(SystemEvent) to bus → all systemSubs taps
    └── (if confidence ≥ threshold) → KanbanIntegration.CreateOrUpdateCard()
```

### 3.4 VS Code Extension Flow

```
VS Code extension
    │
    ├── GET /api/vscode/status         → status bar data (agent running?, tasks, model)
    ├── POST /api/vscode/todo {text}   → creates kanban card, returns card_id
    ├── POST /api/vscode/ask {question}→ routes to agent.ProcessDirect() → LLM response
    ├── POST /api/vscode/diff/preview  → codex.Validate(diff) → errors[]
    ├── POST /api/vscode/diff/apply    → codex.Apply(diff) → applies changes to files
    └── GET /api/vscode/tasks          → kanban tasks assigned to "vscode" executor
```

---

## 4. Data Persistence Model

```
~/.picoclaw/
├── config.json                      ← source of truth for all config
├── auth.json                        ← OAuth credentials (encrypted at rest: NOT YET)
└── workspace/
    ├── AGENTS.md                    ← agent behavior instructions (user edits)
    ├── SOUL.md                      ← agent personality
    ├── USER.md                      ← user profile (populated by agent)
    ├── IDENTITY.md                  ← agent identity block
    ├── HEARTBEAT.md                 ← periodic check agenda
    ├── memory/
    │   ├── MEMORY.md                ← long-term persistent memory (agent writes)
    │   ├── YYYYMM/
    │   │   └── YYYYMMDD.md          ← daily notes
    │   └── heartbeat.log            ← heartbeat service log
    ├── sessions/
    │   └── {channel}:{chatID}.json  ← conversation history + summary per session
    ├── cron/
    │   └── jobs.json                ← persistent cron job store
    └── skills/
        └── {skill-name}/
            └── SKILL.md             ← skill definition (read by contextBuilder)
```

---

## 5. Bounded Contexts (DDD Layers)

```
┌──────────────── Domain Layer (pkg/domain/) ───────────────────────┐
│  Pure types. No I/O. No imports from infra.                       │
│  Aggregates: Agent, Channel, Session, Skill, Workflow, Provider   │
│  Events: 30+ typed EventType constants                            │
│  Value Objects: ChannelType, ProviderType, MessageRole, EntityID  │
└───────────────────────────────────────────────────────────────────┘
                              ▲
┌──────────────── Application Layer (pkg/app/) ─────────────────────┐
│  Orchestrates domain objects. Depends on domain + infra.           │
│  Services: AgentService, ChannelService, SessionService,           │
│            SkillService, WorkflowService                           │
│  Container: DI container (app/container.go)                        │
│  NOTE: app/ services are defined but NOT YET wired in gateway.    │
│  The gateway currently uses direct package construction.           │
└───────────────────────────────────────────────────────────────────┘
                              ▲
┌──────────────── Infrastructure Layer (pkg/infrastructure/) ────────┐
│  EventBus adapter (wraps domain EventBus interface)                │
│  Persistence repository implementations                            │
└───────────────────────────────────────────────────────────────────┘
```

**Note:** The DDD layer (`pkg/domain/`, `pkg/app/`, `pkg/infrastructure/`) is architecturally defined but largely disconnected from the working gateway code. The gateway (`main.go > gatewayCmd()`) constructs packages directly without using the `app.Container`. This is the main architectural debt.

---

## 6. Security Model

| Layer | Mechanism | Status |
|-------|-----------|--------|
| API auth | API key (Bearer or X-API-Key header) | ✅ Implemented |
| API key generation | Auto-generated per session if not configured | ✅ Implemented |
| CORS | Localhost-only origins | ✅ Implemented |
| Channel auth | Per-channel allow-list (senderID) | ✅ Implemented |
| Shell tool | Deny-list regex patterns for dangerous commands | ✅ Implemented |
| File tool | Optional `fsAllowedDir` path restriction | ✅ Implemented (not enabled by default) |
| OAuth | PKCE flow for OpenAI | ✅ Implemented |
| Credential storage | `~/.picoclaw/auth.json` (plaintext JSON) | ⚠️ Not encrypted |
| API key in config | Stored in `config.json` (plaintext) | ⚠️ Not encrypted |
| WebSocket | Localhost-only origin check | ✅ Implemented |

---

## 7. Concurrency Model

```
Main goroutines in gateway mode:
├── ChannelManager.StartAll()
│   ├── TelegramChannel.Start() goroutine (polling)
│   ├── DiscordChannel.Start() goroutine (WebSocket)
│   ├── SlackChannel.Start() goroutine (WebSocket)
│   └── ... other channels
├── AgentLoop.Run(ctx) goroutine — consumes inbound bus messages sequentially
│   └── summarizeSession() — launched as separate goroutine when triggered
│       └── sync.Map (summarizing) prevents duplicate summarization
├── CronService.Start() goroutines — one ticker per job
├── HeartbeatService.Start() goroutine
├── IntegrationRegistry.StartAll() goroutines
├── api.Server.Start() goroutine
│   ├── WSHub.Run(ctx) goroutine
│   ├── EventBridge.Run(ctx) goroutine
│   └── http.Server.ListenAndServe() goroutine
└── SubagentManager goroutines (one per spawned subagent)
```

**Critical:** `AgentLoop.Run()` processes inbound messages **sequentially** (one at a time). This means concurrent messages from different channels queue up. For high-volume usage, this is a bottleneck. See DEVELOPMENT_PLAN.md.

---

## 8. LLM Provider Architecture

```
providers.CreateProvider(cfg) → LLMProvider
    ├── if Anthropic APIKey → NewClaudeProvider()
    ├── elif OpenAI APIKey  → NewCodexProvider()
    ├── elif OpenRouter     → NewHTTPProvider(openrouter.ai base, key)
    ├── elif Zhipu          → NewHTTPProvider(zhipu base, key)
    ├── elif Moonshot       → NewMoonshotProvider()
    ├── elif Gemini         → NewHTTPProvider(gemini base, key)
    ├── elif Groq           → NewHTTPProvider(groq base, key)
    └── elif VLLM           → NewHTTPProvider(vllm base, key)

Default fallback: HTTPProvider (if no key → error)
Default model: "glm-4.7" (Zhipu GLM 4.7)
```

All providers use fixed params in `runLLMIteration()`:
```go
"max_tokens":  8192,
"temperature": 0.7,
```
These are hardcoded — not configurable without code change. See BUGS_AND_ISSUES.md.

---

## 9. Skills System

```
workspace/skills/{name}/SKILL.md           ← user-installed
~/.picoclaw/skills/{name}/SKILL.md         ← global
./skills/{name}/SKILL.md                   ← built-in (repo)

Loader priority: workspace > global > builtin

ContextBuilder.BuildMessages()
    └── skillsLoader.BuildSkillsSummary()  ← list of skill names + first line
         (agent can read full content with read_file tool)
```

Built-in skills: `github`, `skill-creator`, `summarize`, `tmux`, `weather`

---

## 10. Python Subsystems Interactions with Go Backend

```
telegram-bots/
    ├── kanban_server.py  →  directly reads kanban.db (SQLite shared)
    ├── bots/*.py         →  executes picoclaw agent via subprocess or shared DB
    └── bots/executor.py  →  polls kanban.db for planned tasks, executes them

ide-monitor/
    └── emitter.py        →  POST /api/events  (HTTP to Go API)

Two kanban databases exist:
    - /var/lib/picoclaw/kanban.db  (Python side — KanbanStore default)
    - picoclaw/kanban.db           (Go side — pkg/integration/kanban)
    These are CURRENTLY NOT SYNCHRONIZED.  ← Known issue
```

---

## 11. Event Bridge (WebSocket Live Updates)

```
EventBridge.Run(ctx) goroutines:
├── tap inbound bus messages → broadcast WSEvent{type: "message.inbound", data: msg}
├── tap outbound bus messages → broadcast WSEvent{type: "message.outbound", data: msg}
└── tap system events → broadcast WSEvent{type: event.Type, data: event.Data}

WSHub.Run(ctx):
├── register/unregister clients
└── broadcast loop: fan-out WSEvent JSON to all connected clients

Client lifecycle (WSClient):
├── readPump: reads client messages (ping/pong, disconnect detection)
└── writePump: writes events to client with 10s write deadline
```

---

## 12. Auth Flow (OAuth2/PKCE)

```
picoclaw auth login --provider openai

→ auth.LoginBrowser(cfg)
    ├── generate PKCE challenge + verifier
    ├── build auth URL
    ├── open browser (or print URL for --device-code)
    ├── start local HTTP server to catch callback (port 18791)
    ├── exchange code for token
    └── auth.SetCredential("openai", cred) → ~/.picoclaw/auth.json

On gateway startup, for API calls:
→ provider.Chat() uses token from auth store if AuthMethod == "oauth"
```

---

## 13. Migration Tool

```
picoclaw migrate [--dry-run] [--refresh] [--config-only] [--workspace-only]

OpenClaw home: ~/.openclaw/
PicoClaw home: ~/.picoclaw/

Config mapping:
    openrouter_api_key    → providers.openrouter.api_key
    anthropic_api_key     → providers.anthropic.api_key
    openai_api_key        → providers.openai.api_key
    workspace             → agents.defaults.workspace
    model                 → agents.defaults.model
    max_tokens            → agents.defaults.max_tokens
    temperature           → agents.defaults.temperature
    max_tool_iterations   → agents.defaults.max_tool_iterations
    telegram_token        → channels.telegram.token
    telegram_allow_from   → channels.telegram.allow_from

Workspace files copied: AGENTS.md, SOUL.md, USER.md, TOOLS.md (if exists),
                        memory/MEMORY.md (if exists)
```
