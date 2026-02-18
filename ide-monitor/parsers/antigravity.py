# IDE Monitor — Antigravity artifact parser
#
# Reads .gemini/antigravity/brain/<guid>/ directories.
# Parses task.md, implementation_plan.md, walkthrough.md into WorkflowEvents.
# Defensive: never raises, never assumes schema stability.

import re
from pathlib import Path
from typing import Optional

from normalizer import WorkflowEvent, EventType


def _safe_read(path: Path, max_bytes: int = 2_000_000) -> Optional[str]:
    """Read a file without ever raising. Returns None on any failure."""
    try:
        if path.stat().st_size > max_bytes:
            return None
        return path.read_text(encoding="utf-8", errors="replace")
    except Exception:
        return None


def _extract_title(md: str) -> Optional[str]:
    """First non-empty heading or first non-empty line."""
    for line in md.splitlines():
        stripped = line.lstrip("#").strip()
        if stripped:
            return stripped[:120]
    return None


def _extract_summary(md: str, max_chars: int = 300) -> Optional[str]:
    """Skip headings and checkboxes; return first substantive paragraph."""
    lines = []
    for line in md.splitlines():
        s = line.strip()
        if not s:
            continue
        if s.startswith("#"):
            continue
        if re.match(r"^- \[.?\]", s):
            continue
        lines.append(s)
        if sum(len(l) for l in lines) >= max_chars:
            break
    text = " ".join(lines)
    return text[:max_chars] if text else None


def _task_status_from_md(md: str) -> str:
    """
    Infer status from checkbox state.
    Deliberately lenient — different Antigravity versions may use different markers.
    """
    has_open = bool(re.search(r"- \[ \]", md))
    has_done = bool(re.search(r"- \[x\]", md, re.IGNORECASE))
    has_fail = bool(re.search(r"failed|error|could not", md, re.IGNORECASE))

    if has_fail and not has_done:
        return "failed"
    if has_done and not has_open:
        return "complete"
    if has_done or has_open:
        return "in_progress"
    return "planning"


def _resolved_count(task_dir: Path) -> int:
    """Count .resolved.N backup files — indicates replanning depth."""
    return len(list(task_dir.glob("*.resolved.*")))


def _is_temp_file(path: Path) -> bool:
    """Ignore editor temp/swap files."""
    return path.name.startswith(".") or path.suffix in {".swp", ".tmp", ".bak"}


def parse_brain_event(changed_path: str, brain_dir: str) -> Optional[WorkflowEvent]:
    """
    Parse a filesystem change inside the Antigravity brain directory
    into a WorkflowEvent.

    Returns None if the change is not meaningful.
    """
    path = Path(changed_path).resolve()
    brain = Path(brain_dir).resolve()

    # Guard: must be inside brain dir
    try:
        path.relative_to(brain)
    except ValueError:
        return None

    if _is_temp_file(path):
        return None

    task_dir = path.parent
    task_guid = task_dir.name
    stem = path.stem

    # Strip .resolved.N suffix — treat as the base artifact
    base_stem = re.sub(r"\.resolved\.\d+$", "", stem)

    # Map stem → event type
    event_map = {
        "task": EventType.AG_TASK_CREATED,  # refined below
        "implementation_plan": EventType.AG_TASK_PLAN_READY,
        "walkthrough": EventType.AG_TASK_COMPLETED,
    }

    if base_stem not in event_map:
        # Unknown artifact — emit as raw event, don't drop
        return WorkflowEvent.make(
            source="antigravity",
            event_type=EventType.AG_ARTIFACT_UNKNOWN,
            task_id=task_guid,
            artifact_type=base_stem,
            artifact_path=str(path),
        )

    event_type = event_map[base_stem]
    content = _safe_read(path)

    task_status = None
    title = None
    iteration = None

    if base_stem == "task" and content:
        task_status = _task_status_from_md(content)
        if task_status == "complete":
            event_type = EventType.AG_TASK_COMPLETED
        elif task_status == "failed":
            event_type = EventType.AG_TASK_FAILED
        else:
            event_type = EventType.AG_TASK_CREATED

        # Detect iteration from .resolved.N presence
        iteration = _resolved_count(task_dir)
        if iteration > 0:
            event_type = EventType.AG_TASK_ITERATED

    elif base_stem == "walkthrough":
        task_status = "complete"

    elif base_stem == "implementation_plan":
        task_status = "planning"

    # Extract title from task.md preferentially
    task_md = _safe_read(task_dir / "task.md")
    if task_md:
        title = _extract_title(task_md)

    return WorkflowEvent.make(
        source="antigravity",
        event_type=event_type,
        task_id=task_guid,
        task_title=title,
        task_status=task_status,
        iteration=iteration if iteration and iteration > 0 else None,
        artifact_type=base_stem,
        artifact_path=str(path),
        summary=_extract_summary(content) if content else None,
    )


def parse_skill_event(changed_path: str) -> Optional[WorkflowEvent]:
    """Parse a change in the skills directory."""
    path = Path(changed_path)
    if _is_temp_file(path):
        return None
    if not path.name.endswith(".md"):
        return None

    content = _safe_read(path)
    return WorkflowEvent.make(
        source="antigravity",
        event_type=EventType.AG_SKILL_UPDATED,
        artifact_type="skill",
        artifact_path=str(path),
        summary=_extract_summary(content or ""),
    )
