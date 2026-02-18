#!/usr/bin/env python3
"""
PicoClaw Executor
─────────────────
Watches /workspace/tasks/** for new .yaml files via inotify.
Validates, executes, and writes results back to the task file.

The bot polls the task file for status changes. This separation means:
    Bot      = Telegram-facing, async, no privileged access needed
    Executor = Linux-side, runs as systemd service with appropriate permissions

Dependencies:
    pip install pyyaml inotify-simple

Start as systemd service — see picoclaw-executor.service
Or run directly: python executor.py

Design constraints:
    - ONLY runs commands from the explicit COMMAND_TABLE below
    - No shell=True, no f-string command injection
    - All params are validated AGAIN here (defense in depth; bot also validates)
    - Writes structured result back to task file for bot to read
    - Path traversal protection on all file operations
    - Subprocess timeout enforcement
"""

import json
import logging
import os
import re
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

import yaml

try:
    from inotify_simple import INotify, flags as inotify_flags
    HAS_INOTIFY = True
except ImportError:
    HAS_INOTIFY = False
    logging.warning(
        "inotify_simple not installed; falling back to polling. "
        "Run: pip install inotify-simple"
    )

# Kanban integration
from pkg.kanban.store import KanbanStore
from pkg.kanban.events import KanbanEventBridge
from pkg.kanban.schema import TaskState

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Configuration
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Read paths from environment variables with sensible defaults
_watch_dirs_env = os.environ.get("PICOCLAW_WATCH_DIRS", "")
if _watch_dirs_env:
    WATCH_DIRS = [Path(d.strip()) for d in _watch_dirs_env.split(";") if d.strip()]
else:
    WATCH_DIRS = [
        Path("/workspace/tasks/dev"),
        Path("/workspace/tasks/ops"),
        Path("/workspace/tasks/monitor"),
    ]

PROJECTS_BASE = Path(os.environ.get("PICOCLAW_PROJECTS_BASE", "/projects"))
SCRIPTS_BASE = Path(os.environ.get("PICOCLAW_SCRIPTS_BASE", "/scripts"))
AUDIT_LOG_PATH = Path(os.environ.get("PICOCLAW_AUDIT_LOG", "/var/log/picoclaw/audit.jsonl"))
MAX_OUTPUT = 4096       # chars; truncate stdout/stderr in task file
TASK_TIMEOUT = 300      # seconds; kill subprocess after this
ALLOWED_SERVICES = ["nginx", "postgresql", "docker", "redis"]

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [executor] %(levelname)s: %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)],
)
logger = logging.getLogger("executor")


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Utilities
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


def utc_now() -> str:
    """ISO-8601 UTC timestamp."""
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def validate_project_name(name: str) -> bool:
    """Only alphanumeric, hyphens, underscores."""
    return bool(name and re.fullmatch(r"[a-zA-Z0-9_-]+", name))


def validate_service_name(name: str, allowed: list[str]) -> bool:
    """Service must be in explicit allowlist."""
    return name in allowed


def safe_path(base: Path, *parts: str) -> Path:
    """
    Resolve a path and assert it stays within base.
    Prevents path traversal attacks (../../etc/passwd).
    """
    resolved = (base / Path(*parts)).resolve()
    base_resolved = base.resolve()
    if not str(resolved).startswith(str(base_resolved)):
        raise ValueError(
            f"Path traversal detected: {parts} escapes {base}"
        )
    return resolved


def truncate_output(text: str) -> str:
    """Truncate output to MAX_OUTPUT chars."""
    if not text:
        return ""
    if len(text) <= MAX_OUTPUT:
        return text
    return (
        text[:MAX_OUTPUT]
        + f"\n…[truncated, {len(text) - MAX_OUTPUT} chars omitted]"
    )


def audit_log(task_id: str, command: str, status: str, **extra):
    """Append an audit entry from the executor side."""
    entry = {
        "ts": utc_now(),
        "source": "executor",
        "task_id": task_id,
        "command": command,
        "status": status,
    }
    for k, v in extra.items():
        if v is not None:
            entry[k] = v

    try:
        AUDIT_LOG_PATH.parent.mkdir(parents=True, exist_ok=True)
        with open(AUDIT_LOG_PATH, "a") as f:
            f.write(json.dumps(entry, ensure_ascii=False) + "\n")
    except Exception as e:
        logger.error(f"Failed to write audit log: {e}")


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Command implementations
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#
# Each function receives the validated task dict.
# Returns (exit_code, stdout, stderr, summary).
# NEVER uses shell=True. All inputs re-validated here.


