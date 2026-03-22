package eventbus

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEventID(t *testing.T) {
	id := NewEventID()

	// Should not be empty
	assert.NotEmpty(t, id)

	// Should be a valid UUID format (8-4-4-4-12)
	parts := strings.Split(id, "-")
	assert.Len(t, parts, 5, "UUID should have 5 parts separated by hyphens")

	// Check part lengths
	assert.Len(t, parts[0], 8, "First part should be 8 characters")
	assert.Len(t, parts[1], 4, "Second part should be 4 characters")
	assert.Len(t, parts[2], 4, "Third part should be 4 characters")
	assert.Len(t, parts[3], 4, "Fourth part should be 4 characters")
	assert.Len(t, parts[4], 12, "Fifth part should be 12 characters")
}

func TestNewEventID_Uniqueness(t *testing.T) {
	// Generate multiple IDs and ensure they're unique
	ids := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		id := NewEventID()
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}

	assert.Len(t, ids, count, "All generated IDs should be unique")
}

func TestNewEventID_TimeOrdering(t *testing.T) {
	// UUIDv7 should be time-ordered, so IDs generated later should be lexicographically greater.
	// Sleep 2ms to guarantee different millisecond timestamps (UUIDv7 has ms precision).
	id1 := NewEventID()
	time.Sleep(2 * time.Millisecond)
	id2 := NewEventID()

	assert.True(t, id2 > id1, "Later ID should be lexicographically greater (UUIDv7 time-ordering)")
}

func TestBaseEvent_GetEventID(t *testing.T) {
	base := BaseEvent{
		EventID:   "test-event-id-123",
		EventType: TypeCaseCreated,
		Timestamp: time.Now(),
	}

	assert.Equal(t, "test-event-id-123", base.GetEventID())
}

func TestBaseEvent_GetEventType(t *testing.T) {
	base := BaseEvent{
		EventID:   "test-event-id-123",
		EventType: TypeCaseCreated,
		Timestamp: time.Now(),
	}

	assert.Equal(t, TypeCaseCreated, base.GetEventType())
	assert.Equal(t, "case.created", base.GetEventType().String())
}

func TestNewBaseEvent(t *testing.T) {
	eventType := TypeIssueCreated
	base := NewBaseEvent(eventType)

	// Should have auto-generated ID
	assert.NotEmpty(t, base.EventID)

	// Should have correct event type
	assert.Equal(t, eventType, base.EventType)

	// Should have timestamp
	assert.False(t, base.Timestamp.IsZero())

	// Timestamp should be recent
	assert.WithinDuration(t, time.Now(), base.Timestamp, time.Second)
}

func TestNewBaseEvent_DifferentTypes(t *testing.T) {
	testTypes := []EventType{
		TypeIssueCreated,
		TypeCaseCreated,
		TypeCaseAssigned,
		TypeIssueResolved,
	}

	for _, et := range testTypes {
		t.Run(et.String(), func(t *testing.T) {
			base := NewBaseEvent(et)
			assert.Equal(t, et, base.EventType)
			assert.NotEmpty(t, base.EventID)
		})
	}
}

// testTypeSafeEvent is a mock event that implements the Event interface
type testTypeSafeEvent struct {
	EventID   string
	EventType EventType
	Timestamp time.Time
	Data      string
	valid     bool
}

func (e testTypeSafeEvent) GetEventID() string {
	return e.EventID
}

func (e testTypeSafeEvent) GetEventType() EventType {
	return e.EventType
}

func (e testTypeSafeEvent) Validate() error {
	if !e.valid {
		return ErrValidationFailed
	}
	if e.EventID == "" {
		return ErrMissingEventID
	}
	if e.EventType.IsZero() {
		return ErrMissingEventType
	}
	return nil
}

func TestEvent_InterfaceCompliance(t *testing.T) {
	// Verify that our test event implements the Event interface
	var _ Event = testTypeSafeEvent{}
	var _ Event = &testTypeSafeEvent{}
}

