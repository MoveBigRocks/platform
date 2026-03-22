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

func TestSeedDefaultRules_CreatesAllRules(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-creates-all")

	seeder := NewDefaultRulesSeeder(store.Rules())

	// Seed the rules
	createdRules, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)

	// Should create 7 rules
	assert.Len(t, createdRules, 7, "Should create 7 default rules")

	// Verify rules are persisted in store
	storedRules, err := store.Rules().ListWorkspaceRules(ctx, workspaceID)
	require.NoError(t, err)
	assert.Len(t, storedRules, 7, "Should have 7 rules in store")

	// Verify each rule has required properties
	systemKeysSeen := make(map[string]bool)
	for _, rule := range storedRules {
		assert.Equal(t, workspaceID, rule.WorkspaceID, "Rule should belong to workspace")
		assert.True(t, rule.IsActive, "Rule should be active")
		assert.True(t, rule.IsSystem, "Rule should be marked as system")
		assert.NotEmpty(t, rule.SystemRuleKey, "Rule should have system key")
		assert.NotEmpty(t, rule.Title, "Rule should have title")
		assert.Equal(t, "system", rule.CreatedByID, "Rule should be created by system")

		systemKeysSeen[rule.SystemRuleKey] = true
	}

	// Verify all expected system keys are present
	expectedKeys := []string{
		SystemRuleKeyCaseCreatedReceipt,
		SystemRuleKeyFirstResponseOpen,
		SystemRuleKeyCustomerReplyReopen,
		SystemRuleKeyCustomerReplyReopenClosed,
		SystemRuleKeyPendingReminder,
		SystemRuleKeyAutoCloseResolved,
		SystemRuleKeyNoResponseAlert,
	}
	for _, key := range expectedKeys {
		assert.True(t, systemKeysSeen[key], "Missing system rule: %s", key)
	}
}

func TestSeedDefaultRules_Idempotent(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-idempotent")

	seeder := NewDefaultRulesSeeder(store.Rules())

	// Seed rules first time
	firstRun, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)
	assert.Len(t, firstRun, 7, "First run should create 7 rules")

	// Seed rules second time - should NOT create duplicates
	secondRun, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)
	assert.Len(t, secondRun, 0, "Second run should create 0 rules (already exist)")

	// Verify only 7 rules in store (not 12)
	storedRules, err := store.Rules().ListWorkspaceRules(ctx, workspaceID)
	require.NoError(t, err)
	assert.Len(t, storedRules, 7, "Should still have only 7 rules after second seed")
}

func TestSeedDefaultRules_MultipleWorkspaces(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace1 := testutil.CreateTestWorkspace(t, store, "default-rules-multi-1")
	workspace2 := testutil.CreateTestWorkspace(t, store, "default-rules-multi-2")

	seeder := NewDefaultRulesSeeder(store.Rules())

	// Seed rules for workspace 1
	rules1, err := seeder.SeedDefaultRules(ctx, workspace1)
	require.NoError(t, err)
	assert.Len(t, rules1, 7)

	// Seed rules for workspace 2
	rules2, err := seeder.SeedDefaultRules(ctx, workspace2)
	require.NoError(t, err)
	assert.Len(t, rules2, 7)

	// Verify each workspace has its own rules
	stored1, err := store.Rules().ListWorkspaceRules(ctx, workspace1)
	require.NoError(t, err)
	assert.Len(t, stored1, 7, "Workspace 1 should have 7 rules")

	stored2, err := store.Rules().ListWorkspaceRules(ctx, workspace2)
	require.NoError(t, err)
	assert.Len(t, stored2, 7, "Workspace 2 should have 7 rules")

	// Verify rules are different (different IDs)
	ids1 := make(map[string]bool)
	for _, r := range stored1 {
		ids1[r.ID] = true
	}

	for _, r := range stored2 {
		assert.False(t, ids1[r.ID], "Workspace 2 rules should have different IDs")
	}
}

