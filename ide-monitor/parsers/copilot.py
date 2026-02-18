# IDE Monitor — VS Code Copilot log parser
#
# Reads JSON log files from ~/.config/Code/User/globalStorage/github.copilot/logs/
# Extracts token usage data from completion entries.

import json
from pathlib import Path
from typing import Optional, List

from normalizer import WorkflowEvent, EventType


def parse_copilot_entries(file_path: str) -> List[WorkflowEvent]:
    """
    Parse a Copilot log file and return WorkflowEvents for entries
    that contain token data.

    Copilot log formats vary between versions — we try multiple field names.
    """
    events = []
    path = Path(file_path)

    if not path.exists() or path.stat().st_size > 10_000_000:  # 10MB cap
        return events

    try:
        with open(path, "r", encoding="utf-8", errors="replace") as f:
            for line in f:
                ev = _parse_line(line.strip(), str(path))
                if ev:
                    events.append(ev)
    except Exception:
        pass

    return events


def _parse_line(line: str, source_path: str) -> Optional[WorkflowEvent]:
    """Try to parse a single log line as a completion event."""
    if not line:
        return None

    try:
        entry = json.loads(line)
    except (json.JSONDecodeError, ValueError):
        return None

    if not isinstance(entry, dict):
        return None

    # Try multiple field names — copilot log format varies
    tokens_prompt = (
        entry.get("tokens_prompt")
        or entry.get("promptTokens")
        or entry.get("prompt_tokens")
    )
    tokens_completion = (
        entry.get("tokens_completion")
        or entry.get("completionTokens")
        or entry.get("completion_tokens")
    )

    # If this entry has no token data, check if it's an error
    if tokens_prompt is None and tokens_completion is None:
        if "error" in entry or "Error" in entry:
            return WorkflowEvent.make(
                source="copilot",
                event_type=EventType.COPILOT_ERROR,
                artifact_path=source_path,
                summary=str(entry.get("error") or entry.get("Error", ""))[:200],
            )
        return None

    # Ensure int types
    try:
        tokens_prompt = int(tokens_prompt) if tokens_prompt else None
    except (ValueError, TypeError):
        tokens_prompt = None
    try:
        tokens_completion = int(tokens_completion) if tokens_completion else None
    except (ValueError, TypeError):
        tokens_completion = None

    timestamp = entry.get("timestamp") or entry.get("ts") or entry.get("time")

    return WorkflowEvent.make(
        source="copilot",
        event_type=EventType.COPILOT_COMPLETION,
        task_id=entry.get("sessionId") or entry.get("session_id"),
        artifact_path=source_path,
        summary=(entry.get("prompt") or "")[:100] or None,
        tokens_prompt=tokens_prompt,
        tokens_completion=tokens_completion,
        model=entry.get("model"),
        raw={"original_timestamp": timestamp} if timestamp else None,
    )
