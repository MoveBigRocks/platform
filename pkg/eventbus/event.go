package eventbus

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Sentinel errors for event validation
var (
	ErrValidationFailed = errors.New("event validation failed")
	ErrMissingEventID   = errors.New("event_id is required")
	ErrMissingEventType = errors.New("event_type is required")
)

// Event is the interface that all events must implement.
// This provides compile-time enforcement that only valid events can be published.
type Event interface {
	// GetEventID returns the unique identifier for this event instance (UUIDv7)
	GetEventID() string

	// GetEventType returns the type-safe event type
	GetEventType() EventType

	// Validate validates the event payload
	Validate() error
}

// ============================================================================
// Event ID Generation (UUIDv7)
// ============================================================================

var (
	clockSeq     uint16
	clockSeqOnce sync.Once
	clockSeqMu   sync.Mutex
	lastTime     int64
)

// initClockSeq initializes the random clock sequence
func initClockSeq() {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to time-based seed if crypto/rand fails
		clockSeq = uint16(time.Now().UnixNano() & 0xFFFF)
		return
	}
	clockSeq = binary.BigEndian.Uint16(b[:])
}

// NewEventID generates a new UUIDv7 event ID.
// UUIDv7 is time-ordered, making it ideal for event tracing and debugging.
// Format: xxxxxxxx-xxxx-7xxx-yxxx-xxxxxxxxxxxx
// Where the first 48 bits are Unix timestamp in milliseconds
func NewEventID() string {
	clockSeqOnce.Do(initClockSeq)

	// Get current timestamp in milliseconds
	now := time.Now().UnixMilli()

	clockSeqMu.Lock()
	// If time hasn't advanced, increment clock sequence
	if now <= lastTime {
		clockSeq++
	}
	lastTime = now
	seq := clockSeq
	clockSeqMu.Unlock()

	// Generate random bytes for the rest
	var randomBytes [8]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		// Fallback: use clock sequence and time
		binary.BigEndian.PutUint64(randomBytes[:], uint64(now)^uint64(seq)<<48)
	}

	// Build UUID bytes
	var uuid [16]byte

	// Timestamp (48 bits) - first 6 bytes
	binary.BigEndian.PutUint16(uuid[0:2], uint16(now>>32))
	binary.BigEndian.PutUint32(uuid[2:6], uint32(now))

	// Version 7 (4 bits) + random (12 bits) - bytes 6-7
	uuid[6] = (randomBytes[0] & 0x0F) | 0x70 // Version 7
	uuid[7] = randomBytes[1]

	// Variant (2 bits) + clock sequence (6 bits) + random (8 bits) - bytes 8-9
	uuid[8] = byte((seq>>8)&0x3F) | 0x80 // Variant 10
	uuid[9] = byte(seq)

	// Random (48 bits) - bytes 10-15
	copy(uuid[10:], randomBytes[2:8])

	// Format as string
	return formatUUID(uuid[:])
}

// formatUUID formats UUID bytes as a hyphenated string
func formatUUID(uuid []byte) string {
	var buf [36]byte
	hex.Encode(buf[0:8], uuid[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], uuid[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], uuid[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], uuid[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], uuid[10:16])
	return string(buf[:])
}

// ============================================================================
// Base Event Struct (for embedding)
// ============================================================================

// BaseEvent provides common fields for all events.
// Embed this in your event structs to get EventID and EventType for free.
//
// Correlation fields enable distributed tracing across event chains:
//   - CorrelationID: Groups related events (e.g., email → case → rule → notification)
//   - ParentEventID: Links to the event that triggered this one (for event graphs)
//   - TraceID/SpanID: OpenTelemetry integration for distributed tracing
type BaseEvent struct {
	EventID   string    `json:"event_id"`
	EventType EventType `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`

	// Correlation fields for distributed tracing
	CorrelationID string `json:"correlation_id,omitempty"`  // Groups related events in a chain
	ParentEventID string `json:"parent_event_id,omitempty"` // The event that triggered this one
	TraceID       string `json:"trace_id,omitempty"`        // OpenTelemetry trace ID
	SpanID        string `json:"span_id,omitempty"`         // OpenTelemetry span ID
}

// GetEventID returns the event's unique identifier
func (e BaseEvent) GetEventID() string {
	return e.EventID
}

// GetEventType returns the event's type
func (e BaseEvent) GetEventType() EventType {
	return e.EventType
}

// GetCorrelationID returns the correlation ID for distributed tracing
func (e BaseEvent) GetCorrelationID() string {
	return e.CorrelationID
}

// GetParentEventID returns the parent event ID for event chain tracking
func (e BaseEvent) GetParentEventID() string {
	return e.ParentEventID
}

// NewBaseEvent creates a new BaseEvent with the given type.
// Use NewBaseEventWithCorrelation for events in an existing chain.
func NewBaseEvent(eventType EventType) BaseEvent {
	return BaseEvent{
		EventID:   NewEventID(),
		EventType: eventType,
		Timestamp: time.Now().UTC(),
	}
}

// ============================================================================
// Event Envelope (for serialization)
// ============================================================================

// EventEnvelope wraps an event for transmission with metadata
type EventEnvelope struct {
	EventID   string                 `json:"event_id"`
	EventType EventType              `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`

	// Correlation fields for distributed tracing
	CorrelationID string `json:"correlation_id,omitempty"`
	ParentEventID string `json:"parent_event_id,omitempty"`
	TraceID       string `json:"trace_id,omitempty"`
	SpanID        string `json:"span_id,omitempty"`
}

// CorrelatedEvent is an optional interface for events with correlation support.
// Events that embed BaseEvent automatically implement this interface.
type CorrelatedEvent interface {
	Event
	GetCorrelationID() string
	GetParentEventID() string
}
