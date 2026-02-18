#!/usr/bin/env python3
"""
PicoClaw Kanban Server
-----------------------
Serves the Kanban web UI and a JSON API backed by the existing SQLite DB.
Designed to sit alongside the existing pkg/kanban/ package.

Usage:
    cd /home/g/picoclaw/telegram-bots
    source .venv/bin/activate
    python kanban_server.py

    # Or as a systemd user service (see kanban_server.service)
    systemctl --user start picoclaw-kanban

Access:
    Local:  http://localhost:3000
    Remote: http://<tailscale-ip>:3000  (only when mode=remote and Tailscale active)

API:
    GET /           → Kanban UI (HTML)
    GET /api/board  → JSON: { cards, events, mode, stats }
    GET /api/mode   → JSON: { mode }
    POST /api/mode  → JSON body: { mode: "local"|"remote" }
                      Returns: { mode, changed_at }

Dependencies: flask (add to requirements if not present)
    pip install flask
"""

import json
import hmac
import os
import sqlite3
import sys
from datetime import datetime, timezone
from functools import wraps
from pathlib import Path

# ── Path setup ───────────────────────────────────────────────────────────────
# Adjust if your DB lives elsewhere
DEFAULT_DB = Path("/var/lib/picoclaw/kanban.db")
FALLBACK_DB = Path("/tmp/picoclaw_test_kanban.db")  # used by verify_kanban.py

UI_FILE = Path(__file__).parent / "kanban_ui.html"

try:
    from flask import Flask, jsonify, request, send_file, abort
except ImportError:
    print("Flask not installed. Run: pip install flask", file=sys.stderr)
    sys.exit(1)

app = Flask(__name__)

# ── Auth ─────────────────────────────────────────────────────────────────────

API_SECRET = os.environ.get("PICOCLAW_API_SECRET", "")


def require_api_key(f):
    """Decorator: reject requests without a valid X-API-Key header."""
    @wraps(f)
    def decorated(*args, **kwargs):
        if not API_SECRET:
            return jsonify({"error": "API_SECRET not set"}), 503
        provided = request.headers.get("X-API-Key", "").strip()
        if not hmac.compare_digest(provided, API_SECRET):
            code = 401 if not provided else 403
            return jsonify({"error": "Unauthorized"}), code
        return f(*args, **kwargs)
    return decorated


# ── Config ───────────────────────────────────────────────────────────────────

def get_db_path() -> Path:
    env = os.environ.get("PICOCLAW_DB")
    if env:
        return Path(env)
    if DEFAULT_DB.exists():
        return DEFAULT_DB
    if FALLBACK_DB.exists():
        return FALLBACK_DB
    # Create DB at default path (will be empty until bots write to it)
    DEFAULT_DB.parent.mkdir(parents=True, exist_ok=True)
    return DEFAULT_DB


def get_mode() -> str:
    """Read current mode from SQLite system table, default local."""
    try:
        db = get_db_path()
        with sqlite3.connect(db) as conn:
            conn.row_factory = sqlite3.Row
            cur = conn.execute(
                "SELECT value FROM system_state WHERE key='mode' LIMIT 1"
            )
            row = cur.fetchone()
            return row["value"] if row else "local"
    except Exception:
        return "local"


def set_mode(mode: str) -> dict:
    if mode not in ("local", "remote"):
        raise ValueError(f"Invalid mode: {mode}")
    db = get_db_path()
    now = datetime.now(timezone.utc).isoformat()
    with sqlite3.connect(db) as conn:
        conn.execute("""
            CREATE TABLE IF NOT EXISTS system_state (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL,
                updated_at TEXT NOT NULL
            )
        """)
        conn.execute("""
            INSERT INTO system_state (key, value, updated_at)
            VALUES ('mode', ?, ?)
            ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at
        """, (mode, now))
        conn.commit()
    return {"mode": mode, "changed_at": now}


