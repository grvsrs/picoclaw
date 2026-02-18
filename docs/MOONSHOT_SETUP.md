# Moonshot LLM Provider Setup

Moonshot is a Chinese LLM provider offering high-quality language models. PicoClaw now supports Moonshot as an LLM backend.

## Quick Start

### 1. Get Your API Key

1. Visit [https://platform.moonshot.cn/](https://platform.moonshot.cn/)
2. Sign up or log in with your account
3. Navigate to **API Keys** section
4. Create a new API key
5. Copy the key (format: `sk-...`)

### 2. Configure PicoClaw

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
      "api_key": "sk-YOUR_API_KEY_HERE"
    }
  }
}
```

**Or set via environment variable:**

```bash
export PICOCLAW_PROVIDERS_MOONSHOT_API_KEY="sk-your-api-key-here"
```

### 3. Run PicoClaw

```bash
picoclaw gateway
# Dashboard → http://127.0.0.1:18790
```

## Available Models

Moonshot provides several models:

- `moonshot-v1-8k` — 8K context window, faster
- `moonshot-v1-32k` — 32K context window (recommended for most tasks)
- `moonshot-v1-128k` — 128K context window, for long documents

Choose the model that fits your needs:

```json
{
  "agents": {
    "defaults": {
      "model": "moonshot-v1-128k"
    }
  }
}
```

## Pricing & Quotas

Check your API quotas and billing at [https://platform.moonshot.cn/billing](https://platform.moonshot.cn/billing).

## Features

✅ Full tool/function calling support  
✅ Streaming responses  
✅ Token counting  
✅ System prompts  
✅ Vision capabilities (in newer models)

## Troubleshooting

### Error: "no API key configured"

Make sure the `api_key` is set in config.json or the `PICOCLAW_PROVIDERS_MOONSHOT_API_KEY` environment variable is exported.

### Error: "401 Unauthorized"

Your API key may be invalid, expired, or used up its quota. Check your credentials at [https://platform.moonshot.cn/](https://platform.moonshot.cn/).

### Error: "rate limited"

Moonshot enforces rate limits. Wait a moment and retry, or upgrade your plan for higher limits.

## Support

- Moonshot Docs: https://platform.moonshot.cn/docs
- PicoClaw Issues: https://github.com/sipeed/picoclaw

