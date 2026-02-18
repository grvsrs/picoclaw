"""Tests for burst detectors (FileBurstDetector + CopilotBurstDetector)."""
import sys
import os
import pytest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from normalizer import WorkflowEvent, EventType
from burst_detector import FileBurstDetector, CopilotBurstDetector, BurstDetector


class TestFileBurstDetector:
    """Tests for filesystem burst detection."""

    def test_alias_exists(self):
        assert BurstDetector is FileBurstDetector

    def test_no_burst_below_threshold(self):
        det = FileBurstDetector(window_secs=5, threshold=3)
        assert det.observe("/a.py", "/ws") is None
        assert det.observe("/b.py", "/ws") is None

    def test_burst_fires_at_threshold(self):
        det = FileBurstDetector(window_secs=5, threshold=3)
        det.observe("/a.py", "/ws")
        det.observe("/b.py", "/ws")
        result = det.observe("/c.py", "/ws")
        assert result is not None
        assert result.event_type == EventType.FS_BATCH_MODIFIED
        assert result.confidence == 0.4
        assert len(result.files_changed) == 3

    def test_burst_resets_after_firing(self):
        det = FileBurstDetector(window_secs=5, threshold=3)
        det.observe("/a.py", "/ws")
        det.observe("/b.py", "/ws")
        det.observe("/c.py", "/ws")  # fires
        # Should need 3 more
        assert det.observe("/d.py", "/ws") is None
        assert det.observe("/e.py", "/ws") is None


class TestCopilotBurstDetector:
    """Tests for Copilot token burst detection."""

    def _make_completion(self, tokens_prompt=100, tokens_completion=50):
        return WorkflowEvent.make(
            source="copilot",
            event_type=EventType.COPILOT_COMPLETION,
            tokens_prompt=tokens_prompt,
            tokens_completion=tokens_completion,
        )

    def test_no_burst_below_thresholds(self):
        det = CopilotBurstDetector(window_secs=120, token_threshold=500, count_threshold=5)
        ev = self._make_completion(50, 30)
        result = det.observe(ev)
        assert len(result) == 0
        assert not det.is_in_burst

    def test_burst_starts_at_count_threshold(self):
        det = CopilotBurstDetector(window_secs=120, token_threshold=99999, count_threshold=3)
        det.observe(self._make_completion(10, 10))
        det.observe(self._make_completion(10, 10))
        result = det.observe(self._make_completion(10, 10))

        assert len(result) == 1
        start_ev = result[0]
        assert start_ev.event_type == EventType.COPILOT_BURST_START
        assert start_ev.burst_id is not None
        assert det.is_in_burst

    def test_burst_starts_at_token_threshold(self):
        det = CopilotBurstDetector(window_secs=120, token_threshold=500, count_threshold=99999)
        det.observe(self._make_completion(200, 100))  # 300 total
        result = det.observe(self._make_completion(200, 100))  # 600 total

        assert len(result) == 1
        assert result[0].event_type == EventType.COPILOT_BURST_START
        assert det.is_in_burst

    def test_events_annotated_with_burst_id_during_burst(self):
        det = CopilotBurstDetector(window_secs=120, token_threshold=99999, count_threshold=3)
        ev1 = self._make_completion()
        ev2 = self._make_completion()
        ev3 = self._make_completion()
        det.observe(ev1)
        det.observe(ev2)
        det.observe(ev3)

        # After burst starts, ev3 should be annotated
        assert ev3.burst_id is not None
        assert ev3.burst_id == det.active_burst_id

    def test_flush_closes_active_burst(self):
        det = CopilotBurstDetector(window_secs=120, token_threshold=99999, count_threshold=2)
        det.observe(self._make_completion())
        det.observe(self._make_completion())
        assert det.is_in_burst

        end_ev = det.flush()
        assert end_ev is not None
        assert end_ev.event_type == EventType.COPILOT_BURST_END
        assert not det.is_in_burst

    def test_flush_returns_none_when_no_burst(self):
        det = CopilotBurstDetector()
        assert det.flush() is None
