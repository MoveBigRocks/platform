package servicehandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/metrics"
	observabilityservices "github.com/movebigrocks/platform/internal/observability/services"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// CaseServiceInterface defines the methods needed from CaseService for event handling
type CaseServiceInterface interface {
	LinkIssueToCase(ctx context.Context, caseID, issueID, projectID string) error
	UnlinkIssueFromCase(ctx context.Context, caseID, issueID string) error
	CreateCaseForIssue(ctx context.Context, params observabilityservices.CreateCaseForIssueParams) (*servicedomain.Case, error)
	MarkCaseResolved(ctx context.Context, caseID string, resolvedAt time.Time) error
	GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error)
	UpdateCase(ctx context.Context, c *servicedomain.Case) error
}

// ContactServiceInterface defines the methods needed from ContactService for event handling
type ContactServiceInterface interface {
	CreateContact(ctx context.Context, params platformservices.CreateContactParams) (*platformdomain.Contact, error)
}

// CaseNotificationServiceInterface defines the methods needed from CaseNotificationService
type CaseNotificationServiceInterface interface {
	NotifyCaseCreated(ctx context.Context, caseObj *servicedomain.Case) error
}

// CaseEventHandler handles case-related events
type CaseEventHandler struct {
	caseService         CaseServiceInterface
	contactService      ContactServiceInterface
	notificationService CaseNotificationServiceInterface
	adminRunner         contracts.AdminContextRunner
	logger              *logger.Logger
}

// NewCaseEventHandler creates a new case event handler
func NewCaseEventHandler(
	caseService CaseServiceInterface,
	contactService ContactServiceInterface,
	notificationService CaseNotificationServiceInterface,
	adminRunner contracts.AdminContextRunner,
	log *logger.Logger,
) *CaseEventHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &CaseEventHandler{
		caseService:         caseService,
		contactService:      contactService,
		notificationService: notificationService,
		adminRunner:         adminRunner,
		logger:              log,
	}
}

// HandleIssueCaseLinked processes IssueCaseLinked events
func (h *CaseEventHandler) HandleIssueCaseLinked(ctx context.Context, eventData []byte) error {
	var event shareddomain.IssueCaseLinked
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueCaseLinked event: %w", err)
	}

	// Validate required fields
	if event.CaseID == "" || event.IssueID == "" {
		return fmt.Errorf("IssueCaseLinked event missing case_id or issue_id")
	}

	// Skip events without LinkedBy - these are informational updates only
	if event.LinkedBy == "" {
		h.logger.WithFields(map[string]interface{}{
			"issue_id": event.IssueID,
			"case_id":  event.CaseID,
		}).Debug("Skipping IssueCaseLinked event with empty LinkedBy")
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"issue_id": event.IssueID,
		"case_id":  event.CaseID,
	}).Info("Processing IssueCaseLinked event")

	// Use admin context to bypass RLS - workers process events across all workspaces
	err := h.adminRunner.WithAdminContext(ctx, func(adminCtx context.Context) error {
		return h.caseService.LinkIssueToCase(adminCtx, event.CaseID, event.IssueID, event.ProjectID)
	})
	if err != nil {
		return fmt.Errorf("failed to link issue to case: %w", err)
	}

	h.logger.WithFields(map[string]interface{}{
		"issue_id": event.IssueID,
		"case_id":  event.CaseID,
	}).Info("Successfully processed IssueCaseLinked event")
	return nil
}

