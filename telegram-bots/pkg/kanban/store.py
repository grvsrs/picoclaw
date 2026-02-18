"""
Kanban card storage backend (SQLite).

Provides CRUD operations and queries for Kanban cards.
"""
import sqlite3
import json
from pathlib import Path
from typing import List, Optional, Dict, Any
from datetime import datetime
from .schema import KanbanCard, TaskState, TaskMode, TaskCategory, TaskSource, StateTransition


def _connect(db_path: str) -> sqlite3.Connection:
    """Open a connection with FK enforcement and WAL mode."""
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    conn.execute("PRAGMA journal_mode = WAL")
    return conn


class KanbanStore:
    """SQLite-backed store for Kanban cards."""
    
    def __init__(self, db_path: str = None):
        """Initialize store and create tables if needed."""
        if db_path is None:
            db_path = str(Path.home() / ".local" / "share" / "picoclaw" / "kanban.db")
        self.db_path = db_path
        Path(db_path).parent.mkdir(parents=True, exist_ok=True)
        self._init_schema()
    
    def _init_schema(self):
        """Create tables if they don't exist."""
        with _connect(self.db_path) as conn:
            conn.execute("""
                CREATE TABLE IF NOT EXISTS kanban_cards (
                    card_id TEXT PRIMARY KEY,
                    title TEXT NOT NULL,
                    description TEXT,
                    category TEXT DEFAULT 'uncategorized',
                    source TEXT DEFAULT 'manual',
                    project TEXT DEFAULT '',
                    mode TEXT DEFAULT 'personal',
                    executor TEXT DEFAULT 'picoclaw',
                    allowed_users TEXT,  -- JSON list
                    priority TEXT DEFAULT 'normal',
                    due_date TEXT,
                    state TEXT DEFAULT 'inbox',
                    state_history TEXT,  -- JSON list (dead column, kept for compat)
                    telegram_message_id TEXT,
                    vscode_task_id TEXT,
                    external_ref TEXT,
                    execution_log_url TEXT,
                    attempts INTEGER DEFAULT 0,
                    last_attempt_time TEXT,
                    last_failure_reason TEXT,
                    llm_categorized INTEGER DEFAULT 0,
                    llm_summary TEXT DEFAULT '',
                    tags TEXT,  -- JSON list
                    assignee TEXT,
                    created_by TEXT,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL
                )
            """)
            # Migrate: add new columns to existing databases
            self._migrate_columns(conn)
            conn.execute("""
                CREATE TABLE IF NOT EXISTS state_history (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    card_id TEXT NOT NULL,
                    from_state TEXT NOT NULL,
                    to_state TEXT NOT NULL,
                    reason TEXT,
                    executor TEXT,
                    timestamp TEXT NOT NULL,
                    FOREIGN KEY (card_id) REFERENCES kanban_cards(card_id)
                )
            """)
            conn.execute("""
                CREATE TABLE IF NOT EXISTS execution_events (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    task_id TEXT,
                    source TEXT NOT NULL,
                    event_type TEXT NOT NULL,
                    summary TEXT NOT NULL,
                    details TEXT,
                    exit_code INTEGER,
                    artifact_path TEXT,
                    created_at TEXT NOT NULL DEFAULT (datetime('now'))
                )
            """)
            conn.execute("""
                CREATE INDEX IF NOT EXISTS idx_execution_events_task 
                ON execution_events(task_id, created_at)
            """)
            conn.execute("""
                CREATE TABLE IF NOT EXISTS notes (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    source TEXT NOT NULL,
                    telegram_message_id TEXT,
                    telegram_user_id TEXT,
                    content TEXT NOT NULL,
                    tags TEXT,
                    linked_task_id TEXT,
                    created_at TEXT NOT NULL DEFAULT (datetime('now'))
                )
            """)
            conn.execute("""
                CREATE INDEX IF NOT EXISTS idx_notes_created ON notes(created_at)
            """)
            conn.execute("""
                CREATE INDEX IF NOT EXISTS idx_notes_linked_task ON notes(linked_task_id)
            """)
            conn.execute("""
                CREATE TABLE IF NOT EXISTS system_state (
                    key TEXT PRIMARY KEY,
                    value TEXT NOT NULL,
                    updated_at TEXT NOT NULL
                )
            """)
            # Performance indexes for task board
            conn.execute("CREATE INDEX IF NOT EXISTS idx_cards_state ON kanban_cards(state)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_cards_category ON kanban_cards(category)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_cards_project ON kanban_cards(project)")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_cards_source ON kanban_cards(source)")
            conn.commit()
    
    def _migrate_columns(self, conn):
        """Add new columns to existing databases (safe: ignores if already present)."""
        new_columns = [
            ("category", "TEXT DEFAULT 'uncategorized'"),
            ("source", "TEXT DEFAULT 'manual'"),
            ("project", "TEXT DEFAULT ''"),
            ("external_ref", "TEXT"),
            ("llm_categorized", "INTEGER DEFAULT 0"),
            ("llm_summary", "TEXT DEFAULT ''"),
        ]
        for col_name, col_type in new_columns:
            try:
                conn.execute(f"ALTER TABLE kanban_cards ADD COLUMN {col_name} {col_type}")
            except Exception:
                pass  # Column already exists
    
    def save(self, card: KanbanCard) -> bool:
        """Save or update a Kanban card. Flushes pending transitions to state_history TABLE."""
        try:
            with _connect(self.db_path) as conn:
                data = card.to_dict()
                conn.execute("""
                    INSERT OR REPLACE INTO kanban_cards 
                    (card_id, title, description, category, source, project,
                     mode, executor, allowed_users, priority,
                     due_date, state, state_history, telegram_message_id, vscode_task_id,
                     external_ref, execution_log_url, attempts, last_attempt_time,
                     last_failure_reason, llm_categorized, llm_summary,
                     tags, assignee, created_by, created_at, updated_at)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """, (
                    data["card_id"],
                    data["title"],
                    data["description"],
                    data.get("category", "uncategorized"),
                    data.get("source", "manual"),
                    data.get("project", ""),
                    data["mode"],
                    data["executor"],
                    json.dumps(data["allowed_users"]),
                    data["priority"],
                    data["due_date"],
                    data["state"],
                    json.dumps(data["state_history"]),  # kept as dead column for compat
                    data["telegram_message_id"],
                    data["vscode_task_id"],
                    data.get("external_ref"),
                    data["execution_log_url"],
                    data["attempts"],
                    data["last_attempt_time"],
                    data["last_failure_reason"],
                    1 if data.get("llm_categorized") else 0,
                    data.get("llm_summary", ""),
                    json.dumps(data["tags"]),
                    data["assignee"],
                    data["created_by"],
                    data["created_at"],
                    data["updated_at"],
                ))
                # Flush pending transitions to the state_history TABLE
                for t in getattr(card, '_pending_transitions', []):
                    conn.execute(
                        "INSERT INTO state_history (card_id, from_state, to_state, reason, executor, timestamp) VALUES (?,?,?,?,?,?)",
                        (card.card_id, t.from_state.value, t.to_state.value,
                         t.reason, t.executor, t.timestamp)
                    )
                conn.commit()
                # Clear pending after successful flush
                card._pending_transitions = []
                return True
        except Exception as e:
            print(f"Error saving card {card.card_id}: {e}")
            return False
    
    def get(self, card_id: str) -> Optional[KanbanCard]:
        """Retrieve a card by ID. Hydrates state_history from the TABLE."""
        try:
            with _connect(self.db_path) as conn:
                row = conn.execute(
                    "SELECT * FROM kanban_cards WHERE card_id = ?",
                    (card_id,)
                ).fetchone()
            
                if not row:
                    return None
            
                card = self._row_to_card(row)
                # Hydrate state_history from the TABLE (single source of truth)
                history = conn.execute(
                    "SELECT from_state, to_state, reason, executor, timestamp FROM state_history WHERE card_id = ? ORDER BY id ASC",
                    (card_id,)
                ).fetchall()
                card.state_history = [dict(r) for r in history]
                card._pending_transitions = []
                return card
        except Exception as e:
            print(f"Error retrieving card {card_id}: {e}")
            return None
    
    def list_by_state(self, state: TaskState, limit: int = 100) -> List[KanbanCard]:
        """List all cards in a given state."""
        try:
            with _connect(self.db_path) as conn:
                rows = conn.execute(
                    "SELECT * FROM kanban_cards WHERE state = ? ORDER BY updated_at DESC LIMIT ?",
                    (state.value, limit)
                ).fetchall()
            
            return [self._row_to_card(row) for row in rows]
        except Exception as e:
            print(f"Error listing cards by state {state}: {e}")
            return []
    
    def list_by_mode(self, mode: TaskMode, limit: int = 100) -> List[KanbanCard]:
        """List all cards for a given mode."""
        try:
            with _connect(self.db_path) as conn:
                rows = conn.execute(
                    "SELECT * FROM kanban_cards WHERE mode = ? ORDER BY updated_at DESC LIMIT ?",
                    (mode.value, limit)
                ).fetchall()
            
            return [self._row_to_card(row) for row in rows]
        except Exception as e:
            print(f"Error listing cards by mode {mode}: {e}")
            return []
    
    def list_all(self, limit: int = 500) -> List[KanbanCard]:
        """List all cards (most recently updated first)."""
        try:
            with _connect(self.db_path) as conn:
                rows = conn.execute(
                    "SELECT * FROM kanban_cards ORDER BY updated_at DESC LIMIT ?",
                    (limit,)
                ).fetchall()
            
            return [self._row_to_card(row) for row in rows]
        except Exception as e:
            print(f"Error listing all cards: {e}")
            return []
    
    def list_by_category(self, category: str, limit: int = 100) -> List[KanbanCard]:
        """List all cards in a given category."""
        try:
            with _connect(self.db_path) as conn:
                rows = conn.execute(
                    "SELECT * FROM kanban_cards WHERE category = ? ORDER BY updated_at DESC LIMIT ?",
                    (category, limit)
                ).fetchall()
            return [self._row_to_card(row) for row in rows]
        except Exception as e:
            print(f"Error listing cards by category {category}: {e}")
            return []
    
    def list_by_project(self, project: str, limit: int = 100) -> List[KanbanCard]:
        """List all cards in a given project."""
        try:
            with _connect(self.db_path) as conn:
                rows = conn.execute(
                    "SELECT * FROM kanban_cards WHERE project = ? ORDER BY updated_at DESC LIMIT ?",
                    (project, limit)
                ).fetchall()
            return [self._row_to_card(row) for row in rows]
        except Exception as e:
            print(f"Error listing cards by project {project}: {e}")
            return []
    
    def list_by_source(self, source: str, limit: int = 100) -> List[KanbanCard]:
        """List all cards from a given source."""
        try:
            with _connect(self.db_path) as conn:
                rows = conn.execute(
                    "SELECT * FROM kanban_cards WHERE source = ? ORDER BY updated_at DESC LIMIT ?",
                    (source, limit)
                ).fetchall()
            return [self._row_to_card(row) for row in rows]
        except Exception as e:
            print(f"Error listing cards by source {source}: {e}")
            return []
    
    def get_stats(self) -> Dict[str, Any]:
        """Get board statistics grouped by state, category, and project."""
        stats = {"by_state": {}, "by_category": {}, "by_project": {}, "total": 0}
        try:
            with _connect(self.db_path) as conn:
                # By state
                for row in conn.execute("SELECT state, COUNT(*) FROM kanban_cards GROUP BY state"):
                    stats["by_state"][row[0]] = row[1]
                    stats["total"] += row[1]
                # By category
                for row in conn.execute("SELECT category, COUNT(*) FROM kanban_cards WHERE state != 'done' GROUP BY category"):
                    stats["by_category"][row[0]] = row[1]
                # By project
                for row in conn.execute("SELECT project, COUNT(*) FROM kanban_cards WHERE state != 'done' AND project != '' GROUP BY project"):
                    stats["by_project"][row[0]] = row[1]
        except Exception as e:
            print(f"Error getting stats: {e}")
        return stats
    
    def delete(self, card_id: str) -> bool:
        """Delete a card and its state history."""
        try:
            with _connect(self.db_path) as conn:
                conn.execute("DELETE FROM state_history WHERE card_id = ?", (card_id,))
                conn.execute("DELETE FROM kanban_cards WHERE card_id = ?", (card_id,))
                conn.commit()
                return True
        except Exception as e:
            print(f"Error deleting card {card_id}: {e}")
            return False
    
    def next_card_id(self) -> str:
        """Generate the next card ID from DB max ID. Single-writer safe."""
        with _connect(self.db_path) as conn:
            row = conn.execute(
                "SELECT card_id FROM kanban_cards ORDER BY card_id DESC LIMIT 1"
            ).fetchone()
            if row:
                try:
                    num = int(row[0].split('-')[1])
                    return f"KAN-{num + 1:03d}"
                except (IndexError, ValueError):
                    pass
            count = conn.execute("SELECT COUNT(*) FROM kanban_cards").fetchone()[0]
        return f"KAN-{count + 1:03d}"
    
    def _row_to_card(self, row: sqlite3.Row) -> KanbanCard:
        """Convert a database row to a KanbanCard object."""
        data = dict(row)
        # Parse JSON fields
        if data.get("allowed_users"):
            try:
                data["allowed_users"] = json.loads(data["allowed_users"])
            except (json.JSONDecodeError, TypeError):
                data["allowed_users"] = []
        if data.get("tags"):
            try:
                data["tags"] = json.loads(data["tags"])
            except (json.JSONDecodeError, TypeError):
                data["tags"] = []
        # state_history is hydrated from the TABLE in get(), not from JSON column
        data["state_history"] = []
        # Handle llm_categorized as bool
        data["llm_categorized"] = bool(data.get("llm_categorized", 0))
        
        return KanbanCard.from_dict(data)
