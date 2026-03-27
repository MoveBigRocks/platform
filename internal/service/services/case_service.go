package serviceapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// CaseService handles all case-related business logic
type CaseService struct {
	queueStore     shared.QueueStore
	queueItemStore shared.QueueItemStore
	caseStore      shared.CaseStore
	workspaceStore shared.WorkspaceStore
	outbox         contracts.OutboxPublisher
	tx             contracts.TransactionRunner // Optional: enables atomic case+event writes
	logger         *logger.Logger
}

// CaseServiceOption is a functional option for configuring CaseService
type CaseServiceOption func(*CaseService)

// WithTransactionRunner configures the transaction runner for atomic case + event creation.
// When set, CreateCase will use transactional writes to ensure case creation and event
// publishing are atomic - both succeed or both fail.
func WithTransactionRunner(tx contracts.TransactionRunner) CaseServiceOption {
	return func(cs *CaseService) {
		cs.tx = tx
	}
}

func WithQueueItemStore(queueItemStore shared.QueueItemStore) CaseServiceOption {
	return func(cs *CaseService) {
		cs.queueItemStore = queueItemStore
	}
}

// NewCaseService creates a new case service.
// The outbox parameter is required for publishing events (case replies, notifications).
// Pass nil only in tests where event publishing is not being tested.
func NewCaseService(queueStore shared.QueueStore, caseStore shared.CaseStore, workspaceStore shared.WorkspaceStore, outbox contracts.OutboxPublisher, opts ...CaseServiceOption) *CaseService {
	cs := &CaseService{
		queueStore:     queueStore,
		caseStore:      caseStore,
		workspaceStore: workspaceStore,
		outbox:         outbox,
		logger:         logger.New().WithField("service", "case"),
	}
	for _, opt := range opts {
		opt(cs)
	}
	return cs
}

// CreateCaseParams contains parameters for creating a case
type CreateCaseParams struct {
	WorkspaceID               string
	Subject                   string
	Description               string
	Priority                  servicedomain.CasePriority
	Channel                   servicedomain.CaseChannel
	Category                  string
	QueueID                   string
	OriginatingConversationID string
	ContactID                 string
	ContactName               string
	ContactEmail              string
	TeamID                    string
	AssignedToID              string
	Tags                      []string
	CustomFields              shareddomain.TypedCustomFields
}

// CaseHandoffParams captures the durable-work routing change applied to a case.
// The handoff acts on the case itself; queue items follow from the updated case.
type CaseHandoffParams struct {
	QueueID          string
	TeamID           string
	AssigneeID       string
	Reason           string
	PerformedByID    string
	PerformedByName  string
	PerformedByType  string
	OnBehalfOfUserID string
}

// CreateCase creates a new case with consistent defaults and validation.
// When a TransactionRunner is configured, the case and its creation event
// are written atomically (proper outbox pattern). Otherwise, event publishing
// is best-effort after the case is created.
func (cs *CaseService) CreateCase(ctx context.Context, params CreateCaseParams) (*servicedomain.Case, error) {
	// Validate required params upfront
	if params.WorkspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if params.Subject == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("subject", "required"))
	}
	if params.QueueID != "" && cs.queueStore != nil {
		queue, err := cs.queueStore.GetQueue(ctx, params.QueueID)
		if err != nil {
			return nil, apierrors.NotFoundError("queue", params.QueueID)
		}
		if queue.WorkspaceID != params.WorkspaceID {
			return nil, apierrors.NotFoundError("queue", params.QueueID)
		}
	}

	// Create case using domain constructor with business rule defaults
	caseObj := servicedomain.NewCaseWithDefaults(servicedomain.NewCaseParams{
		WorkspaceID:               params.WorkspaceID,
		Subject:                   params.Subject,
		Description:               params.Description,
		Priority:                  params.Priority,
		Channel:                   params.Channel,
		Category:                  params.Category,
		QueueID:                   params.QueueID,
		OriginatingConversationID: params.OriginatingConversationID,
		ContactID:                 params.ContactID,
		ContactName:               params.ContactName,
		ContactEmail:              params.ContactEmail,
		TeamID:                    params.TeamID,
		AssignedToID:              params.AssignedToID,
		Tags:                      params.Tags,
		CustomFields:              params.CustomFields,
	})

	// Generate HumanID using workspace prefix
	prefix := cs.getCasePrefix(ctx, params.WorkspaceID)
	caseObj.GenerateHumanID(prefix)

	// Validate using domain method
	if err := caseObj.Validate(); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "case validation failed")
	}

	// Use transactional write if available (atomic case + event)
	if cs.tx != nil && cs.outbox != nil {
		if err := cs.createCaseWithEvent(ctx, caseObj); err != nil {
			return nil, err
		}
		return caseObj, nil
	}

	// Non-transactional path for runtimes without event-backed transactions.
	if err := cs.caseStore.CreateCase(ctx, caseObj); err != nil {
		return nil, apierrors.DatabaseError("create case", err)
	}
	if err := cs.syncCaseQueueItem(ctx, caseObj); err != nil {
		return nil, err
	}

	// Publish case_created event for rule evaluation (best-effort, non-blocking)
	cs.publishCaseCreatedEvent(ctx, caseObj)

	return caseObj, nil
}

