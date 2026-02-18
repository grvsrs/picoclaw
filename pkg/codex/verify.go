// Package codex — verification and approval pipeline for structured diffs.
// This file adds the "Verify → Report" stage to the existing
// "Task → Agent → StructuredDiff → Validate → Apply" pipeline.
//
// Pipeline flow:
//   1. Apply() modifies files and returns ApplyResult
//   2. RunVerification() executes syntax check + tests
//   3. ApprovalGate checks whether the diff requires human approval
//   4. On failure: rollback + event log
package codex

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// VerifyResult captures the outcome of post-apply verification.
type VerifyResult struct {
	DiffID        string        `json:"diff_id"`
	TaskID        string        `json:"task_id"`
	SyntaxPassed  *bool         `json:"syntax_passed,omitempty"`
	SyntaxOutput  string        `json:"syntax_output,omitempty"`
	TestsPassed   *bool         `json:"tests_passed,omitempty"`
	TestOutput    string        `json:"test_output,omitempty"`
	RolledBack    bool          `json:"rolled_back"`
	RollbackError string        `json:"rollback_error,omitempty"`
	Duration      time.Duration `json:"duration_ms"`
	Error         string        `json:"error,omitempty"`
}

// ApprovalLevel describes how critical a diff is and whether it needs human review.
type ApprovalLevel string

const (
	ApprovalAuto     ApprovalLevel = "auto"     // safe to apply without review
	ApprovalNotify   ApprovalLevel = "notify"   // apply but notify the user
	ApprovalRequired ApprovalLevel = "required" // block until human approves
)

// ApprovalPolicy defines rules for when human approval is needed.
type ApprovalPolicy struct {
	// Paths that always require approval (glob patterns)
	CriticalPaths []string `json:"critical_paths"`

	// Operations that require approval
	CriticalOps []DiffOperation `json:"critical_ops"`

	// Max number of files changed before requiring approval
	MaxAutoFiles int `json:"max_auto_files"`

	// Max total lines changed before requiring approval
	MaxAutoLines int `json:"max_auto_lines"`

	// Categories that require approval (from kanban task category)
	CriticalCategories []string `json:"critical_categories"`
}

// DefaultPolicy returns a sensible default approval policy for a personal
// automation OS — auto-apply most things, require approval for destructive ops.
func DefaultPolicy() *ApprovalPolicy {
	return &ApprovalPolicy{
		CriticalPaths: []string{
			"*.env", "*.secret*", "**/credentials*",
			"Makefile", "Dockerfile", "docker-compose*",
			"go.mod", "go.sum", "package.json", "package-lock.json",
			".github/**", ".gitlab-ci*",
			"**/main.go", "**/main.py",
		},
		CriticalOps:  []DiffOperation{OpDelete},
		MaxAutoFiles: 10,
		MaxAutoLines: 500,
	}
}

// EvaluateApproval determines the approval level for a diff.
func (p *ApprovalPolicy) EvaluateApproval(diff *StructuredDiff) (ApprovalLevel, string) {
	if p == nil {
		return ApprovalAuto, ""
	}

	// Check file count
	if p.MaxAutoFiles > 0 && len(diff.Changes) > p.MaxAutoFiles {
		return ApprovalRequired, fmt.Sprintf(
			"diff touches %d files (max auto: %d)", len(diff.Changes), p.MaxAutoFiles)
	}

	// Check for critical operations
	for _, change := range diff.Changes {
		for _, critOp := range p.CriticalOps {
			if change.Op == critOp {
				return ApprovalRequired, fmt.Sprintf(
					"diff contains critical operation %s on %s", change.Op, change.Path)
			}
		}
	}

	// Check for critical paths
	for _, change := range diff.Changes {
		for _, pattern := range p.CriticalPaths {
			matched, _ := filepath.Match(pattern, change.Path)
			if !matched {
				// Try matching just the filename
				matched, _ = filepath.Match(pattern, filepath.Base(change.Path))
			}
			if !matched && strings.Contains(pattern, "**") {
				// Simple ** glob: check if the non-** suffix matches
				suffix := strings.SplitN(pattern, "**", 2)
				if len(suffix) == 2 {
					matched = strings.HasSuffix(change.Path, strings.TrimPrefix(suffix[1], "/"))
				}
			}
			if matched {
				return ApprovalRequired, fmt.Sprintf(
					"diff modifies critical path %s (pattern: %s)", change.Path, pattern)
			}
		}
	}

	// Check total lines changed
	if p.MaxAutoLines > 0 {
		totalLines := 0
		for _, change := range diff.Changes {
			totalLines += strings.Count(change.NewContent, "\n") + 1
			if change.OldContent != "" {
				totalLines += strings.Count(change.OldContent, "\n") + 1
			}
		}
		if totalLines > p.MaxAutoLines {
			return ApprovalNotify, fmt.Sprintf(
				"diff changes ~%d lines (threshold: %d)", totalLines, p.MaxAutoLines)
		}
	}

	return ApprovalAuto, ""
}

