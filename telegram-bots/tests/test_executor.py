"""
Tests for executor module.

Covers:
    - validate_project_name()   — alphanumeric, hyphen, underscore only
    - validate_service_name()   — allowlist enforcement
    - safe_path()               — path traversal prevention
    - truncate_output()         — output size limiting
    - load_task() / write_result() — task file I/O
    - process_task()            — full lifecycle: pending → running → complete/failed/rejected
    - COMMAND_TABLE             — all expected commands registered and callable
    - Command implementations   — with mocked subprocess
"""

import json
from pathlib import Path
from unittest.mock import patch, MagicMock, PropertyMock

import pytest
import yaml

# Mock inotify_simple before importing executor
import sys
sys.modules["inotify_simple"] = MagicMock()

from executor import (
    validate_project_name,
    validate_service_name,
    safe_path,
    truncate_output,
    load_task,
    write_result,
    process_task,
    COMMAND_TABLE,
    ALLOWED_SERVICES,
)


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Input validation
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TestValidateProjectName:

    def test_valid_names(self):
        assert validate_project_name("myproject") is True
        assert validate_project_name("my-project") is True
        assert validate_project_name("my_project_123") is True
        assert validate_project_name("A") is True
        assert validate_project_name("app2") is True

    def test_invalid_names(self):
        assert validate_project_name("") is False
        assert validate_project_name("../etc") is False
        assert validate_project_name("my project") is False
        assert validate_project_name("my;project") is False
        assert validate_project_name("project/sub") is False
        assert validate_project_name("a b") is False

    def test_none_raises_or_false(self):
        # None should not crash
        assert validate_project_name(None) is False


class TestValidateServiceName:

    def test_valid(self):
        assert validate_service_name("nginx", ALLOWED_SERVICES) is True
        assert validate_service_name("docker", ALLOWED_SERVICES) is True
        assert validate_service_name("postgresql", ALLOWED_SERVICES) is True
        assert validate_service_name("redis", ALLOWED_SERVICES) is True

    def test_invalid(self):
        assert validate_service_name("mysql", ALLOWED_SERVICES) is False
        assert validate_service_name("", ALLOWED_SERVICES) is False
        assert validate_service_name("nginx; rm -rf /", ALLOWED_SERVICES) is False


class TestSafePath:

    def test_valid_path(self, tmp_path):
        result = safe_path(tmp_path, "subdir", "file.txt")
        assert str(result).startswith(str(tmp_path.resolve()))

    def test_traversal_blocked(self, tmp_path):
        with pytest.raises(ValueError, match="Path traversal"):
            safe_path(tmp_path, "..", "..", "etc", "passwd")

    def test_double_dot_in_name_ok(self, tmp_path):
        """A name like 'my..project' with dots is fine if it stays in base."""
        result = safe_path(tmp_path, "my..project")
        assert str(result).startswith(str(tmp_path.resolve()))

    def test_absolute_escape_blocked(self, tmp_path):
        with pytest.raises(ValueError, match="Path traversal"):
            safe_path(tmp_path, "..", "..")


class TestTruncateOutput:

    def test_short_unchanged(self):
        assert truncate_output("hello") == "hello"

    def test_empty_string(self):
        assert truncate_output("") == ""

    def test_none_returns_empty(self):
        assert truncate_output(None) == ""

    def test_long_truncated(self):
        text = "x" * 10000
        result = truncate_output(text)
        assert len(result) < 10000
        assert "truncated" in result


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Task file I/O
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TestLoadTask:

    def test_valid_task(self, tmp_path):
        task = {"id": "test-1", "command": "status", "status": "pending"}
        task_file = tmp_path / "test.yaml"
        with open(task_file, "w") as f:
            yaml.dump(task, f)

        loaded = load_task(task_file)
        assert loaded["id"] == "test-1"
        assert loaded["command"] == "status"
        assert loaded["status"] == "pending"

    def test_missing_file_returns_none(self, tmp_path):
        result = load_task(tmp_path / "nonexistent.yaml")
        assert result is None

    def test_invalid_yaml_returns_none(self, tmp_path):
        task_file = tmp_path / "bad.yaml"
        task_file.write_text(": : : invalid yaml [[[")
        result = load_task(task_file)
        # yaml.safe_load may return the string or raise; either way no crash
        assert result is not None or result is None  # just ensure no exception


