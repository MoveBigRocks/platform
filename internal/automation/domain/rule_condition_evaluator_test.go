package automationdomain

import (
	"testing"
	"time"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRuleConditionEvaluator(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	assert.NotNil(t, evaluator)
}

func TestRuleConditionEvaluator_EvaluateConditions_EmptyConditions(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	context := &RuleContext{
		Case: servicedomain.NewCase("workspace-1", "Test", "test@example.com"),
	}

	result, err := evaluator.EvaluateConditions(RuleConditionsData{}, context)
	require.NoError(t, err)
	assert.True(t, result, "Empty conditions should always match")
}

func TestRuleConditionEvaluator_EvaluateConditions_SimpleEquality(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	caseObj.Status = servicedomain.CaseStatusOpen

	context := &RuleContext{
		Case: caseObj,
	}

	// Test matching condition
	conditions := RuleConditionsData{
		Conditions: []RuleCondition{
			{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.StringValue("open")},
		},
	}
	result, err := evaluator.EvaluateConditions(conditions, context)
	require.NoError(t, err)
	assert.True(t, result)

	// Test non-matching condition
	conditions = RuleConditionsData{
		Conditions: []RuleCondition{
			{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.StringValue("closed")},
		},
	}
	result, err = evaluator.EvaluateConditions(conditions, context)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestRuleConditionEvaluator_EvaluateConditions_ArrayFormat(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "URGENT: Please help", "test@example.com")
	caseObj.Priority = servicedomain.CasePriorityHigh

	context := &RuleContext{
		Case: caseObj,
	}

	conditions := RuleConditionsData{
		Operator: "and",
		Conditions: []RuleCondition{
			{Type: "field", Field: "subject", Operator: "contains", Value: shareddomain.StringValue("urgent")},
			{Type: "field", Field: "priority", Operator: "equals", Value: shareddomain.StringValue("high")},
		},
	}

	result, err := evaluator.EvaluateConditions(conditions, context)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestRuleConditionEvaluator_CompareValues_Operators(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	testCases := []struct {
		name     string
		actual   interface{}
		expected interface{}
		operator string
		want     bool
	}{
		// Equals
		{"equals_match", "open", "open", "equals", true},
		{"equals_no_match", "open", "closed", "equals", false},
		{"eq_alias", "test", "test", "eq", true},
		{"double_equals_alias", "test", "test", "==", true},

		// Not equals
		{"not_equals_match", "open", "closed", "not_equals", true},
		{"not_equals_no_match", "open", "open", "not_equals", false},
		{"ne_alias", "a", "b", "ne", true},
		{"not_equals_double_alias", "a", "b", "!=", true},

		// Contains
		{"contains_match", "Hello World", "world", "contains", true},
		{"contains_case_insensitive", "URGENT", "urgent", "contains", true},
		{"contains_no_match", "Hello", "world", "contains", false},

		// Not contains
		{"not_contains_match", "Hello", "world", "not_contains", true},
		{"not_contains_no_match", "Hello World", "world", "not_contains", false},

		// Starts with
		{"starts_with_match", "Hello World", "hello", "starts_with", true},
		{"starts_with_no_match", "Hello World", "world", "starts_with", false},

		// Ends with
		{"ends_with_match", "Hello World", "world", "ends_with", true},
		{"ends_with_no_match", "Hello World", "hello", "ends_with", false},

		// In
		{"in_array_match", "high", []interface{}{"high", "urgent"}, "in", true},
		{"in_array_no_match", "low", []interface{}{"high", "urgent"}, "in", false},
		{"in_string_match", "high", "high,urgent", "in", true},

		// Not in
		{"not_in_array_match", "low", []interface{}{"high", "urgent"}, "not_in", true},
		{"not_in_array_no_match", "high", []interface{}{"high", "urgent"}, "not_in", false},

		// Greater than
		{"greater_than_match", 10, 5, "greater_than", true},
		{"greater_than_no_match", 5, 10, "greater_than", false},
		{"gt_alias", 10, 5, "gt", true},
		{"greater_than_symbol", 10, 5, ">", true},

		// Greater equal
		{"greater_equal_match_greater", 10, 5, "greater_equal", true},
		{"greater_equal_match_equal", 5, 5, "greater_equal", true},
		{"greater_equal_no_match", 4, 5, "greater_equal", false},

		// Less than
		{"less_than_match", 5, 10, "less_than", true},
		{"less_than_no_match", 10, 5, "less_than", false},
		{"lt_alias", 5, 10, "lt", true},

		// Less equal
		{"less_equal_match_less", 5, 10, "less_equal", true},
		{"less_equal_match_equal", 5, 5, "less_equal", true},
		{"less_equal_no_match", 10, 5, "less_equal", false},

		// Is empty
		{"is_empty_empty_string", "", nil, "is_empty", true},
		{"is_empty_non_empty_string", "test", nil, "is_empty", false},
		{"is_empty_nil", nil, nil, "is_empty", true},
		{"is_empty_empty_slice", []interface{}{}, nil, "is_empty", true},

		// Is not empty
		{"is_not_empty_non_empty", "test", nil, "is_not_empty", true},
		{"is_not_empty_empty", "", nil, "is_not_empty", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.compareValues(tc.actual, tc.expected, tc.operator)
			require.NoError(t, err)
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestRuleConditionEvaluator_CompareValues_Regex(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	// Valid regex match
	result, err := evaluator.compareValues("test@example.com", `^[a-z]+@[a-z]+\.[a-z]+$`, "regex")
	require.NoError(t, err)
	assert.True(t, result)

	// Valid regex no match
	result, err = evaluator.compareValues("invalid", `^[a-z]+@[a-z]+\.[a-z]+$`, "regex")
	require.NoError(t, err)
	assert.False(t, result)

	// Invalid regex should error
	_, err = evaluator.compareValues("test", `[invalid`, "regex")
	assert.Error(t, err)
}

func TestRuleConditionEvaluator_CompareValues_UnknownOperator(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	_, err := evaluator.compareValues("test", "test", "unknown_operator")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown operator")
}

func TestRuleConditionEvaluator_GetCaseFieldValue(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test Subject", "contact@example.com")
	caseObj.Status = servicedomain.CaseStatusOpen
	caseObj.Priority = servicedomain.CasePriorityHigh
	caseObj.Channel = servicedomain.CaseChannelEmail
	caseObj.Category = "billing"
	caseObj.Tags = []string{"vip", "urgent"}
	caseObj.AssignedToID = "user-123"
	caseObj.TeamID = "team-456"
	caseObj.ContactID = "contact-789"
	caseObj.ContactName = "John Doe"
	caseObj.CustomFields.SetString("custom_field", "custom_value")

	testCases := []struct {
		field    string
		expected interface{}
	}{
		{"id", caseObj.ID},
		{"subject", "Test Subject"},
		{"status", "open"},
		{"priority", "high"},
		{"channel", "email"},
		{"category", "billing"},
		{"assigned_to_id", "user-123"},
		{"team_id", "team-456"},
		{"contact_id", "contact-789"},
		{"contact_email", "contact@example.com"},
		{"contact_name", "John Doe"},
		{"custom_field", "custom_value"},
	}

	for _, tc := range testCases {
		t.Run(tc.field, func(t *testing.T) {
			value, err := evaluator.getCaseFieldValue(tc.field, caseObj)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, value)
		})
	}

	// Test tags separately (slice comparison)
	tagsValue, err := evaluator.getCaseFieldValue("tags", caseObj)
	require.NoError(t, err)
	assert.Equal(t, []string{"vip", "urgent"}, tagsValue)

	// Test unknown field
	_, err = evaluator.getCaseFieldValue("unknown_field", caseObj)
	assert.Error(t, err)
}

func TestRuleConditionEvaluator_GetContactFieldValue(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	contact := &platformdomain.Contact{
		ID:      "contact-1",
		Email:   "test@example.com",
		Name:    "John Doe",
		Phone:   "+1234567890",
		Company: "Acme Corp",
	}

	testCases := []struct {
		field    string
		expected interface{}
	}{
		{"id", "contact-1"},
		{"email", "test@example.com"},
		{"name", "John Doe"},
		{"phone", "+1234567890"},
		{"company", "Acme Corp"},
	}

	for _, tc := range testCases {
		t.Run(tc.field, func(t *testing.T) {
			value, err := evaluator.getContactFieldValue(tc.field, contact)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, value)
		})
	}

	// Test unknown field
	_, err := evaluator.getContactFieldValue("unknown_field", contact)
	assert.Error(t, err)
}

func TestRuleConditionEvaluator_GetFieldValue_DotNotation(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	caseObj.Status = servicedomain.CaseStatusOpen

	contact := &platformdomain.Contact{
		Email: "contact@example.com",
		Name:  "John Doe",
	}

	fc := NewFieldChanges()
	fc.Set("status", "pending")
	context := &RuleContext{
		Case:    caseObj,
		Contact: contact,
		Changes: fc,
	}

	// Test case field with dot notation
	value, err := evaluator.getFieldValue("case.status", context)
	require.NoError(t, err)
	assert.Equal(t, "open", value)

	// Test contact field with dot notation
	value, err = evaluator.getFieldValue("contact.email", context)
	require.NoError(t, err)
	assert.Equal(t, "contact@example.com", value)

	// Test changes field
	value, err = evaluator.getFieldValue("changes.status", context)
	require.NoError(t, err)
	assert.Equal(t, "pending", value)

	// Test direct field (defaults to case)
	value, err = evaluator.getFieldValue("subject", context)
	require.NoError(t, err)
	assert.Equal(t, "Test", value)
}

func TestRuleConditionEvaluator_GetFieldValue_NilContext(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	// Nil case
	context := &RuleContext{
		Case: nil,
	}
	_, err := evaluator.getFieldValue("status", context)
	assert.Error(t, err)

	// Nil contact
	context = &RuleContext{
		Case:    servicedomain.NewCase("workspace-1", "Test", "test@example.com"),
		Contact: nil,
	}
	value, err := evaluator.getFieldValue("contact.email", context)
	require.NoError(t, err)
	assert.Nil(t, value)
}

func TestRuleConditionEvaluator_EvaluateCondition_Types(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	context := &RuleContext{
		Case: servicedomain.NewCase("workspace-1", "Test", "test@example.com"),
	}

	// Field type (default)
	condition := RuleCondition{
		Type:     "field",
		Field:    "subject",
		Operator: "equals",
		Value:    shareddomain.StringValue("Test"),
	}
	result, err := evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)

	// Empty type defaults to field
	condition.Type = ""
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)

	// Time type - now implemented
	// Use a proper time condition to test it evaluates
	timeCondition := RuleCondition{
		Type:     "time",
		Field:    "created_at",
		Operator: "hours_since",
		Value:    shareddomain.IntValue(1000), // Case just created, so won't match 1000 hours
	}
	result, err = evaluator.EvaluateCondition(timeCondition, context)
	require.NoError(t, err)
	assert.False(t, result) // Case just created, not 1000 hours old

	// Custom type (not implemented, returns false)
	condition.Type = "custom"
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)

	// Custom type with metadata lookup
	context.Metadata = NewRuleMetadata()
	context.Metadata.SetExtensionAny("priority_bucket", "vip")

	metadataCondition := RuleCondition{
		Type:     "custom",
		Field:    "metadata.priority_bucket",
		Operator: "equals",
		Value:    shareddomain.StringValue("vip"),
	}
	result, err = evaluator.EvaluateCondition(metadataCondition, context)
	require.NoError(t, err)
	assert.True(t, result)

	metadataCondition.Value = shareddomain.StringValue("standard")
	result, err = evaluator.EvaluateCondition(metadataCondition, context)
	require.NoError(t, err)
	assert.False(t, result)

	// Unknown type
	condition.Type = "unknown"
	_, err = evaluator.EvaluateCondition(condition, context)
	assert.Error(t, err)
}

