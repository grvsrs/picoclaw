# IDE Monitor — Temporal Correlation Engine
#
# Links events from different sources that describe the SAME underlying
# activity. The engine maintains sliding windows of recent events from
# each source and produces derived events:
#
#   git.commit_linked_to_task — a commit matched to an Antigravity task
#   workflow.activity_clustered — multiple sources active in same window
#
# Correlation strategies:
#   1. Task match — event.task_id == other.task_id
#   2. File overlap — changed files intersect
#   3. Time proximity — events within ±N minutes
#
# Design principle: "Git proves execution. Antigravity defines intent.
#   Picoclaw correlates truth."

import time
from collections import deque
from typing import Optional, List

from normalizer import WorkflowEvent, EventType


class TemporalCorrelator:
    """
    Multi-source temporal correlation engine.

    Replaces the old TaskCommitCorrelator with a general-purpose
    event correlation system.
    """

    # Time windows (seconds)
    TASK_COMMIT_WINDOW = 300.0    # 5 min: task completion → git commit
    CLUSTER_WINDOW = 180.0        # 3 min: activity clustering
    MAX_EVENTS = 200              # max events per sliding window

    def __init__(
        self,
        task_commit_window: float = 300.0,
        cluster_window: float = 180.0,
    ):
        self.TASK_COMMIT_WINDOW = task_commit_window
        self.CLUSTER_WINDOW = cluster_window

        # Sliding windows: deque of (monotonic_time, WorkflowEvent)
        self._tasks: deque = deque()       # antigravity task events
        self._commits: deque = deque()     # git commits
        self._fs_events: deque = deque()   # filesystem activity
        self._copilot_events: deque = deque()  # copilot completions/bursts

    def _evict_all(self, max_age: float):
        """Remove entries older than max_age from all windows."""
        now = time.monotonic()
        for window in (self._tasks, self._commits, self._fs_events, self._copilot_events):
            while window and (now - window[0][0]) > max_age:
                window.popleft()

    def _evict_window(self, window: deque, max_age: float):
        now = time.monotonic()
        while window and (now - window[0][0]) > max_age:
            window.popleft()

    def record(self, event: WorkflowEvent):
        """Record an event into the appropriate sliding window."""
        now = time.monotonic()
        entry = (now, event)

        if event.source == "antigravity" and event.task_id:
            self._tasks.append(entry)
            if len(self._tasks) > self.MAX_EVENTS:
                self._tasks.popleft()

        elif event.source == "git":
            self._commits.append(entry)
            if len(self._commits) > self.MAX_EVENTS:
                self._commits.popleft()

        elif event.source == "filesystem":
            self._fs_events.append(entry)
            if len(self._fs_events) > self.MAX_EVENTS:
                self._fs_events.popleft()

        elif event.source == "copilot":
            self._copilot_events.append(entry)
            if len(self._copilot_events) > self.MAX_EVENTS:
                self._copilot_events.popleft()

    def correlate(self, event: WorkflowEvent) -> List[WorkflowEvent]:
        """
        Process an event and return any derived correlation events.
        Also mutates the input event with correlation metadata.

        Call sequence: score → record → correlate → emit
        """
        derived: List[WorkflowEvent] = []

        if event.event_type == EventType.GIT_COMMIT:
            linked = self._correlate_commit(event)
            if linked:
                derived.append(linked)

        # Check for activity clustering
        cluster = self._check_cluster(event)
        if cluster:
            derived.append(cluster)

        return derived

    # ── Commit ↔ Task Linking ────────────────────────────────

    def _correlate_commit(self, commit: WorkflowEvent) -> Optional[WorkflowEvent]:
        """
        Link a git commit to the most recent Antigravity task.

        Returns a git.commit_linked_to_task event if a match is found.
        Also annotates the original commit event.
        """
        self._evict_window(self._tasks, self.TASK_COMMIT_WINDOW)

        if not self._tasks:
            return None

        best_task: Optional[WorkflowEvent] = None
        best_type: str = "time_proximity"

        # Strategy 1: task_id match (strongest)
        if commit.task_id:
            for _, task_ev in reversed(self._tasks):
                if task_ev.task_id == commit.task_id:
                    best_task = task_ev
                    best_type = "task_match"
                    break

        # Strategy 2: file overlap
        if best_task is None and commit.files_changed:
            commit_files = {f.path for f in commit.files_changed}
            for _, task_ev in reversed(self._tasks):
                if task_ev.files_changed:
                    task_files = {f.path for f in task_ev.files_changed}
                    if commit_files & task_files:
                        best_task = task_ev
                        best_type = "file_overlap"
                        break

        # Strategy 3: time proximity (weakest — most recent task)
        if best_task is None:
            _, best_task = self._tasks[-1]
            best_type = "time_proximity"

        # Annotate the original commit event
        commit.task_id = best_task.task_id
        commit.task_title = best_task.task_title
        commit.correlation_type = best_type
        commit.correlated_events = [best_task.id]

        # Emit a derived linked event
        return WorkflowEvent.make(
            source="git",
            event_type=EventType.GIT_COMMIT_LINKED,
            task_id=best_task.task_id,
            task_title=best_task.task_title,
            git_commit_sha=commit.git_commit_sha,
            git_branch=commit.git_branch,
            correlation_type=best_type,
            correlated_events=[commit.id, best_task.id],
            confidence=self._link_confidence(best_type),
            summary=(
                f"Commit {(commit.git_commit_sha or '?')[:8]} linked to "
                f"task '{best_task.task_title or best_task.task_id}' "
                f"via {best_type}"
            ),
        )

    def _link_confidence(self, correlation_type: str) -> float:
        """Confidence for commit-task link based on method."""
        return {
            "task_match": 1.0,
            "file_overlap": 0.8,
            "time_proximity": 0.5,
        }.get(correlation_type, 0.3)

    # ── Activity Clustering ──────────────────────────────────

    def _check_cluster(self, trigger: WorkflowEvent) -> Optional[WorkflowEvent]:
        """
        Detect activity clusters: multiple sources active within CLUSTER_WINDOW.

        Emits workflow.activity_clustered when ≥2 distinct sources are
        present in the current window.
        """
        self._evict_all(self.CLUSTER_WINDOW)

        # Collect unique sources in the cluster window
        sources = set()
        event_ids = []

        for window in (self._tasks, self._commits, self._fs_events, self._copilot_events):
            for _, ev in window:
                sources.add(ev.source)
                event_ids.append(ev.id)

        # Need ≥3 sources for a meaningful cluster
        # (e.g. antigravity + git + filesystem = real workflow)
        if len(sources) < 3:
            return None

        # Don't spam — only emit if the trigger event is from a new source
        # that brings the source count to exactly 3
        # (We'll let the dedup/debounce layer handle more if needed)
        if trigger.source not in sources:
            return None

        return WorkflowEvent.make(
            source="agent",
            event_type=EventType.WORKFLOW_ACTIVITY_CLUSTERED,
            correlated_events=event_ids[:50],  # cap for sanity
            correlation_type="temporal_cluster",
            summary=(
                f"Activity cluster: {', '.join(sorted(sources))} active "
                f"within {self.CLUSTER_WINDOW}s ({len(event_ids)} events)"
            ),
            confidence=0.7,
        )


# ── Backward compatibility ───────────────────────────────────
# Old code imported TaskCommitCorrelator; provide a thin wrapper.

class TaskCommitCorrelator:
    """Backward-compatible wrapper around TemporalCorrelator."""

    def __init__(self):
        self._engine = TemporalCorrelator()

    def record_task(self, event: WorkflowEvent):
        self._engine.record(event)

    def correlate_commit(self, commit_event: WorkflowEvent) -> WorkflowEvent:
        self._engine.record(commit_event)
        derived = self._engine.correlate(commit_event)
        # Return the annotated commit; derived events are lost in old API
        return commit_event
