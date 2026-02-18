// Package workflow defines the Workflow bounded context.
// A Workflow is an aggregate root representing a composable pipeline of
// skill steps — enabling deterministic multi-step agent automation.
package workflow

import (
	"github.com/sipeed/picoclaw/pkg/domain"
)

// ---------------------------------------------------------------------------
// Workflow aggregate root
// ---------------------------------------------------------------------------

// Workflow is the aggregate root for skill chaining and pipeline execution.
// It represents a deterministic sequence (or DAG) of skill invocations.
type Workflow struct {
	domain.AggregateRoot

	// Identity
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Version     string      `json:"version"`
	Author      string      `json:"author,omitempty"`
	Tags        domain.Tags `json:"tags,omitempty"`

	// Pipeline definition
	Steps     []Step     `json:"steps"`
	Variables []Variable `json:"variables,omitempty"`

	// State
	Status    WorkflowStatus `json:"status"`
	Enabled   bool           `json:"enabled"`

	// Trigger — what starts this workflow
	Trigger Trigger `json:"trigger"`

	// Metrics
	Metrics WorkflowMetrics `json:"metrics"`

	// Lifecycle
	CreatedAt domain.Timestamp `json:"created_at"`
	UpdatedAt domain.Timestamp `json:"updated_at"`
}

// NewWorkflow creates a new Workflow aggregate.
func NewWorkflow(name, description string) *Workflow {
	w := &Workflow{
		Name:        name,
		Description: description,
		Version:     "0.1.0",
		Steps:       make([]Step, 0),
		Variables:   make([]Variable, 0),
		Status:      StatusDraft,
		Enabled:     true,
		Metrics:     NewWorkflowMetrics(),
		CreatedAt:   domain.Now(),
		UpdatedAt:   domain.Now(),
	}
	w.SetID(domain.NewID())
	return w
}

// ---------------------------------------------------------------------------
// Workflow behavior
// ---------------------------------------------------------------------------

// AddStep appends a step to the workflow pipeline.
func (w *Workflow) AddStep(step Step) {
	step.Order = len(w.Steps) + 1
	if step.ID.IsZero() {
		step.ID = domain.NewID()
	}
	w.Steps = append(w.Steps, step)
	w.UpdatedAt = domain.Now()
}

// InsertStep inserts a step at a specific position (0-indexed).
func (w *Workflow) InsertStep(position int, step Step) {
	if step.ID.IsZero() {
		step.ID = domain.NewID()
	}
	if position >= len(w.Steps) {
		w.AddStep(step)
		return
	}

	w.Steps = append(w.Steps[:position+1], w.Steps[position:]...)
	w.Steps[position] = step

	// Reorder
	for i := range w.Steps {
		w.Steps[i].Order = i + 1
	}
	w.UpdatedAt = domain.Now()
}

// RemoveStep removes a step by ID.
func (w *Workflow) RemoveStep(stepID domain.EntityID) bool {
	for i, step := range w.Steps {
		if step.ID == stepID {
			w.Steps = append(w.Steps[:i], w.Steps[i+1:]...)
			// Reorder
			for j := range w.Steps {
				w.Steps[j].Order = j + 1
			}
			w.UpdatedAt = domain.Now()
			return true
		}
	}
	return false
}

// Validate checks that the workflow is executable.
func (w *Workflow) Validate() error {
	if w.Name == "" {
		return ErrEmptyName
	}
	if len(w.Steps) == 0 {
		return ErrNoSteps
	}
	// Check for duplicate step IDs
	seen := make(map[domain.EntityID]bool)
	for _, step := range w.Steps {
		if seen[step.ID] {
			return ErrDuplicateStepID
		}
		seen[step.ID] = true
	}
	return nil
}

// Activate marks the workflow as ready for execution.
func (w *Workflow) Activate() error {
	if err := w.Validate(); err != nil {
		return err
	}
	w.Status = StatusActive
	w.UpdatedAt = domain.Now()
	w.RecordEvent(domain.NewEvent(domain.EventWorkflowCreated, w.ID(), map[string]string{
		"workflow": w.Name,
		"steps":    string(rune(len(w.Steps))),
	}))
	return nil
}

