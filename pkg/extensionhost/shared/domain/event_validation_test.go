package shareddomain

import (
	"strings"
	"testing"
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
)

// TestIssueUpdatedValidation tests IssueUpdated validation
func TestIssueUpdatedValidation(t *testing.T) {
	tests := []struct {
		name      string
		event     IssueUpdated
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid event",
			event: IssueUpdated{
				BaseEvent:  eventbus.NewBaseEvent(eventbus.TypeIssueUpdated),
				IssueID:    "issue_123",
				ProjectID:  "project_456",
				NewEventID: "event_789",
				EventCount: 10,
				UserCount:  5,
				LastSeen:   time.Now(),
				UpdatedAt:  time.Now(),
			},
			wantError: false,
		},
		{
			name: "missing issue_id",
			event: IssueUpdated{
				BaseEvent:  eventbus.NewBaseEvent(eventbus.TypeIssueUpdated),
				ProjectID:  "project_456",
				NewEventID: "event_789",
				UpdatedAt:  time.Now(),
			},
			wantError: true,
			errorMsg:  "issue_id",
		},
		{
			name: "missing project_id",
			event: IssueUpdated{
				BaseEvent: eventbus.NewBaseEvent(eventbus.TypeIssueUpdated),
				IssueID:   "issue_123",
				UpdatedAt: time.Now(),
			},
			wantError: true,
			errorMsg:  "project_id",
		},
		{
			name: "missing updated_at",
			event: IssueUpdated{
				BaseEvent: eventbus.NewBaseEvent(eventbus.TypeIssueUpdated),
				IssueID:   "issue_123",
				ProjectID: "project_456",
			},
			wantError: true,
			errorMsg:  "updated_at",
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

// TestIssueResolvedValidation tests IssueResolved validation
func TestIssueResolvedValidation(t *testing.T) {
	tests := []struct {
		name      string
		event     IssueResolved
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid event",
			event: IssueResolved{
				BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueResolved),
				IssueID:     "issue_123",
				ProjectID:   "project_456",
				WorkspaceID: "ws_789",
				Resolution:  "fixed",
				ResolvedBy:  "user_123",
				ResolvedAt:  time.Now(),
			},
			wantError: false,
		},
		{
			name: "invalid resolution",
			event: IssueResolved{
				BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueResolved),
				IssueID:     "issue_123",
				ProjectID:   "project_456",
				WorkspaceID: "ws_789",
				Resolution:  "maybe_fixed",
				ResolvedBy:  "user_123",
				ResolvedAt:  time.Now(),
			},
			wantError: true,
			errorMsg:  "resolution",
		},
		{
			name: "missing resolved_by",
			event: IssueResolved{
				BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueResolved),
				IssueID:     "issue_123",
				ProjectID:   "project_456",
				WorkspaceID: "ws_789",
				Resolution:  "fixed",
				ResolvedAt:  time.Now(),
			},
			wantError: true,
			errorMsg:  "resolved_by",
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

// TestCaseStatusChangedValidation tests CaseStatusChanged validation
func TestCaseStatusChangedValidation(t *testing.T) {
	tests := []struct {
		name      string
		event     CaseStatusChanged
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid status change",
			event: CaseStatusChanged{
				BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseStatusChanged),
				CaseID:      "case_123",
				WorkspaceID: "ws_789",
				OldStatus:   CaseStatusOpen,
				NewStatus:   CaseStatusPending,
				ChangedBy:   "user_123",
				ChangedAt:   time.Now(),
			},
			wantError: false,
		},
		{
			name: "invalid old_status",
			event: CaseStatusChanged{
				BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseStatusChanged),
				CaseID:      "case_123",
				WorkspaceID: "ws_789",
				OldStatus:   CaseStatus("invalid_status"),
				NewStatus:   CaseStatusClosed,
				ChangedBy:   "user_123",
				ChangedAt:   time.Now(),
			},
			wantError: true,
			errorMsg:  "old_status",
		},
		{
			name: "invalid new_status",
			event: CaseStatusChanged{
				BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseStatusChanged),
				CaseID:      "case_123",
				WorkspaceID: "ws_789",
				OldStatus:   CaseStatusOpen,
				NewStatus:   CaseStatus("unknown"),
				ChangedBy:   "user_123",
				ChangedAt:   time.Now(),
			},
			wantError: true,
			errorMsg:  "new_status",
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

// TestCaseResolvedValidation tests CaseResolved validation
func TestCaseResolvedValidation(t *testing.T) {
	tests := []struct {
		name      string
		event     CaseResolved
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid case resolved",
			event: CaseResolved{
				BaseEvent:     eventbus.NewBaseEvent(eventbus.TypeCaseResolved),
				CaseID:        "case_123",
				WorkspaceID:   "ws_789",
				Resolution:    "Issue fixed in production",
				ResolvedBy:    "user_123",
				ResolvedAt:    time.Now(),
				TimeToResolve: 3600,
			},
			wantError: false,
		},
		{
			name: "negative time_to_resolve",
			event: CaseResolved{
				BaseEvent:     eventbus.NewBaseEvent(eventbus.TypeCaseResolved),
				CaseID:        "case_123",
				WorkspaceID:   "ws_789",
				Resolution:    "Fixed",
				ResolvedBy:    "user_123",
				ResolvedAt:    time.Now(),
				TimeToResolve: -100,
			},
			wantError: true,
			errorMsg:  "time_to_resolve",
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
