"""Tests for WorkflowEvent v1 schema (normalizer.py)"""
import sys
import os
import json
import pytest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from normalizer import WorkflowEvent, EventType, FileChange, SPEC_VERSION


class TestEventType:
    """Tests for the closed event taxonomy."""

    def test_all_types_is_set(self):
        all_types = EventType.all_types()
        assert isinstance(all_types, set)
        assert len(all_types) > 15  # We have ~20 types

    def test_copilot_types_present(self):
        assert EventType.COPILOT_COMPLETION in EventType.all_types()
        assert EventType.COPILOT_BURST_START in EventType.all_types()
        assert EventType.COPILOT_BURST_END in EventType.all_types()
        assert EventType.COPILOT_ERROR in EventType.all_types()

    def test_antigravity_types_present(self):
        assert EventType.AG_TASK_CREATED in EventType.all_types()
        assert EventType.AG_TASK_COMPLETED in EventType.all_types()
        assert EventType.AG_TASK_FAILED in EventType.all_types()
        assert EventType.AG_TASK_PLAN_READY in EventType.all_types()

    def test_git_types_present(self):
        assert EventType.GIT_COMMIT in EventType.all_types()
        assert EventType.GIT_COMMIT_LINKED in EventType.all_types()

    def test_derived_types_present(self):
        assert EventType.WORKFLOW_ACTIVITY_CLUSTERED in EventType.all_types()
        assert EventType.WORKFLOW_TASK_INFERRED in EventType.all_types()

    def test_is_valid_accepts_known_types(self):
        assert EventType.is_valid("copilot.completion")
        assert EventType.is_valid("antigravity.task.created")
        assert EventType.is_valid("git.commit")

    def test_is_valid_rejects_unknown_types(self):
        assert not EventType.is_valid("made.up.event")
        assert not EventType.is_valid("")
        assert not EventType.is_valid("copilot.random")


class TestWorkflowEvent:
    """Tests for the WorkflowEvent dataclass and make() factory."""

    def test_make_sets_required_fields(self):
        ev = WorkflowEvent.make(source="copilot", event_type=EventType.COPILOT_COMPLETION)
        assert ev.id  # uuid
        assert ev.spec_version == SPEC_VERSION
        assert ev.source == "copilot"
        assert ev.event_type == "copilot.completion"
        assert ev.timestamp  # ISO8601
        assert ev.hostname
        assert ev.workspace_id  # computed from cwd

    def test_make_computes_workspace_id(self):
        ev = WorkflowEvent.make(
            source="git", event_type=EventType.GIT_COMMIT,
            workspace_root="/home/user/myproject",
        )
        assert ev.workspace_id is not None
        assert len(ev.workspace_id) == 12

    def test_make_computes_external_ref(self):
        ev = WorkflowEvent.make(
            source="antigravity", event_type=EventType.AG_TASK_CREATED,
            task_id="abc-123",
            workspace_root="/home/user/myproject",
        )
        assert ev.external_ref is not None
        assert ":" in ev.external_ref
        assert ev.external_ref.endswith("abc-123")

    def test_make_without_task_id_has_no_external_ref(self):
        ev = WorkflowEvent.make(source="copilot", event_type=EventType.COPILOT_COMPLETION)
        assert ev.external_ref is None

    def test_confidence_defaults_to_zero(self):
        ev = WorkflowEvent.make(source="copilot", event_type=EventType.COPILOT_COMPLETION)
        assert ev.confidence == 0.0

    def test_confidence_can_be_set(self):
        ev = WorkflowEvent.make(
            source="copilot", event_type=EventType.COPILOT_COMPLETION,
            confidence=0.8,
        )
        assert ev.confidence == 0.8

    def test_to_json_omits_none_fields(self):
        ev = WorkflowEvent.make(source="git", event_type=EventType.GIT_COMMIT)
        data = json.loads(ev.to_json())
        assert "task_id" not in data
        assert "burst_id" not in data
        assert "tokens_prompt" not in data

    def test_to_json_keeps_confidence_even_if_zero(self):
        ev = WorkflowEvent.make(source="copilot", event_type=EventType.COPILOT_COMPLETION)
        data = json.loads(ev.to_json())
        assert "confidence" in data
        assert data["confidence"] == 0.0

    def test_to_json_includes_burst_fields_when_set(self):
        ev = WorkflowEvent.make(
            source="copilot", event_type=EventType.COPILOT_BURST_START,
            burst_id="burst-001",
            burst_token_total=500,
            burst_entry_count=5,
        )
        data = json.loads(ev.to_json())
        assert data["burst_id"] == "burst-001"
        assert data["burst_token_total"] == 500

    def test_files_changed_serialization(self):
        ev = WorkflowEvent.make(
            source="filesystem", event_type=EventType.FS_BATCH_MODIFIED,
            files_changed=[
                FileChange(path="src/main.py", change_type="modified"),
                FileChange(path="src/test.py", change_type="created"),
            ],
        )
        data = json.loads(ev.to_json())
        assert len(data["files_changed"]) == 2
        assert data["files_changed"][0]["path"] == "src/main.py"

    def test_unique_ids(self):
        ev1 = WorkflowEvent.make(source="copilot", event_type=EventType.COPILOT_COMPLETION)
        ev2 = WorkflowEvent.make(source="copilot", event_type=EventType.COPILOT_COMPLETION)
        assert ev1.id != ev2.id
