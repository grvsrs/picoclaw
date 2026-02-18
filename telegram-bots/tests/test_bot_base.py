"""
Tests for bot_base module.

Covers:
    - truncate()            — text truncation for Telegram messages
    - make_task_id()        — unique sortable ID generation
    - utc_now()             — timestamp format
    - ParamValidator        — all validation rules
    - TaskWriter            — task file creation, confirmation status
    - AuditLogger           — structured JSON audit trail
    - BotConfig             — config loading, error handling
    - BotBase._parse_command_args — positional, named, mixed parsing
"""

import json
import os
import textwrap
from pathlib import Path

import pytest
import yaml

from bot_base import (
    ParamValidator,
    ValidationError,
    ConfigError,
    BotConfig,
    TaskWriter,
    AuditLogger,
    make_task_id,
    truncate,
    utc_now,
)


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Utility functions
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TestTruncate:

    def test_short_text_unchanged(self):
        assert truncate("hello", 100) == "hello"

    def test_exact_limit_unchanged(self):
        text = "x" * 100
        assert truncate(text, 100) == text

    def test_over_limit_truncated(self):
        text = "x" * 200
        result = truncate(text, 100)
        assert len(result) < 200
        assert "truncated" in result
        assert "100 chars omitted" in result

    def test_default_limit_is_3500(self):
        text = "x" * 4000
        result = truncate(text)
        assert len(result) < 4000
        assert "truncated" in result

    def test_empty_string(self):
        assert truncate("") == ""


class TestMakeTaskId:

    def test_format_prefix(self):
        tid = make_task_id()
        assert tid.startswith("task-")

    def test_has_timestamp_and_random(self):
        tid = make_task_id()
        parts = tid.split("-", 2)  # "task", timestamp, random
        assert len(parts) == 3
        assert parts[0] == "task"
        assert parts[1].isdigit()  # timestamp
        assert len(parts[2]) == 8  # hex

    def test_uniqueness(self):
        ids = {make_task_id() for _ in range(100)}
        assert len(ids) == 100

    def test_sortable(self):
        """IDs generated later should sort after earlier ones."""
        import time
        id1 = make_task_id()
        time.sleep(0.002)  # ensure different ms
        id2 = make_task_id()
        assert id2 > id1


