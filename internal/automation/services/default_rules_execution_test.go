//go:build integration

package automationservices

import (
	"context"
	"testing"

	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestCaseWithStatus creates a test case for rule execution tests with specific status and priority
func createTestCaseWithStatus(t testing.TB, workspaceID string, status servicedomain.CaseStatus, priority servicedomain.CasePriority) *servicedomain.Case {
	c := servicedomain.NewCase(workspaceID, "Test Case for Rule Execution", testutil.UniqueEmail(t))
	c.ID = testutil.UniqueID("case")
	c.HumanID = "TEST-" + testutil.UniqueID("human")
	c.Description = "Testing rule execution"
	c.Status = status
	c.Priority = priority
	c.Channel = servicedomain.CaseChannelEmail
	c.ContactName = "Test User"
	c.Tags = []string{}
	return c
}

func TestDefaultRules_FirstResponsePending_ExecutesStatusChange(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-first-response")

	// Seed default rules
	seeder := NewDefaultRulesSeeder(store.Rules())
	_, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)

	// Create a case with status "new"
	caseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusNew, servicedomain.CasePriorityMedium)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	// Create rules engine
	engine := newRulesEngineForStore(store)

	// Simulate agent reply event - this should trigger "First Response - Set Pending" rule
	err = engine.EvaluateRulesForCase(ctx, caseObj, "agent_reply", nil)
	require.NoError(t, err)

	// Fetch the case from store to see if it was updated
	updatedCase, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)

	// The First Response rule should have changed status from "new" to "pending"
	// (agent replied, now waiting for customer response)
	assert.Equal(t, servicedomain.CaseStatusPending, updatedCase.Status,
		"Case status should be changed to 'pending' after first agent reply (awaiting customer response)")
}

func TestDefaultRules_CustomerReplyResolved_ExecutesStatusChange(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-customer-resolved")

	// Seed default rules
	seeder := NewDefaultRulesSeeder(store.Rules())
	_, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)

	// Create a case with status "resolved"
	caseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusResolved, servicedomain.CasePriorityMedium)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	// Create rules engine
	engine := newRulesEngineForStore(store)

	// Simulate customer reply event - this should trigger "Customer Reply - Set Open" rule
	err = engine.EvaluateRulesForCase(ctx, caseObj, "customer_reply", nil)
	require.NoError(t, err)

	// Fetch the case from store to see if it was updated
	updatedCase, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)

	// The Customer Reply rule should have changed status from "resolved" to "open"
	assert.Equal(t, servicedomain.CaseStatusOpen, updatedCase.Status,
		"Case status should be changed to 'open' after customer reply to resolved case")

	// Should NOT have "reopened" tag (only for closed cases)
	assert.NotContains(t, updatedCase.Tags, "reopened",
		"Case should NOT have 'reopened' tag for resolved->open transition")
}

func TestDefaultRules_CustomerReplyClosed_ExecutesReopenWithTag(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-customer-closed")

	// Seed default rules
	seeder := NewDefaultRulesSeeder(store.Rules())
	_, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)

	// Create a case with status "closed"
	caseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusClosed, servicedomain.CasePriorityMedium)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	// Create rules engine
	engine := newRulesEngineForStore(store)

	// Simulate customer reply event - this should trigger "Customer Reply - Reopen Closed Case" rule
	err = engine.EvaluateRulesForCase(ctx, caseObj, "customer_reply", nil)
	require.NoError(t, err)

	// Fetch the case from store to see if it was updated
	updatedCase, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)

	// The Customer Reply Reopen Closed rule should have changed status from "closed" to "open"
	assert.Equal(t, servicedomain.CaseStatusOpen, updatedCase.Status,
		"Case status should be changed to 'open' after customer reply to closed case")

	// Should have "reopened" tag (only for closed cases)
	assert.Contains(t, updatedCase.Tags, "reopened",
		"Case should have 'reopened' tag when reopening from closed status")
}

func TestDefaultRules_CustomerReplyPending_ExecutesStatusChange(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-customer-pending")

	// Seed default rules
	seeder := NewDefaultRulesSeeder(store.Rules())
	_, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)

	// Create a case with status "pending"
	caseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusPending, servicedomain.CasePriorityMedium)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	// Create rules engine
	engine := newRulesEngineForStore(store)

	// Simulate customer reply event
	err = engine.EvaluateRulesForCase(ctx, caseObj, "customer_reply", nil)
	require.NoError(t, err)

	// Fetch the case from store
	updatedCase, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)

	// The Customer Reply rule should have changed status from "pending" to "open"
	assert.Equal(t, servicedomain.CaseStatusOpen, updatedCase.Status,
		"Case status should be changed to 'open' after customer reply to pending case")
}

