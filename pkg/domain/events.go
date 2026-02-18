package domain

import "time"

// ---------------------------------------------------------------------------
// Domain event system — the backbone of DDD communication
// ---------------------------------------------------------------------------

// EventType classifies domain events for routing and filtering.
type EventType string

// Bounded context prefixes ensure global uniqueness of event names.
const (
	// Channel context events
	EventChannelConnected    EventType = "channel.connected"
	EventChannelDisconnected EventType = "channel.disconnected"
	EventChannelError        EventType = "channel.error"
	EventMessageReceived     EventType = "channel.message.received"
	EventMessageSent         EventType = "channel.message.sent"
	EventMessageFailed       EventType = "channel.message.failed"

	// Agent context events
	EventAgentStarted        EventType = "agent.started"
	EventAgentStopped        EventType = "agent.stopped"
	EventAgentThinking       EventType = "agent.thinking"
	EventAgentResponded      EventType = "agent.responded"
	EventAgentError          EventType = "agent.error"
	EventToolExecutionStart  EventType = "agent.tool.start"
	EventToolExecutionEnd    EventType = "agent.tool.end"

	// Session context events
	EventSessionCreated      EventType = "session.created"
	EventSessionUpdated      EventType = "session.updated"
	EventSessionDeleted      EventType = "session.deleted"
	EventSessionSummarized   EventType = "session.summarized"

	// Skill context events
	EventSkillInstalled      EventType = "skill.installed"
	EventSkillUninstalled    EventType = "skill.uninstalled"
	EventSkillExecuted       EventType = "skill.executed"
	EventSkillError          EventType = "skill.error"

	// Workflow context events
	EventWorkflowCreated     EventType = "workflow.created"
	EventWorkflowStarted     EventType = "workflow.started"
	EventWorkflowStepDone    EventType = "workflow.step.done"
	EventWorkflowCompleted   EventType = "workflow.completed"
	EventWorkflowFailed      EventType = "workflow.failed"

	// Cron context events
	EventCronJobCreated      EventType = "cron.job.created"
	EventCronJobTriggered    EventType = "cron.job.triggered"
	EventCronJobCompleted    EventType = "cron.job.completed"
	EventCronJobFailed       EventType = "cron.job.failed"
	EventCronJobRemoved      EventType = "cron.job.removed"

	// Provider context events
	EventProviderRequest     EventType = "provider.request"
	EventProviderResponse    EventType = "provider.response"
	EventProviderError       EventType = "provider.error"

	// System-level events
	EventSystemStartup       EventType = "system.startup"
	EventSystemShutdown      EventType = "system.shutdown"
	EventSystemHealthCheck   EventType = "system.health"
)

// Event is the interface all domain events implement.
type Event interface {
	// EventType returns the classified event type.
	EventType() EventType
	// OccurredAt returns when the event happened.
	OccurredAt() time.Time
	// AggregateID returns the ID of the aggregate that produced this event.
	AggregateID() EntityID
	// Payload returns the event-specific data.
	Payload() interface{}
}

// BaseEvent provides a reusable implementation of the Event interface.
type BaseEvent struct {
	Type        EventType   `json:"type"`
	Timestamp   time.Time   `json:"timestamp"`
	AggID       EntityID    `json:"aggregate_id"`
	EventData   interface{} `json:"data,omitempty"`
}

func (e BaseEvent) EventType() EventType    { return e.Type }
func (e BaseEvent) OccurredAt() time.Time   { return e.Timestamp }
func (e BaseEvent) AggregateID() EntityID   { return e.AggID }
func (e BaseEvent) Payload() interface{}    { return e.EventData }

// NewEvent creates a new domain event.
func NewEvent(eventType EventType, aggregateID EntityID, data interface{}) BaseEvent {
	return BaseEvent{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		AggID:     aggregateID,
		EventData: data,
	}
}

// ---------------------------------------------------------------------------
// Event bus — decoupled cross-context communication
// ---------------------------------------------------------------------------

// EventHandler processes a domain event. Handlers should be idempotent.
type EventHandler func(Event)

// EventBus dispatches domain events to registered handlers.
// This is the anti-corruption layer between bounded contexts.
type EventBus interface {
	// Publish dispatches an event to all registered handlers.
	Publish(event Event)
	// Subscribe registers a handler for a specific event type.
	Subscribe(eventType EventType, handler EventHandler)
	// SubscribeAll registers a handler that receives every event.
	SubscribeAll(handler EventHandler)
	// Close shuts down the event bus.
	Close()
}