// RunVerification executes the verify spec after a diff has been applied.
// If verification fails and RollbackOnFailure is true, the rollback function
// is called to undo changes.
func RunVerification(
	ctx context.Context,
	diff *StructuredDiff,
	workspaceRoot string,
	rollbackFn func() error,
) (*VerifyResult, error) {
	if diff.Verify == nil {
		return &VerifyResult{
			DiffID: diff.ID,
			TaskID: diff.TaskID,
		}, nil
	}

	start := time.Now()
	result := &VerifyResult{
		DiffID: diff.ID,
		TaskID: diff.TaskID,
	}

	spec := diff.Verify

	// Stage 1: Syntax check
	if spec.SyntaxCheck != "" {
		passed, output, err := runCommand(ctx, workspaceRoot, spec.SyntaxCheck, 60*time.Second)
		result.SyntaxPassed = &passed
		result.SyntaxOutput = truncateOutput(output, 4096)
		if err != nil && !passed {
			result.Error = fmt.Sprintf("syntax check failed: %s", err)
			if spec.RollbackOnFailure && rollbackFn != nil {
				if rbErr := rollbackFn(); rbErr != nil {
					result.RollbackError = rbErr.Error()
				} else {
					result.RolledBack = true
				}
			}
			result.Duration = time.Since(start)
			return result, nil
		}
	}

	// Stage 2: Test command
	if spec.TestCommand != "" {
		passed, output, err := runCommand(ctx, workspaceRoot, spec.TestCommand, 300*time.Second)
		result.TestsPassed = &passed
		result.TestOutput = truncateOutput(output, 8192)
		if err != nil && !passed {
			result.Error = fmt.Sprintf("tests failed: %s", err)
			if spec.RollbackOnFailure && rollbackFn != nil {
				if rbErr := rollbackFn(); rbErr != nil {
					result.RollbackError = rbErr.Error()
				} else {
					result.RolledBack = true
				}
			}
			result.Duration = time.Since(start)
			return result, nil
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// ApplyAndVerify is the full pipeline: apply → verify → rollback on failure.
// This is the recommended entry point for automated diff application.
func (sd *StructuredDiff) ApplyAndVerify(
	ctx context.Context,
	workspaceRoot string,
	policy *ApprovalPolicy,
) (*ApplyVerifyResult, error) {
	avr := &ApplyVerifyResult{
		DiffID:  sd.ID,
		TaskID:  sd.TaskID,
		AgentID: sd.AgentID,
	}

	// Step 1: Check approval
	if policy != nil {
		level, reason := policy.EvaluateApproval(sd)
		avr.ApprovalLevel = level
		avr.ApprovalReason = reason
		if level == ApprovalRequired {
			avr.Status = "pending_approval"
			return avr, nil
		}
	} else {
		avr.ApprovalLevel = ApprovalAuto
	}

	// Step 2: Check preconditions
	if err := sd.CheckPreconditions(workspaceRoot); err != nil {
		avr.Status = "precondition_failed"
		avr.Error = err.Error()
		return avr, err
	}

	// Step 3: Apply
	applyResult, err := sd.Apply(workspaceRoot)
	avr.Apply = applyResult
	if err != nil {
		avr.Status = "apply_failed"
		avr.Error = err.Error()
		return avr, err
	}

	// Step 4: Build rollback function from the workspace state
	rollbackFn := func() error {
		// Re-apply in reverse by reading current state and reverting
		// This is a simplified rollback — full rollback already happened in Apply
		// on failure, but this is for post-apply verification rollback.
		for i := len(sd.Changes) - 1; i >= 0; i-- {
			change := sd.Changes[i]
			if err := rollbackChange(workspaceRoot, change); err != nil {
				return fmt.Errorf("rollback change[%d] %s: %w", i, change.Path, err)
			}
		}
		return nil
	}

	// Step 5: Verify
	verifyResult, err := RunVerification(ctx, sd, workspaceRoot, rollbackFn)
	avr.Verify = verifyResult
	if verifyResult != nil && verifyResult.RolledBack {
		avr.Status = "rolled_back"
		avr.Error = verifyResult.Error
		return avr, nil
	}

	if verifyResult != nil && verifyResult.Error != "" {
		avr.Status = "verify_failed"
		avr.Error = verifyResult.Error
		return avr, nil
	}

	avr.Status = "success"
	return avr, nil
}

// ApplyVerifyResult is the complete outcome of the apply+verify pipeline.
type ApplyVerifyResult struct {
	DiffID         string         `json:"diff_id"`
	TaskID         string         `json:"task_id"`
	AgentID        string         `json:"agent_id"`
	Status         string         `json:"status"` // success, pending_approval, precondition_failed, apply_failed, verify_failed, rolled_back
	ApprovalLevel  ApprovalLevel  `json:"approval_level"`
	ApprovalReason string         `json:"approval_reason,omitempty"`
	Apply          *ApplyResult   `json:"apply,omitempty"`
	Verify         *VerifyResult  `json:"verify,omitempty"`
	Error          string         `json:"error,omitempty"`
}

// --- Internal helpers ---

// runCommand executes a shell command in the workspace and returns (passed, output, error).
func runCommand(ctx context.Context, workDir, cmdStr string, timeout time.Duration) (bool, string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Split command for exec — use shell for complex commands
	cmd := exec.CommandContext(cmdCtx, "sh", "-c", cmdStr)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "CI=true") // hint to test frameworks

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n--- stderr ---\n"
		}
		output += stderr.String()
	}

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return false, output, fmt.Errorf("command timed out after %s", timeout)
		}
		return false, output, err
	}

	return true, output, nil
}

