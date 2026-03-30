package automationdomain

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
)

type FormCaseOptions struct {
	Priority   servicedomain.CasePriority
	AssignToID string
	TeamID     string
	CaseType   string
	Tags       []string
}

type ruleActionSpec struct {
	RequiresCase      bool
	RequiresForm      bool
	RequiresIssue     bool
	RequiresExtension string
}

var supportedRuleActionSpecs = map[string]ruleActionSpec{
	"assign_case":           {RequiresCase: true},
	"assign":                {RequiresCase: true},
	"set_team":              {RequiresCase: true},
	"change_status":         {RequiresCase: true},
	"set_status":            {RequiresCase: true},
	"status":                {RequiresCase: true},
	"change_priority":       {RequiresCase: true},
	"set_priority":          {RequiresCase: true},
	"priority":              {RequiresCase: true},
	"add_tags":              {RequiresCase: true},
	"add_tag":               {RequiresCase: true},
	"tag":                   {RequiresCase: true},
	"remove_tags":           {RequiresCase: true},
	"remove_tag":            {RequiresCase: true},
	"untag":                 {RequiresCase: true},
	"close_case":            {RequiresCase: true},
	"close":                 {RequiresCase: true},
	"set_custom_field":      {RequiresCase: true},
	"add_communication":     {RequiresCase: true},
	"comment":               {RequiresCase: true},
	"add_note":              {RequiresCase: true},
	"escalate_case":         {RequiresCase: true},
	"escalate":              {RequiresCase: true},
	"send_email":            {},
	"email":                 {},
	"publish_event":         {},
	"create_case":           {RequiresIssue: true, RequiresExtension: "error-tracking"},
	"create_case_from_form": {RequiresForm: true},
}

func SupportedRuleActionTypes() []string {
	types := make([]string, 0, len(supportedRuleActionSpecs))
	for actionType := range supportedRuleActionSpecs {
		types = append(types, actionType)
	}
	sort.Strings(types)
	return types
}

func RequiredExtensionForAction(actionType string) string {
	spec, ok := supportedRuleActionSpecs[actionType]
	if !ok {
		return ""
	}
	return spec.RequiresExtension
}

func ValidateRuleActions(conditions TypedConditions, actions TypedActions) error {
	triggers := explicitTriggerTypes(conditions)

	for _, action := range actions.Actions {
		spec, ok := supportedRuleActionSpecs[action.Type]
		if !ok {
			return fmt.Errorf("unsupported automation action %q", action.Type)
		}
		if len(triggers) == 0 {
			continue
		}

		if slices.Contains(triggers, "form_submitted") {
			switch {
			case spec.RequiresCase:
				return fmt.Errorf("action %q requires case context but the rule is explicitly triggered by form_submitted", action.Type)
			case spec.RequiresIssue:
				return fmt.Errorf("action %q requires issue context but the rule is explicitly triggered by form_submitted", action.Type)
			}
		}

		if anyCaseTrigger(triggers) && spec.RequiresForm {
			return fmt.Errorf("action %q requires form submission context but the rule is explicitly triggered by case events", action.Type)
		}
	}

	return nil
}

func CasePriorityFromIssueLevel(level string, override string) servicedomain.CasePriority {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return servicedomain.CasePriority(trimmed)
	}

	switch strings.ToLower(strings.TrimSpace(level)) {
	case "fatal":
		return servicedomain.CasePriorityUrgent
	case "error":
		return servicedomain.CasePriorityHigh
	case "warning":
		return servicedomain.CasePriorityMedium
	default:
		return servicedomain.CasePriorityLow
	}
}

func BuildCaseFromIssue(issue *IssueContextData, priorityOverride string) (*servicedomain.Case, error) {
	if issue == nil {
		return nil, fmt.Errorf("issue context is required")
	}

	caseObj := servicedomain.NewCase(
		issue.WorkspaceID,
		fmt.Sprintf("[%s] %s", strings.ToUpper(issue.Level), issue.Title),
		"",
	)
	caseObj.Description = IssueCaseDescription(issue)
	caseObj.Channel = servicedomain.CaseChannelInternal
	caseObj.Priority = CasePriorityFromIssueLevel(issue.Level, priorityOverride)
	caseObj.Tags = []string{"auto-created", "error-monitoring", issue.Level}
	caseObj.CustomFields.SetString("linked_issue_id", issue.ID)
	caseObj.CustomFields.SetString("project_id", issue.ProjectID)
	caseObj.CustomFields.SetString("issue_level", issue.Level)
	caseObj.CustomFields.SetString("source", "automation_rule")
	caseObj.CustomFields.SetBool("auto_created", true)
	caseObj.MarkAsAutoCreated("automation_rule", issue.ID)

	if err := caseObj.LinkIssue(issue.ID, issue.ProjectID); err != nil {
		return nil, err
	}

	return caseObj, nil
}

func IssueCaseDescription(issue *IssueContextData) string {
	return fmt.Sprintf("Auto-created case for error monitoring issue.\n\n"+
		"**Issue Details:**\n"+
		"- Issue ID: %s\n"+
		"- Level: %s\n"+
		"- Culprit: %s\n"+
		"- Platform: %s\n"+
		"- Event Count: %d\n"+
		"- User Count: %d\n"+
		"- First Seen: %s\n"+
		"- Last Seen: %s\n",
		issue.ID, issue.Level, issue.Culprit, issue.Platform,
		issue.EventCount, issue.UserCount,
		issue.FirstSeen.Format(time.RFC3339),
		issue.LastSeen.Format(time.RFC3339))
}