// HandleIssueCaseUnlinked processes IssueCaseUnlinked events
func (h *CaseEventHandler) HandleIssueCaseUnlinked(ctx context.Context, eventData []byte) error {
	var event shareddomain.IssueCaseUnlinked
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueCaseUnlinked event: %w", err)
	}

	// Validate required fields
	if event.CaseID == "" || event.IssueID == "" {
		return fmt.Errorf("IssueCaseUnlinked event missing case_id or issue_id")
	}

	// Skip events without UnlinkedBy - these are informational updates only
	if event.UnlinkedBy == "" {
		h.logger.WithFields(map[string]interface{}{
			"issue_id": event.IssueID,
			"case_id":  event.CaseID,
		}).Debug("Skipping IssueCaseUnlinked event with empty UnlinkedBy")
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"issue_id": event.IssueID,
		"case_id":  event.CaseID,
	}).Info("Processing IssueCaseUnlinked event")

	// Use admin context to bypass RLS - workers process events across all workspaces
	err := h.adminRunner.WithAdminContext(ctx, func(adminCtx context.Context) error {
		return h.caseService.UnlinkIssueFromCase(adminCtx, event.CaseID, event.IssueID)
	})
	if err != nil {
		return fmt.Errorf("failed to unlink issue from case: %w", err)
	}

	h.logger.WithFields(map[string]interface{}{
		"issue_id": event.IssueID,
		"case_id":  event.CaseID,
	}).Info("Successfully processed IssueCaseUnlinked event")
	return nil
}

