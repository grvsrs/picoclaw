#!/bin/bash
# PicoClaw Setup Script for Ubuntu 22.04
# ────────────────────────────────────────
# Run as the user who will own the bot processes (NOT root).
# The executor systemd service uses sudo for specific commands —
# configure /etc/sudoers accordingly.
#
# Usage:
#   chmod +x setup.sh
#   ./setup.sh

set -euo pipefail

PICOCLAW_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
USER="$(whoami)"
VENV_DIR="$PICOCLAW_DIR/.venv"

echo "=== PicoClaw Bot Handlers Setup ==="
echo "Directory: $PICOCLAW_DIR"
echo "User: $USER"
echo ""

# ── [1/5] Python dependencies ──
echo "[1/5] Installing Python dependencies..."
python3 -m venv "$VENV_DIR"
"$VENV_DIR/bin/pip" install --upgrade pip --quiet
"$VENV_DIR/bin/pip" install --quiet -r "$PICOCLAW_DIR/requirements.txt"
echo "      Done."

# ── [2/5] Workspace directories ──
echo "[2/5] Creating workspace directories..."
mkdir -p \
    /workspace/tasks/{dev,ops,monitor} \
    /workspace/vscode/{intents,status} \
    /workspace/patches \
    /workspace/ai-tasks

# Log directories
if sudo mkdir -p /var/log/picoclaw 2>/dev/null; then
    sudo chown "$USER:$USER" /var/log/picoclaw
else
    echo "      Note: Cannot create /var/log/picoclaw, using ./logs instead"
    mkdir -p "$PICOCLAW_DIR/logs"
fi
echo "      Done."

# ── [3/5] Config check ──
echo "[3/5] Checking config..."
CONFIG="$PICOCLAW_DIR/config/picoclaw.yaml"
if [ ! -f "$CONFIG" ]; then
    echo "      ERROR: Config not found at $CONFIG"
    exit 1
fi

if grep -q "YOUR_TELEGRAM_ID" "$CONFIG"; then
    echo ""
    echo "  ⚠️  You must edit config/picoclaw.yaml:"
    echo "     Replace YOUR_TELEGRAM_ID with your actual Telegram user ID."
    echo "     Find yours by messaging @userinfobot on Telegram."
    echo ""
fi
echo "      Config found."

# ── [4/5] Environment variables check ──
echo "[4/5] Checking environment variables..."
MISSING=0
for var in PICOCLAW_DEV_BOT_TOKEN PICOCLAW_OPS_BOT_TOKEN PICOCLAW_MONITOR_BOT_TOKEN; do
    if [ -z "${!var:-}" ]; then
        echo "      ⚠️  Missing: $var"
        MISSING=$((MISSING + 1))
    else
        echo "      ✅ $var is set"
    fi
done

if [ $MISSING -gt 0 ]; then
    echo ""
    echo "  Set missing tokens before starting bots:"
    echo "    export PICOCLAW_DEV_BOT_TOKEN=your_token"
    echo "    export PICOCLAW_OPS_BOT_TOKEN=your_token"
    echo "    export PICOCLAW_MONITOR_BOT_TOKEN=your_token"
    echo ""
    echo "  Get tokens from @BotFather on Telegram."
    echo "  Add them to .env (never commit to git)."
fi

# ── [5/5] Systemd services ──
echo "[5/5] Installing systemd services..."
SYSTEMD_DIR="$HOME/.config/systemd/user"
mkdir -p "$SYSTEMD_DIR"

write_user_service() {
    local name="$1"
    local script="$2"
    cat > "$SYSTEMD_DIR/picoclaw-${name}.service" <<EOF
[Unit]
Description=PicoClaw ${name^} Bot
After=network.target
Wants=network.target

[Service]
Type=simple
WorkingDirectory=$PICOCLAW_DIR
ExecStart=$VENV_DIR/bin/python $PICOCLAW_DIR/bots/${script}
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

# Load tokens from secrets file (chmod 600)
EnvironmentFile=-$PICOCLAW_DIR/.env

[Install]
WantedBy=default.target
EOF
    echo "      Wrote picoclaw-${name}.service (user)"
}

write_user_service "dev" "dev_bot.py"
write_user_service "ops" "ops_bot.py"
write_user_service "monitor" "monitor_bot.py"

# Executor runs as system service (needs sudo for systemctl restart)
if sudo tee /etc/systemd/system/picoclaw-executor.service > /dev/null <<EOF
[Unit]
Description=PicoClaw Executor
After=multi-user.target
Wants=multi-user.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$PICOCLAW_DIR
ExecStart=$VENV_DIR/bin/python $PICOCLAW_DIR/bots/executor.py
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
then
    echo "      Wrote picoclaw-executor.service (system)"
else
    echo "      ⚠️  Could not write system service (need sudo)"
fi

# ── Sudoers for executor ──
SUDOERS_LINE="$USER ALL=(ALL) NOPASSWD: /bin/systemctl restart nginx, /bin/systemctl restart postgresql, /bin/systemctl restart docker, /bin/systemctl restart redis"
SUDOERS_FILE="/etc/sudoers.d/picoclaw"
if sudo bash -c "echo '$SUDOERS_LINE' > $SUDOERS_FILE && chmod 440 $SUDOERS_FILE" 2>/dev/null; then
    echo "      Wrote $SUDOERS_FILE"
else
    echo "      ⚠️  Could not write sudoers. Add manually:"
    echo "      $SUDOERS_LINE"
fi

# ── .env template ──
ENV_FILE="$PICOCLAW_DIR/.env"
if [ ! -f "$ENV_FILE" ]; then
    cp "$PICOCLAW_DIR/.env.example" "$ENV_FILE" 2>/dev/null || \
    cat > "$ENV_FILE" <<'EOF'
# PicoClaw secrets — chmod 600 this file, never commit to git
PICOCLAW_DEV_BOT_TOKEN=your_dev_bot_token_here
PICOCLAW_OPS_BOT_TOKEN=your_ops_bot_token_here
PICOCLAW_MONITOR_BOT_TOKEN=your_monitor_bot_token_here
EOF
    chmod 600 "$ENV_FILE"
    echo "      Created .env template (fill in your bot tokens)"
fi

echo ""
echo "=== Setup complete ==="
echo ""
echo "Next steps:"
echo "  1. Edit config/picoclaw.yaml — set your Telegram user ID"
echo "  2. Edit .env — set your bot tokens"
echo "  3. Reload systemd:"
echo "       systemctl --user daemon-reload"
echo "       sudo systemctl daemon-reload"
echo ""
echo "  4. Start services:"
echo "       systemctl --user enable --now picoclaw-dev picoclaw-ops picoclaw-monitor"
echo "       sudo systemctl enable --now picoclaw-executor"
echo ""
echo "  5. Check logs:"
echo "       journalctl --user -u picoclaw-dev -f"
echo "       journalctl -u picoclaw-executor -f"
echo ""
echo "  6. Test: message your dev bot on Telegram: /help"
