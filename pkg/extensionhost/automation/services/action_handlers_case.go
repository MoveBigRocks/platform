package automationservices

import (
	"context"
	"fmt"
	"strings"

	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// CaseActionHandler handles case-related actions (assign, status, priority, tags, close, custom fields)
// It uses CaseServiceInterface instead of direct store access to ensure:
// - Domain validation runs on all mutations
// - Events are published correctly (CaseAssigned, CaseStatusChanged, etc.)
// - Business logic isn't duplicated
type CaseActionHandler struct {
	caseService contracts.CaseServiceInterface
	logger      *logger.Logger
}

// NewCaseActionHandler creates a new case action handler
func NewCaseActionHandler(caseService contracts.CaseServiceInterface) *CaseActionHandler {
	return &CaseActionHandler{
		caseService: caseService,
		logger:      logger.New().WithField("handler", "case_action"),
	}
}

// ActionTypes returns the action types this handler supports
func (h *CaseActionHandler) ActionTypes() []string {
	return []string{
		"assign_case", "assign", "set_team",
		"change_status", "set_status", "status",
		"change_priority", "set_priority", "priority",
		"add_tags", "add_tag", "tag",
		"remove_tags", "remove_tag", "untag",
		"close_case", "close",
		"set_custom_field",
		"add_communication", "comment", "add_note",
	}
}

// Handle executes a case action
func (h *CaseActionHandler) Handle(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	switch action.Type {
	case "assign_case", "assign", "set_team":
		return h.handleAssign(ctx, action, ruleContext, result)
	case "change_status", "set_status", "status":
		return h.handleStatus(ctx, action, ruleContext, result)
	case "change_priority", "set_priority", "priority":
		return h.handlePriority(ctx, action, ruleContext, result)
	case "add_tags", "add_tag", "tag":
		return h.handleAddTags(ctx, action, ruleContext, result)
	case "remove_tags", "remove_tag", "untag":
		return h.handleRemoveTags(ctx, action, ruleContext, result)
	case "close_case", "close":
		return h.handleClose(ctx, action, ruleContext, result)
	case "set_custom_field":
		return h.handleSetCustomField(ctx, action, ruleContext, result)
	case "add_communication", "comment", "add_note":
		return h.handleAddCommunication(ctx, action, ruleContext, result)
	default:
		return fmt.Errorf("unsupported action type: %s", action.Type)
	}
}

func (h *CaseActionHandler) handleAssign(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	caseObj, err := requireCaseContext(action.Type, ruleContext)
	if err != nil {
		return err
	}
	if h.caseService == nil {
		return fmt.Errorf("%s action requires case service", action.Type)
	}

	userID := action.Value.AsString()
	teamID := action.Options.GetString("team_id")
	if action.Type == "set_team" && teamID == "" {
		teamID = userID
		userID = ""
	}

	if userID == "" && teamID == "" {
		return fmt.Errorf("no user ID or team ID specified for assignment")
	}

	// Use service method - handles validation and publishes CaseAssigned event
	if err := h.caseService.AssignCase(ctx, caseObj.ID, userID, teamID); err != nil {
		return fmt.Errorf("failed to assign case: %w", err)
	}

	result.Changes.SetString("assigned_to_id", userID)
	if teamID != "" {
		result.Changes.SetString("team_id", teamID)
	}
	return nil
}

func (h *CaseActionHandler) handleStatus(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	caseObj, err := requireCaseContext(action.Type, ruleContext)
	if err != nil {
		return err
	}
	if h.caseService == nil {
		return fmt.Errorf("%s action requires case service", action.Type)
	}

	status := servicedomain.CaseStatus(action.Value.AsString())

	// Use service method - handles validation and publishes CaseStatusChanged event
	if err := h.caseService.SetCaseStatus(ctx, caseObj.ID, status); err != nil {
		return fmt.Errorf("failed to update case status: %w", err)
	}

	result.Changes.SetString("status", string(status))
	return nil
}

func (h *CaseActionHandler) handlePriority(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	caseObj, err := requireCaseContext(action.Type, ruleContext)
	if err != nil {
		return err
	}
	if h.caseService == nil {
		return fmt.Errorf("%s action requires case service", action.Type)
	}

	priority := servicedomain.CasePriority(action.Value.AsString())

	// Use service method - handles validation
	if err := h.caseService.SetCasePriority(ctx, caseObj.ID, priority); err != nil {
		return fmt.Errorf("failed to update case priority: %w", err)
	}

	result.Changes.SetString("priority", string(priority))
	return nil
}

func (h *CaseActionHandler) handleAddTags(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	caseObj, err := requireCaseContext(action.Type, ruleContext)
	if err != nil {
		return err
	}
	if h.caseService == nil {
		return fmt.Errorf("%s action requires case service", action.Type)
	}

	tagsToAdd := parseTagsFromValue(action.Value)
	var addedTags []string

	for _, tag := range tagsToAdd {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		// Use service method for each tag
		if err := h.caseService.AddCaseTag(ctx, caseObj.ID, tag); err != nil {
			h.logger.Warn("Failed to add tag", "tag", tag, "error", err)
			continue
		}
		addedTags = append(addedTags, tag)
	}

	result.Changes.SetStrings("tags_added", addedTags)
	return nil
}

func (h *CaseActionHandler) handleRemoveTags(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	caseObj, err := requireCaseContext(action.Type, ruleContext)
	if err != nil {
		return err
	}
	if h.caseService == nil {
		return fmt.Errorf("%s action requires case service", action.Type)
	}

	tagsToRemove := parseTagsFromValue(action.Value)
	var removedTags []string

	for _, tag := range tagsToRemove {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		// Use service method for each tag
		if err := h.caseService.RemoveCaseTag(ctx, caseObj.ID, tag); err != nil {
			h.logger.Warn("Failed to remove tag", "tag", tag, "error", err)
			continue
		}
		removedTags = append(removedTags, tag)
	}

	result.Changes.SetStrings("tags_removed", removedTags)
	return nil
}

func (h *CaseActionHandler) handleClose(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	caseObj, err := requireCaseContext(action.Type, ruleContext)
	if err != nil {
		return err
	}
	if h.caseService == nil {
		return fmt.Errorf("%s action requires case service", action.Type)
	}

	// Use service method - handles validation and state transitions
	if err := h.caseService.CloseCase(ctx, caseObj.ID); err != nil {
		return fmt.Errorf("failed to close case: %w", err)
	}

	result.Changes.SetString("status", "closed")
	return nil
}

func (h *CaseActionHandler) handleSetCustomField(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	caseObj, err := requireCaseContext(action.Type, ruleContext)
	if err != nil {
		return err
	}
	if h.caseService == nil {
		return fmt.Errorf("%s action requires case service", action.Type)
	}

	field := action.Target
	if field == "" {
		field = action.Field
	}
	if field == "" {
		return fmt.Errorf("no field specified for custom field action")
	}

	// Get fresh case from service
	caseObj, err = h.caseService.GetCase(ctx, caseObj.ID)
	if err != nil {
		return fmt.Errorf("failed to get case: %w", err)
	}

	// Modify custom field (domain logic)
	caseObj.CustomFields.SetAny(field, valueToInterface(action.Value))

	// Save via service
	if err := h.caseService.SaveCase(ctx, caseObj); err != nil {
		return fmt.Errorf("failed to set custom field: %w", err)
	}

	result.Changes.Set(fmt.Sprintf("custom_field_%s", field), action.Value)
	return nil
}

// valueToInterface converts a shareddomain.Value to its underlying Go value
func valueToInterface(v shareddomain.Value) interface{} {
	if v.IsZero() {
		return nil
	}
	switch v.Type() {
	case shareddomain.ValueTypeString:
		return v.AsString()
	case shareddomain.ValueTypeInt:
		return v.AsInt()
	case shareddomain.ValueTypeFloat:
		return v.AsFloat()
	case shareddomain.ValueTypeBool:
		return v.AsBool()
	case shareddomain.ValueTypeStrings:
		return v.AsStrings()
	case shareddomain.ValueTypeTime:
		return v.AsTime()
	case shareddomain.ValueTypeDuration:
		return v.AsDuration()
	default:
		return v.AsString()
	}
}

func (h *CaseActionHandler) handleAddCommunication(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	caseObj, err := requireCaseContext(action.Type, ruleContext)
	if err != nil {
		return err
	}
	if h.caseService == nil {
		return fmt.Errorf("%s action requires case service", action.Type)
	}

	message := action.Value.AsString()

	// Use service method to add internal note
	_, err = h.caseService.AddInternalNote(
		ctx,
		caseObj.ID,
		caseObj.WorkspaceID,
		"",           // no human user ID for automation-authored notes
		"Automation", // userName
		message,
	)
	if err != nil {
		return fmt.Errorf("failed to add communication: %w", err)
	}

	result.Changes.SetString("communication_added", message)
	return nil
}

// parseTagsFromValue converts shareddomain.Value to a slice of tags
func parseTagsFromValue(value shareddomain.Value) []string {
	// If it's already a string slice, return directly
	if value.Type() == shareddomain.ValueTypeStrings {
		return value.AsStrings()
	}
	// Otherwise, treat as single string (possibly comma-separated)
	tagsStr := value.AsString()
	if tagsStr == "" {
		return nil
	}
	return strings.Split(tagsStr, ",")
}
