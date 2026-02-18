"""
Event bridge: connects Picoclaw executor events to Kanban card state updates.

Picoclaw emits events (task_started, task_progress, task_completed, task_failed).
This module listens and updates Kanban cards accordingly.
"""
from typing import Optional, Dict, Any, Callable
from datetime import datetime, timezone
from .schema import KanbanCard, TaskState
from .store import KanbanStore


class KanbanEventBridge:
    """Routes Picoclaw events to Kanban card updates."""
    
    def __init__(self, store: KanbanStore):
        """Initialize bridge with a Kanban store."""
        self.store = store
        self.subscribers: Dict[str, list] = {}  # event_type -> list of callbacks
    
    def subscribe(self, event_type: str, callback: Callable) -> None:
        """Register a callback for an event type."""
        if event_type not in self.subscribers:
            self.subscribers[event_type] = []
        self.subscribers[event_type].append(callback)
    
    def _emit(self, event_type: str, **kwargs) -> None:
        """Emit an event to all subscribers."""
        for callback in self.subscribers.get(event_type, []):
            try:
                callback(**kwargs)
            except Exception as e:
                print(f"Error in {event_type} callback: {e}")
    
    def on_task_started(self, card_id: str, executor: str = "picoclaw") -> None:
        """Picoclaw task started."""
        card = self.store.get(card_id)
        if not card:
            print(f"Card {card_id} not found")
            return
        
        card.transition_to(TaskState.RUNNING, reason="Execution started", executor=executor)
        card.last_attempt_time = datetime.now(timezone.utc)
        card.attempts += 1
        self.store.save(card)
        
        self._emit("card_updated", card_id=card_id, state=TaskState.RUNNING)
    
    def on_task_progress(self, card_id: str, progress: float, message: str = "") -> None:
        """Picoclaw reports progress (0.0 to 1.0)."""
        card = self.store.get(card_id)
        if not card:
            return
        
        # Update metadata without changing state
        if message:
            card.last_failure_reason = message  # Reuse field for progress message
        
        self.store.save(card)
        self._emit("progress_update", card_id=card_id, progress=progress)
    
    def on_task_completed(self, card_id: str, result: str = "", log_url: str = "") -> None:
        """Picoclaw task completed successfully."""
        card = self.store.get(card_id)
        if not card:
            return
        
        card.transition_to(TaskState.REVIEW, reason="Execution completed", executor="picoclaw")
        card.execution_log_url = log_url or ""
        card.last_failure_reason = ""
        self.store.save(card)
        
        self._emit("task_completed", card_id=card_id, result=result)
    
    def on_task_failed(self, card_id: str, error: str = "", log_url: str = "") -> None:
        """Picoclaw task failed."""
        card = self.store.get(card_id)
        if not card:
            return
        
        card.transition_to(TaskState.BLOCKED, reason="Execution failed", executor="picoclaw")
        card.last_failure_reason = error or "Unknown error"
        card.execution_log_url = log_url or ""
        self.store.save(card)
        
        self._emit("task_failed", card_id=card_id, error=error)
    
    def on_task_approved(self, card_id: str, approver: str = "") -> None:
        """Human approves a completed task (REVIEW -> DONE)."""
        card = self.store.get(card_id)
        if not card:
            return
        
        if card.state == TaskState.REVIEW:
            card.transition_to(TaskState.DONE, reason="Approved", executor=approver)
            self.store.save(card)
            self._emit("task_approved", card_id=card_id)
    
    def on_task_rejected(self, card_id: str, reason: str = "", approver: str = "") -> None:
        """Human rejects a completed task (REVIEW -> BLOCKED)."""
        card = self.store.get(card_id)
        if not card:
            return
        
        if card.state == TaskState.REVIEW:
            card.last_failure_reason = reason or "Rejected by reviewer"
            card.transition_to(TaskState.BLOCKED, reason=f"Rejected: {reason}", executor=approver)
            self.store.save(card)
            self._emit("task_rejected", card_id=card_id, reason=reason)


