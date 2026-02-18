"""
Kanban card schema and state machine.

Task lifecycle:
  Inbox → Planned → Running → Blocked → Review → Done
  
Each card is auditable and immutable-append-only for state changes.
"""
from enum import Enum
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from typing import Optional, List, Dict, Any
import json


class TaskState(Enum):
    """Valid task states in the Kanban workflow."""
    INBOX = "inbox"          # Created, not yet planned
    PLANNED = "planned"      # Scheduled for execution
    RUNNING = "running"      # Currently executing
    BLOCKED = "blocked"      # Waiting on something external
    REVIEW = "review"        # Completed, awaiting approval/review
    DONE = "done"            # Fully completed

    @classmethod
    def from_str(cls, value: str) -> "TaskState":
        try:
            return cls[value.upper()]
        except KeyError:
            return cls.INBOX


class TaskCategory(Enum):
    """LLM-assignable task categories."""
    CODE = "code"
    DESIGN = "design"
    INFRA = "infra"
    BUG = "bug"
    FEATURE = "feature"
    RESEARCH = "research"
    OPS = "ops"
    PERSONAL = "personal"
    MEETING = "meeting"
    UNCATEGORIZED = "uncategorized"

    @classmethod
    def from_str(cls, value: str) -> "TaskCategory":
        try:
            return cls[value.upper()]
        except KeyError:
            return cls.UNCATEGORIZED


class TaskSource(Enum):
    """Where the task originated."""
    TELEGRAM = "telegram"
    VSCODE = "vscode"
    API = "api"
    CLI = "cli"
    LLM = "llm"
    MANUAL = "manual"

    @classmethod
    def from_str(cls, value: str) -> "TaskSource":
        try:
            return cls[value.upper()]
        except KeyError:
            return cls.MANUAL


class TaskMode(Enum):
    """Execution mode for the task."""
    PERSONAL = "personal"    # Personal mode: full shell, no approvals
    REMOTE = "remote"        # Remote mode: sandboxed, requires card, rate-limited


@dataclass
class StateTransition:
    """One pending state transition, queued for DB flush."""
    from_state: TaskState
    to_state: TaskState
    reason: Optional[str] = None
    executor: Optional[str] = None
    timestamp: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())

    def to_dict(self) -> Dict[str, Any]:
        return {
            "from_state": self.from_state.value,
            "to_state": self.to_state.value,
            "timestamp": self.timestamp,
            "reason": self.reason or "",
            "executor": self.executor or "",
        }


# Keep old name as alias for backwards compatibility
TaskStateChange = StateTransition


