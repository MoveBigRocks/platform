package automationservices

import (
	"context"
	"fmt"

	automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	"github.com/movebigrocks/platform/pkg/logger"
)

// EscalationActionHandler handles case escalation actions
// It uses CaseServiceInterface to ensure proper validation and event publishing
type EscalationActionHandler struct {
	caseService contracts.CaseServiceInterface
	logger      *logger.Logger
}

// NewEscalationActionHandler creates a new escalation handler
func NewEscalationActionHandler(caseService contracts.CaseServiceInterface) *EscalationActionHandler {
	return &EscalationActionHandler{
		caseService: caseService,
		logger:      logger.New().WithField("handler", "escalation_action"),
	}
}

// ActionTypes returns the action types this handler supports
func (h *EscalationActionHandler) ActionTypes() []string {
	return []string{"escalate_case", "escalate"}
}

// Handle executes the escalation action
func (h *EscalationActionHandler) Handle(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	caseObj, err := requireCaseContext(action.Type, ruleContext)
	if err != nil {
		return err
	}
	if h.caseService == nil {
		return fmt.Errorf("%s action requires case service", action.Type)
	}

	caseID := caseObj.ID
	workspaceID := caseObj.WorkspaceID
	escalateTo := action.Options.GetString("assign_to")

	oldPriority := caseObj.Priority
	newPriority := automationdomain.EscalatedCasePriority(oldPriority)

	// Use service methods - each handles its own validation and events
	if err := h.caseService.SetCasePriority(ctx, caseID, newPriority); err != nil {
		return fmt.Errorf("failed to escalate priority: %w", err)
	}

	// Add escalation tag
	if err := h.caseService.AddCaseTag(ctx, caseID, "escalated"); err != nil {
		h.logger.Warn("Failed to add escalation tag", "error", err)
	}

	// Reassign if specified
	if escalateTo != "" {
		if err := h.caseService.AssignCase(ctx, caseID, escalateTo, ""); err != nil {
			return fmt.Errorf("failed to reassign for escalation: %w", err)
		}
	}

	// Add escalation note via service
	escalationNote := automationdomain.EscalationNote(oldPriority, newPriority, escalateTo)

	if _, err := h.caseService.AddInternalNote(ctx, caseID, workspaceID, "", "Automation", escalationNote); err != nil {
		h.logger.Warn("Failed to add escalation note", "error", err)
	}

	h.logger.Info("Escalated case",
		"case_id", caseID,
		"from_priority", oldPriority,
		"to_priority", newPriority)

	result.Changes.SetBool("escalated", true)
	result.Changes.SetString("old_priority", string(oldPriority))
	result.Changes.SetString("new_priority", string(newPriority))
	if escalateTo != "" {
		result.Changes.SetString("escalated_to", escalateTo)
	}
	return nil
}