func TestUnwrapValue(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	assert.Equal(t, "value", unwrapValue(shareddomain.StringValue("value")))
	assert.Equal(t, int64(3), unwrapValue(shareddomain.IntValue(3)))
	assert.Equal(t, []string{"a", "b"}, unwrapValue(shareddomain.StringsValue([]string{"a", "b"})))
	assert.Equal(t, now, unwrapValue(shareddomain.TimeValue(now)))
	assert.Nil(t, unwrapValue(shareddomain.NullValue()))
	assert.Equal(t, "raw", unwrapValue("raw"))
}

func TestRuleConditionEvaluatorEvaluateEventCondition(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	context := &RuleContext{
		Case:  servicedomain.NewCase("workspace-1", "Subject", "test@example.com"),
		Event: "case_created",
	}

	match, err := evaluator.evaluateEventCondition(RuleCondition{
		Type:     "event",
		Field:    "trigger",
		Operator: "equals",
		Value:    shareddomain.StringValue("case_created"),
	}, context)
	require.NoError(t, err)
	assert.True(t, match)

	match, err = evaluator.evaluateEventCondition(RuleCondition{
		Type:     "event",
		Field:    "trigger",
		Operator: "not_equals",
		Value:    shareddomain.StringValue("case_created"),
	}, context)
	require.NoError(t, err)
	assert.False(t, match)
}