# ── ExecutionEvent emission (new canonical API) ──────────────────────────────

def emit_event(
    store: KanbanStore,
    task_id: Optional[str],
    source: str,
    event_type: str,
    summary: str,
    details: Optional[str] = None,
    exit_code: Optional[int] = None,
    artifact_path: Optional[str] = None,
) -> int:
    """
    Emit an ExecutionEvent and optionally trigger Kanban state reactions.

    Args:
        store: KanbanStore instance
        task_id: Card ID (e.g. KAN-001) or None for ad-hoc
        source: "executor" | "terminal" | "vscode" | "bot"
        event_type: "started" | "progress" | "completed" | "failed"
        summary: Short human-readable line
        details: Optional extended info
        exit_code: Exit code for completed/failed events
        artifact_path: Path to logs or output files

    Returns:
        Event ID (autoincrement integer)

    Side effects:
        - Inserts row into execution_events table
        - If terminal event + task_id exists: updates card state
    """
    import sqlite3
    from datetime import datetime, timezone
    
    # Validate enum values (strict)
    valid_sources = {"executor", "terminal", "vscode", "bot"}
    valid_types = {"started", "progress", "completed", "failed"}
    
    if source not in valid_sources:
        raise ValueError(f"Invalid source: {source}")
    if event_type not in valid_types:
        raise ValueError(f"Invalid event_type: {event_type}")

    # Insert event
    now = datetime.now(timezone.utc).isoformat()
    with sqlite3.connect(store.db_path) as conn:
        cursor = conn.execute(
            """
            INSERT INTO execution_events 
            (task_id, source, event_type, summary, details, exit_code, artifact_path, created_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (task_id, source, event_type, summary, details, exit_code, artifact_path, now),
        )
        event_id = cursor.lastrowid
        conn.commit()

    # React to terminal events (update Kanban state)
    if task_id and event_type in ("started", "completed", "failed"):
        _react_to_event(store, task_id, event_type)

    return event_id


def _react_to_event(store: KanbanStore, task_id: str, event_type: str):
    """
    Update Kanban card state based on terminal execution events.

    Rules:
        started   → state = RUNNING
        completed → state = REVIEW (or DONE if review disabled)
        failed    → state = BLOCKED
    """
    card = store.get(task_id)
    if not card:
        return  # Card not found, nothing to do

    state_map = {
        "started": TaskState.RUNNING,
        "completed": TaskState.REVIEW,  # or TaskState.DONE if you prefer auto-done
        "failed": TaskState.BLOCKED,
    }

    new_state = state_map.get(event_type)
    if new_state and card.state != new_state:
        card.transition_to(
            new_state,
            reason=f"Execution {event_type}",
            executor="system",
        )
        store.save(card)


def get_recent_events(store: KanbanStore, task_id: Optional[str] = None, limit: int = 50) -> list[dict]:
    """
    Fetch recent execution events, optionally filtered by task_id.

    Args:
        store: KanbanStore instance
        task_id: Optional filter by card ID
        limit: Max rows to return

    Returns:
        List of event dicts (most recent first)
    """
    import sqlite3
    with sqlite3.connect(store.db_path) as conn:
        conn.row_factory = sqlite3.Row
        if task_id:
            rows = conn.execute(
                """
                SELECT * FROM execution_events 
                WHERE task_id = ? 
                ORDER BY created_at DESC 
                LIMIT ?
                """,
                (task_id, limit),
            ).fetchall()
        else:
            rows = conn.execute(
                """
                SELECT * FROM execution_events 
                ORDER BY created_at DESC 
                LIMIT ?
                """,
                (limit,),
            ).fetchall()
    
    return [dict(r) for r in rows]


def get_task_events(store: KanbanStore, task_id: str) -> list[dict]:
    """Convenience: all events for a specific task (most recent first)."""
    return get_recent_events(store, task_id=task_id, limit=1000)
