# PicoClaw & Moonshot: Quick Reference

## Your Setup Status

✅ **Moonshot API Key**: Stored in `.env`  
✅ **LLM Integration**: Ready  
✅ **Config**: Multiple options available

---

## Will `.env` Work?

❌ **NOT automatically.** Go gateway doesn't auto-load `.env`

### ✅ Solutions:

**Quick (recommended for dev):**
```bash
bash run_with_env.sh  # or: bash start.sh
```

**Permanent (recommended for production):**
```bash
# Copy .env values to ~/.picoclaw/config.json
cat > ~/.picoclaw/config.json << 'EOF'
{
  "agents": { "defaults": { "model": "moonshot-v1-32k" } },
  "providers": { "moonshot": { "api_key": "sk-kimi-xxx" } }
}
EOF

picoclaw gateway
```

**Manual export:**
```bash
export PICOCLAW_PROVIDERS_MOONSHOT_API_KEY="sk-kimi-xxx"
picoclaw gateway
```

---

## All Configured LLM Providers

| # | Provider | Status | Default Model | Free? |
|---|----------|--------|---|---|
| 1️⃣ | **Moonshot** | ✅ Set up | `moonshot-v1-32k` | ¥ Affordable |
| 2️⃣ | Anthropic Claude | ⏳ Needs API key | `claude-sonnet-4-5` | Paid |
| 3️⃣ | OpenAI GPT | ⏳ Needs API key | `gpt-4o` | Paid |
| 4️⃣ | OpenRouter | ⏳ Needs API key | 100+ models | Variable |
| 5️⃣ | Groq | ⏳ Needs API key | `llama-3.1-70b` | Free tier |
| 6️⃣ | Zhipu GLM | ⏳ Needs API key | `glm-4.7` | Free tier |
| 7️⃣ | Google Gemini | ⏳ Needs API key | `gemini-2.0-flash` | Free tier |
| 8️⃣ | VLLM (local) | ⏳ Self-hosted | Any | Free |

**Add more:** Edit `~/.picoclaw/config.json`

---

## WhatsApp Bot Setup

### **Telegram** (Recommended ✅)

```
✅ Free
✅ Simple setup (1 command)
✅ Native PicoClaw support
✅ No business account needed
⏱️ 5 minutes to full bot
```

**Setup:**
```bash
# 1. Message @BotFather on Telegram
# 2. Create bot → copy token
# 3. Set in config:
export TELEGRAM_BOT_TOKEN="123:ABC..."
picoclaw gateway
```

### **WhatsApp** (Complex ❌)

```
❌ Paid (₹800-2000/month business account)
❌ Complex setup (requires bridge server)
❌ Needs infrastructure
⏱️ Days to full bot
```

**Architecture needed:**
```
WhatsApp → Meta Cloud API → Bridge Server → PicoClaw
          (third-party)    (self-hosted)
```

**Why harder?**
- No official bot API
- Requires WhatsApp Business Account
- Must self-host webhook bridge
- Rate limits apply
- Message delivery guarantees

---

## Recommended Path

### 🎯 Phase 1: Test Moonshot (10 min)
```bash
bash start.sh
# Opens dashboard: http://127.0.0.1:18790
# Uses Moonshot LLM for AI responses
```

### 🎯 Phase 2: Add Telegram Bot (15 min)
```bash
# 1. Get Telegram bot token from @BotFather
# 2. Add to ~/.picoclaw/config.json:
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_TOKEN",
      "allow_from": ["YOUR_USER_ID"]
    }
  }
}
# 3. Restart: bash start.sh
# 4. Text your bot on Telegram
```

### 🎯 Phase 3: More Providers (optional)
```bash
# Add OpenAI, Claude, etc. to config.json
# PicoClaw auto-detects via model name
```

### 🎯 Phase 4: Deploy (production)
```bash
# Move to ~/.picoclaw/config.json (secure location)
# Run as systemd service
# See SETUP_GUIDE.md for production deployment
```

---

## Files Created for You

| File | Purpose |
|------|---------|
| `.env` | Your secrets (Moonshot API key) |
| `run_with_env.sh` | Load .env → run picoclaw |
| `start.sh` | Interactive startup menu |
| `config.moonshot.json` | Moonshot config example |
| `config.telegram.json` | Moonshot + Telegram config |
| `SETUP_GUIDE.md` | Detailed setup documentation |
| `MOONSHOT_SETUP.md` | Moonshot-specific guide |

---

## Troubleshooting

**Error: "no API key configured"**
```bash
# Make sure you sourced .env or config is set:
export PICOCLAW_PROVIDERS_MOONSHOT_API_KEY="sk-kimi-xxx"
picoclaw gateway
```

**Error: "401 Unauthorized"**
```bash
# Check your Moonshot API key is valid:
# https://platform.moonshot.cn/
```

**Dashboard not accessible**
```bash
# Check port 18790 is not blocked:
netstat -an | grep 18790
# Or try different port in config.json
```

---

## Next Steps

1. **Start with Moonshot alone:**
   ```bash
   bash start.sh
   ```

2. **Add Telegram (optional):**
   - Message @BotFather for token
   - Update config.json
   - Restart

3. **Explore the dashboard:**
   - http://127.0.0.1:18790
   - Check LLM models, tools, skills

4. **Read docs:**
   - `SETUP_GUIDE.md` — Full setup steps
   - `MOONSHOT_SETUP.md` — Moonshot specifics

**You're all set! 🚀**
