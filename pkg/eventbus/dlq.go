// Package eventbus provides dead letter queue (DLQ) support for failed events.
// Failed events are stored for later analysis and potential replay.
package eventbus

import (
	"context"
	"encoding/json"
	"time"
)

// FailureType constants for categorizing DLQ entries.
const (
	FailureTypeParseError   = "parse_error"   // JSON unmarshal failed
	FailureTypeHandlerError = "handler_error" // Handler returned an error
	FailureTypeUnknownType  = "unknown_type"  // No handler registered for event type
	FailureTypeMissingType  = "missing_type"  // Event has no type field
)

// DLQEntry represents a failed event stored in the dead letter queue.
type DLQEntry struct {
	ID          int64      `json:"id"`
	Stream      string     `json:"stream"`
	EventType   string     `json:"event_type"`
	EventData   []byte     `json:"event_data"`
	FailureType string     `json:"failure_type"` // parse_error, handler_error, unknown_type, missing_type
	ErrorMsg    string     `json:"error_msg"`
	Worker      string     `json:"worker"`
	ConsumerID  string     `json:"consumer_id"`
	CreatedAt   time.Time  `json:"created_at"`
	RetryCount  int        `json:"retry_count"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"` // NULL until manually processed
}

// AlertCallback is called when an event is sent to the DLQ.
// This allows integration with external alerting systems (e.g., Sentry).
type AlertCallback func(stream, eventType, failureType, errorMsg string)

// DLQ defines the interface for dead letter queue operations.
type DLQ interface {
	// Send stores a failed event in the dead letter queue.
	// Also triggers alert callback (e.g., Sentry) if configured.
	Send(ctx context.Context, stream, eventType, failureType, errorMsg string, eventData []byte) error

	// SendParseError stores an event that failed to parse.
	SendParseError(ctx context.Context, stream string, eventData []byte, err error) error

	// SendHandlerError stores an event that failed during handler processing.
	SendHandlerError(ctx context.Context, stream, eventType string, eventData []byte, err error) error

	// SendUnknownType stores an event with an unknown type (no handler registered).
	SendUnknownType(ctx context.Context, stream, eventType string, eventData []byte) error

	// SendMissingType stores an event with no type field.
	SendMissingType(ctx context.Context, stream string, eventData []byte) error

	// GetUnprocessed returns unprocessed DLQ entries for analysis.
	GetUnprocessed(ctx context.Context, limit int) ([]DLQEntry, error)

	// GetUnprocessedByStream returns unprocessed DLQ entries for a specific stream.
	GetUnprocessedByStream(ctx context.Context, stream string, limit int) ([]DLQEntry, error)

	// GetByID fetches a single DLQ entry by ID.
	GetByID(ctx context.Context, id int64) (*DLQEntry, error)

	// MarkProcessed marks a DLQ entry as processed (after manual review/replay).
	MarkProcessed(ctx context.Context, id int64) error

	// Delete permanently removes a DLQ entry (use with caution).
	Delete(ctx context.Context, id int64) error

	// PurgeProcessed removes all processed DLQ entries older than the given duration.
	PurgeProcessed(ctx context.Context, olderThan time.Duration) (int64, error)

	// GetStats returns DLQ statistics grouped by failure type.
	GetStats(ctx context.Context) (map[string]int, error)

	// SetAlertCallback sets a callback function that is called when events are sent to DLQ.
	// This enables integration with Sentry or other alerting systems.
	SetAlertCallback(callback AlertCallback)
}

// DLQReplay defines interface for DLQ replay operations.
// Separated from DLQ to avoid circular dependency with Bus interface.
type DLQReplay interface {
	// Replay republishes a single DLQ entry to its original stream.
	// If the replay succeeds, the entry is marked as processed.
	// Returns the republished event data on success.
	Replay(ctx context.Context, bus Bus, id int64) ([]byte, error)

	// ReplayBatch replays multiple DLQ entries.
	// Returns the number of successfully replayed entries and any errors.
	ReplayBatch(ctx context.Context, bus Bus, ids []int64) (int, []error)

	// ReplayByStream replays all unprocessed DLQ entries for a specific stream.
	// Returns the number of successfully replayed entries and any errors.
	ReplayByStream(ctx context.Context, bus Bus, stream string, limit int) (int, []error)

	// ReplayAll replays all unprocessed DLQ entries up to the given limit.
	// Returns the number of successfully replayed entries and any errors.
	ReplayAll(ctx context.Context, bus Bus, limit int) (int, []error)
}

// ParseEventType attempts to extract the event type from raw JSON data.
// Expects event_type as an object with type and version fields, or a simple type string.
// Returns empty string if parsing fails or type not found.
func ParseEventType(data []byte) string {
	var envelope struct {
		Type      string `json:"type"`
		EventType struct {
			Type    string `json:"type"`
			Version int    `json:"version"`
		} `json:"event_type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ""
	}
	if envelope.EventType.Type != "" {
		return envelope.EventType.Type
	}
	return envelope.Type
}