// rollbackChange reverses a single file change.
// This is used for post-apply rollback (when verification fails).
func rollbackChange(root string, change FileChange) error {
	fullPath := filepath.Join(root, change.Path)

	switch change.Op {
	case OpCreate:
		// Undo create → delete the file
		return os.Remove(fullPath)

	case OpModify:
		// Undo modify → reverse the replacement
		existing, err := os.ReadFile(fullPath)
		if err != nil {
			return err
		}
		content := string(existing)
		if !strings.Contains(content, change.NewContent) {
			return fmt.Errorf("cannot rollback %s: new_content not found", change.Path)
		}
		reverted := strings.Replace(content, change.NewContent, change.OldContent, 1)
		return os.WriteFile(fullPath, []byte(reverted), 0644)

	case OpDelete:
		// Undo delete → recreate (we don't have content, best effort)
		return fmt.Errorf("cannot rollback delete of %s: original content not preserved", change.Path)

	case OpRename:
		// Undo rename → rename back
		newPath := filepath.Join(root, change.NewPath)
		return os.Rename(newPath, fullPath)

	case OpInsert:
		// Undo insert → remove the inserted lines
		existing, err := os.ReadFile(fullPath)
		if err != nil {
			return err
		}
		content := string(existing)
		if !strings.Contains(content, change.NewContent) {
			return fmt.Errorf("cannot rollback insert in %s: content not found", change.Path)
		}
		reverted := strings.Replace(content, change.NewContent+"\n", "", 1)
		if reverted == content {
			reverted = strings.Replace(content, "\n"+change.NewContent, "", 1)
		}
		return os.WriteFile(fullPath, []byte(reverted), 0644)
	}

	return nil
}

// truncateOutput limits output to maxLen bytes, appending a truncation notice.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... [truncated]"
}

// RollbackLog records a rollback event for audit trail.
type RollbackLog struct {
	DiffID      string    `json:"diff_id"`
	TaskID      string    `json:"task_id"`
	AgentID     string    `json:"agent_id"`
	Reason      string    `json:"reason"`
	Stage       string    `json:"stage"` // syntax_check, test, manual
	RolledBack  bool      `json:"rolled_back"`
	Error       string    `json:"error,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewRollbackLog creates a rollback log entry from verification results.
func NewRollbackLog(diff *StructuredDiff, verify *VerifyResult, stage string) *RollbackLog {
	log := &RollbackLog{
		DiffID:    diff.ID,
		TaskID:    diff.TaskID,
		AgentID:   diff.AgentID,
		Stage:     stage,
		Timestamp: time.Now(),
	}
	if verify != nil {
		log.RolledBack = verify.RolledBack
		log.Reason = verify.Error
		log.Error = verify.RollbackError
	}
	return log
}
