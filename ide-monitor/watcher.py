#!/usr/bin/env python3
"""
IDE Monitor — main entry point

Watches VS Code Copilot logs + Google Antigravity brain artifacts + git activity.
Emits normalized WorkflowEvents to Picoclaw or local JSONL.

Usage:
    python watcher.py                              # standalone, default paths
    python watcher.py --workspace ~/myproject       # specify workspace
    python watcher.py --picoclaw http://localhost:8080  # connect to picoclaw
    python watcher.py --picoclaw off                # force standalone mode
"""

import sys
import time
import argparse
from pathlib import Path

from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

from config import Config
from parsers.antigravity import parse_brain_event, parse_skill_event
from parsers.copilot import parse_copilot_entries
from parsers.git import parse_commit_event
from burst_detector import FileBurstDetector, CopilotBurstDetector
from correlator import TemporalCorrelator
from confidence import ConfidenceScorer
from normalizer import EventType
import emitter

# ── Debounce state ─────────────────────────────────────────────────────────

_last_seen: dict = {}


def _debounce(path: str, ms: int) -> bool:
    """Return True if this path hasn't been seen within the debounce window."""
    now = time.monotonic()
    if path in _last_seen and (now - _last_seen[path]) < (ms / 1000):
        return False
    _last_seen[path] = now
    return True


# ── Unified filesystem handler ─────────────────────────────────────────────

class UnifiedHandler(FileSystemEventHandler):
    """Routes filesystem events to the correct parser, scorer, and correlator."""

    def __init__(self, cfg: Config, burst: FileBurstDetector,
                 copilot_burst: CopilotBurstDetector,
                 correlator_: TemporalCorrelator,
                 scorer: ConfidenceScorer):
        self.cfg = cfg
        self.burst = burst
        self.copilot_burst = copilot_burst
        self.correlator = correlator_
        self.scorer = scorer
        self.brain_dir = Path(cfg.antigravity_brain_dir).resolve()
        self.skills_dirs = [Path(d).resolve() for d in cfg.antigravity_skills_dirs]
        self.copilot_dir = Path(cfg.copilot_log_dir).resolve()
        self.workspace = Path(cfg.workspace_root).resolve()

    def on_any_event(self, fs_event):
        if fs_event.is_directory:
            return
        self._route(fs_event.src_path)

    def _route(self, path: str):
        p = Path(path).resolve()

        # Antigravity brain artifacts
        if self._is_under(p, self.brain_dir) and p.suffix == ".md":
            if not _debounce(path, self.cfg.debounce_brain_ms):
                return
            event = parse_brain_event(path, str(self.brain_dir))
            if event:
                self._emit_pipeline(event)
            return

        # Antigravity skills
        for skills_dir in self.skills_dirs:
            if self._is_under(p, skills_dir):
                if not _debounce(path, self.cfg.debounce_skill_ms):
                    return
                event = parse_skill_event(path)
                if event:
                    self._emit_pipeline(event)
                return

        # Copilot logs
        if self._is_under(p, self.copilot_dir) and p.suffix == ".json":
            if not _debounce(path, self.cfg.debounce_copilot_ms):
                return
            for event in parse_copilot_entries(path):
                # Feed through burst detector first
                burst_events = self.copilot_burst.observe(event)
                # Emit burst_start/burst_end events
                for be in burst_events:
                    self._emit_pipeline(be)
                # Emit the (now burst-annotated) completion event
                self._emit_pipeline(event)
            return

        # Git commits
        if p.name == "COMMIT_EDITMSG":
            if not _debounce(path, 100):
                return
            event = parse_commit_event(path)
            if event:
                self._emit_pipeline(event)
            return

        # Burst detection for everything else (agent code-writing)
        burst_event = self.burst.observe(path, str(self.workspace))
        if burst_event:
            self._emit_pipeline(burst_event)

    def _emit_pipeline(self, event):
        """
        Central emission pipeline:
          1. Score confidence
          2. Record in correlation windows
          3. Generate derived correlation events
          4. Emit all
        """
        # Step 1: Confidence scoring
        self.scorer.score_and_record(event)

        # Step 2: Record in correlator windows
        self.correlator.record(event)

        # Step 3: Generate derived events (linked commits, clusters)
        derived = self.correlator.correlate(event)

        # Step 4: Emit primary event
        emitter.emit(event)

        # Step 5: Emit any derived events (already scored by correlator)
        for d in derived:
            self.scorer.score_and_record(d)
            emitter.emit(d)

    @staticmethod
    def _is_under(path: Path, parent: Path) -> bool:
        try:
            path.relative_to(parent)
            return True
        except ValueError:
            return False