func TestRuleConditionEvaluatorGetIssueFieldValue(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	now := time.Unix(1700000000, 0).UTC()
	issue := &IssueContextData{
		ID:          "issue-1",
		Title:       "Crash loop",
		Status:      "unresolved",
		Level:       "error",
		Culprit:     "worker.go",
		Type:        "error",
		EventCount:  11,
		UserCount:   4,
		ProjectID:   "project-1",
		WorkspaceID: "workspace-1",
		AssignedTo:  "user-1",
		FirstSeen:   now.Add(-time.Hour),
		LastSeen:    now,
	}

	value, err := evaluator.getIssueFieldValue("event_count", issue)
	require.NoError(t, err)
	assert.Equal(t, 11, value)

	value, err = evaluator.getIssueFieldValue("assigned_to", issue)
	require.NoError(t, err)
	assert.Equal(t, "user-1", value)

	value, err = evaluator.getIssueFieldValue("last_seen", issue)
	require.NoError(t, err)
	assert.Equal(t, now, value)

	_, err = evaluator.getIssueFieldValue("unknown", issue)
	assert.Error(t, err)

	_, err = evaluator.getIssueFieldValue("title", nil)
	assert.EqualError(t, err, "issue is nil")
}

func TestRuleConditionEvaluatorGetFormFieldValue(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	form := &contracts.FormSubmittedEvent{
		FormID:         "form-1",
		FormSlug:       "contact",
		SubmissionID:   "sub-1",
		WorkspaceID:    "workspace-1",
		SubmitterEmail: "person@example.com",
		SubmitterName:  "Person",
		Data: map[string]interface{}{
			"subject": "Hello",
			"count":   3,
		},
	}

	value, err := evaluator.getFormFieldValue("submitter_email", form)
	require.NoError(t, err)
	assert.Equal(t, "person@example.com", value)

	value, err = evaluator.getFormFieldValue("subject", form)
	require.NoError(t, err)
	assert.Equal(t, "Hello", value)

	_, err = evaluator.getFormFieldValue("missing", form)
	assert.Error(t, err)

	_, err = evaluator.getFormFieldValue("form_id", nil)
	assert.EqualError(t, err, "form submission is nil")
}