func TestEvent_Validation(t *testing.T) {
	tests := []struct {
		name      string
		event     testTypeSafeEvent
		wantError bool
	}{
		{
			name: "valid event",
			event: testTypeSafeEvent{
				EventID:   NewEventID(),
				EventType: TypeCaseCreated,
				Timestamp: time.Now(),
				Data:      "test data",
				valid:     true,
			},
			wantError: false,
		},
		{
			name: "missing event ID",
			event: testTypeSafeEvent{
				EventID:   "",
				EventType: TypeCaseCreated,
				Timestamp: time.Now(),
				valid:     true,
			},
			wantError: true,
		},
		{
			name: "missing event type",
			event: testTypeSafeEvent{
				EventID:   NewEventID(),
				EventType: EventType{},
				Timestamp: time.Now(),
				valid:     true,
			},
			wantError: true,
		},
		{
			name: "validation disabled",
			event: testTypeSafeEvent{
				EventID:   NewEventID(),
				EventType: TypeCaseCreated,
				Timestamp: time.Now(),
				valid:     false,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEventErrors(t *testing.T) {
	// Verify error sentinel values
	assert.Error(t, ErrValidationFailed)
	assert.Error(t, ErrMissingEventID)
	assert.Error(t, ErrMissingEventType)

	// Verify error messages
	assert.Contains(t, ErrValidationFailed.Error(), "validation")
	assert.Contains(t, ErrMissingEventID.Error(), "event_id")
	assert.Contains(t, ErrMissingEventType.Error(), "event_type")
}

// Test that FileEventBus.PublishEvent requires Event interface
func TestFileEventBus_PublishEvent_TypeSafety(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Create a valid event that implements Event interface
	event := testTypeSafeEvent{
		EventID:   NewEventID(),
		EventType: TypeCaseCreated,
		Timestamp: time.Now(),
		Data:      "test data",
		valid:     true,
	}

	// PublishEvent should accept the event
	err := eb.PublishEvent(StreamCaseEvents, event)
	require.NoError(t, err)
}

func TestFileEventBus_PublishEvent_ValidationFailure(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Create an invalid event
	event := testTypeSafeEvent{
		EventID:   NewEventID(),
		EventType: TypeCaseCreated,
		Timestamp: time.Now(),
		valid:     false, // Will fail validation
	}

	// PublishEvent should reject the event due to validation failure
	err := eb.PublishEvent(StreamCaseEvents, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation")
}

func TestFileEventBus_PublishEvent_MissingEventID(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Create an event with missing ID
	event := testTypeSafeEvent{
		EventID:   "",
		EventType: TypeCaseCreated,
		Timestamp: time.Now(),
		valid:     true,
	}

	err := eb.PublishEvent(StreamCaseEvents, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event_id")
}

func TestFileEventBus_PublishEvent_MissingEventType(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Create an event with missing type
	event := testTypeSafeEvent{
		EventID:   NewEventID(),
		EventType: EventType{}, // Zero value
		Timestamp: time.Now(),
		valid:     true,
	}

	err := eb.PublishEvent(StreamCaseEvents, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event_type")
}

func TestFileEventBus_PublishEventWithRetry(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	event := testTypeSafeEvent{
		EventID:   NewEventID(),
		EventType: TypeCaseCreated,
		Timestamp: time.Now(),
		Data:      "retry test",
		valid:     true,
	}

	// Should succeed on first try
	err := eb.PublishEventWithRetry(StreamCaseEvents, event, 3)
	require.NoError(t, err)
}

func TestFileEventBus_PublishEventWithRetry_ValidationFailure(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	event := testTypeSafeEvent{
		EventID:   NewEventID(),
		EventType: TypeCaseCreated,
		Timestamp: time.Now(),
		valid:     false, // Will fail validation
	}

	// Should fail validation before retry logic
	err := eb.PublishEventWithRetry(StreamCaseEvents, event, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation")
}
