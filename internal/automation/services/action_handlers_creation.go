package automationservices

import (
	"context"
	"fmt"

	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	"github.com/movebigrocks/platform/pkg/logger"
)

// CaseCreationActionHandler handles case creation actions (from issues, forms, etc.)
// It uses CaseServiceInterface instead of direct store access to ensure:
// - Domain validation runs
// - HumanID is properly generated
// - CaseCreated events are published
type CaseCreationActionHandler struct {
	caseService contracts.CaseServiceInterface
	logger      *logger.Logger
}

// NewCaseCreationActionHandler creates a new case creation handler
func NewCaseCreationActionHandler(caseService contracts.CaseServiceInterface) *CaseCreationActionHandler {
	return &CaseCreationActionHandler{
		caseService: caseService,
		logger:      logger.New().WithField("handler", "case_creation_action"),
	}
}

// ActionTypes returns the action types this handler supports
func (h *CaseCreationActionHandler) ActionTypes() []string {
	return []string{"create_case", "create_case_from_form"}
}

// Handle executes the case creation action
func (h *CaseCreationActionHandler) Handle(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	switch action.Type {
	case "create_case":
		return h.handleCreateCase(ctx, action, ruleContext, result)
	case "create_case_from_form":
		return h.handleCreateCaseFromForm(ctx, action, ruleContext, result)
	default:
		return fmt.Errorf("unsupported action type: %s", action.Type)
	}
}

func (h *CaseCreationActionHandler) handleCreateCase(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	issue, err := requireIssueContext(action.Type, ruleContext)
	if err != nil {
		return err
	}
	if h.caseService == nil {
		return fmt.Errorf("%s action requires case service", action.Type)
	}

	// Check if a case already exists for this issue
	if issue.HasRelatedCase && len(issue.RelatedCaseIDs) > 0 {
		h.logger.Debug("Case already exists for issue, skipping creation", "issue_id", issue.ID)
		result.Changes.SetString("skipped", "case_already_exists")
		result.Changes.SetStrings("existing_case_ids", issue.RelatedCaseIDs)
		return nil
	}

	caseObj, err := automationdomain.BuildCaseFromIssue(issue, action.Options.GetString("priority"))
	if err != nil {
		return fmt.Errorf("failed to build case from issue: %w", err)
	}

	// Save the case via service - handles HumanID generation, validation, and events
	if err := h.caseService.SaveCase(ctx, caseObj); err != nil {
		return fmt.Errorf("failed to create case: %w", err)
	}

	h.logger.Info("Created case for issue", "case_id", caseObj.ID, "issue_id", issue.ID)
	result.Changes.SetBool("case_created", true)
	result.Changes.SetString("case_id", caseObj.ID)
	result.Changes.SetString("case_human_id", caseObj.HumanID)
	return nil
}

func (h *CaseCreationActionHandler) handleCreateCaseFromForm(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	formEvent, err := requireFormContext(action.Type, ruleContext)
	if err != nil {
		return err
	}
	if h.caseService == nil {
		return fmt.Errorf("%s action requires case service", action.Type)
	}

	priority := servicedomain.CasePriorityMedium
	var tags []string

	if p := action.Options.GetString("priority"); p != "" {
		priority = servicedomain.CasePriority(p)
	}
	if tagsVal, ok := action.Options.Get("tags"); ok {
		tags = tagsVal.AsStrings()
	}

	caseObj, err := automationdomain.BuildCaseFromFormSubmission(formEvent, automationdomain.FormCaseOptions{
		Priority:   priority,
		AssignToID: action.Options.GetString("assign_to"),
		TeamID:     action.Options.GetString("team_id"),
		CaseType:   action.Options.GetString("case_type"),
		Tags:       tags,
	})
	if err != nil {
		return fmt.Errorf("failed to build case from form submission: %w", err)
	}

	// Save the case via service - handles HumanID generation, validation, and events
	if err := h.caseService.SaveCase(ctx, caseObj); err != nil {
		return fmt.Errorf("failed to create case from form: %w", err)
	}

	h.logger.Info("Created case from form submission",
		"case_id", caseObj.ID,
		"submission_id", formEvent.SubmissionID)

	result.Changes.SetBool("case_created", true)
	result.Changes.SetString("case_id", caseObj.ID)
	result.Changes.SetString("case_human_id", caseObj.HumanID)
	result.Changes.SetString("form_id", formEvent.FormID)
	return nil
}
