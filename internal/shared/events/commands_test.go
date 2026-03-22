package events

import (
	"testing"
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendEmailRequestedEvent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		event   SendEmailRequestedEvent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event with text content",
			event: SendEmailRequestedEvent{
				BaseEvent:   eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeSendEmailRequested, Timestamp: time.Now()},
				WorkspaceID: "ws-123",
				RequestedAt: time.Now(),
				RequestedBy: "rule_action_executor",
				ToEmails:    []string{"test@example.com"},
				Subject:     "Test Subject",
				TextContent: "Test body",
				Category:    "rule",
			},
			wantErr: false,
		},
		{
			name: "valid event with HTML content",
			event: SendEmailRequestedEvent{
				BaseEvent:   eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeSendEmailRequested, Timestamp: time.Now()},
				WorkspaceID: "ws-123",
				RequestedAt: time.Now(),
				RequestedBy: "form_notification_service",
				ToEmails:    []string{"test@example.com"},
				Subject:     "Test Subject",
				HTMLContent: "<p>Test body</p>",
				Category:    "form_notification",
			},
			wantErr: false,
		},
		{
			name: "valid event with template",
			event: SendEmailRequestedEvent{
				BaseEvent:   eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeSendEmailRequested, Timestamp: time.Now()},
				WorkspaceID: "ws-123",
				RequestedAt: time.Now(),
				RequestedBy: "system",
				ToEmails:    []string{"test@example.com"},
				Subject:     "Test Subject",
				TemplateID:  "welcome-template",
				Category:    "system",
			},
			wantErr: false,
		},
		{
			name: "missing workspace_id",
			event: SendEmailRequestedEvent{
				BaseEvent:   eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeSendEmailRequested, Timestamp: time.Now()},
				ToEmails:    []string{"test@example.com"},
				Subject:     "Test Subject",
				TextContent: "Test body",
			},
			wantErr: true,
			errMsg:  "workspace_id is required",
		},
		{
			name: "missing recipients",
			event: SendEmailRequestedEvent{
				BaseEvent:   eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeSendEmailRequested, Timestamp: time.Now()},
				WorkspaceID: "ws-123",
				ToEmails:    []string{},
				Subject:     "Test Subject",
				TextContent: "Test body",
			},
			wantErr: true,
			errMsg:  "at least one recipient email is required",
		},
		{
			name: "missing subject",
			event: SendEmailRequestedEvent{
				BaseEvent:   eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeSendEmailRequested, Timestamp: time.Now()},
				WorkspaceID: "ws-123",
				ToEmails:    []string{"test@example.com"},
				Subject:     "",
				TextContent: "Test body",
			},
			wantErr: true,
			errMsg:  "subject is required",
		},
		{
			name: "missing content and template",
			event: SendEmailRequestedEvent{
				BaseEvent:   eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeSendEmailRequested, Timestamp: time.Now()},
				WorkspaceID: "ws-123",
				ToEmails:    []string{"test@example.com"},
				Subject:     "Test Subject",
			},
			wantErr: true,
			errMsg:  "content or template_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateCaseRequestedEvent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		event   CreateCaseRequestedEvent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event",
			event: CreateCaseRequestedEvent{
				BaseEvent:    eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeCreateCaseRequested, Timestamp: time.Now()},
				WorkspaceID:  "ws-123",
				RequestedAt:  time.Now(),
				RequestedBy:  "form_auto_action_executor",
				Subject:      "Test Case Subject",
				Description:  "Test case description",
				ContactEmail: "customer@example.com",
				SourceType:   "form",
				SourceID:     "form-123",
			},
			wantErr: false,
		},
		{
			name: "valid event with all optional fields",
			event: CreateCaseRequestedEvent{
				BaseEvent:          eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeCreateCaseRequested, Timestamp: time.Now()},
				WorkspaceID:        "ws-123",
				RequestedAt:        time.Now(),
				RequestedBy:        "form_auto_action_executor",
				Subject:            "Test Case Subject",
				Description:        "Test case description",
				Priority:           "high",
				Channel:            "form",
				ContactEmail:       "customer@example.com",
				ContactName:        "John Doe",
				ContactPhone:       "+1234567890",
				TeamID:             "team-123",
				AssignedToID:       "user-456",
				Category:           "support",
				Tags:               []string{"urgent", "billing"},
				SourceType:         "form",
				SourceID:           "form-123",
				SourceSubmissionID: "submission-789",
				CustomFields: map[string]interface{}{
					"custom_field_1": "value1",
				},
			},
			wantErr: false,
		},
		{
			name: "missing workspace_id",
			event: CreateCaseRequestedEvent{
				BaseEvent:    eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeCreateCaseRequested, Timestamp: time.Now()},
				Subject:      "Test Case Subject",
				ContactEmail: "customer@example.com",
			},
			wantErr: true,
			errMsg:  "workspace_id is required",
		},
		{
			name: "missing subject",
			event: CreateCaseRequestedEvent{
				BaseEvent:    eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeCreateCaseRequested, Timestamp: time.Now()},
				WorkspaceID:  "ws-123",
				Subject:      "",
				ContactEmail: "customer@example.com",
			},
			wantErr: true,
			errMsg:  "subject is required",
		},
		{
			name: "missing contact_email",
			event: CreateCaseRequestedEvent{
				BaseEvent:   eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeCreateCaseRequested, Timestamp: time.Now()},
				WorkspaceID: "ws-123",
				Subject:     "Test Case Subject",
			},
			wantErr: true,
			errMsg:  "contact_email is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSendNotificationRequestedEvent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		event   SendNotificationRequestedEvent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid email notification",
			event: SendNotificationRequestedEvent{
				BaseEvent:   eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeSendNotificationRequested, Timestamp: time.Now()},
				WorkspaceID: "ws-123",
				RequestedAt: time.Now(),
				RequestedBy: "form_notification_service",
				Type:        "email",
				Recipients:  []string{"admin@example.com"},
				Subject:     "New form submission",
				Body:        "A new form was submitted",
				SourceType:  "form",
				SourceID:    "form-123",
			},
			wantErr: false,
		},
		{
			name: "missing workspace_id",
			event: SendNotificationRequestedEvent{
				BaseEvent:  eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeSendNotificationRequested, Timestamp: time.Now()},
				Type:       "email",
				Recipients: []string{"admin@example.com"},
			},
			wantErr: true,
			errMsg:  "workspace_id is required",
		},
		{
			name: "missing recipients",
			event: SendNotificationRequestedEvent{
				BaseEvent:   eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeSendNotificationRequested, Timestamp: time.Now()},
				WorkspaceID: "ws-123",
				Type:        "email",
				Recipients:  []string{},
			},
			wantErr: true,
			errMsg:  "at least one recipient is required",
		},
		{
			name: "missing type",
			event: SendNotificationRequestedEvent{
				BaseEvent:   eventbus.BaseEvent{EventID: "evt-123", EventType: eventbus.TypeSendNotificationRequested, Timestamp: time.Now()},
				WorkspaceID: "ws-123",
				Recipients:  []string{"admin@example.com"},
			},
			wantErr: true,
			errMsg:  "notification type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
