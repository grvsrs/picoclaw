// VSCode extension API — endpoints consumed by the PicoClaw VSCode extension.
// These are thin wrappers that map extension commands to backend capabilities.
//
// Routes:
//   GET    /api/vscode/status      — extension status bar data
//   POST   /api/vscode/todo        — send TODO from editor to kanban
//   POST   /api/vscode/ask         — ask coding bot a question
//   POST   /api/vscode/diff/apply  — apply a structured diff from extension
//   POST   /api/vscode/diff/preview — validate diff without applying
//   GET    /api/vscode/tasks       — get assigned/available tasks for coding
//   POST   /api/vscode/tasks/{id}/claim — claim a task from the extension
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/codex"
	"github.com/sipeed/picoclaw/pkg/integration/kanban"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// handleVSCode dispatches /api/vscode/* requests.
func (s *Server) handleVSCode(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/vscode")
	if path == "" || path == "/" {
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "picoclaw-vscode-api",
			"version": "0.1.0",
		})
		return
	}

	switch {
	case path == "/status":
		s.handleVSCodeStatus(w, r)
	case path == "/todo":
		s.handleVSCodeTodo(w, r)
	case path == "/ask":
		s.handleVSCodeAsk(w, r)
	case path == "/diff/apply":
		s.handleVSCodeDiffApply(w, r)
	case path == "/diff/preview":
		s.handleVSCodeDiffPreview(w, r)
	case path == "/tasks":
		s.handleVSCodeTasks(w, r)
	case strings.HasPrefix(path, "/tasks/") && strings.HasSuffix(path, "/claim"):
		taskID := strings.TrimSuffix(strings.TrimPrefix(path, "/tasks/"), "/claim")
		s.handleVSCodeClaimTask(w, r, taskID)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown vscode endpoint"})
	}
}

// handleVSCodeStatus returns status bar data for the extension.
// Response: { tasks_todo, tasks_in_progress, agent_running, model }
func (s *Server) handleVSCodeStatus(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{
		"agent_running": false,
		"model":         "",
		"tasks_todo":    0,
		"tasks_progress": 0,
		"tasks_total":   0,
		"workspace":     "",
	}

	if s.agentLoop != nil {
		result["agent_running"] = s.agentLoop.IsRunning()
		result["model"] = s.agentLoop.GetModel()
		result["workspace"] = s.agentLoop.GetWorkspace()
	}

	if kb := s.getKanban(); kb != nil {
		if stats, err := kb.GetBoardStats(); err == nil {
			result["tasks_todo"] = stats["inbox"] + stats["planned"]
			result["tasks_progress"] = stats["running"]
			result["tasks_total"] = stats["total"]
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// handleVSCodeTodo creates a task from a TODO comment or selection in the editor.
func (s *Server) handleVSCodeTodo(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		File        string `json:"file"`     // source file path
		Line        int    `json:"line"`     // line number
		Category    string `json:"category"`
		Priority    string `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title required"})
		return
	}

	kb := s.getKanban()
	if kb == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kanban not available"})
		return
	}

	// Build description with file context
	desc := req.Description
	if req.File != "" {
		if desc != "" {
			desc += "\n\n"
		}
		desc += "Source: " + req.File
		if req.Line > 0 {
			desc += fmt.Sprintf(" (line %d)", req.Line)
		}
	}

	task := &kanban.Task{
		Title:       req.Title,
		Description: desc,
		Source:      kanban.SourceVSCode,
		Category:    kanban.TaskCategory(req.Category),
		Priority:    req.Priority,
	}

	if task.Priority == "" {
		task.Priority = "normal"
	}

	if err := kb.CreateTask(task); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

// handleVSCodeAsk sends a question to the coding agent and returns the response.
func (s *Server) handleVSCodeAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req struct {
		Question string `json:"question"`
		Context  string `json:"context"` // selected code or file content
		File     string `json:"file"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question required"})
		return
	}

	if s.agentLoop == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "agent not available"})
		return
	}

	// Build prompt with context
	prompt := req.Question
	if req.Context != "" {
		prompt = "Context:\n```\n" + req.Context + "\n```\n\n" + req.Question
	}
	if req.File != "" {
		prompt = "File: " + req.File + "\n" + prompt
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	response, err := s.agentLoop.ProcessDirectWithChannel(ctx, prompt, "vscode:extension", "vscode", "extension")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"response": response,
	})
}

