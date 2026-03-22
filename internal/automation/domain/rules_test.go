package automationdomain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

// TestNewRule tests rule creation
func TestNewRule(t *testing.T) {
	workspaceID := "test-workspace"
	title := "Test Rule"
	createdByID := "test-user"

	rule := NewRule(workspaceID, title, createdByID)

	assert.Empty(t, rule.ID)
	assert.Equal(t, workspaceID, rule.WorkspaceID)
	assert.Equal(t, title, rule.Title)
	assert.Equal(t, createdByID, rule.CreatedByID)
	assert.True(t, rule.IsActive)
	assert.NotNil(t, rule.Conditions)
	assert.NotNil(t, rule.Actions)
	assert.NotNil(t, rule.MuteFor)
	assert.NotNil(t, rule.ExecutionHistory)
	assert.False(t, rule.CreatedAt.IsZero())
	assert.False(t, rule.UpdatedAt.IsZero())
}

// TestNewWorkflow tests workflow creation
func TestRuleEvaluate(t *testing.T) {
	t.Run("empty conditions always match", func(t *testing.T) {
		rule := NewRule("test-workspace", "Test Rule", "test-user")
		caseObj := servicedomain.NewCase("test-workspace", "Test Case", "contact@example.com")

		result := rule.Evaluate(caseObj, nil)
		assert.True(t, result) // Empty conditions = always match
	})

	t.Run("simple field equals match", func(t *testing.T) {
		rule := NewRule("test-workspace", "Test Rule", "test-user")
		rule.Conditions = RuleConditionsData{
			Operator: "and",
			Conditions: []RuleCondition{
				{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.StringValue("new")},
			},
		}
		caseObj := servicedomain.NewCase("test-workspace", "Test Case", "contact@example.com")
		caseObj.Status = servicedomain.CaseStatusNew

		result := rule.Evaluate(caseObj, nil)
		assert.True(t, result)
	})

	t.Run("simple field equals no match", func(t *testing.T) {
		rule := NewRule("test-workspace", "Test Rule", "test-user")
		rule.Conditions = RuleConditionsData{
			Operator: "and",
			Conditions: []RuleCondition{
				{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.StringValue("open")},
			},
		}
		caseObj := servicedomain.NewCase("test-workspace", "Test Case", "contact@example.com")
		caseObj.Status = servicedomain.CaseStatusNew

		result := rule.Evaluate(caseObj, nil)
		assert.False(t, result)
	})

	t.Run("complex condition with operator", func(t *testing.T) {
		rule := NewRule("test-workspace", "Test Rule", "test-user")
		rule.Conditions = RuleConditionsData{
			Operator: "and",
			Conditions: []RuleCondition{
				{Type: "field", Field: "subject", Operator: "contains", Value: shareddomain.StringValue("urgent")},
			},
		}
		caseObj := servicedomain.NewCase("test-workspace", "URGENT: Help needed", "contact@example.com")

		result := rule.Evaluate(caseObj, nil)
		assert.True(t, result)
	})

	t.Run("nil case returns false", func(t *testing.T) {
		rule := NewRule("test-workspace", "Test Rule", "test-user")

		result := rule.Evaluate(nil, nil)
		assert.False(t, result)
	})

	t.Run("scope restriction by priority", func(t *testing.T) {
		rule := NewRule("test-workspace", "Test Rule", "test-user")
		rule.Priorities = []shareddomain.CasePriority{shareddomain.CasePriority("high"), shareddomain.CasePriority("urgent")}
		rule.Conditions = RuleConditionsData{
			Operator: "and",
			Conditions: []RuleCondition{
				{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.StringValue("new")},
			},
		}

		highCase := servicedomain.NewCase("test-workspace", "Test", "contact@example.com")
		highCase.Priority = servicedomain.CasePriorityHigh
		highCase.Status = servicedomain.CaseStatusNew

		lowCase := servicedomain.NewCase("test-workspace", "Test", "contact@example.com")
		lowCase.Priority = servicedomain.CasePriorityLow
		lowCase.Status = servicedomain.CaseStatusNew

		assert.True(t, rule.Evaluate(highCase, nil))
		assert.False(t, rule.Evaluate(lowCase, nil)) // Low priority not in allowed list
	})
}

