// Package integration provides an extensible plugin registry for connecting
// picoclaw to external services and applications.
//
// To create a new integration:
//  1. Implement the Integration interface
//  2. Register it with the global registry via Register()
//  3. Start it alongside the application via StartAll()
//
// This is the central extension point for adding new API connections to
// external programs built in parallel (e.g., task boards, code tools, etc.).
package integration

import (
	"context"
	"fmt"
	"sync"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// Integration represents a pluggable external service connection.
// Each integration can provide tools, consume events, and expose API endpoints.
type Integration interface {
	// Name returns a unique identifier for this integration.
	Name() string

	// Init sets up the integration with the shared config and message bus.
	Init(cfg *config.Config, bus *bus.MessageBus) error

	// Start begins the integration's event loop (non-blocking).
	Start(ctx context.Context) error

	// Stop gracefully shuts down the integration.
	Stop(ctx context.Context) error

	// Health returns nil if healthy, or an error describing the problem.
	Health() error
}

// APIIntegration extends Integration for services that expose HTTP endpoints.
type APIIntegration interface {
	Integration

	// Routes returns a map of path -> http handler to be mounted on the API server.
	// Example: {"/api/ext/kanban/cards": handlerFunc}
	Routes() map[string]HTTPHandler
}

// HTTPHandler is a simplified HTTP handler type for integration routes.
type HTTPHandler struct {
	Method  string // GET, POST, PUT, DELETE
	Handler func(ctx context.Context, body []byte) (interface{}, error)
}

// EventConsumer extends Integration for services that subscribe to bus events.
type EventConsumer interface {
	Integration

	// EventTypes returns the event types this integration subscribes to.
	EventTypes() []string

	// HandleEvent processes an event from the message bus.
	HandleEvent(ctx context.Context, eventType string, data map[string]interface{}) error
}

// ToolProvider extends Integration for services that provide executable tools
// to the AI agent.
type ToolProvider interface {
	Integration

	// Tools returns tool definitions this integration provides.
	Tools() []ToolInfo
}

// ToolInfo describes a tool exposed by an integration.
type ToolInfo struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	Execute     func(ctx context.Context, args map[string]interface{}) (string, error)
}

// Registry manages all registered integrations.
type Registry struct {
	integrations map[string]Integration
	mu           sync.RWMutex
	started      bool
}

// NewRegistry creates a new integration registry.
func NewRegistry() *Registry {
	return &Registry{
		integrations: make(map[string]Integration),
	}
}

// Global registry instance
var globalRegistry = NewRegistry()

// Register adds an integration to the global registry.
func Register(i Integration) {
	globalRegistry.Register(i)
}

// GetRegistry returns the global registry.
func GetRegistry() *Registry {
	return globalRegistry
}

// Register adds an integration to this registry.
func (r *Registry) Register(i Integration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.integrations[i.Name()] = i
	logger.InfoCF("integration", "Registered integration", map[string]interface{}{
		"name": i.Name(),
	})
}

// Get retrieves an integration by name.
func (r *Registry) Get(name string) (Integration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	i, ok := r.integrations[name]
	return i, ok
}

// List returns all registered integration names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.integrations))
	for name := range r.integrations {
		names = append(names, name)
	}
	return names
}

// InitAll initializes all registered integrations.
func (r *Registry) InitAll(cfg *config.Config, msgBus *bus.MessageBus) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, i := range r.integrations {
		if err := i.Init(cfg, msgBus); err != nil {
			logger.ErrorCF("integration", "Failed to init integration", map[string]interface{}{
				"name":  name,
				"error": err.Error(),
			})
			return fmt.Errorf("init integration %s: %w", name, err)
		}
	}
	return nil
}

// StartAll starts all registered integrations.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, i := range r.integrations {
		if err := i.Start(ctx); err != nil {
			logger.ErrorCF("integration", "Failed to start integration", map[string]interface{}{
				"name":  name,
				"error": err.Error(),
			})
			return fmt.Errorf("start integration %s: %w", name, err)
		}
		logger.InfoCF("integration", "Started integration", map[string]interface{}{
			"name": name,
		})
	}
	r.started = true
	return nil
}

// StopAll gracefully stops all integrations.
func (r *Registry) StopAll(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, i := range r.integrations {
		if err := i.Stop(ctx); err != nil {
			logger.ErrorCF("integration", "Failed to stop integration", map[string]interface{}{
				"name":  name,
				"error": err.Error(),
			})
		}
	}
	r.started = false
}

// HealthAll returns a map of integration name → health status.
func (r *Registry) HealthAll() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	status := make(map[string]string, len(r.integrations))
	for name, i := range r.integrations {
		if err := i.Health(); err != nil {
			status[name] = err.Error()
		} else {
			status[name] = "ok"
		}
	}
	return status
}

// GetAllRoutes collects HTTP routes from all APIIntegration instances.
func (r *Registry) GetAllRoutes() map[string]HTTPHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	routes := make(map[string]HTTPHandler)
	for _, i := range r.integrations {
		if api, ok := i.(APIIntegration); ok {
			for path, handler := range api.Routes() {
				routes[path] = handler
			}
		}
	}
	return routes
}

// GetAllTools collects tools from all ToolProvider instances.
func (r *Registry) GetAllTools() []ToolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var tools []ToolInfo
	for _, i := range r.integrations {
		if tp, ok := i.(ToolProvider); ok {
			tools = append(tools, tp.Tools()...)
		}
	}
	return tools
}