func TestRuleConditionEvaluator_TimeCondition_DurationOperators(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	caseObj.CreatedAt = time.Now().Add(-72 * time.Hour)

	context := &RuleContext{
		Case: caseObj,
	}

	condition := RuleCondition{
		Type:     "time",
		Field:    "created_at",
		Operator: "older_than",
		Value:    shareddomain.StringValue("3d"),
	}
	result, err := evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)

	condition.Value = shareddomain.StringValue("4d")
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.False(t, result)

	// Numeric values are interpreted as seconds.
	condition.Value = shareddomain.IntValue(300000)
	condition.Operator = "younger_than"
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)

	condition.Value = shareddomain.IntValue(100000)
	condition.Operator = "younger_than"
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.False(t, result)

	condition.Value = shareddomain.IntValue(1000000)
	condition.Operator = "younger_than"
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestRuleConditionEvaluator_IsEmpty(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	// Nil
	assert.True(t, evaluator.isEmpty(nil))

	// Empty string
	assert.True(t, evaluator.isEmpty(""))
	assert.True(t, evaluator.isEmpty("   "))

	// Non-empty string
	assert.False(t, evaluator.isEmpty("test"))

	// Empty slices
	assert.True(t, evaluator.isEmpty([]interface{}{}))
	assert.True(t, evaluator.isEmpty([]string{}))

	// Non-empty slices
	assert.False(t, evaluator.isEmpty([]interface{}{"a"}))
	assert.False(t, evaluator.isEmpty([]string{"a"}))

	// Empty map
	assert.True(t, evaluator.isEmpty(map[string]interface{}{}))

	// Non-empty map
	assert.False(t, evaluator.isEmpty(map[string]interface{}{"key": "value"}))
}

