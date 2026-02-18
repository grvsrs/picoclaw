# IDE Monitor — Git commit parser
#
# Triggered when .git/COMMIT_EDITMSG is written (a commit just happened).
# Extracts commit metadata and correlates with recent Antigravity tasks.

import subprocess
from pathlib import Path
from typing import Optional

from normalizer import WorkflowEvent, FileChange, EventType


def _run(cmd: list, cwd: str) -> Optional[str]:
    """Run a git command safely. Returns stdout or None on failure."""
    try:
        result = subprocess.run(
            cmd, cwd=cwd, capture_output=True, text=True, timeout=5
        )
        return result.stdout.strip() if result.returncode == 0 else None
    except Exception:
        return None


def _safe_read(path: Path) -> Optional[str]:
    try:
        return path.read_text(encoding="utf-8", errors="replace")
    except Exception:
        return None


def parse_commit_event(commit_msg_path: str) -> Optional[WorkflowEvent]:
    """
    Parse a git commit event from COMMIT_EDITMSG.
    Extracts SHA, branch, commit message, and changed files.
    """
    path = Path(commit_msg_path)
    repo_root = str(path.parent.parent)  # .git/../

    msg = _safe_read(path)
    sha = _run(["git", "rev-parse", "HEAD"], cwd=repo_root)
    branch = _run(["git", "rev-parse", "--abbrev-ref", "HEAD"], cwd=repo_root)

    # Files changed in this commit
    diff_output = _run(
        ["git", "diff-tree", "--no-commit-id", "-r", "--name-status", "HEAD"],
        cwd=repo_root,
    )

    files = []
    if diff_output:
        change_map = {"A": "created", "M": "modified", "D": "deleted"}
        for line in diff_output.splitlines():
            parts = line.split("\t", 1)
            if len(parts) == 2:
                files.append(
                    FileChange(
                        path=parts[1],
                        change_type=change_map.get(parts[0], "modified"),
                    )
                )

    return WorkflowEvent.make(
        source="git",
        event_type=EventType.GIT_COMMIT,
        task_id=sha,
        summary=msg[:200] if msg else None,
        git_commit_sha=sha,
        git_branch=branch,
        workspace_root=repo_root,
        files_changed=files or None,
    )
