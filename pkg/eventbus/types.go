package eventbus

import (
	"encoding/json"
	"fmt"
)

// EventType is a type-safe event type identifier with versioning.
// The unexported fields prevent arbitrary creation outside this package.
// This pattern ensures compile-time safety - you cannot create EventType{"arbitrary", 1}
// outside the eventbus package.
type EventType struct {
	value   string
	version int // Event schema version for backwards compatibility
}

// String returns the string representation of the event type
func (t EventType) String() string {
	return t.value
}

// Version returns the schema version of this event type
func (t EventType) Version() int {
	if t.version == 0 {
		return 1 // Default to version 1 for backwards compatibility
	}
	return t.version
}

// WithVersion returns a copy of this EventType with a different version.
// This is useful for handling schema migrations.
func (t EventType) WithVersion(version int) EventType {
	return EventType{value: t.value, version: version}
}

// eventTypeJSON is the JSON representation of EventType
type eventTypeJSON struct {
	Type    string `json:"type"`
	Version int    `json:"version,omitempty"`
}

// MarshalJSON implements json.Marshaler
// Serializes as {"type": "event.name", "version": 1}
func (t EventType) MarshalJSON() ([]byte, error) {
	return json.Marshal(eventTypeJSON{
		Type:    t.value,
		Version: t.Version(),
	})
}

// UnmarshalJSON implements json.Unmarshaler
// Handles both old format (plain string) and new format (object with version)
func (t *EventType) UnmarshalJSON(data []byte) error {
	// Try new format first (object with type and version)
	var obj eventTypeJSON
	if err := json.Unmarshal(data, &obj); err == nil {
		// Accept object format even if Type is empty (zero value EventType)
		t.value = obj.Type
		t.version = obj.Version
		if t.version == 0 {
			t.version = 1
		}
		return nil
	}

	// Fall back to old format (plain string) for backwards compatibility
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("invalid event type format: %w", err)
	}
	t.value = str
	t.version = 1
	return nil
}

// IsZero returns true if the EventType is uninitialized
func (t EventType) IsZero() bool {
	return t.value == ""
}

// IsUnknown returns true if this is the TypeUnknown sentinel value
func (t EventType) IsUnknown() bool {
	return t.value == "unknown" && t.version == 0
}

// ============================================================================
// Special Event Types
// ============================================================================

var (
	// TypeUnknown represents an unrecognized event type.
	// Used when deserializing events with unknown types to avoid crashes.
	TypeUnknown = EventType{value: "unknown", version: 0}
)

// ============================================================================
// Observability Events
// ============================================================================

var (
	// TypeIssueCreated is published when a new issue is created
	TypeIssueCreated = EventType{value: "issue.created", version: 1}

	// TypeIssueUpdated is published when an issue is updated
	TypeIssueUpdated = EventType{value: "issue.updated", version: 1}

	// TypeIssueResolved is published when an issue is resolved
	TypeIssueResolved = EventType{value: "issue.resolved", version: 1}
)

// ============================================================================
// Case Events
// ============================================================================

var (
	// TypeCaseCreated is published when a new case is created
	TypeCaseCreated = EventType{value: "case.created", version: 1}

	// TypeCaseAssigned is published when a case is assigned
	TypeCaseAssigned = EventType{value: "case.assigned", version: 1}

	// TypeCaseStatusChanged is published when case status changes
	TypeCaseStatusChanged = EventType{value: "case.status_changed", version: 1}

	// TypeCaseResolved is published when a case is resolved
	TypeCaseResolved = EventType{value: "case.resolved", version: 1}
)

// ============================================================================
// Knowledge Events
// ============================================================================

var (
	// TypeKnowledgeCreated is published when a new knowledge resource is created.
	TypeKnowledgeCreated = EventType{value: "knowledge.created", version: 1}

	// TypeKnowledgeReviewRequested is published when a knowledge item needs review.
	TypeKnowledgeReviewRequested = EventType{value: "knowledge.review_requested", version: 1}
)

// ============================================================================
// Form Events
// ============================================================================

var (
	// TypeFormSubmitted is published when a public form submission is accepted.
	TypeFormSubmitted = EventType{value: "form.submitted", version: 1}
)

// ============================================================================
// Integration Events (Cross-Domain)
// ============================================================================

var (
	// TypeIssueCaseLinked is published when an issue is linked to a case
	TypeIssueCaseLinked = EventType{value: "issue_case.linked", version: 1}

	// TypeIssueCaseUnlinked is published when an issue is unlinked from a case
	TypeIssueCaseUnlinked = EventType{value: "issue_case.unlinked", version: 1}

	// TypeCaseCreatedForContact is published when a case is auto-created for a contact
	TypeCaseCreatedForContact = EventType{value: "case.created_for_contact", version: 1}

	// TypeCasesBulkResolved is published when multiple cases are resolved in bulk
	TypeCasesBulkResolved = EventType{value: "cases.bulk_resolved", version: 1}
)

// ============================================================================
// Command Events (for cross-domain requests)
// ============================================================================

var (
	// TypeSendEmailRequested is a command to send an email (cross-domain)
	TypeSendEmailRequested = EventType{value: "email.send_requested", version: 1}

	// TypeCreateCaseRequested is a command to create a case (cross-domain)
	TypeCreateCaseRequested = EventType{value: "case.create_requested", version: 1}

	// TypeSendNotificationRequested is a command to send a notification (cross-domain)
	TypeSendNotificationRequested = EventType{value: "notification.send_requested", version: 1}

	// TypeCaseCreatedFromCommand is a response event after a case is created from a command
	TypeCaseCreatedFromCommand = EventType{value: "case.created_from_command", version: 1}
)

// allEventTypes is the list of all registered event types
var allEventTypes = []EventType{
	// Observability
	TypeIssueCreated,
	TypeIssueUpdated,
	TypeIssueResolved,
	// Case
	TypeCaseCreated,
	TypeCaseAssigned,
	TypeCaseStatusChanged,
	TypeCaseResolved,
	// Knowledge
	TypeKnowledgeCreated,
	TypeKnowledgeReviewRequested,
	// Form
	TypeFormSubmitted,
	// Integration
	TypeIssueCaseLinked,
	TypeIssueCaseUnlinked,
	TypeCaseCreatedForContact,
	TypeCasesBulkResolved,
	// Commands
	TypeSendEmailRequested,
	TypeCreateCaseRequested,
	TypeSendNotificationRequested,
	TypeCaseCreatedFromCommand,
}

// eventTypeMap provides O(1) lookup by event type string
var eventTypeMap = make(map[string]EventType)

func init() {
	for _, t := range allEventTypes {
		eventTypeMap[t.value] = t
	}
}

// RegisteredEventTypes returns a copy of the core event types known to the runtime.
func RegisteredEventTypes() []EventType {
	result := make([]EventType, len(allEventTypes))
	copy(result, allEventTypes)
	return result
}

// LookupEventTypeOrUnknown returns the EventType for a given string value,
// or TypeUnknown if the event type is not recognized.
// This is the preferred way to look up event types from external/untrusted data
// as it never panics and allows graceful handling of unknown events.
func LookupEventTypeOrUnknown(value string) EventType {
	t, ok := eventTypeMap[value]
	if !ok {
		return TypeUnknown
	}
	return t
}

// EventTypeFromString converts a string to an EventType.
// Returns TypeUnknown for unrecognized types, consistent with StreamFromString.
// This is primarily used for deserialization from storage (e.g., outbox pattern).
func EventTypeFromString(value string) EventType {
	return LookupEventTypeOrUnknown(value)
}
