#!/usr/bin/env python3
"""
PicoClaw Ops Bot
─────────────────
Handles operations commands: status, logs, restart_service, disk_usage, process_info.

Key feature: /logs streams journalctl output to Telegram in chunks,
respecting rate limits (max 1 msg per LOG_SEND_INTERVAL, buffered to
LOG_CHUNK_SIZE bytes, hard-capped at LOG_MAX_LINES).

Setup:
    export PICOCLAW_OPS_BOT_TOKEN=your_token_here
    python ops_bot.py

Commands:
    /status
    /logs <service> [lines=50]
    /stop                          — cancel active log stream
    /restart_service <service>     — requires confirmation
    /disk_usage [path=/workspace]
    /process_info <name>
    /help
"""

import asyncio
import logging
import sys
import time
from pathlib import Path

from telegram import Update
from telegram.ext import CommandHandler, ContextTypes

sys.path.insert(0, str(Path(__file__).parent))
from bot_base import BotBase, make_task_id, truncate

CONFIG_PATH = Path(__file__).parent.parent / "config" / "picoclaw.yaml"

logger = logging.getLogger(__name__)

# ── Rate limit guards for log streaming ──
LOG_SEND_INTERVAL = 2.0   # min seconds between Telegram messages per stream
LOG_CHUNK_SIZE = 3000      # max chars to buffer before flushing
LOG_MAX_LINES = 500        # hard cap per stream session