def get_cards(limit: int = 200) -> list:
    """Fetch all non-archived cards from the Kanban store."""
    db = get_db_path()
    try:
        with sqlite3.connect(db) as conn:
            conn.row_factory = sqlite3.Row
            try:
                rows = conn.execute(
                    "SELECT * FROM kanban_cards WHERE state != 'archived' LIMIT ?",
                    (limit,)
                ).fetchall()
            except sqlite3.OperationalError:
                return []

            cards = [dict(r) for r in rows]

            # Normalize tags (stored as JSON string) and ensure keys exist
            for c in cards:
                if isinstance(c.get("tags"), str):
                    try:
                        c["tags"] = json.loads(c["tags"])
                    except Exception:
                        c["tags"] = []

            # Order states according to desired display priority
            order = {"running": 0, "blocked": 1, "review": 2, "planned": 3, "inbox": 4, "done": 5}
            def sort_key(card):
                return (order.get(card.get("state"), 6),
                        card.get("created_at") or "")

            cards.sort(key=sort_key)
            return cards
    except Exception as e:
        app.logger.warning(f"get_cards error: {e}")
        return []


def get_recent_events(limit: int = 50) -> list:
    """Fetch recent events for the event log panel."""
    db = get_db_path()
    try:
        with sqlite3.connect(db) as conn:
            conn.row_factory = sqlite3.Row
            try:
                rows = conn.execute("""
                    SELECT event_type, task_id as card_id, summary, created_at
                    FROM execution_events
                    ORDER BY created_at DESC
                    LIMIT ?
                """, (limit,)).fetchall()
            except sqlite3.OperationalError:
                return []

            events = []
            for row in rows:
                e = dict(row)
                e["timestamp"] = e.pop("created_at", None)
                e["message"] = e.pop("summary", "")
                events.append(e)
            return events
    except Exception as e:
        app.logger.warning(f"get_events error: {e}")
        return []


# ── Routes ───────────────────────────────────────────────────────────────────

@app.route("/")
def index():
    # Serve the Kanban HTML UI, injecting API key for authenticated JS calls
    search_paths = [
        UI_FILE,
        Path(__file__).parent / "kanban_ui.html",
        Path.home() / "picoclaw" / "kanban_ui.html",
        Path("/home/g/workspace/kanban_ui.html"),
    ]
    html_path = None
    for p in search_paths:
        if p.exists():
            html_path = p
            break
    if not html_path:
        abort(404, "kanban_ui.html not found. Place it alongside kanban_server.py")
    html = html_path.read_text(encoding="utf-8")
    # Inject API key so the UI can call authenticated endpoints
    html = html.replace(
        "/* INJECT_API_KEY */",
        f"const API_KEY = '{API_SECRET}';" if API_SECRET else "const API_KEY = '';"
    )
    return html


@app.route("/api/board")
def api_board():
    cards  = get_cards()
    events = get_recent_events(20)
    mode   = get_mode()

    stats = {
        "total":   len(cards),
        "running": sum(1 for c in cards if c.get("state") == "running"),
        "blocked": sum(1 for c in cards if c.get("state") == "blocked"),
        "review":  sum(1 for c in cards if c.get("state") == "review"),
        "done":    sum(1 for c in cards if c.get("state") == "done"),
    }

    # Category stats
    categories = {}
    for c in cards:
        cat = c.get("category", "uncategorized")
        categories[cat] = categories.get(cat, 0) + 1

    # Project stats
    projects = {}
    for c in cards:
        proj = c.get("project", "")
        if proj:
            projects[proj] = projects.get(proj, 0) + 1

    return jsonify({
        "cards":      cards,
        "events":     events,
        "mode":       mode,
        "stats":      stats,
        "categories": categories,
        "projects":   projects,
    })


@app.route("/api/mode", methods=["GET"])
def api_mode_get():
    return jsonify({"mode": get_mode()})


