// Webhook API endpoints — accept events from local programs and external sources
package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/domain"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// POST /api/webhook/:source — accept an event from a local program or webhook source
//
// Request body can be either:
//
// 1. A complete domain event (with type, aggregate_id, data):
//      {
//        "type": "workflow.triggered",
//        "aggregate_id": "my-workflow",
//        "data": { "param": "value" }
//      }
//
// 2. A simple payload (will be wrapped as an event with type=webhook.{source}):
//      {
//        "message": "Build completed",
//        "status": "success"
//      }
//
// The webhook source name (from URL) becomes the aggregate_id and event categorization.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodOptions {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if r.Method == http.MethodOptions {
		w.Header().Set("Allow", "POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Extract the source name from the URL path (/api/webhook/{source})
	source := r.PathValue("source")
	if source == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "webhook source name required"})
		return
	}

	// Parse incoming payload
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	if len(payload) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty payload"})
		return
	}

	var event domain.Event

	// Check if payload contains domain event fields
	if _, hasType := payload["type"]; hasType {
		// Try to unmarshal as a complete domain event
		rawBytes, _ := json.Marshal(payload)
		var baseEvent domain.BaseEvent
		if err := json.Unmarshal(rawBytes, &baseEvent); err == nil {
			// Validate extracted type is a valid EventType
			if baseEvent.Type != "" {
				event = baseEvent
			}
		}
	}

	// If not a complete domain event, wrap the payload
	if event == nil {
		eventType := domain.EventType(fmt.Sprintf("webhook.%s", source))
		event = domain.NewEvent(eventType, domain.EntityID(source), payload)
	}

	// Convert domain event to system event and publish
	var sysEvent bus.SystemEvent
	if event != nil {
		sysEvent = bus.SystemEvent{
			Type:   string(event.EventType()),
			Source: source,
			Data:   event.Payload(),
		}
	}

	if s.messageBus != nil {
		s.messageBus.PublishSystem(sysEvent)
		logger.InfoCF("webhook", "Event received and published",
			map[string]interface{}{
				"source": source,
				"type":   event.EventType(),
				"aggregate_id": event.AggregateID(),
			},
		)
	} else {
		logger.WarnC("webhook", "Message bus not available, event not published")
	}

	// Return success
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"message": fmt.Sprintf("webhook from %s accepted", source),
		"event_type": event.EventType(),
		"aggregate_id": event.AggregateID(),
	})
}
