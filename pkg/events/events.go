// Package events defines the typed event contracts for the entire picoclaw system.
// Every event flowing through WebSocket, message bus, or orchestrator MUST
// use one of these types. No ad-hoc map[string]interface{} events.
package events

import "time"

// --- Event Envelope ---

// Event is the universal envelope for all system events.
type Event struct {
	// Type identifies the event (e.g., "bot.started", "task.created")
	Type string `json:"type"`

	// Source identifies who emitted the event
	Source string `json:"source"`

	// Timestamp is when the event was emitted
	Timestamp time.Time `json:"timestamp"`

	// Data is the typed payload
	Data interface{} `json:"data"`
}

// New creates a timestamped event.
func New(eventType, source string, data interface{}) Event {
	return Event{
		Type:      eventType,
		Source:    source,
		Timestamp: time.Now(),
		Data:      data,
	}
}

// --- Event Type Constants ---

const (
	// Bot lifecycle events
	BotCreated      = "bot.created"
	BotUpdated      = "bot.updated"
	BotDeleted      = "bot.deleted"
	BotStarted      = "bot.started"
	BotStopped      = "bot.stopped"
	BotError        = "bot.error"
	BotConfigChanged = "bot.config_changed"

	// Message flow events
	MessageInbound  = "message.inbound"
	MessageOutbound = "message.outbound"
	MessageDropped  = "message.dropped"

	// Agent events
	AgentThinking     = "agent.thinking"
	AgentResponded    = "agent.responded"
	AgentError        = "agent.error"
	AgentToolUse      = "agent.tool_use"
	AgentSpawned      = "agent.spawned"
	AgentCompleted    = "agent.completed"

	// Task / Kanban events
	TaskCreated     = "task.created"
	TaskUpdated     = "task.updated"
	TaskAssigned    = "task.assigned"
	TaskClaimed     = "task.claimed"
	TaskCompleted   = "task.completed"
	TaskFailed      = "task.failed"
	TaskRetry       = "task.retry"
	TaskEscalated   = "task.escalated"

	// Orchestration events
	OrchAgentRegistered   = "orch.agent_registered"
	OrchAgentUnregistered = "orch.agent_unregistered"
	OrchTaskRouted        = "orch.task_routed"
	OrchLeaseExpired      = "orch.lease_expired"

	// System events
	SystemStarted  = "system.started"
	SystemStopping = "system.stopping"
	SystemHealth   = "system.health"

	// Coding pipeline events
	DiffCreated    = "diff.created"
	DiffValidated  = "diff.validated"
	DiffApplied    = "diff.applied"
	DiffRolledBack = "diff.rolled_back"
	DiffVerified   = "diff.verified"

	// Workflow / IDE monitor events (Antigravity + Copilot)
	WorkflowAntigravityTaskCreated   = "antigravity.task.created"
	WorkflowAntigravityTaskUpdated   = "antigravity.task.updated"
	WorkflowAntigravityTaskProgress  = "antigravity.task.progress"
	WorkflowAntigravityTaskPlanReady = "antigravity.task.plan_ready"
	WorkflowAntigravityTaskIterated  = "antigravity.task.iterated"
	WorkflowAntigravityTaskCompleted = "antigravity.task.completed"
	WorkflowAntigravityTaskFailed    = "antigravity.task.failed"
	WorkflowAntigravityTaskWalkthrough = "antigravity.task.walkthrough_added"
	WorkflowAntigravitySkillUpdated  = "antigravity.skill.updated"
	WorkflowAntigravityArtifactUnknown = "antigravity.artifact.unknown"

	WorkflowCopilotPrompt            = "copilot.prompt"
	WorkflowCopilotCompletion        = "copilot.completion"
	WorkflowCopilotError             = "copilot.error"
	WorkflowCopilotBurstStart        = "copilot.burst_start"
	WorkflowCopilotBurstEnd          = "copilot.burst_end"

	WorkflowGitCommit                = "git.commit"
	WorkflowGitCommitLinked          = "git.commit_linked_to_task"

	WorkflowFilesystemBurst          = "filesystem.batch_modified"

	WorkflowTaskInferred             = "workflow.task_inferred"
	WorkflowActivityClustered        = "workflow.activity_clustered"
	WorkflowConflictDetected         = "workflow.conflict_detected"
)

// --- Typed Payloads ---

// BotEventData is the payload for bot lifecycle events.
type BotEventData struct {
	BotID    string `json:"bot_id"`
	BotType  string `json:"bot_type"`
	BotName  string `json:"bot_name,omitempty"`
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
}

// MessageEventData is the payload for message flow events.
type MessageEventData struct {
	MessageID string `json:"message_id,omitempty"`
	Channel   string `json:"channel"`
	From      string `json:"from,omitempty"`
	Preview   string `json:"preview"` // truncated content
	Timestamp time.Time `json:"timestamp"`
}

// AgentEventData is the payload for agent lifecycle events.
type AgentEventData struct {
	AgentID    string `json:"agent_id"`
	SessionID  string `json:"session_id,omitempty"`
	Action     string `json:"action,omitempty"` // for tool_use events
	ToolName   string `json:"tool_name,omitempty"`
	Duration   int64  `json:"duration_ms,omitempty"`
	TokensUsed int    `json:"tokens_used,omitempty"`
}

// TaskEventData is the payload for kanban/task events.
type TaskEventData struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title,omitempty"`
	Status      string `json:"status,omitempty"`
	Category    string `json:"category,omitempty"`
	Project     string `json:"project,omitempty"`
	AssignedTo  string `json:"assigned_to,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	Source      string `json:"source,omitempty"`
}

// OrchEventData is the payload for orchestration events.
type OrchEventData struct {
	AgentID   string `json:"agent_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Category  string `json:"category,omitempty"`
}

// DiffEventData is the payload for coding pipeline diff events.
type DiffEventData struct {
	DiffID       string `json:"diff_id"`
	TaskID       string `json:"task_id"`
	AgentID      string `json:"agent_id,omitempty"`
	FilesChanged int    `json:"files_changed,omitempty"`
	Success      bool   `json:"success"`
	Error        string `json:"error,omitempty"`
	Summary      string `json:"summary,omitempty"`
}

// SystemEventData is the payload for system health events.
type SystemEventData struct {
	Uptime       int64  `json:"uptime_seconds,omitempty"`
	ActiveBots   int    `json:"active_bots,omitempty"`
	ActiveAgents int    `json:"active_agents,omitempty"`
	PendingTasks int    `json:"pending_tasks,omitempty"`
	Message      string `json:"message,omitempty"`
}
