# IDE Monitor — Confidence scoring engine
#
# Implements the scoring rules from the development plan:
#
#   Base confidence:  0.2
#   + 0.2  if in a burst
#   + 0.2  if near filesystem edits (temporal proximity)
#   + 0.2  if near git commit (temporal proximity)
#   + 0.2  if task context exists (antigravity task_id linked)
#   Cap at 1.0
#
# Design principle:
#   Copilot is EVIDENCE, not INTENT. Antigravity defines intent.
#   Confidence must be computed, never assumed.

import time
from collections import deque
from typing import Optional

from normalizer import WorkflowEvent, EventType


class ConfidenceScorer:
    """
    Scores the confidence of each WorkflowEvent.

    Maintains sliding windows of recent signals to compute
    temporal proximity bonuses.
    """

    # Time windows for proximity checks
    FS_PROXIMITY_SECS = 30.0     # filesystem edits within 30s
    GIT_PROXIMITY_SECS = 120.0   # git commits within 120s
    TASK_PROXIMITY_SECS = 300.0  # antigravity task events within 5 min

    # Score components
    BASE = 0.2
    BURST_BONUS = 0.2
    FS_PROXIMITY_BONUS = 0.2
    GIT_PROXIMITY_BONUS = 0.2
    TASK_CONTEXT_BONUS = 0.2

    def __init__(self):
        # Sliding windows: deque of (monotonic_time, event_id)
        self._recent_fs: deque = deque()
        self._recent_git: deque = deque()
        self._recent_tasks: deque = deque()

    def _evict(self, window: deque, max_age: float):
        """Remove entries older than max_age seconds."""
        now = time.monotonic()
        while window and (now - window[0][0]) > max_age:
            window.popleft()

    def record_signal(self, event: WorkflowEvent):
        """
        Record an event as a signal for future scoring.
        Call this AFTER scoring but BEFORE emission.
        """
        now = time.monotonic()

        if event.source == "filesystem" or event.event_type == EventType.FS_BATCH_MODIFIED:
            self._recent_fs.append((now, event.id))

        elif event.source == "git" or event.event_type == EventType.GIT_COMMIT:
            self._recent_git.append((now, event.id))

        elif event.source == "antigravity" and event.task_id:
            self._recent_tasks.append((now, event.task_id))

    def score(self, event: WorkflowEvent) -> float:
        """
        Compute confidence score for an event.

        Rules vary by source:
          - Antigravity: always high (it defines intent)
          - Git: high when linked to task
          - Copilot: starts low, boosted by context
          - Filesystem: moderate
          - Derived: computed by correlator
        """
        # Antigravity events are authoritative — intent source
        if event.source == "antigravity":
            return self._score_antigravity(event)

        # Git events
        if event.source == "git":
            return self._score_git(event)

        # Copilot events — the careful one
        if event.source == "copilot":
            return self._score_copilot(event)

        # Filesystem
        if event.source == "filesystem":
            return self._score_filesystem(event)

        # Derived/system events keep whatever confidence was pre-set
        return event.confidence

    def _score_antigravity(self, event: WorkflowEvent) -> float:
        """Antigravity defines intent — always high confidence."""
        if event.event_type in (
            EventType.AG_TASK_COMPLETED,
            EventType.AG_TASK_WALKTHROUGH,
        ):
            return 1.0
        if event.event_type in (
            EventType.AG_TASK_CREATED,
            EventType.AG_TASK_PLAN_READY,
        ):
            return 0.9
        if event.event_type == EventType.AG_TASK_FAILED:
            return 0.9
        if event.event_type == EventType.AG_ARTIFACT_UNKNOWN:
            return 0.3
        return 0.8  # default for other antigravity events

    def _score_git(self, event: WorkflowEvent) -> float:
        """Git proves execution — high when linked."""
        base = 0.6

        # Bonus for task linkage
        if event.task_id:
            self._evict(self._recent_tasks, self.TASK_PROXIMITY_SECS)
            for _, tid in self._recent_tasks:
                if tid == event.task_id:
                    return min(1.0, base + self.TASK_CONTEXT_BONUS)

        # Bonus for temporal proximity to task events
        self._evict(self._recent_tasks, self.TASK_PROXIMITY_SECS)
        if self._recent_tasks:
            base += 0.1

        return min(1.0, base)

    def _score_copilot(self, event: WorkflowEvent) -> float:
        """
        Copilot is evidence, not intent.
        Individual completions start at 0.2. Build up with context.
        """
        score = self.BASE

        # +0.2 if in a burst
        if event.burst_id:
            score += self.BURST_BONUS

        # +0.2 if near filesystem edits
        self._evict(self._recent_fs, self.FS_PROXIMITY_SECS)
        if self._recent_fs:
            score += self.FS_PROXIMITY_BONUS

        # +0.2 if near git commit
        self._evict(self._recent_git, self.GIT_PROXIMITY_SECS)
        if self._recent_git:
            score += self.GIT_PROXIMITY_BONUS

        # +0.2 if task context exists
        if event.task_id:
            score += self.TASK_CONTEXT_BONUS
        else:
            self._evict(self._recent_tasks, self.TASK_PROXIMITY_SECS)
            if self._recent_tasks:
                score += self.TASK_CONTEXT_BONUS * 0.5  # 0.1 for proximity without link

        return min(1.0, score)

    def _score_filesystem(self, event: WorkflowEvent) -> float:
        """Filesystem is supporting evidence."""
        base = 0.4

        # Bonus for task proximity
        self._evict(self._recent_tasks, self.TASK_PROXIMITY_SECS)
        if self._recent_tasks:
            base += 0.2

        # Bonus for git proximity
        self._evict(self._recent_git, self.GIT_PROXIMITY_SECS)
        if self._recent_git:
            base += 0.1

        return min(1.0, base)

    def score_and_record(self, event: WorkflowEvent) -> WorkflowEvent:
        """
        Score an event and record it as a signal. Returns the mutated event.
        This is the primary API — call this for every event before emission.
        """
        event.confidence = self.score(event)
        self.record_signal(event)
        return event
