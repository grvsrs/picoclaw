#!/usr/bin/env python3
"""
PicoClaw Inbox Bot
-------------------
Handles the three highest-priority flows:

  /save <project> [filename]   — Save pasted text/code to a Linux file
  /task <project> <title>      — Create a Kanban card from a quick idea
  /mode local|remote           — Switch system mode instantly
  /board                       — Show Kanban board summary in Telegram
  /inbox                       — Show recent saved files

This bot handles the "capture" layer — getting things FROM Android INTO Linux.
It's separate from dev_bot/ops_bot intentionally: different purpose, different
trust model (these write files; ops touches services).

Setup:
    export PICOCLAW_INBOX_BOT_TOKEN=your_token
    Add to config/picoclaw.yaml under bots: inbox_bot

Usage examples (from Telegram):
    /save glass-walls           [then paste code on next line]
    /save glass-walls signup.py <code block here>
    /task glass-walls add email validation to signup form
    /mode remote
    /board
"""

import json
import logging
import os
import re
import sqlite3
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

from telegram import Update
from telegram.ext import (
    Application,
    CommandHandler,
    ContextTypes,
    MessageHandler,
    ConversationHandler,
    filters,
)

sys.path.insert(0, str(Path(__file__).parent))
sys.path.insert(0, str(Path(__file__).parent.parent))
from bot_base import BotBase, utc_now, make_task_id
from pkg.kanban.store import KanbanStore
from pkg.kanban.telegram_bridge import TelegramKanbanBridge
from pkg.kanban.schema import TaskMode

CONFIG_PATH = Path(__file__).parent.parent / "config" / "picoclaw.yaml"

# ── Config ───────────────────────────────────────────────────────────────────

INBOX_DIR = Path("/home/g/workspace/inbox")
PROJECTS_BASE  = Path("/projects")
DB_PATH        = Path(os.environ.get("PICOCLAW_DB", "/var/lib/picoclaw/kanban.db"))

# Conversation state for /save flow
AWAITING_CONTENT = 1

# ── Lazy KanbanStore singleton ───────────────────────────────────────────────

_STORE: Optional[KanbanStore] = None


def _get_store() -> KanbanStore:
    global _STORE
    if _STORE is None:
        _STORE = KanbanStore(str(DB_PATH))
    return _STORE

# ── Helpers ──────────────────────────────────────────────────────────────────

def detect_extension(content: str, hint: str = "") -> str:
    """
    Guess file extension from content or hint.
    Very simple heuristic — not a full parser.
    """
    if hint and "." in hint:
        return ""  # filename already has extension

    # Check for shebang
    first_line = content.strip().split("\n")[0]
    if "python" in first_line or "#!/usr/bin/env py" in first_line:
        return ".py"
    if "bash" in first_line or "sh" in first_line:
        return ".sh"
    if "node" in first_line or "javascript" in first_line:
        return ".js"

    # Check content patterns
    stripped = content.strip()
    if stripped.startswith("<!DOCTYPE") or stripped.startswith("<html"):
        return ".html"
    if stripped.startswith("{") or stripped.startswith("["):
        try:
            json.loads(stripped)
            return ".json"
        except Exception:
            pass
    if "def " in content and "import " in content:
        return ".py"
    if "function " in content or "const " in content or "let " in content:
        return ".js"
    if stripped.startswith("---") or "yaml" in hint.lower():
        return ".yaml"
    if "# " in content[:200] and stripped.endswith("\n"):
        return ".md"

    return ".txt"


def sanitize_filename(name: str) -> str:
    """Strip anything that could cause path issues."""
    name = re.sub(r"[^\w\-_. ]", "", name).strip()
    name = re.sub(r"\s+", "_", name)
    return name[:80] or "untitled"


def save_to_inbox(content: str, project: Optional[str],
                  filename: Optional[str]) -> Path:
    """
    Save content to /workspace/inbox/<project>/<timestamp>-<filename>
    Returns the saved path.
    """
    ts = datetime.now(timezone.utc).strftime("%Y%m%d-%H%M%S")

    if not filename:
        ext = detect_extension(content)
        filename = f"{ts}{ext}"
    else:
        filename = sanitize_filename(filename)
        if "." not in filename:
            filename += detect_extension(content, filename)

    if project:
        dest_dir = INBOX_DIR / sanitize_filename(project)
    else:
        dest_dir = INBOX_DIR / "_unsorted"

    dest_dir.mkdir(parents=True, exist_ok=True)

    # Don't overwrite — add suffix if conflict
    dest = dest_dir / filename
    if dest.exists():
        stem = Path(filename).stem
        suffix = Path(filename).suffix
        dest = dest_dir / f"{stem}-{ts}{suffix}"

    dest.write_text(content, encoding="utf-8")
    return dest