class OpsBot(BotBase):

    def __init__(self):
        super().__init__(str(CONFIG_PATH), "ops_bot")
        # user_id → asyncio.Task running _stream_logs
        self._active_streams: dict[int, asyncio.Task] = {}

    def register_handlers(self, app):
        super().register_handlers(app)
        app.add_handler(CommandHandler("status", self.cmd_status))
        app.add_handler(CommandHandler("logs", self.cmd_logs))
        app.add_handler(CommandHandler("stop", self.cmd_stop))
        app.add_handler(
            CommandHandler("restart_service", self.cmd_restart_service)
        )
        app.add_handler(CommandHandler("disk_usage", self.cmd_disk_usage))
        app.add_handler(
            CommandHandler("process_info", self.cmd_process_info)
        )

    # ──────────────────────────────────────────
    # /status
    # ──────────────────────────────────────────

    async def cmd_status(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        await self.execute_command(
            update, command_name="status", raw_params={}
        )

    # ──────────────────────────────────────────
    # /logs <service> [lines=50]
    # Streams log output to Telegram with rate limiting and buffering.
    # ──────────────────────────────────────────

    async def cmd_logs(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        text = update.message.text
        raw = self._parse_command_args(
            text, self.cfg.commands["logs"]["params"]
        )

        service = raw.get("service")
        if not service:
            await update.message.reply_text(
                "Usage: `/logs <service> [lines=50]`\n"
                "Example: `/logs nginx lines=100`\n\n"
                "Send /stop to end an active stream.",
                parse_mode="Markdown",
            )
            return

        # Validate params before starting the stream
        try:
            params = self.validator.validate(
                raw, self.cfg.commands["logs"]["params"]
            )
        except Exception as e:
            await update.message.reply_text(f"⚠️ {e}")
            return

        user = update.effective_user

        # Cancel any existing stream for this user
        existing = self._active_streams.get(user.id)
        if existing and not existing.done():
            existing.cancel()

        await update.message.reply_text(
            f"📋 Streaming `{service}` logs "
            f"({params['lines']} lines)…\n"
            "Send /stop to end.",
            parse_mode="Markdown",
        )

        # Audit the log stream request
        self.audit.log(
            user_id=user.id,
            username=user.username or "",
            bot=self.cfg.bot_name,
            command="logs",
            task_id=make_task_id(),
            status="streaming",
            service=service,
            lines=params["lines"],
        )

        # Run streaming in background so the bot stays responsive
        task = asyncio.get_event_loop().create_task(
            self._stream_logs(
                update, user.id, service, params["lines"]
            )
        )
        self._active_streams[user.id] = task

    async def _stream_logs(
        self,
        update: Update,
        user_id: int,
        service: str,
        lines: int,
    ):
        """
        Stream journalctl output to Telegram in buffered, rate-limited chunks.

        Flow:
            1. Spawn journalctl as async subprocess
            2. Read lines, buffer them
            3. Flush buffer when size limit or time interval reached
            4. Stop at LOG_MAX_LINES or stream end
        """
        chat_id = update.effective_chat.id
        bot = update.get_bot()

        # Use journalctl for systemd services
        cmd = [
            "journalctl",
            "-u", service,
            "-n", str(lines),
            "--no-pager",
            "--output", "short-iso",
        ]

        try:
            proc = await asyncio.create_subprocess_exec(
                *cmd,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
        except FileNotFoundError:
            await bot.send_message(
                chat_id,
                "❌ `journalctl` not found. Is systemd available?",
                parse_mode="Markdown",
            )
            return
        except Exception as e:
            await bot.send_message(
                chat_id, f"❌ Failed to start log stream: {e}"
            )
            return

        buffer: list[str] = []
        buffer_size = 0
        last_sent = 0.0
        line_count = 0

        async def flush_buffer():
            nonlocal buffer, buffer_size, last_sent
            if not buffer:
                return
            text = "".join(buffer)
            buffer = []
            buffer_size = 0
            try:
                await bot.send_message(
                    chat_id,
                    f"```\n{truncate(text, 3500)}\n```",
                    parse_mode="Markdown",
                )
            except Exception as e:
                logger.error(f"Failed to send log chunk: {e}")
            last_sent = time.monotonic()

        try:
            while line_count < LOG_MAX_LINES:
                try:
                    line_bytes = await asyncio.wait_for(
                        proc.stdout.readline(), timeout=10.0
                    )
                except asyncio.TimeoutError:
                    # No more output within timeout
                    break

                if not line_bytes:
                    break  # EOF

                decoded = line_bytes.decode("utf-8", errors="replace")
                buffer.append(decoded)
                buffer_size += len(decoded)
                line_count += 1

                now = time.monotonic()
                should_flush = (
                    buffer_size >= LOG_CHUNK_SIZE
                    or (now - last_sent) >= LOG_SEND_INTERVAL
                )
                if should_flush:
                    await flush_buffer()

            # Final flush
            await flush_buffer()
            await bot.send_message(
                chat_id, f"📋 Stream ended ({line_count} lines)"
            )

        except asyncio.CancelledError:
            await flush_buffer()
            await bot.send_message(chat_id, "📋 Stream stopped by user")
        except Exception as e:
            await bot.send_message(chat_id, f"❌ Stream error: {e}")
        finally:
            if proc.returncode is None:
                try:
                    proc.kill()
                    await proc.wait()
                except Exception:
                    pass
            # Clean up active stream reference
            self._active_streams.pop(user_id, None)

    # ──────────────────────────────────────────
    # /stop — cancel active log stream
    # ──────────────────────────────────────────

    async def cmd_stop(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        user = update.effective_user
        task = self._active_streams.pop(user.id, None)

        if task and not task.done():
            task.cancel()
            await update.message.reply_text("🛑 Stopping log stream…")
        else:
            await update.message.reply_text(
                "ℹ️ No active log stream to stop."
            )

    # ──────────────────────────────────────────
    # /restart_service <service>  (requires confirmation)
    # ──────────────────────────────────────────

    async def cmd_restart_service(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        text = update.message.text
        raw = self._parse_command_args(
            text, self.cfg.commands["restart_service"]["params"]
        )

        service = raw.get("service")
        if not service:
            allowed = self.cfg.commands["restart_service"]["params"][
                "service"
            ].get("allowed", [])
            await update.message.reply_text(
                "Usage: `/restart_service <service>`\n"
                f"Allowed services: {', '.join(allowed)}\n\n"
                "⚠️ This command requires confirmation.",
                parse_mode="Markdown",
            )
            return

        await self.execute_command(
            update,
            command_name="restart_service",
            raw_params=raw,
            service=service,
        )

    # ──────────────────────────────────────────
    # /disk_usage [path=/workspace]
    # ──────────────────────────────────────────

    async def cmd_disk_usage(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        text = update.message.text
        raw = self._parse_command_args(
            text, self.cfg.commands["disk_usage"]["params"]
        )

        await self.execute_command(
            update,
            command_name="disk_usage",
            raw_params=raw,
        )

    # ──────────────────────────────────────────
    # /process_info <name>
    # ──────────────────────────────────────────

    async def cmd_process_info(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        text = update.message.text
        raw = self._parse_command_args(
            text, self.cfg.commands["process_info"]["params"]
        )

        name = raw.get("name")
        if not name:
            await update.message.reply_text(
                "Usage: `/process_info <name>`\n"
                "Example: `/process_info nginx`",
                parse_mode="Markdown",
            )
            return

        await self.execute_command(
            update,
            command_name="process_info",
            raw_params=raw,
        )


if __name__ == "__main__":
    OpsBot().run()
