// Ops Monitor Bot Commands — handler skill for remote operations control
// Wires the ops-monitor template slash commands to live gateway API endpoints.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// OpsMonitorTool provides gateway API access for ops-monitor bot commands.
// Allows remote code execution via Telegram /run command with safe-list enforcement.
type OpsMonitorTool struct {
	gatewayURL string // e.g., "http://127.0.0.1:18790"
	apiKey     string
	httpClient *http.Client
}

// NewOpsMonitorTool creates a new ops monitor command handler.
func NewOpsMonitorTool(gatewayURL, apiKey string) *OpsMonitorTool {
	if gatewayURL == "" {
		gatewayURL = "http://127.0.0.1:18790"
	}
	return &OpsMonitorTool{
		gatewayURL: strings.TrimRight(gatewayURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// Registry implementation
func (t *OpsMonitorTool) Name() string                    { return "ops_monitor" }
func (t *OpsMonitorTool) Description() string {
	return "Remote operations control via gateway API (for ops-monitor bot)"
}
func (t *OpsMonitorTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type": "string",
				"enum": []string{"status", "bots", "tasks", "logs", "run"},
				"description": "The ops command to execute",
			},
			"args": map[string]interface{}{
				"type": "string",
				"description": "Command arguments (e.g., task status, log count, shell command)",
			},
		},
		"required": []string{"command"},
	}
}

// Execute handles all ops-monitor commands
func (t *OpsMonitorTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Convert map args to param map
	params := make(map[string]string)
	
	if cmd, ok := args["command"].(string); ok {
		params["cmd"] = cmd
	} else if cmd, ok := args["cmd"].(string); ok {
		params["cmd"] = cmd
	} else {
		return "", fmt.Errorf("missing 'command' argument (status|bots|tasks|logs|run)")
	}

	if cmdArgs, ok := args["args"].(string); ok {
		// Parse additional args from string
		parts := strings.Fields(cmdArgs)
		if len(parts) > 0 {
			if params["cmd"] == "tasks" {
				params["status"] = parts[0]
			} else if params["cmd"] == "logs" {
				params["n"] = parts[0]
			} else if params["cmd"] == "run" {
				params["cmd_args"] = cmdArgs
			}
		}
	}

	command := params["cmd"]
	if command == "" {
		return "", fmt.Errorf("missing command (status|bots|tasks|logs|run)")
	}

	logger.InfoCF("ops-monitor", "Executing command", map[string]interface{}{
		"command": command,
		"params":  params,
	})

	switch command {
	case "status":
		result, err := t.cmdStatus(ctx)
		return fmt.Sprintf("%v", result), err
	case "bots":
		result, err := t.cmdBots(ctx)
		return fmt.Sprintf("%v", result), err
	case "tasks":
		result, err := t.cmdTasks(ctx, params)
		return fmt.Sprintf("%v", result), err
	case "logs":
		result, err := t.cmdLogs(ctx, params)
		return fmt.Sprintf("%v", result), err
	case "run":
		result, err := t.cmdRun(ctx, params)
		return fmt.Sprintf("%v", result), err
	default:
		return "", fmt.Errorf("unknown command: %s", command)
	}
}

// Remaining helper functions below

// Helper: call gateway API
func (t *OpsMonitorTool) callAPI(ctx context.Context, method, path string, body interface{}) (map[string]interface{}, error) {
	url := t.gatewayURL + path

	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %v", resp.StatusCode, result)
	}

	return result, nil
}

// /status — system health
func (t *OpsMonitorTool) cmdStatus(ctx context.Context) (interface{}, error) {
	data, err := t.callAPI(ctx, "GET", "/api/system/status", nil)
	if err != nil {
		return nil, err
	}

	// Format as readable output
	out := "🔍 **System Status**\n\n"
	if uptime, ok := data["uptime"].(string); ok {
		out += fmt.Sprintf("⏱️ Uptime: %s\n", uptime)
	}
	if memory, ok := data["memory"].(map[string]interface{}); ok {
		if alloc, ok := memory["alloc"].(string); ok {
			out += fmt.Sprintf("💾 Memory: %s\n", alloc)
		}
	}

	return out, nil
}

