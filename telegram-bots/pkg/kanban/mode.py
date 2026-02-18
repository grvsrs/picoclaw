"""
Dual mode enforcement: Personal vs Remote.

- Personal mode: full shell, loose constraints, Telegram is read-only status
- Remote mode: sandboxed, whitelist-based, mandatory Kanban card, rate-limited
"""
from enum import Enum
from typing import Optional
from .schema import TaskMode, KanbanCard, TaskState


class ModeViolation(Exception):
    """Raised when an operation violates the current mode."""
    pass


class LinuxModeManager:
    """Enforces personal vs remote mode policies."""
    
    def __init__(self, mode: TaskMode = TaskMode.PERSONAL):
        """Initialize with a mode."""
        self.current_mode = mode
    
    def switch_mode(self, new_mode: TaskMode, reason: str = "") -> None:
        """Switch to a different mode."""
        if self.current_mode != new_mode:
            print(f"[MODE] Switching from {self.current_mode.value} to {new_mode.value}: {reason}")
            self.current_mode = new_mode
    
    def enforce_task_requirement(self, card_id: Optional[str]) -> None:
        """
        In REMOTE mode, every execution must have a valid Kanban card.
        
        Raises ModeViolation if the requirement is not met.
        """
        if self.current_mode == TaskMode.REMOTE and not card_id:
            raise ModeViolation(
                "Remote mode requires a valid Kanban card (card_id). "
                "Create a card first and include its ID in the task."
            )
    
    def enforce_command_whitelist(self, command: str, whitelist: Optional[list] = None) -> bool:
        """
        In REMOTE mode, only whitelisted commands are allowed.
        
        Returns True if allowed, raises ModeViolation if forbidden.
        """
        if self.current_mode == TaskMode.PERSONAL:
            return True  # Full shell access
        
        if whitelist is None:
            whitelist = self._get_default_remote_whitelist()
        
        # Check if command prefix matches any whitelisted command
        for allowed_cmd in whitelist:
            if command.strip().startswith(allowed_cmd):
                return True
        
        raise ModeViolation(
            f"Command '{command}' not allowed in remote mode. "
            f"Whitelist: {whitelist}"
        )
    
    def _get_default_remote_whitelist(self) -> list:
        """Default whitelist for remote mode."""
        return [
            "python",
            "pytest",
            "git",
            "ls",
            "cat",
            "echo",
            "mkdir",
            "rm",
            "touch",
            "cp",
            "mv",
            "grep",
            "awk",
            "sed",
            "head",
            "tail",
            "wc",
            "find",
            "chmod",
            "chown",
            "true",
            "false",
        ]
    
    def rate_limit_check(self) -> bool:
        """
        In REMOTE mode, enforce rate limiting (stub for now).
        
        In a real system, you'd track execution counts per user per time window.
        """
        if self.current_mode == TaskMode.REMOTE:
            # TODO: implement actual rate limiting
            # For now, always allow
            pass
        return True
    
    def log_execution(self, card: KanbanCard, command: str, result: str) -> None:
        """
        Log execution details for audit.
        
        In REMOTE mode, log everything. In PERSONAL mode, log failures only.
        """
        if self.current_mode == TaskMode.REMOTE:
            print(f"[AUDIT] REMOTE {card.card_id}: {command} -> {result[:100]}")
        elif result and "error" in result.lower():
            print(f"[AUDIT] PERSONAL {card.card_id}: {command} failed -> {result[:100]}")
