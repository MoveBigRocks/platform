package shareddomain

import (
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
)

// Integration Events (Cross-Domain)

// IssueCaseLinked is published when an issue is linked to a case
type IssueCaseLinked struct {
	eventbus.BaseEvent

	// Event payload
	IssueID     string
	CaseID      string
	ProjectID   string
	WorkspaceID string
	ContactID   string
	LinkedBy    string
	LinkReason  string
	LinkedAt    time.Time
}

// IssueCaseUnlinked is published when an issue is unlinked from a case
type IssueCaseUnlinked struct {
	eventbus.BaseEvent

	// Event payload
	IssueID     string
	CaseID      string
	ProjectID   string
	WorkspaceID string
	UnlinkedBy  string
	UnlinkedAt  time.Time
}

// CaseCreatedForContact is published when a case is auto-created for a contact affected by an issue
type CaseCreatedForContact struct {
	eventbus.BaseEvent

	// Event payload
	ContactID    string
	ContactEmail string
	IssueID      string
	ProjectID    string
	WorkspaceID  string
	IssueTitle   string
	IssueLevel   string
	Priority     string
	CreatedAt    time.Time
}

// ContactNotificationRequired is published when contacts need to be notified
type ContactNotificationRequired struct {
	eventbus.BaseEvent

	// Event payload
	IssueID        string
	ProjectID      string
	WorkspaceID    string
	ContactIDs     []string
	NotifyType     string
	Template       string
	CustomMessage  string
	IncludeInEmail bool
	Metadata       Metadata
	CreatedAt      time.Time
}

// CasesBulkResolved is published when multiple cases are resolved in bulk
type CasesBulkResolved struct {
	eventbus.BaseEvent

	// Event payload
	IssueID         string
	ProjectID       string
	WorkspaceID     string
	CaseIDs         []string
	SystemCaseIDs   []string
	CustomerCaseIDs []string
	Resolution      string
	ResolvedAt      time.Time
}

// NotificationSent is published after a notification is sent
type NotificationSent struct {
	eventbus.BaseEvent

	// Event payload
	NotificationID string
	RecipientID    string
	RecipientEmail string
	Channel        string
	Template       string
	Success        bool
	Error          string
	SentAt         time.Time
}

// Validation Methods

// Validate validates the IssueCaseLinked event
func (e IssueCaseLinked) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("issue_id", e.IssueID); err != nil {
		return err
	}
	if err := validateNonEmpty("case_id", e.CaseID); err != nil {
		return err
	}
	if err := validateNonEmpty("project_id", e.ProjectID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateNonEmpty("linked_by", e.LinkedBy); err != nil {
		return err
	}

	if e.LinkedAt.IsZero() {
		return ErrRequiredField("linked_at")
	}

	return nil
}

// Validate validates the IssueCaseUnlinked event
func (e IssueCaseUnlinked) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("issue_id", e.IssueID); err != nil {
		return err
	}
	if err := validateNonEmpty("case_id", e.CaseID); err != nil {
		return err
	}
	if err := validateNonEmpty("project_id", e.ProjectID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateNonEmpty("unlinked_by", e.UnlinkedBy); err != nil {
		return err
	}
	if e.UnlinkedAt.IsZero() {
		return ErrRequiredField("unlinked_at")
	}
	return nil
}

// Validate validates the CaseCreatedForContact event
func (e CaseCreatedForContact) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("contact_id", e.ContactID); err != nil {
		return err
	}
	if err := validateEmailRequired("contact_email", e.ContactEmail); err != nil {
		return err
	}
	if err := validateNonEmpty("issue_id", e.IssueID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if e.CreatedAt.IsZero() {
		return ErrRequiredField("created_at")
	}
	return nil
}

// Validate validates the CasesBulkResolved event
func (e CasesBulkResolved) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("issue_id", e.IssueID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateNonEmpty("resolution", e.Resolution); err != nil {
		return err
	}
	if e.ResolvedAt.IsZero() {
		return ErrRequiredField("resolved_at")
	}
	return nil
}