// createCaseWithEvent writes the case and its creation event atomically.
// Both succeed or both fail - no orphaned cases without events.
func (cs *CaseService) createCaseWithEvent(ctx context.Context, caseObj *servicedomain.Case) error {
	return cs.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		// Create case within transaction
		if err := cs.caseStore.CreateCase(txCtx, caseObj); err != nil {
			return apierrors.DatabaseError("create case", err)
		}
		if err := cs.syncCaseQueueItem(txCtx, caseObj); err != nil {
			return err
		}

		event := shareddomain.NewCaseCreatedEvent(
			caseObj.ID,
			caseObj.WorkspaceID,
			caseObj.ContactID,
			caseObj.ContactEmail,
			caseObj.Subject,
			caseObj.Description,
			caseObj.Priority,
			caseObj.Channel,
			"service",
			false, // isSystemCase
			false, // autoCreated
		)

		// Publish event within same transaction (outbox save uses txCtx)
		if err := cs.outbox.PublishEvent(txCtx, eventbus.StreamCaseEvents, event); err != nil {
			return apierrors.DatabaseError("publish case event", err)
		}

		return nil
	})
}

// publishCaseCreatedEvent publishes a CaseCreated event to trigger rules
func (cs *CaseService) publishCaseCreatedEvent(ctx context.Context, caseObj *servicedomain.Case) {
	if cs.outbox == nil {
		cs.logger.Debug("Skipping case created event - outbox not configured", "case_id", caseObj.ID)
		return
	}

	event := shareddomain.NewCaseCreatedEvent(
		caseObj.ID,
		caseObj.WorkspaceID,
		caseObj.ContactID,
		caseObj.ContactEmail,
		caseObj.Subject,
		caseObj.Description,
		caseObj.Priority,
		caseObj.Channel,
		"service",
		false, // isSystemCase
		false, // autoCreated
	)

	if err := cs.outbox.PublishEvent(ctx, eventbus.StreamCaseEvents, event); err != nil {
		// Log but don't fail the operation - event publishing is best-effort
		// The outbox pattern ensures eventual delivery
		cs.logger.WithError(err).Warn("Failed to publish case created event", "case_id", caseObj.ID)
	}
}

// CreateCaseFromEmail creates a case from an inbound email
func (cs *CaseService) CreateCaseFromEmail(ctx context.Context, workspaceID, subject, body, fromEmail, fromName string) (*servicedomain.Case, error) {
	return cs.CreateCase(ctx, CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      subject,
		Description:  body,
		Channel:      servicedomain.CaseChannelEmail,
		ContactEmail: fromEmail,
		ContactName:  fromName,
		Priority:     servicedomain.CasePriorityMedium,
	})
}

// CreateCaseFromInboundEmail creates a case from an inbound email and records the
// inbound message as the first case communication.
func (cs *CaseService) CreateCaseFromInboundEmail(ctx context.Context, email *servicedomain.InboundEmail) (*servicedomain.Case, *servicedomain.Communication, error) {
	if email == nil {
		return nil, nil, apierrors.NewValidationErrors(apierrors.NewValidationError("email", "required"))
	}

	caseObj, err := cs.CreateCaseFromEmail(
		ctx,
		email.WorkspaceID,
		email.Subject,
		inboundEmailBody(email),
		email.FromEmail,
		email.FromName,
	)
	if err != nil {
		return nil, nil, err
	}

	comm, updatedCase, err := cs.AddInboundEmailToCase(ctx, caseObj.ID, email)
	if err != nil {
		return nil, nil, err
	}
	return updatedCase, comm, nil
}

// CreateCaseFromForm creates a case from a form submission
func (cs *CaseService) CreateCaseFromForm(ctx context.Context, workspaceID, subject, description string, params CreateCaseParams) (*servicedomain.Case, error) {
	params.WorkspaceID = workspaceID
	params.Subject = subject
	params.Description = description
	params.Channel = servicedomain.CaseChannelWeb

	// Use provided priority or default
	if params.Priority == "" {
		params.Priority = servicedomain.CasePriorityMedium
	}

	return cs.CreateCase(ctx, params)
}

// CreateCaseFromAPI creates a case from an API request
func (cs *CaseService) CreateCaseFromAPI(ctx context.Context, params CreateCaseParams) (*servicedomain.Case, error) {
	params.Channel = servicedomain.CaseChannelAPI
	return cs.CreateCase(ctx, params)
}

// UpdateCase updates an existing case
func (cs *CaseService) UpdateCase(ctx context.Context, caseObj *servicedomain.Case) error {
	if caseObj.ID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("id", "required"))
	}

	caseObj.UpdatedAt = time.Now().UTC()

	// Validate using domain method
	if err := caseObj.Validate(); err != nil {
		return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "case validation failed")
	}

	if err := cs.caseStore.UpdateCase(ctx, caseObj); err != nil {
		return apierrors.DatabaseError("update case", err)
	}

	return nil
}

// GetCase retrieves a case by ID
func (cs *CaseService) GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error) {
	if caseID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("case_id", "required"))
	}

	caseObj, err := cs.caseStore.GetCase(ctx, caseID)
	if err != nil {
		return nil, apierrors.NotFoundError("case", caseID)
	}
	return caseObj, nil
}

// GetCaseInWorkspace retrieves a case only if it belongs to the specified workspace.
// Returns ErrNotFound if case doesn't exist OR belongs to different workspace.
// This prevents information disclosure about entity existence across workspaces.
func (cs *CaseService) GetCaseInWorkspace(ctx context.Context, workspaceID, caseID string) (*servicedomain.Case, error) {
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if caseID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("case_id", "required"))
	}

	caseObj, err := cs.caseStore.GetCaseInWorkspace(ctx, workspaceID, caseID)
	if err != nil {
		return nil, apierrors.NotFoundError("case", caseID)
	}

	// Layer 3 defense-in-depth (ADR-0003): verify store returned correct workspace
	// Guards against store implementation bugs - same error to prevent enumeration
	if caseObj.WorkspaceID != workspaceID {
		return nil, apierrors.NotFoundError("case", caseID)
	}

	return caseObj, nil
}