@dataclass
class KanbanCard:
    """Core Kanban card representing a task."""
    
    # Identifiers
    card_id: str                    # Unique card ID (e.g., KAN-142)
    
    # Content
    title: str
    description: str = ""
    
    # Classification (LLM-powered)
    category: TaskCategory = TaskCategory.UNCATEGORIZED
    source: TaskSource = TaskSource.MANUAL
    project: str = ""              # Project grouping (e.g., "picoclaw", "glass-walls")
    
    # Execution
    mode: TaskMode = TaskMode.PERSONAL
    executor: str = "picoclaw"     # "picoclaw", "manual", "vscode", etc.
    allowed_users: List[str] = field(default_factory=list)
    
    # Priority & scheduling
    priority: str = "normal"       # "low", "normal", "high", "critical"
    due_date: Optional[datetime] = None
    
    # State
    state: TaskState = TaskState.INBOX
    state_history: List[Dict[str, Any]] = field(default_factory=list)
    _pending_transitions: List[StateTransition] = field(default_factory=list, repr=False)
    
    # Links
    telegram_message_id: Optional[str] = None  # Original Telegram message
    vscode_task_id: Optional[str] = None       # VS Code task reference
    external_ref: Optional[str] = None         # Any external reference
    
    # Execution tracking
    execution_log_url: str = ""    # Link to detailed logs (not raw output)
    attempts: int = 0
    last_attempt_time: Optional[datetime] = None
    last_failure_reason: str = ""
    
    # LLM metadata
    llm_categorized: bool = False  # Whether category was assigned by LLM
    llm_summary: str = ""          # LLM-generated task summary
    
    # Metadata
    tags: List[str] = field(default_factory=list)
    assignee: str = ""
    created_by: str = ""
    created_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))
    updated_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))
    
    def transition_to(
        self, 
        new_state: TaskState, 
        reason: str = "", 
        executor: str = ""
    ) -> bool:
        """Attempt a state transition. Returns True if successful."""
        # Allowed transitions
        allowed_next = {
            TaskState.INBOX: [TaskState.PLANNED, TaskState.RUNNING],  # Allow direct INBOX → RUNNING for terminal
            TaskState.PLANNED: [TaskState.RUNNING, TaskState.BLOCKED],
            TaskState.RUNNING: [TaskState.BLOCKED, TaskState.REVIEW, TaskState.DONE],
            TaskState.BLOCKED: [TaskState.RUNNING, TaskState.PLANNED],
            TaskState.REVIEW: [TaskState.DONE, TaskState.BLOCKED],
            TaskState.DONE: [],  # Terminal state
        }
        
        if new_state not in allowed_next.get(self.state, []):
            return False
        
        # Queue a DB row for store.save() to flush
        t = StateTransition(
            from_state=self.state,
            to_state=new_state,
            reason=reason or None,
            executor=executor or None,
        )
        self._pending_transitions.append(t)
        # Also keep in-memory history for immediate reads
        self.state_history.append(t.to_dict())
        self.state = new_state
        self.updated_at = datetime.now(timezone.utc)
        return True
    
    def to_dict(self) -> Dict[str, Any]:
        """Serialize to dict, preserving state history."""
        return {
            "card_id": self.card_id,
            "title": self.title,
            "description": self.description,
            "category": self.category.value if isinstance(self.category, TaskCategory) else self.category,
            "source": self.source.value if isinstance(self.source, TaskSource) else self.source,
            "project": self.project,
            "mode": self.mode.value,
            "executor": self.executor,
            "allowed_users": self.allowed_users,
            "priority": self.priority,
            "due_date": self.due_date.isoformat() if self.due_date else None,
            "state": self.state.value,
            "state_history": self.state_history,
            "telegram_message_id": self.telegram_message_id,
            "vscode_task_id": self.vscode_task_id,
            "external_ref": self.external_ref,
            "execution_log_url": self.execution_log_url,
            "attempts": self.attempts,
            "last_attempt_time": self.last_attempt_time.isoformat() if self.last_attempt_time else None,
            "last_failure_reason": self.last_failure_reason,
            "llm_categorized": self.llm_categorized,
            "llm_summary": self.llm_summary,
            "tags": self.tags,
            "assignee": self.assignee,
            "created_by": self.created_by,
            "created_at": self.created_at.isoformat() if isinstance(self.created_at, datetime) else self.created_at,
            "updated_at": self.updated_at.isoformat() if isinstance(self.updated_at, datetime) else self.updated_at,
        }
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "KanbanCard":
        """Deserialize from dict."""
        state_history = data.get("state_history", [])
        if isinstance(state_history, str):
            try:
                state_history = json.loads(state_history)
            except Exception:
                state_history = []
        
        # Handle category (backward compatible — old cards won't have it)
        category = TaskCategory.UNCATEGORIZED
        if data.get("category"):
            try:
                category = TaskCategory(data["category"])
            except ValueError:
                category = TaskCategory.UNCATEGORIZED
        
        # Handle source (backward compatible)
        source = TaskSource.MANUAL
        if data.get("source"):
            try:
                source = TaskSource(data["source"])
            except ValueError:
                source = TaskSource.MANUAL
        
        card = cls(
            card_id=data.get("card_id", ""),
            title=data.get("title", ""),
            description=data.get("description", ""),
            category=category,
            source=source,
            project=data.get("project", ""),
            mode=TaskMode(data.get("mode", "personal")),
            executor=data.get("executor", "picoclaw"),
            allowed_users=data.get("allowed_users", []),
            priority=data.get("priority", "normal"),
            due_date=datetime.fromisoformat(data["due_date"]) if data.get("due_date") else None,
            state=TaskState(data.get("state", "inbox")),
            telegram_message_id=data.get("telegram_message_id"),
            vscode_task_id=data.get("vscode_task_id"),
            external_ref=data.get("external_ref"),
            execution_log_url=data.get("execution_log_url", ""),
            attempts=data.get("attempts", 0),
            last_attempt_time=datetime.fromisoformat(data["last_attempt_time"]) if data.get("last_attempt_time") else None,
            last_failure_reason=data.get("last_failure_reason", ""),
            llm_categorized=bool(data.get("llm_categorized", False)),
            llm_summary=data.get("llm_summary", ""),
            tags=data.get("tags", []),
            assignee=data.get("assignee", ""),
            created_by=data.get("created_by", ""),
            created_at=datetime.fromisoformat(data["created_at"]) if data.get("created_at") else datetime.now(timezone.utc),
            updated_at=datetime.fromisoformat(data["updated_at"]) if data.get("updated_at") else datetime.now(timezone.utc),
        )
        card.state_history = state_history if isinstance(state_history, list) else []
        return card
