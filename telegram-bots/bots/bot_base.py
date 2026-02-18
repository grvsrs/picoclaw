#!/usr/bin/env python3
"""
PicoClaw Bot Base
─────────────────
Shared logic for all PicoClaw Telegram bots.
Each bot (dev, ops, monitor) subclasses BotBase and registers its handlers.

Components:
    BotConfig       — loads YAML config, resolves token, sets up paths
    ParamValidator   — validates & coerces command parameters against schema
    TaskWriter       — writes task YAML files and polls for executor results
    AuditLogger      — appends structured JSON lines to audit log
    BotBase          — base class with auth, parsing, execution, confirmation

Dependencies:
    pip install python-telegram-bot==20.* pyyaml

Usage:
    See dev_bot.py, ops_bot.py, monitor_bot.py
"""

import asyncio
import json
import logging
import os
import re
import sys
import time
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import yaml
from telegram import Update, BotCommand
from telegram.ext import (
    Application,
    CommandHandler,
    ContextTypes,
    ConversationHandler,
    MessageHandler,
    filters,
)

# Kanban system
from pkg.kanban.store import KanbanStore
from pkg.kanban.telegram_bridge import TelegramKanbanBridge
from pkg.kanban.schema import TaskMode

logger = logging.getLogger(__name__)

# ── Conversation states for confirmation flow ──
AWAITING_CONFIRM = 1


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Utilities
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


def utc_now() -> str:
    """ISO-8601 UTC timestamp."""
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def make_task_id() -> str:
    """Generate a sortable unique task ID (ms-precision timestamp + random hex)."""
    ts = int(time.time() * 1000)
    rand = uuid.uuid4().hex[:8]
    return f"task-{ts}-{rand}"


def truncate(text: str, max_chars: int = 3500) -> str:
    """Truncate text to fit in a single Telegram message (4096 char limit)."""
    if len(text) <= max_chars:
        return text
    return text[:max_chars] + f"\n…[truncated, {len(text) - max_chars} chars omitted]"


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Exceptions
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class ConfigError(Exception):
    """Raised when configuration is invalid or incomplete."""
    pass