func TestDefaultRules_CaseCreated_DoesNotChangeStatusFromNew(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-case-created")

	// Seed default rules
	seeder := NewDefaultRulesSeeder(store.Rules())
	_, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)

	// Create a case with status "new"
	caseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusNew, servicedomain.CasePriorityMedium)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	// Create rules engine
	engine := newRulesEngineForStore(store)

	// Simulate case created event - this should NOT change status (only acknowledgment email)
	err = engine.EvaluateRulesForCase(ctx, caseObj, "case_created", nil)
	require.NoError(t, err)

	// Fetch the case from store
	updatedCase, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)

	// Status should still be "new" - only case_created acknowledgment rule fires
	assert.Equal(t, servicedomain.CaseStatusNew, updatedCase.Status,
		"Case status should remain 'new' after case_created event (per documentation)")
}

func TestDefaultRules_AllRulesAreActive(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-all-active")

	// Seed default rules
	seeder := NewDefaultRulesSeeder(store.Rules())
	_, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)

	// Get all active rules
	activeRules, err := store.Rules().ListActiveRules(ctx, workspaceID)
	require.NoError(t, err)

	// All 7 default rules should be active
	assert.Len(t, activeRules, 7, "All 7 default rules should be active")

	// Verify each rule is marked active
	for _, rule := range activeRules {
		assert.True(t, rule.IsActive, "Rule %s should be active", rule.Title)
		assert.True(t, rule.IsSystem, "Rule %s should be a system rule", rule.Title)
	}
}

func TestDefaultRules_RulesEngineEvaluatesAllRules(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-evaluate-all")

	// Seed default rules
	seeder := NewDefaultRulesSeeder(store.Rules())
	rules, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)
	require.Len(t, rules, 7)

	// Create a case
	caseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusNew, servicedomain.CasePriorityMedium)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	// Create rules engine
	engine := newRulesEngineForStore(store)

	// Evaluate rules - should not error even with all rules present
	err = engine.EvaluateRulesForCase(ctx, caseObj, "case_created", nil)
	assert.NoError(t, err, "Rules engine should evaluate all rules without error")
}

func TestDefaultRules_ConditionEvaluation(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-condition")

	// Seed default rules
	seeder := NewDefaultRulesSeeder(store.Rules())
	_, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)

	// Create condition evaluator
	evaluator := NewRuleConditionEvaluator()

	// Get the rules
	rules, err := store.Rules().ListWorkspaceRules(ctx, workspaceID)
	require.NoError(t, err)

	// Find the "First Response - Set Pending" rule
	var firstResponseRule *automationdomain.Rule
	for _, r := range rules {
		if r.SystemRuleKey == SystemRuleKeyFirstResponseOpen {
			firstResponseRule = r
			break
		}
	}
	require.NotNil(t, firstResponseRule, "Should find First Response rule")

	// Test condition evaluation with matching case (status=new, event=agent_reply)
	t.Run("matches when status is new and event is agent_reply", func(t *testing.T) {
		caseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusNew, servicedomain.CasePriorityMedium)
		ruleContext := &RuleContext{
			Case:  caseObj,
			Event: "agent_reply",
		}

		matches, err := evaluator.EvaluateConditions(firstResponseRule.Conditions, ruleContext)
		require.NoError(t, err)
		assert.True(t, matches, "Condition should match for new case with agent_reply event")
	})

	t.Run("does not match when status is open", func(t *testing.T) {
		caseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusOpen, servicedomain.CasePriorityMedium)
		ruleContext := &RuleContext{
			Case:  caseObj,
			Event: "agent_reply",
		}

		matches, err := evaluator.EvaluateConditions(firstResponseRule.Conditions, ruleContext)
		require.NoError(t, err)
		assert.False(t, matches, "Condition should NOT match when status is already open")
	})

	t.Run("does not match when event is not agent_reply", func(t *testing.T) {
		caseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusNew, servicedomain.CasePriorityMedium)
		ruleContext := &RuleContext{
			Case:  caseObj,
			Event: "case_created",
		}

		matches, err := evaluator.EvaluateConditions(firstResponseRule.Conditions, ruleContext)
		require.NoError(t, err)
		assert.False(t, matches, "Condition should NOT match when event is case_created")
	})
}

