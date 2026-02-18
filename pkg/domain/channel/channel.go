// Package channel defines the Channel bounded context.
// A Channel is an aggregate root representing a messaging transport
// (Telegram, Discord, Slack, etc.) that PicoClaw communicates through.
package channel

import (
	"context"
	"time"

	"github.com/sipeed/picoclaw/pkg/domain"
)

// ---------------------------------------------------------------------------
// Channel aggregate root
// ---------------------------------------------------------------------------

// Channel is the aggregate root for the messaging context.
// It encapsulates identity, connection state, access control, and metrics.
type Channel struct {
	domain.AggregateRoot

	// Identity
	Name        string             `json:"name"`
	Type        domain.ChannelType `json:"type"`
	Description string             `json:"description,omitempty"`

	// State
	Status    domain.ConnectionStatus `json:"status"`
	Enabled   bool                    `json:"enabled"`
	Error     string                  `json:"error,omitempty"`

	// Access control (value object)
	ACL AccessControlList `json:"acl"`

	// Configuration (value object — channel-specific settings)
	Config ChannelConfig `json:"config"`

	// Metrics (value object)
	Metrics ChannelMetrics `json:"metrics"`

	// Lifecycle
	CreatedAt domain.Timestamp `json:"created_at"`
	UpdatedAt domain.Timestamp `json:"updated_at"`
}

// NewChannel creates a new Channel aggregate with a generated ID.
func NewChannel(name string, channelType domain.ChannelType, cfg ChannelConfig) *Channel {
	ch := &Channel{
		Name:      name,
		Type:      channelType,
		Status:    domain.StatusDisconnected,
		Enabled:   false,
		ACL:       NewAccessControlList(nil),
		Config:    cfg,
		Metrics:   NewChannelMetrics(),
		CreatedAt: domain.Now(),
		UpdatedAt: domain.Now(),
	}
	ch.SetID(domain.NewID())
	return ch
}

// ---------------------------------------------------------------------------
// Channel behavior — rich domain methods
// ---------------------------------------------------------------------------

// Enable activates the channel for message processing.
func (ch *Channel) Enable() {
	ch.Enabled = true
	ch.UpdatedAt = domain.Now()
}

// Disable deactivates the channel.
func (ch *Channel) Disable() {
	ch.Enabled = false
	ch.UpdatedAt = domain.Now()
}

// MarkConnected transitions the channel to connected state.
func (ch *Channel) MarkConnected() {
	ch.Status = domain.StatusConnected
	ch.Error = ""
	ch.UpdatedAt = domain.Now()
	ch.RecordEvent(domain.NewEvent(domain.EventChannelConnected, ch.ID(), map[string]string{
		"channel": ch.Name,
		"type":    string(ch.Type),
	}))
}

// MarkDisconnected transitions the channel to disconnected state.
func (ch *Channel) MarkDisconnected() {
	ch.Status = domain.StatusDisconnected
	ch.UpdatedAt = domain.Now()
	ch.RecordEvent(domain.NewEvent(domain.EventChannelDisconnected, ch.ID(), map[string]string{
		"channel": ch.Name,
	}))
}

// MarkError records an error state on the channel.
func (ch *Channel) MarkError(err string) {
	ch.Status = domain.StatusError
	ch.Error = err
	ch.UpdatedAt = domain.Now()
	ch.RecordEvent(domain.NewEvent(domain.EventChannelError, ch.ID(), map[string]string{
		"channel": ch.Name,
		"error":   err,
	}))
}

// RecordMessageSent increments the outbound message counter.
func (ch *Channel) RecordMessageSent() {
	ch.Metrics.MessagesSent++
	ch.Metrics.LastActivityAt = domain.Now()
	ch.UpdatedAt = domain.Now()
}

// RecordMessageReceived increments the inbound message counter.
func (ch *Channel) RecordMessageReceived() {
	ch.Metrics.MessagesReceived++
	ch.Metrics.LastActivityAt = domain.Now()
	ch.UpdatedAt = domain.Now()
}

// IsAllowed checks if a sender is permitted by the access control list.
func (ch *Channel) IsAllowed(senderID string) bool {
	return ch.ACL.IsAllowed(senderID)
}

// ---------------------------------------------------------------------------
// Value objects
// ---------------------------------------------------------------------------

