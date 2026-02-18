# PicoClaw Bot Handlers

Android → Telegram → Linux execution pipeline.  
Mobile sovereignty over Linux execution — deterministic, auditable, zero-trust.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        CONTROL PLANE                            │
│                                                                 │
│  Android / Telegram App                                         │
│       │                                                         │
│       ▼                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │   dev_bot     │  │   ops_bot    │  │   monitor_bot        │  │
│  │  /run_tests   │  │  /status     │  │  /health             │  │
│  │  /deploy      │  │  /logs       │  │  /recent             │  │
│  │  /scaffold    │  │  /restart    │  │  /alerts_on/off      │  │
│  │  /git_status  │  │  /disk_usage │  │  push notifications  │  │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘  │
│         │                 │                      │              │
│         └────────────┬────┘──────────────────────┘              │
│                      ▼                                          │
│            ┌──────────────────┐                                 │
│            │    bot_base.py   │  ← auth, validate, write task   │
│            │  (shared logic)  │                                 │
│            └────────┬─────────┘                                 │
└─────────────────────┼───────────────────────────────────────────┘
                      │  writes task YAML
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                     EXECUTION PLANE                             │
│                                                                 │
│  /workspace/tasks/{dev,ops,monitor}/task-*.yaml                 │
│                      │                                          │
│                      ▼                                          │
│            ┌──────────────────┐                                 │
│            │   executor.py    │  ← inotify watcher              │
│            │  (systemd svc)   │     validates again (defense)    │
│            └────────┬─────────┘     hardcoded dispatch table     │
│                     │                no shell=True               │
│                     ▼                                           │
│            subprocess.run()                                     │
│                     │                                           │
│                     ▼                                           │
│            writes result back to task YAML                      │
│                     │                                           │
│                     ▼                                           │
│            bot polls (0.5s→3s backoff) → Telegram reply          │
└─────────────────────────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                     AUDIT PLANE                                 │
│                                                                 │
│  /var/log/picoclaw/audit.jsonl                                  │
│  Every command, every user, every result — structured JSON      │
│  Both bot and executor write here independently                 │
└─────────────────────────────────────────────────────────────────┘
```

## Files

```
telegram-bots/
├── config/
│   └── picoclaw.yaml       # All bot config, users, commands
├── schemas/
│   └── task.yaml            # Task file format documentation
├── bots/
│   ├── bot_base.py          # Shared auth, validation, task writing, result polling
│   ├── dev_bot.py           # /run_tests /deploy /scaffold /git_status
│   ├── ops_bot.py           # /status /logs /stop /restart_service /disk_usage /process_info
│   ├── monitor_bot.py       # /health /recent /alerts_on /alerts_off + push notifications
│   └── executor.py          # inotify watcher + deterministic command runner
├── tests/
│   ├── conftest.py
│   ├── test_bot_base.py     # Validation, task writer, audit, config tests
│   └── test_executor.py     # Command dispatch, process_task, input validation tests
├── setup.sh                 # Ubuntu 22.04 install script
├── .env.example             # Bot token template
├── requirements.txt         # Python dependencies
├── Makefile                 # install, test, lint, setup targets
└── .gitignore
```

## Quickstart

### 1. Create bots on Telegram

Message [@BotFather](https://t.me/BotFather):
```
/newbot
```
Create three bots: dev_bot, ops_bot, monitor_bot. Save the tokens.

### 2. Find your Telegram user ID

Message [@userinfobot](https://t.me/userinfobot). It replies with your numeric user ID.

### 3. Configure

```bash
cd telegram-bots

# Copy and edit config
vi config/picoclaw.yaml
# → Replace YOUR_TELEGRAM_ID with your numeric ID

# Copy and fill in tokens
cp .env.example .env
chmod 600 .env
vi .env
```

### 4. Install

```bash
# Full setup (systemd services, directories, sudoers)
chmod +x setup.sh
./setup.sh

# Or just install Python deps
make install
```

### 5. Test

```bash
make test    # Run all tests
make check   # Lint + test
```

### 6. Start

```bash
# Reload systemd
systemctl --user daemon-reload
sudo systemctl daemon-reload

# Start everything
systemctl --user enable --now picoclaw-dev picoclaw-ops picoclaw-monitor
sudo systemctl enable --now picoclaw-executor

# Or run directly for development
.venv/bin/python bots/dev_bot.py
```

### 7. Use

Send `/help` to your dev bot from Telegram:
```
/run_tests myproject
/run_tests myproject suite=integration
/deploy myproject env=staging
/deploy myproject env=production   → asks for confirmation
/git_status myproject
```

## Security Model

| Layer | Mechanism |
|-------|-----------|
| **Identity** | Numeric Telegram user IDs only (not usernames, which can change) |
| **Authorization** | Per-bot allowlist in config |
| **Input validation** | Bot validates params → executor validates again (defense in depth) |
| **Command dispatch** | Hardcoded `COMMAND_TABLE` dict — no dynamic execution |
| **Shell safety** | No `shell=True` anywhere, no f-string injection |
| **Path safety** | `safe_path()` prevents traversal (resolves + asserts within base) |
| **Service allowlist** | `restart_service` only allows hardcoded service names |
| **Confirmation** | Dangerous commands require explicit `/confirm` |
| **Sudoers** | Least-privilege: only specific `systemctl restart` commands |
| **Audit** | Every action logged to structured JSONL (bot + executor independently) |

## Task State Machine

```
pending → running → complete
                 ↘ failed
                 ↘ timeout
pending → rejected              (unknown command)
pending → awaiting_confirmation
                 ↘ pending      (after /confirm)
                 ↘ cancelled    (after /cancel)
```

## Telegram Rate Limits

Log streaming in `ops_bot.py` is rate-limited:
- Max 1 message per `LOG_SEND_INTERVAL` (2s) per user
- Buffers output up to `LOG_CHUNK_SIZE` (3000 chars) before sending
- Hard cap at `LOG_MAX_LINES` (500 lines) per stream session

Push notifications in `monitor_bot.py`:
- Polls audit log every 5 seconds (not per-line)
- Only notifies on `failed` and `timeout` statuses
- Uses file seek position to avoid re-reading

## Adding a New Command

1. Add to `config/picoclaw.yaml` under the relevant bot's `commands:`
2. Add a handler method in the bot class (e.g., `cmd_mycommand`)
3. Register it in `register_handlers()`
4. Add the implementation function to `executor.py`
5. Add it to `COMMAND_TABLE`
6. Add input validation (defense in depth)
7. Run tests: `make test`
8. Restart both the bot and executor services

## Logs

```bash
# Bot logs (user services)
journalctl --user -u picoclaw-dev -f
journalctl --user -u picoclaw-ops -f
journalctl --user -u picoclaw-monitor -f

# Executor logs (system service)
journalctl -u picoclaw-executor -f

# Audit log (every command, every user, every result)
tail -f /var/log/picoclaw/audit.jsonl | python3 -m json.tool
```

## Design Principles

1. **Mobile sovereignty** — Android is control plane, Linux is executor, AI is constrained tool
2. **Deterministic execution** — No dynamic dispatch, no eval, no shell injection surface
3. **Defense in depth** — Dual validation (bot + executor), path traversal guards, service allowlists
4. **Full auditability** — Every action generates structured audit entries from both sides
5. **Separation of concerns** — Bots handle Telegram; executor handles Linux; task files are the contract
6. **Graceful degradation** — inotify preferred, polling fallback; /var/log preferred, local fallback
7. **Rate-limit awareness** — Buffered log streaming, backoff polling, batched notifications