// testValueToInterface converts shareddomain.Value to interface{} for test assertions
func testValueToInterface(v shareddomain.Value) interface{} {
	if v.IsZero() {
		return nil
	}
	switch v.Type() {
	case shareddomain.ValueTypeString:
		return v.AsString()
	case shareddomain.ValueTypeInt:
		return v.AsInt()
	case shareddomain.ValueTypeBool:
		return v.AsBool()
	case shareddomain.ValueTypeStrings:
		return v.AsStrings()
	default:
		return v.AsString()
	}
}

// metadataToMap converts shareddomain.Metadata to map[string]interface{} for test assertions
func metadataToMap(m shareddomain.Metadata) map[string]interface{} {
	keys := m.Keys()
	if len(keys) == 0 {
		return nil
	}
	result := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		if val, ok := m.Get(key); ok {
			result[key] = testValueToInterface(val)
		}
	}
	return result
}

// Helper to safely get conditions from a rule for map-based test assertions.
func getConditionsFromRule(conditions automationdomain.RuleConditionsData) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(conditions.Conditions))
	for _, c := range conditions.Conditions {
		result = append(result, map[string]interface{}{
			"type":     c.Type,
			"field":    c.Field,
			"operator": c.Operator,
			"value":    testValueToInterface(c.Value),
		})
	}
	return result
}

// Helper to safely get actions from a rule for map-based test assertions.
func getActionsFromRule(actions automationdomain.RuleActionsData) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(actions.Actions))
	for _, a := range actions.Actions {
		actionMap := map[string]interface{}{
			"type":   a.Type,
			"target": a.Target,
			"value":  testValueToInterface(a.Value),
			"field":  a.Field,
		}
		if opts := metadataToMap(a.Options); opts != nil {
			actionMap["options"] = opts
		}
		result = append(result, actionMap)
	}
	return result
}