// GetCaseByHumanID retrieves a case by its human-readable ID (e.g., "ABC-2401-001")
func (cs *CaseService) GetCaseByHumanID(ctx context.Context, humanID string) (*servicedomain.Case, error) {
	if humanID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("human_id", "required"))
	}

	caseObj, err := cs.caseStore.GetCaseByHumanID(ctx, humanID)
	if err != nil {
		return nil, apierrors.NotFoundError("case", humanID)
	}
	return caseObj, nil
}

// DeleteCase deletes a case
func (cs *CaseService) DeleteCase(ctx context.Context, workspaceID, caseID string) error {
	if workspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if caseID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("case_id", "required"))
	}

	if err := cs.caseStore.DeleteCase(ctx, workspaceID, caseID); err != nil {
		return apierrors.DatabaseError("delete case", err)
	}
	if err := cs.deleteCaseQueueItem(ctx, caseID); err != nil {
		return err
	}
	return nil
}

// AddCommunication adds a communication to a case
func (cs *CaseService) AddCommunication(ctx context.Context, comm *servicedomain.Communication) error {
	if comm.CaseID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("case_id", "required"))
	}

	// Ensure communication has fresh timestamps before persistence.
	now := time.Now()
	comm.CreatedAt = now
	comm.UpdatedAt = now

	// Wrap communication creation and case timestamp update in a transaction
	// so both succeed or both fail — no orphaned communications.
	if cs.tx != nil {
		return cs.tx.WithTransaction(ctx, func(txCtx context.Context) error {
			if err := cs.caseStore.CreateCommunication(txCtx, comm); err != nil {
				return apierrors.DatabaseError("create communication", err)
			}
			caseObj, err := cs.GetCase(txCtx, comm.CaseID)
			if err != nil {
				return err
			}
			if err := cs.caseStore.UpdateCase(txCtx, caseObj); err != nil {
				return apierrors.DatabaseError("update case timestamp", err)
			}
			return nil
		})
	}

	// Fallback for tests without a transaction runner
	if err := cs.caseStore.CreateCommunication(ctx, comm); err != nil {
		return apierrors.DatabaseError("create communication", err)
	}
	caseObj, err := cs.GetCase(ctx, comm.CaseID)
	if err != nil {
		return err
	}
	if err := cs.caseStore.UpdateCase(ctx, caseObj); err != nil {
		return apierrors.DatabaseError("update case timestamp", err)
	}
	return nil
}

// AddInboundEmailToCase records an inbound email as a case communication and
// applies the standard customer-reply case state transitions.
func (cs *CaseService) AddInboundEmailToCase(ctx context.Context, caseID string, email *servicedomain.InboundEmail) (*servicedomain.Communication, *servicedomain.Case, error) {
	if strings.TrimSpace(caseID) == "" {
		return nil, nil, apierrors.NewValidationErrors(apierrors.NewValidationError("case_id", "required"))
	}
	if email == nil {
		return nil, nil, apierrors.NewValidationErrors(apierrors.NewValidationError("email", "required"))
	}

	caseObj, err := cs.GetCase(ctx, caseID)
	if err != nil {
		return nil, nil, err
	}

	comm := newInboundEmailCommunication(caseObj, email)
	oldStatus := caseObj.Status

	switch caseObj.Status {
	case servicedomain.CaseStatusPending:
		caseObj.Status = servicedomain.CaseStatusOpen
	case servicedomain.CaseStatusResolved, servicedomain.CaseStatusClosed:
		if err := caseObj.Reopen(); err != nil {
			return nil, nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "customer reply state transition failed")
		}
	}
	caseObj.IncrementMessageCount()

	if err := caseObj.Validate(); err != nil {
		return nil, nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "case validation failed")
	}

	run := func(txCtx context.Context) error {
		if err := cs.caseStore.CreateCommunication(txCtx, comm); err != nil {
			return apierrors.DatabaseError("create communication", err)
		}
		if err := cs.caseStore.UpdateCase(txCtx, caseObj); err != nil {
			return apierrors.DatabaseError("update case", err)
		}
		return nil
	}

	if cs.tx != nil {
		if err := cs.tx.WithTransaction(ctx, run); err != nil {
			return nil, nil, err
		}
	} else if err := run(ctx); err != nil {
		return nil, nil, err
	}

	if caseObj.Status != oldStatus {
		cs.publishCaseStatusChangedEvent(ctx, caseObj, oldStatus, caseObj.Status)
	}

	return comm, caseObj, nil
}

// =============================================================================
// CASE MUTATION HELPERS
// =============================================================================

// CaseMutator is a function that mutates a case. It receives the case and returns
// an error if the mutation fails (e.g., validation error from domain method).
type CaseMutator func(c *servicedomain.Case) error

// CaseEventPublisher is a function that publishes events after a successful case mutation.
// It receives the case in its final state after mutation and persistence.
type CaseEventPublisher func(c *servicedomain.Case)