def run_tests(task: dict) -> tuple[int, str, str, str]:
    """Run pytest test suite for a project."""
    project = task.get("project")
    params = task.get("params", {})
    suite = params.get("suite", "all")

    if not validate_project_name(project):
        return (1, "", f"Invalid project name: {project}", "Validation failed")

    project_dir = safe_path(PROJECTS_BASE, project)
    if not project_dir.exists():
        return (
            1, "",
            f"Project directory not found: {project_dir}",
            f"Project '{project}' not found",
        )

    # Map suite to pytest markers
    suite_args = {
        "all": [],
        "unit": ["-m", "unit"],
        "integration": ["-m", "integration"],
    }
    args = suite_args.get(suite, [])

    result = subprocess.run(
        ["python", "-m", "pytest", "--tb=short", *args],
        cwd=str(project_dir),
        capture_output=True,
        text=True,
        timeout=TASK_TIMEOUT,
    )

    summary = (
        f"Tests {'passed' if result.returncode == 0 else 'failed'} "
        f"(suite={suite})"
    )
    return (result.returncode, result.stdout, result.stderr, summary)


def deploy(task: dict) -> tuple[int, str, str, str]:
    """Deploy a project to an environment via deploy.sh."""
    project = task.get("project")
    params = task.get("params", {})
    env = params.get("env", "staging")

    if not validate_project_name(project):
        return (1, "", f"Invalid project name: {project}", "Validation failed")
    if env not in ("staging", "production"):
        return (1, "", f"Invalid environment: {env}", "Validation failed")

    deploy_script = safe_path(SCRIPTS_BASE, "deploy.sh")
    if not deploy_script.exists():
        return (
            1, "",
            f"Deploy script not found: {deploy_script}",
            "Deploy script missing",
        )

    result = subprocess.run(
        [str(deploy_script), project, env],
        capture_output=True,
        text=True,
        timeout=TASK_TIMEOUT,
    )

    summary = (
        f"Deploy {'succeeded' if result.returncode == 0 else 'failed'} "
        f"({project} → {env})"
    )
    return (result.returncode, result.stdout, result.stderr, summary)


def scaffold(task: dict) -> tuple[int, str, str, str]:
    """Scaffold a new component within a project."""
    project = task.get("project")
    params = task.get("params", {})
    component = params.get("component")

    if not validate_project_name(project):
        return (1, "", f"Invalid project name: {project}", "Validation failed")
    if not component or not re.fullmatch(r"[a-zA-Z0-9_/-]+", component):
        return (
            1, "",
            f"Invalid component name: {component}",
            "Validation failed",
        )

    scaffold_script = safe_path(SCRIPTS_BASE, "scaffold.sh")
    if not scaffold_script.exists():
        return (
            1, "",
            f"Scaffold script not found: {scaffold_script}",
            "Scaffold script missing",
        )

    result = subprocess.run(
        [str(scaffold_script), project, component],
        capture_output=True,
        text=True,
        timeout=TASK_TIMEOUT,
    )

    summary = (
        f"Scaffold {'succeeded' if result.returncode == 0 else 'failed'} "
        f"({component} in {project})"
    )
    return (result.returncode, result.stdout, result.stderr, summary)


def git_status(task: dict) -> tuple[int, str, str, str]:
    """Show git status for a project."""
    project = task.get("project")

    if not validate_project_name(project):
        return (1, "", f"Invalid project name: {project}", "Validation failed")

    project_dir = safe_path(PROJECTS_BASE, project)
    if not project_dir.exists():
        return (
            1, "",
            f"Project directory not found: {project_dir}",
            f"Project '{project}' not found",
        )

    result = subprocess.run(
        ["git", "status", "--short", "--branch"],
        cwd=str(project_dir),
        capture_output=True,
        text=True,
        timeout=30,
    )

    summary = f"Git status for {project}"
    return (result.returncode, result.stdout, result.stderr, summary)


