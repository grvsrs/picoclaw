"""
LLM-powered task categorizer for the Kanban system.

Uses the configured LLM to automatically categorize tasks by analyzing
their title and description, assigning:
  - Category (code, design, infra, bug, feature, research, ops, personal, meeting)
  - Priority suggestion (low, normal, high, critical)
  - Tags
  - Project inference

Can be used from:
  - Python bots (direct import)
  - Go backend (via integration API)
  - CLI tools
"""
import json
import os
import re
from typing import Dict, Any, Optional, Tuple, List

from .schema import TaskCategory, KanbanCard


# System prompt for the LLM categorizer
CATEGORIZER_PROMPT = """You are a task categorizer for a personal task board. Given a task title and optional description, return a JSON object with:

1. "category": one of: code, design, infra, bug, feature, research, ops, personal, meeting, uncategorized
2. "priority": one of: low, normal, high, critical
3. "tags": array of 1-3 short lowercase tags (e.g., ["auth", "backend", "urgent"])
4. "project": inferred project name if mentioned (lowercase, hyphenated), or "" if unclear
5. "summary": a one-line clarified summary of the task (max 80 chars)

Category definitions:
- code: writing or modifying source code
- design: UI/UX, architecture, system design
- infra: deployment, CI/CD, server setup, DevOps
- bug: fixing broken functionality
- feature: new capabilities or enhancements
- research: investigation, learning, exploration
- ops: monitoring, maintenance, operations
- personal: non-technical personal tasks
- meeting: calls, reviews, syncs
- uncategorized: cannot determine

Rules:
- Be decisive — avoid "uncategorized" when possible
- Prefer "bug" over "code" when the task mentions fixing/broken/error
- Prefer "feature" over "code" when the task mentions "add", "implement", "new"
- Extract project names from patterns like "project-name:" or "[project]" in titles
- Respond with ONLY the JSON object, no markdown, no explanation"""


def categorize_task_with_prompt(title: str, description: str = "") -> str:
    """Build the user prompt for LLM categorization."""
    prompt = f"Task: {title}"
    if description:
        prompt += f"\nDescription: {description[:500]}"
    return prompt


def parse_categorization_response(response: str) -> Dict[str, Any]:
    """Parse the LLM's JSON response into a categorization dict."""
    # Strip markdown code fences if present
    response = response.strip()
    if response.startswith("```"):
        response = re.sub(r'^```(?:json)?\s*', '', response)
        response = re.sub(r'\s*```$', '', response)
    
    try:
        result = json.loads(response)
    except json.JSONDecodeError:
        # Try to extract JSON from the response
        match = re.search(r'\{[^}]+\}', response, re.DOTALL)
        if match:
            try:
                result = json.loads(match.group())
            except json.JSONDecodeError:
                return _default_categorization()
        else:
            return _default_categorization()
    
    # Validate and normalize
    valid_categories = [c.value for c in TaskCategory]
    if result.get("category") not in valid_categories:
        result["category"] = "uncategorized"
    
    valid_priorities = ["low", "normal", "high", "critical"]
    if result.get("priority") not in valid_priorities:
        result["priority"] = "normal"
    
    if not isinstance(result.get("tags"), list):
        result["tags"] = []
    result["tags"] = [str(t).lower()[:20] for t in result["tags"][:5]]
    
    if not isinstance(result.get("project"), str):
        result["project"] = ""
    
    if not isinstance(result.get("summary"), str):
        result["summary"] = ""
    
    return result


def _default_categorization() -> Dict[str, Any]:
    """Fallback categorization when LLM fails."""
    return {
        "category": "uncategorized",
        "priority": "normal",
        "tags": [],
        "project": "",
        "summary": "",
    }