// TestRuleExecute tests rule execution
func TestRuleExecute(t *testing.T) {
	rule := NewRule("test-workspace", "Test Rule", "test-user")
	caseObj := servicedomain.NewCase("test-workspace", "Test Case", "contact@example.com")

	execution, err := rule.Execute(caseObj)

	require.NoError(t, err)
	assert.Empty(t, execution.ID)
	assert.Equal(t, rule.WorkspaceID, execution.WorkspaceID)
	assert.Equal(t, rule.ID, execution.RuleID)
	assert.Equal(t, caseObj.ID, execution.CaseID)
	assert.Equal(t, shareddomain.TriggerType("case_updated"), execution.TriggerType)
	assert.Equal(t, ExecStatusSuccess, execution.Status)
	assert.NotNil(t, execution.CompletedAt)
	assert.GreaterOrEqual(t, execution.ExecutionTime, int64(0))
}

// TestRuleExecuteActions tests rule action execution
func TestRuleExecuteActions(t *testing.T) {
	t.Run("set_status action", func(t *testing.T) {
		rule := NewRule("test-workspace", "Test Rule", "test-user")
		rule.Actions = RuleActionsData{
			Actions: []RuleAction{
				{Type: "set_status", Value: shareddomain.StringValue("pending")},
			},
		}
		caseObj := servicedomain.NewCase("test-workspace", "Test", "contact@example.com")

		execution, err := rule.Execute(caseObj)
		require.NoError(t, err)
		assert.Equal(t, ExecStatusSuccess, execution.Status)
		assert.Equal(t, servicedomain.CaseStatusPending, caseObj.Status)
	})

	t.Run("add_tag action", func(t *testing.T) {
		rule := NewRule("test-workspace", "Test Rule", "test-user")
		rule.Actions = RuleActionsData{
			Actions: []RuleAction{
				{Type: "add_tag", Value: shareddomain.StringValue("vip")},
			},
		}
		caseObj := servicedomain.NewCase("test-workspace", "Test", "contact@example.com")

		execution, err := rule.Execute(caseObj)
		require.NoError(t, err)
		assert.Equal(t, ExecStatusSuccess, execution.Status)
		assert.Contains(t, caseObj.Tags, "vip")
	})

	t.Run("multiple actions", func(t *testing.T) {
		rule := NewRule("test-workspace", "Test Rule", "test-user")
		rule.Actions = RuleActionsData{
			Actions: []RuleAction{
				{Type: "set_priority", Value: shareddomain.StringValue("urgent")},
				{Type: "set_category", Value: shareddomain.StringValue("billing")},
			},
		}
		caseObj := servicedomain.NewCase("test-workspace", "Test", "contact@example.com")

		execution, err := rule.Execute(caseObj)
		require.NoError(t, err)
		assert.Equal(t, ExecStatusSuccess, execution.Status)
		assert.Equal(t, servicedomain.CasePriorityUrgent, caseObj.Priority)
		assert.Equal(t, "billing", caseObj.Category)
	})
}

// TestRuleMuteUnmute tests mute functionality
func TestRuleMuteUnmute(t *testing.T) {
	rule := NewRule("test-workspace", "Test Rule", "test-user")

	// Initially not muted
	assert.False(t, rule.IsMuted("case-1"))

	// Mute a case
	rule.MuteFor = append(rule.MuteFor, "case-1")
	assert.True(t, rule.IsMuted("case-1"))
	assert.False(t, rule.IsMuted("case-2"))

	// Unmute the case
	rule.Unmute("case-1")
	assert.False(t, rule.IsMuted("case-1"))
}

// TestRuleWithComplexConditions tests rule with complex conditions
func TestRuleWithComplexConditions(t *testing.T) {
	rule := NewRule("test-workspace", "Complex Rule", "test-user")
	rule.Priority = 10
	rule.MaxExecutionsPerDay = 100
	rule.MaxExecutionsPerHour = 10
	rule.TeamID = "team-123"
	rule.CaseTypes = []shareddomain.CaseChannel{"bug", "feature"}
	rule.Priorities = []shareddomain.CasePriority{"high", "critical"}

	assert.Equal(t, 10, rule.Priority)
	assert.Equal(t, 100, rule.MaxExecutionsPerDay)
	assert.Equal(t, 10, rule.MaxExecutionsPerHour)
	assert.Equal(t, "team-123", rule.TeamID)
	assert.Equal(t, []shareddomain.CaseChannel{"bug", "feature"}, rule.CaseTypes)
	assert.Equal(t, []shareddomain.CasePriority{"high", "critical"}, rule.Priorities)
}