class ValidationError(Exception):
    """Raised when command parameters fail validation."""
    pass


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# BotConfig — configuration loader
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class BotConfig:
    """
    Loads and exposes config for a single bot.

    Reads picoclaw.yaml, resolves the bot token from environment variables,
    sets up task and audit paths, and builds the user allowlist.
    """

    def __init__(self, config_path: str, bot_name: str):
        with open(config_path) as f:
            raw = yaml.safe_load(f)

        self.global_cfg = raw.get("global", {})
        self.bot_name = bot_name

        bots = raw.get("bots", {})
        if bot_name not in bots:
            raise ConfigError(
                f"Bot '{bot_name}' not found in config. "
                f"Available: {list(bots.keys())}"
            )

        self.bot_cfg = bots[bot_name]
        self.users = raw.get("users", {})

        # ── Resolve token from environment ──
        token_env = self.bot_cfg.get("token_env")
        if not token_env:
            raise ConfigError(f"Bot '{bot_name}' has no token_env configured")
        self.token = os.environ.get(token_env)
        if not self.token:
            raise ConfigError(
                f"Environment variable {token_env} is not set.\n"
                f"Set it:  export {token_env}=your_bot_token\n"
                f"Get a token from @BotFather on Telegram."
            )

        # ── Task directory ──
        self.task_path = Path(self.bot_cfg["task_path"])
        self.task_path.mkdir(parents=True, exist_ok=True)

        # ── Allowlist (numeric Telegram user IDs as strings for comparison) ──
        self.allowed_users = [
            str(uid) for uid in self.bot_cfg.get("allowed_users", [])
        ]
        self.commands = self.bot_cfg.get("commands", {})

        # ── Audit log path ──
        audit_path_str = self.global_cfg.get(
            "audit_log", "/var/log/picoclaw/audit.jsonl"
        )
        self.audit_log = Path(audit_path_str)
        try:
            self.audit_log.parent.mkdir(parents=True, exist_ok=True)
            self.audit_log.touch(exist_ok=True)
        except PermissionError:
            fallback = Path(__file__).parent.parent / "logs" / "audit.jsonl"
            fallback.parent.mkdir(parents=True, exist_ok=True)
            fallback.touch(exist_ok=True)
            self.audit_log = fallback
            logger.warning(
                f"Cannot write to {audit_path_str}, using {fallback}"
            )

        self.result_timeout = self.global_cfg.get("result_timeout", 30)

    def is_authorized(self, user_id: int) -> bool:
        """Check if a Telegram user ID is in the allowlist."""
        return str(user_id) in self.allowed_users

    def get_command(self, name: str) -> dict | None:
        """Return a command config dict, or None if not found."""
        return self.commands.get(name)


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# ParamValidator — input validation & coercion
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class ParamValidator:
    """
    Validates and coerces command parameters against config schema.

    Supports:
        - required / optional with defaults
        - type coercion (string, integer)
        - allowed-value lists
        - regex pattern matching
        - min/max bounds for integers
        - rejection of unknown parameters
    """

    def validate(self, params: dict, schema: dict) -> dict:
        """
        Validate and coerce params against schema.

        Returns:
            dict of validated, coerced parameters.

        Raises:
            ValidationError with a user-friendly message on failure.
        """
        result = {}

        for param_name, param_schema in schema.items():
            value = params.get(param_name)
            param_type = param_schema.get("type", "string")
            required = param_schema.get("required", False)
            default = param_schema.get("default")

            # ── Missing value handling ──
            if value is None or value == "":
                if required:
                    raise ValidationError(
                        f"Missing required parameter: {param_name}"
                    )
                if default is not None:
                    result[param_name] = default
                continue

            # ── Type: string ──
            if param_type == "string":
                value = str(value)

                allowed = param_schema.get("allowed")
                if allowed and value not in allowed:
                    raise ValidationError(
                        f"Invalid value for {param_name}: '{value}'. "
                        f"Allowed: {', '.join(str(a) for a in allowed)}"
                    )

                pattern = param_schema.get("pattern")
                if pattern and not re.fullmatch(pattern, value):
                    raise ValidationError(
                        f"Invalid format for {param_name}: '{value}' "
                        f"does not match pattern {pattern}"
                    )

            # ── Type: integer ──
            elif param_type == "integer":
                try:
                    value = int(value)
                except (ValueError, TypeError):
                    raise ValidationError(
                        f"Parameter {param_name} must be an integer, "
                        f"got: '{value}'"
                    )

                min_val = param_schema.get("min")
                max_val = param_schema.get("max")
                if min_val is not None and value < min_val:
                    raise ValidationError(
                        f"Parameter {param_name} must be >= {min_val}, "
                        f"got: {value}"
                    )
                if max_val is not None and value > max_val:
                    raise ValidationError(
                        f"Parameter {param_name} must be <= {max_val}, "
                        f"got: {value}"
                    )

            else:
                raise ValidationError(
                    f"Unknown parameter type in schema: {param_type}"
                )

            result[param_name] = value

        # ── Reject unknown parameters ──
        known = set(schema.keys())
        unknown = set(params.keys()) - known
        if unknown:
            raise ValidationError(
                f"Unknown parameters: {', '.join(sorted(unknown))}"
            )

        return result


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# TaskWriter — task file I/O and result polling
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TaskWriter:
    """
    Writes task YAML files and polls for results.

    The bot writes a "pending" task file. The executor picks it up,
    marks it "running", executes, and writes the result back.
    This class polls the file for terminal status.
    """

    def __init__(self, task_path: Path, timeout: int):
        self.task_path = task_path
        self.timeout = timeout

    def write(
        self,
        task_id: str,
        bot_name: str,
        command: str,
        user_id: int,
        username: str,
        params: dict,
        confirmation_required: bool = False,
        project: str = None,
        service: str = None,
        card_id: str = None,
    ) -> Path:
        """Write a task YAML file and return its path."""
        task = {
            "id": task_id,
            "bot": bot_name,
            "command": command,
            "user_id": user_id,
            "username": username,
            "params": params,
            "status": "pending",
            "created_at": utc_now(),
        }

        if project:
            task["project"] = project
        if service:
            task["service"] = service
        if confirmation_required:
            task["confirmation_required"] = True
            task["status"] = "awaiting_confirmation"
        if card_id:
            task["card_id"] = card_id

        task_file = self.task_path / f"{task_id}.yaml"

        # Atomic write: write to temp, then rename
        tmp_file = task_file.with_suffix(".yaml.tmp")
        with open(tmp_file, "w") as f:
            yaml.dump(task, f, default_flow_style=False, sort_keys=False)
        tmp_file.rename(task_file)

        logger.info(
            f"Task written: {task_file} "
            f"(command={command}, user={username})"
        )
        return task_file

    async def poll_result(self, task_file: Path) -> dict | None:
        """
        Poll a task file for status change to a terminal state.

        Uses exponential backoff: 0.5s → 1s → 1.5s → … → 3s max.
        Returns the updated task dict, or None on timeout.
        """
        start = time.monotonic()
        interval = 0.5
        max_interval = 3.0

        while (time.monotonic() - start) < self.timeout:
            await asyncio.sleep(interval)

            try:
                with open(task_file) as f:
                    task = yaml.safe_load(f)
            except Exception:
                continue

            status = task.get("status", "")
            if status in ("complete", "failed", "rejected", "timeout"):
                return task

            # Backoff: 0.5 → 0.75 → 1.125 → … → 3.0
            interval = min(interval * 1.5, max_interval)

        return None


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# AuditLogger — structured JSON audit trail
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class AuditLogger:
    """
    Appends structured JSON audit entries to a .jsonl file.
    Every command submission, confirmation, cancellation, and result
    is recorded for full auditability.
    """

    def __init__(self, log_path: Path):
        self.log_path = log_path

    def log(
        self,
        user_id: int,
        username: str,
        bot: str,
        command: str,
        task_id: str,
        status: str,
        **extra,
    ):
        """Append one audit entry. Extra kwargs are merged in."""
        entry = {
            "ts": utc_now(),
            "user_id": user_id,
            "username": username,
            "bot": bot,
            "command": command,
            "task_id": task_id,
            "status": status,
        }
        # Merge extras, filtering None values for cleanliness
        for k, v in extra.items():
            if v is not None:
                entry[k] = v

        try:
            with open(self.log_path, "a") as f:
                f.write(json.dumps(entry, ensure_ascii=False) + "\n")
        except Exception as e:
            logger.error(f"Failed to write audit log: {e}")


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# BotBase — base class for all PicoClaw bots
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class BotBase:
    """
    Base class for all PicoClaw bots.

    Subclass contract:
        1. Call super().__init__(config_path, bot_name)
        2. Override register_handlers() — call super() then add own handlers
        3. Call self.run() to start the bot

    Provides:
        - User authorization (numeric Telegram ID allowlist)
        - Command argument parsing (positional + named)
        - Parameter validation & coercion
        - Task file writing + result polling
        - Confirmation flow for dangerous commands
        - Help command with auto-generated usage
        - Structured audit logging
    """

    def __init__(self, config_path: str, bot_name: str):
        self.cfg = BotConfig(config_path, bot_name)
        self.validator = ParamValidator()
        self.task_writer = TaskWriter(
            self.cfg.task_path, self.cfg.result_timeout
        )
        self.audit = AuditLogger(self.cfg.audit_log)
        
        # Kanban integration
        db_path = os.environ.get("PICOCLAW_DB", "/var/lib/picoclaw/kanban.db")
        self.kanban_store = KanbanStore(db_path)
        self.kanban_bridge = TelegramKanbanBridge(self.kanban_store)
        
        # user_id → {task_id, command, params, project, service}
        self._pending_confirms: dict[int, dict] = {}

        logging.basicConfig(
            level=logging.INFO,
            format=f"%(asctime)s [{bot_name}] %(levelname)s: %(message)s",
            handlers=[logging.StreamHandler(sys.stdout)],
        )

    # ──────────────────────────────────────────
    # Auth + parsing helpers
    # ──────────────────────────────────────────

    def _is_authorized(self, update: Update) -> bool:
        """Check if the message sender is in the allowlist."""
        return self.cfg.is_authorized(update.effective_user.id)

    async def _reject_unauthorized(self, update: Update):
        """Log and reply to unauthorized access attempts."""
        user = update.effective_user
        logger.warning(
            f"Unauthorized access attempt: user_id={user.id}, "
            f"username={user.username}, name={user.full_name}"
        )
        self.audit.log(
            user_id=user.id,
            username=user.username or "",
            bot=self.cfg.bot_name,
            command="UNAUTHORIZED",
            task_id="",
            status="rejected",
        )
        await update.message.reply_text(
            "⛔ Unauthorized. This incident has been logged."
        )

    def _parse_command_args(self, text: str, command_schema: dict) -> dict:
        """
        Parse command text into a params dict.

        Supports three forms:
            /cmd arg1 arg2            → positional, matched to schema key order
            /cmd key1=val1 key2=val2  → named
            /cmd arg1 key2=val2       → mixed

        Returns:
            dict of param_name → raw string value
        """
        parts = text.split()[1:]  # drop the /command itself
        schema_keys = list(command_schema.keys())
        result = {}
        positional_idx = 0

        for part in parts:
            if "=" in part:
                # Named param: key=value
                key, _, value = part.partition("=")
                result[key] = value
            else:
                # Positional: assign to next schema key in order
                if positional_idx < len(schema_keys):
                    result[schema_keys[positional_idx]] = part
                    positional_idx += 1
                # Extra positional args beyond schema size are silently dropped

        return result

    # ──────────────────────────────────────────
    # Core command execution flow
    # ──────────────────────────────────────────

    async def execute_command(
        self,
        update: Update,
        command_name: str,
        raw_params: dict,
        project: str = None,
        service: str = None,
    ):
        """
        Validate params, check if confirmation required, write task,
        poll for executor result, and send reply.

        This is the common flow for all commands across all bots.
        """
        user = update.effective_user
        cmd_cfg = self.cfg.get_command(command_name)

        if not cmd_cfg:
            await update.message.reply_text(
                f"❌ Unknown command: {command_name}"
            )
            return

        # ── Validate parameters ──
        try:
            params = self.validator.validate(
                raw_params, cmd_cfg.get("params", {})
            )
        except ValidationError as e:
            await update.message.reply_text(f"⚠️ {e}")
            return

        # ── Check if confirmation is needed ──
        needs_confirm = cmd_cfg.get("require_confirmation", False)

        # Per-value confirmation (e.g., deploy env=production)
        if not needs_confirm:
            for param_name, param_schema in cmd_cfg.get("params", {}).items():
                confirm_for = param_schema.get("require_confirmation_for", [])
                if params.get(param_name) in confirm_for:
                    needs_confirm = True
                    break

        if needs_confirm:
            task_id = make_task_id()
            self._pending_confirms[user.id] = {
                "task_id": task_id,
                "command": command_name,
                "params": params,
                "project": project,
                "service": service,
            }

            summary = self._format_confirm_summary(
                command_name, params, project, service
            )
            await update.message.reply_text(
                f"⚠️ *Confirmation required*\n\n{summary}\n\n"
                "Reply /confirm to proceed or /cancel to abort.",
                parse_mode="Markdown",
            )
            return

        # ── No confirmation needed — execute directly ──
        task_id = make_task_id()
        await self._run_task(
            update, task_id, command_name, params, project, service
        )

    async def _run_task(
        self,
        update: Update,
        task_id: str,
        command_name: str,
        params: dict,
        project: str,
        service: str,
    ):
        """Write task file, audit the submission, poll for result, reply."""
        user = update.effective_user

        # ── Create Kanban card FIRST ──
        task_title = f"{command_name}"
        if project:
            task_title += f" ({project})"
        if service:
            task_title += f" / {service}"
        
        kanban_card = self.kanban_bridge.create_card_from_telegram(
            title=task_title,
            telegram_message_id=str(update.message.message_id),
            telegram_user_id=str(user.id),
            mode=TaskMode.PERSONAL,
            executor="picoclaw",
            priority="normal",
            description=str(params),
            tags=[self.cfg.bot_name, command_name],
        )
        
        card_id = kanban_card.card_id if kanban_card else None

        # ── Write task file (with card_id linked) ──
        task_file = self.task_writer.write(
            task_id=task_id,
            bot_name=self.cfg.bot_name,
            command=command_name,
            user_id=user.id,
            username=user.username or user.full_name or str(user.id),
            params=params,
            project=project,
            service=service,
            card_id=card_id,
        )

        # ── Audit: submitted ──
        self.audit.log(
            user_id=user.id,
            username=user.username or "",
            bot=self.cfg.bot_name,
            command=command_name,
            task_id=task_id,
            status="submitted",
            params=params,
            project=project,
            service=service,
        )

        # ── Acknowledge to user ──
        desc_parts = [f"`{command_name}`"]
        if project:
            desc_parts.append(f"({project})")
        if service:
            desc_parts.append(f"({service})")
        desc = " ".join(desc_parts)

        ack_lines = [
            f"⏳ Running {desc}…",
            f"Task: `{task_id}`",
        ]
        if kanban_card:
            ack_lines.append(f"Card: `{kanban_card.card_id}`")
        
        await update.message.reply_text(
            "\n".join(ack_lines),
            parse_mode="Markdown",
        )

        # ── Poll for result ──
        result = await self.task_writer.poll_result(task_file)

        if result is None:
            self.audit.log(
                user_id=user.id,
                username=user.username or "",
                bot=self.cfg.bot_name,
                command=command_name,
                task_id=task_id,
                status="timeout",
            )
            await update.message.reply_text(
                f"⏰ Timeout waiting for result "
                f"({self.cfg.result_timeout}s).\n"
                f"Task `{task_id}` may still be running.\n"
                "Check with /logs or /status.",
                parse_mode="Markdown",
            )
            return

        # ── Audit + reply with result ──
        status = result.get("status", "unknown")
        self.audit.log(
            user_id=user.id,
            username=user.username or "",
            bot=self.cfg.bot_name,
            command=command_name,
            task_id=task_id,
            status=status,
            exit_code=result.get("exit_code"),
            summary=result.get("summary"),
        )

        formatted = self._format_result(result)
        await update.message.reply_text(formatted, parse_mode="Markdown")

    # ──────────────────────────────────────────
    # Confirmation handlers
    # ──────────────────────────────────────────

    async def handle_confirm(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        """Handle /confirm — execute a previously-held command."""
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        user = update.effective_user
        pending = self._pending_confirms.pop(user.id, None)

        if not pending:
            await update.message.reply_text(
                "ℹ️ Nothing pending confirmation."
            )
            return

        self.audit.log(
            user_id=user.id,
            username=user.username or "",
            bot=self.cfg.bot_name,
            command=pending["command"],
            task_id=pending["task_id"],
            status="confirmed",
        )

        await self._run_task(
            update,
            task_id=pending["task_id"],
            command_name=pending["command"],
            params=pending["params"],
            project=pending.get("project"),
            service=pending.get("service"),
        )

    async def handle_cancel(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        """Handle /cancel — discard a confirmation-held command."""
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        user = update.effective_user
        pending = self._pending_confirms.pop(user.id, None)

        if not pending:
            await update.message.reply_text(
                "ℹ️ Nothing pending to cancel."
            )
            return

        self.audit.log(
            user_id=user.id,
            username=user.username or "",
            bot=self.cfg.bot_name,
            command=pending["command"],
            task_id=pending["task_id"],
            status="cancelled",
        )

        await update.message.reply_text(
            f"❌ Cancelled `{pending['command']}`.\n"
            f"Task `{pending['task_id']}` discarded.",
            parse_mode="Markdown",
        )

    # ──────────────────────────────────────────
    # Help handler
    # ──────────────────────────────────────────

    async def handle_help(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        """Handle /help — auto-generate usage from config schema."""
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        lines = [f"*{self.cfg.bot_name} — Commands*\n"]

        for cmd_name, cmd_cfg in self.cfg.commands.items():
            desc = cmd_cfg.get("description", "")
            params = cmd_cfg.get("params", {})
            param_parts = []

            for p_name, p_schema in params.items():
                required = p_schema.get("required", False)
                default = p_schema.get("default")
                if required:
                    param_parts.append(f"<{p_name}>")
                elif default is not None:
                    param_parts.append(f"[{p_name}={default}]")
                else:
                    param_parts.append(f"[{p_name}]")

            param_str = " ".join(param_parts)
            lines.append(f"/{cmd_name} {param_str}")
            lines.append(f"  ↳ {desc}\n")

        lines.append("/confirm — confirm a pending action")
        lines.append("/cancel — cancel a pending action")
        lines.append("/help — show this message")

        await update.message.reply_text(
            "\n".join(lines), parse_mode="Markdown"
        )

    # ──────────────────────────────────────────
    # Formatting helpers
    # ──────────────────────────────────────────

    def _format_confirm_summary(
        self,
        command: str,
        params: dict,
        project: str,
        service: str,
    ) -> str:
        """Format a confirmation prompt with command details."""
        parts = [f"Command: `{command}`"]
        if project:
            parts.append(f"Project: `{project}`")
        if service:
            parts.append(f"Service: `{service}`")
        for k, v in params.items():
            parts.append(f"{k}: `{v}`")
        return "\n".join(parts)

    def _format_result(self, task: dict) -> str:
        """Format a task result for Telegram display."""
        status = task.get("status", "unknown")
        command = task.get("command", "?")
        task_id = task.get("id", "?")
        summary = task.get("summary", "")
        exit_code = task.get("exit_code")
        stdout = task.get("stdout", "")
        stderr = task.get("stderr", "")
        duration = task.get("duration_s")

        icon = {
            "complete": "✅",
            "failed": "❌",
            "rejected": "🚫",
            "timeout": "⏰",
        }.get(status, "❓")

        parts = [f"{icon} *{command}* — {status}"]

        if task_id:
            parts.append(f"Task: `{task_id}`")
        if summary:
            parts.append(f"Summary: {summary}")
        if exit_code is not None:
            parts.append(f"Exit code: {exit_code}")
        if duration is not None:
            parts.append(f"Duration: {duration:.1f}s")

        if stdout:
            parts.append(
                f"\n```\n{truncate(stdout, 2500)}\n```"
            )

        if stderr and status == "failed":
            parts.append(
                f"\n*stderr:*\n```\n{truncate(stderr, 1000)}\n```"
            )

        return "\n".join(parts)

    # ──────────────────────────────────────────
    # Kanban handler
    # ──────────────────────────────────────────

    async def handle_kanban(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        """Handle /kanban — display Kanban board summary."""
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        summary = self.kanban_bridge.list_cards_summary(state=None, limit=10)
        await update.message.reply_text(summary, parse_mode="Markdown")

    # ──────────────────────────────────────────
    # Lifecycle
    # ──────────────────────────────────────────

    def register_handlers(self, app: Application):
        """
        Register base command handlers.
        Subclasses MUST call super().register_handlers(app)
        before adding their own handlers.
        """
        app.add_handler(CommandHandler("help", self.handle_help))
        app.add_handler(CommandHandler("start", self.handle_help))
        app.add_handler(CommandHandler("confirm", self.handle_confirm))
        app.add_handler(CommandHandler("cancel", self.handle_cancel))
        app.add_handler(CommandHandler("kanban", self.handle_kanban))

    async def set_bot_commands(self, app: Application):
        """Set command suggestions in Telegram UI (autocomplete menu)."""
        commands = []
        for cmd_name, cmd_cfg in self.cfg.commands.items():
            desc = cmd_cfg.get("description", cmd_name)
            commands.append(BotCommand(cmd_name, desc[:256]))
        commands.append(BotCommand("help", "Show available commands"))
        commands.append(BotCommand("kanban", "Show Kanban board"))
        await app.bot.set_my_commands(commands)

    def run(self):
        """Build Telegram Application, register handlers, and start polling."""
        app = Application.builder().token(self.cfg.token).build()
        self.register_handlers(app)

        async def post_init(application):
            await self.set_bot_commands(application)

        app.post_init = post_init
        logger.info(f"Starting {self.cfg.bot_name}…")
        app.run_polling(drop_pending_updates=True)
