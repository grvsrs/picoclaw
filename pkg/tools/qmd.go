package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// QMDTool gives agents access to the QMD hybrid search engine.
//
// Search modes:
//   - "auto"  (default): use MCP HTTP daemon if reachable, otherwise fall back
//     to the qmd CLI which uses BM25-only search.
//   - "mcp":  always use the HTTP daemon (fail if not running).
//   - "cli":  always use the qmd CLI; never attempts the daemon.
//
// Daemon mode keeps the 2 GB of local ML models warm across requests so
// repeated searches are fast.  Start with:
//
//	qmd mcp --http --daemon --port 8181
//
// Operations exposed to the LLM:
//
//	search   – fast BM25 keyword search (no daemon required)
//	vsearch  – semantic vector search (hybrid, daemon preferred)
//	query    – best quality: keyword + vector + LLM reranking (daemon required)
//	get      – retrieve one document by path or short docid (#abc123)
//	status   – show index collections and document counts
type QMDTool struct {
	mcpEndpoint string
	mode        string
	httpClient  *http.Client
}

// NewQMDTool creates a QMDTool.
//   - mcpEndpoint: QMD HTTP MCP URL (empty → "http://localhost:8181/mcp")
//   - mode:        "auto" | "mcp" | "cli"  (empty → "auto")
func NewQMDTool(mcpEndpoint, mode string) *QMDTool {
	if mcpEndpoint == "" {
		mcpEndpoint = "http://localhost:8181/mcp"
	}
	if mode == "" {
		mode = "auto"
	}
	return &QMDTool{
		mcpEndpoint: mcpEndpoint,
		mode:        mode,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (q *QMDTool) Name() string { return "qmd" }

func (q *QMDTool) Description() string {
	return `Search your personal knowledge base (notes, docs, kanban history, workspace files) using QMD — a local hybrid search engine.

Available operations:
  • search  — fast BM25 keyword search, always available, no daemon required
  • vsearch — semantic/conceptual search (finds related content even if keywords don't match)
  • query   — best quality: BM25 + vector + LLM reranking; requires the QMD daemon
  • get     — retrieve a full document by path or docid (#abc123 shown in search results)
  • status  — show indexed collections and document counts

Always search before answering questions about past decisions, kanban tasks, or system history.
Use 'search' for quick lookups; 'query' when you need the most accurate results.`
}

func (q *QMDTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type": "string",
				"enum": []string{"search", "vsearch", "query", "get", "status"},
				"description": "Operation to perform:\n" +
					"  search  = fast BM25 keyword (always available)\n" +
					"  vsearch = semantic vector search\n" +
					"  query   = hybrid full-quality search (requires daemon)\n" +
					"  get     = retrieve document by path or #docid\n" +
					"  status  = show index health and collections",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query text — or document path / #docid for 'get'",
			},
			"collection": map[string]interface{}{
				"type":        "string",
				"description": "Optional: restrict to a specific collection (e.g. 'picoclaw', 'workspace', 'kanban')",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum results to return (default: 5)",
				"default":     5,
			},
		},
		"required": []string{"operation"},
	}
}

func (q *QMDTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	operation, _ := args["operation"].(string)
	query, _ := args["query"].(string)
	collection, _ := args["collection"].(string)
	limit := 5
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	useMCP := q.mode == "mcp" || (q.mode == "auto" && q.isDaemonReachable())

	switch operation {
	case "search":
		if useMCP {
			return q.mcpToolCall(ctx, "search", mcpArgs(query, collection, limit))
		}
		return q.cliSearch(ctx, "search", query, collection, limit)

	case "vsearch":
		if useMCP {
			return q.mcpToolCall(ctx, "vector_search", mcpArgs(query, collection, limit))
		}
		// Fall back to BM25 with a note
		result, err := q.cliSearch(ctx, "search", query, collection, limit)
		if err != nil {
			return result, err
		}
		return "[Note: QMD daemon not running — showing BM25 keyword results instead of vector search]\n\n" + result, nil

	case "query":
		if useMCP {
			return q.mcpToolCall(ctx, "deep_search", mcpArgs(query, collection, limit))
		}
		result, err := q.cliSearch(ctx, "search", query, collection, limit)
		if err != nil {
			return result, err
		}
		return "[Note: QMD daemon not running — showing BM25 results. Start daemon for full hybrid search.]\n\n" + result, nil

	case "get":
		if query == "" {
			return "", fmt.Errorf("'query' must contain the document path or #docid to retrieve")
		}
		if useMCP {
			return q.mcpToolCall(ctx, "get", map[string]interface{}{"file": query})
		}
		return q.cliRun(ctx, []string{"get", query})

	case "status":
		if useMCP {
			return q.mcpToolCall(ctx, "status", map[string]interface{}{})
		}
		return q.cliRun(ctx, []string{"status"})

	default:
		return "", fmt.Errorf("unknown qmd operation %q; valid: search, vsearch, query, get, status", operation)
	}
}

// ---------------------------------------------------------------------------
// MCP HTTP client
// ---------------------------------------------------------------------------

