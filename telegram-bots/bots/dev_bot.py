#!/usr/bin/env python3
"""
PicoClaw Dev Bot
─────────────────
Handles development commands: run_tests, deploy, scaffold, git_status.

Setup:
    export PICOCLAW_DEV_BOT_TOKEN=your_token_here
    python dev_bot.py

Commands:
    /run_tests <project> [suite=all|unit|integration]
    /deploy <project> [env=staging|production]
    /scaffold <project> <component>
    /git_status <project>
    /help
"""

import sys
from pathlib import Path

from telegram import Update
from telegram.ext import CommandHandler, ContextTypes

# Allow running from project root or bots/ directory
sys.path.insert(0, str(Path(__file__).parent))
from bot_base import BotBase

CONFIG_PATH = Path(__file__).parent.parent / "config" / "picoclaw.yaml"


class DevBot(BotBase):

    def __init__(self):
        super().__init__(str(CONFIG_PATH), "dev_bot")

    def register_handlers(self, app):
        super().register_handlers(app)  # registers /help, /confirm, /cancel
        app.add_handler(CommandHandler("run_tests", self.cmd_run_tests))
        app.add_handler(CommandHandler("deploy", self.cmd_deploy))
        app.add_handler(CommandHandler("scaffold", self.cmd_scaffold))
        app.add_handler(CommandHandler("git_status", self.cmd_git_status))

    # ──────────────────────────────────────────
    # /run_tests <project> [suite=all|unit|integration]
    # ──────────────────────────────────────────

    async def cmd_run_tests(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        text = update.message.text
        raw = self._parse_command_args(
            text, self.cfg.commands["run_tests"]["params"]
        )

        project = raw.pop("project", None)
        if not project:
            await update.message.reply_text(
                "Usage: `/run_tests <project> [suite=all|unit|integration]`\n"
                "Example: `/run_tests glass-walls suite=integration`",
                parse_mode="Markdown",
            )
            return

        await self.execute_command(
            update,
            command_name="run_tests",
            raw_params=raw,
            project=project,
        )

    # ──────────────────────────────────────────
    # /deploy <project> [env=staging|production]
    # ──────────────────────────────────────────

    async def cmd_deploy(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        text = update.message.text
        raw = self._parse_command_args(
            text, self.cfg.commands["deploy"]["params"]
        )

        project = raw.pop("project", None)
        if not project:
            await update.message.reply_text(
                "Usage: `/deploy <project> [env=staging|production]`\n"
                "Example: `/deploy glass-walls env=staging`",
                parse_mode="Markdown",
            )
            return

        await self.execute_command(
            update,
            command_name="deploy",
            raw_params=raw,
            project=project,
        )

    # ──────────────────────────────────────────
    # /scaffold <project> <component>
    # ──────────────────────────────────────────

    async def cmd_scaffold(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        text = update.message.text
        raw = self._parse_command_args(
            text, self.cfg.commands["scaffold"]["params"]
        )

        project = raw.pop("project", None)
        component = raw.get("component")

        if not project or not component:
            await update.message.reply_text(
                "Usage: `/scaffold <project> <component>`\n"
                "Example: `/scaffold glass-walls auth/email-validator`",
                parse_mode="Markdown",
            )
            return

        await self.execute_command(
            update,
            command_name="scaffold",
            raw_params=raw,
            project=project,
        )

    # ──────────────────────────────────────────
    # /git_status <project>
    # ──────────────────────────────────────────

    async def cmd_git_status(
        self, update: Update, context: ContextTypes.DEFAULT_TYPE
    ):
        if not self._is_authorized(update):
            await self._reject_unauthorized(update)
            return

        text = update.message.text
        raw = self._parse_command_args(
            text, self.cfg.commands["git_status"]["params"]
        )

        project = raw.pop("project", None)
        if not project:
            await update.message.reply_text(
                "Usage: `/git_status <project>`\n"
                "Example: `/git_status glass-walls`",
                parse_mode="Markdown",
            )
            return

        await self.execute_command(
            update,
            command_name="git_status",
            raw_params={},
            project=project,
        )


if __name__ == "__main__":
    DevBot().run()