// AccessControlList controls who can interact through a channel.
type AccessControlList struct {
	AllowList []string `json:"allow_list"`
}

// NewAccessControlList creates an ACL from a whitelist.
func NewAccessControlList(allowList []string) AccessControlList {
	if allowList == nil {
		allowList = []string{}
	}
	return AccessControlList{AllowList: allowList}
}

// IsAllowed returns true if the sender is in the allow list, or if the list is empty (open).
func (acl AccessControlList) IsAllowed(senderID string) bool {
	if len(acl.AllowList) == 0 {
		return true
	}
	for _, allowed := range acl.AllowList {
		if allowed == senderID {
			return true
		}
	}
	return false
}

// ChannelConfig holds channel-specific configuration as a flexible map.
// Each channel type interprets its own keys (token, host, port, etc.)
type ChannelConfig struct {
	Values map[string]interface{} `json:"values,omitempty"`
}

// NewChannelConfig creates a typed channel configuration.
func NewChannelConfig(values map[string]interface{}) ChannelConfig {
	if values == nil {
		values = make(map[string]interface{})
	}
	return ChannelConfig{Values: values}
}

// Get retrieves a configuration value.
func (cc ChannelConfig) Get(key string) (interface{}, bool) {
	v, ok := cc.Values[key]
	return v, ok
}

