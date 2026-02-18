// WorkflowEvent ingestion handler — receives events from the ide-monitor
// Python daemon and routes them through the existing picoclaw infrastructure:
//   - WSHub for real-time dashboard updates
//   - Kanban integration for task card creation/updates
//   - MessageBus for system event fan-out
//
// This file adds NO new types that duplicate existing ones. It uses:
//   - WSHub.Broadcast() from ws.go
//   - KanbanIntegration from integration/kanban
//   - MessageBus.PublishSystem() from bus
package api

import (
	"encoding/json"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/integration"
	kanban "github.com/sipeed/picoclaw/pkg/integration/kanban"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// WorkflowEvent mirrors the Python WorkflowEvent v1 spec from normalizer.py.
// Fields are pointers so absent JSON fields stay nil (lean payloads).
type WorkflowEvent struct {
	// Identity
	ID               string  `json:"id"`
	SpecVersion      string  `json:"spec_version"`
	Source           string  `json:"source"`
	EventType        string  `json:"event_type"`
	EventVersion     int     `json:"event_version,omitempty"`
	Timestamp        string  `json:"timestamp"`
	Hostname         *string `json:"hostname,omitempty"`
	WorkspaceID      *string `json:"workspace_id,omitempty"`

	// Confidence (always computed)
	Confidence       float64 `json:"confidence"`

	// Task Context
	TaskID           *string `json:"task_id,omitempty"`
	TaskTitle        *string `json:"task_title,omitempty"`
	TaskStatus       *string `json:"task_status,omitempty"`
	Iteration        *int    `json:"iteration,omitempty"`
	ExternalRef      *string `json:"external_ref,omitempty"`

	// Artifact
	ArtifactType     *string `json:"artifact_type,omitempty"`
	ArtifactPath     *string `json:"artifact_path,omitempty"`
	Summary          *string `json:"summary,omitempty"`

	// Token Data (Copilot)
	TokensPrompt     *int    `json:"tokens_prompt,omitempty"`
	TokensCompletion *int    `json:"tokens_completion,omitempty"`
	Model            *string `json:"model,omitempty"`

	// Burst Context
	BurstID           *string  `json:"burst_id,omitempty"`
	BurstTokenTotal   *int     `json:"burst_token_total,omitempty"`
	BurstDurationSecs *float64 `json:"burst_duration_secs,omitempty"`
	BurstEntryCount   *int     `json:"burst_entry_count,omitempty"`

	// File Activity
	FilesChanged     []WorkflowFile `json:"files_changed,omitempty"`
	WorkspaceRoot    *string        `json:"workspace_root,omitempty"`

	// Git Correlation
	GitCommitSHA     *string `json:"git_commit_sha,omitempty"`
	GitBranch        *string `json:"git_branch,omitempty"`

	// Correlation Metadata
	CorrelatedEvents []string `json:"correlated_events,omitempty"`
	CorrelationType  *string  `json:"correlation_type,omitempty"`

	// Extension Point
	Raw              map[string]interface{} `json:"raw,omitempty"`
}

// WorkflowFile represents a file change inside a WorkflowEvent.
type WorkflowFile struct {
	Path       string `json:"path"`
	ChangeType string `json:"change_type"`
	SizeBytes  *int64 `json:"size_bytes,omitempty"`
}

// handleWorkflowEvent handles POST /api/events from the ide-monitor.
func (s *Server) handleWorkflowEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var ev WorkflowEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Basic validation
	if ev.ID == "" || ev.EventType == "" || ev.Source == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id, event_type, source required"})
		return
	}

	logger.InfoCF("workflow", "Received event", map[string]interface{}{
		"id":         ev.ID,
		"event_type": ev.EventType,
		"source":     ev.Source,
	})

	// Route asynchronously — don't block the HTTP response
	go s.routeWorkflowEvent(ev)

	writeJSON(w, http.StatusAccepted, map[string]interface{}{"ok": true})
}

// routeWorkflowEvent fans out a workflow event to all downstream systems.
func (s *Server) routeWorkflowEvent(ev WorkflowEvent) {
	// 1. Broadcast to dashboard via existing WSHub
	s.wsHub.Broadcast("workflow."+ev.EventType, ev)

	// 2. Publish to existing message bus for EventBridge fan-out
	if s.messageBus != nil {
		s.messageBus.PublishSystem(bus.SystemEvent{
			Type:   ev.EventType,
			Source: ev.Source,
			Data:   ev,
		})
	}

	// 3. Route task lifecycle events to kanban
	// Rule: NEVER auto-create Kanban cards from Copilot alone.
	// Only Antigravity (intent) and Git (execution) touch Kanban.
	switch ev.EventType {
	case "antigravity.task.created":
		s.upsertWorkflowKanbanCard(ev, kanban.StateInbox)
	case "antigravity.task.plan_ready":
		s.upsertWorkflowKanbanCard(ev, kanban.StatePlanned)
	case "antigravity.task.iterated":
		s.upsertWorkflowKanbanCard(ev, kanban.StateRunning)
	case "antigravity.task.completed":
		s.upsertWorkflowKanbanCard(ev, kanban.StateDone)
	case "antigravity.task.failed":
		s.upsertWorkflowKanbanCard(ev, kanban.StateBlocked)
	case "git.commit", "git.commit_linked_to_task":
		s.logWorkflowGitCommit(ev)
	}
}

