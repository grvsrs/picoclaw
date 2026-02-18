// Package skill defines the Skill bounded context.
// A Skill is an aggregate root representing a reusable, composable capability
// that PicoClaw agents can invoke — the building block of the skill ecosystem.
package skill

import (
	"github.com/sipeed/picoclaw/pkg/domain"
)

// ---------------------------------------------------------------------------
// Skill aggregate root
// ---------------------------------------------------------------------------

// Skill is the aggregate root for the skill ecosystem.
// It represents a deterministic, composable module that wraps a tool or pipeline.
type Skill struct {
	domain.AggregateRoot

	// Identity & discovery
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author,omitempty"`
	License     string            `json:"license,omitempty"`
	Tags        domain.Tags       `json:"tags,omitempty"`
	Category    SkillCategory     `json:"category"`
	Source      domain.SkillSource `json:"source"`

	// Spec — the deterministic interface contract
	Spec SkillSpec `json:"spec"`

	// State
	Enabled   bool             `json:"enabled"`
	Installed bool             `json:"installed"`
	Path      string           `json:"path"`

	// Metrics
	Metrics SkillMetrics `json:"metrics"`

	// Dependencies — skills this skill requires
	Dependencies []SkillDependency `json:"dependencies,omitempty"`

	// Lifecycle
	CreatedAt domain.Timestamp `json:"created_at"`
	UpdatedAt domain.Timestamp `json:"updated_at"`
}

// NewSkill creates a new Skill aggregate.
func NewSkill(name, version, description string, category SkillCategory, source domain.SkillSource) *Skill {
	s := &Skill{
		Name:        name,
		Version:     version,
		Description: description,
		Category:    category,
		Source:      source,
		Spec:        SkillSpec{},
		Enabled:     true,
		Installed:   true,
		Metrics:     NewSkillMetrics(),
		CreatedAt:   domain.Now(),
		UpdatedAt:   domain.Now(),
	}
	s.SetID(domain.NewID())
	return s
}

// ---------------------------------------------------------------------------
// Skill behavior
// ---------------------------------------------------------------------------

// Install marks the skill as installed at a specific path.
func (s *Skill) Install(path string) {
	s.Installed = true
	s.Path = path
	s.UpdatedAt = domain.Now()
	s.RecordEvent(domain.NewEvent(domain.EventSkillInstalled, s.ID(), map[string]string{
		"skill":   s.Name,
		"version": s.Version,
		"source":  string(s.Source),
	}))
}

// Uninstall marks the skill as removed.
func (s *Skill) Uninstall() {
	s.Installed = false
	s.Enabled = false
	s.Path = ""
	s.UpdatedAt = domain.Now()
	s.RecordEvent(domain.NewEvent(domain.EventSkillUninstalled, s.ID(), map[string]string{
		"skill": s.Name,
	}))
}

// Enable activates the skill for agent use.
func (s *Skill) Enable() {
	s.Enabled = true
	s.UpdatedAt = domain.Now()
}

// Disable deactivates the skill.
func (s *Skill) Disable() {
	s.Enabled = false
	s.UpdatedAt = domain.Now()
}

// RecordExecution tracks a successful skill execution.
func (s *Skill) RecordExecution(durationMS int64) {
	s.Metrics.ExecutionCount++
	s.Metrics.TotalDurationMS += durationMS
	s.Metrics.LastExecutedAt = domain.Now()
	if s.Metrics.ExecutionCount > 0 {
		s.Metrics.AvgDurationMS = s.Metrics.TotalDurationMS / s.Metrics.ExecutionCount
	}
}

// RecordError tracks a failed skill execution.
func (s *Skill) RecordError(errMsg string) {
	s.Metrics.ErrorCount++
	s.Metrics.LastError = errMsg
	s.Metrics.LastErrorAt = domain.Now()
}

// HasDependency returns true if this skill depends on another.
func (s *Skill) HasDependency(skillName string) bool {
	for _, dep := range s.Dependencies {
		if dep.SkillName == skillName {
			return true
		}
	}
	return false
}

// AddDependency declares a dependency on another skill.
func (s *Skill) AddDependency(skillName, versionConstraint string, required bool) {
	s.Dependencies = append(s.Dependencies, SkillDependency{
		SkillName:         skillName,
		VersionConstraint: versionConstraint,
		Required:          required,
	})
	s.UpdatedAt = domain.Now()
}

// ---------------------------------------------------------------------------
// Value objects
// ---------------------------------------------------------------------------

// SkillCategory classifies skills for discovery.
type SkillCategory string

const (
	CategoryResearch   SkillCategory = "research"
	CategoryKnowledge  SkillCategory = "knowledge"
	CategoryAutomation SkillCategory = "automation"
	CategoryMedia      SkillCategory = "media"
	CategoryDevOps     SkillCategory = "devops"
	CategoryData       SkillCategory = "data"
	CategoryComms      SkillCategory = "communication"
	CategorySystem     SkillCategory = "system"
	CategoryCustom     SkillCategory = "custom"
)

func (sc SkillCategory) String() string { return string(sc) }