func TestRuleConditionEvaluator_ToFloat64(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	// Float64
	result, err := evaluator.toFloat64(3.14)
	require.NoError(t, err)
	assert.Equal(t, 3.14, result)

	// Float32
	result, err = evaluator.toFloat64(float32(3.14))
	require.NoError(t, err)
	assert.InDelta(t, 3.14, result, 0.001)

	// Int
	result, err = evaluator.toFloat64(42)
	require.NoError(t, err)
	assert.Equal(t, 42.0, result)

	// Int32
	result, err = evaluator.toFloat64(int32(42))
	require.NoError(t, err)
	assert.Equal(t, 42.0, result)

	// Int64
	result, err = evaluator.toFloat64(int64(42))
	require.NoError(t, err)
	assert.Equal(t, 42.0, result)

	// String
	result, err = evaluator.toFloat64("3.14")
	require.NoError(t, err)
	assert.Equal(t, 3.14, result)

	// Invalid string
	_, err = evaluator.toFloat64("not a number")
	assert.Error(t, err)

	// Unsupported type
	_, err = evaluator.toFloat64(struct{}{})
	assert.Error(t, err)
}

func TestRuleConditionEvaluator_MultipleConditions_ANDLogic(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Billing question", "test@example.com")
	caseObj.Status = servicedomain.CaseStatusNew
	caseObj.Priority = servicedomain.CasePriorityHigh

	context := &RuleContext{
		Case: caseObj,
	}

	// Both conditions match
	conditions := RuleConditionsData{
		Operator: "and",
		Conditions: []RuleCondition{
			{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.StringValue("new")},
			{Type: "field", Field: "priority", Operator: "equals", Value: shareddomain.StringValue("high")},
		},
	}
	result, err := evaluator.EvaluateConditions(conditions, context)
	require.NoError(t, err)
	assert.True(t, result)

	// First condition matches, second doesn't
	conditions = RuleConditionsData{
		Operator: "and",
		Conditions: []RuleCondition{
			{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.StringValue("new")},
			{Type: "field", Field: "priority", Operator: "equals", Value: shareddomain.StringValue("low")},
		},
	}
	result, err = evaluator.EvaluateConditions(conditions, context)
	require.NoError(t, err)
	assert.False(t, result)

	// Neither condition matches
	conditions = RuleConditionsData{
		Operator: "and",
		Conditions: []RuleCondition{
			{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.StringValue("closed")},
			{Type: "field", Field: "priority", Operator: "equals", Value: shareddomain.StringValue("low")},
		},
	}
	result, err = evaluator.EvaluateConditions(conditions, context)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestRuleConditionEvaluator_Equals_NilHandling(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	// Both nil
	assert.True(t, evaluator.equals(nil, nil))

	// Actual nil, expected not nil
	assert.False(t, evaluator.equals(nil, "value"))

	// Actual not nil, expected nil
	assert.False(t, evaluator.equals("value", nil))

	// Both non-nil but equal
	assert.True(t, evaluator.equals("test", "test"))

	// Both non-nil but different
	assert.False(t, evaluator.equals("test", "different"))

	// Different types that convert to same string
	assert.True(t, evaluator.equals(123, "123"))
	assert.True(t, evaluator.equals(true, "true"))
}

func TestRuleConditionEvaluator_In_StringSplitWithSpaces(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	// String with spaces in comma-separated list
	result := evaluator.in("high", "low, medium, high, critical")
	assert.True(t, result)

	// Value not in list
	result = evaluator.in("urgent", "low, medium, high")
	assert.False(t, result)

	// Value that isn't an interface{} array or string
	result = evaluator.in("test", 123)
	assert.False(t, result)
}

func TestRuleConditionEvaluator_GreaterThanComparisons_EdgeCases(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	// Test with string numbers
	result, err := evaluator.compareValues("10", "5", "greater_than")
	require.NoError(t, err)
	assert.True(t, result)

	// Test with float strings
	result, err = evaluator.compareValues("10.5", "10.1", "greater_than")
	require.NoError(t, err)
	assert.True(t, result)

	// Test greater_equal with equal values
	result, err = evaluator.compareValues(5, 5, "greater_equal")
	require.NoError(t, err)
	assert.True(t, result)

	// Test less_equal with equal values
	result, err = evaluator.compareValues(5, 5, "less_equal")
	require.NoError(t, err)
	assert.True(t, result)

	// Test with non-numeric values (should error or return false)
	_, err = evaluator.compareValues("abc", "def", "greater_than")
	assert.Error(t, err)
}

func TestRuleConditionEvaluator_IsEmpty_MoreTypes(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	// Whitespace only string
	assert.True(t, evaluator.isEmpty("   \t\n"))

	// Numbers are not considered "empty" in this implementation
	assert.False(t, evaluator.isEmpty(0))
	assert.False(t, evaluator.isEmpty(1))

	// Boolean values are not considered empty
	assert.False(t, evaluator.isEmpty(false))
	assert.False(t, evaluator.isEmpty(true))

	// Pointer types
	var nilPtr *string = nil
	assert.True(t, evaluator.isEmpty(nilPtr))

	nonNilStr := "test"
	assert.False(t, evaluator.isEmpty(&nonNilStr))
}

func TestRuleConditionEvaluator_EvaluateFieldCondition_NilCase(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	// Context with nil case
	context := &RuleContext{
		Case: nil,
	}

	result, err := evaluator.evaluateFieldCondition(RuleCondition{
		Field:    "status",
		Operator: "equals",
		Value:    shareddomain.StringValue("open"),
	}, context)

	assert.Error(t, err)
	assert.False(t, result)
}

func TestRuleConditionEvaluator_GetCaseFieldValue_Description(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test Subject", "test@example.com")
	caseObj.Description = "This is a detailed description of the issue"

	value, err := evaluator.getCaseFieldValue("description", caseObj)
	require.NoError(t, err)
	assert.Equal(t, "This is a detailed description of the issue", value)
}

// Note: workspace_id is not a recognized field in getCaseFieldValue

func TestRuleConditionEvaluator_ContainsOperator(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	assert.True(t, evaluator.contains("Hello World", "world"))
	assert.True(t, evaluator.contains("Hello World", "Hello"))
	assert.False(t, evaluator.contains("Hello", "World"))
}

func TestRuleConditionEvaluator_StartsWithOperator(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	assert.True(t, evaluator.startsWith("Hello World", "hello"))
	assert.True(t, evaluator.startsWith("Hello World", "Hello"))
	assert.False(t, evaluator.startsWith("Hello World", "World"))
}

func TestRuleConditionEvaluator_EndsWithOperator(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	assert.True(t, evaluator.endsWith("Hello World", "world"))
	assert.False(t, evaluator.endsWith("Hello World", "Hello"))
}

func TestRuleConditionEvaluator_EvaluateConditions_ORFormat(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test Subject", "test@example.com")
	caseObj.Status = servicedomain.CaseStatusNew
	caseObj.Priority = servicedomain.CasePriorityLow

	context := &RuleContext{
		Case: caseObj,
	}

	// Using "or" logic - at least one condition matches
	conditions := RuleConditionsData{
		Operator: "or",
		Conditions: []RuleCondition{
			{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.StringValue("closed")}, // Does not match
			{Type: "field", Field: "priority", Operator: "equals", Value: shareddomain.StringValue("low")},  // Matches
		},
	}

	// With OR logic, at least one match means true
	result, err := evaluator.EvaluateConditions(conditions, context)
	require.NoError(t, err)
	// One condition matches, so result should be true with OR logic
	assert.True(t, result)
}

func TestRuleConditionEvaluator_GetFieldValue_Changes(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")

	fc := NewFieldChanges()
	fc.Set("old_status", "open")
	fc.Set("new_status", "closed")
	context := &RuleContext{
		Case:    caseObj,
		Changes: fc,
	}

	// Test getting values from changes
	value, err := evaluator.getFieldValue("changes.old_status", context)
	require.NoError(t, err)
	assert.Equal(t, "open", value)

	value, err = evaluator.getFieldValue("changes.new_status", context)
	require.NoError(t, err)
	assert.Equal(t, "closed", value)

	// Non-existent change field
	value, err = evaluator.getFieldValue("changes.nonexistent", context)
	require.NoError(t, err)
	assert.Nil(t, value)
}

// Note: metadata access is not implemented in getFieldValue for direct field access

func TestRuleCondition_AllFields(t *testing.T) {
	condition := RuleCondition{
		Type:     "field",
		Field:    "status",
		Operator: "equals",
		Value:    shareddomain.StringValue("open"),
	}

	assert.Equal(t, "field", condition.Type)
	assert.Equal(t, "status", condition.Field)
	assert.Equal(t, "equals", condition.Operator)
	assert.Equal(t, "open", condition.Value.AsString())
}

// ==================== TIME CONDITION TESTS ====================

func TestRuleConditionEvaluator_TimeCondition_HoursSince(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	// Set created_at to 5 hours ago
	caseObj.CreatedAt = time.Now().Add(-5 * time.Hour)

	context := &RuleContext{
		Case: caseObj,
	}

	// Condition: case created more than 4 hours ago - should match
	condition := RuleCondition{
		Type:     "time",
		Field:    "created_at",
		Operator: "hours_since",
		Value:    shareddomain.IntValue(4),
	}
	result, err := evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result, "Case created 5 hours ago should match 'hours_since: 4'")

	// Condition: case created more than 6 hours ago - should not match
	condition.Value = shareddomain.IntValue(6)
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.False(t, result, "Case created 5 hours ago should NOT match 'hours_since: 6'")
}