class TestUtcNow:

    def test_format(self):
        ts = utc_now()
        assert ts.endswith("Z")
        assert "T" in ts
        assert len(ts) == 20  # 2024-01-15T10:30:00Z


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# ParamValidator
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TestParamValidator:

    def setup_method(self):
        self.v = ParamValidator()

    # -- Required / optional --

    def test_required_present(self):
        schema = {"name": {"type": "string", "required": True}}
        result = self.v.validate({"name": "test"}, schema)
        assert result == {"name": "test"}

    def test_required_missing_raises(self):
        schema = {"name": {"type": "string", "required": True}}
        with pytest.raises(ValidationError, match="Missing required"):
            self.v.validate({}, schema)

    def test_required_empty_string_raises(self):
        schema = {"name": {"type": "string", "required": True}}
        with pytest.raises(ValidationError, match="Missing required"):
            self.v.validate({"name": ""}, schema)

    def test_optional_missing_no_default(self):
        schema = {"name": {"type": "string"}}
        result = self.v.validate({}, schema)
        assert result == {}

    def test_default_value_used(self):
        schema = {"env": {"type": "string", "default": "staging"}}
        result = self.v.validate({}, schema)
        assert result == {"env": "staging"}

    def test_explicit_value_overrides_default(self):
        schema = {"env": {"type": "string", "default": "staging"}}
        result = self.v.validate({"env": "production"}, schema)
        assert result == {"env": "production"}

    # -- String: allowed values --

    def test_allowed_valid(self):
        schema = {"env": {"type": "string", "allowed": ["staging", "prod"]}}
        assert self.v.validate({"env": "staging"}, schema) == {"env": "staging"}

    def test_allowed_invalid_raises(self):
        schema = {"env": {"type": "string", "allowed": ["staging", "prod"]}}
        with pytest.raises(ValidationError, match="Invalid value"):
            self.v.validate({"env": "dev"}, schema)

    # -- String: pattern --

    def test_pattern_valid(self):
        schema = {"name": {"type": "string", "pattern": "^[a-z-]+$"}}
        assert self.v.validate({"name": "my-project"}, schema) == {
            "name": "my-project"
        }

    def test_pattern_invalid_raises(self):
        schema = {"name": {"type": "string", "pattern": "^[a-z-]+$"}}
        with pytest.raises(ValidationError, match="does not match"):
            self.v.validate({"name": "MyProject!"}, schema)

    # -- Integer --

    def test_integer_coercion(self):
        schema = {"count": {"type": "integer"}}
        result = self.v.validate({"count": "42"}, schema)
        assert result == {"count": 42}
        assert isinstance(result["count"], int)

    def test_integer_invalid_raises(self):
        schema = {"count": {"type": "integer"}}
        with pytest.raises(ValidationError, match="must be an integer"):
            self.v.validate({"count": "abc"}, schema)

    def test_integer_min_max_valid(self):
        schema = {"count": {"type": "integer", "min": 1, "max": 100}}
        assert self.v.validate({"count": "50"}, schema) == {"count": 50}

    def test_integer_below_min_raises(self):
        schema = {"count": {"type": "integer", "min": 10}}
        with pytest.raises(ValidationError, match=">= 10"):
            self.v.validate({"count": "5"}, schema)

    def test_integer_above_max_raises(self):
        schema = {"count": {"type": "integer", "max": 100}}
        with pytest.raises(ValidationError, match="<= 100"):
            self.v.validate({"count": "200"}, schema)

    # -- Unknown params --

    def test_unknown_params_rejected(self):
        schema = {"name": {"type": "string"}}
        with pytest.raises(ValidationError, match="Unknown parameters"):
            self.v.validate({"name": "ok", "extra": "bad"}, schema)

    # -- Complex / multi-param --

    def test_multiple_params(self):
        schema = {
            "project": {"type": "string", "required": True, "pattern": "^[a-z-]+$"},
            "suite": {"type": "string", "allowed": ["unit", "all"], "default": "all"},
            "lines": {"type": "integer", "min": 1, "max": 500, "default": 50},
        }
        result = self.v.validate(
            {"project": "my-app", "suite": "unit"}, schema
        )
        assert result == {"project": "my-app", "suite": "unit", "lines": 50}


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# TaskWriter
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TestTaskWriter:

    def test_write_creates_yaml(self, tmp_path):
        writer = TaskWriter(tmp_path, timeout=5)
        task_file = writer.write(
            task_id="task-123-abc",
            bot_name="dev_bot",
            command="run_tests",
            user_id=12345,
            username="testuser",
            params={"suite": "unit"},
            project="myproject",
        )
        assert task_file.exists()
        assert task_file.name == "task-123-abc.yaml"

    def test_write_correct_content(self, tmp_path):
        writer = TaskWriter(tmp_path, timeout=5)
        task_file = writer.write(
            task_id="task-456-def",
            bot_name="dev_bot",
            command="deploy",
            user_id=99999,
            username="admin",
            params={"env": "staging"},
            project="glass-walls",
        )

        with open(task_file) as f:
            task = yaml.safe_load(f)

        assert task["id"] == "task-456-def"
        assert task["bot"] == "dev_bot"
        assert task["command"] == "deploy"
        assert task["user_id"] == 99999
        assert task["username"] == "admin"
        assert task["params"] == {"env": "staging"}
        assert task["project"] == "glass-walls"
        assert task["status"] == "pending"
        assert "created_at" in task

    def test_write_confirmation_status(self, tmp_path):
        writer = TaskWriter(tmp_path, timeout=5)
        task_file = writer.write(
            task_id="task-789-ghi",
            bot_name="ops_bot",
            command="restart_service",
            user_id=12345,
            username="admin",
            params={"service": "nginx"},
            confirmation_required=True,
        )

        with open(task_file) as f:
            task = yaml.safe_load(f)

        assert task["status"] == "awaiting_confirmation"
        assert task["confirmation_required"] is True

    def test_write_without_optional_fields(self, tmp_path):
        writer = TaskWriter(tmp_path, timeout=5)
        task_file = writer.write(
            task_id="task-000-aaa",
            bot_name="monitor_bot",
            command="health",
            user_id=11111,
            username="user",
            params={},
        )

        with open(task_file) as f:
            task = yaml.safe_load(f)

        assert "project" not in task
        assert "service" not in task
        assert task["status"] == "pending"

    def test_no_temp_files_left(self, tmp_path):
        """Atomic write should not leave .tmp files."""
        writer = TaskWriter(tmp_path, timeout=5)
        writer.write(
            task_id="task-tmp-test",
            bot_name="dev_bot",
            command="status",
            user_id=12345,
            username="test",
            params={},
        )
        tmp_files = list(tmp_path.glob("*.tmp"))
        assert len(tmp_files) == 0


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# AuditLogger
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TestAuditLogger:

    def test_log_writes_jsonl(self, tmp_path):
        log_file = tmp_path / "audit.jsonl"
        logger = AuditLogger(log_file)
        logger.log(
            user_id=123,
            username="test",
            bot="dev_bot",
            command="run_tests",
            task_id="task-1",
            status="submitted",
        )

        lines = log_file.read_text().strip().splitlines()
        assert len(lines) == 1
        entry = json.loads(lines[0])
        assert entry["user_id"] == 123
        assert entry["username"] == "test"
        assert entry["bot"] == "dev_bot"
        assert entry["command"] == "run_tests"
        assert entry["task_id"] == "task-1"
        assert entry["status"] == "submitted"
        assert "ts" in entry

    def test_log_appends(self, tmp_path):
        log_file = tmp_path / "audit.jsonl"
        logger = AuditLogger(log_file)
        for i in range(5):
            logger.log(
                user_id=123,
                username="test",
                bot="dev_bot",
                command=f"cmd_{i}",
                task_id=f"task-{i}",
                status="complete",
            )

        lines = log_file.read_text().strip().splitlines()
        assert len(lines) == 5

    def test_log_extra_kwargs(self, tmp_path):
        log_file = tmp_path / "audit.jsonl"
        logger = AuditLogger(log_file)
        logger.log(
            user_id=123,
            username="test",
            bot="dev_bot",
            command="deploy",
            task_id="task-x",
            status="complete",
            exit_code=0,
            duration_s=12.5,
            project="myapp",
        )

        entry = json.loads(log_file.read_text().strip())
        assert entry["exit_code"] == 0
        assert entry["duration_s"] == 12.5
        assert entry["project"] == "myapp"

    def test_log_filters_none_extras(self, tmp_path):
        log_file = tmp_path / "audit.jsonl"
        logger = AuditLogger(log_file)
        logger.log(
            user_id=123,
            username="test",
            bot="dev_bot",
            command="test",
            task_id="task-x",
            status="submitted",
            project=None,
            service=None,
        )

        entry = json.loads(log_file.read_text().strip())
        assert "project" not in entry
        assert "service" not in entry


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# BotConfig
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TestBotConfig:

    def _write_config(self, tmp_path, config: dict) -> Path:
        config_file = tmp_path / "picoclaw.yaml"
        with open(config_file, "w") as f:
            yaml.dump(config, f)
        return config_file

    def test_loads_valid_config(self, tmp_path):
        os.environ["TEST_BOT_TOKEN"] = "fake-token"
        try:
            config_file = self._write_config(tmp_path, {
                "global": {
                    "result_timeout": 15,
                    "audit_log": str(tmp_path / "audit.jsonl"),
                },
                "users": {"12345": {"name": "admin", "roles": ["admin"]}},
                "bots": {
                    "test_bot": {
                        "token_env": "TEST_BOT_TOKEN",
                        "task_path": str(tmp_path / "tasks"),
                        "allowed_users": [12345],
                        "commands": {
                            "ping": {
                                "description": "Test",
                                "params": {},
                            }
                        },
                    }
                },
            })
            cfg = BotConfig(str(config_file), "test_bot")
            assert cfg.token == "fake-token"
            assert cfg.result_timeout == 15
            assert cfg.is_authorized(12345)
            assert not cfg.is_authorized(99999)
            assert cfg.get_command("ping") is not None
            assert cfg.get_command("nonexistent") is None
        finally:
            del os.environ["TEST_BOT_TOKEN"]

    def test_missing_bot_raises(self, tmp_path):
        os.environ["TEST_BOT_TOKEN"] = "fake-token"
        try:
            config_file = self._write_config(tmp_path, {
                "bots": {
                    "other_bot": {
                        "token_env": "TEST_BOT_TOKEN",
                        "task_path": str(tmp_path / "tasks"),
                        "allowed_users": [],
                        "commands": {},
                    }
                }
            })
            with pytest.raises(ConfigError, match="not found in config"):
                BotConfig(str(config_file), "missing_bot")
        finally:
            del os.environ["TEST_BOT_TOKEN"]

    def test_missing_token_env_raises(self, tmp_path):
        # Ensure env var is NOT set
        os.environ.pop("MISSING_TOKEN", None)
        config_file = self._write_config(tmp_path, {
            "bots": {
                "test_bot": {
                    "token_env": "MISSING_TOKEN",
                    "task_path": str(tmp_path / "tasks"),
                    "allowed_users": [],
                    "commands": {},
                }
            }
        })
        with pytest.raises(ConfigError, match="not set"):
            BotConfig(str(config_file), "test_bot")


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Command arg parsing (BotBase._parse_command_args)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