// withCase is a helper that implements the common get-mutate-persist pattern.
// It fetches the case, applies the mutation via domain methods, validates, and persists.
// This eliminates the repetitive boilerplate across all case action methods.
func (cs *CaseService) withCase(ctx context.Context, caseID string, mutate CaseMutator) error {
	c, err := cs.GetCase(ctx, caseID)
	if err != nil {
		return err // Error already wrapped by GetCase
	}

	if err := mutate(c); err != nil {
		return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "case mutation failed")
	}

	return cs.UpdateCase(ctx, c)
}

// withCaseTrackingStatus captures old status before mutation.
// Use this when you need to detect and publish status transitions.
func (cs *CaseService) withCaseTrackingStatus(ctx context.Context, caseID string, mutate CaseMutator, publishWithOldStatus func(c *servicedomain.Case, oldStatus servicedomain.CaseStatus)) error {
	c, err := cs.GetCase(ctx, caseID)
	if err != nil {
		return err // Error already wrapped by GetCase
	}

	oldStatus := c.Status

	if err := mutate(c); err != nil {
		return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "case mutation failed")
	}

	if err := cs.UpdateCase(ctx, c); err != nil {
		return err // Error already wrapped by UpdateCase
	}

	// Publish events with old status context
	if publishWithOldStatus != nil {
		publishWithOldStatus(c, oldStatus)
	}

	return nil
}

// getCasePrefix retrieves the case prefix for a workspace.
// Uses the workspace ShortCode (e.g., "ac" for Acme).
// Returns lowercase prefix for use in human-readable case IDs (e.g., ac-2512-a3e9ef).
func (cs *CaseService) getCasePrefix(ctx context.Context, workspaceID string) string {
	// Get workspace ShortCode
	workspace, err := cs.workspaceStore.GetWorkspace(ctx, workspaceID)
	if err == nil && workspace != nil && workspace.ShortCode != "" {
		return strings.ToLower(workspace.ShortCode)
	}

	// Final fallback
	return "case"
}

// SaveCase persists a case object, creating it when it does not yet exist and
// updating it otherwise.
//
// Create-path writes still use the normal case creation flow so HumanID
// generation, validation, and case_created event publishing stay consistent.
func (cs *CaseService) SaveCase(ctx context.Context, caseObj *servicedomain.Case) error {
	// Generate HumanID if not set
	if caseObj.HumanID == "" {
		prefix := cs.getCasePrefix(ctx, caseObj.WorkspaceID)
		caseObj.GenerateHumanID(prefix)
	}

	// Validate using domain method
	if err := caseObj.Validate(); err != nil {
		return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "case validation failed")
	}

	_, err := cs.caseStore.GetCase(ctx, caseObj.ID)
	switch {
	case err == nil:
		if err := cs.caseStore.UpdateCase(ctx, caseObj); err != nil {
			return apierrors.DatabaseError("update case", err)
		}
		return nil
	case !errors.Is(err, shared.ErrNotFound):
		return apierrors.DatabaseError("get case", err)
	}

	if cs.tx != nil && cs.outbox != nil {
		if err := cs.createCaseWithEvent(ctx, caseObj); err != nil {
			return err
		}
		return nil
	}

	if err := cs.caseStore.CreateCase(ctx, caseObj); err != nil {
		return apierrors.DatabaseError("create case", err)
	}
	cs.publishCaseCreatedEvent(ctx, caseObj)
	return nil
}

// LinkIssueToCase links an issue to a case using domain logic
func (cs *CaseService) LinkIssueToCase(ctx context.Context, caseID, issueID, projectID string) error {
	return cs.withCase(ctx, caseID, func(c *servicedomain.Case) error {
		return c.LinkIssue(issueID, projectID)
	})
}

// UnlinkIssueFromCase unlinks an issue from a case using domain logic
func (cs *CaseService) UnlinkIssueFromCase(ctx context.Context, caseID, issueID string) error {
	return cs.withCase(ctx, caseID, func(c *servicedomain.Case) error {
		c.UnlinkIssue(issueID)
		return nil
	})
}

// MarkCaseResolved marks a case as resolved using domain logic
func (cs *CaseService) MarkCaseResolved(ctx context.Context, caseID string, resolvedAt time.Time) error {
	return cs.withCaseTrackingStatus(ctx, caseID,
		func(c *servicedomain.Case) error {
			return c.MarkResolved(resolvedAt)
		},
		func(c *servicedomain.Case, oldStatus servicedomain.CaseStatus) {
			cs.publishCaseStatusChangedEvent(ctx, c, oldStatus, c.Status)
			cs.publishCaseResolvedEvent(ctx, c)
		},
	)
}

// NotifyCaseContact marks that a contact has been notified about a case
func (cs *CaseService) NotifyCaseContact(ctx context.Context, caseID string, notifiedAt time.Time, template string) error {
	return cs.withCase(ctx, caseID, func(c *servicedomain.Case) error {
		c.NotifyContact(notifiedAt, template)
		return nil
	})
}

// AutoCloseResolvedCasesResult contains statistics from auto-close operation
type AutoCloseResolvedCasesResult struct {
	Processed int
	Closed    int
	Errors    int
}

