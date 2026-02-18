package app

import (
	"github.com/sipeed/picoclaw/pkg/domain"
	sessiondomain "github.com/sipeed/picoclaw/pkg/domain/session"
)

// ---------------------------------------------------------------------------
// Session application service
// ---------------------------------------------------------------------------

// SessionService orchestrates session and conversation use cases.
type SessionService struct {
	repo     sessiondomain.Repository
	eventBus domain.EventBus
}

// NewSessionService creates a new session application service.
func NewSessionService(repo sessiondomain.Repository, eventBus domain.EventBus) *SessionService {
	return &SessionService{
		repo:     repo,
		eventBus: eventBus,
	}
}

// GetOrCreateSession retrieves an existing session by key or creates a new one.
func (s *SessionService) GetOrCreateSession(key string, channelType domain.ChannelType, chatID, userID string) (*sessiondomain.Session, error) {
	existing, err := s.repo.FindByKey(key)
	if err == nil {
		return existing, nil
	}

	sess := sessiondomain.NewSession(key, channelType, chatID, userID)
	if err := s.repo.Save(sess); err != nil {
		return nil, err
	}

	s.publishEvents(sess)
	return sess, nil
}

// AddUserMessage appends a user message to a session.
func (s *SessionService) AddUserMessage(sessionID domain.EntityID, content string) error {
	sess, err := s.repo.FindByID(sessionID)
	if err != nil {
		return err
	}

	sess.AddMessage(domain.RoleUser, content)

	if err := s.repo.Save(sess); err != nil {
		return err
	}

	s.publishEvents(sess)
	return nil
}

// AddAssistantMessage appends an assistant message (with optional tool calls).
func (s *SessionService) AddAssistantMessage(sessionID domain.EntityID, content string, toolCalls []sessiondomain.ToolCallInfo) error {
	sess, err := s.repo.FindByID(sessionID)
	if err != nil {
		return err
	}

	if len(toolCalls) > 0 {
		sess.AddAssistantMessageWithTools(content, toolCalls)
	} else {
		sess.AddMessage(domain.RoleAssistant, content)
	}

	if err := s.repo.Save(sess); err != nil {
		return err
	}

	s.publishEvents(sess)
	return nil
}

// AddToolResult appends a tool result message.
func (s *SessionService) AddToolResult(sessionID domain.EntityID, toolName, callID, result string) error {
	sess, err := s.repo.FindByID(sessionID)
	if err != nil {
		return err
	}

	sess.AddToolMessage(toolName, callID, result)
	return s.repo.Save(sess)
}

// SetSummary stores a conversation summary for context-window management.
func (s *SessionService) SetSummary(sessionID domain.EntityID, summary string, upToIndex int) error {
	sess, err := s.repo.FindByID(sessionID)
	if err != nil {
		return err
	}

	sess.SetSummary(summary, upToIndex)
	return s.repo.Save(sess)
}

// TruncateHistory removes older messages beyond maxMessages.
func (s *SessionService) TruncateHistory(sessionID domain.EntityID, maxMessages int) error {
	sess, err := s.repo.FindByID(sessionID)
	if err != nil {
		return err
	}

	sess.TruncateHistory(maxMessages)
	return s.repo.Save(sess)
}

// GetSession retrieves a session by ID.
func (s *SessionService) GetSession(id domain.EntityID) (*sessiondomain.Session, error) {
	return s.repo.FindByID(id)
}

// GetSessionByKey retrieves a session by its unique key.
func (s *SessionService) GetSessionByKey(key string) (*sessiondomain.Session, error) {
	return s.repo.FindByKey(key)
}

// ListSessionsByChannel returns sessions for a given channel type.
func (s *SessionService) ListSessionsByChannel(channelType domain.ChannelType) ([]*sessiondomain.Session, error) {
	return s.repo.FindByChannel(channelType)
}

// ListActiveSessions returns non-archived sessions.
func (s *SessionService) ListActiveSessions() ([]*sessiondomain.Session, error) {
	return s.repo.FindActive()
}

// ArchiveSession archives a session.
func (s *SessionService) ArchiveSession(id domain.EntityID) error {
	sess, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	sess.Archive()
	return s.repo.Save(sess)
}

// PinSession pins an important session.
func (s *SessionService) PinSession(id domain.EntityID) error {
	sess, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	sess.Pin()
	return s.repo.Save(sess)
}

// UnpinSession unpins a session.
func (s *SessionService) UnpinSession(id domain.EntityID) error {
	sess, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	sess.Unpin()
	return s.repo.Save(sess)
}

// DeleteSession soft-deletes a session.
func (s *SessionService) DeleteSession(id domain.EntityID) error {
	sess, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	sess.Delete()

	if err := s.repo.Save(sess); err != nil {
		return err
	}

	s.publishEvents(sess)
	return nil
}

// GetSessionMetrics returns the metrics snapshot for a session.
func (s *SessionService) GetSessionMetrics(id domain.EntityID) (*sessiondomain.SessionMetrics, error) {
	sess, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	m := sess.GetMetrics()
	return &m, nil
}

func (s *SessionService) publishEvents(sess *sessiondomain.Session) {
	events := sess.PullEvents()
	for _, event := range events {
		s.eventBus.Publish(event)
	}
}
