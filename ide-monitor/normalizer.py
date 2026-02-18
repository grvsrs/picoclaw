# IDE Monitor — WorkflowEvent v1 (FROZEN SCHEMA)
#
# Every event from every source (Antigravity, Copilot, Git, filesystem)
# gets normalized into this single type before emission.
#
# SCHEMA RULES:
#   - Required fields MUST always be present (enforced by make())
#   - Payload is SOURCE-SPECIFIC; correlation happens OUTSIDE the payload
#   - Confidence is ALWAYS computed, never assumed
#   - event_type MUST be from the closed taxonomy below
#   - New fields are additive only (no removals until v2)

from dataclasses import dataclass, field, asdict
from typing import Optional, List, Dict, Any
from datetime import datetime, timezone
import json
import uuid

SPEC_VERSION = "1.0"


# ═══════════════════════════════════════════════════════════════
# EVENT TAXONOMY — CLOSED SET (no ad-hoc strings beyond this)
# ═══════════════════════════════════════════════════════════════

class EventType:
    """All valid event_type values. Nothing else is permitted."""

    # ── Copilot ──────────────────────────────────────────────
    COPILOT_PROMPT       = "copilot.prompt"
    COPILOT_COMPLETION   = "copilot.completion"
    COPILOT_ERROR        = "copilot.error"
    COPILOT_BURST_START  = "copilot.burst_start"
    COPILOT_BURST_END    = "copilot.burst_end"

    # ── Antigravity ──────────────────────────────────────────
    AG_TASK_CREATED      = "antigravity.task.created"
    AG_TASK_UPDATED      = "antigravity.task.updated"
    AG_TASK_PROGRESS     = "antigravity.task.progress"
    AG_TASK_COMPLETED    = "antigravity.task.completed"
    AG_TASK_FAILED       = "antigravity.task.failed"
    AG_TASK_WALKTHROUGH  = "antigravity.task.walkthrough_added"
    AG_TASK_PLAN_READY   = "antigravity.task.plan_ready"
    AG_TASK_ITERATED     = "antigravity.task.iterated"
    AG_SKILL_UPDATED     = "antigravity.skill.updated"
    AG_ARTIFACT_UNKNOWN  = "antigravity.artifact.unknown"

    # ── Git ──────────────────────────────────────────────────
    GIT_COMMIT           = "git.commit"
    GIT_COMMIT_LINKED    = "git.commit_linked_to_task"

    # ── Filesystem ───────────────────────────────────────────
    FS_BATCH_MODIFIED    = "filesystem.batch_modified"

    # ── Derived / System ─────────────────────────────────────
    WORKFLOW_TASK_INFERRED      = "workflow.task_inferred"
    WORKFLOW_ACTIVITY_CLUSTERED = "workflow.activity_clustered"
    WORKFLOW_CONFLICT_DETECTED  = "workflow.conflict_detected"

    _ALL = None  # populated below

    @classmethod
    def all_types(cls) -> set:
        if cls._ALL is None:
            cls._ALL = {
                v for k, v in vars(cls).items()
                if isinstance(v, str) and not k.startswith("_")
            }
        return cls._ALL

    @classmethod
    def is_valid(cls, event_type: str) -> bool:
        return event_type in cls.all_types()


# ═══════════════════════════════════════════════════════════════
# DATA MODEL
# ═══════════════════════════════════════════════════════════════

@dataclass
class FileChange:
    """A single file that changed."""
    path: str                          # relative to workspace root
    change_type: str                   # "created" | "modified" | "deleted"
    size_bytes: Optional[int] = None


