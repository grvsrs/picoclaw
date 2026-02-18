package providers

import (
	"context"
	"fmt"
)

// MoonshotProvider is a provider for Moonshot AI API
// (Chinese LLM provider: https://www.moonshot.cn/)
// Moonshot uses OpenAI-compatible API format
type MoonshotProvider struct {
	httpProvider *HTTPProvider
}

// NewMoonshotProvider creates a new Moonshot provider
func NewMoonshotProvider(apiKey string) *MoonshotProvider {
	return &MoonshotProvider{
		httpProvider: NewHTTPProvider(apiKey, "https://api.moonshot.cn/v1"),
	}
}

// NewMoonshotProviderWithBase creates a new Moonshot provider with custom API base
func NewMoonshotProviderWithBase(apiKey, apiBase string) *MoonshotProvider {
	return &MoonshotProvider{
		httpProvider: NewHTTPProvider(apiKey, apiBase),
	}
}

// Chat sends a request to Moonshot API
func (p *MoonshotProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (*LLMResponse, error) {
	if p.httpProvider == nil {
		return nil, fmt.Errorf("HTTP provider not initialized")
	}

	// Use the HTTP provider's Chat method (Moonshot uses OpenAI-compatible API)
	return p.httpProvider.Chat(ctx, messages, tools, model, options)
}

// GetDefaultModel returns the default Moonshot model
func (p *MoonshotProvider) GetDefaultModel() string {
	return "moonshot-v1-32k"
}

// Ensure MoonshotProvider implements LLMProvider interface
var _ LLMProvider = (*MoonshotProvider)(nil)