// Pause puts the workflow into paused state.
func (w *Workflow) Pause() {
	w.Status = StatusPaused
	w.UpdatedAt = domain.Now()
}

// SetTrigger configures what starts this workflow.
func (w *Workflow) SetTrigger(trigger Trigger) {
	w.Trigger = trigger
	w.UpdatedAt = domain.Now()
}

// RecordExecution updates metrics after a workflow run.
func (w *Workflow) RecordExecution(success bool, durationMS int64) {
	w.Metrics.RunCount++
	w.Metrics.LastRunAt = domain.Now()
	w.Metrics.TotalDurationMS += durationMS
	if success {
		w.Metrics.SuccessCount++
	} else {
		w.Metrics.FailureCount++
	}
}

// ---------------------------------------------------------------------------
// Value objects
// ---------------------------------------------------------------------------

// WorkflowStatus tracks the lifecycle state of a workflow.
type WorkflowStatus string

const (
	StatusDraft   WorkflowStatus = "draft"
	StatusActive  WorkflowStatus = "active"
	StatusPaused  WorkflowStatus = "paused"
	StatusArchived WorkflowStatus = "archived"
)

func (ws WorkflowStatus) String() string { return string(ws) }

// Step represents a single unit of work in the workflow pipeline.
type Step struct {
	ID          domain.EntityID        `json:"id"`
	Order       int                    `json:"order"`
	SkillName   string                 `json:"skill_name"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	InputMap    map[string]string      `json:"input_map,omitempty"`  // maps step inputs to vars/prev outputs
	OutputMap   map[string]string      `json:"output_map,omitempty"` // maps step outputs to workflow vars
	OnError     ErrorStrategy          `json:"on_error"`
	Condition   string                 `json:"condition,omitempty"` // optional expression to skip step
	TimeoutSec  int                    `json:"timeout_sec,omitempty"`
	RetryCount  int                    `json:"retry_count,omitempty"`
}

// NewStep creates a new workflow step.
func NewStep(skillName, name string) Step {
	return Step{
		ID:        domain.NewID(),
		SkillName: skillName,
		Name:      name,
		Config:    make(map[string]interface{}),
		InputMap:  make(map[string]string),
		OutputMap: make(map[string]string),
		OnError:   ErrorStop,
	}
}

// ErrorStrategy defines what happens when a step fails.
type ErrorStrategy string

const (
	ErrorStop     ErrorStrategy = "stop"     // abort the workflow
	ErrorContinue ErrorStrategy = "continue" // skip and continue
	ErrorRetry    ErrorStrategy = "retry"    // retry the step
)

// Variable stores data flowing between workflow steps.
type Variable struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"` // "string", "json", "int", "bool"
	Value   interface{} `json:"value,omitempty"`
	Default interface{} `json:"default,omitempty"`
}

// Trigger defines what starts a workflow execution.
type Trigger struct {
	Type     TriggerType `json:"type"`
	Schedule string      `json:"schedule,omitempty"` // cron expression
	Event    string      `json:"event,omitempty"`    // domain event type
	Webhook  string      `json:"webhook,omitempty"`  // webhook path
	Manual   bool        `json:"manual,omitempty"`
}

// TriggerType classifies workflow triggers.
type TriggerType string

const (
	TriggerManual   TriggerType = "manual"
	TriggerSchedule TriggerType = "schedule"
	TriggerEvent    TriggerType = "event"
	TriggerWebhook  TriggerType = "webhook"
)

// WorkflowMetrics tracks execution statistics.
type WorkflowMetrics struct {
	RunCount        int64            `json:"run_count"`
	SuccessCount    int64            `json:"success_count"`
	FailureCount    int64            `json:"failure_count"`
	TotalDurationMS int64            `json:"total_duration_ms"`
	LastRunAt       domain.Timestamp `json:"last_run_at"`
}

