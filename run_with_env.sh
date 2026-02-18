#!/bin/bash
# run_with_env.sh - Load .env and start PicoClaw
# Usage: bash run_with_env.sh [command]

ENV_FILE="$(dirname "$0")/.env"

if [ -f "$ENV_FILE" ]; then
    echo "📂 Loading environment from $ENV_FILE..."
    set -a  # Export all variables
    source "$ENV_FILE"
    set +a  # Stop exporting
    echo "✓ Environment loaded"
else
    echo "⚠ .env file not found at $ENV_FILE"
    echo "  Create it: cp config.example.json ~/.picoclaw/config.json"
fi

# Run the command (default: gateway)
COMMAND="${1:-gateway}"
echo "▶ Starting: picoclaw $COMMAND"
exec picoclaw "$COMMAND"
