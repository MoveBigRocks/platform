package automationhandlers

import (
	"context"
	"encoding/json"
	"fmt"

	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// CaseServiceInterface defines the methods needed to get cases for rule evaluation
type CaseServiceInterface interface {
	GetCase(ctx context.Context, caseID string) (*servicedomain.Case, error)
}

// RuleEvaluationHandler handles rule-triggering domain events and dispatches
// them into the rules engine.
type RuleEvaluationHandler struct {
	rulesEngine *automationservices.RulesEngine
	caseService *serviceapp.CaseService
	logger      *logger.Logger
}

// NewRuleEvaluationHandler creates a new rule evaluation handler
func NewRuleEvaluationHandler(
	rulesEngine *automationservices.RulesEngine,
	caseService *serviceapp.CaseService,
	log *logger.Logger,
) *RuleEvaluationHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &RuleEvaluationHandler{
		rulesEngine: rulesEngine,
		caseService: caseService,
		logger:      log,
	}
}

// HandleCaseCreated evaluates rules when a case is created
func (h *RuleEvaluationHandler) HandleCaseCreated(ctx context.Context, eventData []byte) error {
	var event shareddomain.CaseCreated
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal CaseCreated event: %w", err)
	}

	// Skip events with missing required fields
	if event.CaseID == "" || event.WorkspaceID == "" {
		h.logger.Debug("Skipping CaseCreated event with missing case_id or workspace_id")
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"case_id":      event.CaseID,
		"workspace_id": event.WorkspaceID,
	}).Info("Evaluating rules for case_created event")

	// Get the case to pass to rules engine
	caseObj, err := h.caseService.GetCase(ctx, event.CaseID)
	if err != nil {
		h.logger.WithError(err).WithField("case_id", event.CaseID).Warn("Failed to get case for rule evaluation")
		return nil // Don't fail - case processing should continue
	}

	// Evaluate rules for case creation — return error so event bus can retry
	if err := h.rulesEngine.EvaluateRulesForCase(ctx, caseObj, "case_created", nil); err != nil {
		h.logger.WithError(err).WithField("case_id", event.CaseID).Error("Rule evaluation failed")
		return fmt.Errorf("rule evaluation failed for case %s: %w", event.CaseID, err)
	}

	h.logger.WithField("case_id", event.CaseID).Debug("Rule evaluation completed for case_created")
	return nil
}

// HandleCaseAssigned evaluates rules when a case is assigned
func (h *RuleEvaluationHandler) HandleCaseAssigned(ctx context.Context, eventData []byte) error {
	var event shareddomain.CaseAssigned
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal CaseAssigned event: %w", err)
	}

	// Skip events with missing required fields
	if event.CaseID == "" || event.WorkspaceID == "" {
		h.logger.Debug("Skipping CaseAssigned event with missing case_id or workspace_id")
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"case_id":      event.CaseID,
		"workspace_id": event.WorkspaceID,
		"assigned_to":  event.AssignedTo,
	}).Info("Evaluating rules for case_assigned event")

	// Get the case
	caseObj, err := h.caseService.GetCase(ctx, event.CaseID)
	if err != nil {
		h.logger.WithError(err).WithField("case_id", event.CaseID).Warn("Failed to get case for rule evaluation")
		return nil
	}

	// Changes map for condition evaluation
	changes := map[string]interface{}{
		"assigned_to": event.AssignedTo,
		"team_id":     event.TeamID,
	}

	// Evaluate rules — return error so event bus can retry
	if err := h.rulesEngine.EvaluateRulesForCase(ctx, caseObj, "case_assigned", changes); err != nil {
		h.logger.WithError(err).WithField("case_id", event.CaseID).Error("Rule evaluation failed")
		return fmt.Errorf("rule evaluation failed for case %s: %w", event.CaseID, err)
	}

	h.logger.WithField("case_id", event.CaseID).Debug("Rule evaluation completed for case_assigned")
	return nil
}

// HandleCaseStatusChanged evaluates rules when case status changes
func (h *RuleEvaluationHandler) HandleCaseStatusChanged(ctx context.Context, eventData []byte) error {
	var event shareddomain.CaseStatusChanged
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal CaseStatusChanged event: %w", err)
	}

	// Skip events with missing required fields
	if event.CaseID == "" || event.WorkspaceID == "" {
		h.logger.Debug("Skipping CaseStatusChanged event with missing case_id or workspace_id")
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"case_id":    event.CaseID,
		"old_status": event.OldStatus,
		"new_status": event.NewStatus,
	}).Info("Evaluating rules for case_status_changed event")

	// Get the case
	caseObj, err := h.caseService.GetCase(ctx, event.CaseID)
	if err != nil {
		h.logger.WithError(err).WithField("case_id", event.CaseID).Warn("Failed to get case for rule evaluation")
		return nil
	}

	// Changes map for condition evaluation
	changes := map[string]interface{}{
		"old_status": event.OldStatus,
		"new_status": event.NewStatus,
		"status":     event.NewStatus,
	}

	// Evaluate rules — return error so event bus can retry
	if err := h.rulesEngine.EvaluateRulesForCase(ctx, caseObj, "case_status_changed", changes); err != nil {
		h.logger.WithError(err).WithField("case_id", event.CaseID).Error("Rule evaluation failed")
		return fmt.Errorf("rule evaluation failed for case %s: %w", event.CaseID, err)
	}

	h.logger.WithField("case_id", event.CaseID).Debug("Rule evaluation completed for case_status_changed")
	return nil
}