// TestRuleExecutionTracking tests execution tracking
func TestRuleExecutionTracking(t *testing.T) {
	rule := NewRule("test-workspace", "Test Rule", "test-user")
	caseObj := servicedomain.NewCase("test-workspace", "Test Case", "contact@example.com")

	// Execute rule - this now automatically updates TotalExecutions and LastExecutedAt
	execution, err := rule.Execute(caseObj)
	require.NoError(t, err)

	// Verify automatic tracking from Execute()
	assert.Equal(t, 1, rule.TotalExecutions)
	assert.NotNil(t, rule.LastExecutedAt)

	// Manually add to execution history (as would be done by the service layer)
	rule.ExecutionHistory = append(rule.ExecutionHistory, *execution)

	assert.Equal(t, 1, len(rule.ExecutionHistory))
	assert.Equal(t, execution.ID, rule.ExecutionHistory[0].ID)

	// Execute again and verify incrementing
	_, err = rule.Execute(caseObj)
	require.NoError(t, err)
	assert.Equal(t, 2, rule.TotalExecutions)
}

func TestRuleMutationHelpersAndParsers(t *testing.T) {
	rule := NewRule("workspace-1", "Rule", "user-1")
	rule.AddCondition(shareddomain.ConditionTypeField, "status", shareddomain.OpEquals, shareddomain.StringValue("open"))
	rule.AddAction(shareddomain.ActionTypeSetStatus, shareddomain.StringValue("pending"))
	rule.AddActionWithField(shareddomain.ActionTypeSetCustomField, "segment", shareddomain.StringValue("vip"))

	conditions, err := rule.ParseConditions()
	require.NoError(t, err)
	require.Len(t, conditions.Conditions, 1)
	assert.Equal(t, "status", conditions.Conditions[0].Field)

	actions, err := rule.ParseActions()
	require.NoError(t, err)
	require.Len(t, actions.Actions, 2)
	assert.Equal(t, "segment", actions.Actions[1].Field)
}

func TestRuleHasTimeBasedConditions(t *testing.T) {
	rule := NewRule("workspace-1", "Rule", "user-1")
	assert.False(t, rule.HasTimeBasedConditions())

	rule.Conditions.Conditions = []TypedCondition{
		{Type: string(shareddomain.ConditionTypeField), Field: "status", Operator: "equals", Value: shareddomain.StringValue("open")},
		{Type: string(shareddomain.ConditionTypeTime), Field: "created_at", Operator: "hours_since", Value: shareddomain.IntValue(1)},
	}
	assert.True(t, rule.HasTimeBasedConditions())
}

func TestRuleGetFieldValueAndCompareValues(t *testing.T) {
	rule := NewRule("workspace-1", "Rule", "user-1")
	caseObj := servicedomain.NewCase("workspace-1", "Billing issue", "contact@example.com")
	caseObj.Priority = servicedomain.CasePriorityHigh
	caseObj.Tags = []string{"vip", "sla"}
	caseObj.CustomFields.SetString("segment", "enterprise")
	caseObj.CustomFields.SetInt("attempts", 3)
	caseObj.CustomFields.SetBool("escalated", true)

	assert.Equal(t, "high", rule.getFieldValue("priority", caseObj).AsString())
	assert.Equal(t, "enterprise", rule.getFieldValue("segment", caseObj).AsString())
	assert.Equal(t, int64(3), rule.getFieldValue("attempts", caseObj).AsInt())
	assert.True(t, rule.getFieldValue("escalated", caseObj).AsBool())
	assert.True(t, rule.getFieldValue("missing", caseObj).IsNull())

	assert.True(t, rule.compareValues(shareddomain.StringValue("vip"), shareddomain.StringsValue([]string{"vip", "sla"}), shareddomain.OpIn))
	assert.True(t, rule.compareValues(shareddomain.StringValue("vip"), shareddomain.StringValue("vip,sla"), shareddomain.OpIn))
	assert.True(t, rule.compareValues(shareddomain.StringValue("low"), shareddomain.StringsValue([]string{"vip", "sla"}), shareddomain.OpNotIn))
	assert.True(t, rule.compareValues(shareddomain.StringValue("value"), shareddomain.NullValue(), shareddomain.OpExists))
	assert.True(t, rule.compareValues(shareddomain.NullValue(), shareddomain.NullValue(), shareddomain.OpNotExists))
	assert.True(t, rule.compareValues(shareddomain.StringValue(""), shareddomain.NullValue(), shareddomain.OpIsEmpty))
	assert.True(t, rule.compareValues(shareddomain.StringValue("value"), shareddomain.NullValue(), shareddomain.OpIsNotEmpty))
}

