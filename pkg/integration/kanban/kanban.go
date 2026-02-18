// Package kanban provides a Go-native integration with the task board system.
// This replaces the need for direct SQLite access from Python and provides
// a unified API that both the Go backend and Python bots can use.
package kanban

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/integration"
	"github.com/sipeed/picoclaw/pkg/logger"
)

func init() {
	// Auto-register with the global integration registry
	integration.Register(&KanbanIntegration{})
}

// TaskState represents the lifecycle state of a task.
type TaskState string

const (
	StateInbox   TaskState = "inbox"
	StatePlanned TaskState = "planned"
	StateRunning TaskState = "running"
	StateBlocked TaskState = "blocked"
	StateReview  TaskState = "review"
	StateDone    TaskState = "done"
)

// TaskCategory represents an LLM-assigned category for a task.
type TaskCategory string

const (
	CategoryCode       TaskCategory = "code"
	CategoryDesign     TaskCategory = "design"
	CategoryInfra      TaskCategory = "infra"
	CategoryBug        TaskCategory = "bug"
	CategoryFeature    TaskCategory = "feature"
	CategoryResearch   TaskCategory = "research"
	CategoryOps        TaskCategory = "ops"
	CategoryPersonal   TaskCategory = "personal"
	CategoryMeeting    TaskCategory = "meeting"
	CategoryUncategorized TaskCategory = "uncategorized"
)

// AllCategories returns all valid task categories.
func AllCategories() []TaskCategory {
	return []TaskCategory{
		CategoryCode, CategoryDesign, CategoryInfra, CategoryBug,
		CategoryFeature, CategoryResearch, CategoryOps,
		CategoryPersonal, CategoryMeeting, CategoryUncategorized,
	}
}

// TaskSource identifies where a task originated from.
type TaskSource string

const (
	SourceTelegram TaskSource = "telegram"
	SourceVSCode   TaskSource = "vscode"
	SourceAPI      TaskSource = "api"
	SourceCLI      TaskSource = "cli"
	SourceLLM      TaskSource = "llm"
	SourceManual   TaskSource = "manual"
)