// AutoCloseResolvedCases finds resolved cases older than the grace period and closes them.
// This implements the business rule: resolved cases auto-close after gracePeriod (e.g., 24 hours)
// if the customer hasn't responded.
func (cs *CaseService) AutoCloseResolvedCases(ctx context.Context, gracePeriod time.Duration, batchSize int) (*AutoCloseResolvedCasesResult, error) {
	result := &AutoCloseResolvedCasesResult{}

	// Calculate cutoff time: cases resolved before this time should be closed
	cutoffTime := time.Now().Add(-gracePeriod)

	// Get resolved cases that are older than the grace period
	cases, err := cs.caseStore.ListResolvedCasesForAutoClose(ctx, cutoffTime, batchSize)
	if err != nil {
		return result, apierrors.DatabaseError("list resolved cases for auto-close", err)
	}

	result.Processed = len(cases)

	// Close each case using domain logic
	for _, c := range cases {
		// Domain method handles status validation and tagging
		if !c.AutoClose() {
			continue
		}

		// Persist
		if err := cs.caseStore.UpdateCase(ctx, c); err != nil {
			result.Errors++
			continue
		}

		result.Closed++
	}

	return result, nil
}

// ==================== CASE ACTION METHODS ====================

// AssignCase assigns a case to a user and/or team
func (cs *CaseService) AssignCase(ctx context.Context, caseID, userID, teamID string) error {
	return cs.withCaseTrackingStatus(ctx, caseID,
		func(c *servicedomain.Case) error {
			return c.Assign(userID, teamID)
		},
		func(c *servicedomain.Case, oldStatus servicedomain.CaseStatus) {
			cs.publishCaseAssignedEvent(ctx, c, userID, teamID)
			if c.Status != oldStatus {
				cs.publishCaseStatusChangedEvent(ctx, c, oldStatus, c.Status)
			}
		},
	)
}

// HandoffCase moves durable work between teams and queues while preserving an
// audit trail on the case itself.
func (cs *CaseService) HandoffCase(ctx context.Context, caseID string, params CaseHandoffParams) error {
	if strings.TrimSpace(params.QueueID) == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("queue_id", "required"))
	}

	caseObj, err := cs.GetCase(ctx, caseID)
	if err != nil {
		return err
	}

	queue, err := cs.queueStore.GetQueue(ctx, strings.TrimSpace(params.QueueID))
	if err != nil {
		return apierrors.NotFoundError("queue", strings.TrimSpace(params.QueueID))
	}
	if queue.WorkspaceID != caseObj.WorkspaceID {
		return apierrors.NotFoundError("queue", strings.TrimSpace(params.QueueID))
	}

	previousQueueID := caseObj.QueueID
	previousTeamID := caseObj.TeamID
	previousAssigneeID := caseObj.AssignedToID
	newQueueID := strings.TrimSpace(params.QueueID)
	newTeamID := strings.TrimSpace(params.TeamID)
	newAssigneeID := strings.TrimSpace(params.AssigneeID)
	reason := strings.TrimSpace(params.Reason)

	run := func(txCtx context.Context) error {
		if newTeamID != "" || newAssigneeID != "" {
			if err := caseObj.Assign(newAssigneeID, newTeamID); err != nil {
				return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "case handoff failed")
			}
		}
		caseObj.QueueID = newQueueID
		caseObj.MessageCount++
		if err := cs.UpdateCase(txCtx, caseObj); err != nil {
			return err
		}
		if err := cs.syncCaseQueueItem(txCtx, caseObj); err != nil {
			return err
		}
		if err := cs.recordCaseHandoffCommunication(txCtx, caseObj, previousQueueID, previousTeamID, previousAssigneeID, params); err != nil {
			return err
		}
		return nil
	}

	if cs.tx != nil {
		if err := cs.tx.WithTransaction(ctx, run); err != nil {
			return err
		}
	} else if err := run(ctx); err != nil {
		return err
	}

	if previousTeamID != caseObj.TeamID || previousAssigneeID != caseObj.AssignedToID {
		cs.publishCaseAssignedEvent(ctx, caseObj, caseObj.AssignedToID, caseObj.TeamID)
	}
	if previousQueueID != caseObj.QueueID || reason != "" {
		cs.logger.Info("Case handed off",
			"case_id", caseObj.ID,
			"queue_id", caseObj.QueueID,
			"team_id", caseObj.TeamID,
			"assignee_id", caseObj.AssignedToID,
		)
	}
	return nil
}

// UnassignCase removes the assigned user from a case (keeps team)
func (cs *CaseService) UnassignCase(ctx context.Context, caseID string) error {
	return cs.withCase(ctx, caseID, func(c *servicedomain.Case) error {
		c.Unassign()
		return nil
	})
}

func (cs *CaseService) recordCaseHandoffCommunication(ctx context.Context, caseObj *servicedomain.Case, previousQueueID, previousTeamID, previousAssigneeID string, params CaseHandoffParams) error {
	body := buildCaseHandoffBody(previousQueueID, previousTeamID, previousAssigneeID, caseObj.QueueID, caseObj.TeamID, caseObj.AssignedToID, params)
	comm := servicedomain.NewCommunication(caseObj.ID, caseObj.WorkspaceID, shareddomain.CommTypeSystem, body)
	comm.Subject = "Case handoff"
	if strings.TrimSpace(params.PerformedByType) == "agent" {
		comm.FromAgentID = strings.TrimSpace(params.PerformedByID)
	} else {
		comm.FromUserID = strings.TrimSpace(params.PerformedByID)
	}
	now := time.Now().UTC()
	comm.CreatedAt = now
	comm.UpdatedAt = now
	if err := cs.caseStore.CreateCommunication(ctx, comm); err != nil {
		return apierrors.DatabaseError("create communication", err)
	}
	return nil
}

