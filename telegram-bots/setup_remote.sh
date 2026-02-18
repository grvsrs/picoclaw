#!/bin/bash
# =============================================================================
# PicoClaw: Tailscale + Remote Mode Setup
# Ubuntu 22.04 — run as your normal user, uses sudo where needed
# =============================================================================
# After this script:
#   - Tailscale installed and running
#   - Linux accessible from Android via Tailscale IP
#   - Kanban server starts on localhost:3000 (local mode)
#     or Tailscale IP:3000 (remote mode)
#   - Mode switch from Telegram changes which interface the server binds to
# =============================================================================

set -euo pipefail

PICOCLAW_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
USER="$(whoami)"
VENV="$PICOCLAW_DIR/.venv"
SYSTEMD_USER="$HOME/.config/systemd/user"

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║  PicoClaw: Tailscale + Remote Mode Setup ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# ── 1. Install Tailscale ─────────────────────────────────────────────────────

echo "[1/5] Installing Tailscale..."

if command -v tailscale &>/dev/null; then
    echo "      Already installed: $(tailscale version | head -1)"
else
    curl -fsSL https://tailscale.com/install.sh | sh
    echo "      Tailscale installed."
fi

# ── 2. Start Tailscale ───────────────────────────────────────────────────────

echo "[2/5] Starting Tailscale..."

if sudo tailscale status &>/dev/null; then
    TAILSCALE_IP=$(tailscale ip -4 2>/dev/null || echo "not connected")
    echo "      Already connected. IP: $TAILSCALE_IP"
else
    echo ""
    echo "  ┌─────────────────────────────────────────────────────┐"
    echo "  │  A browser window will open for authentication.      │"
    echo "  │  On Android: install Tailscale app → same account.   │"
    echo "  └─────────────────────────────────────────────────────┘"
    echo ""
    sudo tailscale up --accept-routes
    sleep 3
    TAILSCALE_IP=$(tailscale ip -4 2>/dev/null || echo "unknown")
    echo "      Connected. Your Tailscale IP: $TAILSCALE_IP"
fi

# ── 3. Install Flask for Kanban server ──────────────────────────────────────

echo "[3/5] Installing Flask..."
if [ -f "$VENV/bin/pip" ]; then
    "$VENV/bin/pip" install --quiet flask 2>/dev/null || true
    echo "      Flask installed in venv."
else
    pip install --user flask 2>/dev/null || pip3 install --user flask 2>/dev/null || true
    echo "      Flask installed (system user)."
fi

# ── 4. Install Kanban UI files ───────────────────────────────────────────────

echo "[4/5] Installing Kanban server files..."

KANBAN_DEST="$PICOCLAW_DIR/bots"

# Copy files if they're elsewhere
for f in kanban_ui.html kanban_server.py inbox_bot.py; do
    SRC="$PICOCLAW_DIR/$f"
    if [ ! -f "$SRC" ]; then
        if [ "$f" != "inbox_bot.py" ]; then
            echo "      ⚠️  Missing: $f — place it in $PICOCLAW_DIR/"
        fi
    else
        echo "      ✅ Found: $f"
    fi
done

mkdir -p /workspace/inbox 2>/dev/null || true
echo "      Created /workspace/inbox"

# ── 5. Systemd services ──────────────────────────────────────────────────────

echo "[5/5] Installing systemd services..."
mkdir -p "$SYSTEMD_USER"

# Kanban server service (user-level, mode-aware)
cat > "$SYSTEMD_USER/picoclaw-kanban.service" <<EOF
[Unit]
Description=PicoClaw Kanban Web Server
After=network.target picoclaw-dev.service

[Service]
Type=simple
WorkingDirectory=$PICOCLAW_DIR
ExecStart=$VENV/bin/python $PICOCLAW_DIR/kanban_server.py --host 127.0.0.1 --port 3000
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
EnvironmentFile=-$PICOCLAW_DIR/.env

[Install]
WantedBy=default.target
EOF

echo "      Wrote picoclaw-kanban.service"

# Inbox bot service
cat > "$SYSTEMD_USER/picoclaw-inbox.service" <<EOF
[Unit]
Description=PicoClaw Inbox Bot (/save /task /mode /board)
After=network.target

[Service]
Type=simple
WorkingDirectory=$PICOCLAW_DIR
ExecStart=$VENV/bin/python $PICOCLAW_DIR/bots/inbox_bot.py
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal
EnvironmentFile=-$PICOCLAW_DIR/.env

[Install]
WantedBy=default.target
EOF

echo "      Wrote picoclaw-inbox.service"