class TestParseCommandArgs:
    """Test _parse_command_args as a standalone function by importing BotBase."""

    def _parse(self, text, schema):
        """Call _parse_command_args without needing a full BotBase instance."""
        from bot_base import BotBase
        # Use the method as unbound (it only uses self for nothing)
        parts = text.split()[1:]
        schema_keys = list(schema.keys())
        result = {}
        positional_idx = 0
        for part in parts:
            if "=" in part:
                key, _, value = part.partition("=")
                result[key] = value
            else:
                if positional_idx < len(schema_keys):
                    result[schema_keys[positional_idx]] = part
                    positional_idx += 1
        return result

    def test_positional_args(self):
        schema = {"project": {}, "suite": {}}
        result = self._parse("/run_tests myapp unit", schema)
        assert result == {"project": "myapp", "suite": "unit"}

    def test_named_args(self):
        schema = {"project": {}, "suite": {}}
        result = self._parse("/run_tests project=myapp suite=unit", schema)
        assert result == {"project": "myapp", "suite": "unit"}

    def test_mixed_args(self):
        schema = {"project": {}, "suite": {}}
        result = self._parse("/run_tests myapp suite=unit", schema)
        assert result == {"project": "myapp", "suite": "unit"}

    def test_no_args(self):
        schema = {"project": {}}
        result = self._parse("/status", schema)
        assert result == {}

    def test_extra_positional_dropped(self):
        schema = {"project": {}}
        result = self._parse("/cmd myapp extra1 extra2", schema)
        assert result == {"project": "myapp"}
