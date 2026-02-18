#!/bin/bash
# Quick setup script for Moonshot LLM integration with PicoClaw
# Usage: bash setup_moonshot.sh <your-moonshot-api-key>

if [ -z "$1" ]; then
    echo "Usage: bash setup_moonshot.sh <your-moonshot-api-key>"
    echo ""
    echo "Get your Moonshot API key from: https://platform.moonshot.cn/"
    exit 1
fi

API_KEY="$1"
CONFIG_DIR="$HOME/.picoclaw"
CONFIG_FILE="$CONFIG_DIR/config.json"

echo "Setting up Moonshot provider for PicoClaw..."
echo ""

# Create directory if not exist
mkdir -p "$CONFIG_DIR"

# If config doesn't exist, create it with Moonshot defaults
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Creating new config at $CONFIG_FILE with Moonshot..."
    
    cat > "$CONFIG_FILE" << EOF
{
  "agents": {
    "defaults": {
      "workspace": "~/.picoclaw/workspace",
      "model": "moonshot-v1-32k",
      "max_tokens": 8192,
      "temperature": 0.7,
      "max_tool_iterations": 20
    }
  },
  "providers": {
    "moonshot": {
      "api_key": "$API_KEY",
      "api_base": ""
    }
  },
  "channels": {
    "telegram": { "enabled": false, "token": "" }
  },
  "gateway": {
    "host": "0.0.0.0",
    "port": 18790
  }
}
EOF
    echo "✓ Config created at $CONFIG_FILE"
else
    echo "Config already exists at $CONFIG_FILE"
    echo "Please update the moonshot API key manually or run:"
    echo ""
    echo "  export PICOCLAW_PROVIDERS_MOONSHOT_API_KEY='$API_KEY'"
fi

echo ""
echo "✓ Moonshot API Key: $API_KEY"
echo ""
echo "Next steps:"
echo "  1. Start PicoClaw:  picoclaw gateway"
echo "  2. Open dashboard: http://127.0.0.1:18790"
echo ""
echo "Documentation: see MOONSHOT_SETUP.md"
