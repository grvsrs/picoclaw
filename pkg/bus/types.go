package bus

type InboundMessage struct {
	Channel    string            `json:"channel"`
	SenderID   string            `json:"sender_id"`
	ChatID     string            `json:"chat_id"`
	Content    string            `json:"content"`
	Media      []string          `json:"media,omitempty"`
	SessionKey string            `json:"session_key"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type OutboundMessage struct {
	Channel string `json:"channel"`
	ChatID  string `json:"chat_id"`
	Content string `json:"content"`
}

// SystemEvent is a typed event flowing through the bus for observability.
// Used for task lifecycle, bot lifecycle, diff events, etc.
type SystemEvent struct {
	Type   string      `json:"type"`   // e.g. "task.created", "bot.started"
	Source string      `json:"source"` // e.g. "kanban", "orchestrator"
	Data   interface{} `json:"data"`
}

type MessageHandler func(InboundMessage) error