# Mode watchdog — watches SQLite for mode changes and restarts kanban server
# with appropriate bind address
cat > "$PICOCLAW_DIR/mode_watchdog.sh" <<'WATCHDOG'
#!/bin/bash
# Watches the system_state table for mode changes.
# Restarts kanban_server with the right --host on mode switch.
# Run as: systemctl --user start picoclaw-mode-watchdog

DB="${PICOCLAW_DB:-/var/lib/picoclaw/kanban.db}"
VENV_PYTHON="${PICOCLAW_VENV:-$HOME/picoclaw/telegram-bots/.venv}/bin/python"
SERVER_SCRIPT="${PICOCLAW_DIR:-$HOME/picoclaw/telegram-bots}/kanban_server.py"

last_mode=""

get_mode() {
    sqlite3 "$DB" "SELECT value FROM system_state WHERE key='mode' LIMIT 1;" 2>/dev/null || echo "local"
}

get_tailscale_ip() {
    tailscale ip -4 2>/dev/null || echo "127.0.0.1"
}

while true; do
    mode=$(get_mode)

    if [ "$mode" != "$last_mode" ]; then
        echo "[mode-watchdog] Mode changed: $last_mode → $mode"

        # Stop current kanban server
        systemctl --user stop picoclaw-kanban 2>/dev/null || true
        sleep 1

        if [ "$mode" = "remote" ]; then
            TS_IP=$(get_tailscale_ip)
            echo "[mode-watchdog] Starting kanban on Tailscale IP $TS_IP:3000"
            # Update the service to bind on Tailscale interface
            # (simplest approach: override ExecStart via drop-in)
            mkdir -p "$HOME/.config/systemd/user/picoclaw-kanban.service.d"
            cat > "$HOME/.config/systemd/user/picoclaw-kanban.service.d/override.conf" <<OVERRIDE
[Service]
ExecStart=
ExecStart=$VENV_PYTHON $SERVER_SCRIPT --host $TS_IP --port 3000
OVERRIDE
        else
            echo "[mode-watchdog] Starting kanban on localhost:3000"
            rm -f "$HOME/.config/systemd/user/picoclaw-kanban.service.d/override.conf"
        fi

        systemctl --user daemon-reload
        systemctl --user start picoclaw-kanban
        last_mode="$mode"
    fi

    sleep 5
done
WATCHDOG

chmod +x "$PICOCLAW_DIR/mode_watchdog.sh"

cat > "$SYSTEMD_USER/picoclaw-mode-watchdog.service" <<EOF
[Unit]
Description=PicoClaw Mode Watchdog (kanban rebind on local/remote switch)
After=picoclaw-kanban.service

[Service]
Type=simple
WorkingDirectory=$PICOCLAW_DIR
ExecStart=/bin/bash $PICOCLAW_DIR/mode_watchdog.sh
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal
Environment=PICOCLAW_DIR=$PICOCLAW_DIR
Environment=PICOCLAW_VENV=$VENV
EnvironmentFile=-$PICOCLAW_DIR/.env

[Install]
WantedBy=default.target
EOF

echo "      Wrote picoclaw-mode-watchdog.service"

# ── Summary ───────────────────────────────────────────────────────────────────

TAILSCALE_IP=$(tailscale ip -4 2>/dev/null || echo "not yet connected")

systemctl --user daemon-reload 2>/dev/null || true

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  Setup complete. Next steps:                                 ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "  1. Add PICOCLAW_INBOX_BOT_TOKEN to .env"
echo "     (create a new bot via @BotFather, add to picoclaw.yaml)"
echo ""
echo "  2. Add inbox_bot to config/picoclaw.yaml:"
echo "     bots:"
echo "       inbox_bot:"
echo "         token_env: PICOCLAW_INBOX_BOT_TOKEN"
echo "         allowed_users: [YOUR_TELEGRAM_ID]"
echo "         commands: {} # handled in code, not config"
echo ""
echo "  3. Enable and start services:"
echo "     systemctl --user enable --now picoclaw-kanban"
echo "     systemctl --user enable --now picoclaw-inbox"
echo "     systemctl --user enable --now picoclaw-mode-watchdog"
echo ""
echo "  4. Install Tailscale on Android:"
echo "     → Play Store: Tailscale"
echo "     → Sign in with same account"
echo "     → Your Linux IP: $TAILSCALE_IP"
echo ""
echo "  5. Test:"
echo "     Local:  http://localhost:3000"
echo "     Remote: http://$TAILSCALE_IP:3000"
echo ""
echo "  6. Switch modes from Telegram:"
echo "     /mode remote   (opens Tailscale access)"
echo "     /mode local    (closes to localhost only)"
echo ""
echo "  Logs:"
echo "     journalctl --user -u picoclaw-kanban -f"
echo "     journalctl --user -u picoclaw-inbox -f"
echo "     journalctl --user -u picoclaw-mode-watchdog -f"
echo ""
