package tools

import (
	"github.com/sipeed/picoclaw/pkg/providers"
)

// Re-export provider types to maintain backward compatibility.
// All canonical type definitions live in pkg/providers/types.go.
type Message = providers.Message
type ToolCall = providers.ToolCall
type FunctionCall = providers.FunctionCall
type LLMResponse = providers.LLMResponse
type UsageInfo = providers.UsageInfo
type LLMProvider = providers.LLMProvider
type ToolDefinition = providers.ToolDefinition
type ToolFunctionDefinition = providers.ToolFunctionDefinition
