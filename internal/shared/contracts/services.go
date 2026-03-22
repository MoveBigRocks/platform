package contracts

import (
	"context"
	"errors"
	"fmt"
	"time"

	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/pkg/eventbus"
)

// =============================================================================
// Shared Error Predicates
// These allow handlers to check store error types without importing stores.
// =============================================================================

// ErrAlreadyUsed indicates a one-time resource has already been consumed.
// This is the canonical sentinel; stores/shared re-exports it.
var ErrAlreadyUsed = errors.New("already used")

// IsAlreadyUsed checks if an error indicates a one-time resource was already consumed.
func IsAlreadyUsed(err error) bool {
	return errors.Is(err, ErrAlreadyUsed)
}

// OutboxPublisher is the minimal interface needed for event publishing.
// This dramatically simplifies testing - just 1 method to mock!
//
// MIGRATION GUIDE:
// All new code should use PublishEvent() with typed events.
// Publish() remains for dynamic/custom event payloads and incremental migration.
//
// TRANSACTION SUPPORT:
// Both methods support transactional publishing when called with a context
// containing an active database transaction. The outbox event will be saved
// within the same transaction as your business data, ensuring atomicity.
// Use TransactionRunner.WithTransaction() to wrap operations atomically.
type OutboxPublisher interface {
	// PublishEvent publishes a type-safe event.
	// This is the preferred method - provides compile-time type checking.
	// Events must implement the eventbus.Event interface (GetEventID, GetEventType, Validate).
	//
	// When ctx contains an active transaction (from TransactionRunner.WithTransaction),
	// the outbox event is saved within that transaction, providing atomic
	// "business data + event" writes (proper outbox pattern).
	PublishEvent(ctx context.Context, stream eventbus.Stream, event eventbus.Event) error

	// Publish publishes an event using the untyped interface for dynamic payloads.
	// Prefer PublishEvent when compile-time type safety is available.
	Publish(ctx context.Context, stream eventbus.Stream, event interface{}) error
}

// TransactionRunner provides transactional execution for services.
// Inject this into services that need to wrap multiple store operations
// (including outbox event publishing) in a single database transaction.
//
// Usage:
//
//	err := svc.tx.WithTransaction(ctx, func(txCtx context.Context) error {
//	    if err := svc.caseStore.CreateCase(txCtx, caseObj); err != nil {
//	        return err
//	    }
//	    return svc.outbox.PublishEvent(txCtx, stream, event)
//	})
//
// If the function returns an error, all operations are rolled back atomically.
type TransactionRunner interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// AdminContextRunner allows running operations with admin context (bypassing RLS).
// This is typically used by event handlers that process events across all workspaces.
//
// Usage:
//
//	err := runner.WithAdminContext(ctx, func(adminCtx context.Context) error {
//	    return svc.ProcessCrossWorkspaceOperation(adminCtx)
//	})
type AdminContextRunner interface {
	WithAdminContext(ctx context.Context, fn func(ctx context.Context) error) error
}

// JobMetrics provides type-safe job statistics
type JobMetrics struct {
	ActiveJobs           int64         `json:"active_jobs"`
	CompletedJobs        int64         `json:"completed_jobs"`
	FailedJobs           int64         `json:"failed_jobs"`
	AverageExecutionTime time.Duration `json:"avg_execution_time"`
	SuccessRate          float64       `json:"success_rate"`
}

// RuleContext provides type-safe context for rule evaluation.
// Only one of Case, Contact, Issue, or Form should be set based on TargetType.
type RuleContext struct {
	TargetType string                     `json:"target_type"` // "case", "contact", "issue", "form"
	Case       *servicedomain.Case        `json:"case,omitempty"`
	Contact    *platformdomain.Contact    `json:"contact,omitempty"`
	Issue      *observabilitydomain.Issue `json:"issue,omitempty"`
	Form       *FormSubmittedEvent        `json:"form,omitempty"`
}

// Request/Response types for error monitoring

type IngestEventRequest struct {
	ProjectDSN string                 `json:"project_dsn"`
	EventData  map[string]interface{} `json:"event_data"`
}

