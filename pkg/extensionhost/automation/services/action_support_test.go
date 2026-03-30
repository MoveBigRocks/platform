package automationservices

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

type testActionExtensionChecker struct {
	enabled bool
	err     error
}

func (t testActionExtensionChecker) HasActiveExtensionInWorkspace(_ context.Context, _, _ string) (bool, error) {
	return t.enabled, t.err
}

func TestValidateRuleActionsRejectsFormTriggeredCaseAction(t *testing.T) {
	conditions := automationdomain.TypedConditions{
		Operator: shareddomain.LogicalAnd,
		Conditions: []automationdomain.TypedCondition{
			{
				Type:     "event",
				Field:    "trigger",
				Operator: "equals",
				Value:    shareddomain.StringValue("form_submitted"),
			},
		},
	}
	actions := automationdomain.TypedActions{
		Actions: []automationdomain.TypedAction{
			{Type: "add_tag", Value: shareddomain.StringValue("review")},
		},
	}

	err := ValidateRuleActions(conditions, actions)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires case context")
}

func TestRuleActionExecutorSupportsCanonicalCaseActionAliases(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	caseObj := testutil.NewIsolatedCase(t, workspace.ID)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	caseService := serviceapp.NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	executor := NewRuleActionExecutor(caseService, nil)

	actions := automationdomain.TypedActions{
		Actions: []automationdomain.TypedAction{
			{Type: "set_status", Value: shareddomain.StringValue(string(servicedomain.CaseStatusPending))},
			{Type: "add_tag", Value: shareddomain.StringValue("ats-review")},
		},
	}

	result, err := executor.ExecuteActions(ctx, actions, &RuleContext{
		Case:     caseObj,
		Event:    "case_created",
		Metadata: NewRuleMetadata(),
	})
	require.NoError(t, err)
	assert.Contains(t, result.ExecutedActions, "set_status")
	assert.Contains(t, result.ExecutedActions, "add_tag")

	updatedCase, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CaseStatusPending, updatedCase.Status)
	assert.Contains(t, updatedCase.Tags, "ats-review")
}

func TestRuleActionExecutorReturnsErrorForCaseActionWithoutCaseContext(t *testing.T) {
	executor := NewRuleActionExecutor(nil, nil)
	actions := automationdomain.TypedActions{
		Actions: []automationdomain.TypedAction{
			{Type: "add_tag", Value: shareddomain.StringValue("ats-review")},
		},
	}

	result, err := executor.ExecuteActions(context.Background(), actions, &RuleContext{
		FormSubmission: &contracts.FormSubmittedEvent{
			FormID:       "form_123",
			FormSlug:     "job-application",
			SubmissionID: "submission_123",
			WorkspaceID:  "ws_123",
			Data:         map[string]interface{}{"full_name": "Candidate"},
		},
		Event:    "form_submitted",
		Metadata: NewRuleMetadata(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execution failed")
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "requires case context")
	assert.Empty(t, result.ExecutedActions)
}

func TestRuleActionExecutorRejectsIssueActionWhenErrorTrackingInactive(t *testing.T) {
	executor := NewRuleActionExecutor(nil, nil, WithRuleActionExecutorExtensionChecker(testActionExtensionChecker{enabled: false}))
	actions := automationdomain.TypedActions{
		Actions: []automationdomain.TypedAction{
			{Type: "create_case", Value: shareddomain.StringValue("auto")},
		},
	}

	result, err := executor.ExecuteActions(context.Background(), actions, &RuleContext{
		Issue: &IssueContextData{
			ID:          "issue_123",
			WorkspaceID: "ws_123",
			ProjectID:   "proj_123",
			Title:       "Production issue",
			Level:       "error",
			FirstSeen:   time.Now(),
			LastSeen:    time.Now(),
		},
		Event:    "issue_created",
		Metadata: NewRuleMetadata(),
	})
	require.Error(t, err)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "requires error-tracking to be active")
	assert.Empty(t, result.ExecutedActions)
}
