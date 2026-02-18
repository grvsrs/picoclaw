package app

import (
	"github.com/sipeed/picoclaw/pkg/domain"
	workflowdomain "github.com/sipeed/picoclaw/pkg/domain/workflow"
)

// ---------------------------------------------------------------------------
// Workflow application service
// ---------------------------------------------------------------------------

// WorkflowService orchestrates workflow use cases.
type WorkflowService struct {
	repo     workflowdomain.Repository
	execRepo workflowdomain.ExecutionRepository
	eventBus domain.EventBus
}

// NewWorkflowService creates a new workflow application service.
func NewWorkflowService(repo workflowdomain.Repository, execRepo workflowdomain.ExecutionRepository, eventBus domain.EventBus) *WorkflowService {
	return &WorkflowService{
		repo:     repo,
		execRepo: execRepo,
		eventBus: eventBus,
	}
}

// CreateWorkflow creates and persists a new workflow.
func (s *WorkflowService) CreateWorkflow(name, description string, steps []workflowdomain.Step) (*workflowdomain.Workflow, error) {
	wf := workflowdomain.NewWorkflow(name, description)
	for _, step := range steps {
		wf.AddStep(step)
	}

	if err := s.repo.Save(wf); err != nil {
		return nil, err
	}

	s.publishEvents(wf)
	return wf, nil
}

// ActivateWorkflow validates and activates a workflow.
func (s *WorkflowService) ActivateWorkflow(id domain.EntityID) error {
	wf, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	if err := wf.Activate(); err != nil {
		return err
	}

	if err := s.repo.Save(wf); err != nil {
		return err
	}

	s.publishEvents(wf)
	return nil
}

// PauseWorkflow pauses a workflow.
func (s *WorkflowService) PauseWorkflow(id domain.EntityID) error {
	wf, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	wf.Pause()
	return s.repo.Save(wf)
}

// AddStep adds a step to an existing workflow.
func (s *WorkflowService) AddStep(workflowID domain.EntityID, step workflowdomain.Step) error {
	wf, err := s.repo.FindByID(workflowID)
	if err != nil {
		return err
	}

	wf.AddStep(step)
	return s.repo.Save(wf)
}

// RemoveStep removes a step from a workflow.
func (s *WorkflowService) RemoveStep(workflowID, stepID domain.EntityID) error {
	wf, err := s.repo.FindByID(workflowID)
	if err != nil {
		return err
	}

	if !wf.RemoveStep(stepID) {
		return workflowdomain.WorkflowError("step not found")
	}
	return s.repo.Save(wf)
}

// SetTrigger configures the trigger for a workflow.
func (s *WorkflowService) SetTrigger(workflowID domain.EntityID, trigger workflowdomain.Trigger) error {
	wf, err := s.repo.FindByID(workflowID)
	if err != nil {
		return err
	}

	wf.SetTrigger(trigger)
	return s.repo.Save(wf)
}

// GetWorkflow retrieves a workflow by ID.
func (s *WorkflowService) GetWorkflow(id domain.EntityID) (*workflowdomain.Workflow, error) {
	return s.repo.FindByID(id)
}

// ListWorkflows returns all workflows.
func (s *WorkflowService) ListWorkflows() ([]*workflowdomain.Workflow, error) {
	return s.repo.FindAll()
}

// ListActiveWorkflows returns only active workflows.
func (s *WorkflowService) ListActiveWorkflows() ([]*workflowdomain.Workflow, error) {
	return s.repo.FindActive()
}

// DeleteWorkflow removes a workflow.
func (s *WorkflowService) DeleteWorkflow(id domain.EntityID) error {
	return s.repo.Delete(id)
}

// GetExecution retrieves a workflow execution.
func (s *WorkflowService) GetExecution(execID domain.EntityID) (*workflowdomain.Execution, error) {
	if s.execRepo == nil {
		return nil, workflowdomain.ErrExecutionNotFound
	}
	return s.execRepo.FindByID(execID)
}

// ListExecutions returns recent workflow executions.
func (s *WorkflowService) ListExecutions(limit int) ([]*workflowdomain.Execution, error) {
	if s.execRepo == nil {
		return nil, nil
	}
	return s.execRepo.FindRecent(limit)
}

func (s *WorkflowService) publishEvents(wf *workflowdomain.Workflow) {
	events := wf.PullEvents()
	for _, event := range events {
		s.eventBus.Publish(event)
	}
}
