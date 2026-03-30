package automationservices

import (
	"fmt"

	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
)

func requireCaseContext(actionType string, ruleContext *RuleContext) (*servicedomain.Case, error) {
	if ruleContext == nil || ruleContext.Case == nil {
		return nil, fmt.Errorf("%s action requires case context", actionType)
	}
	return ruleContext.Case, nil
}

func requireFormContext(actionType string, ruleContext *RuleContext) (*contracts.FormSubmittedEvent, error) {
	if ruleContext == nil || ruleContext.FormSubmission == nil {
		return nil, fmt.Errorf("%s action requires form submission context", actionType)
	}
	return ruleContext.FormSubmission, nil
}

func requireIssueContext(actionType string, ruleContext *RuleContext) (*IssueContextData, error) {
	if ruleContext == nil || ruleContext.Issue == nil {
		return nil, fmt.Errorf("%s action requires issue context", actionType)
	}
	return ruleContext.Issue, nil
}

func workspaceIDFromRuleContext(ruleContext *RuleContext) (string, error) {
	if ruleContext == nil {
		return "", fmt.Errorf("rule context is nil")
	}
	switch {
	case ruleContext.Case != nil:
		return ruleContext.Case.WorkspaceID, nil
	case ruleContext.Issue != nil:
		return ruleContext.Issue.WorkspaceID, nil
	case ruleContext.FormSubmission != nil:
		return ruleContext.FormSubmission.WorkspaceID, nil
	default:
		return "", fmt.Errorf("rule context has no target workspace")
	}
}

func defaultNotificationSubject(ruleContext *RuleContext) string {
	if ruleContext == nil {
		return "Automation notification"
	}
	switch {
	case ruleContext.Case != nil:
		return fmt.Sprintf("Update on case: %s", ruleContext.Case.Subject)
	case ruleContext.FormSubmission != nil:
		return fmt.Sprintf("Update on form submission: %s", ruleContext.FormSubmission.FormSlug)
	case ruleContext.Issue != nil:
		return fmt.Sprintf("Update on issue: %s", ruleContext.Issue.Title)
	default:
		return "Automation notification"
	}
}

func safeRuleID(ruleContext *RuleContext) string {
	if ruleContext == nil {
		return ""
	}
	return ruleContext.RuleID
}

func safeTargetType(ruleContext *RuleContext) string {
	if ruleContext == nil {
		return "unknown"
	}
	return ruleContext.TargetType()
}

func safeTargetID(ruleContext *RuleContext) string {
	if ruleContext == nil {
		return ""
	}
	return ruleContext.TargetID()
}