// handleVSCodeDiffPreview validates a structured diff without applying it.
func (s *Server) handleVSCodeDiffPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req struct {
		Diff      string `json:"diff"`      // raw JSON diff from agent
		Workspace string `json:"workspace"` // workspace root for precondition check
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	diff, err := codex.ParseDiff(req.Diff)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"valid":  false,
			"error":  err.Error(),
			"stage":  "parse",
		})
		return
	}

	if err := diff.Validate(); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"valid":   false,
			"error":   err.Error(),
			"stage":   "validate",
			"diff_id": diff.ID,
		})
		return
	}

	// Check preconditions if workspace provided
	workspace := req.Workspace
	if workspace == "" && s.config != nil {
		workspace = s.config.WorkspacePath()
	}

	if workspace != "" {
		if err := diff.CheckPreconditions(workspace); err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"valid":   false,
				"error":   err.Error(),
				"stage":   "preconditions",
				"diff_id": diff.ID,
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":        true,
		"diff_id":      diff.ID,
		"task_id":      diff.TaskID,
		"changes":      len(diff.Changes),
		"has_verify":   diff.Verify != nil,
		"summary":      diff.Summary,
	})
}

// handleVSCodeDiffApply applies a validated structured diff to the workspace.
func (s *Server) handleVSCodeDiffApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req struct {
		Diff      string `json:"diff"`
		Workspace string `json:"workspace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	diff, err := codex.ParseDiff(req.Diff)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := diff.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	workspace := req.Workspace
	if workspace == "" && s.config != nil {
		workspace = s.config.WorkspacePath()
	}

	if workspace == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workspace path required"})
		return
	}

	// Check preconditions
	if err := diff.CheckPreconditions(workspace); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": err.Error(),
			"stage": "preconditions",
		})
		return
	}

	// Apply
	result, err := diff.Apply(workspace)
	if err != nil {
		logger.ErrorCF("vscode", "Diff apply failed", map[string]interface{}{
			"diff_id": diff.ID,
			"error":   err.Error(),
		})
	}

	// Publish event
	if s.messageBus != nil {
		eventType := "diff.applied"
		if !result.Success {
			eventType = "diff.rolled_back"
		}
		s.messageBus.PublishSystem(bus.SystemEvent{
			Type:   eventType,
			Source: "vscode",
			Data: map[string]interface{}{
				"diff_id":       result.DiffID,
				"task_id":       result.TaskID,
				"success":       result.Success,
				"files_changed": result.FilesChanged,
				"error":         result.Error,
			},
		})
	}

	// Update kanban task if we have one
	if result.Success && diff.TaskID != "" {
		if kb := s.getKanban(); kb != nil {
			kb.UpdateTask(diff.TaskID, map[string]interface{}{
				"last_error": "",
			})
			kb.LogEvent(diff.TaskID, "vscode", "diff.applied", diff.Summary)
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// handleVSCodeTasks returns tasks suitable for coding bots.
func (s *Server) handleVSCodeTasks(w http.ResponseWriter, r *http.Request) {
	kb := s.getKanban()
	if kb == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kanban not available"})
		return
	}

	// Return code-related tasks that are claimable
	tasks, err := kb.ListTasks(kanban.TaskFilters{
		ExcludeDone: true,
		Limit:       50,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Filter to coding-relevant categories
	codeTasks := []*kanban.Task{}
	for _, t := range tasks {
		if t.Category == kanban.CategoryCode ||
			t.Category == kanban.CategoryBug ||
			t.Category == kanban.CategoryFeature ||
			t.Category == kanban.CategoryInfra {
			codeTasks = append(codeTasks, t)
		}
	}

	writeJSON(w, http.StatusOK, codeTasks)
}

// handleVSCodeClaimTask claims a task from the extension for the local coding agent.
func (s *Server) handleVSCodeClaimTask(w http.ResponseWriter, r *http.Request, taskID string) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	kb := s.getKanban()
	if kb == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kanban not available"})
		return
	}

	if err := kb.ClaimTask(taskID, "vscode-agent", 10*time.Minute); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	task, _ := kb.GetTask(taskID)
	writeJSON(w, http.StatusOK, task)
}