func TestSeedDefaultRules_RuleStructure(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-structure")

	seeder := NewDefaultRulesSeeder(store.Rules())

	_, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)

	rules, err := store.Rules().ListWorkspaceRules(ctx, workspaceID)
	require.NoError(t, err)

	// Build map by system key for easier testing
	rulesByKey := make(map[string]*automationdomain.Rule)
	for _, r := range rules {
		rulesByKey[r.SystemRuleKey] = r
	}

	t.Run("CaseCreatedReceipt", func(t *testing.T) {
		rule := rulesByKey[SystemRuleKeyCaseCreatedReceipt]
		require.NotNil(t, rule)
		assert.Equal(t, "New Case Acknowledgment", rule.Title)
		assert.Equal(t, 1, rule.Priority)

		// Check conditions include case_created trigger
		conditions := getConditionsFromRule(rule.Conditions)
		hasCaseCreatedTrigger := false
		for _, cond := range conditions {
			if cond["value"] == "case_created" {
				hasCaseCreatedTrigger = true
			}
		}
		assert.True(t, hasCaseCreatedTrigger, "Should trigger on case_created")

		// Check actions include send_email
		actions := getActionsFromRule(rule.Actions)
		hasSendEmail := false
		for _, action := range actions {
			if action["type"] == "send_email" {
				hasSendEmail = true
			}
		}
		assert.True(t, hasSendEmail, "Should have send_email action")
	})

	t.Run("FirstResponsePending", func(t *testing.T) {
		rule := rulesByKey[SystemRuleKeyFirstResponseOpen]
		require.NotNil(t, rule)
		assert.Equal(t, "First Response - Set Pending", rule.Title)

		// Check actions include change_status to pending
		actions := getActionsFromRule(rule.Actions)
		hasStatusChange := false
		for _, action := range actions {
			if action["type"] == "change_status" && action["value"] == string(servicedomain.CaseStatusPending) {
				hasStatusChange = true
			}
		}
		assert.True(t, hasStatusChange, "Should change status to pending")
	})

	t.Run("CustomerReplyReopen", func(t *testing.T) {
		rule := rulesByKey[SystemRuleKeyCustomerReplyReopen]
		require.NotNil(t, rule)
		assert.Equal(t, "Customer Reply - Set Open", rule.Title)

		// Check conditions include customer_reply trigger
		conditions := getConditionsFromRule(rule.Conditions)
		hasCustomerReplyTrigger := false
		for _, cond := range conditions {
			if cond["value"] == "customer_reply" {
				hasCustomerReplyTrigger = true
			}
		}
		assert.True(t, hasCustomerReplyTrigger, "Should trigger on customer_reply")
	})

	t.Run("PendingReminder", func(t *testing.T) {
		rule := rulesByKey[SystemRuleKeyPendingReminder]
		require.NotNil(t, rule)
		assert.Equal(t, "Pending Case Reminder", rule.Title)

		// Check has time condition for 3 days
		conditions := getConditionsFromRule(rule.Conditions)
		hasTimeCondition := false
		for _, cond := range conditions {
			if cond["type"] == "time" && cond["value"] == "3d" {
				hasTimeCondition = true
			}
		}
		assert.True(t, hasTimeCondition, "Should have 3 day time condition")
	})

	t.Run("AutoCloseResolved", func(t *testing.T) {
		rule := rulesByKey[SystemRuleKeyAutoCloseResolved]
		require.NotNil(t, rule)
		assert.Equal(t, "Auto-Close Resolved Cases", rule.Title)

		// Check has close_case action
		actions := getActionsFromRule(rule.Actions)
		hasCloseCase := false
		for _, action := range actions {
			if action["type"] == "close_case" {
				hasCloseCase = true
			}
		}
		assert.True(t, hasCloseCase, "Should have close_case action")
	})

	t.Run("NoResponseAlert", func(t *testing.T) {
		rule := rulesByKey[SystemRuleKeyNoResponseAlert]
		require.NotNil(t, rule)
		assert.Equal(t, "No Response Alert (24h)", rule.Title)

		// Check has 24h time condition
		conditions := getConditionsFromRule(rule.Conditions)
		has24hCondition := false
		for _, cond := range conditions {
			if cond["type"] == "time" && cond["value"] == "24h" {
				has24hCondition = true
			}
		}
		assert.True(t, has24hCondition, "Should have 24h time condition")

		// Check sends email to workspace_members
		actions := getActionsFromRule(rule.Actions)
		sendsToMembers := false
		for _, action := range actions {
			if action["type"] == "send_email" {
				if opts, ok := action["options"].(map[string]interface{}); ok {
					if opts["to"] == "{{workspace_members}}" {
						sendsToMembers = true
					}
				}
			}
		}
		assert.True(t, sendsToMembers, "Should send email to workspace_members")
	})
}

func TestSeedDefaultRules_RulesCanBeRetrievedById(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-retrieve")

	seeder := NewDefaultRulesSeeder(store.Rules())

	createdRules, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)

	// Verify each created rule can be retrieved by ID
	for _, createdRule := range createdRules {
		retrieved, err := store.Rules().GetRule(ctx, createdRule.ID)
		require.NoError(t, err, "Should retrieve rule %s", createdRule.ID)
		assert.Equal(t, createdRule.ID, retrieved.ID)
		assert.Equal(t, createdRule.Title, retrieved.Title)
		assert.Equal(t, createdRule.SystemRuleKey, retrieved.SystemRuleKey)
	}
}

func TestSeedDefaultRules_PartialSeed(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := testutil.CreateTestWorkspace(t, store, "default-rules-partial")

	seeder := NewDefaultRulesSeeder(store.Rules())

	// Manually create one system rule first
	existingRule := seeder.getDefaultRules(workspaceID)[0]
	err := store.Rules().CreateRule(ctx, existingRule)
	require.NoError(t, err)

	// Now seed - should only create the remaining 6
	createdRules, err := seeder.SeedDefaultRules(ctx, workspaceID)
	require.NoError(t, err)
	assert.Len(t, createdRules, 6, "Should only create 6 new rules (1 already exists)")

	// Verify total is 7
	allRules, err := store.Rules().ListWorkspaceRules(ctx, workspaceID)
	require.NoError(t, err)
	assert.Len(t, allRules, 7, "Total should be 7 rules")
}
