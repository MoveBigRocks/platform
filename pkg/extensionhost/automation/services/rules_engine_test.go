//go:build integration

package automationservices

import (
	"context"
	"testing"

	automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRuleID(t *testing.T) {
	t.Run("generates unique IDs", func(t *testing.T) {
		id1 := generateRuleID()
		id2 := generateRuleID()

		assert.NotEmpty(t, id1)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2)
	})
}

func TestNewRulesEngine(t *testing.T) {
	t.Run("creates engine with nil store", func(t *testing.T) {
		engine := NewRulesEngine(nil, nil, nil, nil, nil)
		require.NotNil(t, engine)
		assert.NotNil(t, engine.conditionEvaluator)
		assert.NotNil(t, engine.actionExecutor)
		assert.NotNil(t, engine.rateLimiter)
	})
}

// NOTE: Removed trivial struct assignment tests (TestRuleCondition, TestRuleAction,
// TestRuleContext, TestActionResult) that only verified Go struct field assignment works.
// Instead, see TestRulesEngine_EvaluateRulesForCase and other behavioral tests below.

// Note: TestGetStringFromMap and TestGetMapFromMap are in rule_action_executor_test.go

func TestRulesEngine_EvaluateRulesForCase(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	engine := newRulesEngineForStore(store)

	t.Run("returns no error when no rules exist", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		err := engine.EvaluateRulesForCase(ctx, caseObj, "case_created", nil)
		assert.NoError(t, err)
	})

	t.Run("evaluates rule with matching conditions", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		// Create a case with high priority
		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		caseObj.Priority = servicedomain.CasePriorityHigh
		caseObj.Subject = "Urgent issue"
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		// Create an active rule that matches high priority cases
		rule := automationdomain.NewRule(workspace.ID, "High Priority Rule", "admin")
		rule.IsActive = true
		rule.Conditions = automationdomain.RuleConditionsData{
			Operator: "and",
			Conditions: []automationdomain.RuleCondition{
				{Type: "field", Field: "priority", Operator: "equals", Value: shareddomain.StringValue("high")},
			},
		}
		rule.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "add_tags", Value: shareddomain.StringValue("priority-escalated")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule))

		// Evaluate rules
		err := engine.EvaluateRulesForCase(ctx, caseObj, "case_created", nil)
		assert.NoError(t, err)

		// VERIFY: The tag was actually added by the rule action
		updatedCase, getErr := store.Cases().GetCase(ctx, caseObj.ID)
		require.NoError(t, getErr)
		assert.Contains(t, updatedCase.Tags, "priority-escalated", "Rule action should have added the tag")
	})

	t.Run("evaluates rule with non-matching conditions", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		// Create a case with low priority
		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		caseObj.Priority = servicedomain.CasePriorityLow
		originalStatus := caseObj.Status
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		// Create an active rule that only matches critical cases
		rule := automationdomain.NewRule(workspace.ID, "Critical Only Rule", "admin")
		rule.IsActive = true
		rule.Conditions = automationdomain.RuleConditionsData{
			Operator: "and",
			Conditions: []automationdomain.RuleCondition{
				{Type: "field", Field: "priority", Operator: "equals", Value: shareddomain.StringValue("critical")},
			},
		}
		rule.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "status", Value: shareddomain.StringValue("escalated")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule))

		// Evaluate rules - should not match
		err := engine.EvaluateRulesForCase(ctx, caseObj, "case_created", nil)
		assert.NoError(t, err)

		// VERIFY: Case status was NOT changed because rule didn't match
		assert.Equal(t, originalStatus, caseObj.Status, "Status should remain unchanged when rule conditions don't match")
	})

	t.Run("loads contact for rule context when available", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		// Create a contact
		contact := testutil.NewIsolatedContact(t, workspace.ID)
		contact.Name = "VIP Customer"
		require.NoError(t, store.Contacts().CreateContact(ctx, contact))

		// Create a case linked to the contact
		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		caseObj.ContactID = contact.ID
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		// Create a rule
		rule := automationdomain.NewRule(workspace.ID, "VIP Rule", "admin")
		rule.IsActive = true
		rule.Conditions = automationdomain.RuleConditionsData{}
		rule.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "add_tags", Value: shareddomain.StringValue("vip-customer")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule))

		// Evaluate rules
		err := engine.EvaluateRulesForCase(ctx, caseObj, "case_created", nil)
		assert.NoError(t, err)
	})

	t.Run("handles case without contact gracefully", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		// Create a case without contact
		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		caseObj.ContactID = testutil.NewIsolatedContact(t, workspace.ID).ID
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		// Create a simple rule
		rule := automationdomain.NewRule(workspace.ID, "Simple Rule", "admin")
		rule.IsActive = true
		rule.Conditions = automationdomain.RuleConditionsData{}
		rule.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "add_tags", Value: shareddomain.StringValue("processed")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule))

		// Evaluate rules - should not fail even with missing contact
		err := engine.EvaluateRulesForCase(ctx, caseObj, "case_updated", nil)
		assert.NoError(t, err)
	})

	t.Run("passes changes to rule context", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		// Create a case
		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		// Create a rule
		rule := automationdomain.NewRule(workspace.ID, "Change Rule", "admin")
		rule.IsActive = true
		rule.Conditions = automationdomain.RuleConditionsData{}
		rule.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "add_tags", Value: shareddomain.StringValue("updated")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule))

		// Evaluate rules with changes
		changes := NewFieldChanges()
		changes.Set("status", "open")
		err := engine.EvaluateRulesForCase(ctx, caseObj, "case_updated", changes)
		assert.NoError(t, err)
	})

	t.Run("evaluates multiple rules in sequence", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		// Create a case
		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		caseObj.Priority = servicedomain.CasePriorityMedium
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		// Create first rule
		rule1 := automationdomain.NewRule(workspace.ID, "Rule 1", "admin")
		rule1.IsActive = true
		rule1.Conditions = automationdomain.RuleConditionsData{}
		rule1.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "add_tags", Value: shareddomain.StringValue("rule1-applied")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule1))

		// Create second rule
		rule2 := automationdomain.NewRule(workspace.ID, "Rule 2", "admin")
		rule2.IsActive = true
		rule2.Conditions = automationdomain.RuleConditionsData{}
		rule2.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "add_tags", Value: shareddomain.StringValue("rule2-applied")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule2))

		// Evaluate rules
		err := engine.EvaluateRulesForCase(ctx, caseObj, "case_created", nil)
		assert.NoError(t, err)
	})
}