// MCP JSON-RPC wire types
type mcpRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int                    `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// isDaemonReachable probes the /health endpoint with a short timeout.
func (q *QMDTool) isDaemonReachable() bool {
	healthURL := strings.Replace(q.mcpEndpoint, "/mcp", "/health", 1)
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return false
	}
	resp, err := q.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// mcpToolCall executes a tools/call via the MCP HTTP transport.
// It follows the MCP session protocol:
//  1. POST initialize → receive Mcp-Session-Id header
//  2. POST tools/call with that session ID
func (q *QMDTool) mcpToolCall(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error) {
	// Strip empty optional strings so QMD doesn't get confused
	for k, v := range arguments {
		if s, ok := v.(string); ok && s == "" {
			delete(arguments, k)
		}
	}

	sessionID, err := q.mcpInit(ctx)
	if err != nil {
		return "", fmt.Errorf("qmd daemon unreachable: %w", err)
	}

	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}
	raw, err := q.mcpPost(ctx, req, sessionID)
	if err != nil {
		return "", err
	}
	return extractMCPText(raw)
}

// mcpInit sends an MCP initialize request and returns the session ID.
func (q *QMDTool) mcpInit(ctx context.Context) (string, error) {
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "picoclaw", "version": "1.0"},
		},
	}
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", q.mcpEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := q.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // drain
	return resp.Header.Get("Mcp-Session-Id"), nil
}

// mcpPost sends a JSON-RPC request and returns the raw result field.
func (q *QMDTool) mcpPost(ctx context.Context, req mcpRequest, sessionID string) (json.RawMessage, error) {
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", q.mcpEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err := q.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("QMD MCP HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var mcpResp mcpResponse
	if err := json.Unmarshal(data, &mcpResp); err != nil {
		return nil, fmt.Errorf("invalid MCP response: %w\nraw: %.500s", err, data)
	}
	if mcpResp.Error != nil {
		return nil, fmt.Errorf("QMD MCP error %d: %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}
	return mcpResp.Result, nil
}

// extractMCPText pulls human-readable text out of a tools/call result.
func extractMCPText(raw json.RawMessage) (string, error) {
	var result struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Resource *struct {
				Name  string `json:"name"`
				Title string `json:"title"`
				Text  string `json:"text"`
			} `json:"resource,omitempty"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return string(raw), nil // return raw if unparseable
	}

	var parts []string
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		case "resource":
			if c.Resource != nil {
				header := c.Resource.Name
				if c.Resource.Title != "" && c.Resource.Title != c.Resource.Name {
					header = fmt.Sprintf("%s (%s)", c.Resource.Name, c.Resource.Title)
				}
				parts = append(parts, fmt.Sprintf("=== %s ===\n%s", header, c.Resource.Text))
			}
		}
	}
	if len(parts) == 0 {
		return "(no results)", nil
	}
	return strings.Join(parts, "\n\n"), nil
}

// ---------------------------------------------------------------------------
// CLI fallback helpers
// ---------------------------------------------------------------------------

// resolveQMDCmd returns the best available QMD CLI binary.
// Preference order:
//  1. qmd-run — our wrapper that hardcodes Node 22 path (most reliable)
//  2. qmd     — works if PATH already has Node >= 22
func resolveQMDCmd() string {
	// Try qmd-run first (our wrapper at /usr/local/bin/qmd-run)
	if path, err := exec.LookPath("qmd-run"); err == nil {
		return path
	}
	// Fall back to plain qmd (may work if user's shell sets up Node 22 in PATH)
	return "qmd"
}

func (q *QMDTool) cliSearch(ctx context.Context, mode, query, collection string, limit int) (string, error) {
	if query == "" {
		return "", fmt.Errorf("'query' is required for %s operation", mode)
	}
	args := []string{mode, query, "--json", "-n", fmt.Sprintf("%d", limit)}
	if collection != "" {
		args = append(args, "-c", collection)
	}
	return q.cliRun(ctx, args)
}

func (q *QMDTool) cliRun(ctx context.Context, args []string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, resolveQMDCmd(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Filter the node-llama-cpp build noise from stderr — it fires on every
		// invocation when no prebuilt binary matches the platform, but it's not a
		// real error (qmd falls back to CPU automatically).
		errMsg := filterLlamaStderr(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("qmd: %s", errMsg)
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return "(no results)", nil
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// filterLlamaStderr removes node-llama-cpp compilation noise from stderr.
// On every cold startup, node-llama-cpp tries to build native binaries and
// emits cmake/CUDA output even when it falls back successfully to CPU.  We only
// want to surface lines that are genuine qmd errors.
func filterLlamaStderr(raw string) string {
	noisy := []string{
		"[node-llama-cpp]",
		"CMake",
		"-- ",
		"Not searching",
		"QMD Warning:",
		"llama/localBuilds",
		"spawnCommand",
		"createError",
		"ChildProcess",
		"at Function.",
		"at Object.",
		"node:internal",
		"ERR! OMG",
	}
	var kept []string
	for _, line := range strings.Split(raw, "\n") {
		isNoise := false
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		for _, prefix := range noisy {
			if strings.Contains(line, prefix) {
				isNoise = true
				break
			}
		}
		if !isNoise {
			kept = append(kept, line)
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// mcpArgs builds a compact argument map for MCP search calls.
func mcpArgs(query, collection string, limit int) map[string]interface{} {
	m := map[string]interface{}{
		"query": query,
		"limit": limit,
	}
	if collection != "" {
		m["collection"] = collection
	}
	return m
}
