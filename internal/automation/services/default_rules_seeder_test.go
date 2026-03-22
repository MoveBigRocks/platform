package automationservices

import (
	"testing"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"

	"github.com/stretchr/testify/assert"
)

func TestGetDefaultRules(t *testing.T) {
	seeder := &DefaultRulesSeeder{}
	workspaceID := "test-workspace-123"

	rules := seeder.getDefaultRules(workspaceID)

	// Should return 7 default rules
	assert.Len(t, rules, 7)

	// Verify all rules have required fields
	for _, rule := range rules {
		assert.Empty(t, rule.ID, "Rule row IDs are assigned by PostgreSQL on insert")
		assert.Equal(t, workspaceID, rule.WorkspaceID, "Rule should have correct workspace ID")
		assert.NotEmpty(t, rule.Title, "Rule should have title")
		assert.True(t, rule.IsActive, "Rule should be active by default")
		assert.True(t, rule.IsSystem, "Rule should be marked as system rule")
		assert.NotEmpty(t, rule.SystemRuleKey, "Rule should have system key")
		assert.Equal(t, "system", rule.CreatedByID, "Rule should be created by system")
	}

	// Verify specific rules exist by key
	ruleKeys := make(map[string]bool)
	for _, rule := range rules {
		ruleKeys[rule.SystemRuleKey] = true
	}

	assert.True(t, ruleKeys[SystemRuleKeyCaseCreatedReceipt], "Should have case created receipt rule")
	assert.True(t, ruleKeys[SystemRuleKeyFirstResponseOpen], "Should have first response open rule")
	assert.True(t, ruleKeys[SystemRuleKeyCustomerReplyReopen], "Should have customer reply reopen rule")
	assert.True(t, ruleKeys[SystemRuleKeyPendingReminder], "Should have pending reminder rule")
	assert.True(t, ruleKeys[SystemRuleKeyAutoCloseResolved], "Should have auto close resolved rule")
	assert.True(t, ruleKeys[SystemRuleKeyNoResponseAlert], "Should have no response alert rule")
}

func TestGetDefaultRules_ValidConditionsAndActions(t *testing.T) {
	seeder := &DefaultRulesSeeder{}
	rules := seeder.getDefaultRules("test-workspace")

	for _, rule := range rules {
		t.Run(rule.SystemRuleKey, func(t *testing.T) {
			// Verify conditions structure - now typed
			assert.NotEmpty(t, rule.Conditions.Conditions, "Rule %s should have at least one condition", rule.SystemRuleKey)

			// Verify actions structure - now typed
			assert.NotEmpty(t, rule.Actions.Actions, "Rule %s should have at least one action", rule.SystemRuleKey)

			// Verify each action has a type
			for _, action := range rule.Actions.Actions {
				assert.NotEmpty(t, action.Type, "Action type should not be empty in rule %s", rule.SystemRuleKey)
			}
		})
	}
}

func TestGetDefaultRules_StatusValues(t *testing.T) {
	seeder := &DefaultRulesSeeder{}
	rules := seeder.getDefaultRules("test-workspace")

	// Find rules that reference status values
	validStatuses := map[string]bool{
		string(servicedomain.CaseStatusNew):      true,
		string(servicedomain.CaseStatusOpen):     true,
		string(servicedomain.CaseStatusPending):  true,
		string(servicedomain.CaseStatusResolved): true,
		string(servicedomain.CaseStatusClosed):   true,
		string(servicedomain.CaseStatusSpam):     true,
	}

	for _, rule := range rules {
		// Check conditions for status values
		for _, cond := range rule.Conditions.Conditions {
			if cond.Field == "status" {
				// Value is now a shareddomain.Value type
				if cond.Value.Type() == shareddomain.ValueTypeString {
					value := cond.Value.AsString()
					assert.True(t, validStatuses[value], "Invalid status value %s in rule %s", value, rule.SystemRuleKey)
				}
				if cond.Value.Type() == shareddomain.ValueTypeStrings {
					values := cond.Value.AsStrings()
					for _, v := range values {
						assert.True(t, validStatuses[v], "Invalid status value %s in rule %s", v, rule.SystemRuleKey)
					}
				}
			}
		}

		// Check actions for status values
		for _, action := range rule.Actions.Actions {
			if action.Type == "change_status" {
				if action.Value.Type() == shareddomain.ValueTypeString {
					value := action.Value.AsString()
					assert.True(t, validStatuses[value], "Invalid status value %s in action of rule %s", value, rule.SystemRuleKey)
				}
			}
		}
	}
}
