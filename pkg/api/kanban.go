// Kanban REST API — native Go handlers backed by KanbanIntegration.
// This is the authoritative task API. The Python proxy remains for
// any Python-specific endpoints not yet migrated.
//
// Routes:
//   GET    /api/tasks              — list tasks (filters: state, category, source, project)
//   POST   /api/tasks              — create task
//   GET    /api/tasks/{id}         — get task
//   PUT    /api/tasks/{id}         — update task fields
//   DELETE /api/tasks/{id}         — delete task
//   POST   /api/tasks/{id}/transition — state machine transition
//   POST   /api/tasks/{id}/claim   — claim task (agent ownership)
//   POST   /api/tasks/{id}/release — release claim
//   POST   /api/tasks/{id}/complete — mark done, clear ownership
//   GET    /api/tasks/stats        — board stats
//   GET    /api/tasks/categories   — category stats
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/pkg/integration"
	"github.com/sipeed/picoclaw/pkg/integration/kanban"
	"github.com/sipeed/picoclaw/pkg/logger"
	"time"
)

// getKanban retrieves the running kanban integration from the global registry.
func (s *Server) getKanban() *kanban.KanbanIntegration {
	reg := integration.GetRegistry()
	integ, ok := reg.Get("kanban")
	if !ok {
		return nil
	}
	ki, ok := integ.(*kanban.KanbanIntegration)
	if !ok {
		return nil
	}
	return ki
}

// handleTasks dispatches GET (list) and POST (create) on /api/tasks.
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	kb := s.getKanban()
	if kb == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kanban not available"})
		return
	}

	switch r.Method {
	case "GET":
		s.handleListTasks(w, r, kb)
	case "POST":
		s.handleCreateTask(w, r, kb)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// handleTaskByID dispatches on /api/tasks/{id}[/action].
func (s *Server) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	kb := s.getKanban()
	if kb == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kanban not available"})
		return
	}

	// Parse: /api/tasks/{id} or /api/tasks/{id}/action
	path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	parts := strings.SplitN(path, "/", 2)
	taskID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	if taskID == "" || taskID == "stats" {
		s.handleTaskStats(w, r, kb)
		return
	}
	if taskID == "categories" {
		s.handleCategoryStats(w, r, kb)
		return
	}

	switch action {
	case "":
		switch r.Method {
		case "GET":
			s.handleGetTask(w, r, kb, taskID)
		case "PUT":
			s.handleUpdateTask(w, r, kb, taskID)
		case "DELETE":
			s.handleDeleteTask(w, r, kb, taskID)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	case "transition":
		s.handleTransitionTask(w, r, kb, taskID)
	case "claim":
		s.handleClaimTask(w, r, kb, taskID)
	case "release":
		s.handleReleaseTask(w, r, kb, taskID)
	case "complete":
		s.handleCompleteTask(w, r, kb, taskID)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown action"})
	}
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request, kb *kanban.KanbanIntegration) {
	q := r.URL.Query()
	filters := kanban.TaskFilters{
		State:       kanban.TaskState(q.Get("state")),
		Category:    kanban.TaskCategory(q.Get("category")),
		Source:      kanban.TaskSource(q.Get("source")),
		Project:     q.Get("project"),
		ExcludeDone: q.Get("exclude_done") == "true",
	}

	tasks, err := kb.ListTasks(filters)
	if err != nil {
		logger.ErrorCF("api", "List tasks failed", map[string]interface{}{"error": err.Error()})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []*kanban.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request, kb *kanban.KanbanIntegration) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Source      string `json:"source"`
		Priority    string `json:"priority"`
		Project     string `json:"project"`
		Assignee    string `json:"assignee"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title required"})
		return
	}

	task := &kanban.Task{
		Title:       req.Title,
		Description: req.Description,
		Category:    kanban.TaskCategory(req.Category),
		Source:      kanban.TaskSource(req.Source),
		Priority:    req.Priority,
		Project:     req.Project,
		Assignee:    req.Assignee,
	}

	if task.Source == "" {
		task.Source = kanban.SourceAPI
	}

	if err := kb.CreateTask(task); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request, kb *kanban.KanbanIntegration, id string) {
	task, err := kb.GetTask(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request, kb *kanban.KanbanIntegration, id string) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// If "status" is provided, use it as a state transition instead of raw update
	if newStatus, ok := updates["status"]; ok {
		delete(updates, "status")
		if statusStr, ok := newStatus.(string); ok {
			if err := kb.TransitionTask(id, kanban.TaskState(statusStr), "dashboard update", "api"); err != nil {
				// If transition fails, try as a field update fallback
				logger.WarnCF("api", "Transition failed, trying field update", map[string]interface{}{"error": err.Error()})
			}
		}
	}

	if len(updates) > 0 {
		if err := kb.UpdateTask(id, updates); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	// Return updated task
	task, err := kb.GetTask(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request, kb *kanban.KanbanIntegration, id string) {
	if err := kb.DeleteTask(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

func (s *Server) handleTransitionTask(w http.ResponseWriter, r *http.Request, kb *kanban.KanbanIntegration, id string) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req struct {
		State    string `json:"state"`
		Reason   string `json:"reason"`
		Executor string `json:"executor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.State == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "state required"})
		return
	}
	if req.Executor == "" {
		req.Executor = "api"
	}

	if err := kb.TransitionTask(id, kanban.TaskState(req.State), req.Reason, req.Executor); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	task, _ := kb.GetTask(id)
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleClaimTask(w http.ResponseWriter, r *http.Request, kb *kanban.KanbanIntegration, id string) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req struct {
		AgentID  string `json:"agent_id"`
		LeaseSec int    `json:"lease_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.AgentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "agent_id required"})
		return
	}

	lease := 5 * time.Minute
	if req.LeaseSec > 0 {
		lease = time.Duration(req.LeaseSec) * time.Second
	}

	if err := kb.ClaimTask(id, req.AgentID, lease); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	task, _ := kb.GetTask(id)
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleReleaseTask(w http.ResponseWriter, r *http.Request, kb *kanban.KanbanIntegration, id string) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req struct {
		AgentID string `json:"agent_id"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := kb.ReleaseTask(id, req.AgentID, req.Reason); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "released"})
}

func (s *Server) handleCompleteTask(w http.ResponseWriter, r *http.Request, kb *kanban.KanbanIntegration, id string) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req struct {
		AgentID string `json:"agent_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := kb.CompleteTask(id, req.AgentID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "completed", "id": id})
}

func (s *Server) handleTaskStats(w http.ResponseWriter, r *http.Request, kb *kanban.KanbanIntegration) {
	stats, err := kb.GetBoardStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleCategoryStats(w http.ResponseWriter, r *http.Request, kb *kanban.KanbanIntegration) {
	stats, err := kb.GetCategoryStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