type IngestResponse struct {
	Success bool   `json:"success"`
	EventID string `json:"event_id,omitempty"`
	IssueID string `json:"issue_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// =============================================================================
// Query Filter Types
// These live in contracts so handlers, services, resolvers, and stores can
// all reference the same filter types without handlers importing stores.
// =============================================================================

// CaseFilters contains filters for listing cases.
type CaseFilters struct {
	WorkspaceID string
	QueueID     string
	Status      string
	Priority    string
	AssignedTo  string
	Limit       int
	Offset      int
}

// IssueFilters contains filters for listing issues.
type IssueFilters struct {
	WorkspaceID string
	ProjectID   string
	Status      string
	Level       string
	Limit       int
	Offset      int
}

// =============================================================================
// Service Interfaces
// These interfaces allow cross-module dependencies without circular imports
// =============================================================================

// CaseServiceInterface defines the methods needed by automation to manage cases.
// This allows the automation module to call case operations through the service layer
// instead of going directly to stores, ensuring proper validation and event publishing.
//
// Note: CaseService in support/services implements this interface.
// Action handlers should use this interface to avoid direct store access.
type CaseServiceInterface interface {
	// Read operations
	GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error)

	// Case mutations - each handles validation and event publishing
	AssignCase(ctx context.Context, caseID, userID, teamID string) error
	UnassignCase(ctx context.Context, caseID string) error
	SetCasePriority(ctx context.Context, caseID string, priority servicedomain.CasePriority) error
	SetCaseStatus(ctx context.Context, caseID string, status servicedomain.CaseStatus) error
	AddCaseTag(ctx context.Context, caseID, tag string) error
	RemoveCaseTag(ctx context.Context, caseID, tag string) error
	SetCaseCategory(ctx context.Context, caseID, category string) error
	CloseCase(ctx context.Context, caseID string) error
	MarkCaseResolved(ctx context.Context, caseID string, resolvedAt time.Time) error

	// Communications
	AddInternalNote(ctx context.Context, caseID, workspaceID, userID, userName, body string) (*servicedomain.Communication, error)
	CreateCommunication(ctx context.Context, comm *servicedomain.Communication) error

	// Case creation - action handlers should build the case and call SaveCase
	// This is simpler than matching the exact params struct signatures
	SaveCase(ctx context.Context, caseObj *servicedomain.Case) error
}

// ContactServiceInterface defines the methods needed for contact lookup.
// Used by the rules engine to get contact context for rule evaluation.
type ContactServiceInterface interface {
	GetContact(ctx context.Context, workspaceID, contactID string) (*platformdomain.Contact, error)
	GetContactByEmail(ctx context.Context, workspaceID, email string) (*platformdomain.Contact, error)
}

// =============================================================================
// Form Events
// =============================================================================

// FormSubmittedEvent is published when a form is submitted
// Used to trigger automation rules and other async processing
type FormSubmittedEvent struct {
	eventbus.BaseEvent
	FormID         string                 `json:"form_id"`
	FormSlug       string                 `json:"form_slug"`
	SubmissionID   string                 `json:"submission_id"`
	WorkspaceID    string                 `json:"workspace_id"`
	SubmitterEmail string                 `json:"submitter_email,omitempty"`
	SubmitterName  string                 `json:"submitter_name,omitempty"`
	Data           map[string]interface{} `json:"data"`
}

func (e FormSubmittedEvent) Validate() error {
	if e.EventID == "" {
		return fmt.Errorf("event_id is required")
	}
	if e.EventType.IsZero() {
		return fmt.Errorf("event_type is required")
	}
	if e.FormID == "" {
		return fmt.Errorf("form_id is required")
	}
	if e.SubmissionID == "" {
		return fmt.Errorf("submission_id is required")
	}
	if e.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	return nil
}

func NewFormSubmittedEvent(
	formID, formSlug, submissionID, workspaceID, submitterEmail, submitterName string,
	data map[string]interface{},
) FormSubmittedEvent {
	return FormSubmittedEvent{
		BaseEvent:      eventbus.NewBaseEvent(eventbus.TypeFormSubmitted),
		FormID:         formID,
		FormSlug:       formSlug,
		SubmissionID:   submissionID,
		WorkspaceID:    workspaceID,
		SubmitterEmail: submitterEmail,
		SubmitterName:  submitterName,
		Data:           data,
	}
}

// CreateCaseFromFormEvent is published when a case should be created from a form submission
type CreateCaseFromFormEvent struct {
	FormID       string                 `json:"form_id"`
	SubmissionID string                 `json:"submission_id"`
	WorkspaceID  string                 `json:"workspace_id"`
	Subject      string                 `json:"subject"`
	Description  string                 `json:"description"`
	Priority     string                 `json:"priority,omitempty"`
	CaseType     string                 `json:"case_type,omitempty"`
	TeamID       string                 `json:"team_id,omitempty"`
	AssigneeID   string                 `json:"assignee_id,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	ContactEmail string                 `json:"contact_email,omitempty"`
	ContactName  string                 `json:"contact_name,omitempty"`
	FormData     map[string]interface{} `json:"form_data"`
	Timestamp    time.Time              `json:"timestamp"`
}