func TestRuleConditionEvaluator_TimeCondition_DaysSince(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	// Set created_at to 3 days ago
	caseObj.CreatedAt = time.Now().Add(-3 * 24 * time.Hour)

	context := &RuleContext{
		Case: caseObj,
	}

	// Condition: case created more than 2 days ago - should match
	condition := RuleCondition{
		Type:     "time",
		Field:    "created_at",
		Operator: "days_since",
		Value:    shareddomain.IntValue(2),
	}
	result, err := evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result, "Case created 3 days ago should match 'days_since: 2'")

	// Condition: case created more than 5 days ago - should not match
	condition.Value = shareddomain.IntValue(5)
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.False(t, result, "Case created 3 days ago should NOT match 'days_since: 5'")
}

func TestRuleConditionEvaluator_TimeCondition_MinutesSince(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	// Set created_at to 30 minutes ago
	caseObj.CreatedAt = time.Now().Add(-30 * time.Minute)

	context := &RuleContext{
		Case: caseObj,
	}

	// Condition: case created more than 15 minutes ago - should match
	condition := RuleCondition{
		Type:     "time",
		Field:    "created_at",
		Operator: "minutes_since",
		Value:    shareddomain.IntValue(15),
	}
	result, err := evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)

	// Condition: case created more than 45 minutes ago - should not match
	condition.Value = shareddomain.IntValue(45)
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestRuleConditionEvaluator_TimeCondition_HoursLessThan(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	// Set created_at to 2 hours ago
	caseObj.CreatedAt = time.Now().Add(-2 * time.Hour)

	context := &RuleContext{
		Case: caseObj,
	}

	// Condition: case created less than 4 hours ago - should match
	condition := RuleCondition{
		Type:     "time",
		Field:    "created_at",
		Operator: "hours_less_than",
		Value:    shareddomain.IntValue(4),
	}
	result, err := evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)

	// Condition: case created less than 1 hour ago - should not match
	condition.Value = shareddomain.IntValue(1)
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestRuleConditionEvaluator_TimeCondition_StatusField(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	caseObj.Status = servicedomain.CaseStatusNew
	// Set updated_at to 2 hours ago (proxy for status changed at)
	caseObj.UpdatedAt = time.Now().Add(-2 * time.Hour)

	context := &RuleContext{
		Case: caseObj,
	}

	// Build options metadata
	opts := shareddomain.NewMetadata()
	opts.Set("field_value", shareddomain.StringValue("new"))

	// Condition: status "new" for more than 1 hour - should match
	condition := RuleCondition{
		Type:     "time",
		Field:    "status",
		Operator: "hours_since",
		Value:    shareddomain.IntValue(1),
		Options:  opts,
	}
	result, err := evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)

	// Condition: status "open" for more than 1 hour - should not match (wrong status)
	openOpts := shareddomain.NewMetadata()
	openOpts.Set("field_value", shareddomain.StringValue("open"))
	condition.Options = openOpts
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestRuleConditionEvaluator_TimeCondition_ResolvedAt(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	caseObj.Status = servicedomain.CaseStatusResolved
	// Set resolved_at to 30 minutes ago
	resolvedAt := time.Now().Add(-30 * time.Minute)
	caseObj.ResolvedAt = &resolvedAt

	context := &RuleContext{
		Case: caseObj,
	}

	// Condition: resolved more than 15 minutes ago - should match
	condition := RuleCondition{
		Type:     "time",
		Field:    "resolved_at",
		Operator: "minutes_since",
		Value:    shareddomain.IntValue(15),
	}
	result, err := evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)

	// Test with nil resolved_at - should return false
	caseObj.ResolvedAt = nil
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.False(t, result, "Nil resolved_at should return false")
}

