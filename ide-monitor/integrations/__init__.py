# IDE Monitor — Picoclaw integration
#
# Optional module for direct Picoclaw interaction beyond the simple
# event POST. Can query kanban state, create cards, etc.

import requests
from typing import Optional, Dict, Any


class PicoclawClient:
    """HTTP client for the Picoclaw API."""

    def __init__(self, base_url: str = "http://localhost:8080"):
        self.base_url = base_url.rstrip("/")
        self.timeout = 5

    def post_event(self, payload: str) -> bool:
        """Post a raw WorkflowEvent JSON string."""
        try:
            r = requests.post(
                f"{self.base_url}/api/events",
                data=payload,
                headers={"Content-Type": "application/json"},
                timeout=self.timeout,
            )
            return r.ok
        except Exception:
            return False

    def create_task(self, title: str, description: str = "",
                    category: str = "code", source: str = "ide-monitor",
                    external_ref: str = "") -> Optional[Dict[str, Any]]:
        """Create a kanban task via the Go API."""
        try:
            r = requests.post(
                f"{self.base_url}/api/tasks",
                json={
                    "title": title,
                    "description": description,
                    "category": category,
                    "source": source,
                    "external_ref": external_ref,
                },
                timeout=self.timeout,
            )
            if r.ok:
                return r.json()
        except Exception:
            pass
        return None

    def health(self) -> bool:
        """Check if Picoclaw is reachable."""
        try:
            r = requests.get(f"{self.base_url}/api/health", timeout=2)
            return r.ok
        except Exception:
            return False