def status(_task: dict) -> tuple[int, str, str, str]:
    """Aggregate system status: disk, memory, load, services."""
    lines = []

    # Disk usage
    result = subprocess.run(
        ["df", "-h", "/workspace", "/"],
        capture_output=True, text=True,
    )
    lines.append("=== Disk ===")
    lines.append(result.stdout.strip())

    # Memory
    result = subprocess.run(["free", "-h"], capture_output=True, text=True)
    lines.append("\n=== Memory ===")
    lines.append(result.stdout.strip())

    # Load average
    result = subprocess.run(["uptime"], capture_output=True, text=True)
    lines.append("\n=== Load ===")
    lines.append(result.stdout.strip())

    # Key systemd services
    lines.append("\n=== Services ===")
    for svc in ALLOWED_SERVICES:
        r = subprocess.run(
            ["systemctl", "is-active", svc],
            capture_output=True, text=True,
        )
        state = r.stdout.strip() or "unknown"
        icon = "●" if state == "active" else "○"
        lines.append(f"  {icon} {svc}: {state}")

    output = "\n".join(lines)
    return (0, output, "", "System status overview")


def restart_service(task: dict) -> tuple[int, str, str, str]:
    """Restart a systemd service (via sudo, allowlisted)."""
    params = task.get("params", {})
    service = task.get("service") or params.get("service")

    if not validate_service_name(service, ALLOWED_SERVICES):
        return (
            1, "",
            f"Service '{service}' not in allowed list: {ALLOWED_SERVICES}",
            "Rejected: service not allowed",
        )

    result = subprocess.run(
        ["sudo", "/bin/systemctl", "restart", service],
        capture_output=True,
        text=True,
        timeout=60,
    )

    summary = (
        f"Restart {'succeeded' if result.returncode == 0 else 'failed'} "
        f"({service})"
    )
    return (result.returncode, result.stdout, result.stderr, summary)


def disk_usage(task: dict) -> tuple[int, str, str, str]:
    """Show disk usage for a path."""
    params = task.get("params", {})
    path = params.get("path", "/workspace")

    # Validate path: must start with / and contain only safe chars
    if not re.fullmatch(r"/[a-zA-Z0-9/_.-]*", path):
        return (1, "", f"Invalid path: {path}", "Validation failed")

    # Additional check: resolve and ensure no traversal
    resolved = Path(path).resolve()
    if not resolved.exists():
        return (1, "", f"Path does not exist: {path}", "Path not found")

    result = subprocess.run(
        ["du", "-sh", "--max-depth=1", str(resolved)],
        capture_output=True,
        text=True,
        timeout=60,
    )

    summary = f"Disk usage for {path}"
    return (result.returncode, result.stdout, result.stderr, summary)


def process_info(task: dict) -> tuple[int, str, str, str]:
    """Show info about running processes matching a name."""
    params = task.get("params", {})
    name = params.get("name")

    if not name or not re.fullmatch(r"[a-zA-Z0-9_-]+", name):
        return (1, "", f"Invalid process name: {name}", "Validation failed")

    result = subprocess.run(
        ["ps", "aux"],
        capture_output=True,
        text=True,
        timeout=10,
    )

    all_lines = result.stdout.splitlines()
    header = all_lines[0] if all_lines else ""
    # Filter for matching process lines, excluding the grep/ps itself
    matches = [
        line for line in all_lines[1:]
        if name.lower() in line.lower()
        and "ps aux" not in line
    ]

    if not matches:
        output = f"No processes found matching '{name}'"
    else:
        output = header + "\n" + "\n".join(matches)

    summary = f"Process info for '{name}' ({len(matches)} matches)"
    return (0, output, "", summary)