// Task represents a universal task card.
type Task struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	State       TaskState    `json:"state"`
	Category    TaskCategory `json:"category"`
	Source      TaskSource   `json:"source"`
	Priority    string       `json:"priority"` // low, normal, high, critical
	Tags        []string     `json:"tags"`
	Assignee    string       `json:"assignee"`
	Project     string       `json:"project"`

	// Tracking
	Attempts         int    `json:"attempts"`
	LastFailureReason string `json:"last_failure_reason"`
	ExecutionLogURL  string `json:"execution_log_url"`

	// Ownership — connects to orchestrator lease system
	ClaimedBy      string     `json:"claimed_by,omitempty"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at,omitempty"`
	ClaimCount     int        `json:"claim_count"`
	LastError      string     `json:"last_error,omitempty"`

	// External links
	TelegramMessageID string `json:"telegram_message_id,omitempty"`
	VSCodeTaskID      string `json:"vscode_task_id,omitempty"`
	ExternalRef       string `json:"external_ref,omitempty"`

	// LLM metadata
	LLMCategorized bool   `json:"llm_categorized"`
	LLMSummary     string `json:"llm_summary,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DueDate   *time.Time `json:"due_date,omitempty"`
}

// StateTransition records a state change event.
type StateTransition struct {
	FromState TaskState `json:"from_state"`
	ToState   TaskState `json:"to_state"`
	Reason    string    `json:"reason"`
	Executor  string    `json:"executor"`
	Timestamp time.Time `json:"timestamp"`
}

// ValidTransitions defines the allowed state machine transitions.
var ValidTransitions = map[TaskState][]TaskState{
	StateInbox:   {StatePlanned, StateRunning, StateDone},
	StatePlanned: {StateRunning, StateBlocked},
	StateRunning: {StateBlocked, StateReview, StateDone},
	StateBlocked: {StateRunning, StatePlanned},
	StateReview:  {StateDone, StateBlocked},
	StateDone:    {}, // terminal
}

// KanbanIntegration is the Go-native task board integration.
type KanbanIntegration struct {
	db     *sql.DB
	dbPath string
	cfg    *config.Config
	bus    *bus.MessageBus
	mu     sync.RWMutex
}

func (k *KanbanIntegration) Name() string {
	return "kanban"
}

func (k *KanbanIntegration) Init(cfg *config.Config, msgBus *bus.MessageBus) error {
	k.cfg = cfg
	k.bus = msgBus

	// Determine DB path
	k.dbPath = os.Getenv("PICOCLAW_DB")
	if k.dbPath == "" {
		k.dbPath = filepath.Join(cfg.WorkspacePath(), "kanban.db")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(k.dbPath), 0755); err != nil {
		return fmt.Errorf("create kanban db dir: %w", err)
	}

	return nil
}

func (k *KanbanIntegration) Start(ctx context.Context) error {
	db, err := sql.Open("sqlite3", k.dbPath+"?_journal_mode=WAL&_foreign_keys=ON")
	if err != nil {
		return fmt.Errorf("open kanban db: %w", err)
	}
	k.db = db

	if err := k.initSchema(); err != nil {
		return fmt.Errorf("init kanban schema: %w", err)
	}

	logger.InfoCF("kanban", "Task board started", map[string]interface{}{
		"db_path": k.dbPath,
	})
	return nil
}

func (k *KanbanIntegration) Stop(ctx context.Context) error {
	if k.db != nil {
		return k.db.Close()
	}
	return nil
}

func (k *KanbanIntegration) Health() error {
	if k.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return k.db.Ping()
}

func (k *KanbanIntegration) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		state TEXT DEFAULT 'inbox',
		category TEXT DEFAULT 'uncategorized',
		source TEXT DEFAULT 'manual',
		priority TEXT DEFAULT 'normal',
		tags TEXT DEFAULT '[]',
		assignee TEXT DEFAULT '',
		project TEXT DEFAULT '',
		attempts INTEGER DEFAULT 0,
		last_failure_reason TEXT DEFAULT '',
		execution_log_url TEXT DEFAULT '',
		telegram_message_id TEXT,
		vscode_task_id TEXT,
		external_ref TEXT,
		llm_categorized INTEGER DEFAULT 0,
		llm_summary TEXT DEFAULT '',
		claimed_by TEXT DEFAULT '',
		lease_expires_at TEXT,
		claim_count INTEGER DEFAULT 0,
		last_error TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		due_date TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_claimed ON tasks(claimed_by);

	CREATE TABLE IF NOT EXISTS task_transitions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		from_state TEXT NOT NULL,
		to_state TEXT NOT NULL,
		reason TEXT DEFAULT '',
		executor TEXT DEFAULT '',
		timestamp TEXT NOT NULL,
		FOREIGN KEY (task_id) REFERENCES tasks(id)
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_state ON tasks(state);
	CREATE INDEX IF NOT EXISTS idx_tasks_category ON tasks(category);
	CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project);
	CREATE INDEX IF NOT EXISTS idx_tasks_source ON tasks(source);
	CREATE INDEX IF NOT EXISTS idx_tasks_external_ref ON tasks(external_ref);
	CREATE INDEX IF NOT EXISTS idx_task_transitions_task ON task_transitions(task_id);

	CREATE TABLE IF NOT EXISTS task_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT,
		source TEXT NOT NULL,
		event_type TEXT NOT NULL,
		summary TEXT NOT NULL,
		details TEXT DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE INDEX IF NOT EXISTS idx_task_events_task ON task_events(task_id, created_at);

	CREATE TABLE IF NOT EXISTS task_notes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT,
		content TEXT NOT NULL,
		author TEXT DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (task_id) REFERENCES tasks(id)
	);

	CREATE TABLE IF NOT EXISTS system_kv (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	`
	_, err := k.db.Exec(schema)
	return err
}