def categorize_by_rules(title: str, description: str = "") -> Dict[str, Any]:
    """
    Rule-based fallback categorizer (no LLM needed).
    Used when LLM is unavailable or for fast inline categorization.
    """
    text = (title + " " + description).lower()
    
    # Category detection rules
    category = "uncategorized"
    priority = "normal"
    tags = []
    project = ""
    
    # Project extraction: "project: xyz" or "[xyz]" prefix
    proj_match = re.match(r'^(?:\[([^\]]+)\]|([a-z0-9_-]+):)\s*', title, re.I)
    if proj_match:
        project = (proj_match.group(1) or proj_match.group(2)).lower().strip()
    
    # Bug detection
    bug_words = ["fix", "bug", "broken", "error", "crash", "fail", "issue", "patch", "hotfix"]
    if any(w in text for w in bug_words):
        category = "bug"
        tags.append("bugfix")
    
    # Feature detection
    elif any(w in text for w in ["add", "implement", "new", "feature", "create", "build"]):
        category = "feature"
    
    # Infrastructure
    elif any(w in text for w in ["deploy", "ci/cd", "docker", "server", "infra", "setup", "install",
                                   "nginx", "systemd", "tailscale", "ssh"]):
        category = "infra"
        tags.append("devops")
    
    # Design
    elif any(w in text for w in ["design", "ui", "ux", "mockup", "wireframe", "layout", "css", "style"]):
        category = "design"
    
    # Research
    elif any(w in text for w in ["research", "investigate", "explore", "learn", "study", "evaluate", "compare"]):
        category = "research"
    
    # Ops
    elif any(w in text for w in ["monitor", "alert", "log", "backup", "maintain", "update dependencies"]):
        category = "ops"
    
    # Meeting
    elif any(w in text for w in ["meeting", "call", "sync", "review", "standup", "retro"]):
        category = "meeting"
    
    # Code (broad)
    elif any(w in text for w in ["code", "refactor", "test", "api", "endpoint", "function", "class",
                                   "module", "import", "parse", "query"]):
        category = "code"
    
    # Priority detection
    if any(w in text for w in ["urgent", "critical", "asap", "immediately", "emergency"]):
        priority = "critical"
        tags.append("urgent")
    elif any(w in text for w in ["important", "high priority", "blocker"]):
        priority = "high"
    elif any(w in text for w in ["nice to have", "low priority", "someday", "eventually"]):
        priority = "low"
    
    # Tag extraction from common patterns
    tag_patterns = [
        (r'\b(auth|authentication)\b', 'auth'),
        (r'\b(frontend|ui|client)\b', 'frontend'),
        (r'\b(backend|server|api)\b', 'backend'),
        (r'\b(database|db|sql|sqlite)\b', 'database'),
        (r'\b(test|testing|spec)\b', 'testing'),
        (r'\b(docs?|documentation)\b', 'docs'),
        (r'\b(security|auth|encryption)\b', 'security'),
        (r'\b(performance|perf|speed|slow)\b', 'performance'),
    ]
    for pattern, tag in tag_patterns:
        if re.search(pattern, text) and tag not in tags:
            tags.append(tag)
    
    return {
        "category": category,
        "priority": priority,
        "tags": tags[:5],
        "project": project,
        "summary": "",
    }


def apply_categorization(card: KanbanCard, categorization: Dict[str, Any], 
                          from_llm: bool = False) -> KanbanCard:
    """Apply categorization results to a card."""
    try:
        card.category = TaskCategory(categorization.get("category", "uncategorized"))
    except ValueError:
        card.category = TaskCategory.UNCATEGORIZED
    
    if categorization.get("priority"):
        card.priority = categorization["priority"]
    
    if categorization.get("tags"):
        # Merge with existing tags
        existing = set(card.tags or [])
        new_tags = set(categorization["tags"])
        card.tags = list(existing | new_tags)
    
    if categorization.get("project") and not card.project:
        card.project = categorization["project"]
    
    if categorization.get("summary"):
        card.llm_summary = categorization["summary"]
    
    card.llm_categorized = from_llm
    
    return card