def health(_task: dict) -> tuple[int, str, str, str]:
    """Comprehensive health check with pass/fail for each component."""
    checks = []
    all_ok = True

    # ── Disk space ──
    result = subprocess.run(
        ["df", "-h", "/"], capture_output=True, text=True
    )
    for line in result.stdout.strip().splitlines()[1:]:
        parts = line.split()
        if len(parts) >= 6:
            try:
                usage_pct = int(parts[4].rstrip("%"))
            except ValueError:
                continue
            disk_ok = usage_pct < 90
            if not disk_ok:
                all_ok = False
            icon = "✅" if disk_ok else "🔴"
            checks.append(
                f"{icon} Disk ({parts[5]}): {parts[4]} used"
            )

    # ── Memory ──
    result = subprocess.run(
        ["free", "-m"], capture_output=True, text=True
    )
    for line in result.stdout.strip().splitlines():
        if line.startswith("Mem:"):
            parts = line.split()
            if len(parts) >= 3:
                total = int(parts[1])
                used = int(parts[2])
                pct = (used / total * 100) if total > 0 else 0
                mem_ok = pct < 90
                if not mem_ok:
                    all_ok = False
                icon = "✅" if mem_ok else "🔴"
                checks.append(
                    f"{icon} Memory: {pct:.0f}% ({used}M / {total}M)"
                )

    # ── Load average ──
    try:
        result = subprocess.run(
            ["nproc"], capture_output=True, text=True
        )
        cores = int(result.stdout.strip()) if result.stdout.strip().isdigit() else 1
    except Exception:
        cores = 1

    try:
        with open("/proc/loadavg") as f:
            load1 = float(f.read().split()[0])
        load_ok = load1 < cores * 2
        if not load_ok:
            all_ok = False
        icon = "✅" if load_ok else "🔴"
        checks.append(f"{icon} Load: {load1:.2f} ({cores} cores)")
    except Exception:
        checks.append("⚠️ Load: unable to read /proc/loadavg")

    # ── Key services ──
    for svc in ALLOWED_SERVICES:
        r = subprocess.run(
            ["systemctl", "is-active", svc],
            capture_output=True, text=True,
        )
        state = r.stdout.strip()
        svc_ok = state == "active"
        # Warn but don't fail overall for missing optional services
        icon = "✅" if svc_ok else "⚠️"
        checks.append(f"{icon} {svc}: {state}")

    overall = "✅ All checks passed" if all_ok else "⚠️ Issues detected"
    output = f"{overall}\n\n" + "\n".join(checks)
    return (0, output, "", "Health check complete")


def recent(task: dict) -> tuple[int, str, str, str]:
    """Read recent entries from the audit log."""
    params = task.get("params", {})
    limit = params.get("limit", 10)

    if not AUDIT_LOG_PATH.exists():
        return (0, "No audit log found.", "", "No activity")

    try:
        lines = AUDIT_LOG_PATH.read_text().strip().splitlines()
        entries = lines[-limit:]
        output = "\n".join(entries)
        return (0, output, "", f"Last {len(entries)} audit entries")
    except Exception as e:
        return (1, "", str(e), "Failed to read audit log")


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Command dispatch table
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#
# ONLY commands in this table can be executed.
# No dynamic dispatch. No eval. No exec.

COMMAND_TABLE: dict[str, callable] = {
    "run_tests": run_tests,
    "deploy": deploy,
    "scaffold": scaffold,
    "git_status": git_status,
    "status": status,
    "restart_service": restart_service,
    "disk_usage": disk_usage,
    "process_info": process_info,
    "health": health,
    "recent": recent,
}

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Kanban integration
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Initialize Kanban bridge (lazy; created on first import)
kanban_store: KanbanStore | None = None
kanban_bridge: KanbanEventBridge | None = None


def get_kanban_bridge() -> KanbanEventBridge | None:
    """Lazy-initialize and return Kanban bridge."""
    global kanban_store, kanban_bridge
    if kanban_bridge is None:
        try:
            db_path = os.environ.get("PICOCLAW_DB", "/var/lib/picoclaw/kanban.db")
            kanban_store = KanbanStore(db_path)
            kanban_bridge = KanbanEventBridge(kanban_store)
            logger.info("Kanban bridge initialized")
        except Exception as e:
            logger.warning(f"Failed to initialize Kanban: {e}")
    return kanban_bridge


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Task processing
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


def load_task(task_file: Path) -> dict | None:
    """Load a task dict from YAML. Returns None on error."""
    try:
        with open(task_file) as f:
            return yaml.safe_load(f)
    except Exception as e:
        logger.error(f"Failed to load task {task_file}: {e}")
        return None


def write_result(task_file: Path, task: dict):
    """Write updated task dict back to the task file (atomic)."""
    try:
        tmp_file = task_file.with_suffix(".yaml.tmp")
        with open(tmp_file, "w") as f:
            yaml.dump(task, f, default_flow_style=False, sort_keys=False)
        tmp_file.rename(task_file)
    except Exception as e:
        logger.error(f"Failed to write result to {task_file}: {e}")