// CreateTask creates a new task and returns it.
func (k *KanbanIntegration) CreateTask(task *Task) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if task.ID == "" {
		id, err := k.nextID()
		if err != nil {
			return err
		}
		task.ID = id
	}

	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now

	if task.State == "" {
		task.State = StateInbox
	}
	if task.Priority == "" {
		task.Priority = "normal"
	}
	if task.Category == "" {
		task.Category = CategoryUncategorized
	}

	tagsJSON, _ := json.Marshal(task.Tags)

	_, err := k.db.Exec(`
		INSERT INTO tasks (id, title, description, state, category, source, priority, tags,
			assignee, project, attempts, last_failure_reason, execution_log_url,
			telegram_message_id, vscode_task_id, external_ref,
			llm_categorized, llm_summary, claimed_by, lease_expires_at, claim_count, last_error,
			created_at, updated_at, due_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Title, task.Description, task.State, task.Category,
		task.Source, task.Priority, string(tagsJSON),
		task.Assignee, task.Project, task.Attempts,
		task.LastFailureReason, task.ExecutionLogURL,
		task.TelegramMessageID, task.VSCodeTaskID, task.ExternalRef,
		task.LLMCategorized, task.LLMSummary,
		task.ClaimedBy, formatOptionalTime(task.LeaseExpiresAt), task.ClaimCount, task.LastError,
		task.CreatedAt.Format(time.RFC3339), task.UpdatedAt.Format(time.RFC3339),
		formatOptionalTime(task.DueDate),
	)

	// Publish task.created event to bus
	if err == nil && k.bus != nil {
		k.bus.PublishSystem(bus.SystemEvent{
			Type:   "task.created",
			Source: "kanban",
			Data: map[string]interface{}{
				"task_id":  task.ID,
				"title":    task.Title,
				"state":    task.State,
				"category": task.Category,
				"source":   task.Source,
			},
		})
	}
	return err
}

// GetTask retrieves a task by ID.
func (k *KanbanIntegration) GetTask(id string) (*Task, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	row := k.db.QueryRow("SELECT * FROM tasks WHERE id = ?", id)
	return k.scanTask(row)
}

// GetTaskByExternalRef looks up a task by its external_ref field.
// Returns nil, nil if no task matches (not an error).
func (k *KanbanIntegration) GetTaskByExternalRef(ref string) (*Task, error) {
	if ref == "" {
		return nil, nil
	}
	k.mu.RLock()
	defer k.mu.RUnlock()

	row := k.db.QueryRow("SELECT * FROM tasks WHERE external_ref = ?", ref)
	task, err := k.scanTask(row)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

// ListTasks returns tasks matching the given filters.
func (k *KanbanIntegration) ListTasks(filters TaskFilters) ([]*Task, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	query := "SELECT * FROM tasks WHERE 1=1"
	args := []interface{}{}

	if filters.State != "" {
		query += " AND state = ?"
		args = append(args, string(filters.State))
	}
	if filters.Category != "" {
		query += " AND category = ?"
		args = append(args, string(filters.Category))
	}
	if filters.Source != "" {
		query += " AND source = ?"
		args = append(args, string(filters.Source))
	}
	if filters.Project != "" {
		query += " AND project = ?"
		args = append(args, filters.Project)
	}
	if filters.ExcludeDone {
		query += " AND state != 'done'"
	}

	query += " ORDER BY updated_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filters.Limit)
	} else {
		query += " LIMIT 500"
	}

	rows, err := k.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		task, err := k.scanTaskFromRows(rows)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// TransitionTask moves a task to a new state if the transition is valid.
func (k *KanbanIntegration) TransitionTask(id string, newState TaskState, reason, executor string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	row := k.db.QueryRow("SELECT state FROM tasks WHERE id = ?", id)
	var currentState string
	if err := row.Scan(&currentState); err != nil {
		return fmt.Errorf("task %s not found: %w", id, err)
	}

	// Validate transition
	allowed := ValidTransitions[TaskState(currentState)]
	valid := false
	for _, s := range allowed {
		if s == newState {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid transition: %s → %s", currentState, newState)
	}

	now := time.Now().UTC()
	tx, err := k.db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE tasks SET state = ?, updated_at = ? WHERE id = ?",
		string(newState), now.Format(time.RFC3339), id)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`INSERT INTO task_transitions (task_id, from_state, to_state, reason, executor, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, currentState, string(newState), reason, executor, now.Format(time.RFC3339))
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Publish state transition event
	if k.bus != nil {
		eventType := "task.updated"
		if newState == StateDone {
			eventType = "task.completed"
		} else if newState == StateBlocked {
			eventType = "task.failed"
		}
		k.bus.PublishSystem(bus.SystemEvent{
			Type:   eventType,
			Source: "kanban",
			Data: map[string]interface{}{
				"task_id":    id,
				"from_state": currentState,
				"to_state":   string(newState),
				"reason":     reason,
				"executor":   executor,
			},
		})
	}
	return nil
}

