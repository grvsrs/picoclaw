// Package orchestration implements deterministic task routing, ownership locking,
// capability-based agent assignment, and retry/failure policies for the bot swarm.
//
// This is the "swarm brain" — it answers:
//   - Which agent handles this task?
//   - Is anyone already working on it?
//   - What happens when it fails?
//   - How do we prevent duplicate execution?
package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TaskAssignment represents a task claimed by an agent.
type TaskAssignment struct {
	TaskID    string    `json:"task_id"`
	AgentID   string    `json:"agent_id"`
	ClaimedAt time.Time `json:"claimed_at"`
	ExpiresAt time.Time `json:"expires_at"` // claim lease — auto-releases if agent dies
	Attempt   int       `json:"attempt"`
	MaxRetry  int       `json:"max_retry"`
	Status    string    `json:"status"` // claimed, executing, completed, failed, expired
}

// AgentCapability describes what an agent can do.
type AgentCapability struct {
	AgentID      string   `json:"agent_id"`
	Categories   []string `json:"categories"`    // task categories this agent handles
	Tools        []string `json:"tools"`          // tools this agent has access to
	MaxConcurrent int     `json:"max_concurrent"` // max tasks at once
	Priority     int      `json:"priority"`       // higher = preferred for matching tasks
}

// RetryPolicy defines how failures are handled.
type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts"`
	Backoff     time.Duration `json:"backoff"`       // base delay between retries
	MaxBackoff  time.Duration `json:"max_backoff"`
	Escalate    bool          `json:"escalate"`      // escalate to human on final failure
}

// DefaultRetryPolicy returns a sensible default.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		Backoff:     5 * time.Second,
		MaxBackoff:  60 * time.Second,
		Escalate:    true,
	}
}

// Orchestrator manages task assignment, locking, and execution policies.
type Orchestrator struct {
	assignments  map[string]*TaskAssignment // taskID -> assignment
	capabilities map[string]*AgentCapability // agentID -> capability
	policies     map[string]RetryPolicy     // category -> retry policy
	mu           sync.RWMutex
	defaultPolicy RetryPolicy
}

// NewOrchestrator creates a new orchestrator with default policies.
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		assignments:   make(map[string]*TaskAssignment),
		capabilities:  make(map[string]*AgentCapability),
		policies:      make(map[string]RetryPolicy),
		defaultPolicy: DefaultRetryPolicy(),
	}
}

// --- Capability Registry ---

// RegisterAgent adds an agent's capabilities to the registry.
func (o *Orchestrator) RegisterAgent(cap AgentCapability) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.capabilities[cap.AgentID] = &cap
}

// UnregisterAgent removes an agent from the registry.
func (o *Orchestrator) UnregisterAgent(agentID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.capabilities, agentID)
}

// GetAgents returns all registered agent capabilities.
func (o *Orchestrator) GetAgents() []AgentCapability {
	o.mu.RLock()
	defer o.mu.RUnlock()
	agents := make([]AgentCapability, 0, len(o.capabilities))
	for _, cap := range o.capabilities {
		agents = append(agents, *cap)
	}
	return agents
}

// --- Task Routing ---

// RouteTask finds the best agent for a given task category.
// Returns the agent ID or empty string if no agent can handle it.
func (o *Orchestrator) RouteTask(category string) (string, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var bestAgent string
	bestPriority := -1

	for agentID, cap := range o.capabilities {
		// Check if agent handles this category
		handles := false
		for _, cat := range cap.Categories {
			if cat == category || cat == "*" {
				handles = true
				break
			}
		}
		if !handles {
			continue
		}

		// Check concurrency limit
		activeCount := o.countActiveAssignments(agentID)
		if cap.MaxConcurrent > 0 && activeCount >= cap.MaxConcurrent {
			continue
		}

		// Prefer higher priority
		if cap.Priority > bestPriority {
			bestPriority = cap.Priority
			bestAgent = agentID
		}
	}

	if bestAgent == "" {
		return "", fmt.Errorf("no agent available for category %q", category)
	}

	return bestAgent, nil
}

func (o *Orchestrator) countActiveAssignments(agentID string) int {
	count := 0
	for _, a := range o.assignments {
		if a.AgentID == agentID && (a.Status == "claimed" || a.Status == "executing") {
			count++
		}
	}
	return count
}

// --- Task Locking ---

