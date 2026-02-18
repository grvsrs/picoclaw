// Package persistence provides repository implementations backed by the filesystem.
// These are the infrastructure adapters for domain repository interfaces.
package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sipeed/picoclaw/pkg/domain"
	agentdomain "github.com/sipeed/picoclaw/pkg/domain/agent"
	channeldomain "github.com/sipeed/picoclaw/pkg/domain/channel"
	skilldomain "github.com/sipeed/picoclaw/pkg/domain/skill"
	sessiondomain "github.com/sipeed/picoclaw/pkg/domain/session"
	workflowdomain "github.com/sipeed/picoclaw/pkg/domain/workflow"
)

// ---------------------------------------------------------------------------
// Generic JSON file store — reusable building block
// ---------------------------------------------------------------------------

// JSONStore provides generic JSON file-based persistence for any serializable type.
// It keeps an in-memory cache and persists to disk on every Save/Delete.
type JSONStore[T any] struct {
	baseDir  string
	items    map[domain.EntityID]*T
	mu       sync.RWMutex
}

// NewJSONStore creates a new file-backed store.
func NewJSONStore[T any](baseDir string) *JSONStore[T] {
	os.MkdirAll(baseDir, 0755)
	return &JSONStore[T]{
		baseDir: baseDir,
		items:   make(map[domain.EntityID]*T),
	}
}

// Load reads all JSON files from the base directory into memory.
func (s *JSONStore[T]) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dir %s: %w", s.baseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.baseDir, entry.Name()))
		if err != nil {
			continue
		}

		var item T
		if err := json.Unmarshal(data, &item); err != nil {
			continue
		}

		// Use filename (without .json) as ID
		id := domain.EntityID(entry.Name()[:len(entry.Name())-5])
		s.items[id] = &item
	}

	return nil
}

// Get retrieves an item by ID.
func (s *JSONStore[T]) Get(id domain.EntityID) (*T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.items[id]
	return item, ok
}

// Put saves an item to memory and disk.
func (s *JSONStore[T]) Put(id domain.EntityID, item *T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[id] = item

	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	path := filepath.Join(s.baseDir, string(id)+".json")
	return os.WriteFile(path, data, 0644)
}

// Remove deletes an item from memory and disk.
func (s *JSONStore[T]) Remove(id domain.EntityID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return false
	}

	delete(s.items, id)
	os.Remove(filepath.Join(s.baseDir, string(id)+".json"))
	return true
}

// All returns all items.
func (s *JSONStore[T]) All() []*T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*T, 0, len(s.items))
	for _, item := range s.items {
		result = append(result, item)
	}
	return result
}

// Count returns the number of stored items.
func (s *JSONStore[T]) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// ---------------------------------------------------------------------------
// Channel repository implementation
// ---------------------------------------------------------------------------

// ChannelRepository is the filesystem-backed implementation of channel.Repository.
type ChannelRepository struct {
	store *JSONStore[channeldomain.Channel]
}

// NewChannelRepository creates a new channel repository.
func NewChannelRepository(baseDir string) *ChannelRepository {
	store := NewJSONStore[channeldomain.Channel](filepath.Join(baseDir, "channels"))
	store.Load()
	return &ChannelRepository{store: store}
}

func (r *ChannelRepository) FindByID(id domain.EntityID) (*channeldomain.Channel, error) {
	ch, ok := r.store.Get(id)
	if !ok {
		return nil, channeldomain.ErrNotFound
	}
	return ch, nil
}

func (r *ChannelRepository) FindByName(name string) (*channeldomain.Channel, error) {
	for _, ch := range r.store.All() {
		if ch.Name == name {
			return ch, nil
		}
	}
	return nil, channeldomain.ErrNotFound
}

func (r *ChannelRepository) FindByType(channelType domain.ChannelType) ([]*channeldomain.Channel, error) {
	var result []*channeldomain.Channel
	for _, ch := range r.store.All() {
		if ch.Type == channelType {
			result = append(result, ch)
		}
	}
	return result, nil
}

func (r *ChannelRepository) FindEnabled() ([]*channeldomain.Channel, error) {
	var result []*channeldomain.Channel
	for _, ch := range r.store.All() {
		if ch.Enabled {
			result = append(result, ch)
		}
	}
	return result, nil
}

func (r *ChannelRepository) FindAll() ([]*channeldomain.Channel, error) {
	return r.store.All(), nil
}

func (r *ChannelRepository) Save(ch *channeldomain.Channel) error {
	return r.store.Put(ch.ID(), ch)
}