// UpdateTask updates a task's mutable fields.
func (k *KanbanIntegration) UpdateTask(id string, updates map[string]interface{}) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	allowedFields := map[string]bool{
		"title": true, "description": true, "category": true,
		"priority": true, "assignee": true, "project": true,
		"tags": true, "due_date": true, "llm_summary": true,
		"llm_categorized": true, "external_ref": true,
		"claimed_by": true, "lease_expires_at": true, "claim_count": true,
		"last_error": true, "last_failure_reason": true,
	}

	setClauses := []string{}
	args := []interface{}{}
	for field, val := range updates {
		if !allowedFields[field] {
			continue
		}
		if field == "tags" {
			if tags, ok := val.([]string); ok {
				j, _ := json.Marshal(tags)
				val = string(j)
			}
		}
		setClauses = append(setClauses, field+" = ?")
		args = append(args, val)
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now().UTC().Format(time.RFC3339))
	args = append(args, id)

	query := "UPDATE tasks SET " + joinStrings(setClauses, ", ") + " WHERE id = ?"
	_, err := k.db.Exec(query, args...)
	return err
}

// DeleteTask removes a task and its transitions.
func (k *KanbanIntegration) DeleteTask(id string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	tx, err := k.db.Begin()
	if err != nil {
		return err
	}

	tx.Exec("DELETE FROM task_transitions WHERE task_id = ?", id)
	tx.Exec("DELETE FROM task_notes WHERE task_id = ?", id)
	tx.Exec("DELETE FROM task_events WHERE task_id = ?", id)
	tx.Exec("DELETE FROM tasks WHERE id = ?", id)

	if err := tx.Commit(); err != nil {
		return err
	}

	if k.bus != nil {
		k.bus.PublishSystem(bus.SystemEvent{
			Type:   "task.deleted",
			Source: "kanban",
			Data:   map[string]interface{}{"task_id": id},
		})
	}
	return nil
}

// ClaimTask marks a task as claimed by an agent with a lease expiry.
// Returns error if already claimed by someone else with an active lease.
func (k *KanbanIntegration) ClaimTask(taskID, agentID string, leaseDuration time.Duration) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now().UTC()

	// Check current claim
	var claimedBy sql.NullString
	var leaseExpires sql.NullString
	err := k.db.QueryRow("SELECT claimed_by, lease_expires_at FROM tasks WHERE id = ?", taskID).
		Scan(&claimedBy, &leaseExpires)
	if err != nil {
		return fmt.Errorf("task %s not found: %w", taskID, err)
	}

	// If claimed by someone else and lease hasn't expired, reject
	if claimedBy.Valid && claimedBy.String != "" && claimedBy.String != agentID {
		if leaseExpires.Valid {
			expiry, _ := time.Parse(time.RFC3339, leaseExpires.String)
			if now.Before(expiry) {
				return fmt.Errorf("task %s already claimed by %s (expires %s)",
					taskID, claimedBy.String, expiry.Format(time.RFC3339))
			}
		}
	}

	expiresAt := now.Add(leaseDuration)
	_, err = k.db.Exec(`UPDATE tasks SET claimed_by = ?, lease_expires_at = ?,
		claim_count = claim_count + 1, state = 'running', updated_at = ? WHERE id = ?`,
		agentID, expiresAt.Format(time.RFC3339), now.Format(time.RFC3339), taskID)
	if err != nil {
		return err
	}

	if k.bus != nil {
		k.bus.PublishSystem(bus.SystemEvent{
			Type:   "task.claimed",
			Source: "kanban",
			Data: map[string]interface{}{
				"task_id":    taskID,
				"claimed_by": agentID,
				"expires_at": expiresAt.Format(time.RFC3339),
			},
		})
	}
	return nil
}