// ClaimTask attempts to lock a task for an agent. Returns error if already claimed.
// Claims have a lease duration — if the agent doesn't complete within the lease,
// the claim expires and another agent can pick it up.
func (o *Orchestrator) ClaimTask(ctx context.Context, taskID, agentID string, leaseDuration time.Duration) (*TaskAssignment, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Check existing claim
	if existing, ok := o.assignments[taskID]; ok {
		if existing.Status == "claimed" || existing.Status == "executing" {
			// Check if lease expired
			if time.Now().Before(existing.ExpiresAt) {
				return nil, fmt.Errorf("task %s already claimed by %s (expires %s)",
					taskID, existing.AgentID, existing.ExpiresAt.Format(time.RFC3339))
			}
			// Lease expired — allow re-claim
			existing.Status = "expired"
		}
	}

	now := time.Now()
	attempt := 1
	if prev, ok := o.assignments[taskID]; ok {
		attempt = prev.Attempt + 1
	}

	assignment := &TaskAssignment{
		TaskID:    taskID,
		AgentID:   agentID,
		ClaimedAt: now,
		ExpiresAt: now.Add(leaseDuration),
		Attempt:   attempt,
		MaxRetry:  o.getPolicy(taskID).MaxAttempts,
		Status:    "claimed",
	}

	o.assignments[taskID] = assignment
	return assignment, nil
}

// CompleteTask marks a task as completed by the claiming agent.
func (o *Orchestrator) CompleteTask(taskID, agentID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	assignment, ok := o.assignments[taskID]
	if !ok {
		return fmt.Errorf("task %s not found in assignments", taskID)
	}

	if assignment.AgentID != agentID {
		return fmt.Errorf("task %s is not claimed by %s", taskID, agentID)
	}

	assignment.Status = "completed"
	return nil
}

// FailTask marks a task as failed. Returns true if retries are available.
func (o *Orchestrator) FailTask(taskID, agentID, reason string) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	assignment, ok := o.assignments[taskID]
	if !ok {
		return false, fmt.Errorf("task %s not found in assignments", taskID)
	}

	if assignment.AgentID != agentID {
		return false, fmt.Errorf("task %s is not claimed by %s", taskID, agentID)
	}

	policy := o.getPolicy(taskID)
	if assignment.Attempt < policy.MaxAttempts {
		// Release the claim so another agent (or same) can retry
		assignment.Status = "failed"
		return true, nil // retryable
	}

	// Final failure
	assignment.Status = "failed"
	return false, nil // no more retries
}

// ReleaseClaim releases a task claim voluntarily (agent can't handle it).
func (o *Orchestrator) ReleaseClaim(taskID, agentID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	assignment, ok := o.assignments[taskID]
	if !ok {
		return nil // nothing to release
	}

	if assignment.AgentID != agentID {
		return fmt.Errorf("task %s is not claimed by %s", taskID, agentID)
	}

	assignment.Status = "released"
	return nil
}

// GetAssignment returns the current assignment for a task.
func (o *Orchestrator) GetAssignment(taskID string) (*TaskAssignment, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	a, ok := o.assignments[taskID]
	return a, ok
}

// GetActiveAssignments returns all active (claimed/executing) assignments.
func (o *Orchestrator) GetActiveAssignments() []TaskAssignment {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var active []TaskAssignment
	for _, a := range o.assignments {
		if a.Status == "claimed" || a.Status == "executing" {
			active = append(active, *a)
		}
	}
	return active
}

// --- Retry Policies ---

// SetPolicy sets a retry policy for a task category.
func (o *Orchestrator) SetPolicy(category string, policy RetryPolicy) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.policies[category] = policy
}

func (o *Orchestrator) getPolicy(taskID string) RetryPolicy {
	// TODO: look up category from task ID via kanban
	return o.defaultPolicy
}

// --- Lease Cleanup ---

// CleanupExpiredLeases releases claims that have passed their expiry.
// Call this periodically (e.g., every 30s).
func (o *Orchestrator) CleanupExpiredLeases() int {
	o.mu.Lock()
	defer o.mu.Unlock()

	now := time.Now()
	expired := 0
	for _, a := range o.assignments {
		if (a.Status == "claimed" || a.Status == "executing") && now.After(a.ExpiresAt) {
			a.Status = "expired"
			expired++
		}
	}
	return expired
}

// RunLeaseWatcher starts a background goroutine that cleans expired leases.
func (o *Orchestrator) RunLeaseWatcher(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			expired := o.CleanupExpiredLeases()
			if expired > 0 {
				// Log or broadcast
				_ = expired
			}
		}
	}
}

// --- Observability ---

// Status returns a snapshot of the orchestrator state.
func (o *Orchestrator) Status() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	claimed, executing, completed, failed := 0, 0, 0, 0
	for _, a := range o.assignments {
		switch a.Status {
		case "claimed":
			claimed++
		case "executing":
			executing++
		case "completed":
			completed++
		case "failed":
			failed++
		}
	}

	return map[string]interface{}{
		"agents_registered": len(o.capabilities),
		"tasks_claimed":     claimed,
		"tasks_executing":   executing,
		"tasks_completed":   completed,
		"tasks_failed":      failed,
		"total_assignments": len(o.assignments),
	}
}
