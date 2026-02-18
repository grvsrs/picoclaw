"""Tests for confidence scoring engine."""
import sys
import os
import time
import pytest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from normalizer import WorkflowEvent, EventType
from confidence import ConfidenceScorer


class TestConfidenceScorer:
    """Tests for the ConfidenceScorer class."""

    def setup_method(self):
        self.scorer = ConfidenceScorer()

    # ── Antigravity (intent source — always high) ────────────

    def test_antigravity_task_created_high(self):
        ev = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_CREATED,
            task_id="guid-1",
        )
        score = self.scorer.score(ev)
        assert score >= 0.8

    def test_antigravity_task_completed_is_1(self):
        ev = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_COMPLETED,
            task_id="guid-1",
        )
        score = self.scorer.score(ev)
        assert score == 1.0

    def test_antigravity_artifact_unknown_low(self):
        ev = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_ARTIFACT_UNKNOWN,
        )
        score = self.scorer.score(ev)
        assert score <= 0.4

    # ── Copilot (evidence, not intent — starts low) ─────────

    def test_copilot_base_is_0_2(self):
        ev = WorkflowEvent.make(
            source="copilot", event_type=EventType.COPILOT_COMPLETION,
        )
        score = self.scorer.score(ev)
        assert score == pytest.approx(0.2)

    def test_copilot_with_burst_is_0_4(self):
        ev = WorkflowEvent.make(
            source="copilot", event_type=EventType.COPILOT_COMPLETION,
            burst_id="burst-001",
        )
        score = self.scorer.score(ev)
        assert score == pytest.approx(0.4)

    def test_copilot_with_burst_and_fs_proximity(self):
        # Record a filesystem event first
        fs_event = WorkflowEvent.make(
            source="filesystem", event_type=EventType.FS_BATCH_MODIFIED,
        )
        self.scorer.record_signal(fs_event)

        ev = WorkflowEvent.make(
            source="copilot", event_type=EventType.COPILOT_COMPLETION,
            burst_id="burst-001",
        )
        score = self.scorer.score(ev)
        # base 0.2 + burst 0.2 + fs 0.2 = 0.6
        assert score == pytest.approx(0.6)

    def test_copilot_with_all_signals_caps_at_1(self):
        # Record all signals
        fs_event = WorkflowEvent.make(
            source="filesystem", event_type=EventType.FS_BATCH_MODIFIED,
        )
        git_event = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
        )
        task_event = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_CREATED,
            task_id="guid-1",
        )
        self.scorer.record_signal(fs_event)
        self.scorer.record_signal(git_event)
        self.scorer.record_signal(task_event)

        ev = WorkflowEvent.make(
            source="copilot", event_type=EventType.COPILOT_COMPLETION,
            burst_id="burst-001",
            task_id="guid-1",
        )
        score = self.scorer.score(ev)
        # base 0.2 + burst 0.2 + fs 0.2 + git 0.2 + task 0.2 = 1.0
        assert score == pytest.approx(1.0)

    def test_copilot_without_task_but_near_task_gets_partial(self):
        task_event = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_CREATED,
            task_id="guid-1",
        )
        self.scorer.record_signal(task_event)

        ev = WorkflowEvent.make(
            source="copilot", event_type=EventType.COPILOT_COMPLETION,
        )
        score = self.scorer.score(ev)
        # base 0.2 + task_proximity 0.1 = 0.3
        assert score == pytest.approx(0.3)

    # ── Git ──────────────────────────────────────────────────

    def test_git_base_is_0_6(self):
        ev = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
        )
        score = self.scorer.score(ev)
        assert score == pytest.approx(0.6)

    def test_git_with_task_match(self):
        task_event = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_COMPLETED,
            task_id="guid-1",
        )
        self.scorer.record_signal(task_event)

        ev = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
            task_id="guid-1",
        )
        score = self.scorer.score(ev)
        # 0.6 + 0.2 task match = 0.8
        assert score == pytest.approx(0.8)

    # ── Filesystem ───────────────────────────────────────────

    def test_filesystem_base_is_0_4(self):
        ev = WorkflowEvent.make(
            source="filesystem", event_type=EventType.FS_BATCH_MODIFIED,
        )
        score = self.scorer.score(ev)
        assert score == pytest.approx(0.4)

    # ── score_and_record ─────────────────────────────────────

    def test_score_and_record_mutates_event(self):
        ev = WorkflowEvent.make(
            source="copilot", event_type=EventType.COPILOT_COMPLETION,
        )
        assert ev.confidence == 0.0
        result = self.scorer.score_and_record(ev)
        assert result is ev  # same object
        assert ev.confidence == pytest.approx(0.2)

    def test_score_and_record_records_signal(self):
        fs_ev = WorkflowEvent.make(
            source="filesystem", event_type=EventType.FS_BATCH_MODIFIED,
        )
        self.scorer.score_and_record(fs_ev)

        # Now a copilot event should get FS proximity bonus
        cop_ev = WorkflowEvent.make(
            source="copilot", event_type=EventType.COPILOT_COMPLETION,
        )
        self.scorer.score_and_record(cop_ev)
        assert cop_ev.confidence == pytest.approx(0.4)  # base + fs

    # ── Score capping ────────────────────────────────────────

    def test_confidence_never_exceeds_1(self):
        """Even with redundant signals, confidence is capped at 1.0."""
        for _ in range(10):
            e = WorkflowEvent.make(source="filesystem", event_type=EventType.FS_BATCH_MODIFIED)
            self.scorer.record_signal(e)
            e = WorkflowEvent.make(source="git", event_type=EventType.GIT_COMMIT)
            self.scorer.record_signal(e)
            e = WorkflowEvent.make(source="antigravity", event_type=EventType.AG_TASK_COMPLETED, task_id="t1")
            self.scorer.record_signal(e)

        ev = WorkflowEvent.make(
            source="copilot", event_type=EventType.COPILOT_COMPLETION,
            burst_id="b", task_id="t1",
        )
        score = self.scorer.score(ev)
        assert score <= 1.0
