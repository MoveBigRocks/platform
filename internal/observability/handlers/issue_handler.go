package observabilityhandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/metrics"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	observabilityservices "github.com/movebigrocks/platform/internal/observability/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// IssueEventHandler handles issue-related events (IssueCreated, IssueUpdated, IssueResolved)
type IssueEventHandler struct {
	issueService *observabilityservices.IssueService
	logger       *logger.Logger
}

// NewIssueEventHandler creates a new issue event handler
func NewIssueEventHandler(issueService *observabilityservices.IssueService, log *logger.Logger) *IssueEventHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &IssueEventHandler{
		issueService: issueService,
		logger:       log,
	}
}

// HandleIssueCreated processes IssueCreated events
// This is where the actual storage write happens - decoupled from the grouping service!
// Uses atomic upsert to prevent race conditions when multiple workers process events
// with the same fingerprint concurrently.
func (h *IssueEventHandler) HandleIssueCreated(ctx context.Context, eventData []byte) error {
	// Parse the event
	var event shareddomain.IssueCreated
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueCreated event: %w", err)
	}

	h.logger.WithFields(map[string]interface{}{
		"issue_id":     event.IssueID,
		"project_id":   event.ProjectID,
		"workspace_id": event.WorkspaceID,
	}).Info("Processing IssueCreated event")

	// Reconstruct the issue from the event
	issue := &observabilitydomain.Issue{
		ID:          event.IssueID,
		WorkspaceID: event.WorkspaceID,
		ProjectID:   event.ProjectID,
		Title:       event.Title,
		Level:       event.Level,
		Fingerprint: event.Fingerprint,
		Platform:    event.Platform,
		Culprit:     event.Culprit,
		Status:      observabilitydomain.IssueStatusUnresolved,
		FirstSeen:   event.CreatedAt,
		LastSeen:    event.CreatedAt,
		EventCount:  1,
		UserCount:   0,
		LastEventID: event.FirstEventID,
		Tags:        make(map[string]string),
	}

	// 1. Use atomic upsert to prevent race conditions
	// If another worker already created an issue with the same fingerprint,
	// this will update the existing issue instead of failing or creating a duplicate.
	resultIssue, wasCreated, err := h.issueService.CreateOrUpdateIssueByFingerprint(ctx, issue)
	if err != nil {
		return fmt.Errorf("failed to upsert issue: %w", err)
	}

	// Use the resulting issue ID (may differ from event.IssueID if it was an update)
	actualIssueID := resultIssue.ID

	if !wasCreated {
		h.logger.WithFields(map[string]interface{}{
			"event_issue_id":  event.IssueID,
			"actual_issue_id": actualIssueID,
			"fingerprint":     event.Fingerprint,
		}).Info("Issue already existed, merged into existing issue (race condition handled)")
	}

	// 1b. Update the first event to link it to this issue
	if event.FirstEventID != "" {
		if err := h.issueService.LinkEventToIssue(ctx, event.WorkspaceID, event.FirstEventID, actualIssueID); err != nil {
			h.logger.WithError(err).Warn("Failed to update first event with issue ID")
			return fmt.Errorf("failed to link first event %s to issue %s: %w", event.FirstEventID, actualIssueID, err)
		}
	}

	// 2. Update metrics (only for new issues)
	if wasCreated {
		severity := event.Level
		if severity == "" {
			severity = "error"
		}
		// Using CasesCreated as a proxy for issues created for now
		metrics.CasesCreated.WithLabelValues(event.WorkspaceID, severity).Inc()
	}

	// 3. Check alert rules (simplified for now)
	// In production, this would check alert conditions and trigger notifications
	h.logger.WithField("issue_id", actualIssueID).Debug("Checking alert rules")

	// 4. Log success
	h.logger.WithFields(map[string]interface{}{
		"issue_id":    actualIssueID,
		"project_id":  event.ProjectID,
		"title":       event.Title,
		"was_created": wasCreated,
	}).Info("Successfully processed IssueCreated event")

	return nil
}