func TestRuleConditionEvaluator_TimeCondition_FirstResponseAt(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	// Set first_response_at to 1 hour ago
	firstResponse := time.Now().Add(-1 * time.Hour)
	caseObj.FirstResponseAt = &firstResponse

	context := &RuleContext{
		Case: caseObj,
	}

	// Condition: first response more than 30 minutes ago
	condition := RuleCondition{
		Type:     "time",
		Field:    "first_response_at",
		Operator: "minutes_since",
		Value:    shareddomain.IntValue(30),
	}
	result, err := evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)

	// Test with nil first_response_at - should return false
	caseObj.FirstResponseAt = nil
	result, err = evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestRuleConditionEvaluator_TimeCondition_UpdatedAt(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	// Set updated_at to 3 hours ago
	caseObj.UpdatedAt = time.Now().Add(-3 * time.Hour)

	context := &RuleContext{
		Case: caseObj,
	}

	// Condition: updated more than 2 hours ago
	condition := RuleCondition{
		Type:     "time",
		Field:    "updated_at",
		Operator: "hours_since",
		Value:    shareddomain.IntValue(2),
	}
	result, err := evaluator.EvaluateCondition(condition, context)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestRuleConditionEvaluator_TimeCondition_UnknownField(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")

	context := &RuleContext{
		Case: caseObj,
	}

	// Condition with unknown field
	condition := RuleCondition{
		Type:     "time",
		Field:    "unknown_field",
		Operator: "hours_since",
		Value:    shareddomain.IntValue(1),
	}
	_, err := evaluator.EvaluateCondition(condition, context)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown time field")
}

func TestRuleConditionEvaluator_TimeCondition_UnknownOperator(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")

	context := &RuleContext{
		Case: caseObj,
	}

	// Condition with unknown operator
	condition := RuleCondition{
		Type:     "time",
		Field:    "created_at",
		Operator: "unknown_operator",
		Value:    shareddomain.IntValue(1),
	}
	_, err := evaluator.EvaluateCondition(condition, context)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown time operator")
}

func TestRuleConditionEvaluator_TimeCondition_InvalidValue(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")

	context := &RuleContext{
		Case: caseObj,
	}

	// Condition with non-numeric value
	condition := RuleCondition{
		Type:     "time",
		Field:    "created_at",
		Operator: "hours_since",
		Value:    shareddomain.StringValue("not a number"),
	}
	_, err := evaluator.EvaluateCondition(condition, context)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "time condition value must be numeric")
}

