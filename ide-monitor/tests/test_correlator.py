"""Tests for the temporal correlation engine."""
import sys
import os
import pytest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from normalizer import WorkflowEvent, EventType, FileChange
from correlator import TemporalCorrelator, TaskCommitCorrelator


class TestTemporalCorrelator:
    """Tests for the TemporalCorrelator class."""

    def setup_method(self):
        self.corr = TemporalCorrelator()

    # ── Commit ↔ Task Linking ────────────────────────────────

    def test_commit_links_to_recent_task_by_time(self):
        task = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_COMPLETED,
            task_id="guid-1", task_title="Implement feature X",
        )
        self.corr.record(task)

        commit = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
            git_commit_sha="abc123", git_branch="main",
        )
        self.corr.record(commit)

        derived = self.corr.correlate(commit)
        assert len(derived) >= 1

        linked = [d for d in derived if d.event_type == EventType.GIT_COMMIT_LINKED]
        assert len(linked) == 1
        assert linked[0].task_id == "guid-1"
        assert linked[0].correlation_type == "time_proximity"

    def test_commit_links_by_file_overlap(self):
        task = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_COMPLETED,
            task_id="guid-1",
            files_changed=[FileChange(path="src/auth.py", change_type="modified")],
        )
        self.corr.record(task)

        commit = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
            git_commit_sha="abc123",
            files_changed=[
                FileChange(path="src/auth.py", change_type="modified"),
                FileChange(path="src/test.py", change_type="created"),
            ],
        )
        self.corr.record(commit)

        derived = self.corr.correlate(commit)
        linked = [d for d in derived if d.event_type == EventType.GIT_COMMIT_LINKED]
        assert len(linked) == 1
        assert linked[0].correlation_type == "file_overlap"

    def test_commit_links_by_task_match(self):
        task = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_COMPLETED,
            task_id="guid-1",
        )
        self.corr.record(task)

        commit = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
            git_commit_sha="abc123",
            task_id="guid-1",  # explicit match
        )
        self.corr.record(commit)

        derived = self.corr.correlate(commit)
        linked = [d for d in derived if d.event_type == EventType.GIT_COMMIT_LINKED]
        assert len(linked) == 1
        assert linked[0].correlation_type == "task_match"
        assert linked[0].confidence == 1.0

    def test_no_link_without_recent_task(self):
        commit = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
            git_commit_sha="abc123",
        )
        self.corr.record(commit)
        derived = self.corr.correlate(commit)
        linked = [d for d in derived if d.event_type == EventType.GIT_COMMIT_LINKED]
        assert len(linked) == 0

    def test_commit_annotated_with_task_context(self):
        task = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_COMPLETED,
            task_id="guid-1", task_title="Fix bug",
        )
        self.corr.record(task)

        commit = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
            git_commit_sha="abc123",
        )
        self.corr.record(commit)
        self.corr.correlate(commit)

        # The original commit event should be annotated
        assert commit.task_id == "guid-1"
        assert commit.task_title == "Fix bug"
        assert commit.correlation_type is not None

    # ── Activity Clustering ──────────────────────────────────

    def test_cluster_requires_3_sources(self):
        # Only 2 sources — not enough
        task = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_CREATED, task_id="g1",
        )
        commit = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
        )
        self.corr.record(task)
        self.corr.record(commit)

        derived = self.corr.correlate(commit)
        clusters = [d for d in derived if d.event_type == EventType.WORKFLOW_ACTIVITY_CLUSTERED]
        assert len(clusters) == 0

    def test_cluster_fires_with_3_sources(self):
        task = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_CREATED, task_id="g1",
        )
        fs = WorkflowEvent.make(
            source="filesystem", event_type=EventType.FS_BATCH_MODIFIED,
        )
        commit = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
        )

        self.corr.record(task)
        self.corr.record(fs)
        self.corr.record(commit)

        derived = self.corr.correlate(commit)
        clusters = [d for d in derived if d.event_type == EventType.WORKFLOW_ACTIVITY_CLUSTERED]
        assert len(clusters) == 1
        assert "antigravity" in clusters[0].summary
        assert "filesystem" in clusters[0].summary
        assert "git" in clusters[0].summary


class TestBackwardCompatibility:
    """Ensure TaskCommitCorrelator still works."""

    def test_old_api_still_works(self):
        tc = TaskCommitCorrelator()

        task = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_COMPLETED,
            task_id="guid-1", task_title="Old API task",
        )
        tc.record_task(task)

        commit = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
            git_commit_sha="abc123",
        )
        result = tc.correlate_commit(commit)
        assert result.task_id == "guid-1"
