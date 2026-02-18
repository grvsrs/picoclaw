// Package agent defines the Agent bounded context.
// An Agent is an aggregate root representing an autonomous AI entity
// with a defined model, tools, skills, and behavioral configuration.
package agent

import (
	"github.com/sipeed/picoclaw/pkg/domain"
)

// ---------------------------------------------------------------------------
// Agent aggregate root
// ---------------------------------------------------------------------------

// Agent is the aggregate root for the AI agent context.
// It represents a configured, autonomous entity that processes messages,
// invokes tools, and produces responses.
type Agent struct {
	domain.AggregateRoot

	// Identity
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Model configuration
	ModelConfig ModelConfig `json:"model_config"`

	// Capabilities — tools and skills this agent can use
	Tools  []ToolBinding  `json:"tools"`
	Skills []SkillBinding `json:"skills"`

	// Behavior configuration
	SystemPrompt  string `json:"system_prompt,omitempty"`
	MaxIterations int    `json:"max_iterations"`
	Workspace     string `json:"workspace"`

	// State
	Status AgentStatus `json:"status"`

	// Metrics
	Metrics AgentMetrics `json:"metrics"`

	// Lifecycle
	CreatedAt domain.Timestamp `json:"created_at"`
	UpdatedAt domain.Timestamp `json:"updated_at"`
}

// NewAgent creates a new Agent aggregate.
func NewAgent(name string, modelCfg ModelConfig) *Agent {
	a := &Agent{
		Name:          name,
		ModelConfig:   modelCfg,
		Tools:         make([]ToolBinding, 0),
		Skills:        make([]SkillBinding, 0),
		MaxIterations: 20,
		Status:        AgentIdle,
		Metrics:       NewAgentMetrics(),
		CreatedAt:     domain.Now(),
		UpdatedAt:     domain.Now(),
	}
	a.SetID(domain.NewID())
	return a
}

// ---------------------------------------------------------------------------
// Agent behavior
// ---------------------------------------------------------------------------

// Start marks the agent as running.
func (a *Agent) Start() {
	a.Status = AgentRunning
	a.UpdatedAt = domain.Now()
	a.RecordEvent(domain.NewEvent(domain.EventAgentStarted, a.ID(), map[string]string{
		"agent": a.Name,
		"model": a.ModelConfig.Model,
	}))
}

// Stop marks the agent as stopped.
func (a *Agent) Stop() {
	a.Status = AgentStopped
	a.UpdatedAt = domain.Now()
	a.RecordEvent(domain.NewEvent(domain.EventAgentStopped, a.ID(), map[string]string{
		"agent": a.Name,
	}))
}

// MarkProcessing indicates the agent is actively processing a request.
func (a *Agent) MarkProcessing() {
	a.Status = AgentProcessing
	a.UpdatedAt = domain.Now()
}

// MarkIdle indicates the agent is waiting for input.
func (a *Agent) MarkIdle() {
	a.Status = AgentIdle
	a.UpdatedAt = domain.Now()
}

// MarkError records an agent error state.
func (a *Agent) MarkError(err string) {
	a.Status = AgentStatusError
	a.Metrics.ErrorCount++
	a.UpdatedAt = domain.Now()
	a.RecordEvent(domain.NewEvent(domain.EventAgentError, a.ID(), map[string]string{
		"agent": a.Name,
		"error": err,
	}))
}

// BindTool adds a tool to the agent's capability set.
func (a *Agent) BindTool(binding ToolBinding) {
	// Prevent duplicates
	for _, t := range a.Tools {
		if t.Name == binding.Name {
			return
		}
	}
	a.Tools = append(a.Tools, binding)
	a.UpdatedAt = domain.Now()
}

// UnbindTool removes a tool from the agent's capability set.
func (a *Agent) UnbindTool(name string) bool {
	for i, t := range a.Tools {
		if t.Name == name {
			a.Tools = append(a.Tools[:i], a.Tools[i+1:]...)
			a.UpdatedAt = domain.Now()
			return true
		}
	}
	return false
}

