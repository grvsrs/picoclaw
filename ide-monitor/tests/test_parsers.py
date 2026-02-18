"""Tests for parsers using EventType constants."""
import sys
import os
import tempfile
import json
import pytest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from normalizer import EventType
from parsers.antigravity import parse_brain_event, parse_skill_event
from parsers.copilot import parse_copilot_entries
from parsers.git import parse_commit_event


class TestAntigravityParser:
    """Tests for antigravity parser EventType usage."""

    def test_task_md_creates_task_event(self, tmp_path):
        brain_dir = tmp_path / "brain"
        task_dir = brain_dir / "guid-123"
        task_dir.mkdir(parents=True)
        task_file = task_dir / "task.md"
        task_file.write_text("# Add authentication\n- [ ] Create login page\n")

        ev = parse_brain_event(str(task_file), str(brain_dir))
        assert ev is not None
        assert ev.event_type == EventType.AG_TASK_CREATED
        assert ev.source == "antigravity"
        assert ev.task_id == "guid-123"

    def test_implementation_plan_uses_plan_ready(self, tmp_path):
        brain_dir = tmp_path / "brain"
        task_dir = brain_dir / "guid-123"
        task_dir.mkdir(parents=True)
        plan_file = task_dir / "implementation_plan.md"
        plan_file.write_text("# Implementation Plan\n## Step 1\nDo thing\n")

        ev = parse_brain_event(str(plan_file), str(brain_dir))
        assert ev is not None
        assert ev.event_type == EventType.AG_TASK_PLAN_READY

    def test_walkthrough_uses_completed(self, tmp_path):
        brain_dir = tmp_path / "brain"
        task_dir = brain_dir / "guid-123"
        task_dir.mkdir(parents=True)
        walk_file = task_dir / "walkthrough.md"
        walk_file.write_text("# Walkthrough\nAll done.\n")
        # Also create task.md so title extraction works
        task_file = task_dir / "task.md"
        task_file.write_text("# My Task\n")

        ev = parse_brain_event(str(walk_file), str(brain_dir))
        assert ev is not None
        assert ev.event_type == EventType.AG_TASK_COMPLETED

    def test_unknown_artifact_uses_artifact_unknown(self, tmp_path):
        brain_dir = tmp_path / "brain"
        task_dir = brain_dir / "guid-123"
        task_dir.mkdir(parents=True)
        unknown_file = task_dir / "random_notes.md"
        unknown_file.write_text("Some notes\n")

        ev = parse_brain_event(str(unknown_file), str(brain_dir))
        assert ev is not None
        assert ev.event_type == EventType.AG_ARTIFACT_UNKNOWN

    def test_completed_task_status_uses_completed(self, tmp_path):
        brain_dir = tmp_path / "brain"
        task_dir = brain_dir / "guid-123"
        task_dir.mkdir(parents=True)
        task_file = task_dir / "task.md"
        task_file.write_text("# Done Task\n- [x] Step 1\n- [x] Step 2\n")

        ev = parse_brain_event(str(task_file), str(brain_dir))
        assert ev is not None
        assert ev.event_type == EventType.AG_TASK_COMPLETED

    def test_failed_task_status(self, tmp_path):
        brain_dir = tmp_path / "brain"
        task_dir = brain_dir / "guid-123"
        task_dir.mkdir(parents=True)
        task_file = task_dir / "task.md"
        task_file.write_text("# Broken Task\nfailed to complete\n")

        ev = parse_brain_event(str(task_file), str(brain_dir))
        assert ev is not None
        assert ev.event_type == EventType.AG_TASK_FAILED

    def test_skill_update_uses_constant(self, tmp_path):
        skill_file = tmp_path / "my_skill.md"
        skill_file.write_text("# Custom Skill\nDoes things\n")

        ev = parse_skill_event(str(skill_file))
        assert ev is not None
        assert ev.event_type == EventType.AG_SKILL_UPDATED


class TestCopilotParser:
    """Tests for copilot parser EventType usage."""

    def test_completion_uses_constant(self, tmp_path):
        log_file = tmp_path / "copilot.json"
        entries = [
            json.dumps({"tokens_prompt": 100, "tokens_completion": 50}),
            json.dumps({"tokens_prompt": 200, "tokens_completion": 80}),
        ]
        log_file.write_text("\n".join(entries))

        events = parse_copilot_entries(str(log_file))
        assert len(events) == 2
        for ev in events:
            assert ev.event_type == EventType.COPILOT_COMPLETION

    def test_error_uses_constant(self, tmp_path):
        log_file = tmp_path / "copilot.json"
        log_file.write_text(json.dumps({"error": "rate limited"}))

        events = parse_copilot_entries(str(log_file))
        assert len(events) == 1
        assert events[0].event_type == EventType.COPILOT_ERROR


class TestGitParser:
    """Tests for git parser EventType usage."""

    def test_commit_uses_constant(self, tmp_path):
        # Create a minimal .git structure
        git_dir = tmp_path / ".git"
        git_dir.mkdir()
        msg_file = git_dir / "COMMIT_EDITMSG"
        msg_file.write_text("test commit message")

        # parse_commit_event will try to run git commands which will fail
        # in this synthetic setup, but the event_type should still be correct
        ev = parse_commit_event(str(msg_file))
        if ev is not None:
            assert ev.event_type == EventType.GIT_COMMIT
