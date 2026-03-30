//go:build integration

package automationhandlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
	"github.com/movebigrocks/platform/pkg/logger"
)

func TestRuleEvaluationHandler_HandleFormSubmitted(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	rule := automationdomain.NewRule(workspace.ID, "Create case from submitted form", "admin")
	rule.IsActive = true
	rule.Conditions = automationdomain.RuleConditionsData{
		Operator: "and",
		Conditions: []automationdomain.RuleCondition{
			{Type: "event", Field: "trigger", Operator: "equals", Value: shareddomain.StringValue("form_submitted")},
			{Type: "field", Field: "form.form_slug", Operator: "equals", Value: shareddomain.StringValue("contact-support")},
		},
	}
	rule.Actions = automationdomain.RuleActionsData{
		Actions: []automationdomain.RuleAction{
			{
				Type: "create_case_from_form",
				Options: shareddomain.MetadataFromMap(map[string]interface{}{
					"priority":  "high",
					"case_type": "support",
					"tags":      []interface{}{"automation"},
				}),
			},
		},
	}
	require.NoError(t, store.Rules().CreateRule(ctx, rule))

	caseService := serviceapp.NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
	contactService := platformservices.NewContactService(store.Contacts())
	rulesEngine := automationservices.NewRulesEngine(
		automationservices.NewRuleService(store.Rules()),
		caseService,
		contactService,
		store.Rules(),
		nil,
	)
	t.Cleanup(rulesEngine.Stop)

	handler := NewRuleEvaluationHandler(rulesEngine, caseService, logger.NewNop())

	event := contracts.NewFormSubmittedEvent(
		"form_123",
		"contact-support",
		"submission_123",
		workspace.ID,
		"jane@example.com",
		"Jane Doe",
		map[string]interface{}{
			"subject": "Need help",
			"message": "Account access is broken",
		},
	)
	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	require.NoError(t, handler.HandleFormSubmitted(ctx, eventData))

	cases, total, err := store.Cases().ListCases(ctx, contracts.CaseFilters{
		WorkspaceID: workspace.ID,
		Limit:       10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, cases, 1)
	assert.Equal(t, "Need help", cases[0].Subject)
	assert.Equal(t, "support", cases[0].Category)
	assert.Contains(t, cases[0].Tags, "form-submission")
	assert.Contains(t, cases[0].Tags, "contact-support")
	assert.Contains(t, cases[0].Tags, "automation")
}