// HandleIssueUpdated processes IssueUpdated events
// Uses atomic increment to prevent race conditions when multiple concurrent events
// update the same issue simultaneously.
func (h *IssueEventHandler) HandleIssueUpdated(ctx context.Context, eventData []byte) error {
	// Parse the event
	var event shareddomain.IssueUpdated
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueUpdated event: %w", err)
	}

	h.logger.WithFields(map[string]interface{}{
		"issue_id":     event.IssueID,
		"project_id":   event.ProjectID,
		"has_new_user": event.HasNewUser,
	}).Info("Processing IssueUpdated event")

	// 1. Atomically update issue stats (event_count++, optionally user_count++)
	// This prevents the race condition where concurrent updates could overwrite
	// each other's event counts with stale values.
	issue, err := h.issueService.AtomicUpdateIssueStats(
		ctx,
		event.WorkspaceID,
		event.IssueID,
		event.NewEventID,
		event.LastSeen,
		event.HasNewUser,
	)
	if err != nil {
		return fmt.Errorf("failed to atomically update issue stats: %w", err)
	}

	// 2. Update the new event to link it to this issue
	if event.NewEventID != "" {
		// Check if event already has issue ID before linking
		newEvent, err := h.issueService.GetErrorEvent(ctx, event.NewEventID)
		if err == nil && newEvent != nil && newEvent.IssueID == "" {
			if err := h.issueService.LinkEventToIssue(ctx, event.WorkspaceID, event.NewEventID, event.IssueID); err != nil {
				h.logger.WithError(err).Warn("Failed to update new event with issue ID")
			}
		}
	}

	// 3. Update metrics
	// Track error events using existing ErrorsIngested metric
	project, err := h.issueService.GetProject(ctx, issue.ProjectID)
	if err == nil && project != nil {
		metrics.ErrorsIngested.WithLabelValues(project.WorkspaceID, issue.ProjectID).Inc()
	}

	// 4. Log success with actual counts from database (not stale event values)
	h.logger.WithFields(map[string]interface{}{
		"issue_id":    event.IssueID,
		"event_count": issue.EventCount,
		"user_count":  issue.UserCount,
	}).Info("Successfully processed IssueUpdated event")

	return nil
}

// HandleIssueResolved processes IssueResolved events
func (h *IssueEventHandler) HandleIssueResolved(ctx context.Context, eventData []byte) error {
	// Parse the event
	var event shareddomain.IssueResolved
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal IssueResolved event: %w", err)
	}

	h.logger.WithFields(map[string]interface{}{
		"issue_id":   event.IssueID,
		"project_id": event.ProjectID,
		"resolution": event.Resolution,
	}).Info("Processing IssueResolved event")

	// 1. Get the issue
	issue, err := h.issueService.GetIssue(ctx, event.IssueID)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}

	// 2. Update issue status
	issue.Status = observabilitydomain.IssueStatusResolved
	resolvedAt := event.ResolvedAt
	issue.ResolvedAt = &resolvedAt

	// 3. Save the updated issue
	if err := h.issueService.UpdateIssue(ctx, issue); err != nil {
		return fmt.Errorf("failed to update issue: %w", err)
	}

	// 4. Update metrics (using existing metrics as proxies)
	// Note: In production, we'd add specific IssuesResolved metric
	metrics.CasesCreated.WithLabelValues(event.WorkspaceID, "resolved").Inc()

	// 5. Log success
	h.logger.WithFields(map[string]interface{}{
		"issue_id":       event.IssueID,
		"resolution":     event.Resolution,
		"affected_cases": event.AffectedCaseCount,
	}).Info("Successfully processed IssueResolved event")

	return nil
}

// RegisterHandlers registers all issue event handlers with the event bus
// This is called during application startup
func (h *IssueEventHandler) RegisterHandlers(subscribe func(stream eventbus.Stream, group, consumer string, handler func(context.Context, []byte) error) error) error {
	// Wrap handlers with middleware
	issueCreatedHandler := EventHandlerMiddleware(h.logger, h.HandleIssueCreated)
	issueUpdatedHandler := EventHandlerMiddleware(h.logger, h.HandleIssueUpdated)
	issueResolvedHandler := EventHandlerMiddleware(h.logger, h.HandleIssueResolved)

	// Each event type gets its own consumer group to ensure all handlers receive all events
	// Register IssueCreated handler
	if err := subscribe(eventbus.StreamIssueEvents, "issue-created", "consumer", issueCreatedHandler); err != nil {
		return fmt.Errorf("failed to register IssueCreated handler: %w", err)
	}

	// Register IssueUpdated handler
	if err := subscribe(eventbus.StreamIssueEvents, "issue-updated", "consumer", issueUpdatedHandler); err != nil {
		return fmt.Errorf("failed to register IssueUpdated handler: %w", err)
	}

	// Register IssueResolved handler
	if err := subscribe(eventbus.StreamIssueEvents, "issue-resolved", "consumer", issueResolvedHandler); err != nil {
		return fmt.Errorf("failed to register IssueResolved handler: %w", err)
	}

	h.logger.Info("Issue event handlers registered successfully")
	return nil
}

// EventHandlerMiddleware wraps event handlers with error handling and logging
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
