# PicoClaw Setup Guide

## 1. Will Your `.env` File Work?

**Short answer: NOT automatically.** The Go gateway doesn't load `.env` files by default.

### Options to make it work:

**Option A: Source .env before running (RECOMMENDED)**
```bash
cd /home/g/picoclaw
bash run_with_env.sh gateway
# This sources .env and exports MOONSHOT_API_KEY to environment
```

**Option B: Export manually in terminal**
```bash
export PICOCLAW_PROVIDERS_MOONSHOT_API_KEY="sk-kimi-P7s71J5XcfPftNnNHzUQ5R6rjedCw1OO4xth0S2UX7gzykP6EzdH8PqohoCEOaz6"
picoclaw gateway
```

**Option C: Use config.json (PERSISTENT)**
```bash
cat > ~/.picoclaw/config.json << 'EOF'
{
  "agents": {
    "defaults": {
      "model": "moonshot-v1-32k"
    }
  },
  "providers": {
    "moonshot": {
      "api_key": "sk-kimi-P7s71J5XcfPftNnNHzUQ5R6rjedCw1OO4xth0S2UX7gzykP6EzdH8PqohoCEOaz6"
    }
  }
}
EOF
picoclaw gateway
```

⚠️ **WARNING**: Don't commit `.env` or `config.json` with API keys to git! Add to `.gitignore`:
```bash
echo ".env" >> .gitignore
echo "~/.picoclaw/config.json" >> .gitignore
```

---

## 2. Configured LLM Providers

PicoClaw supports **8 LLM providers**:

| Provider | Model Examples | Cost | Setup |
|----------|---|---|---|
| **Moonshot** 🚀 | `moonshot-v1-32k`, `v1-128k` | ¥ Affordable | API key |
| **Anthropic Claude** | `claude-sonnet-4-5` | Premium | API key |
| **OpenAI** | `gpt-4o`, `gpt-4` | Premium | API key |
| **OpenRouter** | 100+ models | Variable | API key |
| **Groq** | `llama-3.1-70b` | Free tier | API key |
| **Zhipu (GLM)** | `glm-4.7`, ~~`glm-4-long`~~ | Free tier | API key |
| **Google Gemini** | `gemini-2.0-flash` | Free tier | API key |
| **VLLM (local)** | Any local model | Free | Self-hosted |

### How to Add Multiple Providers

Edit `~/.picoclaw/config.json`:

```json
{
  "agents": {
    "defaults": {
      "model": "moonshot-v1-32k"
    }
  },
  "providers": {
    "moonshot": {
      "api_key": "sk-kimi-xxx"
    },
    "openai": {
      "api_key": "sk-proj-xxx"
    },
    "anthropic": {
      "api_key": "sk-ant-xxx"
    }
  }
}
```

**PicoClaw will auto-detect which provider to use based on model name.**

---

## 3. Building a WhatsApp Bot

### Quick Comparison: Telegram vs WhatsApp

| Feature | Telegram | WhatsApp |
|---------|----------|----------|
| **Bot Setup** | ✅ Simple (BotFather) | ❌ Complex (business account) |
| **Detection** | ✅ Auto (token-based) | ❌ Manual (bridge URL) |
| **Cost** | ✅ Free | ❌ Paid (WhatsApp Biz account) |
| **Development** | ✅ BotAPI | ❌ WhatsApp Cloud API |
| **PicoClaw Support** | ✅ Native | ⚠️ Via Bridge |

### Telegram Bot Setup (easy, recommended for testing)

```bash
# 1. Find @BotFather on Telegram
# 2. Create bot → get token → set webhook URL
# 3. Add to config:
cat > ~/.picoclaw/config.json << 'EOF'
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN_HERE",
      "allow_from": ["YOUR_USER_ID"]
    }
  }
}
EOF

picoclaw gateway
```

### WhatsApp Bot Setup (complex, requires business account)

**Prerequisites:**
- WhatsApp Business Account
- Meta Developer account
- WhatsApp Cloud API access
- A self-hosted bridge server

**Architecture:**
```
WhatsApp ↔ Meta Cloud API ↔ Bridge Server (Websocket) ↔ PicoClaw
```

**Configuration:**

1. **Deploy a bridge** (e.g., WhatsApp Web API bridge):
```bash
# You need to self-host a bridge server
# Example: https://github.com/tulir/whatsmeow
git clone https://github.com/tulir/whatsmeow
cd whatsmeow
go build ./cmd/whatsmeow
./whatsmeow
# Bridge runs on ws://localhost:8080
```

2. **Configure PicoClaw:**
```json
{
  "channels": {
    "whatsapp": {
      "enabled": true,
      "bridge_url": "ws://localhost:8080",
      "allow_from": ["1234567890"]
    }
  }
}
```

3. **Handle incoming messages:**
   - Messages via bridge → PicoClaw agent → uses Moonshot LLM → responds back via WebSocket

### Why WhatsApp is Harder

1. **No official bot API** — must use Meta's Cloud API
2. **Requires business account** — not free (₹800-2000/month)
3. **Webhook setup** — more infrastructure
4. **Bridge dependency** — relies on third-party bridge server
5. **Message limits** — rate-limited by WhatsApp

### Recommended: Start with Telegram

If you're building a bot to test PicoClaw's AI + Moonshot integration, **Telegram is the way to go**:
- ✅ Free, instant setup
- ✅ Full bot API
- ✅ PicoClaw native support
- ✅ Perfect for experimentation

```bash
# Quick Telegram setup:
picoclaw gateway  # Uses Moonshot by default
# Then text your bot on Telegram
```

---

## Quick Start (3 steps)

```bash
# 1. Build
cd /home/g/picoclaw
go build ./cmd/picoclaw/

# 2. Set up Moonshot
bash run_with_env.sh

# 3. Access dashboard
# Open: http://127.0.0.1:18790
```

**You're ready to use Moonshot! 🚀**

