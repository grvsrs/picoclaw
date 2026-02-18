package app

import (
	"context"
	"fmt"

	"github.com/sipeed/picoclaw/pkg/domain"
	channeldomain "github.com/sipeed/picoclaw/pkg/domain/channel"
)

// ---------------------------------------------------------------------------
// Channel application service
// ---------------------------------------------------------------------------

// ChannelService orchestrates channel use cases.
type ChannelService struct {
	repo       channeldomain.Repository
	transports map[domain.EntityID]channeldomain.Transport
	eventBus   domain.EventBus
	factory    channeldomain.Factory
}

// NewChannelService creates a new channel application service.
func NewChannelService(repo channeldomain.Repository, eventBus domain.EventBus) *ChannelService {
	return &ChannelService{
		repo:       repo,
		transports: make(map[domain.EntityID]channeldomain.Transport),
		eventBus:   eventBus,
	}
}

// RegisterChannel creates and persists a new channel.
func (s *ChannelService) RegisterChannel(name string, channelType domain.ChannelType, cfg channeldomain.ChannelConfig, allowList []string) (*channeldomain.Channel, error) {
	// Check for duplicate name
	if existing, _ := s.repo.FindByName(name); existing != nil {
		return nil, fmt.Errorf("channel '%s' already exists", name)
	}

	ch, err := s.factory.CreateChannel(name, channelType, cfg, allowList)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ch); err != nil {
		return nil, fmt.Errorf("save channel: %w", err)
	}

	// Publish domain events
	events := ch.PullEvents()
	for _, event := range events {
		s.eventBus.Publish(event)
	}

	return ch, nil
}

// RegisterTransport associates an infrastructure transport with a channel.
func (s *ChannelService) RegisterTransport(channelID domain.EntityID, transport channeldomain.Transport) {
	s.transports[channelID] = transport
}

// EnableChannel activates a channel.
func (s *ChannelService) EnableChannel(id domain.EntityID) error {
	ch, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	ch.Enable()
	if err := s.repo.Save(ch); err != nil {
		return err
	}

	events := ch.PullEvents()
	for _, event := range events {
		s.eventBus.Publish(event)
	}
	return nil
}

// DisableChannel deactivates a channel.
func (s *ChannelService) DisableChannel(id domain.EntityID) error {
	ch, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	ch.Disable()
	return s.repo.Save(ch)
}

// ConnectChannel starts the transport and updates state.
func (s *ChannelService) ConnectChannel(ctx context.Context, id domain.EntityID) error {
	ch, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	if !ch.Enabled {
		return channeldomain.ErrNotEnabled
	}

	transport, ok := s.transports[id]
	if !ok {
		return fmt.Errorf("no transport registered for channel %s", ch.Name)
	}

	if err := transport.Connect(ctx); err != nil {
		ch.MarkError(err.Error())
		s.repo.Save(ch)
		s.publishEvents(ch)
		return err
	}

	ch.MarkConnected()
	if err := s.repo.Save(ch); err != nil {
		return err
	}
	s.publishEvents(ch)
	return nil
}

// DisconnectChannel stops the transport and updates state.
func (s *ChannelService) DisconnectChannel(ctx context.Context, id domain.EntityID) error {
	ch, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	transport, ok := s.transports[id]
	if ok {
		transport.Disconnect(ctx)
	}

	ch.MarkDisconnected()
	if err := s.repo.Save(ch); err != nil {
		return err
	}
	s.publishEvents(ch)
	return nil
}

// SendMessage delivers a message through a channel.
func (s *ChannelService) SendMessage(ctx context.Context, channelID domain.EntityID, chatID, content string) error {
	ch, err := s.repo.FindByID(channelID)
	if err != nil {
		return err
	}

	transport, ok := s.transports[channelID]
	if !ok {
		return fmt.Errorf("no transport for channel %s", ch.Name)
	}

	msg := channeldomain.NewOutboundMessage(channelID, chatID, content)

	if err := transport.Send(ctx, msg); err != nil {
		ch.MarkError(err.Error())
		s.repo.Save(ch)
		return err
	}

	ch.RecordMessageSent()
	s.repo.Save(ch)
	s.eventBus.Publish(domain.NewEvent(domain.EventMessageSent, channelID, map[string]string{
		"channel": ch.Name,
		"chat_id": chatID,
	}))
	return nil
}

// GetChannel retrieves channel details.
func (s *ChannelService) GetChannel(id domain.EntityID) (*channeldomain.Channel, error) {
	return s.repo.FindByID(id)
}

// ListChannels returns all registered channels.
func (s *ChannelService) ListChannels() ([]*channeldomain.Channel, error) {
	return s.repo.FindAll()
}

// RemoveChannel unregisters and deletes a channel.
func (s *ChannelService) RemoveChannel(id domain.EntityID) error {
	return s.repo.Delete(id)
}

// GetStatus returns the current status of all channels.
func (s *ChannelService) GetStatus() map[string]interface{} {
	channels, _ := s.repo.FindAll()
	status := make(map[string]interface{})
	for _, ch := range channels {
		status[ch.Name] = map[string]interface{}{
			"type":    string(ch.Type),
			"enabled": ch.Enabled,
			"status":  string(ch.Status),
			"metrics": ch.Metrics,
		}
	}
	return status
}

func (s *ChannelService) publishEvents(ch *channeldomain.Channel) {
	events := ch.PullEvents()
	for _, event := range events {
		s.eventBus.Publish(event)
	}
}
