# IDE Monitor — event emitter
#
# Sends WorkflowEvents to Picoclaw (if available) or writes to local JSONL.
# Dual-mode: connected or standalone, with retry queue.

import requests
import time
import threading
from pathlib import Path
from typing import Optional
from collections import deque

from normalizer import WorkflowEvent

# Set by watcher.py at startup based on CLI args
PICOCLAW_URL: Optional[str] = None
JSONL_PATH: str = str(Path("~/.local/share/ide-monitor/events.jsonl").expanduser())

# Retry queue for failed emits
_retry_queue = deque(maxlen=1000)  # Keep last 1000 failed events
_retry_lock = threading.Lock()
_retry_thread = None


def _flush_retry_queue():
    """Try to send any queued events that failed before."""
    global _retry_thread
    if not PICOCLAW_URL or _retry_thread and _retry_thread.is_alive():
        return
    
    def _retry_worker():
        while _retry_queue:
            with _retry_lock:
                if not _retry_queue:
                    break
                payload, first_try_time = _retry_queue[0]
            
            try:
                r = requests.post(
                    PICOCLAW_URL,
                    data=payload,
                    headers={"Content-Type": "application/json"},
                    timeout=2,
                )
                if r.ok:
                    with _retry_lock:
                        _retry_queue.popleft()
                    continue
            except Exception:
                pass
            break  # Backoff if still failing
    
    _retry_thread = threading.Thread(target=_retry_worker, daemon=True)
    _retry_thread.start()


def emit(event: WorkflowEvent):
    """
    Emit a WorkflowEvent. Tries Picoclaw first with retry queue,
    falls back to JSONL. Never raises.
    """
    payload = event.to_json()

    # Try Picoclaw if configured
    if PICOCLAW_URL:
        try:
            r = requests.post(
                PICOCLAW_URL,
                data=payload,
                headers={"Content-Type": "application/json"},
                timeout=2,
            )
            if r.ok:
                _print_event(event, "→ picoclaw")
                _flush_retry_queue()  # Try queued events again
                return
        except Exception:
            pass  # Fall through to retry queue

        # Add to retry queue instead of immediately falling back to JSONL
        with _retry_lock:
            _retry_queue.append((payload, time.time()))

    # Fallback: append to local JSONL file
    _write_jsonl(payload)
    _print_event(event, "→ jsonl")



def _write_jsonl(payload: str):
    """Append a JSON line to the fallback log file."""
    try:
        path = Path(JSONL_PATH)
        path.parent.mkdir(parents=True, exist_ok=True)
        with open(path, "a") as f:
            f.write(payload + "\n")
    except Exception as e:
        print(f"  [!] JSONL write error: {e}")


def _print_event(event: WorkflowEvent, dest: str):
    """Print a compact event summary to stdout."""
    parts = [
        f"  [{event.source}]",
        event.event_type,
    ]
    if event.task_title:
        parts.append(f'"{event.task_title}"')
    if event.tokens_prompt or event.tokens_completion:
        parts.append(f"tokens:{event.tokens_prompt or 0}+{event.tokens_completion or 0}")
    if event.summary:
        parts.append(event.summary[:60])
    parts.append(dest)
    print(" ".join(parts))