func BuildCaseFromFormSubmission(formEvent *contracts.FormSubmittedEvent, options FormCaseOptions) (*servicedomain.Case, error) {
	if formEvent == nil {
		return nil, fmt.Errorf("form submission is required")
	}

	priority := options.Priority
	if priority == "" {
		priority = servicedomain.CasePriorityMedium
	}

	caseObj := servicedomain.NewCase(formEvent.WorkspaceID, FormSubmissionSubject(formEvent), "")
	caseObj.Description = FormSubmissionDescription(formEvent)
	caseObj.Channel = servicedomain.CaseChannelWeb
	caseObj.Priority = priority
	caseObj.AssignedToID = options.AssignToID
	caseObj.TeamID = options.TeamID
	caseObj.Category = options.CaseType

	if formEvent.SubmitterEmail != "" {
		caseObj.ContactEmail = formEvent.SubmitterEmail
	}
	if formEvent.SubmitterName != "" {
		caseObj.ContactName = formEvent.SubmitterName
	}

	caseObj.Tags = append([]string{"form-submission", formEvent.FormSlug}, options.Tags...)

	caseObj.CustomFields.SetString("form_id", formEvent.FormID)
	caseObj.CustomFields.SetString("form_slug", formEvent.FormSlug)
	caseObj.CustomFields.SetString("submission_id", formEvent.SubmissionID)
	caseObj.CustomFields.SetString("source", "form_automation")
	caseObj.CustomFields.SetBool("auto_created", true)
	caseObj.MarkAsAutoCreated("form_automation", "")

	for key, value := range formEvent.Data {
		caseObj.CustomFields.SetAny("form_"+key, value)
	}

	return caseObj, nil
}

func FormSubmissionSubject(formEvent *contracts.FormSubmittedEvent) string {
	subject := fmt.Sprintf("Form submission: %s", formEvent.FormSlug)
	if subjectField, ok := formEvent.Data["subject"].(string); ok && subjectField != "" {
		return subjectField
	}
	if titleField, ok := formEvent.Data["title"].(string); ok && titleField != "" {
		return titleField
	}
	return subject
}

func FormSubmissionDescription(formEvent *contracts.FormSubmittedEvent) string {
	descParts := []string{
		fmt.Sprintf("**Form:** %s\n**Submission ID:** %s\n", formEvent.FormSlug, formEvent.SubmissionID),
	}

	switch {
	case stringField(formEvent.Data, "description") != "":
		descParts = append(descParts, stringField(formEvent.Data, "description"))
	case stringField(formEvent.Data, "message") != "":
		descParts = append(descParts, stringField(formEvent.Data, "message"))
	case stringField(formEvent.Data, "body") != "":
		descParts = append(descParts, stringField(formEvent.Data, "body"))
	}

	descParts = append(descParts, "\n**Form Data:**")
	for key, value := range formEvent.Data {
		if key == "subject" || key == "description" || key == "message" || key == "body" || key == "title" {
			continue
		}
		descParts = append(descParts, fmt.Sprintf("- **%s:** %v", key, value))
	}

	return strings.Join(descParts, "\n")
}

func EscalatedCasePriority(current servicedomain.CasePriority) servicedomain.CasePriority {
	switch current {
	case servicedomain.CasePriorityLow:
		return servicedomain.CasePriorityMedium
	case servicedomain.CasePriorityMedium:
		return servicedomain.CasePriorityHigh
	case servicedomain.CasePriorityHigh:
		return servicedomain.CasePriorityUrgent
	default:
		return current
	}
}

func EscalationNote(oldPriority, newPriority servicedomain.CasePriority, assignee string) string {
	note := fmt.Sprintf("Case escalated from %s to %s priority", oldPriority, newPriority)
	if strings.TrimSpace(assignee) != "" {
		note += fmt.Sprintf(" and reassigned to %s", assignee)
	}
	return note
}

func explicitTriggerTypes(conditions TypedConditions) []string {
	var triggers []string
	for _, condition := range conditions.Conditions {
		if condition.Type != "event" {
			continue
		}
		if strings.TrimSpace(condition.Field) != "trigger" {
			continue
		}

		switch condition.Operator {
		case "", "equals":
			trigger := strings.TrimSpace(condition.Value.AsString())
			if trigger != "" {
				triggers = append(triggers, trigger)
			}
		case "in":
			for _, trigger := range condition.Value.AsStrings() {
				if trimmed := strings.TrimSpace(trigger); trimmed != "" {
					triggers = append(triggers, trimmed)
				}
			}
		}
	}

	if len(triggers) == 0 {
		return nil
	}

	sort.Strings(triggers)
	return slices.Compact(triggers)
}

func anyCaseTrigger(triggers []string) bool {
	for _, trigger := range triggers {
		if strings.HasPrefix(trigger, "case_") {
			return true
		}
		switch trigger {
		case "customer_reply", "agent_reply":
			return true
		}
	}
	return false
}

func stringField(data map[string]interface{}, key string) string {
	value, ok := data[key].(string)
	if !ok {
		return ""
	}
	return value
}