# ── Main ───────────────────────────────────────────────────────────────────

def start(cfg: Config):
    """Start the filesystem observer and monitoring loop."""

    burst = FileBurstDetector(
        window_secs=cfg.burst_window_secs,
        threshold=cfg.burst_threshold,
    )
    copilot_burst = CopilotBurstDetector(
        window_secs=cfg.copilot_burst_window_secs,
        token_threshold=cfg.copilot_burst_token_threshold,
        count_threshold=cfg.copilot_burst_count_threshold,
    )
    correlator_ = TemporalCorrelator(
        task_commit_window=cfg.correlation_task_commit_window,
        cluster_window=cfg.correlation_cluster_window,
    )
    scorer = ConfidenceScorer()
    handler = UnifiedHandler(cfg, burst, copilot_burst, correlator_, scorer)

    watch_paths = [
        ("antigravity brain", Path(cfg.antigravity_brain_dir)),
        ("copilot logs", Path(cfg.copilot_log_dir)),
        ("git", Path(cfg.workspace_root) / ".git"),
    ]
    for d in cfg.antigravity_skills_dirs:
        watch_paths.append(("antigravity skills", Path(d)))
    watch_paths.append(("workspace", Path(cfg.workspace_root)))

    observer = Observer()
    active = 0

    for name, wp in watch_paths:
        if wp.exists():
            observer.schedule(handler, str(wp), recursive=True)
            print(f"  \u2713 {name}: {wp}")
            active += 1
        else:
            print(f"  \u2717 {name}: {wp} (not found, skipping)")

    if active == 0:
        print("\nNo watch paths found. Exiting.")
        sys.exit(1)

    observer.start()
    print(f"\nide-monitor v1.0 running ({active} paths). Ctrl+C to stop.\n")

    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print("\nStopping...")
        # Flush any active copilot burst
        flush_event = copilot_burst.flush()
        if flush_event:
            scorer.score_and_record(flush_event)
            emitter.emit(flush_event)
        observer.stop()
    observer.join()


def main():
    ap = argparse.ArgumentParser(
        description="IDE Workflow Monitor — Antigravity + Copilot → Picoclaw"
    )
    ap.add_argument(
        "--workspace", default=".",
        help="Workspace root to monitor (default: current directory)",
    )
    ap.add_argument(
        "--picoclaw", default=None,
        help="Picoclaw endpoint (e.g. http://localhost:8080). Omit for standalone.",
    )
    ap.add_argument(
        "--jsonl", default=None,
        help="Fallback JSONL path (default: ~/.local/share/ide-monitor/events.jsonl)",
    )
    ap.add_argument(
        "--config", default=None,
        help="Path to config.yaml",
    )
    args = ap.parse_args()

    # Load config
    cfg = Config.load(args.config)
    cfg.workspace_root = args.workspace

    # CLI overrides
    if args.picoclaw and args.picoclaw != "off":
        cfg.picoclaw_url = args.picoclaw
    elif args.picoclaw == "off":
        cfg.picoclaw_url = None

    if args.jsonl:
        cfg.jsonl_path = args.jsonl

    cfg.resolve_paths()

    # Configure emitter
    if cfg.picoclaw_url:
        emitter.PICOCLAW_URL = f"{cfg.picoclaw_url.rstrip('/')}/api/events"
        print(f"Picoclaw: {emitter.PICOCLAW_URL}")
    else:
        emitter.PICOCLAW_URL = None
        print("Picoclaw: off (standalone mode)")

    emitter.JSONL_PATH = cfg.jsonl_path
    print(f"JSONL fallback: {cfg.jsonl_path}")
    print()

    start(cfg)


if __name__ == "__main__":
    main()