// NewWorkflowMetrics creates zero-value metrics.
func NewWorkflowMetrics() WorkflowMetrics {
	return WorkflowMetrics{}
}

// ---------------------------------------------------------------------------
// Execution — a running instance of a workflow
// ---------------------------------------------------------------------------

// Execution represents a single run of a workflow.
type Execution struct {
	domain.AggregateRoot

	WorkflowID   domain.EntityID   `json:"workflow_id"`
	WorkflowName string            `json:"workflow_name"`
	Status       ExecutionStatus   `json:"status"`
	StepResults  []StepResult      `json:"step_results"`
	Variables    map[string]interface{} `json:"variables"`
	StartedAt    domain.Timestamp  `json:"started_at"`
	CompletedAt  domain.Timestamp  `json:"completed_at,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// ExecutionStatus tracks the state of a workflow execution.
type ExecutionStatus string

const (
	ExecPending   ExecutionStatus = "pending"
	ExecRunning   ExecutionStatus = "running"
	ExecCompleted ExecutionStatus = "completed"
	ExecFailed    ExecutionStatus = "failed"
	ExecCancelled ExecutionStatus = "cancelled"
)

// StepResult captures the outcome of a single step execution.
type StepResult struct {
	StepID     domain.EntityID        `json:"step_id"`
	StepName   string                 `json:"step_name"`
	SkillName  string                 `json:"skill_name"`
	Status     ExecutionStatus        `json:"status"`
	Output     map[string]interface{} `json:"output,omitempty"`
	Error      string                 `json:"error,omitempty"`
	DurationMS int64                  `json:"duration_ms"`
	StartedAt  domain.Timestamp       `json:"started_at"`
}

// NewExecution creates a new workflow execution.
func NewExecution(workflowID domain.EntityID, workflowName string) *Execution {
	e := &Execution{
		WorkflowID:   workflowID,
		WorkflowName: workflowName,
		Status:       ExecPending,
		StepResults:  make([]StepResult, 0),
		Variables:    make(map[string]interface{}),
		StartedAt:    domain.Now(),
	}
	e.SetID(domain.NewID())
	return e
}

// ---------------------------------------------------------------------------
// Repository interface
// ---------------------------------------------------------------------------

// Repository defines persistence for Workflow aggregates.
type Repository interface {
	FindByID(id domain.EntityID) (*Workflow, error)
	FindByName(name string) (*Workflow, error)
	FindActive() ([]*Workflow, error)
	FindAll() ([]*Workflow, error)
	Save(wf *Workflow) error
	Delete(id domain.EntityID) error
}

// ExecutionRepository persists workflow execution records.
type ExecutionRepository interface {
	FindByID(id domain.EntityID) (*Execution, error)
	FindByWorkflow(workflowID domain.EntityID) ([]*Execution, error)
	FindRecent(limit int) ([]*Execution, error)
	Save(exec *Execution) error
	Delete(id domain.EntityID) error
}

// ---------------------------------------------------------------------------
// Engine — workflow runtime
// ---------------------------------------------------------------------------

// Engine orchestrates the execution of workflows.
type Engine interface {
	// Execute runs a workflow with optional initial variables.
	Execute(wf *Workflow, inputs map[string]interface{}) (*Execution, error)
	// Cancel aborts a running execution.
	Cancel(executionID domain.EntityID) error
	// Status returns the current state of an execution.
	Status(executionID domain.EntityID) (*Execution, error)
}

// ---------------------------------------------------------------------------
// Domain errors
// ---------------------------------------------------------------------------

type WorkflowError string

func (e WorkflowError) Error() string { return string(e) }

const (
	ErrEmptyName       WorkflowError = "workflow name cannot be empty"
	ErrNoSteps         WorkflowError = "workflow must have at least one step"
	ErrDuplicateStepID WorkflowError = "duplicate step ID in workflow"
	ErrWorkflowNotFound WorkflowError = "workflow not found"
	ErrExecutionNotFound WorkflowError = "execution not found"
	ErrInvalidTrigger  WorkflowError = "invalid workflow trigger"
)