func TestDefaultRules_ActionExecution(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-action")

	// Create action executor
	executor := newRuleActionExecutorForTest(store)

	// Create a case
	caseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusNew, servicedomain.CasePriorityMedium)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	t.Run("executes change_status action", func(t *testing.T) {
		actions := automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "change_status", Value: shareddomain.StringValue(string(servicedomain.CaseStatusOpen))},
			},
		}

		ruleContext := &RuleContext{
			Case:   caseObj,
			RuleID: "test-rule-1",
		}

		result, err := executor.ExecuteActions(ctx, actions, ruleContext)
		require.NoError(t, err)
		assert.Contains(t, result.ExecutedActions, "change_status")
		status, _ := result.Changes.GetString("status")
		assert.Equal(t, string(servicedomain.CaseStatusOpen), status)
	})

	t.Run("executes add_tags action", func(t *testing.T) {
		// Reset case tags
		caseObj.Tags = []string{}
		require.NoError(t, store.Cases().UpdateCase(ctx, caseObj))

		actions := automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "add_tags", Value: shareddomain.StringValue("test-tag")},
			},
		}

		ruleContext := &RuleContext{
			Case:   caseObj,
			RuleID: "test-rule-2",
		}

		result, err := executor.ExecuteActions(ctx, actions, ruleContext)
		require.NoError(t, err)
		assert.Contains(t, result.ExecutedActions, "add_tags")

		// Verify tag was added to case
		updatedCase, err := store.Cases().GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Contains(t, updatedCase.Tags, "test-tag")
	})

	t.Run("executes change_priority action", func(t *testing.T) {
		actions := automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "change_priority", Value: shareddomain.StringValue(string(servicedomain.CasePriorityHigh))},
			},
		}

		ruleContext := &RuleContext{
			Case:   caseObj,
			RuleID: "test-rule-3",
		}

		result, err := executor.ExecuteActions(ctx, actions, ruleContext)
		require.NoError(t, err)
		assert.Contains(t, result.ExecutedActions, "change_priority")

		// Verify priority was changed
		updatedCase, err := store.Cases().GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CasePriorityHigh, updatedCase.Priority)
	})

	t.Run("executes close_case action", func(t *testing.T) {
		// Create a fresh case for this test
		closeCaseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusResolved, servicedomain.CasePriorityLow)
		require.NoError(t, store.Cases().CreateCase(ctx, closeCaseObj))

		actions := automationdomain.RuleActionsData{
			Actions: []automationdomain.RuleAction{
				{Type: "close_case", Value: shareddomain.BoolValue(true)},
			},
		}

		ruleContext := &RuleContext{
			Case:   closeCaseObj,
			RuleID: "test-rule-4",
		}

		result, err := executor.ExecuteActions(ctx, actions, ruleContext)
		require.NoError(t, err)
		assert.Contains(t, result.ExecutedActions, "close_case")

		// Verify case was closed
		updatedCase, err := store.Cases().GetCase(ctx, closeCaseObj.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.CaseStatusClosed, updatedCase.Status)
		assert.NotNil(t, updatedCase.ClosedAt)
	})
}

func TestDefaultRules_EndToEndExecution(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-end-to-end")

	// Step 1: Seed default rules (simulating workspace creation)
	seeder := NewDefaultRulesSeeder(store.Rules())
	rules, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)
	require.Len(t, rules, 7, "Should have 7 default rules")

	// Step 2: Create a new case (status: new)
	caseObj := createTestCaseWithStatus(t, workspaceID, servicedomain.CaseStatusNew, servicedomain.CasePriorityMedium)
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	// Create rules engine
	engine := newRulesEngineForStore(store)

	// Step 3: Simulate case_created event
	err = engine.EvaluateRulesForCase(ctx, caseObj, "case_created", nil)
	require.NoError(t, err)

	// Verify case is still "new" (case_created only sends acknowledgment)
	caseAfterCreate, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CaseStatusNew, caseAfterCreate.Status, "Status should still be 'new' after creation")

	// Step 4: Simulate agent_reply event (first response)
	err = engine.EvaluateRulesForCase(ctx, caseAfterCreate, "agent_reply", nil)
	require.NoError(t, err)

	// Verify case is now "pending" (First Response rule fired - awaiting customer response)
	caseAfterReply, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CaseStatusPending, caseAfterReply.Status, "Status should be 'pending' after first agent reply (awaiting customer response)")

	// Step 5: Simulate customer_reply event (customer responds to pending case)
	err = engine.EvaluateRulesForCase(ctx, caseAfterReply, "customer_reply", nil)
	require.NoError(t, err)

	// Verify case is now "open" (Customer Reply rule fired - case is being actively worked)
	caseAfterCustomerReply, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CaseStatusOpen, caseAfterCustomerReply.Status, "Status should be 'open' after customer reply to pending")
	// Note: NO "reopened" tag for pending->open (only for closed->open)

	// Step 6: Manually set case to "closed" (simulating case being fully closed)
	caseAfterCustomerReply.Status = servicedomain.CaseStatusClosed
	caseAfterCustomerReply.Tags = []string{} // Clear tags for clean test
	require.NoError(t, store.Cases().UpdateCase(ctx, caseAfterCustomerReply))

	// Step 7: Simulate customer_reply event (customer responds to closed case)
	err = engine.EvaluateRulesForCase(ctx, caseAfterCustomerReply, "customer_reply", nil)
	require.NoError(t, err)

	// Verify case is reopened and tagged (closed->open should add "reopened" tag)
	caseFinal, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CaseStatusOpen, caseFinal.Status, "Status should be 'open' after customer reply to closed case")
	assert.Contains(t, caseFinal.Tags, "reopened", "Should have 'reopened' tag when reopening from closed")

	t.Log("End-to-end rule execution test passed!")
	t.Log("  - Case created: status remained 'new'")
	t.Log("  - Agent replied: status changed to 'pending' (awaiting customer)")
	t.Log("  - Customer replied to pending: status changed to 'open' (no reopened tag)")
	t.Log("  - Case closed, customer replied: status changed to 'open' with 'reopened' tag")
}
