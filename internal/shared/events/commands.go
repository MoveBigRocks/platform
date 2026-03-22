// Package events defines cross-domain event types for event-driven communication
// between bounded contexts. These events enable loose coupling between domains
// without direct service dependencies.
package events

import (
	"fmt"
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
)

// Stream names for command events (requests to perform actions)
const (
	// StreamEmailCommands contains requests to send emails
	StreamEmailCommands = "email-commands"
	// StreamCaseCommands contains requests to create/modify cases
	StreamCaseCommands = "case-commands"
	// StreamNotificationCommands contains requests to send notifications
	StreamNotificationCommands = "notification-commands"
)

// SendEmailRequestedEvent is published when a domain needs to send an email
// without directly depending on the email service.
// Consumers: EmailCommandHandler in service domain
type SendEmailRequestedEvent struct {
	eventbus.BaseEvent

	// Request metadata
	WorkspaceID string
	RequestedAt time.Time
	RequestedBy string // Service/component that requested

	// Email content
	ToEmails    []string
	CcEmails    []string
	Subject     string
	TextContent string
	HTMLContent string

	// Context
	Category     string // "rule", "form", "system", "notification"
	CaseID       string
	SourceRuleID string
	SourceFormID string

	// Options
	Priority     string // "high", "normal", "low"
	ReplyTo      string
	TemplateID   string
	TemplateData map[string]interface{}
}

// Validate implements eventbus.Event
func (e SendEmailRequestedEvent) Validate() error {
	if e.EventID == "" {
		return fmt.Errorf("event_id is required")
	}
	if e.EventType.IsZero() {
		return fmt.Errorf("event_type is required")
	}
	if e.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if len(e.ToEmails) == 0 {
		return fmt.Errorf("at least one recipient email is required")
	}
	if e.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if e.TextContent == "" && e.HTMLContent == "" && e.TemplateID == "" {
		return fmt.Errorf("content or template_id is required")
	}
	return nil
}

// NewSendEmailRequestedEvent creates a new type-safe SendEmailRequestedEvent
func NewSendEmailRequestedEvent(workspaceID, requestedBy string, toEmails []string, subject, textContent string) SendEmailRequestedEvent {
	return SendEmailRequestedEvent{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeSendEmailRequested),
		WorkspaceID: workspaceID,
		RequestedAt: time.Now().UTC(),
		RequestedBy: requestedBy,
		ToEmails:    toEmails,
		Subject:     subject,
		TextContent: textContent,
	}
}

// CreateCaseRequestedEvent is published when a domain needs to create a case
// without directly depending on the case service.
// Consumers: CaseCommandHandler in service domain
type CreateCaseRequestedEvent struct {
	eventbus.BaseEvent

	// Request metadata
	WorkspaceID string
	RequestedAt time.Time
	RequestedBy string // Service/component that requested

	// Case details
	Subject     string
	Description string
	Priority    string // "low", "medium", "high", "urgent"
	Channel     string // "web", "email", "api", "form"

	// Contact info
	ContactEmail string
	ContactName  string
	ContactPhone string

	// Assignment
	TeamID       string
	AssignedToID string

	// Categorization
	Category string
	Tags     []string

	// Source context
	SourceType         string // "form", "rule", "email", "api"
	SourceID           string // Form ID, Rule ID, etc.
	SourceSubmissionID string // Form submission ID
	Metadata           map[string]interface{}

	// Custom fields
	CustomFields map[string]interface{}
}

// Validate implements eventbus.Event
func (e CreateCaseRequestedEvent) Validate() error {
	if e.EventID == "" {
		return fmt.Errorf("event_id is required")
	}
	if e.EventType.IsZero() {
		return fmt.Errorf("event_type is required")
	}
	if e.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if e.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if e.ContactEmail == "" {
		return fmt.Errorf("contact_email is required")
	}
	return nil
}

// NewCreateCaseRequestedEvent creates a new type-safe CreateCaseRequestedEvent
func NewCreateCaseRequestedEvent(workspaceID, requestedBy, subject, contactEmail string) CreateCaseRequestedEvent {
	return CreateCaseRequestedEvent{
		BaseEvent:    eventbus.NewBaseEvent(eventbus.TypeCreateCaseRequested),
		WorkspaceID:  workspaceID,
		RequestedAt:  time.Now().UTC(),
		RequestedBy:  requestedBy,
		Subject:      subject,
		ContactEmail: contactEmail,
	}
}

// SendNotificationRequestedEvent is published when a domain needs to send
// a notification (email, webhook, etc.) without directly depending on
// notification services.
// Consumers: NotificationCommandHandler
type SendNotificationRequestedEvent struct {
	eventbus.BaseEvent

	// Request metadata
	WorkspaceID string
	RequestedAt time.Time
	RequestedBy string

	// Notification type
	Type string // "email", "webhook", "in_app"

	// Recipients
	Recipients []string // Email addresses or user IDs

	// Content
	Subject  string
	Body     string
	Template string
	Data     map[string]interface{}

	// Context
	SourceType string // "form", "case", "rule"
	SourceID   string
}

// Validate implements eventbus.Event
func (e SendNotificationRequestedEvent) Validate() error {
	if e.EventID == "" {
		return fmt.Errorf("event_id is required")
	}
	if e.EventType.IsZero() {
		return fmt.Errorf("event_type is required")
	}
	if e.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if len(e.Recipients) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	if e.Type == "" {
		return fmt.Errorf("notification type is required")
	}
	return nil
}

// NewSendNotificationRequestedEvent creates a new type-safe SendNotificationRequestedEvent
func NewSendNotificationRequestedEvent(workspaceID, requestedBy, notifType string, recipients []string) SendNotificationRequestedEvent {
	return SendNotificationRequestedEvent{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeSendNotificationRequested),
		WorkspaceID: workspaceID,
		RequestedAt: time.Now().UTC(),
		RequestedBy: requestedBy,
		Type:        notifType,
		Recipients:  recipients,
	}
}

// CaseCreatedFromCommandEvent is published after a case is successfully created
// from a CreateCaseRequestedEvent. This allows the original requester to be
// notified of the result.
type CaseCreatedFromCommandEvent struct {
	eventbus.BaseEvent

	// Original request reference
	RequestID   string
	RequestedBy string

	// Created case details
	CaseID      string
	HumanID     string // e.g., ac-2512-a3e9ef
	WorkspaceID string
	CreatedAt   time.Time

	// Source context (echoed from request)
	SourceType         string
	SourceID           string
	SourceSubmissionID string
}

// Validate implements eventbus.Event
func (e CaseCreatedFromCommandEvent) Validate() error {
	if e.EventID == "" {
		return fmt.Errorf("event_id is required")
	}
	if e.EventType.IsZero() {
		return fmt.Errorf("event_type is required")
	}
	if e.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}
	if e.CaseID == "" {
		return fmt.Errorf("case_id is required")
	}
	if e.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	return nil
}

// NewCaseCreatedFromCommandEvent creates a new CaseCreatedFromCommandEvent
func NewCaseCreatedFromCommandEvent(requestID, requestedBy, caseID, workspaceID, humanID string) CaseCreatedFromCommandEvent {
	return CaseCreatedFromCommandEvent{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCaseCreatedFromCommand),
		RequestID:   requestID,
		RequestedBy: requestedBy,
		CaseID:      caseID,
		HumanID:     humanID,
		WorkspaceID: workspaceID,
		CreatedAt:   time.Now().UTC(),
	}
}
