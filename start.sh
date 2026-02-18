#!/bin/bash
# Quick start guide for PicoClaw with Moonshot + .env file

set -e

echo "🚀 PicoClaw Moonshot Quick Start"
echo "=================================="
echo ""

cd "$(dirname "$0")"

# Check .env exists
if [ ! -f .env ]; then
    echo "❌ .env file not found!"
    echo ""
    echo "Create one with:"
    echo "  cat > .env << 'EOF'"
    echo "  MOONSHOT_API_KEY=sk-kimi-xxx"
    echo "  EOF"
    exit 1
fi

echo "✓ Found .env file"

# Check if picoclaw binary exists
if [ ! -f picoclaw ]; then
    echo ""
    echo "⚙️ Building picoclaw binary..."
    go build ./cmd/picoclaw/
    echo "✓ Build complete"
fi

echo ""
echo "📂 Loading configuration..."

# Load .env into environment
set -a
source .env
set +a

echo "✓ Environment variables loaded:"
echo "  - MOONSHOT_API_KEY: ${MOONSHOT_API_KEY:0:10}..."

echo ""
echo "Select startup mode:"
echo ""
echo "  1) Dashboard only (no channels)"
echo "  2) With Telegram bot"
echo "  3) Custom config"
echo ""

read -p "Choose [1-3] (default: 1): " choice
choice=${choice:-1}

case $choice in
    1)
        echo "▶️ Starting: picoclaw gateway (Moonshot LLM)"
        exec ./picoclaw gateway
        ;;
    2)
        echo "Setting up Telegram... (you need TELEGRAM_BOT_TOKEN)"
        if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
            read -p "Enter Telegram bot token: " token
            export TELEGRAM_BOT_TOKEN="$token"
        fi
        echo "▶️ Starting: picoclaw gateway with Telegram"
        exec ./picoclaw gateway
        ;;
    3)
        read -p "Enter PicoClaw command: " cmd
        echo "▶️ Starting: picoclaw $cmd"
        exec ./picoclaw $cmd
        ;;
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac
