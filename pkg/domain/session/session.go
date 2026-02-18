// Package session defines the Session bounded context.
// A Session is an aggregate root representing a conversation thread
// between a user and the PicoClaw agent, with message history and memory.
package session

import (
	"github.com/sipeed/picoclaw/pkg/domain"
)

// ---------------------------------------------------------------------------
// Session aggregate root
// ---------------------------------------------------------------------------

// Session is the aggregate root for conversation management.
// It owns the message history, summary state, and memory references.
type Session struct {
	domain.AggregateRoot

	// Identity
	Key         string             `json:"key"`          // human-readable session key (e.g., "telegram:12345")
	Title       string             `json:"title,omitempty"`
	Description string             `json:"description,omitempty"`

	// Context
	ChannelType domain.ChannelType `json:"channel_type,omitempty"`
	ChatID      string             `json:"chat_id,omitempty"`
	UserID      string             `json:"user_id,omitempty"`

	// Messages (the core data)
	Messages []ConversationMessage `json:"messages"`

	// Summarization state
	Summary       string `json:"summary,omitempty"`
	SummaryIndex  int    `json:"summary_index"` // last message index included in summary

	// State
	Status   SessionStatus    `json:"status"`
	Pinned   bool             `json:"pinned"`

	// Metrics
	Metrics SessionMetrics `json:"metrics"`

	// Lifecycle
	CreatedAt domain.Timestamp `json:"created_at"`
	UpdatedAt domain.Timestamp `json:"updated_at"`
	LastActiveAt domain.Timestamp `json:"last_active_at"`
}

// NewSession creates a new Session aggregate.
func NewSession(key string, channelType domain.ChannelType, chatID, userID string) *Session {
	s := &Session{
		Key:         key,
		ChannelType: channelType,
		ChatID:      chatID,
		UserID:      userID,
		Messages:    make([]ConversationMessage, 0),
		Status:      SessionActive,
		Metrics:     NewSessionMetrics(),
		CreatedAt:   domain.Now(),
		UpdatedAt:   domain.Now(),
		LastActiveAt: domain.Now(),
	}
	s.SetID(domain.NewID())
	return s
}

// ---------------------------------------------------------------------------
// Session behavior
// ---------------------------------------------------------------------------

// AddMessage appends a message to the conversation history.
func (s *Session) AddMessage(role domain.MessageRole, content string) {
	msg := ConversationMessage{
		ID:        domain.NewID(),
		Role:      role,
		Content:   content,
		Timestamp: domain.Now(),
	}
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = domain.Now()
	s.LastActiveAt = domain.Now()
	s.Metrics.MessageCount++

	switch role {
	case domain.RoleUser:
		s.Metrics.UserMessageCount++
	case domain.RoleAssistant:
		s.Metrics.AssistantMessageCount++
	case domain.RoleTool:
		s.Metrics.ToolCallCount++
	}

	s.RecordEvent(domain.NewEvent(domain.EventSessionUpdated, s.ID(), map[string]string{
		"session_key": s.Key,
		"role":        string(role),
	}))
}

// AddToolMessage appends a tool call result message.
func (s *Session) AddToolMessage(toolCallID, toolName, result string) {
	msg := ConversationMessage{
		ID:         domain.NewID(),
		Role:       domain.RoleTool,
		Content:    result,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Timestamp:  domain.Now(),
	}
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = domain.Now()
	s.LastActiveAt = domain.Now()
	s.Metrics.ToolCallCount++
}

// AddAssistantMessageWithTools appends an assistant message that includes tool calls.
func (s *Session) AddAssistantMessageWithTools(content string, toolCalls []ToolCallInfo) {
	msg := ConversationMessage{
		ID:        domain.NewID(),
		Role:      domain.RoleAssistant,
		Content:   content,
		ToolCalls: toolCalls,
		Timestamp: domain.Now(),
	}
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = domain.Now()
	s.LastActiveAt = domain.Now()
	s.Metrics.AssistantMessageCount++
}

// SetSummary updates the conversation summary.
func (s *Session) SetSummary(summary string, upToIndex int) {
	s.Summary = summary
	s.SummaryIndex = upToIndex
	s.UpdatedAt = domain.Now()
	s.RecordEvent(domain.NewEvent(domain.EventSessionSummarized, s.ID(), map[string]string{
		"session_key": s.Key,
	}))
}

// TruncateHistory keeps only the N most recent messages.
func (s *Session) TruncateHistory(keepLast int) {
	if len(s.Messages) <= keepLast {
		return
	}
	s.Messages = s.Messages[len(s.Messages)-keepLast:]
	s.UpdatedAt = domain.Now()
}

// MessageCount returns the total number of messages.
func (s *Session) MessageCount() int {
	return len(s.Messages)
}

// GetHistory returns a copy of all messages.
func (s *Session) GetHistory() []ConversationMessage {
	result := make([]ConversationMessage, len(s.Messages))
	copy(result, s.Messages)
	return result
}

// GetMetrics returns a copy of the session metrics.
func (s *Session) GetMetrics() SessionMetrics {
	return s.Metrics
}

// Archive marks the session as archived.
func (s *Session) Archive() {
	s.Status = SessionArchived
	s.UpdatedAt = domain.Now()
}

// Pin marks the session as pinned (won't be auto-archived).
func (s *Session) Pin() {
	s.Pinned = true
	s.UpdatedAt = domain.Now()
}

// Unpin removes the pinned flag.
func (s *Session) Unpin() {
	s.Pinned = false
	s.UpdatedAt = domain.Now()
}

// Delete records a deletion event.
func (s *Session) Delete() {
	s.Status = SessionDeleted
	s.UpdatedAt = domain.Now()
	s.RecordEvent(domain.NewEvent(domain.EventSessionDeleted, s.ID(), map[string]string{
		"session_key": s.Key,
	}))
}

// ---------------------------------------------------------------------------
// Value objects
// ---------------------------------------------------------------------------

// SessionStatus represents the lifecycle state of a session.
type SessionStatus string

const (
	SessionActive   SessionStatus = "active"
	SessionArchived SessionStatus = "archived"
	SessionDeleted  SessionStatus = "deleted"
)

func (ss SessionStatus) String() string { return string(ss) }

// ConversationMessage represents a single message in the conversation.
// This is a value object — immutable once appended.
type ConversationMessage struct {
	ID         domain.EntityID   `json:"id"`
	Role       domain.MessageRole `json:"role"`
	Content    string            `json:"content"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ToolName   string            `json:"tool_name,omitempty"`
	ToolCalls  []ToolCallInfo    `json:"tool_calls,omitempty"`
	Timestamp  domain.Timestamp  `json:"timestamp"`
}

// ToolCallInfo captures metadata about a tool invocation within a message.
type ToolCallInfo struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// SessionMetrics tracks conversation statistics.
type SessionMetrics struct {
	MessageCount         int   `json:"message_count"`
	UserMessageCount     int   `json:"user_message_count"`
	AssistantMessageCount int  `json:"assistant_message_count"`
	ToolCallCount        int   `json:"tool_call_count"`
	TokensUsed           int64 `json:"tokens_used"`
}

// NewSessionMetrics creates zero-value metrics.
func NewSessionMetrics() SessionMetrics {
	return SessionMetrics{}
}

// ---------------------------------------------------------------------------
// Repository interface
// ---------------------------------------------------------------------------

// Repository defines persistence for Session aggregates.
type Repository interface {
	FindByID(id domain.EntityID) (*Session, error)
	FindByKey(key string) (*Session, error)
	FindByChannel(channelType domain.ChannelType) ([]*Session, error)
	FindActive() ([]*Session, error)
	FindAll() ([]*Session, error)
	Save(session *Session) error
	Delete(id domain.EntityID) error
}

// ---------------------------------------------------------------------------
// Domain errors
// ---------------------------------------------------------------------------

type SessionError string

func (e SessionError) Error() string { return string(e) }

const (
	ErrSessionNotFound SessionError = "session not found"
	ErrEmptyKey        SessionError = "session key cannot be empty"
	ErrSessionArchived SessionError = "session is archived"
)