// HandleCaseCreatedForContact processes CaseCreatedForContact events
func (h *CaseEventHandler) HandleCaseCreatedForContact(ctx context.Context, eventData []byte) error {
	var event shareddomain.CaseCreatedForContact
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal CaseCreatedForContact event: %w", err)
	}

	// Skip events with missing required fields
	if event.ContactEmail == "" || event.IssueID == "" {
		h.logger.Debug("Skipping CaseCreatedForContact event with missing contact_email or issue_id")
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"contact_id":    event.ContactID,
		"issue_id":      event.IssueID,
		"contact_email": event.ContactEmail,
	}).Info("Processing CaseCreatedForContact event")

	// Use admin context to bypass RLS - workers process events across all workspaces
	var c *servicedomain.Case
	err := h.adminRunner.WithAdminContext(ctx, func(adminCtx context.Context) error {
		// Delegate to specialized service method - encapsulates all business logic
		var createErr error
		c, createErr = h.caseService.CreateCaseForIssue(adminCtx, observabilityservices.CreateCaseForIssueParams{
			WorkspaceID:  event.WorkspaceID,
			IssueID:      event.IssueID,
			ProjectID:    event.ProjectID,
			IssueTitle:   event.IssueTitle,
			IssueLevel:   event.IssueLevel,
			Priority:     servicedomain.CasePriority(event.Priority),
			ContactID:    event.ContactID,
			ContactEmail: event.ContactEmail,
		})
		if createErr != nil {
			return createErr
		}

		// Send admin notifications (non-blocking - log errors but don't fail)
		if h.notificationService != nil {
			if notifyErr := h.notificationService.NotifyCaseCreated(adminCtx, c); notifyErr != nil {
				h.logger.WithError(notifyErr).WithField("case_id", c.ID).Warn("Failed to send case notification")
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create case for issue: %w", err)
	}

	// Update metrics
	metrics.CasesCreated.WithLabelValues(event.WorkspaceID, event.Priority).Inc()

	h.logger.WithFields(map[string]interface{}{
		"contact_id": event.ContactID,
		"case_id":    c.ID,
		"issue_id":   event.IssueID,
	}).Info("Successfully processed CaseCreatedForContact event")
	return nil
}

// HandleCasesBulkResolved processes CasesBulkResolved events
func (h *CaseEventHandler) HandleCasesBulkResolved(ctx context.Context, eventData []byte) error {
	var event shareddomain.CasesBulkResolved
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal CasesBulkResolved event: %w", err)
	}

	caseIDs := make([]string, 0, len(event.CaseIDs)+len(event.SystemCaseIDs)+len(event.CustomerCaseIDs))
	seen := make(map[string]struct{}, len(event.CaseIDs)+len(event.SystemCaseIDs)+len(event.CustomerCaseIDs))
	appendCaseID := func(caseID string) {
		if caseID == "" {
			return
		}
		if _, ok := seen[caseID]; ok {
			return
		}
		seen[caseID] = struct{}{}
		caseIDs = append(caseIDs, caseID)
	}
	for _, caseID := range event.CaseIDs {
		appendCaseID(caseID)
	}
	for _, caseID := range event.SystemCaseIDs {
		appendCaseID(caseID)
	}
	for _, caseID := range event.CustomerCaseIDs {
		appendCaseID(caseID)
	}

	// Skip events with no cases to resolve
	if len(caseIDs) == 0 {
		h.logger.Debug("Skipping CasesBulkResolved event with no cases to resolve")
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"issue_id":    event.IssueID,
		"total_cases": len(caseIDs),
	}).Info("Processing CasesBulkResolved event")

	var (
		totalResolved int
		failedCases   []string
	)

	// Use admin context to bypass RLS - workers process events across all workspaces
	if err := h.adminRunner.WithAdminContext(ctx, func(adminCtx context.Context) error {
		// Helper function to resolve a single case
		resolveCase := func(caseID string) {
			if err := h.caseService.MarkCaseResolved(adminCtx, caseID, event.ResolvedAt); err != nil {
				h.logger.WithError(err).WithField("case_id", caseID).Warn("Failed to resolve case")
				failedCases = append(failedCases, caseID)
				return
			}

			// Mark issue as resolved in the case
			c, err := h.caseService.GetCase(adminCtx, caseID)
			if err != nil {
				h.logger.WithError(err).WithField("case_id", caseID).Warn("Failed to get case for issue resolution")
				failedCases = append(failedCases, caseID)
				return
			}

			c.MarkIssueResolved(event.ResolvedAt)
			if err := h.caseService.UpdateCase(adminCtx, c); err != nil {
				h.logger.WithError(err).WithField("case_id", caseID).Warn("Failed to update case with issue resolution")
				failedCases = append(failedCases, caseID)
				return
			}

			totalResolved++
		}

		for _, caseID := range caseIDs {
			resolveCase(caseID)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to establish admin context for bulk resolution: %w", err)
	}

	// Metrics tracking - can be added when Prometheus dashboards are set up
	// metrics.CasesResolved.WithLabelValues(event.WorkspaceID).Add(float64(totalResolved))

	// Log summary with any failures
	logFields := map[string]interface{}{
		"issue_id":       event.IssueID,
		"total_resolved": totalResolved,
		"total_failed":   len(failedCases),
	}
	if len(failedCases) > 0 {
		logFields["failed_case_ids"] = failedCases
		h.logger.WithFields(logFields).Warn("CasesBulkResolved completed with some failures")
	} else {
		h.logger.WithFields(logFields).Info("Successfully processed CasesBulkResolved event")
	}

	return nil
}

// HandleContactCreatedFromEmail processes ContactCreatedFromEmail events
func (h *CaseEventHandler) HandleContactCreatedFromEmail(ctx context.Context, eventData []byte) error {
	var event shareddomain.ContactCreatedFromEmail
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal ContactCreatedFromEmail event: %w", err)
	}

	// Skip events with missing required fields
	if event.Email == "" {
		h.logger.Debug("Skipping ContactCreatedFromEmail event with missing email")
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"contact_id": event.ContactID,
		"email":      event.Email,
	}).Info("Processing ContactCreatedFromEmail event")

	// Use admin context to bypass RLS - workers process events across all workspaces
	var contact *platformdomain.Contact
	err := h.adminRunner.WithAdminContext(ctx, func(adminCtx context.Context) error {
		// Delegate to ContactService with proper validation and business logic
		var createErr error
		contact, createErr = h.contactService.CreateContact(adminCtx, platformservices.CreateContactParams{
			WorkspaceID: event.WorkspaceID,
			Email:       event.Email,
			Name:        event.Name,
			Company:     event.Organization,
			Source:      "email",
			Metadata: map[string]interface{}{
				"first_email_id":    event.EmailID,
				"original_event_id": event.ContactID,
			},
		})
		return createErr
	})

	if err != nil {
		return fmt.Errorf("failed to create contact: %w", err)
	}

	// Metrics tracking - ContactsCreated counter
	// metrics.ContactsCreated.WithLabelValues(event.WorkspaceID).Inc()

	h.logger.WithFields(map[string]interface{}{
		"contact_id": contact.ID,
		"email":      event.Email,
	}).Info("Successfully processed ContactCreatedFromEmail event")
	return nil
}

