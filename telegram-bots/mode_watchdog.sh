#!/bin/bash
DB="${PICOCLAW_DB:-/var/lib/picoclaw/kanban.db}"
VENV_PYTHON="${PICOCLAW_VENV:-/home/g/picoclaw/telegram-bots/.venv}/bin/python"
SERVER_SCRIPT="/home/g/picoclaw/telegram-bots/kanban_server.py"

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
