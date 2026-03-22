package shareddomain

import (
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
)

// Service Domain Events

// CaseCreated is published when a new case is created
type CaseCreated struct {
	eventbus.BaseEvent

	// Event payload
	CaseID       string
	WorkspaceID  string
	ContactID    string
	ContactEmail string
	Title        string
	Description  string // Initial case description/message content
	Priority     CasePriority
	Channel      CaseChannel
	Source       string
	IsSystemCase bool
	AutoCreated  bool
	CreatedAt    time.Time
}

func NewCaseCreatedEvent(caseID, workspaceID, contactID, contactEmail, title, description string, priority CasePriority, channel CaseChannel, source string, isSystemCase, autoCreated bool) CaseCreated {
	return CaseCreated{
		BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeCaseCreated),
		CaseID:       caseID,
		WorkspaceID:  workspaceID,
		ContactID:    contactID,
		ContactEmail: contactEmail,
		Title:        title,
		Description:  description,
		Priority:     priority,
		Channel:      channel,
		Source:       source,
		IsSystemCase: isSystemCase,
		AutoCreated:  autoCreated,
		CreatedAt:    time.Now().UTC(),
	}
}

// CaseAssigned is published when a case is assigned
type CaseAssigned struct {
	eventbus.BaseEvent

	// Event payload
	CaseID      string
	WorkspaceID string
	AssignedTo  string
	AssignedBy  string
	TeamID      string
	AssignedAt  time.Time
}

func NewCaseAssignedEvent(caseID, workspaceID, assignedTo, assignedBy, teamID string) CaseAssigned {
	return CaseAssigned{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseAssigned),
		CaseID:      caseID,
		WorkspaceID: workspaceID,
		AssignedTo:  assignedTo,
		AssignedBy:  assignedBy,
		TeamID:      teamID,
		AssignedAt:  time.Now().UTC(),
	}
}

// CaseStatusChanged is published when case status changes
type CaseStatusChanged struct {
	eventbus.BaseEvent

	// Event payload
	CaseID      string
	WorkspaceID string
	OldStatus   CaseStatus
	NewStatus   CaseStatus
	Resolution  string
	ChangedBy   string
	ChangedAt   time.Time
}

func NewCaseStatusChangedEvent(caseID, workspaceID string, oldStatus, newStatus CaseStatus, resolution, changedBy string) CaseStatusChanged {
	return CaseStatusChanged{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseStatusChanged),
		CaseID:      caseID,
		WorkspaceID: workspaceID,
		OldStatus:   oldStatus,
		NewStatus:   newStatus,
		Resolution:  resolution,
		ChangedBy:   changedBy,
		ChangedAt:   time.Now().UTC(),
	}
}

// CaseResolved is published when a case is resolved
type CaseResolved struct {
	eventbus.BaseEvent

	// Event payload
	CaseID        string
	WorkspaceID   string
	Resolution    string
	ResolvedBy    string
	ResolvedAt    time.Time
	TimeToResolve int64
}

func NewCaseResolvedEvent(caseID, workspaceID, resolution, resolvedBy string, timeToResolve int64) CaseResolved {
	return CaseResolved{
		BaseEvent:     eventbus.NewBaseEvent(eventbus.TypeCaseResolved),
		CaseID:        caseID,
		WorkspaceID:   workspaceID,
		Resolution:    resolution,
		ResolvedBy:    resolvedBy,
		ResolvedAt:    time.Now().UTC(),
		TimeToResolve: timeToResolve,
	}
}

// CommunicationAdded is published when communication is added to a case
type CommunicationAdded struct {
	eventbus.BaseEvent

	// Event payload
	CommunicationID string
	CaseID          string
	WorkspaceID     string
	Direction       CommunicationDirection
	From            string
	To              string
	Subject         string
	HasAttachments  bool
	CreatedAt       time.Time
}

// Validation Methods

// Validate validates the CaseCreated event
func (e CaseCreated) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("case_id", e.CaseID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateNonEmpty("contact_email", e.ContactEmail); err != nil {
		return err
	}
	if err := validateNonEmpty("title", e.Title); err != nil {
		return err
	}

	if err := validateCasePriority("priority", e.Priority); err != nil {
		return err
	}

	if err := validateCaseChannel("channel", e.Channel); err != nil {
		return err
	}

	if e.CreatedAt.IsZero() {
		return ErrRequiredField("created_at")
	}

	return nil
}

// Validate validates the CaseAssigned event
func (e CaseAssigned) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("case_id", e.CaseID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateNonEmpty("assigned_to", e.AssignedTo); err != nil {
		return err
	}
	if err := validateNonEmpty("assigned_by", e.AssignedBy); err != nil {
		return err
	}
	if e.AssignedAt.IsZero() {
		return ErrRequiredField("assigned_at")
	}
	return nil
}

// Validate validates the CaseStatusChanged event
func (e CaseStatusChanged) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("case_id", e.CaseID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateCaseStatus("old_status", e.OldStatus); err != nil {
		return err
	}
	if err := validateCaseStatus("new_status", e.NewStatus); err != nil {
		return err
	}
	if err := validateNonEmpty("changed_by", e.ChangedBy); err != nil {
		return err
	}

	if e.ChangedAt.IsZero() {
		return ErrRequiredField("changed_at")
	}

	return nil
}

// Validate validates the CaseResolved event
func (e CaseResolved) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("case_id", e.CaseID); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}
	if err := validateNonEmpty("resolution", e.Resolution); err != nil {
		return err
	}
	if err := validateNonEmpty("resolved_by", e.ResolvedBy); err != nil {
		return err
	}
	if e.ResolvedAt.IsZero() {
		return ErrRequiredField("resolved_at")
	}
	if err := validateNonNegativeInt("time_to_resolve", int(e.TimeToResolve)); err != nil {
		return err
	}
	return nil
}