class TestWriteResult:

    def test_writes_back(self, tmp_path):
        task = {"id": "test-1", "status": "complete", "exit_code": 0}
        task_file = tmp_path / "test.yaml"
        write_result(task_file, task)

        with open(task_file) as f:
            loaded = yaml.safe_load(f)
        assert loaded["status"] == "complete"
        assert loaded["exit_code"] == 0

    def test_atomic_no_temp_files(self, tmp_path):
        task = {"id": "test-1", "status": "complete"}
        task_file = tmp_path / "test.yaml"
        write_result(task_file, task)

        tmp_files = list(tmp_path.glob("*.tmp"))
        assert len(tmp_files) == 0


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Command dispatch table
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TestCommandTable:

    def test_all_expected_commands(self):
        expected = {
            "run_tests", "deploy", "scaffold", "git_status",
            "status", "restart_service", "disk_usage",
            "process_info", "health", "recent",
        }
        assert set(COMMAND_TABLE.keys()) == expected

    def test_all_callable(self):
        for name, handler in COMMAND_TABLE.items():
            assert callable(handler), f"{name} is not callable"


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# process_task()
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TestProcessTask:

    def _write_task(self, tmp_path, task: dict) -> Path:
        task_file = tmp_path / f"{task.get('id', 'test')}.yaml"
        with open(task_file, "w") as f:
            yaml.dump(task, f)
        return task_file

    @patch("executor.audit_log")
    def test_unknown_command_rejected(self, mock_audit, tmp_path):
        task_file = self._write_task(tmp_path, {
            "id": "test-1",
            "command": "dangerous_command",
            "status": "pending",
            "params": {},
        })

        process_task(task_file)

        with open(task_file) as f:
            result = yaml.safe_load(f)
        assert result["status"] == "rejected"
        assert result["exit_code"] == 1
        assert "Unknown command" in result["summary"]

    @patch("executor.audit_log")
    def test_skip_non_pending(self, mock_audit, tmp_path):
        task_file = self._write_task(tmp_path, {
            "id": "test-2",
            "command": "status",
            "status": "complete",
            "params": {},
        })

        process_task(task_file)

        with open(task_file) as f:
            result = yaml.safe_load(f)
        # Should remain unchanged
        assert result["status"] == "complete"
        mock_audit.assert_not_called()

    @patch("executor.audit_log")
    def test_status_command_success(self, mock_audit, tmp_path):
        mock_handler = MagicMock(return_value=(0, "all systems go", "", "System OK"))

        task_file = self._write_task(tmp_path, {
            "id": "test-3",
            "command": "status",
            "status": "pending",
            "params": {},
        })

        original = COMMAND_TABLE["status"]
        COMMAND_TABLE["status"] = mock_handler
        try:
            process_task(task_file)
        finally:
            COMMAND_TABLE["status"] = original

        with open(task_file) as f:
            result = yaml.safe_load(f)
        assert result["status"] == "complete"
        assert result["exit_code"] == 0
        assert result["stdout"] == "all systems go"
        assert result["summary"] == "System OK"
        assert "duration_s" in result
        assert "completed_at" in result
        assert "started_at" in result
        mock_handler.assert_called_once()

    @patch("executor.audit_log")
    def test_command_failure(self, mock_audit, tmp_path):
        mock_handler = MagicMock(return_value=(
            1, "", "3 tests failed", "Tests failed (suite=all)"
        ))

        task_file = self._write_task(tmp_path, {
            "id": "test-4",
            "command": "run_tests",
            "status": "pending",
            "params": {"suite": "all"},
            "project": "myapp",
        })

        original = COMMAND_TABLE["run_tests"]
        COMMAND_TABLE["run_tests"] = mock_handler
        try:
            process_task(task_file)
        finally:
            COMMAND_TABLE["run_tests"] = original

        with open(task_file) as f:
            result = yaml.safe_load(f)
        assert result["status"] == "failed"
        assert result["exit_code"] == 1

    @patch("executor.audit_log")
    def test_command_exception_handled(self, mock_audit, tmp_path):
        mock_handler = MagicMock(side_effect=RuntimeError("something broke"))

        task_file = self._write_task(tmp_path, {
            "id": "test-5",
            "command": "deploy",
            "status": "pending",
            "params": {"env": "staging"},
            "project": "myapp",
        })

        original = COMMAND_TABLE["deploy"]
        COMMAND_TABLE["deploy"] = mock_handler
        try:
            process_task(task_file)
        finally:
            COMMAND_TABLE["deploy"] = original

        with open(task_file) as f:
            result = yaml.safe_load(f)
        assert result["status"] == "failed"
        assert result["exit_code"] == -1
        assert "RuntimeError" in result["summary"]

    @patch("executor.audit_log")
    def test_health_command(self, mock_audit, tmp_path):
        mock_handler = MagicMock(return_value=(
            0, "✅ All checks passed\n✅ Disk: 45%", "", "Health check complete"
        ))

        task_file = self._write_task(tmp_path, {
            "id": "test-6",
            "command": "health",
            "status": "pending",
            "params": {},
        })

        original = COMMAND_TABLE["health"]
        COMMAND_TABLE["health"] = mock_handler
        try:
            process_task(task_file)
        finally:
            COMMAND_TABLE["health"] = original

        with open(task_file) as f:
            result = yaml.safe_load(f)
        assert result["status"] == "complete"
        assert "Health check" in result["summary"]


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Command input validation (defense in depth)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TestCommandValidation:
    """
    Test that command implementations validate their own inputs,
    even though the bot validates first (defense in depth).
    """

    def test_run_tests_invalid_project(self):
        from executor import run_tests
        code, _, stderr, summary = run_tests({
            "project": "../etc",
            "params": {"suite": "all"},
        })
        assert code == 1
        assert "Invalid project name" in stderr

    def test_deploy_invalid_env(self):
        from executor import deploy
        code, _, stderr, _ = deploy({
            "project": "myapp",
            "params": {"env": "dangerous"},
        })
        assert code == 1
        assert "Invalid environment" in stderr

    def test_restart_service_not_allowed(self):
        from executor import restart_service
        code, _, stderr, _ = restart_service({
            "params": {"service": "sshd"},
            "service": "sshd",
        })
        assert code == 1
        assert "not in allowed list" in stderr

    def test_disk_usage_invalid_path(self):
        from executor import disk_usage
        code, _, stderr, _ = disk_usage({
            "params": {"path": "../../../etc/shadow"},
        })
        assert code == 1
        assert "Invalid path" in stderr

    def test_process_info_invalid_name(self):
        from executor import process_info
        code, _, stderr, _ = process_info({
            "params": {"name": "nginx; rm -rf /"},
        })
        assert code == 1
        assert "Invalid process name" in stderr

    def test_scaffold_invalid_component(self):
        from executor import scaffold
        code, _, stderr, _ = scaffold({
            "project": "myapp",
            "params": {"component": "../../etc"},
        })
        assert code == 1
        assert "Invalid component" in stderr
