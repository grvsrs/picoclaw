package bus

import (
	"context"
	"sync"
)

// Subscriber is a named tap on a message stream. Multiple subscribers can
// independently consume the same published messages (fan-out).
type Subscriber struct {
	Name string
	ch   chan interface{} // receives copies of published messages
}

type MessageBus struct {
	inbound  chan InboundMessage
	outbound chan OutboundMessage
	handlers map[string]MessageHandler
	mu       sync.RWMutex
	closed   bool
	closeOnce sync.Once

	// Fan-out subscribers — every published message is sent to all taps
	inboundSubs  []*Subscriber
	outboundSubs []*Subscriber
	systemSubs   []*Subscriber // for SystemEvent fan-out
}

func NewMessageBus() *MessageBus {
	return &MessageBus{
		inbound:  make(chan InboundMessage, 100),
		outbound: make(chan OutboundMessage, 100),
		handlers: make(map[string]MessageHandler),
	}
}

// --- Fan-out subscriptions ---

// SubscribeInboundTap creates a named subscriber that receives copies of all
// inbound messages. The returned channel is buffered; slow consumers drop.
func (mb *MessageBus) SubscribeInboundTap(name string) <-chan interface{} {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	sub := &Subscriber{Name: name, ch: make(chan interface{}, 64)}
	mb.inboundSubs = append(mb.inboundSubs, sub)
	return sub.ch
}

// SubscribeOutboundTap creates a named subscriber for outbound messages.
func (mb *MessageBus) SubscribeOutboundTap(name string) <-chan interface{} {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	sub := &Subscriber{Name: name, ch: make(chan interface{}, 64)}
	mb.outboundSubs = append(mb.outboundSubs, sub)
	return sub.ch
}

// SubscribeSystem creates a named subscriber for system events.
func (mb *MessageBus) SubscribeSystem(name string) <-chan interface{} {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	sub := &Subscriber{Name: name, ch: make(chan interface{}, 64)}
	mb.systemSubs = append(mb.systemSubs, sub)
	return sub.ch
}

// PublishSystem publishes a system event to all system subscribers.
func (mb *MessageBus) PublishSystem(event SystemEvent) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	if mb.closed {
		return
	}
	for _, sub := range mb.systemSubs {
		select {
		case sub.ch <- event:
		default: // drop if slow
		}
	}
}

func (mb *MessageBus) fanOutInbound(msg InboundMessage) {
	for _, sub := range mb.inboundSubs {
		select {
		case sub.ch <- msg:
		default: // non-blocking — drop if subscriber is slow
		}
	}
}

func (mb *MessageBus) fanOutOutbound(msg OutboundMessage) {
	for _, sub := range mb.outboundSubs {
		select {
		case sub.ch <- msg:
		default:
		}
	}
}

// --- Original publish/consume (primary consumer unchanged) ---

func (mb *MessageBus) PublishInbound(msg InboundMessage) {
	mb.mu.RLock()
	if mb.closed {
		mb.mu.RUnlock()
		return
	}
	// Fan out to all taps
	mb.fanOutInbound(msg)
	mb.mu.RUnlock()

	select {
	case mb.inbound <- msg:
	default:
		// Channel full — drop oldest and retry
		select {
		case <-mb.inbound:
		default:
		}
		select {
		case mb.inbound <- msg:
		default:
		}
	}
}

func (mb *MessageBus) ConsumeInbound(ctx context.Context) (InboundMessage, bool) {
	select {
	case msg := <-mb.inbound:
		return msg, true
	case <-ctx.Done():
		return InboundMessage{}, false
	}
}

func (mb *MessageBus) PublishOutbound(msg OutboundMessage) {
	mb.mu.RLock()
	if mb.closed {
		mb.mu.RUnlock()
		return
	}
	// Fan out to all taps
	mb.fanOutOutbound(msg)
	mb.mu.RUnlock()

	select {
	case mb.outbound <- msg:
	default:
		// Channel full — drop oldest and retry
		select {
		case <-mb.outbound:
		default:
		}
		select {
		case mb.outbound <- msg:
		default:
		}
	}
}

func (mb *MessageBus) SubscribeOutbound(ctx context.Context) (OutboundMessage, bool) {
	select {
	case msg := <-mb.outbound:
		return msg, true
	case <-ctx.Done():
		return OutboundMessage{}, false
	}
}

func (mb *MessageBus) RegisterHandler(channel string, handler MessageHandler) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.handlers[channel] = handler
}

func (mb *MessageBus) GetHandler(channel string) (MessageHandler, bool) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	handler, ok := mb.handlers[channel]
	return handler, ok
}

func (mb *MessageBus) Close() {
	mb.closeOnce.Do(func() {
		mb.mu.Lock()
		mb.closed = true
		// Close subscriber channels
		for _, sub := range mb.inboundSubs {
			close(sub.ch)
		}
		for _, sub := range mb.outboundSubs {
			close(sub.ch)
		}
		for _, sub := range mb.systemSubs {
			close(sub.ch)
		}
		mb.mu.Unlock()
		close(mb.inbound)
		close(mb.outbound)
	})
}
