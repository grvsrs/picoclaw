package providers

import (
	"context"
	"testing"
)

// TestMoonshotProviderCreation verifies Moonshot provider can be created
func TestMoonshotProviderCreation(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		wantError bool
	}{
		{
			name:   "valid API key",
			apiKey: "sk-test-key-12345",
		},
		{
			name:   "empty API key",
			apiKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMoonshotProvider(tt.apiKey)
			if provider == nil {
				t.Fatal("expected non-nil provider")
			}

			defaultModel := provider.GetDefaultModel()
			if defaultModel != "moonshot-v1-32k" {
				t.Errorf("expected default model moonshot-v1-32k, got %s", defaultModel)
			}
		})
	}
}

// TestMoonshotProviderWithCustomBase verifies custom API base can be set
func TestMoonshotProviderWithCustomBase(t *testing.T) {
	customBase := "https://custom.moonshot.cn/v1"
	provider := NewMoonshotProviderWithBase("sk-test", customBase)

	if provider == nil {
		t.Fatal("expected non-nil provider")
	}

	// Verify provider was created (httpProvider is private, so just verify creation succeeds)
	if provider.httpProvider == nil {
		t.Errorf("expected non-nil httpProvider")
	}
}

// TestMoonshotProviderImplementsInterface verifies interface compliance
func TestMoonshotProviderImplementsInterface(t *testing.T) {
	var _ LLMProvider = (*MoonshotProvider)(nil)
	t.Log("✓ MoonshotProvider implements LLMProvider interface")
}

// TestMoonshotChatSignature verifies Chat method signature
func TestMoonshotChatSignature(t *testing.T) {
	ctx := context.Background()
	provider := NewMoonshotProvider("sk-test")

	// This will fail due to invalid API key, but verifies the method exists and signature is correct
	_, err := provider.Chat(ctx, []Message{}, []ToolDefinition{}, "moonshot-v1-32k", nil)

	// We expect an error since we're not calling a real API, but we're checking the method exists
	if err == nil {
		t.Log("Chat method exists and is callable")
	} else {
		t.Logf("Chat method test: %v (expected, not calling real API)", err)
	}
}
