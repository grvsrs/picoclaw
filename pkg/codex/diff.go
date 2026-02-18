// Package codex defines the structured diff format for deterministic coding bot
// operations. This is the typed contract that coding agents MUST produce instead
// of free-text responses when modifying code.
//
// The pipeline: Task → Agent → StructuredDiff → Validate → Apply → Verify → Report
package codex

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DiffOperation represents the type of file change.
type DiffOperation string

const (
	OpCreate  DiffOperation = "create"
	OpModify  DiffOperation = "modify"  // replace specific content
	OpDelete  DiffOperation = "delete"
	OpRename  DiffOperation = "rename"
	OpInsert  DiffOperation = "insert"  // insert at line number
)

// FileChange represents a single atomic file change in a structured diff.
type FileChange struct {
	// Operation type
	Op DiffOperation `json:"op"`

	// Target file path (relative to workspace root)
	Path string `json:"path"`

	// For rename: the new path
	NewPath string `json:"new_path,omitempty"`

	// For create/modify — the content
	// For modify: OldContent → NewContent (search/replace)
	// For create: NewContent is the full file
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`

	// For insert: line number (1-based) and content to insert after that line
	LineNumber int    `json:"line_number,omitempty"`

	// Description of what this change does
	Description string `json:"description"`
}

// StructuredDiff is the complete output a coding agent must produce.
// This replaces free-text responses for code modification tasks.
type StructuredDiff struct {
	// Unique ID for this diff (for idempotency)
	ID string `json:"id"`

	// The kanban task ID this diff is for
	TaskID string `json:"task_id"`

	// Agent that produced this diff
	AgentID string `json:"agent_id"`

	// Timestamp
	CreatedAt time.Time `json:"created_at"`

	// Human-readable summary of all changes
	Summary string `json:"summary"`

	// The ordered list of file changes
	Changes []FileChange `json:"changes"`

	// Pre-conditions: files that must exist with expected content hashes
	// before applying (ensures we don't apply to stale files)
	Preconditions []FilePrecondition `json:"preconditions,omitempty"`

	// Post-conditions: optional verification steps
	Verify *VerifySpec `json:"verify,omitempty"`
}

// FilePrecondition ensures a file hasn't changed since the agent read it.
type FilePrecondition struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`      // content hash
	MustExist  bool   `json:"must_exist"`
}

// VerifySpec defines how to verify the diff was applied correctly.
type VerifySpec struct {
	// Command to run for syntax check (e.g., "go build ./...")
	SyntaxCheck string `json:"syntax_check,omitempty"`

	// Command to run tests (e.g., "go test ./...")
	TestCommand string `json:"test_command,omitempty"`

	// If true, rollback all changes on test failure
	RollbackOnFailure bool `json:"rollback_on_failure"`
}

// --- Validation ---

// Validate checks that the diff is well-formed before applying.
func (sd *StructuredDiff) Validate() error {
	if sd.ID == "" {
		return fmt.Errorf("diff ID is required")
	}
	if sd.TaskID == "" {
		return fmt.Errorf("task ID is required")
	}
	if len(sd.Changes) == 0 {
		return fmt.Errorf("diff has no changes")
	}

	for i, change := range sd.Changes {
		if err := change.Validate(); err != nil {
			return fmt.Errorf("change[%d]: %w", i, err)
		}
	}
	return nil
}

// Validate checks a single file change.
func (fc *FileChange) Validate() error {
	if fc.Path == "" {
		return fmt.Errorf("path is required")
	}
	if strings.Contains(fc.Path, "..") {
		return fmt.Errorf("path traversal not allowed: %s", fc.Path)
	}

	switch fc.Op {
	case OpCreate:
		if fc.NewContent == "" {
			return fmt.Errorf("new_content required for create")
		}
	case OpModify:
		if fc.OldContent == "" || fc.NewContent == "" {
			return fmt.Errorf("old_content and new_content required for modify")
		}
	case OpDelete:
		// path only
	case OpRename:
		if fc.NewPath == "" {
			return fmt.Errorf("new_path required for rename")
		}
	case OpInsert:
		if fc.LineNumber < 1 {
			return fmt.Errorf("line_number must be >= 1 for insert")
		}
		if fc.NewContent == "" {
			return fmt.Errorf("new_content required for insert")
		}
	default:
		return fmt.Errorf("unknown operation: %s", fc.Op)
	}
	return nil
}

// --- Precondition Checking ---

// CheckPreconditions verifies all preconditions against the filesystem.
func (sd *StructuredDiff) CheckPreconditions(workspaceRoot string) error {
	for _, pre := range sd.Preconditions {
		fullPath := filepath.Join(workspaceRoot, pre.Path)
		data, err := os.ReadFile(fullPath)

		if err != nil {
			if os.IsNotExist(err) && pre.MustExist {
				return fmt.Errorf("precondition failed: %s must exist", pre.Path)
			}
			continue
		}

		if pre.SHA256 != "" {
			hash := fmt.Sprintf("%x", sha256.Sum256(data))
			if hash != pre.SHA256 {
				return fmt.Errorf("precondition failed: %s has changed (expected %s, got %s)",
					pre.Path, pre.SHA256[:12], hash[:12])
			}
		}
	}
	return nil
}

// --- Application ---