func buildCaseHandoffBody(previousQueueID, previousTeamID, previousAssigneeID, newQueueID, newTeamID, newAssigneeID string, params CaseHandoffParams) string {
	reason := strings.TrimSpace(params.Reason)
	lines := []string{"Case handed off."}
	if previousQueueID != newQueueID {
		lines = append(lines, fmt.Sprintf("Queue: %s -> %s", emptyLabel(previousQueueID), emptyLabel(newQueueID)))
	}
	if previousTeamID != newTeamID {
		lines = append(lines, fmt.Sprintf("Team: %s -> %s", emptyLabel(previousTeamID), emptyLabel(newTeamID)))
	}
	if previousAssigneeID != newAssigneeID {
		lines = append(lines, fmt.Sprintf("Assignee: %s -> %s", emptyLabel(previousAssigneeID), emptyLabel(newAssigneeID)))
	}
	if actor := strings.TrimSpace(params.PerformedByID); actor != "" {
		lines = append(lines, fmt.Sprintf("Performed by: %s %s", emptyLabel(strings.TrimSpace(params.PerformedByType)), actor))
	}
	if performedByName := strings.TrimSpace(params.PerformedByName); performedByName != "" {
		lines = append(lines, "Actor label: "+performedByName)
	}
	if strings.TrimSpace(params.PerformedByType) == "agent" {
		lines = append(lines, "Routing mode: delegated agent")
	} else {
		lines = append(lines, "Routing mode: direct user")
	}
	if onBehalf := strings.TrimSpace(params.OnBehalfOfUserID); onBehalf != "" {
		lines = append(lines, "On behalf of user: "+onBehalf)
	}
	if reason != "" {
		lines = append(lines, "Reason: "+reason)
	}
	return strings.Join(lines, "\n")
}

func emptyLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return strings.TrimSpace(value)
}

// SetCasePriority changes the priority of a case
func (cs *CaseService) SetCasePriority(ctx context.Context, caseID string, priority servicedomain.CasePriority) error {
	return cs.withCase(ctx, caseID, func(c *servicedomain.Case) error {
		return c.SetPriority(priority)
	})
}

// SetCaseStatus changes the status of a case with validation
func (cs *CaseService) SetCaseStatus(ctx context.Context, caseID string, status servicedomain.CaseStatus) error {
	return cs.withCaseTrackingStatus(ctx, caseID,
		func(c *servicedomain.Case) error {
			return c.SetStatus(status)
		},
		func(c *servicedomain.Case, oldStatus servicedomain.CaseStatus) {
			if c.Status != oldStatus {
				cs.publishCaseStatusChangedEvent(ctx, c, oldStatus, c.Status)
			}
		},
	)
}

// CloseCase closes a resolved case
func (cs *CaseService) CloseCase(ctx context.Context, caseID string) error {
	return cs.withCase(ctx, caseID, func(c *servicedomain.Case) error {
		return c.MarkClosed(time.Now())
	})
}

// ReopenCase reopens a resolved or closed case
func (cs *CaseService) ReopenCase(ctx context.Context, caseID string) error {
	return cs.withCase(ctx, caseID, func(c *servicedomain.Case) error {
		return c.Reopen()
	})
}

// AddCaseTag adds a tag to a case
func (cs *CaseService) AddCaseTag(ctx context.Context, caseID, tag string) error {
	return cs.withCase(ctx, caseID, func(c *servicedomain.Case) error {
		return c.AddTag(tag)
	})
}

// RemoveCaseTag removes a tag from a case
func (cs *CaseService) RemoveCaseTag(ctx context.Context, caseID, tag string) error {
	return cs.withCase(ctx, caseID, func(c *servicedomain.Case) error {
		return c.RemoveTag(tag)
	})
}

// SetCaseCategory sets the category of a case
func (cs *CaseService) SetCaseCategory(ctx context.Context, caseID, category string) error {
	return cs.withCase(ctx, caseID, func(c *servicedomain.Case) error {
		c.SetCategory(category)
		return nil
	})
}

// SetCaseQueue assigns or clears the queue for a case.
func (cs *CaseService) SetCaseQueue(ctx context.Context, caseID, queueID string) error {
	c, err := cs.GetCase(ctx, caseID)
	if err != nil {
		return err
	}
	if queueID == "" {
		c.QueueID = ""
	} else {
		if cs.queueStore == nil {
			return fmt.Errorf("queue store is not configured")
		}
		queue, err := cs.queueStore.GetQueue(ctx, queueID)
		if err != nil || queue == nil || queue.WorkspaceID != c.WorkspaceID {
			return fmt.Errorf("queue not found")
		}
		c.QueueID = queueID
	}
	if err := cs.UpdateCase(ctx, c); err != nil {
		return err
	}
	return cs.syncCaseQueueItem(ctx, c)
}

func (cs *CaseService) syncCaseQueueItem(ctx context.Context, caseObj *servicedomain.Case) error {
	if cs.queueItemStore == nil || caseObj == nil {
		return nil
	}
	if caseObj.QueueID == "" {
		return cs.deleteCaseQueueItem(ctx, caseObj.ID)
	}

	item, err := cs.queueItemStore.GetQueueItemByCaseID(ctx, caseObj.ID)
	switch {
	case err == nil && item != nil:
		if item.QueueID == caseObj.QueueID {
			return nil
		}
		if err := item.MoveToQueue(caseObj.QueueID); err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "queue item mutation failed")
		}
		if err := cs.queueItemStore.UpdateQueueItem(ctx, item); err != nil {
			return apierrors.DatabaseError("update queue item", err)
		}
		return nil
	case errors.Is(err, shared.ErrNotFound):
		item = servicedomain.NewCaseQueueItem(caseObj.WorkspaceID, caseObj.QueueID, caseObj.ID)
		if err := item.Validate(); err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "queue item validation failed")
		}
		if err := cs.queueItemStore.CreateQueueItem(ctx, item); err != nil {
			return apierrors.DatabaseError("create queue item", err)
		}
		return nil
	case err != nil:
		return apierrors.DatabaseError("get queue item", err)
	default:
		return nil
	}
}

