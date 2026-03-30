package shareddomain

import (
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
)

// Error Monitoring Domain Events

// IssueCreated is published when a new issue is created
type IssueCreated struct {
	eventbus.BaseEvent

	// Event payload
	IssueID      string
	ProjectID    string
	WorkspaceID  string
	Title        string
	Level        string
	Fingerprint  string
	FirstEventID string
	Platform     string
	Culprit      string
	CreatedAt    time.Time
}

func NewIssueCreatedEvent(issueID, projectID, workspaceID, title, level, fingerprint, firstEventID, platform, culprit string) IssueCreated {
	return IssueCreated{
		BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeIssueCreated),
		IssueID:      issueID,
		ProjectID:    projectID,
		WorkspaceID:  workspaceID,
		Title:        title,
		Level:        level,
		Fingerprint:  fingerprint,
		FirstEventID: firstEventID,
		Platform:     platform,
		Culprit:      culprit,
		CreatedAt:    time.Now().UTC(),
	}
}

// IssueUpdated is published when an issue is updated
type IssueUpdated struct {
	eventbus.BaseEvent

	// Event payload
	IssueID     string
	ProjectID   string
	WorkspaceID string
	NewEventID  string
	HasNewUser  bool // Whether to atomically increment user_count
	LastSeen    time.Time
	UpdatedAt   time.Time
}

// NewIssueUpdatedEventWithUserFlag creates an IssueUpdated event with explicit user tracking.
// Use hasNewUser=true if the error event includes a new user that should increment the count.
func NewIssueUpdatedEventWithUserFlag(issueID, projectID, workspaceID, newEventID string, lastSeen time.Time, hasNewUser bool) IssueUpdated {
	return IssueUpdated{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeIssueUpdated),
		IssueID:     issueID,
		ProjectID:   projectID,
		WorkspaceID: workspaceID,
		NewEventID:  newEventID,
		HasNewUser:  hasNewUser,
		LastSeen:    lastSeen,
		UpdatedAt:   time.Now().UTC(),
	}
}

// IssueResolved is published when an issue is resolved
type IssueResolved struct {
	eventbus.BaseEvent

	// Event payload
	IssueID              string
	ProjectID            string
	WorkspaceID          string
	Resolution           string
	ResolutionNotes      string
	ResolvedInVersion    string
	ResolvedInCommit     string
	ResolvedBy           string
	ResolvedAt           time.Time
	AffectedCaseCount    int
	AutoCloseSystemCases bool
	NotifyContacts       bool
	SystemCaseIDs        []string
}

// ErrorEventIngested is published when a new error event is ingested
type ErrorEventIngested struct {
	eventbus.BaseEvent

	// Event payload
	ErrorEventID string
	ProjectID    string
	IssueID      string
	Level        string
	Message      string
	UserEmail    string
	Environment  string
	Platform     string
	Tags         map[string]string
	IngestedAt   time.Time
}

// Alert Management Events

// AlertTriggered is published when an alert condition is met
type AlertTriggered struct {
	eventbus.BaseEvent

	// Event payload
	AlertID     string
	AlertName   string
	IssueID     string
	ProjectID   string
	WorkspaceID string
	Condition   string
	Reason      string
	Threshold   float64
	ActualValue float64
	Severity    string
	Actions     []string
	Metadata    Metadata
	TriggeredAt time.Time
}

// AlertCreated is published when a new alert configuration is created
type AlertCreated struct {
	eventbus.BaseEvent

	// Event payload
	AlertID        string
	ProjectID      string
	WorkspaceID    string
	Name           string
	Description    string
	Enabled        bool
	ConditionCount int
	ActionCount    int
	CreatedBy      string
	CreatedAt      time.Time
}

// AlertUpdated is published when an alert configuration is updated
type AlertUpdated struct {
	eventbus.BaseEvent

	// Event payload
	AlertID     string
	ProjectID   string
	WorkspaceID string
	Name        string
	Enabled     bool
	UpdatedBy   string
	UpdatedAt   time.Time
}

// AlertDeleted is published when an alert is deleted
type AlertDeleted struct {
	eventbus.BaseEvent

	// Event payload
	AlertID     string
	ProjectID   string
	WorkspaceID string
	DeletedBy   string
	DeletedAt   time.Time
}

// AlertEvaluated is published when alert evaluation completes (whether triggered or not)
type AlertEvaluated struct {
	eventbus.BaseEvent

	// Event payload
	AlertID     string
	ProjectID   string
	IssueID     string
	Triggered   bool
	Reason      string
	InCooldown  bool
	EvaluatedAt time.Time
}

// AlertNotificationSent is published when a notification is sent
type AlertNotificationSent struct {
	eventbus.BaseEvent

	// Event payload
	AlertID   string
	ProjectID string
	IssueID   string
	Provider  string
	Success   bool
	Error     string
	Metadata  Metadata
	SentAt    time.Time
}

// Validation Methods

// Validate validates the IssueCreated event
func (e IssueCreated) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("project_id", e.ProjectID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateNonEmpty("title", e.Title); err != nil {
		return err
	}
	if err := validateNonEmpty("fingerprint", e.Fingerprint); err != nil {
		return err
	}

	validLevels := []string{"error", "warning", "info", "fatal", "debug"}
	if err := validateEnum("level", e.Level, validLevels); err != nil {
		return err
	}

	if e.CreatedAt.IsZero() {
		return ErrRequiredField("created_at")
	}

	return nil
}

// Validate validates the IssueUpdated event
func (e IssueUpdated) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("issue_id", e.IssueID); err != nil {
		return err
	}
	if err := validateNonEmpty("project_id", e.ProjectID); err != nil {
		return err
	}
	if e.UpdatedAt.IsZero() {
		return ErrRequiredField("updated_at")
	}
	return nil
}

// Validate validates the IssueResolved event
func (e IssueResolved) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("issue_id", e.IssueID); err != nil {
		return err
	}
	if err := validateNonEmpty("project_id", e.ProjectID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateNonEmpty("resolved_by", e.ResolvedBy); err != nil {
		return err
	}

	validResolutions := []string{"fixed", "wont_fix", "duplicate", "invalid", "archived"}
	if err := validateEnum("resolution", e.Resolution, validResolutions); err != nil {
		return err
	}

	if e.ResolvedAt.IsZero() {
		return ErrRequiredField("resolved_at")
	}

	return nil
}
