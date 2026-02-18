#!/usr/bin/env python3
"""
PicoClaw Monitor Bot
─────────────────────
Handles /health, /recent, and sends proactive alerts when task status changes.

The monitor bot watches the audit log and pushes notifications to Telegram
when important events occur (failures, timeouts).

Setup:
    export PICOCLAW_MONITOR_BOT_TOKEN=your_token_here
    python monitor_bot.py

Commands:
    /health         — Full system health check
    /recent [n=10]  — Recent activity from audit log
    /alerts_on      — Subscribe to proactive failure notifications
    /alerts_off     — Unsubscribe from notifications
    /help
"""

import asyncio
import json
import logging
import sys
from pathlib import Path
from datetime import datetime, timezone

from telegram import Update
from telegram.ext import Application, CommandHandler, ContextTypes

sys.path.insert(0, str(Path(__file__).parent))
from bot_base import BotBase, truncate

CONFIG_PATH = Path(__file__).parent.parent / "config" / "picoclaw.yaml"

logger = logging.getLogger(__name__)

# How often to check for new audit events to push as notifications (seconds)
PUSH_POLL_INTERVAL = 5
# Notify on these statuses proactively
NOTIFY_STATUSES = {"failed", "timeout"}


class MonitorBot(BotBase):

    def __init__(self):
        super().__init__(str(CONFIG_PATH), "monitor_bot")
        # Set of chat IDs subscribed to push notifications
        self._notification_chat_ids: set[int] = set()
        # Byte position in audit log (to read only new entries)
        self._last_audit_pos: int = 0

    def register_handlers(self, app: Application):
        super().register_handlers(app)
        app.add_handler(CommandHandler("health", self.cmd_health))
        app.add_handler(CommandHandler("recent", self.cmd_recent))
        app.add_handler(CommandHandler("alerts_on", self.cmd_alerts_on))
        app.add_handler(CommandHandler("alerts_off", self.cmd_alerts_off))

    # ──────────────────────────────────────────
    # /health — system health summary
    # ──────────────────────────────────────────

    async def cmd_health(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        await self.execute_command(
            update, command_name="health", raw_params={}
        )

    # ──────────────────────────────────────────
    # /recent [n=10] — recent audit log entries
    # ──────────────────────────────────────────

    async def cmd_recent(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        text = update.message.text
        raw = self._parse_command_args(
            text, self.cfg.commands["recent"]["params"]
        )

        try:
            params = self.validator.validate(
                raw, self.cfg.commands["recent"]["params"]
            )
        except Exception as e:
            await update.message.reply_text(f"⚠️ {e}")
            return

        limit = params.get("limit", 10)
        entries = self._read_recent_audit(limit)

        if not entries:
            await update.message.reply_text("📋 No recent activity found.")
            return

        lines = ["*Recent Activity*\n"]
        for entry in entries:
            ts = entry.get("ts", "?")
            user = entry.get("username", entry.get("user_id", "?"))
            cmd = entry.get("command", "?")
            status = entry.get("status", "?")
            bot = entry.get("bot", "?")
            task_id = entry.get("task_id", "")

            icon = {
                "complete": "✅",
                "failed": "❌",
                "timeout": "⏰",
                "submitted": "📤",
                "confirmed": "✔️",
                "cancelled": "🚫",
                "rejected": "⛔",
                "streaming": "📋",
                "running": "⏳",
            }.get(status, "❓")

            # Show time portion only (HH:MM:SS)
            time_str = ts[-9:-1] if len(ts) > 9 else ts
            lines.append(
                f"{icon} `{time_str}` {bot}/{cmd} → {status} ({user})"
            )

        await update.message.reply_text(
            "\n".join(lines), parse_mode="Markdown"
        )

    def _read_recent_audit(self, limit: int) -> list[dict]:
        """Read last N entries from the audit log."""
        audit_path = self.cfg.audit_log
        if not audit_path.exists():
            return []

        try:
            lines = audit_path.read_text().strip().splitlines()
            entries = []
            for line in lines[-limit:]:
                try:
                    entries.append(json.loads(line))
                except json.JSONDecodeError:
                    continue  # skip malformed entries
            return entries
        except Exception:
            return []

    # ──────────────────────────────────────────
    # /alerts_on / /alerts_off — push notifications
    # ──────────────────────────────────────────

    async def cmd_alerts_on(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        chat_id = update.effective_chat.id
        self._notification_chat_ids.add(chat_id)

        self.audit.log(
            user_id=update.effective_user.id,
            username=update.effective_user.username or "",
            bot=self.cfg.bot_name,
            command="alerts_on",
            task_id="",
            status="enabled",
        )

        await update.message.reply_text(
            "🔔 Alerts enabled. You'll be notified of failures and timeouts.\n"
            "Send /alerts_off to disable."
        )

    async def cmd_alerts_off(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        chat_id = update.effective_chat.id
        self._notification_chat_ids.discard(chat_id)

        self.audit.log(
            user_id=update.effective_user.id,
            username=update.effective_user.username or "",
            bot=self.cfg.bot_name,
            command="alerts_off",
            task_id="",
            status="disabled",
        )

        await update.message.reply_text("🔕 Alerts disabled.")

    # ──────────────────────────────────────────
    # Background push notification loop
    # ──────────────────────────────────────────

    async def _push_notification_loop(self, app: Application):
        """
        Polls the audit log for new failure/timeout entries and
        pushes them to subscribed chats.

        Uses file seek position to only read new entries (no re-reading).
        Handles log truncation/rotation gracefully.
        """
        audit_path = self.cfg.audit_log

        # Initialize to end of file (don't replay history on startup)
        if audit_path.exists():
            self._last_audit_pos = audit_path.stat().st_size

        while True:
            await asyncio.sleep(PUSH_POLL_INTERVAL)

            if not audit_path.exists() or not self._notification_chat_ids:
                continue

            try:
                current_size = audit_path.stat().st_size

                # Handle log truncation / rotation
                if current_size < self._last_audit_pos:
                    self._last_audit_pos = 0

                if current_size <= self._last_audit_pos:
                    continue

                with open(audit_path) as f:
                    f.seek(self._last_audit_pos)
                    new_data = f.read()
                    self._last_audit_pos = f.tell()

            except Exception as e:
                logger.debug(f"Push notification read error: {e}")
                continue

            if not new_data.strip():
                continue

            for line in new_data.strip().splitlines():
                try:
                    entry = json.loads(line)
                except json.JSONDecodeError:
                    continue

                status = entry.get("status", "")
                if status not in NOTIFY_STATUSES:
                    continue

                # Build alert message
                cmd = entry.get("command", "?")
                task_id = entry.get("task_id", "?")
                user = entry.get(
                    "username", entry.get("user_id", "?")
                )
                ts = entry.get("ts", "?")
                summary = entry.get("summary", "")
                source = entry.get("source", entry.get("bot", "?"))

                icon = "❌" if status == "failed" else "⏰"
                msg_parts = [
                    f"{icon} *Alert: {status.upper()}*",
                    f"Command: `{cmd}`",
                    f"Task: `{task_id}`",
                    f"Source: {source}",
                    f"User: {user}",
                    f"Time: {ts}",
                ]
                if summary:
                    msg_parts.append(f"Summary: {summary}")

                msg = "\n".join(msg_parts)

                for chat_id in list(self._notification_chat_ids):
                    try:
                        await app.bot.send_message(
                            chat_id, msg, parse_mode="Markdown"
                        )
                    except Exception as e:
                        logger.error(
                            f"Failed to send alert to {chat_id}: {e}"
                        )

    # ──────────────────────────────────────────
    # Lifecycle override
    # ──────────────────────────────────────────

    def run(self):
        """Override run to also start the push notification background task."""
        app = Application.builder().token(self.cfg.token).build()
        self.register_handlers(app)

        async def post_init(application):
            await self.set_bot_commands(application)
            # Start push notification loop as a background task
            asyncio.get_event_loop().create_task(
                self._push_notification_loop(application)
            )

        app.post_init = post_init

        logger.info("Starting monitor_bot…")
        app.run_polling(drop_pending_updates=True)


if __name__ == "__main__":
    MonitorBot().run()
