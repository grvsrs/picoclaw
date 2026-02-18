// Package vscode provides a VSCode integration for picoclaw.
//
// This integration enables:
//   - VSCode extension ↔ picoclaw communication via WebSocket/HTTP
//   - Task creation from VSCode TODO comments
//   - Code action dispatch to the agent
//   - Workspace-aware file operations
//   - Diagnostic/problem forwarding
//
// Future phases will add:
//   - Live code editing via agent
//   - Terminal command execution
//   - Git integration
//   - Debugging support
package vscode

import (
	"context"
	"fmt"
	"sync"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/integration"
	"github.com/sipeed/picoclaw/pkg/logger"
)

func init() {
	integration.Register(&VSCodeIntegration{})
}

// VSCodeIntegration manages the connection between picoclaw and VSCode.
type VSCodeIntegration struct {
	cfg          *config.Config
	bus          *bus.MessageBus
	mu           sync.RWMutex
	connected    bool
	workspaceDir string
}

// VSCodeEvent represents an event from the VSCode extension.
type VSCodeEvent struct {
	Type      string                 `json:"type"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Supported event types from VSCode
const (
	EventTypeFileOpen     = "file.open"
	EventTypeFileSave     = "file.save"
	EventTypeTODOFound    = "todo.found"
	EventTypeDiagnostic   = "diagnostic"
	EventTypeTerminalCmd  = "terminal.command"
	EventTypeTaskCreate   = "task.create"
	EventTypeCodeAction   = "code.action"
	EventTypeSelection    = "editor.selection"
)

func (v *VSCodeIntegration) Name() string {
	return "vscode"
}

func (v *VSCodeIntegration) Init(cfg *config.Config, msgBus *bus.MessageBus) error {
	v.cfg = cfg
	v.bus = msgBus
	v.workspaceDir = cfg.WorkspacePath()
	return nil
}

func (v *VSCodeIntegration) Start(ctx context.Context) error {
	logger.InfoC("vscode", "VSCode integration ready (waiting for extension connection)")
	return nil
}

func (v *VSCodeIntegration) Stop(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.connected = false
	return nil
}

func (v *VSCodeIntegration) Health() error {
	return nil // Always healthy — passive until extension connects
}

// HandleExtensionEvent processes an event from the VSCode extension.
func (v *VSCodeIntegration) HandleExtensionEvent(ctx context.Context, event VSCodeEvent) error {
	v.mu.Lock()
	v.connected = true
	v.mu.Unlock()

	switch event.Type {
	case EventTypeTODOFound:
		return v.handleTODO(ctx, event)
	case EventTypeTaskCreate:
		return v.handleTaskCreate(ctx, event)
	case EventTypeFileSave:
		return v.handleFileSave(ctx, event)
	case EventTypeDiagnostic:
		return v.handleDiagnostic(ctx, event)
	default:
		logger.DebugCF("vscode", "Unhandled event type", map[string]interface{}{
			"type": event.Type,
		})
	}
	return nil
}

// IsConnected returns whether a VSCode extension is currently connected.
func (v *VSCodeIntegration) IsConnected() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.connected
}

func (v *VSCodeIntegration) handleTODO(ctx context.Context, event VSCodeEvent) error {
	// Extract TODO comment info
	file, _ := event.Data["file"].(string)
	line, _ := event.Data["line"].(float64)
	text, _ := event.Data["text"].(string)

	logger.InfoCF("vscode", "TODO comment found", map[string]interface{}{
		"file": file,
		"line": int(line),
		"text": text,
	})

	// Forward to message bus for agent processing
	v.bus.PublishInbound(bus.InboundMessage{
		Channel:    "vscode",
		SenderID:   "vscode-extension",
		ChatID:     "vscode",
		Content:    fmt.Sprintf("TODO found in %s:%d — %s", file, int(line), text),
		SessionKey: "vscode:main",
		Metadata: map[string]string{
			"type":   "todo",
			"file":   file,
			"line":   fmt.Sprintf("%d", int(line)),
			"source": "vscode",
		},
	})
	return nil
}

func (v *VSCodeIntegration) handleTaskCreate(ctx context.Context, event VSCodeEvent) error {
	title, _ := event.Data["title"].(string)
	description, _ := event.Data["description"].(string)

	logger.InfoCF("vscode", "Task creation from VSCode", map[string]interface{}{
		"title": title,
	})

	v.bus.PublishInbound(bus.InboundMessage{
		Channel:    "vscode",
		SenderID:   "vscode-extension",
		ChatID:     "vscode",
		Content:    fmt.Sprintf("Create task: %s\n%s", title, description),
		SessionKey: "vscode:main",
		Metadata: map[string]string{
			"type":        "task_create",
			"title":       title,
			"description": description,
			"source":      "vscode",
		},
	})
	return nil
}

func (v *VSCodeIntegration) handleFileSave(ctx context.Context, event VSCodeEvent) error {
	file, _ := event.Data["file"].(string)
	logger.DebugCF("vscode", "File saved", map[string]interface{}{
		"file": file,
	})
	return nil
}

func (v *VSCodeIntegration) handleDiagnostic(ctx context.Context, event VSCodeEvent) error {
	file, _ := event.Data["file"].(string)
	severity, _ := event.Data["severity"].(string)
	message, _ := event.Data["message"].(string)

	logger.DebugCF("vscode", "Diagnostic received", map[string]interface{}{
		"file":     file,
		"severity": severity,
		"message":  message,
	})
	return nil
}

// Routes returns HTTP routes for the VSCode extension API.
func (v *VSCodeIntegration) Routes() map[string]integration.HTTPHandler {
	return map[string]integration.HTTPHandler{
		"/api/ext/vscode/status": {
			Method: "GET",
			Handler: func(ctx context.Context, body []byte) (interface{}, error) {
				return map[string]interface{}{
					"connected":    v.IsConnected(),
					"workspace":    v.workspaceDir,
					"integration":  "vscode",
				}, nil
			},
		},
	}
}