func (cs *CaseService) deleteCaseQueueItem(ctx context.Context, caseID string) error {
	if cs.queueItemStore == nil || caseID == "" {
		return nil
	}
	if err := cs.queueItemStore.DeleteQueueItemByCaseID(ctx, caseID); err != nil && !errors.Is(err, shared.ErrNotFound) {
		return apierrors.DatabaseError("delete queue item", err)
	}
	return nil
}

// AddInternalNote adds an internal note to a case (visible only to agents)
func (cs *CaseService) AddInternalNote(ctx context.Context, caseID, workspaceID, userID, userName, body string) (*servicedomain.Communication, error) {
	// Create communication using domain constructor
	comm := servicedomain.NewCommunication(caseID, workspaceID, shareddomain.CommTypeNote, body)
	comm.Direction = shareddomain.DirectionInternal
	comm.IsInternal = true
	comm.FromUserID = userID
	comm.FromName = userName

	// Add communication through the service (handles timestamps and case update)
	if err := cs.AddCommunication(ctx, comm); err != nil {
		return nil, err // Error already wrapped by AddCommunication
	}

	return comm, nil
}

// ReplyToCaseParams contains parameters for replying to a case
type ReplyToCaseParams struct {
	CaseID      string
	WorkspaceID string
	UserID      string
	UserName    string
	UserEmail   string
	Body        string
	BodyHTML    string
	ToEmails    []string
	CCEmails    []string
	Subject     string
}

