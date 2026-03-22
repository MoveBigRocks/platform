package eventbus

import (
	"encoding/json"
	"fmt"
)

// EventHeader represents the minimal header fields needed to route an event.
// This allows handlers to determine the event type without unmarshaling the full payload.
type EventHeader struct {
	EventID   string    `json:"event_id"`
	EventType EventType `json:"event_type"`
}

// ParseEventHeader extracts just the header fields from raw event JSON.
// This is more efficient than unmarshaling the full event when only routing is needed.
func ParseEventHeader(data []byte) (EventHeader, error) {
	var hdr EventHeader
	if err := json.Unmarshal(data, &hdr); err != nil {
		return EventHeader{}, fmt.Errorf("parse event header: %w", err)
	}
	return hdr, nil
}