@dataclass
class WorkflowEvent:
    """
    Canonical event emitted by the IDE monitor.
    Both Picoclaw and standalone consumers use this schema.

    REQUIRED (always set by make()):
        id, spec_version, source, event_type, timestamp, workspace_id

    OPTIONAL but NORMALIZED:
        task_id, task_title, external_ref, confidence
    """

    # ── Identity (REQUIRED) ──────────────────────────────────
    id: str = ""                       # uuid4, always set by make()
    spec_version: str = SPEC_VERSION   # for schema migration
    source: str = ""                   # "antigravity" | "copilot" | "git" | "filesystem" | "agent"
    event_type: str = ""               # from EventType taxonomy
    event_version: int = 1             # per-event-type schema version
    timestamp: str = ""                # ISO8601 UTC
    hostname: Optional[str] = None     # multi-machine support
    workspace_id: Optional[str] = None # stable workspace identifier

    # ── Confidence (ALWAYS computed) ─────────────────────────
    confidence: float = 0.0            # 0.0–1.0, computed by scoring engine

    # ── Task Context ─────────────────────────────────────────
    task_id: Optional[str] = None      # antigravity GUID | copilot session | git SHA
    task_title: Optional[str] = None   # first heading from task.md
    task_status: Optional[str] = None  # "planning"|"in_progress"|"complete"|"failed"
    iteration: Optional[int] = None    # resolved.N count
    external_ref: Optional[str] = None # workspace_id:task_id — stable key for Kanban

    # ── Artifact ─────────────────────────────────────────────
    artifact_type: Optional[str] = None  # "task"|"plan"|"walkthrough"|"skill"|"log"
    artifact_path: Optional[str] = None  # absolute path
    summary: Optional[str] = None        # extracted text, max 300 chars

    # ── Token Data (copilot only — always null for others) ───
    tokens_prompt: Optional[int] = None
    tokens_completion: Optional[int] = None
    model: Optional[str] = None

    # ── Burst Context (populated by burst detector) ──────────
    burst_id: Optional[str] = None     # links events in the same burst
    burst_token_total: Optional[int] = None
    burst_duration_secs: Optional[float] = None
    burst_entry_count: Optional[int] = None

    # ── File Activity ────────────────────────────────────────
    files_changed: Optional[List[FileChange]] = None
    workspace_root: Optional[str] = None

    # ── Git Correlation ──────────────────────────────────────
    git_commit_sha: Optional[str] = None
    git_branch: Optional[str] = None

    # ── Correlation Metadata ─────────────────────────────────
    correlated_events: Optional[List[str]] = None  # event IDs in this cluster
    correlation_type: Optional[str] = None          # "time_proximity"|"file_overlap"|"task_match"

    # ── Extension Point ──────────────────────────────────────
    raw: Optional[Dict[str, Any]] = None

    @classmethod
    def make(cls, source: str, event_type: str, **kwargs) -> "WorkflowEvent":
        """Factory that auto-sets id, spec_version, timestamp, hostname."""
        import socket
        import hashlib
        import os

        # Compute stable workspace_id from workspace_root if available
        ws_root = kwargs.get("workspace_root") or os.getcwd()
        workspace_id = kwargs.pop("workspace_id", None)
        if workspace_id is None:
            workspace_id = hashlib.sha256(ws_root.encode()).hexdigest()[:12]

        # Compute external_ref if task_id is present
        task_id = kwargs.get("task_id")
        external_ref = kwargs.get("external_ref")
        if task_id and not external_ref:
            external_ref = f"{workspace_id}:{task_id}"

        return cls(
            id=str(uuid.uuid4()),
            spec_version=SPEC_VERSION,
            source=source,
            event_type=event_type,
            timestamp=datetime.now(timezone.utc).isoformat(),
            hostname=socket.gethostname(),
            workspace_id=workspace_id,
            external_ref=external_ref,
            **kwargs,
        )

    def to_json(self) -> str:
        """Serialize to JSON, omitting null/zero-default fields for lean payloads."""
        d = asdict(self)
        # Remove None values and default zeros (but keep confidence even if 0.0)
        return json.dumps({
            k: v for k, v in d.items()
            if v is not None and not (k != "confidence" and v == 0)
        })