def create_kanban_card(title: str, project: Optional[str],
                       user_id: str, priority: str = "normal",
                       source: str = "telegram",
                       tags: list = None) -> Optional[str]:
    """
    Create a card via TelegramKanbanBridge (single DB path,
    persistent card IDs, state_history flushed to TABLE).
    """
    try:
        store = _get_store()
        bridge = TelegramKanbanBridge(store)
        card = bridge.create_card_from_telegram(
            title=title,
            telegram_message_id="",
            telegram_user_id=user_id,
            mode=TaskMode.PERSONAL,
            priority=priority,
            description=project or "",
            tags=list(tags or []),
        )
        return card.card_id if card else None
    except Exception as e:
        logging.getLogger(__name__).error(f"create_kanban_card failed: {e}")
        return None


def get_mode() -> str:
    try:
        db_path = str(DB_PATH)
        with sqlite3.connect(db_path) as conn:
            cur = conn.execute("SELECT value FROM system_state WHERE key='mode' LIMIT 1")
            row = cur.fetchone()
            return row[0] if row else "local"
    except Exception:
        return "local"


def set_mode(mode: str) -> bool:
    try:
        DB_PATH.parent.mkdir(parents=True, exist_ok=True)
        db_path = str(DB_PATH)
        now = utc_now()
        with sqlite3.connect(db_path) as conn:
            conn.execute("""
                CREATE TABLE IF NOT EXISTS system_state (
                    key TEXT PRIMARY KEY,
                    value TEXT,
                    updated_at TEXT
                )
            """)
            conn.execute("""
                INSERT INTO system_state (key, value, updated_at)
                VALUES (?, ?, ?)
                ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at
            """, ("mode", mode, now))
            conn.commit()
        return True
    except Exception as e:
        logging.getLogger(__name__).error(f"set_mode failed: {e}")
        return False


def get_board_summary(limit: int = 12) -> str:
    """Return a Telegram-formatted board summary."""
    try:
        db_path = str(DB_PATH)
        with sqlite3.connect(db_path) as conn:
            rows = conn.execute("""
                SELECT state, COUNT(*) as cnt FROM kanban_cards WHERE state != 'archived'
                GROUP BY state
            """).fetchall()

            counts = {"inbox": 0, "planned": 0, "running": 0, "blocked": 0, "review": 0, "done": 0}
            for state, cnt in rows:
                if state in counts:
                    counts[state] = cnt

            # Get current running card
            active_row = conn.execute("""
                SELECT card_id, title FROM kanban_cards WHERE state='running' ORDER BY created_at DESC LIMIT 1
            """).fetchone()
            active = active_row[1] if active_row else None

        mode = get_mode()
        mode_icon = "🏠" if mode == "local" else "🔒"

        lines = [
            f"📋 *Kanban Board* {mode_icon} `{mode}`\n",
            f"📥 Inbox: {counts['inbox']}  "
            f"📐 Planned: {counts['planned']}  "
            f"⚡ Running: {counts['running']}",
            f"🚧 Blocked: {counts['blocked']}  "
            f"👁 Review: {counts['review']}  "
            f"✅ Done: {counts['done']}",
        ]

        if active:
            lines.append(f"\n🔴 Now: `{active[:50]}`")

        return "\n".join(lines)

    except Exception as e:
        return f"❌ Board unavailable: {e}"


def get_recent_inbox(limit: int = 8) -> str:
    """Return Telegram-formatted list of recent inbox files."""
    inbox = INBOX_DIR
    if not inbox.exists():
        return "📂 Inbox is empty."

    files = sorted(inbox.rglob("*"), key=lambda f: f.stat().st_mtime, reverse=True)
    files = [f for f in files if f.is_file()][:limit]

    if not files:
        return "📂 Inbox is empty."

    lines = ["📂 *Recent inbox files:*\n"]
    for f in files:
        rel = f.relative_to(inbox)
        mtime = datetime.fromtimestamp(f.stat().st_mtime).strftime("%m/%d %H:%M")
        size = f.stat().st_size
        size_str = f"{size}B" if size < 1024 else f"{size//1024}KB"
        lines.append(f"`{rel}` — {size_str} @ {mtime}")

    return "\n".join(lines)


