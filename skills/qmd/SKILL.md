````skill
---
name: qmd
description: Search the picoclaw knowledge base — notes, kanban task history, workspace docs, IDE monitor logs, and past decisions — using QMD local hybrid search. Use when asked about project history, previous decisions, task progress, or to look up information in indexed documents. Triggers on "search my notes", "find in docs", "what did we decide about", "kanban history", "look up", "past decisions", "search workspace".
license: MIT
compatibility: Requires qmd CLI installed. Install via `npm install -g @tobilu/qmd` or `bun install -g @tobilu/qmd`. Enable in picoclaw config: tools.qmd.enabled = true.
metadata:
  author: picoclaw
  version: "1.0.0"
---

# QMD — picoclaw Knowledge Base Search

QMD gives you fast, accurate retrieval from indexed local documents before calling the LLM.
Always search before answering questions about past decisions, kanban tasks, agent history, or workspace content.

## Indexed Collections

| Collection    | Content                                      |
|---------------|----------------------------------------------|
| `picoclaw`    | System notes, docs, MEMORY.md, daily notes   |
| `workspace`   | Current project files (Go, Markdown, configs)|
| `kanban`      | Kanban task history and status exports       |
| `ide-monitor` | IDE monitor event logs and parsed data       |

> Run `qmd status` to see currently active collections and document counts.

## Which Operation to Use

| Operation | Use when                                          | Speed    | Requires daemon? |
|-----------|---------------------------------------------------|----------|-----------------|
| `search`  | You know specific keywords or exact phrases       | ~30ms    | No              |
| `vsearch` | Keywords aren't matching, need conceptual results | ~2s      | Yes (fallback)  |
| `query`   | Best result quality needed, some latency OK       | ~10s     | Yes (fallback)  |
| `get`     | You have a file path or #docid from search        | instant  | No              |
| `status`  | Check what's indexed and index health             | instant  | No              |

## Usage Examples

```
# Fast keyword search across all collections
qmd(operation="search", query="kanban task authentication", limit=5)

# Semantic search — finds related content even when vocabulary differs
qmd(operation="vsearch", query="deploy strategy for limited hardware", limit=5)

# Best quality — hybrid BM25 + vector + reranking (best used when daemon is running)
qmd(operation="query", query="memory pressure solutions", limit=5)

# Scoped search — only look in the picoclaw collection
qmd(operation="search", query="workspace path config", collection="picoclaw", limit=5)

# Retrieve full document by path (from search results)
qmd(operation="get", query="picoclaw/docs/ARCHITECTURE.md")

# Retrieve by short docid from search result (e.g. #a1b2c3)
qmd(operation="get", query="#a1b2c3")

# Check index status
qmd(operation="status")
```

## Standard Workflow

1. Always call `search` first with the most relevant keywords from the user's question.
2. If results are empty or irrelevant, try `vsearch` with a rephrased, more conceptual query.
3. If you need a full document, use `get` with the path or docid from the search result.
4. Only call the LLM with the retrieved snippet — do not send full indexed documents to the LLM.

## Starting the QMD Daemon (for hybrid search)

The daemon pre-loads ML models and keeps them warm between requests:

```bash
# Start daemon (models load once, ~30s cold start, then fast)
qmd mcp --http --daemon --port 8181

# Verify it's running
curl http://localhost:8181/health

# Check memory usage
free -h
```

## Setting Up Collections

```bash
# Index picoclaw workspace notes
qmd collection add ~/.picoclaw/workspace/memory --name picoclaw
qmd context add qmd://picoclaw "picoclaw agent memory, decisions, and notes"

# Index current project files
qmd collection add ~/picoclaw --name workspace --glob "**/*.{md,go,json,yaml}"
qmd context add qmd://workspace "picoclaw source code and configuration"

# Index kanban task history
qmd collection add ~/picoclaw/telegram-bots/logs --name kanban --glob "**/*.{md,json,log}"
qmd context add qmd://kanban "Telegram kanban bot task history and status"

# Generate embeddings (first time, and after adding new documents)
qmd embed
```
````