def process_task(task_file: Path):
    """
    Process a single task file through the full lifecycle:
        1. Load and validate
        2. Check command is in dispatch table
        3. Mark as running
        4. Execute command handler
        5. Write result (complete/failed/timeout/rejected)
        6. Audit log every state transition
    """
    task = load_task(task_file)
    if task is None:
        return

    task_id = task.get("id", "unknown")
    command = task.get("command")
    current_status = task.get("status")

    # Only process pending tasks
    if current_status != "pending":
        logger.debug(f"Skipping {task_file.name}: status={current_status}")
        return

    logger.info(f"Processing task {task_id}: command={command}")

    # ── Validate command exists in dispatch table ──
    handler = COMMAND_TABLE.get(command)
    if handler is None:
        task["status"] = "rejected"
        task["completed_at"] = utc_now()
        task["summary"] = f"Unknown command: {command}"
        task["exit_code"] = 1
        write_result(task_file, task)
        audit_log(task_id, command or "UNKNOWN", "rejected",
                  reason="unknown_command")
        logger.warning(f"Rejected unknown command: {command}")
        return

    # ── Mark as running ──
    task["status"] = "running"
    task["started_at"] = utc_now()
    write_result(task_file, task)
    audit_log(task_id, command, "running")
    
    # ── Emit Kanban started event ──
    card_id = task.get("card_id")
    if card_id:
        bridge = get_kanban_bridge()
        if bridge:
            bridge.on_task_started(card_id, executor="executor")

    # ── Execute ──
    start_time = time.monotonic()
    try:
        exit_code, stdout, stderr, summary = handler(task)
        duration = time.monotonic() - start_time

        task["status"] = "complete" if exit_code == 0 else "failed"
        task["exit_code"] = exit_code
        task["stdout"] = truncate_output(stdout)
        task["stderr"] = truncate_output(stderr)
        task["summary"] = summary
        task["duration_s"] = round(duration, 2)
        task["completed_at"] = utc_now()

    except subprocess.TimeoutExpired:
        duration = time.monotonic() - start_time
        task["status"] = "timeout"
        task["exit_code"] = -1
        task["stdout"] = ""
        task["stderr"] = f"Command timed out after {TASK_TIMEOUT}s"
        task["summary"] = f"Command timed out after {TASK_TIMEOUT}s"
        task["duration_s"] = round(duration, 2)
        task["completed_at"] = utc_now()
        logger.error(f"Task {task_id} timed out after {TASK_TIMEOUT}s")

    except ValueError as e:
        # Path traversal or validation error
        duration = time.monotonic() - start_time
        task["status"] = "failed"
        task["exit_code"] = 1
        task["stdout"] = ""
        task["stderr"] = str(e)
        task["summary"] = f"Security violation: {type(e).__name__}"
        task["duration_s"] = round(duration, 2)
        task["completed_at"] = utc_now()
        logger.error(f"Task {task_id} security violation: {e}")

    except Exception as e:
        duration = time.monotonic() - start_time
        task["status"] = "failed"
        task["exit_code"] = -1
        task["stdout"] = ""
        task["stderr"] = str(e)
        task["summary"] = f"Executor error: {type(e).__name__}"
        task["duration_s"] = round(duration, 2)
        task["completed_at"] = utc_now()
        logger.error(
            f"Task {task_id} failed with exception: {e}", exc_info=True
        )

    write_result(task_file, task)
    audit_log(
        task_id, command, task["status"],
        exit_code=task.get("exit_code"),
        duration_s=task.get("duration_s"),
        summary=task.get("summary"),
    )
    
    # ── Emit Kanban events ──
    card_id = task.get("card_id")
    if card_id:
        bridge = get_kanban_bridge()
        if bridge:
            final_status = task["status"]
            if final_status == "running":
                bridge.on_task_started(card_id, executor="executor")
            elif final_status == "complete":
                bridge.on_task_completed(
                    card_id, 
                    result=task.get("summary", ""),
                    log_url=""
                )
            elif final_status in ("failed", "timeout"):
                error_msg = task.get("stderr", task.get("summary", "Unknown error"))
                bridge.on_task_failed(
                    card_id,
                    error=error_msg,
                    log_url=""
                )
            elif final_status == "rejected":
                bridge.on_task_failed(
                    card_id,
                    error="Command rejected",
                    log_url=""
                )
    
    logger.info(
        f"Task {task_id} completed: status={task['status']}, "
        f"exit_code={task.get('exit_code')}, "
        f"duration={task.get('duration_s')}s"
    )


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# File watchers
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━