@app.route("/api/mode", methods=["POST"])
@require_api_key
def api_mode_set():
    data = request.get_json(force=True, silent=True) or {}
    mode = data.get("mode", "").strip().lower()
    if mode not in ("local", "remote"):
        return jsonify({"error": "mode must be 'local' or 'remote'"}), 400
    try:
        result = set_mode(mode)
        return jsonify(result)
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/api/cards", methods=["GET"])
def api_cards():
    state = request.args.get("state")
    category = request.args.get("category")
    project = request.args.get("project")
    source = request.args.get("source")
    cards = get_cards()
    if state:
        cards = [c for c in cards if c.get("state") == state]
    if category:
        cards = [c for c in cards if c.get("category") == category]
    if project:
        cards = [c for c in cards if c.get("project") == project]
    if source:
        cards = [c for c in cards if c.get("source") == source]
    return jsonify({"cards": cards, "count": len(cards)})


@app.route("/api/cards", methods=["POST"])
@require_api_key
def api_create_card():
    """Create a new task card via API."""
    data = request.get_json(force=True, silent=True) or {}
    title = data.get("title", "").strip()
    if not title:
        return jsonify({"error": "title is required"}), 400

    try:
        # Import store here to avoid circular dependency at module level
        sys.path.insert(0, str(Path(__file__).parent))
        from pkg.kanban.store import KanbanStore
        from pkg.kanban.schema import KanbanCard, TaskCategory, TaskSource

        store = KanbanStore(str(get_db_path()))
        card_id = store.next_card_id()

        # Parse category
        category = TaskCategory.UNCATEGORIZED
        if data.get("category"):
            try:
                category = TaskCategory(data["category"])
            except ValueError:
                category = TaskCategory.UNCATEGORIZED

        # Parse source
        source = TaskSource.API
        if data.get("source"):
            try:
                source = TaskSource(data["source"])
            except ValueError:
                source = TaskSource.API

        card = KanbanCard(
            card_id=card_id,
            title=title,
            description=data.get("description", ""),
            category=category,
            source=source,
            project=data.get("project", ""),
            priority=data.get("priority", "normal"),
            tags=data.get("tags", []),
            assignee=data.get("assignee", ""),
        )
        store.save(card)
        return jsonify({"card": card.to_dict(), "id": card_id}), 201
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/api/cards/<card_id>", methods=["PUT"])
@require_api_key
def api_update_card(card_id):
    """Update an existing task card."""
    data = request.get_json(force=True, silent=True) or {}
    try:
        sys.path.insert(0, str(Path(__file__).parent))
        from pkg.kanban.store import KanbanStore

        store = KanbanStore(str(get_db_path()))
        card = store.get(card_id)
        if not card:
            return jsonify({"error": "Card not found"}), 404

        # Update allowed fields
        if "title" in data:
            card.title = data["title"]
        if "description" in data:
            card.description = data["description"]
        if "priority" in data:
            card.priority = data["priority"]
        if "project" in data:
            card.project = data["project"]
        if "tags" in data:
            card.tags = data["tags"]
        if "assignee" in data:
            card.assignee = data["assignee"]
        if "category" in data:
            from pkg.kanban.schema import TaskCategory
            try:
                card.category = TaskCategory(data["category"])
            except ValueError:
                pass

        store.save(card)
        return jsonify({"card": card.to_dict()})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/api/cards/<card_id>/transition", methods=["POST"])
@require_api_key
def api_transition_card(card_id):
    """Move a card to a new state."""
    data = request.get_json(force=True, silent=True) or {}
    new_state = data.get("state", "").strip().lower()
    reason = data.get("reason", "")
    executor = data.get("executor", "api")

    if not new_state:
        return jsonify({"error": "state is required"}), 400

    try:
        sys.path.insert(0, str(Path(__file__).parent))
        from pkg.kanban.store import KanbanStore
        from pkg.kanban.schema import TaskState

        store = KanbanStore(str(get_db_path()))
        card = store.get(card_id)
        if not card:
            return jsonify({"error": "Card not found"}), 404

        try:
            target_state = TaskState(new_state)
        except ValueError:
            return jsonify({"error": f"Invalid state: {new_state}"}), 400

        if not card.transition_to(target_state, reason=reason, executor=executor):
            return jsonify({
                "error": f"Invalid transition: {card.state.value} → {new_state}"
            }), 400

        store.save(card)
        return jsonify({"card": card.to_dict()})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/api/categories")
