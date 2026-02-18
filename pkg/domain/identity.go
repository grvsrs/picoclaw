// Package domain provides the core DDD building blocks for PicoClaw.
// All bounded contexts share these foundational types.
package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Entity base — every domain object that has identity
// ---------------------------------------------------------------------------

// EntityID is a typed identifier. All entities use string IDs for portability.
type EntityID string

// NewID generates a cryptographically random 16-byte hex identifier.
func NewID() EntityID {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("domain: failed to generate ID: %v", err))
	}
	return EntityID(hex.EncodeToString(b))
}

// String implements fmt.Stringer.
func (id EntityID) String() string { return string(id) }

// IsZero returns true if the ID is empty.
func (id EntityID) IsZero() bool { return id == "" }

// ---------------------------------------------------------------------------
// Timestamp value object
// ---------------------------------------------------------------------------

// Timestamp wraps time.Time with JSON-friendly serialization and domain semantics.
type Timestamp struct {
	time.Time
}

// Now returns the current UTC timestamp.
func Now() Timestamp { return Timestamp{time.Now().UTC()} }

// ZeroTime returns the zero-value timestamp.
func ZeroTime() Timestamp { return Timestamp{} }

// TimestampFrom wraps an existing time.Time.
func TimestampFrom(t time.Time) Timestamp { return Timestamp{t.UTC()} }

// ---------------------------------------------------------------------------
// Aggregate root base
// ---------------------------------------------------------------------------

// AggregateRoot is the base for all aggregate roots. It records domain events
// that occurred during a unit of work, to be dispatched after persistence.
type AggregateRoot struct {
	id     EntityID
	events []Event
}

// ID returns the aggregate's identity.
func (a *AggregateRoot) ID() EntityID { return a.id }

// SetID sets the aggregate's identity (used during reconstitution).
func (a *AggregateRoot) SetID(id EntityID) { a.id = id }

// RecordEvent appends a domain event to be dispatched after persistence.
func (a *AggregateRoot) RecordEvent(e Event) {
	a.events = append(a.events, e)
}

// PullEvents returns and clears all pending domain events.
func (a *AggregateRoot) PullEvents() []Event {
	events := a.events
	a.events = nil
	return events
}

// HasPendingEvents returns true if there are undispatched events.
func (a *AggregateRoot) HasPendingEvents() bool {
	return len(a.events) > 0
}