// /bots — list running bots
func (t *OpsMonitorTool) cmdBots(ctx context.Context) (interface{}, error) {
	data, err := t.callAPI(ctx, "GET", "/api/bots", nil)
	if err != nil {
		return nil, err
	}

	out := "🤖 **Active Bots**\n\n"
	if bots, ok := data["bots"].([]interface{}); ok {
		if len(bots) == 0 {
			out += "_No bots running_"
		} else {
			for _, bot := range bots {
				if b, ok := bot.(map[string]interface{}); ok {
					id := b["id"].(string)
					running := b["running"].(bool)
					status := "✅ Running"
					if !running {
						status = "⛔ Stopped"
					}
					out += fmt.Sprintf("• %s — %s\n", id, status)
				}
			}
		}
	}

	return out, nil
}

// /tasks [status] — list kanban tasks
func (t *OpsMonitorTool) cmdTasks(ctx context.Context, params map[string]string) (interface{}, error) {
	path := "/api/tasks"
	if status, ok := params["status"]; ok && status != "" {
		path += fmt.Sprintf("?status=%s", status)
	}

	data, err := t.callAPI(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	out := "📋 **Tasks**\n\n"
	if tasks, ok := data["tasks"].([]interface{}); ok {
		if len(tasks) == 0 {
			out += "_No tasks_"
		} else {
			for i, task := range tasks {
				if t, ok := task.(map[string]interface{}); ok {
					title := t["title"].(string)
					status := t["status"].(string)
					emoji := "📝"
					if status == "done" {
						emoji = "✅"
					}
					out += fmt.Sprintf("%d. %s %s (%s)\n", i+1, emoji, title, status)
				}
			}
		}
	}

	return out, nil
}

// /logs [n] — tail logs (default 20 lines, max 100)
func (t *OpsMonitorTool) cmdLogs(ctx context.Context, params map[string]string) (interface{}, error) {
	lines := "20"
	if n, ok := params["n"]; ok && n != "" {
		lines = n
	}

	data, err := t.callAPI(ctx, "GET", fmt.Sprintf("/api/cron/status?limit=%s", lines), nil)
	if err != nil {
		return nil, err
	}

	out := fmt.Sprintf("📜 **Last %s log lines**\n\n", lines)
	out += "```\n"
	if logs, ok := data["logs"].(string); ok {
		out += logs
	}
	out += "\n```"

	return out, nil
}

// /run <cmd> — safe shell execution (restricted safe-list)
func (t *OpsMonitorTool) cmdRun(ctx context.Context, params map[string]string) (interface{}, error) {
	cmd := params["cmd"]
	if cmd == "" {
		return nil, fmt.Errorf("missing command argument")
	}

	// Safe-list enforcement
	allowedCmds := map[string]bool{
		"git status":   true,
		"go test":      true,
		"make":         true,
		"ls":           true,
		"df":           true,
		"free":         true,
		"uptime":       true,
		"ps aux":       true,
		"kubectl get": true,
		"docker ps":    true,
	}

	// Check if command is in safe-list
	allowed := false
	for safeCmd := range allowedCmds {
		if strings.HasPrefix(cmd, safeCmd) {
			allowed = true
			break
		}
	}

	if !allowed {
		return nil, fmt.Errorf("command not in safe-list: %s\n\nAllowed: %v", cmd, allowedCmds)
	}

	// Execute via exec tool call or direct HTTP
	reqBody := map[string]interface{}{
		"command": cmd,
	}

	data, err := t.callAPI(ctx, "POST", "/api/tools/exec", reqBody)
	if err != nil {
		return nil, err
	}

	if output, ok := data["output"].(string); ok {
		return fmt.Sprintf("```\n%s\n```", output), nil
	}

	return data, nil
}