// BindSkill adds a skill to the agent's capability set.
func (a *Agent) BindSkill(binding SkillBinding) {
	for _, s := range a.Skills {
		if s.Name == binding.Name {
			return
		}
	}
	a.Skills = append(a.Skills, binding)
	a.UpdatedAt = domain.Now()
}

// UnbindSkill removes a skill from the agent's capability set.
func (a *Agent) UnbindSkill(name string) bool {
	for i, s := range a.Skills {
		if s.Name == name {
			a.Skills = append(a.Skills[:i], a.Skills[i+1:]...)
			a.UpdatedAt = domain.Now()
			return true
		}
	}
	return false
}

// RecordRequest tracks a completed LLM request.
func (a *Agent) RecordRequest(tokensUsed int) {
	a.Metrics.RequestCount++
	a.Metrics.TotalTokens += int64(tokensUsed)
	a.Metrics.LastRequestAt = domain.Now()
}

// RecordToolCall tracks a tool execution.
func (a *Agent) RecordToolCall() {
	a.Metrics.ToolCallCount++
}

// SetSystemPrompt updates the agent's system prompt.
func (a *Agent) SetSystemPrompt(prompt string) {
	a.SystemPrompt = prompt
	a.UpdatedAt = domain.Now()
}

// SetWorkspace sets the agent's filesystem workspace.
func (a *Agent) SetWorkspace(path string) {
	a.Workspace = path
	a.UpdatedAt = domain.Now()
}

// ---------------------------------------------------------------------------
// Value objects
// ---------------------------------------------------------------------------

// AgentStatus represents the operational state of an agent.
type AgentStatus string

const (
	AgentIdle       AgentStatus = "idle"
	AgentRunning    AgentStatus = "running"
	AgentProcessing AgentStatus = "processing"
	AgentStopped    AgentStatus = "stopped"
	AgentStatusError AgentStatus = "error"
)

func (as AgentStatus) String() string { return string(as) }

// ModelConfig holds the LLM configuration for an agent.
type ModelConfig struct {
	Provider      domain.ProviderType `json:"provider"`
	Model         string              `json:"model"`
	MaxTokens     int                 `json:"max_tokens"`
	Temperature   float64             `json:"temperature"`
	ContextWindow int                 `json:"context_window"`
}

// ToolBinding represents a tool attached to an agent.
type ToolBinding struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// SkillBinding represents a skill attached to an agent.
type SkillBinding struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Enabled bool   `json:"enabled"`
}

// AgentMetrics tracks agent performance statistics.
type AgentMetrics struct {
	RequestCount    int64            `json:"request_count"`
	ToolCallCount   int64            `json:"tool_call_count"`
	ToolErrorCount  int64            `json:"tool_error_count"`
	ErrorCount      int64            `json:"error_count"`
	TotalTokens     int64            `json:"total_tokens"`
	TotalDurationMS int64            `json:"total_duration_ms"`
	LastRequestAt   domain.Timestamp `json:"last_request_at"`
}

// NewAgentMetrics creates zero-value metrics.
func NewAgentMetrics() AgentMetrics {
	return AgentMetrics{}
}

// ---------------------------------------------------------------------------
// Repository interface
// ---------------------------------------------------------------------------

// Repository defines persistence for Agent aggregates.
type Repository interface {
	FindByID(id domain.EntityID) (*Agent, error)
	FindByName(name string) (*Agent, error)
	FindRunning() (*Agent, error)
	FindAll() ([]*Agent, error)
	Save(agent *Agent) error
	Delete(id domain.EntityID) error
}

// ---------------------------------------------------------------------------
// Domain errors
// ---------------------------------------------------------------------------

type AgentError string

func (e AgentError) Error() string { return string(e) }

const (
	ErrAgentNotFound    AgentError = "agent not found"
	ErrAgentNotRunning  AgentError = "agent is not running"
	ErrAgentBusy        AgentError = "agent is currently processing"
	ErrNoProvider       AgentError = "no LLM provider configured"
	ErrMaxIterations    AgentError = "maximum tool iterations reached"
	ErrToolNotBound     AgentError = "tool is not bound to agent"
	ErrSkillNotBound    AgentError = "skill is not bound to agent"
)