func TestRuleConditionEvaluator_TimeCondition_NilCase(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()

	context := &RuleContext{
		Case: nil,
	}

	condition := RuleCondition{
		Type:     "time",
		Field:    "created_at",
		Operator: "hours_since",
		Value:    shareddomain.IntValue(1),
	}
	_, err := evaluator.EvaluateCondition(condition, context)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no case in context")
}

func TestRuleConditionEvaluator_TimeCondition_InArrayFormat(t *testing.T) {
	evaluator := NewRuleConditionEvaluator()
	caseObj := servicedomain.NewCase("workspace-1", "Test", "test@example.com")
	caseObj.Status = servicedomain.CaseStatusNew
	caseObj.CreatedAt = time.Now().Add(-5 * time.Hour)
	caseObj.UpdatedAt = time.Now().Add(-5 * time.Hour)

	context := &RuleContext{
		Case: caseObj,
	}

	// Combined conditions: time + field
	conditions := RuleConditionsData{
		Operator: "and",
		Conditions: []RuleCondition{
			{Type: "time", Field: "created_at", Operator: "hours_since", Value: shareddomain.IntValue(4)},
			{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.StringValue("new")},
		},
	}

	result, err := evaluator.EvaluateConditions(conditions, context)
	require.NoError(t, err)
	assert.True(t, result, "Both time and field conditions should match")
}
