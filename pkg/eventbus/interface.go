package eventbus

import (
	"context"
	"time"
)

// Stream is a type-safe stream identifier.
// The unexported value field prevents arbitrary creation outside this package,
// ensuring compile-time safety for stream names.
type Stream struct {
	value string
}

// String returns the string representation of the stream
func (s Stream) String() string {
	return s.value
}

// IsZero returns true if the Stream is uninitialized
func (s Stream) IsZero() bool {
	return s.value == ""
}

// StreamFromString creates a Stream from a string value.
// This is primarily used for deserialization from storage.
// For new code, prefer using the type-safe Stream constants.
func StreamFromString(value string) Stream {
	return Stream{value: value}
}

// Type-safe stream definitions
// Use these variables instead of string literals for compile-time safety
var (
	// Event streams for different domains
	StreamErrorEvents      = Stream{value: "error-events"}
	StreamIssueEvents      = Stream{value: "issue-events"}
	StreamCaseEvents       = Stream{value: "case-events"}
	StreamKnowledgeEvents  = Stream{value: "knowledge-events"}
	StreamEmailEvents      = Stream{value: "email-events"}
	StreamAlertEvents      = Stream{value: "alert-events"}
	StreamAuditEvents      = Stream{value: "audit-events"}
	StreamMetrics          = Stream{value: "metrics"}
	StreamAnalytics        = Stream{value: "analytics"}
	StreamJobEvents        = Stream{value: "job-events"}
	StreamPermissionEvents = Stream{value: "permission-events"}
	StreamSystemEvents     = Stream{value: "system-events"}

	// Form events
	StreamFormEvents = Stream{value: "form-events"}

	// Command streams for cross-domain requests
	// These enable loose coupling between bounded contexts
	StreamEmailCommands        = Stream{value: "email-commands"}
	StreamCaseCommands         = Stream{value: "case-commands"}
	StreamNotificationCommands = Stream{value: "notification-commands"}
)

// EventBus is an alias for Bus for backwards compatibility
type EventBus = Bus

// Bus defines the interface for event publishing and consumption
// Implementations: FileEventBus (filesystem-based)
type Bus interface {
	// PublishEvent publishes a type-safe event to a stream (compile-time enforcement)
	// Events must implement the Event interface which includes Validate()
	PublishEvent(stream Stream, event Event) error

	// PublishEventWithRetry publishes a type-safe event with automatic retry
	PublishEventWithRetry(stream Stream, event Event, maxRetries int) error

	// Subscribe subscribes to a stream with a consumer group
	// Handler receives context for cancellation and timeout control
	Subscribe(stream Stream, group, consumer string, handler func(ctx context.Context, data []byte) error) error

	// GetStreamInfo returns info about a stream (for monitoring)
	GetStreamInfo(stream Stream) (length int64, groups int64, err error)

	// GetPendingMessages returns number of pending messages for a consumer group
	GetPendingMessages(stream Stream, group string) (int64, error)

	// HealthCheck verifies the event bus is operational
	HealthCheck() error

	// Shutdown gracefully shuts down the event bus
	Shutdown(timeout time.Duration) error

	// Close closes the event bus connection
	Close() error

	// PublishValidated publishes a validated event (alias for PublishEvent for backwards compatibility)
	PublishValidated(stream Stream, event Event) error

	// Publish publishes raw data to a stream (for backwards compatibility with outbox pattern)
	Publish(stream Stream, data interface{}) error

	// PublishWithType publishes raw data with preserved event type information.
	// Used by the outbox retry mechanism to preserve EventType across retries.
	// The eventID parameter allows preserving the original event ID for traceability.
	// For new events, pass an empty eventID and one will be generated.
	PublishWithType(stream Stream, eventType EventType, eventID string, data interface{}) error
}

// Ensure FileEventBus implements Bus interface
var _ Bus = (*FileEventBus)(nil)