// HandleCaseEvent is a dispatcher that routes case events to the appropriate handler
// based on the event_type field. This provides type-safe routing instead of checking
// which fields are present, preventing accidental misrouting when event schemas change.
func (h *CaseEventHandler) HandleCaseEvent(ctx context.Context, eventData []byte) error {
	// Parse just the event header to determine the type
	hdr, err := eventbus.ParseEventHeader(eventData)
	if err != nil {
		return fmt.Errorf("failed to parse case event header: %w", err)
	}

	// Route to appropriate handler based on event type
	switch hdr.EventType {
	case eventbus.TypeCasesBulkResolved:
		return nil
	case eventbus.TypeCaseCreatedForContact:
		return nil
	// NOTE: TypeCaseCreated is NOT handled here to avoid duplicate case creation.
	// CaseService.CreateCase already inserts the case and publishes the event.
	// RuleEvaluationHandler handles CaseCreated events for automation rules.
	default:
		h.logger.Debug("Unknown case event type, skipping",
			"event_type", hdr.EventType.String(),
			"event_id", hdr.EventID)
		return nil
	}
}

// HandleErrorTrackingCaseEvent dispatches error-tracking case integration events.
// These are owned by the error-tracking extension runtime rather than the core worker manager.
func (h *CaseEventHandler) HandleErrorTrackingCaseEvent(ctx context.Context, eventData []byte) error {
	hdr, err := eventbus.ParseEventHeader(eventData)
	if err != nil {
		return fmt.Errorf("failed to parse error-tracking case event header: %w", err)
	}

	switch hdr.EventType {
	case eventbus.TypeIssueCaseLinked:
		return h.HandleIssueCaseLinked(ctx, eventData)
	case eventbus.TypeIssueCaseUnlinked:
		return h.HandleIssueCaseUnlinked(ctx, eventData)
	case eventbus.TypeCaseCreatedForContact:
		return h.HandleCaseCreatedForContact(ctx, eventData)
	case eventbus.TypeCasesBulkResolved:
		return h.HandleCasesBulkResolved(ctx, eventData)
	default:
		h.logger.Debug("Unknown error-tracking case event type, skipping",
			"event_type", hdr.EventType.String(),
			"event_id", hdr.EventID)
		return nil
	}
}

// HandleIssueLinkEvent delegates to the error-tracking case-event handler.
func (h *CaseEventHandler) HandleIssueLinkEvent(ctx context.Context, eventData []byte) error {
	return h.HandleErrorTrackingCaseEvent(ctx, eventData)
}

// RegisterHandlers registers all case event handlers with the event bus
func (h *CaseEventHandler) RegisterHandlers(subscribe func(stream eventbus.Stream, group, consumer string, handler func(context.Context, []byte) error) error) error {
	// Use a single dispatcher for case-events to avoid race conditions
	// with filesystem-based event bus (multiple handlers would race for same file)
	caseEventDispatcher := EventHandlerMiddleware(h.logger, h.HandleCaseEvent)
	contactCreatedFromEmailHandler := EventHandlerMiddleware(h.logger, h.HandleContactCreatedFromEmail)

	// Register single dispatcher for all case events
	caseStream := eventbus.StreamCaseEvents

	if err := subscribe(caseStream, "case-events-handler", "consumer", caseEventDispatcher); err != nil {
		return fmt.Errorf("failed to register case events dispatcher: %w", err)
	}

	// Register email event handler for contact creation (separate stream)
	emailStream := eventbus.StreamEmailEvents

	if err := subscribe(emailStream, "contact-created-from-email", "consumer", contactCreatedFromEmailHandler); err != nil {
		return fmt.Errorf("failed to register ContactCreatedFromEmail handler: %w", err)
	}

	h.logger.Info("Case event handlers registered successfully")
	return nil
}

// EventHandlerMiddleware wraps event handlers with logging and error handling
func EventHandlerMiddleware(logger *logger.Logger, handler func(context.Context, []byte) error) func(context.Context, []byte) error {
	return func(ctx context.Context, data []byte) error {
		start := time.Now()

		err := handler(ctx, data)

		duration := time.Since(start)
		if err != nil {
			logger.WithError(err).WithField("duration_ms", duration.Milliseconds()).Error("Event handler failed")
			return err
		}

		logger.WithField("duration_ms", duration.Milliseconds()).Debug("Event handler completed")
		return nil
	}
}
