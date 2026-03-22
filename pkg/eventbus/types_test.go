package eventbus

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventType_String(t *testing.T) {
	tests := []struct {
		name     string
		et       EventType
		expected string
	}{
		{"IssueCreated", TypeIssueCreated, "issue.created"},
		{"CaseCreated", TypeCaseCreated, "case.created"},
		{"CaseAssigned", TypeCaseAssigned, "case.assigned"},
		{"IssueResolved", TypeIssueResolved, "issue.resolved"},
		{"Zero value", EventType{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.et.String())
		})
	}
}

func TestEventType_IsZero(t *testing.T) {
	tests := []struct {
		name     string
		et       EventType
		expected bool
	}{
		{"Zero value", EventType{}, true},
		{"Non-zero value", TypeIssueCreated, false},
		{"Another non-zero", TypeCaseCreated, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.et.IsZero())
		})
	}
}

func TestEventType_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		et       EventType
		expected string
	}{
		{"IssueCreated", TypeIssueCreated, `{"type":"issue.created","version":1}`},
		{"CaseCreated", TypeCaseCreated, `{"type":"case.created","version":1}`},
		{"Zero value", EventType{}, `{"type":"","version":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.et)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

func TestEventType_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected EventType
	}{
		{"IssueCreated", `"issue.created"`, EventType{value: "issue.created"}},
		{"CaseCreated", `"case.created"`, EventType{value: "case.created"}},
		{"Empty string", `""`, EventType{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var et EventType
			err := json.Unmarshal([]byte(tt.json), &et)
			require.NoError(t, err)
			assert.Equal(t, tt.expected.value, et.value)
		})
	}
}

func TestEventType_JSONRoundTrip(t *testing.T) {
	// Test that EventType survives JSON round-trip
	original := TypeIssueCreated

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded EventType
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.String(), decoded.String())
}

func TestEventType_InStruct(t *testing.T) {
	// Test EventType when embedded in a struct (simulates event serialization)
	type TestEvent struct {
		ID        string    `json:"id"`
		EventType EventType `json:"event_type"`
		Message   string    `json:"message"`
	}

	original := TestEvent{
		ID:        "test-123",
		EventType: TypeCaseCreated,
		Message:   "Test message",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded TestEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.EventType.String(), decoded.EventType.String())
	assert.Equal(t, original.Message, decoded.Message)
}
