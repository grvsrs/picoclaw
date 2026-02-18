# Telegram Bot Map — Definitive Truth

Last verified: 2026-02-18

## Bot Registry (from Telegram API — no guesses)

| Username | Bot ID | Display Name | Token Env Var | Role |
|---|---|---|---|---|
| **@gvneobot** | `8553334823` | PicoClaw Inbox bot | `PICOCLAW_INBOX_BOT_TOKEN` | capture layer |
| **@gvagentsmithbot** | `8212270122` | make bot | `PICOCLAW_OPS_BOT_TOKEN` | ops (unused) |
| **@gvinboxbot** | `8383635095` | PicoClaw Inbox | `PICOCLAW_MONITOR_BOT_TOKEN` | reserved for Go gateway |

## Running Processes (at boot, via systemd user services)

| Service | Binary | PID on last check | What it does |
|---|---|---|---|
| `picoclaw-inbox` | `telegram-bots/.venv/bin/python bots/inbox_bot.py` | 1056 | Runs **@gvneobot** — handles `/save /task /mode /board /inbox` |
| `picoclaw-mode-watchdog` | `/bin/bash mode_watchdog.sh` | 1059 | Polls current mode (local/remote), updates state |
| `picoclaw-kanban` | `telegram-bots/.venv/bin/python kanban_server.py` | 1285 | Kanban HTTP API on `127.0.0.1:3000` |

## picoclaw Go Gateway (manually managed)

Running: `./picoclaw gateway` (starts on demand, not a systemd service)

- API: `http://127.0.0.1:18790`
- Auth: bearer key in `PICOCLAW_API_SECRET` env var
- Telegram: **DISABLED** (`channels.telegram.enabled = false` in `config.json`)
- Telegram token stored but disabled: `PICOCLAW_MONITOR_BOT_TOKEN` = **@gvinboxbot**

## Token Configuration

### `telegram-bots/.env` (loaded by all three systemd services)
```
PICOCLAW_INBOX_BOT_TOKEN=8553334823:...   # → @gvneobot (ACTIVE)
PICOCLAW_OPS_BOT_TOKEN=8212270122:...     # → @gvagentsmithbot (idle)
PICOCLAW_MONITOR_BOT_TOKEN=8383635095:... # → @gvinboxbot (idle)
PICOCLAW_DB=/home/g/picoclaw/kanban.db
PICOCLAW_API_SECRET=f52aa8...
```

### `picoclaw/.env` (loaded by Go gateway / Makefile)
```
PICOCLAW_INBOX_BOT_TOKEN=8553334823:...   # same value, kept in sync
PICOCLAW_API_SECRET=f52aa8...
MOONSHOT_API_KEY=sk-kimi-...
GLM-5_API_KEY=nvapi-...
```

## The Bug That Was Fixed

**Before:** `config.json` had `channels.telegram.enabled = true` with token `8553334823`
(@gvneobot). This meant **both** `inbox_bot.py` AND the Go gateway were polling the same
Telegram token simultaneously. Telegram delivers each update to only one poller, so messages
were randomly lost/stolen between the two processes.

**After:** `config.json` sets `channels.telegram.enabled = false`. Only `inbox_bot.py`
polls @gvneobot. The stored token in config.json is now **@gvinboxbot** (unused, ready
for when the Go agent gateway needs its own bot).

## Authorized Users

User ID `9411488118` = admin (in all `allowed_users` lists)
User ID `7527847638` = secondary user (in `inbox_bot` allowed_users only)

## How to Enable Go Gateway Telegram (Future)

1. In `config.json`: set `channels.telegram.enabled = true`
2. Token is already set to `PICOCLAW_MONITOR_BOT_TOKEN` (`8383635095`) = **@gvinboxbot**
3. Make sure `PICOCLAW_MONITOR_BOT_TOKEN` is exported in the environment before starting picoclaw
4. This gives the Go LLM agent gateway its own dedicated bot, separate from inbox_bot

## Quick Verification Commands

```bash
# Which token is which username:
curl -s "https://api.telegram.org/bot<TOKEN>/getMe"

# What's running:
ps aux | grep -E "inbox_bot|picoclaw|kanban" | grep -v grep

# Go gateway health:
curl -s http://127.0.0.1:18790/api/health
```