// ReplyToCase adds an agent reply to a case and sends the email via event bus
func (cs *CaseService) ReplyToCase(ctx context.Context, params ReplyToCaseParams) (*servicedomain.Communication, error) {
	// Get case to record first response if needed
	caseObj, err := cs.GetCase(ctx, params.CaseID)
	if err != nil {
		return nil, err // Error already wrapped by GetCase
	}

	// Create communication
	comm := servicedomain.NewCommunication(params.CaseID, params.WorkspaceID, shareddomain.CommTypeEmail, params.Body)
	comm.Direction = shareddomain.DirectionOutbound
	comm.IsInternal = false
	comm.FromUserID = params.UserID
	comm.FromName = params.UserName
	comm.FromEmail = params.UserEmail
	comm.ToEmails = params.ToEmails
	comm.CCEmails = params.CCEmails
	comm.Subject = params.Subject
	comm.BodyHTML = params.BodyHTML

	// Ensure communication has fresh timestamps before persistence.
	now := time.Now()
	comm.CreatedAt = now
	comm.UpdatedAt = now

	// Apply domain mutations before persisting
	caseObj.RecordFirstResponse(time.Now())
	caseObj.TransitionAfterAgentReply()
	caseObj.IncrementMessageCount()

	if err := caseObj.Validate(); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "case validation failed")
	}

	// Wrap communication creation and case state update in a single transaction
	// so both succeed or both fail atomically.
	if cs.tx != nil {
		if err := cs.tx.WithTransaction(ctx, func(txCtx context.Context) error {
			if err := cs.caseStore.CreateCommunication(txCtx, comm); err != nil {
				return apierrors.DatabaseError("create communication", err)
			}
			if err := cs.caseStore.UpdateCase(txCtx, caseObj); err != nil {
				return apierrors.DatabaseError("update case", err)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	} else {
		// Fallback for tests without a transaction runner
		if err := cs.caseStore.CreateCommunication(ctx, comm); err != nil {
			return nil, apierrors.DatabaseError("create communication", err)
		}
		if err := cs.caseStore.UpdateCase(ctx, caseObj); err != nil {
			return nil, apierrors.DatabaseError("update case", err)
		}
	}

	// Publish email send event via outbox for actual delivery
	cs.publishReplyEmailEvent(ctx, params, caseObj)

	return comm, nil
}

// publishReplyEmailEvent publishes a SendEmailRequestedEvent for case replies
func (cs *CaseService) publishReplyEmailEvent(ctx context.Context, params ReplyToCaseParams, caseObj *servicedomain.Case) {
	if cs.outbox == nil {
		cs.logger.Debug("Skipping reply email event - outbox not configured", "case_id", caseObj.ID)
		return
	}

	emailEvent := events.NewSendEmailRequestedEvent(
		params.WorkspaceID,
		"case-service",
		params.ToEmails,
		params.Subject,
		params.Body,
	)
	emailEvent.CcEmails = params.CCEmails
	emailEvent.HTMLContent = params.BodyHTML
	emailEvent.Category = "case-reply"
	emailEvent.CaseID = params.CaseID
	emailEvent.ReplyTo = params.UserEmail

	if err := cs.outbox.PublishEvent(ctx, eventbus.StreamEmailCommands, emailEvent); err != nil {
		cs.logger.WithError(err).Warn("Failed to publish reply email event", "case_id", params.CaseID)
	}
}

// GetCaseCommunications retrieves all communications for a case
func (cs *CaseService) GetCaseCommunications(ctx context.Context, caseID string) ([]*servicedomain.Communication, error) {
	comms, err := cs.caseStore.ListCaseCommunications(ctx, caseID)
	if err != nil {
		return nil, apierrors.DatabaseError("get case communications", err)
	}
	return comms, nil
}

// ==================== EVENT PUBLISHING HELPERS ====================

// Actor identifier for system/automated actions when no user context is available.
// Used in event publishing when actions are triggered by automation rules, workers,
// or API calls where the acting user isn't tracked through the call chain.
const systemActor = "system"

// publishCaseAssignedEvent publishes a CaseAssigned event
func (cs *CaseService) publishCaseAssignedEvent(ctx context.Context, caseObj *servicedomain.Case, userID, teamID string) {
	if cs.outbox == nil {
		cs.logger.Debug("Skipping case assigned event - outbox not configured", "case_id", caseObj.ID)
		return
	}

	event := shareddomain.NewCaseAssignedEvent(
		caseObj.ID,
		caseObj.WorkspaceID,
		userID,
		systemActor,
		teamID,
	)

	if err := cs.outbox.PublishEvent(ctx, eventbus.StreamCaseEvents, event); err != nil {
		cs.logger.WithError(err).Warn("Failed to publish case assigned event", "case_id", caseObj.ID)
	}
}

// publishCaseStatusChangedEvent publishes a CaseStatusChanged event
func (cs *CaseService) publishCaseStatusChangedEvent(ctx context.Context, caseObj *servicedomain.Case, oldStatus, newStatus servicedomain.CaseStatus) {
	if cs.outbox == nil {
		cs.logger.Debug("Skipping case status changed event - outbox not configured", "case_id", caseObj.ID)
		return
	}

	event := shareddomain.NewCaseStatusChangedEvent(
		caseObj.ID,
		caseObj.WorkspaceID,
		oldStatus,
		newStatus,
		"", // resolution
		systemActor,
	)

	if err := cs.outbox.PublishEvent(ctx, eventbus.StreamCaseEvents, event); err != nil {
		cs.logger.WithError(err).Warn("Failed to publish case status changed event", "case_id", caseObj.ID)
	}
}

// publishCaseResolvedEvent publishes a CaseResolved event
func (cs *CaseService) publishCaseResolvedEvent(ctx context.Context, caseObj *servicedomain.Case) {
	if cs.outbox == nil {
		cs.logger.Debug("Skipping case resolved event - outbox not configured", "case_id", caseObj.ID)
		return
	}

	var timeToResolve int64
	if !caseObj.CreatedAt.IsZero() && caseObj.ResolvedAt != nil {
		timeToResolve = int64(caseObj.ResolvedAt.Sub(caseObj.CreatedAt).Seconds())
	}

	event := shareddomain.NewCaseResolvedEvent(
		caseObj.ID,
		caseObj.WorkspaceID,
		"resolved",
		systemActor,
		timeToResolve,
	)

	if err := cs.outbox.PublishEvent(ctx, eventbus.StreamCaseEvents, event); err != nil {
		cs.logger.WithError(err).Warn("Failed to publish case resolved event", "case_id", caseObj.ID)
	}
}

// ListCases lists cases with filters
func (cs *CaseService) ListCases(ctx context.Context, filters shared.CaseFilters) ([]*servicedomain.Case, int, error) {
	return cs.caseStore.ListCases(ctx, filters)
}

// GetCasesByIDs retrieves multiple cases by their IDs
func (cs *CaseService) GetCasesByIDs(ctx context.Context, caseIDs []string) ([]*servicedomain.Case, error) {
	if len(caseIDs) == 0 {
		return []*servicedomain.Case{}, nil
	}
	return cs.caseStore.GetCasesByIDs(ctx, caseIDs)
}

// CreateCommunication creates a new communication for a case
// Alias for AddCommunication for API consistency
func (cs *CaseService) CreateCommunication(ctx context.Context, comm *servicedomain.Communication) error {
	return cs.AddCommunication(ctx, comm)
}

// ListCaseCommunications lists all communications for a case
// Alias for GetCaseCommunications for API consistency
func (cs *CaseService) ListCaseCommunications(ctx context.Context, caseID string) ([]*servicedomain.Communication, error) {
	return cs.GetCaseCommunications(ctx, caseID)
}

func newInboundEmailCommunication(caseObj *servicedomain.Case, email *servicedomain.InboundEmail) *servicedomain.Communication {
	comm := servicedomain.NewCommunication(caseObj.ID, caseObj.WorkspaceID, shareddomain.CommTypeEmail, inboundEmailBody(email))
	comm.Direction = shareddomain.DirectionInbound
	comm.IsInternal = false
	comm.FromEmail = email.FromEmail
	comm.FromName = email.FromName
	comm.ToEmails = append([]string(nil), email.ToEmails...)
	comm.CCEmails = append([]string(nil), email.CCEmails...)
	comm.BCCEmails = append([]string(nil), email.BCCEmails...)
	comm.Subject = email.Subject
	comm.BodyHTML = email.HTMLContent
	comm.MessageID = email.MessageID
	comm.InReplyTo = email.InReplyTo
	comm.References = append([]string(nil), email.References...)
	comm.AttachmentIDs = append([]string(nil), email.AttachmentIDs...)
	now := time.Now().UTC()
	comm.CreatedAt = now
	comm.UpdatedAt = now
	return comm
}

func inboundEmailBody(email *servicedomain.InboundEmail) string {
	if email == nil {
		return ""
	}
	if body := strings.TrimSpace(email.TextContent); body != "" {
		return body
	}
	return strings.TrimSpace(email.HTMLContent)
}