// GetString retrieves a string configuration value.
func (cc ChannelConfig) GetString(key string) string {
	v, ok := cc.Values[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// GetInt retrieves an integer configuration value.
func (cc ChannelConfig) GetInt(key string) int {
	v, ok := cc.Values[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

// ChannelMetrics tracks channel usage statistics.
type ChannelMetrics struct {
	MessagesReceived int64            `json:"messages_received"`
	MessagesSent     int64            `json:"messages_sent"`
	ErrorCount       int64            `json:"error_count"`
	LastActivityAt   domain.Timestamp `json:"last_activity_at"`
	ConnectedSince   domain.Timestamp `json:"connected_since"`
}

// NewChannelMetrics creates zero-value metrics.
func NewChannelMetrics() ChannelMetrics {
	return ChannelMetrics{}
}

// ---------------------------------------------------------------------------
// Message value object — represents a single message in the channel context
// ---------------------------------------------------------------------------

// Message represents a message flowing through a channel.
// This is a value object — immutable once created.
type Message struct {
	ID         domain.EntityID    `json:"id"`
	ChannelID  domain.EntityID    `json:"channel_id"`
	SenderID   string             `json:"sender_id"`
	ChatID     string             `json:"chat_id"`
	Content    string             `json:"content"`
	Media      []MediaAttachment  `json:"media,omitempty"`
	Direction  MessageDirection   `json:"direction"`
	Metadata   domain.Metadata    `json:"metadata,omitempty"`
	Timestamp  domain.Timestamp   `json:"timestamp"`
}

// MessageDirection indicates message flow.
type MessageDirection string

const (
	DirectionInbound  MessageDirection = "inbound"
	DirectionOutbound MessageDirection = "outbound"
)

// MediaAttachment represents a file or media item attached to a message.
type MediaAttachment struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

// NewInboundMessage creates an inbound message value object.
func NewInboundMessage(channelID domain.EntityID, senderID, chatID, content string, media []MediaAttachment) Message {
	return Message{
		ID:        domain.NewID(),
		ChannelID: channelID,
		SenderID:  senderID,
		ChatID:    chatID,
		Content:   content,
		Media:     media,
		Direction: DirectionInbound,
		Metadata:  make(domain.Metadata),
		Timestamp: domain.Now(),
	}
}

// NewOutboundMessage creates an outbound message value object.
func NewOutboundMessage(channelID domain.EntityID, chatID, content string) Message {
	return Message{
		ID:        domain.NewID(),
		ChannelID: channelID,
		ChatID:    chatID,
		Content:   content,
		Direction: DirectionOutbound,
		Metadata:  make(domain.Metadata),
		Timestamp: domain.Now(),
	}
}

// ---------------------------------------------------------------------------
// Channel transport interface — infrastructure contract
// ---------------------------------------------------------------------------

// Transport defines the infrastructure-level operations for a channel.
// This lives in the domain as a port; implementations are in infrastructure.
type Transport interface {
	// Connect establishes the transport connection.
	Connect(ctx context.Context) error
	// Disconnect tears down the transport connection.
	Disconnect(ctx context.Context) error
	// Send delivers a message through the transport.
	Send(ctx context.Context, msg Message) error
	// OnReceive registers a callback for incoming messages.
	OnReceive(handler func(msg Message))
	// IsConnected returns the current connection state.
	IsConnected() bool
}

// ---------------------------------------------------------------------------
// Repository interface — persistence port
// ---------------------------------------------------------------------------

// Repository defines persistence operations for Channel aggregates.
type Repository interface {
	FindByID(id domain.EntityID) (*Channel, error)
	FindByName(name string) (*Channel, error)
	FindByType(channelType domain.ChannelType) ([]*Channel, error)
	FindEnabled() ([]*Channel, error)
	FindAll() ([]*Channel, error)
	Save(ch *Channel) error
	Delete(id domain.EntityID) error
}

// ---------------------------------------------------------------------------
// Service interface — application service port
// ---------------------------------------------------------------------------

// Service defines the application-level operations for channel management.
type Service interface {
	// RegisterChannel creates and persists a new channel.
	RegisterChannel(name string, channelType domain.ChannelType, cfg ChannelConfig) (*Channel, error)
	// EnableChannel activates a channel.
	EnableChannel(id domain.EntityID) error
	// DisableChannel deactivates a channel.
	DisableChannel(id domain.EntityID) error
	// ConnectChannel starts the transport.
	ConnectChannel(ctx context.Context, id domain.EntityID) error
	// DisconnectChannel stops the transport.
	DisconnectChannel(ctx context.Context, id domain.EntityID) error
	// SendMessage delivers a message through a channel.
	SendMessage(ctx context.Context, channelID domain.EntityID, chatID, content string) error
	// GetChannel retrieves channel details.
	GetChannel(id domain.EntityID) (*Channel, error)
	// ListChannels returns all registered channels.
	ListChannels() ([]*Channel, error)
	// RemoveChannel unregisters and deletes a channel.
	RemoveChannel(id domain.EntityID) error
	// GetStatus returns the current status of all channels.
	GetStatus() map[string]interface{}
}

// ---------------------------------------------------------------------------
// Factory — creates channels with validation
// ---------------------------------------------------------------------------

// Factory creates Channel aggregates with invariant validation.
type Factory struct{}

// CreateChannel validates inputs and constructs a new Channel aggregate.
func (f Factory) CreateChannel(name string, channelType domain.ChannelType, cfg ChannelConfig, allowList []string) (*Channel, error) {
	if name == "" {
		return nil, ErrEmptyName
	}
	if !channelType.Valid() {
		return nil, ErrInvalidChannelType
	}

	ch := &Channel{
		Name:      name,
		Type:      channelType,
		Status:    domain.StatusDisconnected,
		Enabled:   false,
		ACL:       NewAccessControlList(allowList),
		Config:    cfg,
		Metrics:   NewChannelMetrics(),
		CreatedAt: domain.Now(),
		UpdatedAt: domain.Now(),
	}
	ch.SetID(domain.NewID())
	return ch, nil
}

// Reconstitute rebuilds a Channel from persisted data (no validation, no events).
func (f Factory) Reconstitute(id domain.EntityID, name string, channelType domain.ChannelType, status domain.ConnectionStatus, enabled bool, createdAt, updatedAt time.Time) *Channel {
	ch := &Channel{
		Name:      name,
		Type:      channelType,
		Status:    status,
		Enabled:   enabled,
		CreatedAt: domain.TimestampFrom(createdAt),
		UpdatedAt: domain.TimestampFrom(updatedAt),
	}
	ch.SetID(id)
	return ch
}

// ---------------------------------------------------------------------------
// Domain errors
// ---------------------------------------------------------------------------

// ChannelError is a typed error for the channel domain.
type ChannelError string

func (e ChannelError) Error() string { return string(e) }

const (
	ErrEmptyName          ChannelError = "channel name cannot be empty"
	ErrInvalidChannelType ChannelError = "invalid channel type"
	ErrNotFound           ChannelError = "channel not found"
	ErrAlreadyConnected   ChannelError = "channel already connected"
	ErrNotConnected       ChannelError = "channel not connected"
	ErrNotEnabled         ChannelError = "channel is not enabled"
	ErrSenderNotAllowed   ChannelError = "sender not in allow list"
)