def api_categories():
    """List all categories with counts."""
    cards = get_cards()
    categories = {}
    for c in cards:
        cat = c.get("category", "uncategorized")
        categories[cat] = categories.get(cat, 0) + 1
    return jsonify({"categories": categories})


@app.route("/api/projects")
def api_projects():
    """List all projects with counts."""
    cards = get_cards()
    projects = {}
    for c in cards:
        proj = c.get("project", "")
        if proj:
            projects[proj] = projects.get(proj, 0) + 1
    return jsonify({"projects": projects})


@app.route("/api/categorize", methods=["POST"])
@require_api_key
def api_categorize():
    """Categorize a task using rule-based engine (LLM integration via Go backend)."""
    data = request.get_json(force=True, silent=True) or {}
    title = data.get("title", "").strip()
    if not title:
        return jsonify({"error": "title is required"}), 400

    try:
        sys.path.insert(0, str(Path(__file__).parent))
        from pkg.kanban.categorizer import categorize_by_rules
        result = categorize_by_rules(title, data.get("description", ""))
        return jsonify(result)
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/api/categorize/card/<card_id>", methods=["POST"])
@require_api_key
def api_categorize_card(card_id):
    """Auto-categorize an existing card."""
    try:
        sys.path.insert(0, str(Path(__file__).parent))
        from pkg.kanban.store import KanbanStore
        from pkg.kanban.categorizer import categorize_by_rules, apply_categorization

        store = KanbanStore(str(get_db_path()))
        card = store.get(card_id)
        if not card:
            return jsonify({"error": "Card not found"}), 404

        result = categorize_by_rules(card.title, card.description)
        card = apply_categorization(card, result, from_llm=False)
        store.save(card)
        return jsonify({"card": card.to_dict(), "categorization": result})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/api/stats")
def api_stats():
    """Get comprehensive board statistics."""
    cards = get_cards()
    
    by_state = {}
    by_category = {}
    by_project = {}
    by_source = {}
    by_priority = {}
    
    for c in cards:
        state = c.get("state", "inbox")
        by_state[state] = by_state.get(state, 0) + 1
        
        cat = c.get("category", "uncategorized")
        by_category[cat] = by_category.get(cat, 0) + 1
        
        proj = c.get("project", "")
        if proj:
            by_project[proj] = by_project.get(proj, 0) + 1
        
        src = c.get("source", "manual")
        by_source[src] = by_source.get(src, 0) + 1
        
        pri = c.get("priority", "normal")
        by_priority[pri] = by_priority.get(pri, 0) + 1
    
    return jsonify({
        "total": len(cards),
        "by_state": by_state,
        "by_category": by_category,
        "by_project": by_project,
        "by_source": by_source,
        "by_priority": by_priority,
    })


@app.route("/health")
def health():
    return jsonify({"status": "ok", "db": str(get_db_path()), "mode": get_mode()})


# ── Main ─────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="PicoClaw Kanban Server")
    parser.add_argument("--host", default="127.0.0.1",
                        help="Bind address (use 0.0.0.0 to expose on network)")
    parser.add_argument("--port", type=int, default=3000)
    parser.add_argument("--db", help="Path to kanban.db (overrides PICOCLAW_DB env var)")
    args = parser.parse_args()

    if args.db:
        os.environ["PICOCLAW_DB"] = args.db

    mode = get_mode()
    db_path = get_db_path()

    print(f"""
╔═══════════════════════════════════════╗
║  PicoClaw Kanban Server               ║
╠═══════════════════════════════════════╣
║  URL:  http://{args.host}:{args.port:<20}║
║  DB:   {str(db_path):<31}║
║  Mode: {mode:<31}║
╚═══════════════════════════════════════╝
""")

    # In local mode: bind to localhost only
    # In remote mode: the systemd service should be called with --host 0.0.0.0
    # Tailscale controls external access; don't expose on 0.0.0.0 without it
    app.run(host=args.host, port=args.port, debug=False, threaded=True)
