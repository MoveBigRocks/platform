package shareddomain

import (
	"strings"
	"testing"
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
)

func TestIssueCreatedValidation(t *testing.T) {
	tests := []struct {
		name      string
		event     IssueCreated
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid event",
			event: IssueCreated{
				BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeIssueCreated),
				IssueID:      "issue_123",
				ProjectID:    "project_456",
				WorkspaceID:  "ws_789",
				Title:        "TypeError in login.js",
				Level:        "error",
				Fingerprint:  "TypeError-login.js-42",
				FirstEventID: "event_001",
				Platform:     "javascript",
				Culprit:      "login.js:42",
				CreatedAt:    time.Now(),
			},
			wantError: false,
		},
		{
			name: "missing issue_id is allowed before persistence",
			event: IssueCreated{
				BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeIssueCreated),
				ProjectID:    "project_456",
				WorkspaceID:  "ws_789",
				Title:        "Error",
				Level:        "error",
				Fingerprint:  "fp",
				FirstEventID: "event_001",
				CreatedAt:    time.Now(),
			},
			wantError: false,
		},
		{
			name: "invalid level",
			event: IssueCreated{
				BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeIssueCreated),
				IssueID:      "issue_123",
				ProjectID:    "project_456",
				WorkspaceID:  "ws_789",
				Title:        "Error",
				Level:        "INVALID",
				Fingerprint:  "fp",
				FirstEventID: "event_001",
				CreatedAt:    time.Now(),
			},
			wantError: true,
			errorMsg:  "level",
		},
		{
			name: "missing created_at",
			event: IssueCreated{
				BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeIssueCreated),
				IssueID:      "issue_123",
				ProjectID:    "project_456",
				WorkspaceID:  "ws_789",
				Title:        "Error",
				Level:        "error",
				Fingerprint:  "fp",
				FirstEventID: "event_001",
			},
			wantError: true,
			errorMsg:  "created_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("error message should contain %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCaseCreatedValidation(t *testing.T) {
	tests := []struct {
		name      string
		event     CaseCreated
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid event",
			event: CaseCreated{
				BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
				CaseID:       "case_123",
				WorkspaceID:  "ws_789",
				ContactEmail: "user@example.com",
				Title:        "Login issue",
				Priority:     "high",
				Channel:      "email",
				Source:       "support@example.com",
				CreatedAt:    time.Now(),
			},
			wantError: false,
		},
		{
			name: "invalid priority",
			event: CaseCreated{
				BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
				CaseID:       "case_123",
				WorkspaceID:  "ws_789",
				ContactEmail: "user@example.com",
				Title:        "Issue",
				Priority:     "super-urgent",
				Channel:      "email",
				CreatedAt:    time.Now(),
			},
			wantError: true,
			errorMsg:  "priority",
		},
		{
			name: "invalid channel",
			event: CaseCreated{
				BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
				CaseID:       "case_123",
				WorkspaceID:  "ws_789",
				ContactEmail: "user@example.com",
				Title:        "Issue",
				Priority:     "high",
				Channel:      "telegram",
				CreatedAt:    time.Now(),
			},
			wantError: true,
			errorMsg:  "channel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("error message should contain %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidationHelpers(t *testing.T) {
	t.Run("validateNonEmpty", func(t *testing.T) {
		if err := validateNonEmpty("test_field", "value"); err != nil {
			t.Errorf("expected no error for non-empty value, got: %v", err)
		}

		err := validateNonEmpty("test_field", "")
		if err == nil {
			t.Error("expected error for empty value")
		}
		if !strings.Contains(err.Error(), "test_field") {
			t.Errorf("error should mention field name, got: %v", err)
		}
	})

	t.Run("validateEnum", func(t *testing.T) {
		validOptions := []string{"red", "green", "blue"}

		if err := validateEnum("color", "red", validOptions); err != nil {
			t.Errorf("expected no error for valid enum value, got: %v", err)
		}

		err := validateEnum("color", "yellow", validOptions)
		if err == nil {
			t.Error("expected error for invalid enum value")
		}
		if !strings.Contains(err.Error(), "color") {
			t.Errorf("error should mention field name, got: %v", err)
		}
	})

	t.Run("contains", func(t *testing.T) {
		slice := []string{"apple", "banana", "cherry"}

		if !contains(slice, "banana") {
			t.Error("expected contains to return true for existing value")
		}

		if contains(slice, "orange") {
			t.Error("expected contains to return false for non-existing value")
		}
	})
}

func TestValidationError(t *testing.T) {
	t.Run("error with expected values", func(t *testing.T) {
		err := ValidationError{
			Field:    "status",
			Value:    "invalid",
			Expected: []string{"open", "closed", "pending"},
		}

		errMsg := err.Error()
		if !strings.Contains(errMsg, "status") {
			t.Errorf("error message should contain field name, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "invalid") {
			t.Errorf("error message should contain invalid value, got: %s", errMsg)
		}
	})

	t.Run("ErrRequiredField", func(t *testing.T) {
		err := ErrRequiredField("user_id")
		if !strings.Contains(err.Error(), "user_id") {
			t.Errorf("error should mention field name, got: %v", err)
		}
	})

	t.Run("ErrInvalidField", func(t *testing.T) {
		err := ErrInvalidField("level", "INVALID", []string{"error", "warning", "info"})
		errMsg := err.Error()
		if !strings.Contains(errMsg, "level") {
			t.Errorf("error should mention field name, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "INVALID") {
			t.Errorf("error should mention invalid value, got: %s", errMsg)
		}
	})
}