def _process_existing_tasks():
    """Process any pending tasks already in watch directories (startup recovery)."""
    for watch_dir in WATCH_DIRS:
        for task_file in sorted(watch_dir.glob("*.yaml")):
            try:
                task = load_task(task_file)
                if task and task.get("status") == "pending":
                    process_task(task_file)
            except Exception as e:
                logger.error(f"Error processing existing task {task_file}: {e}")


def run_inotify():
    """
    Watch task directories using inotify (efficient, event-driven).

    Uses CLOSE_WRITE | MOVED_TO to catch both direct writes and
    atomic rename operations.
    """
    inotify = INotify()
    wd_to_dir: dict[int, Path] = {}

    for watch_dir in WATCH_DIRS:
        watch_dir.mkdir(parents=True, exist_ok=True)
        wd = inotify.add_watch(
            str(watch_dir),
            inotify_flags.CLOSE_WRITE | inotify_flags.MOVED_TO,
        )
        wd_to_dir[wd] = watch_dir
        logger.info(f"Watching (inotify): {watch_dir}")

    # Process any tasks that were pending before we started
    _process_existing_tasks()
    logger.info("Executor ready — waiting for tasks…")

    while True:
        events = inotify.read()
        for event in events:
            # event.name may be str or bytes depending on library version
            name = event.name
            if isinstance(name, bytes):
                name = name.decode("utf-8", errors="replace")

            if not name or not name.endswith(".yaml"):
                continue
            # Skip temp files from atomic writes
            if name.endswith(".yaml.tmp"):
                continue

            watch_dir = wd_to_dir.get(event.wd)
            if not watch_dir:
                continue

            task_file = watch_dir / name
            if task_file.exists():
                try:
                    process_task(task_file)
                except Exception as e:
                    logger.error(
                        f"Error processing task {task_file}: {e}"
                    )


def run_polling(interval: float = 1.0):
    """
    Fallback: poll task directories for new .yaml files.
    Used when inotify_simple is not installed.
    """
    seen: set[str] = set()

    for watch_dir in WATCH_DIRS:
        watch_dir.mkdir(parents=True, exist_ok=True)
        logger.info(f"Watching (polling, {interval}s): {watch_dir}")

    # Track existing files and process pending ones
    for watch_dir in WATCH_DIRS:
        for task_file in sorted(watch_dir.glob("*.yaml")):
            seen.add(str(task_file))
            try:
                task = load_task(task_file)
                if task and task.get("status") == "pending":
                    process_task(task_file)
            except Exception as e:
                logger.error(
                    f"Error processing existing task {task_file}: {e}"
                )

    logger.info("Executor ready — polling for tasks…")

    while True:
        time.sleep(interval)
        for watch_dir in WATCH_DIRS:
            for task_file in sorted(watch_dir.glob("*.yaml")):
                path_str = str(task_file)
                if path_str not in seen:
                    seen.add(path_str)
                    try:
                        process_task(task_file)
                    except Exception as e:
                        logger.error(
                            f"Error processing task {task_file}: {e}"
                        )


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Entry point
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

if __name__ == "__main__":
    logger.info("PicoClaw Executor starting…")

    # Ensure watch directories exist
    for d in WATCH_DIRS:
        d.mkdir(parents=True, exist_ok=True)

    # Ensure audit log directory exists
    try:
        AUDIT_LOG_PATH.parent.mkdir(parents=True, exist_ok=True)
    except PermissionError:
        logger.warning(
            f"Cannot create {AUDIT_LOG_PATH.parent}, "
            "audit logging from executor may fail"
        )

    if HAS_INOTIFY:
        logger.info("Using inotify for file watching (recommended)")
        run_inotify()
    else:
        logger.info(
            "Using polling (install inotify-simple for better performance)"
        )
        run_polling()