// ReleaseTask clears the claim on a task, optionally setting error info.
func (k *KanbanIntegration) ReleaseTask(taskID, agentID, reason string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now().UTC()
	newState := string(StatePlanned)
	if reason != "" {
		newState = string(StateBlocked)
	}

	_, err := k.db.Exec(`UPDATE tasks SET claimed_by = '', lease_expires_at = NULL,
		state = ?, last_error = ?, updated_at = ? WHERE id = ? AND claimed_by = ?`,
		newState, reason, now.Format(time.RFC3339), taskID, agentID)
	if err != nil {
		return err
	}

	eventType := "task.released"
	if reason != "" {
		eventType = "task.failed"
	}
	if k.bus != nil {
		k.bus.PublishSystem(bus.SystemEvent{
			Type:   eventType,
			Source: "kanban",
			Data: map[string]interface{}{
				"task_id":  taskID,
				"agent_id": agentID,
				"reason":   reason,
			},
		})
	}
	return nil
}

// CompleteTask marks a task as done and clears ownership.
func (k *KanbanIntegration) CompleteTask(taskID, agentID string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now().UTC()
	_, err := k.db.Exec(`UPDATE tasks SET claimed_by = '', lease_expires_at = NULL,
		state = 'done', last_error = '', updated_at = ? WHERE id = ?`,
		now.Format(time.RFC3339), taskID)
	if err != nil {
		return err
	}

	if k.bus != nil {
		k.bus.PublishSystem(bus.SystemEvent{
			Type:   "task.completed",
			Source: "kanban",
			Data: map[string]interface{}{
				"task_id":  taskID,
				"agent_id": agentID,
			},
		})
	}
	return nil
}

// CleanupExpiredClaims releases tasks where the lease has expired.
// Returns the number of tasks released.
func (k *KanbanIntegration) CleanupExpiredClaims() (int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := k.db.Exec(`UPDATE tasks SET claimed_by = '', lease_expires_at = NULL,
		state = 'planned', last_error = 'lease expired'
		WHERE claimed_by != '' AND lease_expires_at IS NOT NULL AND lease_expires_at < ?`, now)
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	if affected > 0 && k.bus != nil {
		k.bus.PublishSystem(bus.SystemEvent{
			Type:   "task.lease_expired",
			Source: "kanban",
			Data:   map[string]interface{}{"count": affected},
		})
	}
	return int(affected), nil
}

// AddNote adds a note to a task.
func (k *KanbanIntegration) AddNote(taskID, content, author string) error {
	_, err := k.db.Exec(
		"INSERT INTO task_notes (task_id, content, author) VALUES (?, ?, ?)",
		taskID, content, author,
	)
	return err
}

// LogEvent records a task event.
func (k *KanbanIntegration) LogEvent(taskID, source, eventType, summary string) error {
	_, err := k.db.Exec(
		"INSERT INTO task_events (task_id, source, event_type, summary) VALUES (?, ?, ?, ?)",
		taskID, source, eventType, summary,
	)
	return err
}

// GetBoardStats returns aggregate stats for the dashboard.
func (k *KanbanIntegration) GetBoardStats() (map[string]int, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	stats := map[string]int{}
	rows, err := k.db.Query("SELECT state, COUNT(*) FROM tasks GROUP BY state")
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	total := 0
	for rows.Next() {
		var state string
		var count int
		rows.Scan(&state, &count)
		stats[state] = count
		total += count
	}
	stats["total"] = total
	return stats, nil
}

// GetCategoryStats returns task counts by category.
func (k *KanbanIntegration) GetCategoryStats() (map[string]int, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	stats := map[string]int{}
	rows, err := k.db.Query("SELECT category, COUNT(*) FROM tasks WHERE state != 'done' GROUP BY category")
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var cat string
		var count int
		rows.Scan(&cat, &count)
		stats[cat] = count
	}
	return stats, nil
}

