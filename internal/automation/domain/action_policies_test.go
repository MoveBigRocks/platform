package automationdomain

import (
	"sort"
	"testing"
	"time"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

func TestValidateRuleActions(t *testing.T) {
	err := ValidateRuleActions(
		TypedConditions{
			Conditions: []TypedCondition{{
				Type:     "event",
				Field:    "trigger",
				Operator: "equals",
				Value:    shareddomain.StringValue("form_submitted"),
			}},
		},
		TypedActions{
			Actions: []TypedAction{{Type: "set_status"}},
		},
	)
	if err == nil {
		t.Fatal("expected form rule to reject case-only action")
	}
}

func TestBuildCasePolicies(t *testing.T) {
	issue := &IssueContextData{
		ID:          "issue_1",
		WorkspaceID: "ws_1",
		ProjectID:   "project_1",
		Title:       "Crash loop",
		Level:       "error",
		Culprit:     "handler",
		Platform:    "go",
		EventCount:  10,
		UserCount:   3,
		FirstSeen:   time.Unix(1, 0).UTC(),
		LastSeen:    time.Unix(2, 0).UTC(),
	}
	caseObj, err := BuildCaseFromIssue(issue, "")
	if err != nil {
		t.Fatalf("BuildCaseFromIssue returned error: %v", err)
	}
	if caseObj.Priority != servicedomain.CasePriorityHigh {
		t.Fatalf("expected mapped priority high, got %s", caseObj.Priority)
	}
	if !caseObj.AutoCreated {
		t.Fatal("expected auto-created issue case metadata to be set")
	}

	formCase, err := BuildCaseFromFormSubmission(&contracts.FormSubmittedEvent{
		WorkspaceID:    "ws_1",
		FormID:         "form_1",
		FormSlug:       "contact",
		SubmissionID:   "sub_1",
		SubmitterEmail: "person@example.com",
		SubmitterName:  "Person",
		Data: map[string]interface{}{
			"title":   "Help needed",
			"message": "The app is down",
		},
	}, FormCaseOptions{
		Priority: servicedomain.CasePriorityUrgent,
		Tags:     []string{"vip"},
	})
	if err != nil {
		t.Fatalf("BuildCaseFromFormSubmission returned error: %v", err)
	}
	if formCase.Subject != "Help needed" {
		t.Fatalf("unexpected form case subject %q", formCase.Subject)
	}
	if formCase.Priority != servicedomain.CasePriorityUrgent {
		t.Fatalf("unexpected form case priority %s", formCase.Priority)
	}
	if len(formCase.Tags) != 3 {
		t.Fatalf("expected default and extra tags, got %v", formCase.Tags)
	}
}

func TestEscalationPolicies(t *testing.T) {
	if EscalatedCasePriority(servicedomain.CasePriorityLow) != servicedomain.CasePriorityMedium {
		t.Fatal("expected low to escalate to medium")
	}
	note := EscalationNote(servicedomain.CasePriorityLow, servicedomain.CasePriorityHigh, "user_1")
	if note == "" {
		t.Fatal("expected escalation note")
	}
}

func TestSupportedRuleActionTypesAndExtensions(t *testing.T) {
	actionTypes := SupportedRuleActionTypes()
	if len(actionTypes) == 0 {
		t.Fatal("expected supported action types")
	}
	sorted := append([]string(nil), actionTypes...)
	sort.Strings(sorted)
	for i := range actionTypes {
		if actionTypes[i] != sorted[i] {
			t.Fatalf("expected action types to be sorted, got %v", actionTypes)
		}
	}

	if got := RequiredExtensionForAction("create_case"); got != "error-tracking" {
		t.Fatalf("expected create_case extension requirement, got %q", got)
	}
	if got := RequiredExtensionForAction("send_email"); got != "" {
		t.Fatalf("expected send_email to require no extension, got %q", got)
	}
	if got := RequiredExtensionForAction("missing"); got != "" {
		t.Fatalf("expected unknown action to require no extension, got %q", got)
	}
}

func TestActionPolicyBranches(t *testing.T) {
	if got := CasePriorityFromIssueLevel("fatal", ""); got != servicedomain.CasePriorityUrgent {
		t.Fatalf("expected fatal issues to map to urgent, got %s", got)
	}
	if got := CasePriorityFromIssueLevel("error", ""); got != servicedomain.CasePriorityHigh {
		t.Fatalf("expected error issues to map to high, got %s", got)
	}
	if got := CasePriorityFromIssueLevel("warning", ""); got != servicedomain.CasePriorityMedium {
		t.Fatalf("expected warning issues to map to medium, got %s", got)
	}
	if got := CasePriorityFromIssueLevel("info", "custom"); got != servicedomain.CasePriority("custom") {
		t.Fatalf("expected override to win, got %s", got)
	}
	if !anyCaseTrigger([]string{"case_created"}) {
		t.Fatal("expected case_created to count as case trigger")
	}
	if !anyCaseTrigger([]string{"customer_reply"}) {
		t.Fatal("expected customer_reply to count as case trigger")
	}
	if anyCaseTrigger([]string{"form_submitted"}) {
		t.Fatal("expected form_submitted not to count as case trigger")
	}
	if got := EscalatedCasePriority(servicedomain.CasePriorityMedium); got != servicedomain.CasePriorityHigh {
		t.Fatalf("expected medium priority to escalate to high, got %s", got)
	}
	if got := EscalatedCasePriority(servicedomain.CasePriorityHigh); got != servicedomain.CasePriorityUrgent {
		t.Fatalf("expected high priority to escalate to urgent, got %s", got)
	}
	if got := EscalatedCasePriority(servicedomain.CasePriorityUrgent); got != servicedomain.CasePriorityUrgent {
		t.Fatalf("expected urgent priority to remain urgent, got %s", got)
	}
}

func TestValidateRuleActionsBranches(t *testing.T) {
	err := ValidateRuleActions(
		TypedConditions{},
		TypedActions{Actions: []TypedAction{{Type: "unsupported"}}},
	)
	if err == nil {
		t.Fatal("expected unsupported action to fail validation")
	}

	err = ValidateRuleActions(
		TypedConditions{
			Conditions: []TypedCondition{{
				Type:     "event",
				Field:    "trigger",
				Operator: "in",
				Value:    shareddomain.StringsValue([]string{"case_created", "customer_reply"}),
			}},
		},
		TypedActions{Actions: []TypedAction{{Type: "create_case_from_form"}}},
	)
	if err == nil || err.Error() != `action "create_case_from_form" requires form submission context but the rule is explicitly triggered by case events` {
		t.Fatalf("unexpected case trigger validation error: %v", err)
	}

	err = ValidateRuleActions(
		TypedConditions{
			Conditions: []TypedCondition{{
				Type:     "event",
				Field:    "trigger",
				Operator: "equals",
				Value:    shareddomain.StringValue("form_submitted"),
			}},
		},
		TypedActions{Actions: []TypedAction{{Type: "create_case"}}},
	)
	if err == nil || err.Error() != `action "create_case" requires issue context but the rule is explicitly triggered by form_submitted` {
		t.Fatalf("unexpected form trigger validation error: %v", err)
	}

	err = ValidateRuleActions(
		TypedConditions{},
		TypedActions{Actions: []TypedAction{{Type: "create_case_from_form"}}},
	)
	if err != nil {
		t.Fatalf("expected implicit trigger rule to remain valid, got %v", err)
	}
}