func TestRuleExecuteActionVariants(t *testing.T) {
	rule := NewRule("workspace-1", "Rule", "user-1")
	caseObj := servicedomain.NewCase("workspace-1", "Subject", "contact@example.com")
	caseObj.ID = "case-1"
	caseObj.Tags = []string{"vip"}
	changes := shareddomain.NewChangeSet()

	actions := []TypedAction{
		{Type: ""},
		{Type: string(shareddomain.ActionTypeAssign), Value: shareddomain.StringValue("user-1")},
		{Type: string(shareddomain.ActionTypeSetTeam), Value: shareddomain.StringValue("team-1")},
		{Type: string(shareddomain.ActionTypeAddTag), Value: shareddomain.StringValue("vip")},
		{Type: string(shareddomain.ActionTypeAddTag), Value: shareddomain.StringValue("urgent")},
		{Type: string(shareddomain.ActionTypeSetCategory), Value: shareddomain.StringValue("billing")},
		{Type: string(shareddomain.ActionTypeSetCustomField), Field: "attempts", Value: shareddomain.IntValue(4)},
		{Type: string(shareddomain.ActionTypeSetCustomField), Field: "escalated", Value: shareddomain.BoolValue(true)},
		{Type: string(shareddomain.ActionTypeSetCustomField), Field: "labels", Value: shareddomain.StringsValue([]string{"vip", "sla"})},
		{Type: string(shareddomain.ActionTypeMute), Value: shareddomain.StringValue("true")},
		{Type: string(shareddomain.ActionTypeSendEmail), Value: shareddomain.StringValue("notify")},
	}
	rule.Actions = TypedActions{Actions: actions}

	executed, err := rule.executeActions(caseObj, changes)
	require.NoError(t, err)
	assert.Len(t, executed, len(actions)-1)
	assert.Equal(t, "user-1", caseObj.AssignedToID)
	assert.Equal(t, "team-1", caseObj.TeamID)
	assert.Equal(t, "billing", caseObj.Category)
	assert.ElementsMatch(t, []string{"vip", "urgent"}, caseObj.Tags)
	assert.True(t, rule.IsMuted("case-1"))

	attempts, ok := caseObj.CustomFields.GetInt("attempts")
	require.True(t, ok)
	assert.Equal(t, int64(4), attempts)
	escalated, ok := caseObj.CustomFields.GetBool("escalated")
	require.True(t, ok)
	assert.True(t, escalated)
	labels, ok := caseObj.CustomFields.GetStrings("labels")
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"vip", "sla"}, labels)

	_, ok = changes.GetChange("send_email_requested")
	assert.True(t, ok)
	_, ok = changes.GetChange("muted")
	assert.True(t, ok)

	err = rule.executeAction(
		TypedAction{Type: string(shareddomain.ActionTypeRemoveTag), Value: shareddomain.StringValue("")},
		caseObj,
		shareddomain.NewChangeSet(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag cannot be empty")
}

func TestRuleEvaluateConditionChangedBranch(t *testing.T) {
	rule := NewRule("workspace-1", "Rule", "user-1")
	current := servicedomain.NewCase("workspace-1", "Subject", "contact@example.com")
	current.Status = servicedomain.CaseStatusOpen
	previous := servicedomain.NewCase("workspace-1", "Subject", "contact@example.com")
	previous.Status = servicedomain.CaseStatusNew

	assert.True(t, rule.evaluateCondition(TypedCondition{
		Field:    "status",
		Operator: string(shareddomain.OpChanged),
	}, current, previous))

	previous.Status = servicedomain.CaseStatusOpen
	assert.False(t, rule.evaluateCondition(TypedCondition{
		Field:    "status",
		Operator: string(shareddomain.OpChanged),
	}, current, previous))

	assert.True(t, rule.evaluateCondition(TypedCondition{
		Field:    "status",
		Operator: string(shareddomain.OpChanged),
	}, current, nil))
}

// TestWorkflowWithStepsAndTriggers tests workflow configuration