func (r *ChannelRepository) Delete(id domain.EntityID) error {
	if !r.store.Remove(id) {
		return channeldomain.ErrNotFound
	}
	return nil
}

// Compile-time verification
var _ channeldomain.Repository = (*ChannelRepository)(nil)

// ---------------------------------------------------------------------------
// Skill repository implementation
// ---------------------------------------------------------------------------

// SkillRepository is the filesystem-backed implementation of skill.Repository.
type SkillRepository struct {
	store *JSONStore[skilldomain.Skill]
}

// NewSkillRepository creates a new skill repository.
func NewSkillRepository(baseDir string) *SkillRepository {
	store := NewJSONStore[skilldomain.Skill](filepath.Join(baseDir, "skills"))
	store.Load()
	return &SkillRepository{store: store}
}

func (r *SkillRepository) FindByID(id domain.EntityID) (*skilldomain.Skill, error) {
	s, ok := r.store.Get(id)
	if !ok {
		return nil, skilldomain.ErrSkillNotFound
	}
	return s, nil
}

func (r *SkillRepository) FindByName(name string) (*skilldomain.Skill, error) {
	for _, s := range r.store.All() {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, skilldomain.ErrSkillNotFound
}

func (r *SkillRepository) FindByCategory(category skilldomain.SkillCategory) ([]*skilldomain.Skill, error) {
	var result []*skilldomain.Skill
	for _, s := range r.store.All() {
		if s.Category == category {
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *SkillRepository) FindByTags(tags domain.Tags) ([]*skilldomain.Skill, error) {
	var result []*skilldomain.Skill
	for _, s := range r.store.All() {
		for _, tag := range tags {
			if s.Tags.Contains(tag) {
				result = append(result, s)
				break
			}
		}
	}
	return result, nil
}

func (r *SkillRepository) FindBySource(source domain.SkillSource) ([]*skilldomain.Skill, error) {
	var result []*skilldomain.Skill
	for _, s := range r.store.All() {
		if s.Source == source {
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *SkillRepository) FindEnabled() ([]*skilldomain.Skill, error) {
	var result []*skilldomain.Skill
	for _, s := range r.store.All() {
		if s.Enabled {
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *SkillRepository) FindAll() ([]*skilldomain.Skill, error) {
	return r.store.All(), nil
}

func (r *SkillRepository) Save(s *skilldomain.Skill) error {
	return r.store.Put(s.ID(), s)
}

func (r *SkillRepository) Delete(id domain.EntityID) error {
	if !r.store.Remove(id) {
		return skilldomain.ErrSkillNotFound
	}
	return nil
}

func (r *SkillRepository) Search(query string) ([]*skilldomain.Skill, error) {
	// Simple substring search across name, description, tags
	var result []*skilldomain.Skill
	for _, s := range r.store.All() {
		if contains(s.Name, query) || contains(s.Description, query) {
			result = append(result, s)
			continue
		}
		for _, tag := range s.Tags {
			if contains(string(tag), query) {
				result = append(result, s)
				break
			}
		}
	}
	return result, nil
}

// Compile-time verification
var _ skilldomain.Repository = (*SkillRepository)(nil)

// ---------------------------------------------------------------------------
// Session repository implementation
// ---------------------------------------------------------------------------

// SessionRepository is the filesystem-backed implementation of session.Repository.
type SessionRepository struct {
	store *JSONStore[sessiondomain.Session]
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(baseDir string) *SessionRepository {
	store := NewJSONStore[sessiondomain.Session](filepath.Join(baseDir, "sessions"))
	store.Load()
	return &SessionRepository{store: store}
}

func (r *SessionRepository) FindByID(id domain.EntityID) (*sessiondomain.Session, error) {
	s, ok := r.store.Get(id)
	if !ok {
		return nil, sessiondomain.ErrSessionNotFound
	}
	return s, nil
}

func (r *SessionRepository) FindByKey(key string) (*sessiondomain.Session, error) {
	for _, s := range r.store.All() {
		if s.Key == key {
			return s, nil
		}
	}
	return nil, sessiondomain.ErrSessionNotFound
}

func (r *SessionRepository) FindByChannel(channelType domain.ChannelType) ([]*sessiondomain.Session, error) {
	var result []*sessiondomain.Session
	for _, s := range r.store.All() {
		if s.ChannelType == channelType {
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *SessionRepository) FindActive() ([]*sessiondomain.Session, error) {
	var result []*sessiondomain.Session
	for _, s := range r.store.All() {
		if s.Status == sessiondomain.SessionActive {
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *SessionRepository) FindAll() ([]*sessiondomain.Session, error) {
	return r.store.All(), nil
}

func (r *SessionRepository) Save(s *sessiondomain.Session) error {
	return r.store.Put(s.ID(), s)
}

func (r *SessionRepository) Delete(id domain.EntityID) error {
	if !r.store.Remove(id) {
		return sessiondomain.ErrSessionNotFound
	}
	return nil
}

// Compile-time verification
var _ sessiondomain.Repository = (*SessionRepository)(nil)

// ---------------------------------------------------------------------------
// Workflow repository implementation
// ---------------------------------------------------------------------------

// WorkflowRepository is the filesystem-backed implementation of workflow.Repository.
type WorkflowRepository struct {
	store *JSONStore[workflowdomain.Workflow]
}

// NewWorkflowRepository creates a new workflow repository.
func NewWorkflowRepository(baseDir string) *WorkflowRepository {
	store := NewJSONStore[workflowdomain.Workflow](filepath.Join(baseDir, "workflows"))
	store.Load()
	return &WorkflowRepository{store: store}
}

func (r *WorkflowRepository) FindByID(id domain.EntityID) (*workflowdomain.Workflow, error) {
	wf, ok := r.store.Get(id)
	if !ok {
		return nil, workflowdomain.ErrWorkflowNotFound
	}
	return wf, nil
}

func (r *WorkflowRepository) FindByName(name string) (*workflowdomain.Workflow, error) {
	for _, wf := range r.store.All() {
		if wf.Name == name {
			return wf, nil
		}
	}
	return nil, workflowdomain.ErrWorkflowNotFound
}

func (r *WorkflowRepository) FindActive() ([]*workflowdomain.Workflow, error) {
	var result []*workflowdomain.Workflow
	for _, wf := range r.store.All() {
		if wf.Status == workflowdomain.StatusActive {
			result = append(result, wf)
		}
	}
	return result, nil
}

func (r *WorkflowRepository) FindAll() ([]*workflowdomain.Workflow, error) {
	return r.store.All(), nil
}

func (r *WorkflowRepository) Save(wf *workflowdomain.Workflow) error {
	return r.store.Put(wf.ID(), wf)
}

func (r *WorkflowRepository) Delete(id domain.EntityID) error {
	if !r.store.Remove(id) {
		return workflowdomain.ErrWorkflowNotFound
	}
	return nil
}

// Compile-time verification
var _ workflowdomain.Repository = (*WorkflowRepository)(nil)

// ---------------------------------------------------------------------------
// Agent repository implementation
// ---------------------------------------------------------------------------

// AgentRepository is the filesystem-backed implementation of agent.Repository.
type AgentRepository struct {
	store *JSONStore[agentdomain.Agent]
}

// NewAgentRepository creates a new agent repository.
func NewAgentRepository(baseDir string) *AgentRepository {
	store := NewJSONStore[agentdomain.Agent](filepath.Join(baseDir, "agents"))
	store.Load()
	return &AgentRepository{store: store}
}

func (r *AgentRepository) FindByID(id domain.EntityID) (*agentdomain.Agent, error) {
	a, ok := r.store.Get(id)
	if !ok {
		return nil, agentdomain.ErrAgentNotFound
	}
	return a, nil
}

func (r *AgentRepository) FindByName(name string) (*agentdomain.Agent, error) {
	for _, a := range r.store.All() {
		if a.Name == name {
			return a, nil
		}
	}
	return nil, agentdomain.ErrAgentNotFound
}

func (r *AgentRepository) FindRunning() (*agentdomain.Agent, error) {
	for _, a := range r.store.All() {
		if a.Status == agentdomain.AgentRunning || a.Status == agentdomain.AgentProcessing {
			return a, nil
		}
	}
	return nil, agentdomain.ErrAgentNotFound
}

func (r *AgentRepository) FindAll() ([]*agentdomain.Agent, error) {
	return r.store.All(), nil
}

func (r *AgentRepository) Save(a *agentdomain.Agent) error {
	return r.store.Put(a.ID(), a)
}

func (r *AgentRepository) Delete(id domain.EntityID) error {
	if !r.store.Remove(id) {
		return agentdomain.ErrAgentNotFound
	}
	return nil
}

// Compile-time verification
var _ agentdomain.Repository = (*AgentRepository)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	return len(haystack) >= len(needle) &&
		(haystack == needle ||
			len(haystack) > 0 && searchSubstring(haystack, needle))
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