// HandleCaseResolved evaluates rules when a case is resolved
func (h *RuleEvaluationHandler) HandleCaseResolved(ctx context.Context, eventData []byte) error {
	var event shareddomain.CaseResolved
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal CaseResolved event: %w", err)
	}

	// Skip events with missing required fields
	if event.CaseID == "" || event.WorkspaceID == "" {
		h.logger.Debug("Skipping CaseResolved event with missing case_id or workspace_id")
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"case_id":         event.CaseID,
		"resolution":      event.Resolution,
		"time_to_resolve": event.TimeToResolve,
	}).Info("Evaluating rules for case_resolved event")

	// Get the case
	caseObj, err := h.caseService.GetCase(ctx, event.CaseID)
	if err != nil {
		h.logger.WithError(err).WithField("case_id", event.CaseID).Warn("Failed to get case for rule evaluation")
		return nil
	}

	// Changes map for condition evaluation
	changes := map[string]interface{}{
		"resolution":      event.Resolution,
		"time_to_resolve": event.TimeToResolve,
	}

	// Evaluate rules — return error so event bus can retry
	if err := h.rulesEngine.EvaluateRulesForCase(ctx, caseObj, "case_resolved", changes); err != nil {
		h.logger.WithError(err).WithField("case_id", event.CaseID).Error("Rule evaluation failed")
		return fmt.Errorf("rule evaluation failed for case %s: %w", event.CaseID, err)
	}

	h.logger.WithField("case_id", event.CaseID).Debug("Rule evaluation completed for case_resolved")
	return nil
}

// HandleFormSubmitted evaluates rules when a form submission is accepted.
func (h *RuleEvaluationHandler) HandleFormSubmitted(ctx context.Context, eventData []byte) error {
	var event contracts.FormSubmittedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal FormSubmitted event: %w", err)
	}

	if event.FormID == "" || event.SubmissionID == "" || event.WorkspaceID == "" {
		h.logger.Debug("Skipping FormSubmitted event with missing identifiers")
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"form_id":       event.FormID,
		"submission_id": event.SubmissionID,
		"workspace_id":  event.WorkspaceID,
	}).Info("Evaluating rules for form_submitted event")

	if err := h.rulesEngine.EvaluateRulesForForm(ctx, &event, "form_submitted"); err != nil {
		h.logger.WithError(err).WithField("submission_id", event.SubmissionID).Error("Rule evaluation failed")
		return fmt.Errorf("rule evaluation failed for form submission %s: %w", event.SubmissionID, err)
	}

	h.logger.WithField("submission_id", event.SubmissionID).Debug("Rule evaluation completed for form_submitted")
	return nil
}

// HandleRuleEvent dispatches rule-related events to the appropriate handler
// based on the event_type field. This provides type-safe routing instead of
// checking which fields are present, preventing accidental misrouting.
func (h *RuleEvaluationHandler) HandleRuleEvent(ctx context.Context, eventData []byte) error {
	// Parse just the event header to determine the type
	hdr, err := eventbus.ParseEventHeader(eventData)
	if err != nil {
		return fmt.Errorf("failed to parse event header: %w", err)
	}

	// Route to appropriate handler based on event type
	switch hdr.EventType {
	case eventbus.TypeCaseResolved:
		return h.HandleCaseResolved(ctx, eventData)
	case eventbus.TypeCaseStatusChanged:
		return h.HandleCaseStatusChanged(ctx, eventData)
	case eventbus.TypeCaseAssigned:
		return h.HandleCaseAssigned(ctx, eventData)
	case eventbus.TypeCaseCreated:
		return h.HandleCaseCreated(ctx, eventData)
	case eventbus.TypeFormSubmitted:
		return h.HandleFormSubmitted(ctx, eventData)
	default:
		// Unknown event type - skip silently (other events on stream)
		h.logger.Debug("Skipping non-rule event type", "event_type", hdr.EventType.String())
		return nil
	}
}

// RegisterHandlers registers the rule evaluation handler with the event bus
func (h *RuleEvaluationHandler) RegisterHandlers(subscribe func(stream eventbus.Stream, group, consumer string, handler func(context.Context, []byte) error) error) error {
	// Wrap handler with logging
	ruleEventHandler := EventHandlerMiddleware(h.logger, h.HandleRuleEvent)

	// Subscribe to case events for rule evaluation
	// Use a separate consumer group so case-handler and rule-evaluation both process events
	if err := subscribe(eventbus.StreamCaseEvents, "rule-evaluation", "consumer", ruleEventHandler); err != nil {
		return fmt.Errorf("failed to register rule evaluation handler: %w", err)
	}
	if err := subscribe(eventbus.StreamFormEvents, "rule-evaluation", "consumer", ruleEventHandler); err != nil {
		return fmt.Errorf("failed to register form rule evaluation handler: %w", err)
	}

	h.logger.Info("Rule evaluation handlers registered successfully")
	return nil
}

// Note: EventHandlerMiddleware is defined in job_handler.go