// Apply applies the diff to the filesystem atomically.
// On any failure, it rolls back all previously applied changes.
func (sd *StructuredDiff) Apply(workspaceRoot string) (*ApplyResult, error) {
	result := &ApplyResult{
		DiffID:    sd.ID,
		TaskID:    sd.TaskID,
		StartedAt: time.Now(),
	}

	// Track applied changes for rollback
	var rollbackOps []rollbackOp

	for i, change := range sd.Changes {
		if err := applyChange(workspaceRoot, change, &rollbackOps); err != nil {
			// Rollback everything
			for j := len(rollbackOps) - 1; j >= 0; j-- {
				rollbackOps[j].undo()
			}
			result.Success = false
			result.Error = fmt.Sprintf("change[%d] (%s %s): %v", i, change.Op, change.Path, err)
			result.CompletedAt = time.Now()
			return result, err
		}
		result.FilesChanged++
	}

	result.Success = true
	result.CompletedAt = time.Now()
	return result, nil
}

// ApplyResult is the outcome of applying a structured diff.
type ApplyResult struct {
	DiffID       string    `json:"diff_id"`
	TaskID       string    `json:"task_id"`
	Success      bool      `json:"success"`
	FilesChanged int       `json:"files_changed"`
	Error        string    `json:"error,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	TestPassed   *bool     `json:"test_passed,omitempty"`
}

type rollbackOp struct {
	undo func()
}

func applyChange(root string, change FileChange, rollbackOps *[]rollbackOp) error {
	fullPath := filepath.Join(root, change.Path)

	switch change.Op {
	case OpCreate:
		// Ensure parent dir exists
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(change.NewContent), 0644); err != nil {
			return err
		}
		*rollbackOps = append(*rollbackOps, rollbackOp{undo: func() { os.Remove(fullPath) }})

	case OpModify:
		// Read current content
		existing, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("file not found: %s", change.Path)
		}

		content := string(existing)
		if !strings.Contains(content, change.OldContent) {
			return fmt.Errorf("old_content not found in %s", change.Path)
		}

		newContent := strings.Replace(content, change.OldContent, change.NewContent, 1)
		backup := string(existing)

		if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
			return err
		}
		*rollbackOps = append(*rollbackOps, rollbackOp{
			undo: func() { os.WriteFile(fullPath, []byte(backup), 0644) },
		})

	case OpDelete:
		existing, _ := os.ReadFile(fullPath)
		backup := string(existing)
		if err := os.Remove(fullPath); err != nil {
			return err
		}
		*rollbackOps = append(*rollbackOps, rollbackOp{
			undo: func() { os.WriteFile(fullPath, []byte(backup), 0644) },
		})

	case OpRename:
		newFullPath := filepath.Join(root, change.NewPath)
		if err := os.MkdirAll(filepath.Dir(newFullPath), 0755); err != nil {
			return err
		}
		if err := os.Rename(fullPath, newFullPath); err != nil {
			return err
		}
		*rollbackOps = append(*rollbackOps, rollbackOp{
			undo: func() { os.Rename(newFullPath, fullPath) },
		})

	case OpInsert:
		existing, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("file not found: %s", change.Path)
		}
		backup := string(existing)

		lines := strings.Split(string(existing), "\n")
		if change.LineNumber > len(lines) {
			lines = append(lines, change.NewContent)
		} else {
			newLines := make([]string, 0, len(lines)+1)
			newLines = append(newLines, lines[:change.LineNumber]...)
			newLines = append(newLines, change.NewContent)
			newLines = append(newLines, lines[change.LineNumber:]...)
			lines = newLines
		}

		if err := os.WriteFile(fullPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
			return err
		}
		*rollbackOps = append(*rollbackOps, rollbackOp{
			undo: func() { os.WriteFile(fullPath, []byte(backup), 0644) },
		})
	}

	return nil
}

// --- LLM Prompt ---

// AgentPrompt returns the system prompt that constrains the coding agent
// to produce StructuredDiff JSON instead of free text.
const AgentPrompt = `You are a deterministic coding agent. When asked to modify code, you MUST respond with a JSON object matching this schema:

{
  "id": "unique-diff-id",
  "task_id": "the kanban task ID",
  "agent_id": "your agent ID",
  "summary": "brief description of all changes",
  "changes": [
    {
      "op": "create|modify|delete|rename|insert",
      "path": "relative/path/to/file",
      "old_content": "exact text to find (for modify)",
      "new_content": "replacement text",
      "description": "what this change does"
    }
  ],
  "preconditions": [
    {"path": "file.go", "sha256": "abc123...", "must_exist": true}
  ],
  "verify": {
    "syntax_check": "go build ./...",
    "test_command": "go test ./...",
    "rollback_on_failure": true
  }
}

Rules:
1. ALWAYS output valid JSON — no markdown, no explanation, just the diff object.
2. For modify operations, include enough context in old_content to be unambiguous.
3. Include preconditions for any file you read to prevent stale-state bugs.
4. Include verify commands when possible.
5. Each change must be independently understandable from its description.
6. Path must be relative to workspace root. No "../" traversal.
`

// ParseDiff parses a JSON string into a StructuredDiff.
func ParseDiff(data string) (*StructuredDiff, error) {
	// Strip markdown code fences if the LLM wraps the JSON
	trimmed := strings.TrimSpace(data)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		// Remove first and last lines (fences)
		if len(lines) > 2 {
			trimmed = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var diff StructuredDiff
	if err := json.Unmarshal([]byte(trimmed), &diff); err != nil {
		return nil, fmt.Errorf("failed to parse structured diff: %w", err)
	}
	return &diff, nil
}
