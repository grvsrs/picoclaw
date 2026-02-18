# IDE Monitor — Burst detectors
#
# Two distinct detectors:
#   1. FileBurstDetector  — filesystem activity (agent code-writing)
#   2. CopilotBurstDetector — token usage bursts (AI activity)
#
# Design principle: individual Copilot entries are meaningless;
# bursts are meaningful. Everything downstream works on bursts.

import time
import uuid
from collections import deque
from typing import Optional, List

from normalizer import WorkflowEvent, FileChange, EventType


# ═══════════════════════════════════════════════════════════════
# Filesystem burst detector (unchanged logic, renamed class)
# ═══════════════════════════════════════════════════════════════

class FileBurstDetector:
    """
    Detects rapid file modification bursts typical of agentic code writing.
    If >N non-brain files change within a short window in the same workspace,
    emit a filesystem.batch_modified event.
    """

    def __init__(self, window_secs: int = 5, threshold: int = 3):
        self.window_secs = window_secs
        self.threshold = threshold
        self._recent: deque = deque()

    def observe(self, path: str, workspace_root: str) -> Optional[WorkflowEvent]:
        now = time.monotonic()
        self._recent.append((now, path))

        # Evict old entries
        while self._recent and (now - self._recent[0][0]) > self.window_secs:
            self._recent.popleft()

        if len(self._recent) >= self.threshold:
            paths_in_window = [p for _, p in self._recent]
            self._recent.clear()  # Reset after firing

            return WorkflowEvent.make(
                source="filesystem",
                event_type=EventType.FS_BATCH_MODIFIED,
                workspace_root=workspace_root,
                files_changed=[
                    FileChange(path=p, change_type="modified")
                    for p in paths_in_window
                ],
                summary=f"{len(paths_in_window)} files changed in {self.window_secs}s burst",
                confidence=0.4,  # filesystem-only, no task context
            )
        return None


# Keep old name as alias for backward compatibility
BurstDetector = FileBurstDetector


# ═══════════════════════════════════════════════════════════════
# Copilot burst detector
# ═══════════════════════════════════════════════════════════════

class CopilotBurstDetector:
    """
    Sliding-window burst detection for Copilot completion events.

    Rules:
      - Window: configurable (default 120s)
      - Trigger: tokens_total > token_threshold OR entries > count_threshold
      - Emits: copilot.burst_start at trigger, copilot.burst_end at window close
      - Attaches: burst_id, total tokens, duration, entry count

    Each burst gets a unique burst_id that links all events in the burst.
    """

    def __init__(
        self,
        window_secs: float = 120.0,
        token_threshold: int = 500,
        count_threshold: int = 5,
    ):
        self.window_secs = window_secs
        self.token_threshold = token_threshold
        self.count_threshold = count_threshold

        # Sliding window of (monotonic_time, tokens_prompt, tokens_completion, event_id)
        self._window: deque = deque()
        self._active_burst_id: Optional[str] = None
        self._burst_start_time: Optional[float] = None
        self._burst_token_total: int = 0
        self._burst_entry_count: int = 0

    def observe(self, event: WorkflowEvent) -> List[WorkflowEvent]:
        """
        Feed a copilot.completion event. Returns 0–2 new events:
          - copilot.burst_start if this triggers a new burst
          - copilot.burst_end if the previous burst expired

        Every input event is annotated with burst_id if inside an active burst.
        """
        now = time.monotonic()
        emitted: List[WorkflowEvent] = []

        tokens = (event.tokens_prompt or 0) + (event.tokens_completion or 0)

        # Add to window
        self._window.append((now, event.tokens_prompt or 0, event.tokens_completion or 0, event.id))

        # Evict expired entries
        while self._window and (now - self._window[0][0]) > self.window_secs:
            self._window.popleft()

        # Compute window stats
        window_tokens = sum(tp + tc for _, tp, tc, _ in self._window)
        window_count = len(self._window)

        in_burst = (window_tokens >= self.token_threshold or window_count >= self.count_threshold)

        if in_burst and self._active_burst_id is None:
            # ── New burst starts ─────────────────────────────
            self._active_burst_id = str(uuid.uuid4())
            self._burst_start_time = now
            self._burst_token_total = window_tokens
            self._burst_entry_count = window_count

            start_event = WorkflowEvent.make(
                source="copilot",
                event_type=EventType.COPILOT_BURST_START,
                burst_id=self._active_burst_id,
                burst_token_total=window_tokens,
                burst_entry_count=window_count,
                summary=f"Copilot burst started: {window_count} entries, {window_tokens} tokens",
                confidence=0.4,  # burst alone = 0.2 base + 0.2 for burst
            )
            emitted.append(start_event)

        elif in_burst and self._active_burst_id is not None:
            # ── Burst continues — update running totals ──────
            self._burst_token_total = window_tokens
            self._burst_entry_count = window_count

        elif not in_burst and self._active_burst_id is not None:
            # ── Burst ends ───────────────────────────────────
            duration = now - (self._burst_start_time or now)
            end_event = WorkflowEvent.make(
                source="copilot",
                event_type=EventType.COPILOT_BURST_END,
                burst_id=self._active_burst_id,
                burst_token_total=self._burst_token_total,
                burst_duration_secs=round(duration, 1),
                burst_entry_count=self._burst_entry_count,
                summary=(
                    f"Copilot burst ended: {self._burst_entry_count} entries, "
                    f"{self._burst_token_total} tokens, {round(duration, 1)}s"
                ),
                confidence=0.4,
            )
            emitted.append(end_event)

            self._active_burst_id = None
            self._burst_start_time = None
            self._burst_token_total = 0
            self._burst_entry_count = 0

        # Annotate the original event with burst context if active
        if self._active_burst_id:
            event.burst_id = self._active_burst_id
            event.burst_token_total = self._burst_token_total
            event.burst_entry_count = self._burst_entry_count

        return emitted

    @property
    def is_in_burst(self) -> bool:
        return self._active_burst_id is not None

    @property
    def active_burst_id(self) -> Optional[str]:
        return self._active_burst_id

    def flush(self) -> Optional[WorkflowEvent]:
        """
        Force-close any active burst. Call on shutdown.
        Returns the burst_end event if a burst was active.
        """
        if self._active_burst_id is None:
            return None

        duration = time.monotonic() - (self._burst_start_time or time.monotonic())
        end_event = WorkflowEvent.make(
            source="copilot",
            event_type=EventType.COPILOT_BURST_END,
            burst_id=self._active_burst_id,
            burst_token_total=self._burst_token_total,
            burst_duration_secs=round(duration, 1),
            burst_entry_count=self._burst_entry_count,
            summary=(
                f"Copilot burst ended (flush): {self._burst_entry_count} entries, "
                f"{self._burst_token_total} tokens"
            ),
            confidence=0.4,
        )
        self._active_burst_id = None
        self._burst_start_time = None
        return end_event
