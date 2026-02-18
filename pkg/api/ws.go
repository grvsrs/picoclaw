package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sipeed/picoclaw/pkg/logger"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Same-origin requests have no Origin header
		}
		// Allow localhost origins only
		for _, prefix := range []string{"http://localhost", "http://127.0.0.1", "https://localhost", "https://127.0.0.1"} {
			if len(origin) >= len(prefix) && origin[:len(prefix)] == prefix {
				return true
			}
		}
		logger.WarnCF("ws", "Rejected WebSocket from disallowed origin", map[string]interface{}{"origin": origin})
		return false
	},
}

// WSEvent represents an event sent to WebSocket clients.
type WSEvent struct {
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// WSClient represents a connected WebSocket client.
type WSClient struct {
	conn *websocket.Conn
	send chan []byte
	hub  *WSHub
}

// WSHub manages WebSocket connections and broadcasts events.
type WSHub struct {
	server     *Server
	clients    map[*WSClient]bool
	broadcast  chan WSEvent
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub(server *Server) *WSHub {
	return &WSHub{
		server:     server,
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan WSEvent, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
}

// Run starts the hub's main loop.
func (h *WSHub) Run(ctx context.Context) {
	// Periodic status broadcast
	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			logger.DebugC("ws", "Client connected")

			// Send initial state
			h.sendInitialState(client)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			logger.DebugC("ws", "Client disconnected")

		case event := <-h.broadcast:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- data:
				default:
					// Client too slow, drop
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()

		case <-statusTicker.C:
			h.broadcastStatus()
		}
	}
}

// Broadcast sends an event to all connected clients.
func (h *WSHub) Broadcast(eventType string, data interface{}) {
	event := WSEvent{
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
	select {
	case h.broadcast <- event:
	default:
		// Channel full, drop event
	}
}

// HandleWebSocket handles WebSocket upgrade requests.
// Auth note: this handler is registered inside the authMiddleware-wrapped mux.
// The HTTP upgrade request is authenticated via extractToken(r) which reads
// the Authorization header, X-API-Key header, or ?token= query param.
// No per-message auth is needed — WebSocket connections are stateful.
func (h *WSHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.ErrorCF("ws", "WebSocket upgrade failed", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	client := &WSClient{
		conn: conn,
		send: make(chan []byte, 256),
		hub:  h,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (h *WSHub) sendInitialState(client *WSClient) {
	// Build and send full initial state
	state := map[string]interface{}{
		"uptime_seconds": int(time.Since(h.server.startTime).Seconds()),
	}

	if h.server.agentLoop != nil {
		state["agent"] = map[string]interface{}{
			"running":   h.server.agentLoop.IsRunning(),
			"model":     h.server.agentLoop.GetModel(),
			"workspace": h.server.agentLoop.GetWorkspace(),
		}
		state["tools"] = h.server.agentLoop.GetToolRegistry().List()
		state["sessions"] = len(h.server.agentLoop.GetSessionManager().ListSessions())
	}

	if h.server.channelManager != nil {
		state["channels"] = h.server.channelManager.GetStatus()
	}

	if h.server.cronService != nil {
		state["cron"] = h.server.cronService.Status()
	}

	event := WSEvent{
		Type:      "initial_state",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      state,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	select {
	case client.send <- data:
	default:
	}
}

func (h *WSHub) broadcastStatus() {
	h.mu.RLock()
	clientCount := len(h.clients)
	h.mu.RUnlock()

	if clientCount == 0 {
		return
	}

	status := map[string]interface{}{
		"uptime_seconds": int(time.Since(h.server.startTime).Seconds()),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}

	if h.server.agentLoop != nil {
		status["agent_running"] = h.server.agentLoop.IsRunning()
	}
	if h.server.channelManager != nil {
		status["channels"] = h.server.channelManager.GetStatus()
	}
	if h.server.cronService != nil {
		status["cron"] = h.server.cronService.Status()
	}

	h.Broadcast("status_update", status)
}

// --- Client methods ---

func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Drain queued messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
