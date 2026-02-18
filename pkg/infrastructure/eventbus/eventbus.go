// Package eventbus provides the in-process implementation of the domain event bus.
// This is the infrastructure adapter for domain.EventBus.
package eventbus

import (
	"sync"

	"github.com/sipeed/picoclaw/pkg/domain"
)

// InProcessEventBus is a synchronous in-process event bus.
// It dispatches events to registered handlers immediately on Publish().
// For production, this can be swapped for an async/distributed implementation
// (NATS, Redis Streams, etc.) behind the same domain.EventBus interface.
type InProcessEventBus struct {
	handlers    map[domain.EventType][]domain.EventHandler
	allHandlers []domain.EventHandler
	mu          sync.RWMutex
	closed      bool
}

// New creates a new in-process event bus.
func New() *InProcessEventBus {
	return &InProcessEventBus{
		handlers:    make(map[domain.EventType][]domain.EventHandler),
		allHandlers: make([]domain.EventHandler, 0),
	}
}

// Publish dispatches an event to all matching handlers.
// Handlers for the specific event type are called first, then global handlers.
func (b *InProcessEventBus) Publish(event domain.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	// Typed handlers
	if handlers, ok := b.handlers[event.EventType()]; ok {
		for _, handler := range handlers {
			handler(event)
		}
	}

	// Global handlers
	for _, handler := range b.allHandlers {
		handler(event)
	}
}

// Subscribe registers a handler for a specific event type.
func (b *InProcessEventBus) Subscribe(eventType domain.EventType, handler domain.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// SubscribeAll registers a handler that receives every event.
func (b *InProcessEventBus) SubscribeAll(handler domain.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.allHandlers = append(b.allHandlers, handler)
}

// Close marks the bus as closed. No more events will be dispatched.
func (b *InProcessEventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
}

// PublishAll dispatches multiple events (e.g., from AggregateRoot.PullEvents).
func (b *InProcessEventBus) PublishAll(events []domain.Event) {
	for _, event := range events {
		b.Publish(event)
	}
}

// HandlerCount returns the total number of registered handlers (for diagnostics).
func (b *InProcessEventBus) HandlerCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := len(b.allHandlers)
	for _, handlers := range b.handlers {
		count += len(handlers)
	}
	return count
}

// Verify interface compliance at compile time.
var _ domain.EventBus = (*InProcessEventBus)(nil)
