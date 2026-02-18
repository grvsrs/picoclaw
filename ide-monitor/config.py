# IDE Monitor — configuration
# Override paths and endpoints via config.yaml or CLI args.

import yaml
from pathlib import Path
from dataclasses import dataclass, field
from typing import List, Optional

CONFIG_PATH = Path(__file__).parent / "config.yaml"


@dataclass
class Config:
    """Runtime configuration for the IDE monitor."""

    # Watch paths (auto-detected if empty)
    antigravity_brain_dir: str = ""
    antigravity_skills_dirs: List[str] = field(default_factory=list)
    copilot_log_dir: str = ""
    workspace_root: str = "."

    # Picoclaw integration
    picoclaw_url: Optional[str] = None  # None = standalone mode

    # Fallback storage
    jsonl_path: str = "~/.local/share/ide-monitor/events.jsonl"

    # Behavior — filesystem burst detector
    debounce_brain_ms: int = 800
    debounce_copilot_ms: int = 200
    debounce_skill_ms: int = 1000
    burst_window_secs: int = 5
    burst_threshold: int = 3

    # Behavior — Copilot burst detector
    copilot_burst_window_secs: float = 120.0
    copilot_burst_token_threshold: int = 500
    copilot_burst_count_threshold: int = 5

    # Behavior — Temporal correlation engine
    correlation_task_commit_window: float = 300.0  # 5 min
    correlation_cluster_window: float = 180.0      # 3 min

    def resolve_paths(self):
        """Expand ~ and set defaults based on detected OS paths."""
        home = Path.home()

        if not self.antigravity_brain_dir:
            self.antigravity_brain_dir = str(home / ".gemini" / "antigravity" / "brain")

        if not self.antigravity_skills_dirs:
            self.antigravity_skills_dirs = [
                str(home / ".gemini" / "antigravity" / "skills"),
            ]
            # Also check project-scoped skills
            ws = Path(self.workspace_root).resolve()
            project_skills = ws / ".agent" / "skills"
            if project_skills.exists():
                self.antigravity_skills_dirs.append(str(project_skills))

        if not self.copilot_log_dir:
            self.copilot_log_dir = str(
                home / ".config" / "Code" / "User" / "globalStorage" / "github.copilot" / "logs"
            )

        self.jsonl_path = str(Path(self.jsonl_path).expanduser())

    @classmethod
    def load(cls, path: Optional[str] = None) -> "Config":
        """Load config from YAML file, falling back to defaults."""
        cfg_path = Path(path) if path else CONFIG_PATH
        if cfg_path.exists():
            try:
                with open(cfg_path, "r") as f:
                    data = yaml.safe_load(f) or {}
                cfg = cls(**{k: v for k, v in data.items() if hasattr(cls, k)})
            except Exception:
                cfg = cls()
        else:
            cfg = cls()
        cfg.resolve_paths()
        return cfg
