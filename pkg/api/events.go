// Event bridge — wires the message bus into the WebSocket hub for real-time
// dashboard updates. Every inbound/outbound message and system event fans out
// to all connected WebSocket clients via bus tap subscriptions.
package api

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// EventBridge connects the message bus to the WebSocket hub for live updates.
type EventBridge struct {
	bus *bus.MessageBus
	hub *WSHub
}

// NewEventBridge creates a bridge that forwards bus events to WebSocket clients.
func NewEventBridge(mb *bus.MessageBus, hub *WSHub) *EventBridge {
	return &EventBridge{bus: mb, hub: hub}
}

// Run starts forwarding loops using fan-out taps on the message bus.
// Call this in a goroutine — it blocks until ctx is cancelled.
func (eb *EventBridge) Run(ctx context.Context) {
	logger.InfoC("events", "Event bridge started — forwarding bus events to WebSocket")

	// Subscribe to fan-out taps — these receive copies of all published messages
	// without stealing from the primary consumer (agent loop / channel dispatch).
	inboundTap := eb.bus.SubscribeInboundTap("event-bridge")
	outboundTap := eb.bus.SubscribeOutboundTap("event-bridge")
	systemTap := eb.bus.SubscribeSystem("event-bridge")

	go eb.forwardInbound(ctx, inboundTap)
	go eb.forwardOutbound(ctx, outboundTap)
	go eb.forwardSystem(ctx, systemTap)
}

func (eb *EventBridge) forwardInbound(ctx context.Context, tap <-chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			logger.InfoC("events", "Inbound event bridge stopped")
			return
		case raw, ok := <-tap:
			if !ok {
				return
			}
			if msg, ok := raw.(bus.InboundMessage); ok {
				eb.hub.Broadcast("message.inbound", map[string]interface{}{
					"channel":     msg.Channel,
					"sender_id":   msg.SenderID,
					"chat_id":     msg.ChatID,
					"content":     truncate(msg.Content, 200),
					"session_key": msg.SessionKey,
				})
			}
		}
	}
}

func (eb *EventBridge) forwardOutbound(ctx context.Context, tap <-chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			logger.InfoC("events", "Outbound event bridge stopped")
			return
		case raw, ok := <-tap:
			if !ok {
				return
			}
			if msg, ok := raw.(bus.OutboundMessage); ok {
				eb.hub.Broadcast("message.outbound", map[string]interface{}{
					"channel": msg.Channel,
					"chat_id": msg.ChatID,
					"content": truncate(msg.Content, 200),
				})
			}
		}
	}
}

func (eb *EventBridge) forwardSystem(ctx context.Context, tap <-chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			logger.InfoC("events", "System event bridge stopped")
			return
		case raw, ok := <-tap:
			if !ok {
				return
			}
			if evt, ok := raw.(bus.SystemEvent); ok {
				eb.hub.Broadcast(evt.Type, evt.Data)
			}
		}
	}
}

// BroadcastSystemEvent is a convenience for direct broadcast (bypass bus).
func (eb *EventBridge) BroadcastSystemEvent(eventType string, data map[string]interface{}) {
	eb.hub.Broadcast(eventType, data)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
