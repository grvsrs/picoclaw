// Package provider defines the Provider bounded context.
// A Provider represents an LLM backend (Anthropic, OpenAI, etc.)
// that agents use for inference.
package provider

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/domain"
)

// ---------------------------------------------------------------------------
// Provider aggregate root
// ---------------------------------------------------------------------------

// Provider is the aggregate root for LLM provider management.
type Provider struct {
	domain.AggregateRoot

	// Identity
	Name string              `json:"name"`
	Type domain.ProviderType `json:"type"`

	// Configuration (value object)
	Config ProviderConfig `json:"config"`

	// State
	Status    domain.ConnectionStatus `json:"status"`
	Available bool                    `json:"available"`

	// Metrics
	Metrics ProviderMetrics `json:"metrics"`

	// Lifecycle
	CreatedAt domain.Timestamp `json:"created_at"`
	UpdatedAt domain.Timestamp `json:"updated_at"`
}

// NewProvider creates a new Provider aggregate.
func NewProvider(name string, providerType domain.ProviderType, cfg ProviderConfig) *Provider {
	p := &Provider{
		Name:      name,
		Type:      providerType,
		Config:    cfg,
		Status:    domain.StatusIdle,
		Available: true,
		Metrics:   NewProviderMetrics(),
		CreatedAt: domain.Now(),
		UpdatedAt: domain.Now(),
	}
	p.SetID(domain.NewID())
	return p
}

// ---------------------------------------------------------------------------
// Provider behavior
// ---------------------------------------------------------------------------

// MarkAvailable sets the provider as usable.
func (p *Provider) MarkAvailable() {
	p.Available = true
	p.Status = domain.StatusConnected
	p.UpdatedAt = domain.Now()
}

// MarkUnavailable sets the provider as unusable.
func (p *Provider) MarkUnavailable(reason string) {
	p.Available = false
	p.Status = domain.StatusError
	p.Metrics.LastError = reason
	p.UpdatedAt = domain.Now()
}

// RecordRequest tracks a completed LLM request.
func (p *Provider) RecordRequest(promptTokens, completionTokens int, durationMS int64) {
	p.Metrics.RequestCount++
	p.Metrics.PromptTokens += int64(promptTokens)
	p.Metrics.CompletionTokens += int64(completionTokens)
	p.Metrics.TotalDurationMS += durationMS
	p.Metrics.LastRequestAt = domain.Now()
	p.UpdatedAt = domain.Now()
}

// RecordError tracks a failed request.
func (p *Provider) RecordError(err string) {
	p.Metrics.ErrorCount++
	p.Metrics.LastError = err
	p.Metrics.LastErrorAt = domain.Now()
	p.UpdatedAt = domain.Now()
}

// ---------------------------------------------------------------------------
// Value objects
// ---------------------------------------------------------------------------

// ProviderConfig holds provider-specific configuration.
type ProviderConfig struct {
	APIKey     string `json:"api_key"`
	APIBase    string `json:"api_base"`
	AuthMethod string `json:"auth_method,omitempty"`
	Model      string `json:"model,omitempty"`
	OrgID      string `json:"org_id,omitempty"`
}

// ProviderMetrics tracks provider usage statistics.
type ProviderMetrics struct {
	RequestCount     int64            `json:"request_count"`
	ErrorCount       int64            `json:"error_count"`
	PromptTokens     int64            `json:"prompt_tokens"`
	CompletionTokens int64            `json:"completion_tokens"`
	TotalDurationMS  int64            `json:"total_duration_ms"`
	LastRequestAt    domain.Timestamp `json:"last_request_at"`
	LastError        string           `json:"last_error,omitempty"`
	LastErrorAt      domain.Timestamp `json:"last_error_at"`
}

// NewProviderMetrics creates zero-value metrics.
func NewProviderMetrics() ProviderMetrics {
	return ProviderMetrics{}
}

// ---------------------------------------------------------------------------
// LLM interface — the core inference contract
// ---------------------------------------------------------------------------

// ChatMessage represents a message in an LLM conversation.
type ChatMessage struct {
	Role       domain.MessageRole     `json:"role"`
	Content    string                 `json:"content"`
	ToolCalls  []ToolCall             `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents the function name and arguments of a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition defines a tool that can be passed to the LLM.
type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function ToolFunctionDefinition `json:"function"`
}

// ToolFunctionDefinition describes a tool function.
type ToolFunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ChatResponse represents the LLM's response.
type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
	Usage        *UsageInfo `json:"usage,omitempty"`
}

// UsageInfo tracks token consumption.
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LLM defines the inference contract. Infrastructure implements this for each provider.
type LLM interface {
	// Chat sends a conversation to the LLM and returns a response.
	Chat(ctx context.Context, messages []ChatMessage, tools []ToolDefinition, model string, options map[string]interface{}) (*ChatResponse, error)
	// GetDefaultModel returns the default model for this provider.
	GetDefaultModel() string
}

// ---------------------------------------------------------------------------
// Repository interface
// ---------------------------------------------------------------------------

// Repository defines persistence for Provider aggregates.
type Repository interface {
	FindByID(id domain.EntityID) (*Provider, error)
	FindByName(name string) (*Provider, error)
	FindByType(t domain.ProviderType) ([]*Provider, error)
	FindAvailable() ([]*Provider, error)
	FindAll() ([]*Provider, error)
	Save(provider *Provider) error
	Delete(id domain.EntityID) error
}

// ---------------------------------------------------------------------------
// Domain errors
// ---------------------------------------------------------------------------

type ProviderError string

func (e ProviderError) Error() string { return string(e) }

const (
	ErrProviderNotFound    ProviderError = "provider not found"
	ErrProviderUnavailable ProviderError = "provider is unavailable"
	ErrNoAPIKey            ProviderError = "no API key configured"
	ErrInvalidProvider     ProviderError = "invalid provider type"
)
