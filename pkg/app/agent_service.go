package app

import (
	"github.com/sipeed/picoclaw/pkg/domain"
	agentdomain "github.com/sipeed/picoclaw/pkg/domain/agent"
)

// ---------------------------------------------------------------------------
// Agent application service
// ---------------------------------------------------------------------------

// AgentService orchestrates agent lifecycle and capability binding.
type AgentService struct {
	repo     agentdomain.Repository
	eventBus domain.EventBus
}

// NewAgentService creates a new agent application service.
func NewAgentService(repo agentdomain.Repository, eventBus domain.EventBus) *AgentService {
	return &AgentService{
		repo:     repo,
		eventBus: eventBus,
	}
}

// CreateAgent creates and persists a new agent.
func (s *AgentService) CreateAgent(name string, config agentdomain.ModelConfig) (*agentdomain.Agent, error) {
	ag := agentdomain.NewAgent(name, config)
	if err := s.repo.Save(ag); err != nil {
		return nil, err
	}
	return ag, nil
}

// StartAgent transitions an agent to the running state.
func (s *AgentService) StartAgent(id domain.EntityID) error {
	ag, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	ag.Start()
	if err := s.repo.Save(ag); err != nil {
		return err
	}

	s.publishEvents(ag)
	return nil
}

// StopAgent transitions an agent to the stopped state.
func (s *AgentService) StopAgent(id domain.EntityID) error {
	ag, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	ag.Stop()
	if err := s.repo.Save(ag); err != nil {
		return err
	}

	s.publishEvents(ag)
	return nil
}

// BindTool attaches a tool to an agent.
func (s *AgentService) BindTool(agentID domain.EntityID, binding agentdomain.ToolBinding) error {
	ag, err := s.repo.FindByID(agentID)
	if err != nil {
		return err
	}

	ag.BindTool(binding)
	return s.repo.Save(ag)
}

// UnbindTool removes a tool from an agent.
func (s *AgentService) UnbindTool(agentID domain.EntityID, toolName string) error {
	ag, err := s.repo.FindByID(agentID)
	if err != nil {
		return err
	}

	if !ag.UnbindTool(toolName) {
		return agentdomain.ErrToolNotBound
	}
	return s.repo.Save(ag)
}

// BindSkill attaches a skill to an agent.
func (s *AgentService) BindSkill(agentID domain.EntityID, binding agentdomain.SkillBinding) error {
	ag, err := s.repo.FindByID(agentID)
	if err != nil {
		return err
	}

	ag.BindSkill(binding)
	return s.repo.Save(ag)
}

// UnbindSkill removes a skill from an agent.
func (s *AgentService) UnbindSkill(agentID domain.EntityID, skillName string) error {
	ag, err := s.repo.FindByID(agentID)
	if err != nil {
		return err
	}

	if !ag.UnbindSkill(skillName) {
		return agentdomain.ErrSkillNotBound
	}
	return s.repo.Save(ag)
}

// SetSystemPrompt updates the agent's system prompt.
func (s *AgentService) SetSystemPrompt(agentID domain.EntityID, prompt string) error {
	ag, err := s.repo.FindByID(agentID)
	if err != nil {
		return err
	}

	ag.SetSystemPrompt(prompt)
	return s.repo.Save(ag)
}

// SetWorkspace updates the agent's workspace directory.
func (s *AgentService) SetWorkspace(agentID domain.EntityID, workspace string) error {
	ag, err := s.repo.FindByID(agentID)
	if err != nil {
		return err
	}

	ag.SetWorkspace(workspace)
	return s.repo.Save(ag)
}

// RecordRequest records a request processed by the agent.
func (s *AgentService) RecordRequest(agentID domain.EntityID, tokens int) error {
	ag, err := s.repo.FindByID(agentID)
	if err != nil {
		return err
	}

	ag.RecordRequest(tokens)
	return s.repo.Save(ag)
}

// RecordToolCall records a tool invocation by the agent.
func (s *AgentService) RecordToolCall(agentID domain.EntityID) error {
	ag, err := s.repo.FindByID(agentID)
	if err != nil {
		return err
	}

	ag.RecordToolCall()
	return s.repo.Save(ag)
}

// GetAgent retrieves an agent by ID.
func (s *AgentService) GetAgent(id domain.EntityID) (*agentdomain.Agent, error) {
	return s.repo.FindByID(id)
}

// ListAgents returns all registered agents.
func (s *AgentService) ListAgents() ([]*agentdomain.Agent, error) {
	return s.repo.FindAll()
}

// GetRunningAgent returns the currently running agent (if any).
func (s *AgentService) GetRunningAgent() (*agentdomain.Agent, error) {
	return s.repo.FindRunning()
}

// DeleteAgent removes an agent.
func (s *AgentService) DeleteAgent(id domain.EntityID) error {
	return s.repo.Delete(id)
}

func (s *AgentService) publishEvents(ag *agentdomain.Agent) {
	events := ag.PullEvents()
	for _, event := range events {
		s.eventBus.Publish(event)
	}
}