// upsertWorkflowKanbanCard creates or updates a kanban card from a workflow event.
// Uses ExternalRef (workspace_id:task_id) as the stable identity key.
// State transitions use TransitionTask for proper audit trail.
func (s *Server) upsertWorkflowKanbanCard(ev WorkflowEvent, state kanban.TaskState) {
	reg := integration.GetRegistry()
	if reg == nil {
		return
	}
	ki, found := reg.Get("kanban")
	if !found {
		return
	}
	k, ok := ki.(*kanban.KanbanIntegration)
	if !ok {
		return
	}

	title := "Untitled Task"
	if ev.TaskTitle != nil {
		title = *ev.TaskTitle
	}

	description := ""
	if ev.Summary != nil {
		description = *ev.Summary
	}

	// Prefer external_ref from event (workspace_id:task_id)
	// Fall back to task_id for backward compatibility
	externalRef := ""
	if ev.ExternalRef != nil {
		externalRef = *ev.ExternalRef
	} else if ev.TaskID != nil {
		externalRef = *ev.TaskID
	}

	// Try to find existing task by external ref (indexed lookup)
	if externalRef != "" {
		existing, err := k.GetTaskByExternalRef(externalRef)
		if err != nil {
			logger.ErrorCF("workflow", "Failed to lookup kanban card by external_ref", map[string]interface{}{
				"external_ref": externalRef,
				"error":        err.Error(),
			})
		}
		if existing != nil {
			// Update description if changed
			if ev.Summary != nil {
				updates := map[string]interface{}{
					"description": description,
				}
				_ = k.UpdateTask(existing.ID, updates)
			}
			// Transition state using proper state machine
			if existing.State != state {
				if err := k.TransitionTask(existing.ID, state, "workflow:"+ev.EventType, "ide-monitor"); err != nil {
					logger.ErrorCF("workflow", "Failed to transition kanban card", map[string]interface{}{
						"task_id":    existing.ID,
						"from_state": string(existing.State),
						"to_state":   string(state),
						"error":      err.Error(),
					})
				} else {
					logger.InfoCF("workflow", "Transitioned kanban card", map[string]interface{}{
						"task_id":      existing.ID,
						"external_ref": externalRef,
						"new_state":    string(state),
					})
				}
			}
			return
		}
	}

	// Create new task
	task := &kanban.Task{
		Title:       title,
		Description: description,
		State:       state,
		Category:    kanban.CategoryCode,
		Source:      kanban.TaskSource("ide-monitor"),
		Priority:    "normal",
		ExternalRef: externalRef,
	}

	if err := k.CreateTask(task); err != nil {
		logger.ErrorCF("workflow", "Failed to create kanban card", map[string]interface{}{
			"title": title,
			"error": err.Error(),
		})
	} else {
		logger.InfoCF("workflow", "Created kanban card", map[string]interface{}{
			"task_id":      task.ID,
			"title":        title,
			"state":        string(state),
			"external_ref": externalRef,
		})
	}
}

// logWorkflowGitCommit logs a git commit event against its correlated task.
func (s *Server) logWorkflowGitCommit(ev WorkflowEvent) {
	reg := integration.GetRegistry()
	if reg == nil {
		return
	}
	ki, found := reg.Get("kanban")
	if !found {
		return
	}
	k, ok := ki.(*kanban.KanbanIntegration)
	if !ok {
		return
	}

	sha := ""
	if ev.GitCommitSHA != nil {
		sha = *ev.GitCommitSHA
	}
	summary := ""
	if ev.Summary != nil {
		summary = *ev.Summary
	}

	// Find the task by external_ref (preferred) or task_id (fallback)
	ref := ""
	if ev.ExternalRef != nil {
		ref = *ev.ExternalRef
	} else if ev.TaskID != nil {
		ref = *ev.TaskID
	}

	if ref == "" {
		return
	}

	existing, err := k.GetTaskByExternalRef(ref)
	if err != nil || existing == nil {
		return
	}

	_ = k.LogEvent(existing.ID, "git", "commit", sha+": "+summary)
}