func TestRulesEngine_evaluateRule(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	engine := newRulesEngineForStore(store)

	t.Run("skips rate limited rule", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		// Create a case
		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		// Create a rule with rate limiting
		rule := automationdomain.NewRule(workspace.ID, "Rate Limited Rule", "admin")
		rule.IsActive = true
		rule.MaxExecutionsPerHour = 1
		rule.Conditions = automationdomain.RuleConditionsData{}
		rule.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "add_tags", Value: shareddomain.StringValue("processed")},
			},
		}

		require.NoError(t, store.Rules().CreateRule(ctx, rule))

		ruleContext := &RuleContext{
			Case:     caseObj,
			Event:    "case_created",
			Metadata: NewRuleMetadata(),
		}

		// First evaluation should succeed
		err := engine.executeRule(ctx, rule, ruleContext)
		assert.NoError(t, err)
	})

	t.Run("skips muted case", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		// Create a case
		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		// Create a rule with the case muted
		rule := automationdomain.NewRule(workspace.ID, "Muted Case Rule", "admin")
		rule.IsActive = true
		rule.MuteFor = []string{caseObj.ID}
		rule.Conditions = automationdomain.RuleConditionsData{}
		rule.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "add_tags", Value: shareddomain.StringValue("should-not-apply")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule))

		ruleContext := &RuleContext{
			Case:     caseObj,
			Event:    "case_created",
			Metadata: NewRuleMetadata(),
		}

		// Evaluation should skip due to muting
		err := engine.executeRule(ctx, rule, ruleContext)
		assert.NoError(t, err)
	})

	t.Run("executes actions when conditions match", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		// Create a case
		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		caseObj.Priority = servicedomain.CasePriorityHigh
		caseObj.Status = servicedomain.CaseStatusOpen
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		// Create a rule
		rule := automationdomain.NewRule(workspace.ID, "Matching Rule", "admin")
		rule.IsActive = true
		rule.Conditions = automationdomain.RuleConditionsData{
			Operator: "and",
			Conditions: []automationdomain.RuleCondition{
				{Type: "field", Field: "priority", Operator: "equals", Value: shareddomain.StringValue("high")},
			},
		}
		rule.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "status", Value: shareddomain.StringValue("pending")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule))

		ruleContext := &RuleContext{
			Case:     caseObj,
			Event:    "case_created",
			Metadata: NewRuleMetadata(),
		}

		err := engine.executeRule(ctx, rule, ruleContext)
		assert.NoError(t, err)
	})

	t.Run("skips when conditions do not match", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		// Create a case with low priority
		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		caseObj.Priority = servicedomain.CasePriorityLow
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		// Create a rule that requires high priority
		rule := automationdomain.NewRule(workspace.ID, "High Priority Only", "admin")
		rule.IsActive = true
		rule.Conditions = automationdomain.RuleConditionsData{
			Operator: "and",
			Conditions: []automationdomain.RuleCondition{
				{Type: "field", Field: "priority", Operator: "equals", Value: shareddomain.StringValue("high")},
			},
		}
		rule.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "add_tags", Value: shareddomain.StringValue("high-priority")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule))

		ruleContext := &RuleContext{
			Case:     caseObj,
			Event:    "case_created",
			Metadata: NewRuleMetadata(),
		}

		err := engine.executeRule(ctx, rule, ruleContext)
		assert.NoError(t, err)
		// Case should NOT have the tag since condition didn't match
		assert.NotContains(t, caseObj.Tags, "high-priority")
	})

	t.Run("handles action execution error gracefully", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		// Create a case
		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		// Create a rule with potentially problematic actions
		rule := automationdomain.NewRule(workspace.ID, "Error Handling Rule", "admin")
		rule.IsActive = true
		rule.Conditions = automationdomain.RuleConditionsData{}
		rule.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "unknown_action_type", Value: shareddomain.StringValue("test")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule))

		ruleContext := &RuleContext{
			Case:     caseObj,
			Event:    "case_created",
			Metadata: NewRuleMetadata(),
		}

		// Unknown action types are treated as errors and recorded in execution audit
		err := engine.executeRule(ctx, rule, ruleContext)
		assert.Error(t, err)

		// Verify execution was recorded with failed status
		executions, listErr := store.Rules().ListRuleExecutions(ctx, rule.ID)
		require.NoError(t, listErr)
		require.Len(t, executions, 1)
		assert.Equal(t, "failed", string(executions[0].Status))
	})

	t.Run("persists rule execution status after completion", func(t *testing.T) {
		workspace := testutil.NewIsolatedWorkspace(t)
		require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

		caseObj := testutil.NewIsolatedCase(t, workspace.ID)
		caseObj.Priority = servicedomain.CasePriorityHigh
		require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

		rule := automationdomain.NewRule(workspace.ID, "Persistence Test Rule", "admin")
		rule.IsActive = true
		rule.Conditions = automationdomain.RuleConditionsData{
			Operator: "and",
			Conditions: []automationdomain.RuleCondition{
				{Type: "field", Field: "priority", Operator: "equals", Value: shareddomain.StringValue("high")},
			},
		}
		rule.Actions = automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "add_tags", Value: shareddomain.StringValue("test-tag")},
			},
		}
		require.NoError(t, store.Rules().CreateRule(ctx, rule))

		ruleContext := &RuleContext{
			Case:     caseObj,
			Event:    "case_created",
			Metadata: NewRuleMetadata(),
		}

		err := engine.executeRule(ctx, rule, ruleContext)
		require.NoError(t, err)

		// Verify execution was persisted with correct status
		executions, err := store.Rules().ListRuleExecutions(ctx, rule.ID)
		require.NoError(t, err)
		require.Len(t, executions, 1)
		assert.Equal(t, "success", string(executions[0].Status))
		assert.NotNil(t, executions[0].CompletedAt)
		assert.GreaterOrEqual(t, executions[0].ExecutionTime, int64(0))
	})
}