// SkillSpec defines the deterministic interface contract of a skill.
// This is the core of composability — skills are Linux commands for AI.
type SkillSpec struct {
	// Inputs define what the skill accepts.
	Inputs []SkillParam `json:"inputs,omitempty"`
	// Outputs define what the skill produces.
	Outputs []SkillParam `json:"outputs,omitempty"`
	// Command is the execution command template (e.g., "python skills/fetch.py {{topic}}")
	Command string `json:"command,omitempty"`
	// Entrypoint is the function/script entry for programmatic invocation.
	Entrypoint string `json:"entrypoint,omitempty"`
	// Timeout in seconds for execution.
	TimeoutSec int `json:"timeout_sec,omitempty"`
	// Idempotent indicates if re-execution with same inputs produces same outputs.
	Idempotent bool `json:"idempotent"`
}

// SkillParam defines a typed input or output.
type SkillParam struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // "string", "int", "float", "bool", "json", "file"
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

// SkillDependency declares a dependency on another skill.
type SkillDependency struct {
	SkillName         string `json:"skill_name"`
	VersionConstraint string `json:"version_constraint,omitempty"` // semver range
	Required          bool   `json:"required"`
}

// SkillMetrics tracks execution statistics.
type SkillMetrics struct {
	ExecutionCount  int64            `json:"execution_count"`
	ErrorCount      int64            `json:"error_count"`
	TotalDurationMS int64            `json:"total_duration_ms"`
	AvgDurationMS   int64            `json:"avg_duration_ms"`
	LastExecutedAt  domain.Timestamp `json:"last_executed_at"`
	LastError       string           `json:"last_error,omitempty"`
	LastErrorAt     domain.Timestamp `json:"last_error_at"`
}

// NewSkillMetrics creates zero-value metrics.
func NewSkillMetrics() SkillMetrics {
	return SkillMetrics{}
}

// ---------------------------------------------------------------------------
// Repository interface
// ---------------------------------------------------------------------------

// Repository defines persistence operations for Skill aggregates.
type Repository interface {
	FindByID(id domain.EntityID) (*Skill, error)
	FindByName(name string) (*Skill, error)
	FindByCategory(category SkillCategory) ([]*Skill, error)
	FindByTags(tags domain.Tags) ([]*Skill, error)
	FindBySource(source domain.SkillSource) ([]*Skill, error)
	FindEnabled() ([]*Skill, error)
	FindAll() ([]*Skill, error)
	Save(skill *Skill) error
	Delete(id domain.EntityID) error
	Search(query string) ([]*Skill, error)
}

// ---------------------------------------------------------------------------
// Registry — the ClawHub equivalent for skill discovery
// ---------------------------------------------------------------------------

// Registry defines the skill registry operations (local ClawHub).
type Registry interface {
	// Register adds a skill to the registry.
	Register(skill *Skill) error
	// Unregister removes a skill from the registry.
	Unregister(name string) error
	// Discover finds skills matching criteria.
	Discover(query string, category SkillCategory, tags domain.Tags) ([]*Skill, error)
	// Get retrieves a specific skill by name.
	Get(name string) (*Skill, error)
	// List returns all registered skills.
	List() ([]*Skill, error)
	// Count returns the number of registered skills.
	Count() int
}

// ---------------------------------------------------------------------------
// Executor — skill runtime
// ---------------------------------------------------------------------------

// ExecutionResult captures the output of a skill execution.
type ExecutionResult struct {
	SkillName  string                 `json:"skill_name"`
	Success    bool                   `json:"success"`
	Output     string                 `json:"output"`
	Data       map[string]interface{} `json:"data,omitempty"`
	DurationMS int64                  `json:"duration_ms"`
	Error      string                 `json:"error,omitempty"`
}

// Executor runs a skill with given inputs and returns results.
type Executor interface {
	Execute(skill *Skill, inputs map[string]interface{}) (*ExecutionResult, error)
}

// ---------------------------------------------------------------------------
// Domain errors
// ---------------------------------------------------------------------------

type SkillError string

func (e SkillError) Error() string { return string(e) }

const (
	ErrSkillNotFound       SkillError = "skill not found"
	ErrSkillAlreadyExists  SkillError = "skill already exists"
	ErrSkillNotInstalled   SkillError = "skill not installed"
	ErrSkillDisabled       SkillError = "skill is disabled"
	ErrInvalidSkillSpec    SkillError = "invalid skill specification"
	ErrMissingDependency   SkillError = "missing required dependency"
	ErrCircularDependency  SkillError = "circular dependency detected"
	ErrExecutionTimeout    SkillError = "skill execution timed out"
	ErrExecutionFailed     SkillError = "skill execution failed"
)

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

// Factory creates Skill aggregates with validation.
type Factory struct{}

// CreateSkill validates and constructs a new Skill aggregate.
func (f Factory) CreateSkill(name, version, description string, category SkillCategory, source domain.SkillSource, spec SkillSpec) (*Skill, error) {
	if name == "" {
		return nil, SkillError("skill name cannot be empty")
	}
	if version == "" {
		version = "0.1.0"
	}

	s := NewSkill(name, version, description, category, source)
	s.Spec = spec
	return s, nil
}