# ── Bot class ─────────────────────────────────────────────────────────────────

class InboxBot(BotBase):
    """
    Handles /save, /task, /mode, /board, /inbox.
    Uses a two-step conversation for /save to receive pasted content.
    """

    def __init__(self):
        super().__init__(str(CONFIG_PATH), "inbox_bot")
        self._save_context: dict = {}  # user_id → {project, filename}

    def register_handlers(self, app: Application):
        super().register_handlers(app)

        # /save uses a ConversationHandler to receive content on next message
        save_conv = ConversationHandler(
            entry_points=[CommandHandler("save", self.cmd_save_start)],
            states={
                AWAITING_CONTENT: [
                    MessageHandler(filters.TEXT & ~filters.COMMAND, self.cmd_save_content)
                ],
            },
            fallbacks=[CommandHandler("cancel", self.cmd_save_cancel)],
            conversation_timeout=120,
        )
        app.add_handler(save_conv)
        app.add_handler(CommandHandler("task",   self.cmd_task))
        app.add_handler(CommandHandler("mode",   self.cmd_mode))
        app.add_handler(CommandHandler("board",  self.cmd_board))
        app.add_handler(CommandHandler("inbox",  self.cmd_inbox))

    # ── /save ────────────────────────────────────────────────────────────────

    async def cmd_save_start(self, update: Update, context: ContextTypes.DEFAULT_TYPE):
        """Step 1: parse args, wait for content to be pasted."""
        if not self._is_authorized(update):
            await update.message.reply_text("❌ Unauthorized")
            return ConversationHandler.END

        text = update.message.text.strip()
        parts = text.split(None, 3)

        project  = parts[1] if len(parts) > 1 else None
        filename = None
        inline   = None

        if not project:
            await update.message.reply_text(
                "📋 Usage: `/save <project> [filename]`\n"
                "Then paste your content on the next message.",
                parse_mode="Markdown",
            )
            return ConversationHandler.END

        # Validate project name
        if not re.fullmatch(r"[a-zA-Z0-9_-]+", project):
            await update.message.reply_text(
                f"❌ Project name must be alphanumeric (got: `{project}`)",
                parse_mode="Markdown",
            )
            return ConversationHandler.END

        if len(parts) > 2:
            filename = parts[2]

        # Store context and wait for content
        self._save_context[update.effective_user.id] = {
            "project": project,
            "filename": filename,
        }
        fn_hint = f" as `{filename}`" if filename else ""
        await update.message.reply_text(
            f"📋 Ready to save to *{project}*{fn_hint}.\n"
            "Paste your content now (send /cancel to abort):",
            parse_mode="Markdown",
        )
        return AWAITING_CONTENT

    async def cmd_save_content(self, update: Update, context: ContextTypes.DEFAULT_TYPE):
        """Step 2: receive pasted content and save it."""
        if not self._is_authorized(update):
            await update.message.reply_text("❌ Unauthorized")
            return ConversationHandler.END

        user_id = update.effective_user.id
        ctx = self._save_context.pop(user_id, None)
        if not ctx:
            await update.message.reply_text("❌ Context lost. Try /save again.")
            return ConversationHandler.END

        content = update.message.text
        await self._do_save(update, ctx["project"], ctx["filename"], content)
        return ConversationHandler.END

    async def cmd_save_cancel(self, update: Update, context: ContextTypes.DEFAULT_TYPE):
        self._save_context.pop(update.effective_user.id, None)
        await update.message.reply_text("✅ Save cancelled.")
        return ConversationHandler.END

    async def _do_save(self, update: Update, project: str,
                       filename: Optional[str], content: str):
        """Actually write the file and report back."""
        user = update.effective_user
        try:
            dest = save_to_inbox(content, project, filename)
            
            # Also create a Kanban card
            card_id = create_kanban_card(
                title=f"Saved: {dest.name}",
                project=project,
                user_id=str(user.id),
                source="telegram",
                tags=["inbox-save"],
            )
            
            rel_path = dest.relative_to(INBOX_DIR)
            size = len(content)
            size_str = f"{size}B" if size < 1024 else f"{size//1024}KB"
            
            msg = f"✅ Saved `{rel_path}` ({size_str})"
            if card_id:
                msg += f"\n🔗 Card: `{card_id}`"
            await update.message.reply_text(msg, parse_mode="Markdown")

        except Exception as e:
            await update.message.reply_text(f"❌ Save failed: {e}")

    # ── /task ────────────────────────────────────────────────────────────────

    async def cmd_task(self, update: Update, context: ContextTypes.DEFAULT_TYPE):
        """
        /task <project> <title>  [priority=high|medium|low]

        Examples:
            /task glass-walls add email validation to signup form
            /task glass-walls fix CORS on staging priority=high
        """
        if not self._is_authorized(update):
            await update.message.reply_text("❌ Unauthorized")
            return

        text = update.message.text.strip()
        parts = text.split(None, 2)

        if len(parts) < 3:
            await update.message.reply_text(
                "❌ Usage: `/task <project> <title> [priority=high|medium|low]`",
                parse_mode="Markdown",
            )
            return

        project   = parts[1]
        remainder = parts[2]

        # Extract priority if present
        priority = "normal"
        priority_match = re.search(r"\bpriority=(high|medium|low|normal)\b", remainder, re.I)
        if priority_match:
            priority = priority_match.group(1).lower()
            remainder = remainder[:priority_match.start()] + remainder[priority_match.end():]

        title = remainder.strip()
        if not title:
            await update.message.reply_text("❌ Title cannot be empty")
            return

        if len(title) > 200:
            await update.message.reply_text("❌ Title too long (max 200 chars)")
            return

        user = update.effective_user
        card_id = create_kanban_card(
            title=title,
            project=project,
            user_id=str(user.id),
            priority=priority,
            source="telegram",
        )

        if card_id:
            await update.message.reply_text(
                f"✅ Card created\n"
                f"🔗 ID: `{card_id}`\n"
                f"📌 Title: {title}\n"
                f"🎯 Priority: `{priority}`",
                parse_mode="Markdown",
            )
        else:
            await update.message.reply_text("❌ Failed to create card")

    # ── /mode ────────────────────────────────────────────────────────────────

    async def cmd_mode(self, update: Update, context: ContextTypes.DEFAULT_TYPE):
        """
        /mode               — show current mode
        /mode local         — switch to local mode
        /mode remote        — switch to remote mode
        """
        if not self._is_authorized(update):
            await update.message.reply_text("❌ Unauthorized")
            return

        text  = update.message.text.strip()
        parts = text.split()

        current = get_mode()

        if len(parts) == 1:
            mode_icon = "🏠" if current == "local" else "🔒"
            await update.message.reply_text(
                f"📌 Current mode: {mode_icon} `{current}`\n\n"
                f"To change:\n"
                f"/mode local\n"
                f"/mode remote",
                parse_mode="Markdown",
            )
            return

        new_mode = parts[1].lower().strip()
        if new_mode not in ("local", "remote"):
            await update.message.reply_text(
                f"❌ Invalid mode. Use 'local' or 'remote'",
                parse_mode="Markdown",
            )
            return

        if new_mode == current:
            await update.message.reply_text(
                f"ℹ️  Already in `{current}` mode",
                parse_mode="Markdown",
            )
            return

        ok = set_mode(new_mode)
        user = update.effective_user

        if ok:
            icon = "🏠" if new_mode == "local" else "🔒"
            await update.message.reply_text(
                f"✅ Mode switched {icon}\n"
                f"Old: `{current}`\n"
                f"New: `{new_mode}`",
                parse_mode="Markdown",
            )
        else:
            await update.message.reply_text("❌ Mode switch failed")

    # ── /board ───────────────────────────────────────────────────────────────

    async def cmd_board(self, update: Update, context: ContextTypes.DEFAULT_TYPE):
        """Show Kanban board summary in Telegram."""
        if not self._is_authorized(update):
            await update.message.reply_text("❌ Unauthorized")
            return

        summary = get_board_summary()
        await update.message.reply_text(summary, parse_mode="Markdown")

    # ── /inbox ───────────────────────────────────────────────────────────────

    async def cmd_inbox(self, update: Update, context: ContextTypes.DEFAULT_TYPE):
        """List recent files saved to /workspace/inbox."""
        if not self._is_authorized(update):
            await update.message.reply_text("❌ Unauthorized")
            return

        listing = get_recent_inbox()
        await update.message.reply_text(listing, parse_mode="Markdown")


if __name__ == "__main__":
    InboxBot().run()