// TaskFilters holds query parameters for listing tasks.
type TaskFilters struct {
	State       TaskState    `json:"state,omitempty"`
	Category    TaskCategory `json:"category,omitempty"`
	Source      TaskSource   `json:"source,omitempty"`
	Project     string       `json:"project,omitempty"`
	ExcludeDone bool         `json:"exclude_done,omitempty"`
	Limit       int          `json:"limit,omitempty"`
}

// Helper functions

func (k *KanbanIntegration) nextID() (string, error) {
	var maxID sql.NullString
	err := k.db.QueryRow("SELECT id FROM tasks ORDER BY id DESC LIMIT 1").Scan(&maxID)
	if err == sql.ErrNoRows || !maxID.Valid {
		return "TASK-001", nil
	}
	if err != nil {
		return "", err
	}

	// Parse numeric suffix
	num := 0
	fmt.Sscanf(maxID.String, "TASK-%d", &num)
	return fmt.Sprintf("TASK-%03d", num+1), nil
}

func (k *KanbanIntegration) scanTask(row *sql.Row) (*Task, error) {
	task := &Task{}
	var tagsJSON, createdAt, updatedAt, dueDate, leaseExpiresAt sql.NullString
	var llmCategorized int

	err := row.Scan(
		&task.ID, &task.Title, &task.Description,
		&task.State, &task.Category, &task.Source,
		&task.Priority, &tagsJSON,
		&task.Assignee, &task.Project,
		&task.Attempts, &task.LastFailureReason, &task.ExecutionLogURL,
		&task.TelegramMessageID, &task.VSCodeTaskID, &task.ExternalRef,
		&llmCategorized, &task.LLMSummary,
		&task.ClaimedBy, &leaseExpiresAt, &task.ClaimCount, &task.LastError,
		&createdAt, &updatedAt, &dueDate,
	)
	if err != nil {
		return nil, err
	}

	task.LLMCategorized = llmCategorized != 0
	if tagsJSON.Valid {
		json.Unmarshal([]byte(tagsJSON.String), &task.Tags)
	}
	if createdAt.Valid {
		task.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if updatedAt.Valid {
		task.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}
	if dueDate.Valid {
		t, _ := time.Parse(time.RFC3339, dueDate.String)
		task.DueDate = &t
	}
	if leaseExpiresAt.Valid {
		t, _ := time.Parse(time.RFC3339, leaseExpiresAt.String)
		task.LeaseExpiresAt = &t
	}

	return task, nil
}

func (k *KanbanIntegration) scanTaskFromRows(rows *sql.Rows) (*Task, error) {
	task := &Task{}
	var tagsJSON, createdAt, updatedAt, dueDate, leaseExpiresAt sql.NullString
	var llmCategorized int

	err := rows.Scan(
		&task.ID, &task.Title, &task.Description,
		&task.State, &task.Category, &task.Source,
		&task.Priority, &tagsJSON,
		&task.Assignee, &task.Project,
		&task.Attempts, &task.LastFailureReason, &task.ExecutionLogURL,
		&task.TelegramMessageID, &task.VSCodeTaskID, &task.ExternalRef,
		&llmCategorized, &task.LLMSummary,
		&task.ClaimedBy, &leaseExpiresAt, &task.ClaimCount, &task.LastError,
		&createdAt, &updatedAt, &dueDate,
	)
	if err != nil {
		return nil, err
	}

	task.LLMCategorized = llmCategorized != 0
	if tagsJSON.Valid {
		json.Unmarshal([]byte(tagsJSON.String), &task.Tags)
	}
	if createdAt.Valid {
		task.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if updatedAt.Valid {
		task.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}
	if dueDate.Valid {
		t, _ := time.Parse(time.RFC3339, dueDate.String)
		task.DueDate = &t
	}
	if leaseExpiresAt.Valid {
		t, _ := time.Parse(time.RFC3339, leaseExpiresAt.String)
		task.LeaseExpiresAt = &t
	}

	return task, nil
}

func formatOptionalTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
