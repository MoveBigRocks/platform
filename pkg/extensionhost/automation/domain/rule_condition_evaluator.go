package automationdomain

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

// ============================================================================
// RuleConditionEvaluator - Core Rule Engine Condition Evaluator
// ============================================================================
//
// RuleConditionEvaluator evaluates rule conditions against a RuleContext to
// determine if a rule should fire. It supports multiple condition types, field
// resolution with dot notation, and 21 comparison operators.
//
// # Condition Formats
//
// The evaluator accepts two condition formats:
//
// Format 1 - Simple field-value pairs (implicit AND with equals operator):
//
//	{"status": "open", "priority": "high"}
//
// Format 2 - Explicit array with operators (supports AND/OR logic):
//
//	{
//	  "operator": "or",
//	  "conditions": [
//	    {"type": "field", "field": "status", "operator": "equals", "value": "open"},
//	    {"type": "field", "field": "priority", "operator": "in", "value": ["high", "urgent"]}
//	  ]
//	}
//
// Format 2 takes precedence if the "conditions" key exists.
//
// # Field Resolution (Dot Notation)
//
// Fields are resolved using dot notation with the following prefixes:
//
//	case.field     → getCaseFieldValue (status, priority, channel, tags, etc.)
//	contact.field  → getContactFieldValue (email, name, phone, company)
//	issue.field    → getIssueFieldValue (title, status, level, event_count)
//	form.field     → getFormFieldValue (form_id, submitter_email, custom data)
//	changes.field  → FieldChanges.Get() for detecting what changed
//
// BACKWARD COMPATIBILITY: Bare field names (without prefix) fall back to case
// context. For example, "status" resolves as "case.status". This means if both
// case and issue have a field with the same name, case takes precedence.
//
// # Supported Operators
//
// Equality operators (case-sensitive):
//
//	equals, eq, ==      - Exact string match
//	not_equals, ne, !=  - Not equal
//
// String operators (case-insensitive):
//
//	contains            - Substring match
//	not_contains        - Substring not present
//	starts_with         - Prefix match
//	ends_with           - Suffix match
//
// Membership operators:
//
//	in                  - Value in array/list
//	not_in              - Value not in array/list
//
// Numeric operators:
//
//	greater_than, gt, > - Greater than
//	greater_equal, gte, >= - Greater than or equal
//	less_than, lt, <    - Less than
//	less_equal, lte, <= - Less than or equal
//
// Pattern operators:
//
//	regex               - Regular expression match
//
// Presence operators:
//
//	is_empty            - Value is nil, empty string, or empty collection
//	is_not_empty        - Value has content
//
// # Type Coercion
//
// The evaluator performs automatic type coercion:
//
//   - shareddomain.Value types are unwrapped to native Go types
//   - Numeric comparisons convert strings to float64
//   - String comparisons use fmt.Sprintf("%v", value) for consistent formatting
//   - contains, starts_with, ends_with are case-insensitive
//   - equals is case-sensitive
//
// # Time Conditions
//
// Time conditions support duration-based comparisons:
//
//	{"type": "time", "field": "created_at", "operator": "hours_since", "value": 24}
//	{"type": "time", "field": "status", "operator": "hours_since", "value": 72,
//	 "options": {"field_value": "pending"}}
//
// Time operators: hours_since, days_since, minutes_since, hours_less_than,
// days_less_than, minutes_less_than
//
// Time fields: created_at, updated_at, first_response_at, resolved_at, status
//
// LIMITATION: For status field, uses UpdatedAt as proxy for when status changed.
// A proper implementation would track StatusChangedAt separately.
//
// # Example Usage
//
//	evaluator := NewRuleConditionEvaluator()
//	context := &RuleContext{Case: caseObj, Event: "case.created"}
//	match, err := evaluator.EvaluateConditions(rule.Conditions, context)
//
// ============================================================================

// RuleConditionEvaluator evaluates rule conditions against context
type RuleConditionEvaluator struct{}

// NewRuleConditionEvaluator creates a new condition evaluator
func NewRuleConditionEvaluator() *RuleConditionEvaluator {
	return &RuleConditionEvaluator{}
}

// EvaluateConditions evaluates all conditions for a rule (AND logic by default)
func (rce *RuleConditionEvaluator) EvaluateConditions(conditions RuleConditionsData, context *RuleContext) (bool, error) {
	// Validate context upfront to fail fast with clear error messages
	if context == nil {
		return false, fmt.Errorf("rule context is nil")
	}
	if err := context.Validate(); err != nil {
		return false, fmt.Errorf("invalid rule context: %w", err)
	}

	if len(conditions.Conditions) == 0 {
		return true, nil // No conditions = always match
	}

	// Evaluate based on operator (AND/OR)
	if conditions.Operator == "or" {
		// OR logic: any condition must match
		for _, condition := range conditions.Conditions {
			matches, err := rce.EvaluateCondition(condition, context)
			if err != nil {
				return false, err
			}
			if matches {
				return true, nil // Any condition passes = rule passes
			}
		}
		return false, nil
	}

	// AND logic (default): all conditions must match
	for _, condition := range conditions.Conditions {
		matches, err := rce.EvaluateCondition(condition, context)
		if err != nil {
			return false, err
		}
		if !matches {
			return false, nil // Any condition fails = rule fails
		}
	}

	return true, nil
}

// EvaluateCondition evaluates a single condition
func (rce *RuleConditionEvaluator) EvaluateCondition(condition RuleCondition, context *RuleContext) (bool, error) {
	switch condition.Type {
	case "field", "":
		return rce.evaluateFieldCondition(condition, context)
	case "event":
		return rce.evaluateEventCondition(condition, context)
	case "time":
		return rce.evaluateTimeCondition(condition, context)
	case "custom":
		return rce.evaluateCustomCondition(condition, context)
	default:
		return false, fmt.Errorf("unknown condition type: %s", condition.Type)
	}
}

// evaluateFieldCondition evaluates a field-based condition
func (rce *RuleConditionEvaluator) evaluateFieldCondition(condition RuleCondition, context *RuleContext) (bool, error) {
	// Get field value from context
	fieldValue, err := rce.getFieldValue(condition.Field, context)
	if err != nil {
		return false, err
	}

	// Compare based on operator
	return rce.compareValues(fieldValue, condition.Value, condition.Operator)
}

// getFieldValue extracts field value from context
func (rce *RuleConditionEvaluator) getFieldValue(field string, context *RuleContext) (interface{}, error) {
	// Handle dot notation (e.g., "case.priority", "contact.email")
	parts := strings.Split(field, ".")

	switch parts[0] {
	case "case", "":
		// Case fields require a case in context
		if !context.HasCase() {
			return nil, fmt.Errorf("field '%s' requires case context, but context type is '%s'",
				field, context.TargetType())
		}
		if len(parts) == 1 && parts[0] != "case" {
			// Direct field name like "priority" - treat as case field
			return rce.getCaseFieldValue(field, context.Case)
		}
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid case field: %s", field)
		}
		return rce.getCaseFieldValue(parts[1], context.Case)

	case "contact":
		if context.Contact == nil {
			return nil, nil // No contact available, return nil (not an error)
		}
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid contact field: %s", field)
		}
		return rce.getContactFieldValue(parts[1], context.Contact)

	case "changes":
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid changes field: %s", field)
		}
		if context.Changes == nil {
			return nil, nil // No changes available, return nil
		}
		v, found := context.Changes.Get(parts[1])
		if !found {
			// Return nil to let condition matching handle the "not found" case.
			// This allows rules like "changes.status exists" to work correctly.
			// If you're debugging why a rule isn't matching, check that the field name
			// is correct and that the event actually includes the expected changes.
			return nil, nil
		}
		// Convert shareddomain.Value to a native Go value for downstream operators.
		return v.ToInterface(), nil

	case "issue":
		// Issue fields require an issue in context
		if !context.HasIssue() {
			return nil, fmt.Errorf("field '%s' requires issue context, but context type is '%s'",
				field, context.TargetType())
		}
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid issue field: %s", field)
		}
		return rce.getIssueFieldValue(parts[1], context.Issue)

	case "form":
		// Form fields require a form submission in context
		if !context.HasFormSubmission() {
			return nil, fmt.Errorf("field '%s' requires form context, but context type is '%s'",
				field, context.TargetType())
		}
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid form field: %s", field)
		}
		return rce.getFormFieldValue(parts[1], context.FormSubmission)

	default:
		// Direct field names resolve against the case object by default.
		if context.HasCase() {
			return rce.getCaseFieldValue(field, context.Case)
		}
		return nil, fmt.Errorf("unknown field prefix '%s' and no case in context (context type: %s)",
			parts[0], context.TargetType())
	}
}

// getCaseFieldValue gets a field value from a case
func (rce *RuleConditionEvaluator) getCaseFieldValue(field string, caseObj *servicedomain.Case) (interface{}, error) {
	switch field {
	case "id":
		return caseObj.ID, nil
	case "subject":
		return caseObj.Subject, nil
	case "description":
		return caseObj.Description, nil
	case "status":
		return string(caseObj.Status), nil
	case "priority":
		return string(caseObj.Priority), nil
	case "channel":
		return string(caseObj.Channel), nil
	case "category":
		return caseObj.Category, nil
	case "tags":
		return caseObj.Tags, nil
	case "assigned_to_id":
		return caseObj.AssignedToID, nil
	case "team_id":
		return caseObj.TeamID, nil
	case "contact_id":
		return caseObj.ContactID, nil
	case "contact_email":
		return caseObj.ContactEmail, nil
	case "contact_name":
		return caseObj.ContactName, nil
	case "created_at":
		return caseObj.CreatedAt, nil
	case "updated_at":
		return caseObj.UpdatedAt, nil
	default:
		// Check custom fields using typed access
		if entry, exists := caseObj.CustomFields.Get(field); exists {
			switch entry.Value.Type() {
			case shareddomain.ValueTypeString:
				return entry.Value.AsString(), nil
			case shareddomain.ValueTypeInt:
				return entry.Value.AsInt(), nil
			case shareddomain.ValueTypeBool:
				return entry.Value.AsBool(), nil
			default:
				return entry.Value.AsString(), nil
			}
		}
		return nil, fmt.Errorf("unknown case field: %s", field)
	}
}

// getContactFieldValue gets a field value from a contact
func (rce *RuleConditionEvaluator) getContactFieldValue(field string, contact *platformdomain.Contact) (interface{}, error) {
	switch field {
	case "id":
		return contact.ID, nil
	case "email":
		return contact.Email, nil
	case "name":
		return contact.Name, nil
	case "phone":
		return contact.Phone, nil
	case "company":
		return contact.Company, nil
	default:
		return nil, fmt.Errorf("unknown contact field: %s", field)
	}
}

// compareValues compares two values using the specified operator
func (rce *RuleConditionEvaluator) compareValues(actual, expected interface{}, operator string) (bool, error) {
	switch operator {
	case "equals", "eq", "==":
		return rce.equals(actual, expected), nil
	case "not_equals", "ne", "!=":
		return !rce.equals(actual, expected), nil
	case "contains":
		return rce.contains(actual, expected), nil
	case "not_contains":
		return !rce.contains(actual, expected), nil
	case "starts_with":
		return rce.startsWith(actual, expected), nil
	case "ends_with":
		return rce.endsWith(actual, expected), nil
	case "in":
		return rce.in(actual, expected), nil
	case "not_in":
		return !rce.in(actual, expected), nil
	case "greater_than", "gt", ">":
		return rce.greaterThan(actual, expected)
	case "greater_equal", "gte", ">=":
		return rce.greaterEqual(actual, expected)
	case "less_than", "lt", "<":
		return rce.lessThan(actual, expected)
	case "less_equal", "lte", "<=":
		return rce.lessEqual(actual, expected)
	case "regex":
		return rce.matchesRegex(actual, expected)
	case "is_empty":
		return rce.isEmpty(actual), nil
	case "is_not_empty":
		return !rce.isEmpty(actual), nil
	default:
		return false, fmt.Errorf("unknown operator: %s", operator)
	}
}

// Comparison helper methods

// unwrapValue extracts the underlying Go value from a shareddomain.Value
func unwrapValue(v interface{}) interface{} {
	if val, ok := v.(shareddomain.Value); ok {
		if val.IsZero() {
			return nil
		}
		switch val.Type() {
		case shareddomain.ValueTypeString:
			return val.AsString()
		case shareddomain.ValueTypeInt:
			return val.AsInt()
		case shareddomain.ValueTypeFloat:
			return val.AsFloat()
		case shareddomain.ValueTypeBool:
			return val.AsBool()
		case shareddomain.ValueTypeStrings:
			return val.AsStrings()
		case shareddomain.ValueTypeTime:
			return val.AsTime()
		case shareddomain.ValueTypeDuration:
			return val.AsDuration()
		default:
			return val.AsString()
		}
	}
	return v
}

func (rce *RuleConditionEvaluator) equals(actual, expected interface{}) bool {
	// Unwrap Value types to their underlying Go values
	actual = unwrapValue(actual)
	expected = unwrapValue(expected)

	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}

	// Convert to strings for comparison if different types
	return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
}

func (rce *RuleConditionEvaluator) contains(actual, expected interface{}) bool {
	actualStr := strings.ToLower(fmt.Sprintf("%v", unwrapValue(actual)))
	expectedStr := strings.ToLower(fmt.Sprintf("%v", unwrapValue(expected)))
	return strings.Contains(actualStr, expectedStr)
}

func (rce *RuleConditionEvaluator) startsWith(actual, expected interface{}) bool {
	actualStr := strings.ToLower(fmt.Sprintf("%v", unwrapValue(actual)))
	expectedStr := strings.ToLower(fmt.Sprintf("%v", unwrapValue(expected)))
	return strings.HasPrefix(actualStr, expectedStr)
}

func (rce *RuleConditionEvaluator) endsWith(actual, expected interface{}) bool {
	actualStr := strings.ToLower(fmt.Sprintf("%v", unwrapValue(actual)))
	expectedStr := strings.ToLower(fmt.Sprintf("%v", unwrapValue(expected)))
	return strings.HasSuffix(actualStr, expectedStr)
}

func (rce *RuleConditionEvaluator) in(actual, expected interface{}) bool {
	// Unwrap both values
	actual = unwrapValue(actual)
	expected = unwrapValue(expected)

	// Handle string slice (from Value.AsStrings())
	if expectedStrings, ok := expected.([]string); ok {
		actualStr := fmt.Sprintf("%v", actual)
		for _, s := range expectedStrings {
			if s == actualStr {
				return true
			}
		}
		return false
	}

	// Handle interface{} array
	expectedArray, ok := expected.([]interface{})
	if !ok {
		// Try string split
		if expectedStr, ok := expected.(string); ok {
			parts := strings.Split(expectedStr, ",")
			for _, part := range parts {
				if strings.TrimSpace(part) == fmt.Sprintf("%v", actual) {
					return true
				}
			}
		}
		return false
	}

	actualStr := fmt.Sprintf("%v", actual)
	for _, expectedVal := range expectedArray {
		if fmt.Sprintf("%v", expectedVal) == actualStr {
			return true
		}
	}
	return false
}

func (rce *RuleConditionEvaluator) greaterThan(actual, expected interface{}) (bool, error) {
	actualNum, err := rce.toFloat64(actual)
	if err != nil {
		return false, err
	}
	expectedNum, err := rce.toFloat64(expected)
	if err != nil {
		return false, err
	}
	return actualNum > expectedNum, nil
}

func (rce *RuleConditionEvaluator) greaterEqual(actual, expected interface{}) (bool, error) {
	actualNum, err := rce.toFloat64(actual)
	if err != nil {
		return false, err
	}
	expectedNum, err := rce.toFloat64(expected)
	if err != nil {
		return false, err
	}
	return actualNum >= expectedNum, nil
}

func (rce *RuleConditionEvaluator) lessThan(actual, expected interface{}) (bool, error) {
	actualNum, err := rce.toFloat64(actual)
	if err != nil {
		return false, err
	}
	expectedNum, err := rce.toFloat64(expected)
	if err != nil {
		return false, err
	}
	return actualNum < expectedNum, nil
}

func (rce *RuleConditionEvaluator) lessEqual(actual, expected interface{}) (bool, error) {
	actualNum, err := rce.toFloat64(actual)
	if err != nil {
		return false, err
	}
	expectedNum, err := rce.toFloat64(expected)
	if err != nil {
		return false, err
	}
	return actualNum <= expectedNum, nil
}

func (rce *RuleConditionEvaluator) matchesRegex(actual, expected interface{}) (bool, error) {
	actualStr := fmt.Sprintf("%v", unwrapValue(actual))
	regexStr := fmt.Sprintf("%v", unwrapValue(expected))

	regex, err := regexp.Compile(regexStr)
	if err != nil {
		return false, fmt.Errorf("invalid regex: %w", err)
	}

	return regex.MatchString(actualStr), nil
}

func (rce *RuleConditionEvaluator) isEmpty(value interface{}) bool {
	// Unwrap shareddomain.Value first
	value = unwrapValue(value)

	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []interface{}:
		return len(v) == 0
	case []string:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		// Use reflection for other types
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Map, reflect.Array:
			return rv.Len() == 0
		case reflect.String:
			return rv.String() == ""
		case reflect.Ptr:
			return rv.IsNil()
		}
		return false
	}
}

func (rce *RuleConditionEvaluator) toFloat64(value interface{}) (float64, error) {
	// Unwrap shareddomain.Value first
	value = unwrapValue(value)

	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

// evaluateEventCondition evaluates event-based conditions
// Checks if the rule context event matches the expected event type
func (rce *RuleConditionEvaluator) evaluateEventCondition(condition RuleCondition, context *RuleContext) (bool, error) {
	// Get the expected event value - unwrap if it's a Value type
	expectedEvent := condition.Value.AsString()
	actualEvent := context.Event

	// Compare based on operator
	return rce.compareValues(actualEvent, expectedEvent, condition.Operator)
}

// evaluateTimeCondition evaluates time-based conditions
// Supported operators: hours_since, days_since, minutes_since, hours_less_than, days_less_than, older_than, younger_than
// Supported fields: created_at, updated_at, first_response_at, resolved_at, status (time since current status)
// Example: { "type": "time", "field": "created_at", "operator": "hours_since", "value": 4 }
// Example: { "type": "time", "field": "status", "field_value": "new", "operator": "hours_since", "value": 24 }
func (rce *RuleConditionEvaluator) evaluateTimeCondition(condition RuleCondition, context *RuleContext) (bool, error) {
	if context.Case == nil {
		return false, fmt.Errorf("no case in context for time condition")
	}

	caseObj := context.Case
	field := condition.Field
	operator := condition.Operator

	// Get optional field_value for status-specific conditions
	fieldValue := condition.Options.GetString("field_value")

	// Determine reference time based on field
	var referenceTime time.Time

	switch field {
	case "created_at":
		referenceTime = caseObj.CreatedAt
	case "updated_at":
		referenceTime = caseObj.UpdatedAt
	case "first_response_at":
		if caseObj.FirstResponseAt == nil {
			return false, nil // No first response yet
		}
		referenceTime = *caseObj.FirstResponseAt
	case "resolved_at":
		if caseObj.ResolvedAt == nil {
			return false, nil // Not resolved yet
		}
		referenceTime = *caseObj.ResolvedAt
	case "status":
		// For status field, check if current status matches field_value
		if fieldValue != "" && string(caseObj.Status) != fieldValue {
			return false, nil // Status doesn't match, condition not applicable
		}
		// Use UpdatedAt as a proxy for when status changed
		// (A proper implementation would track StatusChangedAt separately)
		referenceTime = caseObj.UpdatedAt
	case "status_changed_at":
		// Treat this as the current status-change reference until a dedicated timestamp exists.
		referenceTime = caseObj.UpdatedAt
	default:
		return false, fmt.Errorf("unknown time field: %s", field)
	}

	// Check if reference time is valid
	if referenceTime.IsZero() {
		return false, nil
	}

	// Calculate elapsed time
	elapsed := time.Since(referenceTime)

	// Compare based on operator
	if condition.Operator == "older_than" || condition.Operator == "younger_than" {
		rawValue := unwrapValue(condition.Value)
		switch typedValue := rawValue.(type) {
		case time.Duration:
			switch condition.Operator {
			case "older_than":
				return elapsed >= typedValue, nil
			case "younger_than":
				return elapsed < typedValue, nil
			}
		case string:
			parsed, err := parseDurationWithDays(typedValue)
			if err != nil {
				return false, fmt.Errorf("time condition value must be a duration: %w", err)
			}
			switch condition.Operator {
			case "older_than":
				return elapsed >= parsed, nil
			case "younger_than":
				return elapsed < parsed, nil
			}
		}

		// Numeric values are interpreted as seconds for concise rule authoring.
		rawNumericValue, err := rce.toFloat64(condition.Value)
		if err != nil {
			return false, fmt.Errorf("time condition value must be a duration or numeric: %w", err)
		}
		if rawNumericValue <= 0 {
			return false, fmt.Errorf("time condition value must be greater than zero")
		}
		value := time.Duration(rawNumericValue) * time.Second

		switch condition.Operator {
		case "older_than":
			return elapsed >= value, nil
		case "younger_than":
			return elapsed < value, nil
		}
	}

	// Get the value (expected to be numeric - hours, days, or minutes)
	value, err := rce.toFloat64(condition.Value)
	if err != nil {
		return false, fmt.Errorf("time condition value must be numeric: %w", err)
	}

	switch operator {
	case "hours_since":
		return elapsed.Hours() >= value, nil
	case "days_since":
		return elapsed.Hours() >= value*24, nil
	case "minutes_since":
		return elapsed.Minutes() >= value, nil
	case "hours_less_than":
		return elapsed.Hours() < value, nil
	case "days_less_than":
		return elapsed.Hours() < value*24, nil
	case "minutes_less_than":
		return elapsed.Minutes() < value, nil
	default:
		return false, fmt.Errorf("unknown time operator: %s", operator)
	}
}

// parseDurationWithDays parses time duration strings and supports `d` suffix for days.
func parseDurationWithDays(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, fmt.Errorf("duration string is empty")
	}

	// Support "3d" as 72h for default rules.
	if strings.HasSuffix(trimmed, "d") {
		daysValue := strings.TrimSuffix(trimmed, "d")
		days, err := strconv.ParseFloat(daysValue, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse duration days value %q: %w", raw, err)
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}

	return time.ParseDuration(trimmed)
}

// evaluateCustomCondition evaluates custom conditions.
// Custom conditions provide a generic extension point for non-standard predicates
// by resolving values using an explicit field path, including metadata lookups.
func (rce *RuleConditionEvaluator) evaluateCustomCondition(condition RuleCondition, context *RuleContext) (bool, error) {
	field := strings.TrimSpace(condition.Field)
	if field == "" {
		rawField, ok := condition.Options.Get("field")
		if ok && strings.TrimSpace(rawField.AsString()) != "" {
			field = strings.TrimSpace(rawField.AsString())
		}
	}
	if field == "" {
		return false, nil
	}

	// Support metadata-based custom predicates: "metadata.<key>".
	if strings.HasPrefix(field, "metadata.") {
		if context.Metadata == nil {
			return false, nil
		}
		key := strings.TrimPrefix(field, "metadata.")
		metadata := context.Metadata.ToMap()
		value, ok := metadata[key]
		if !ok {
			return false, nil
		}
		return rce.compareValues(value, condition.Value, condition.Operator)
	}

	return rce.evaluateFieldCondition(condition, context)
}

// getIssueFieldValue gets a field value from an issue
func (rce *RuleConditionEvaluator) getIssueFieldValue(field string, issue *IssueContextData) (interface{}, error) {
	if issue == nil {
		return nil, fmt.Errorf("issue is nil")
	}

	switch field {
	case "id":
		return issue.ID, nil
	case "title":
		return issue.Title, nil
	case "status":
		return issue.Status, nil
	case "level":
		return issue.Level, nil
	case "culprit":
		return issue.Culprit, nil
	case "type":
		return issue.Type, nil
	case "event_count":
		return issue.EventCount, nil
	case "user_count":
		return issue.UserCount, nil
	case "project_id":
		return issue.ProjectID, nil
	case "workspace_id":
		return issue.WorkspaceID, nil
	case "assigned_to":
		return issue.AssignedTo, nil
	case "first_seen":
		return issue.FirstSeen, nil
	case "last_seen":
		return issue.LastSeen, nil
	default:
		return nil, fmt.Errorf("unknown issue field: %s", field)
	}
}

// getFormFieldValue gets a field value from a form submission
func (rce *RuleConditionEvaluator) getFormFieldValue(field string, form *contracts.FormSubmittedEvent) (interface{}, error) {
	if form == nil {
		return nil, fmt.Errorf("form submission is nil")
	}

	switch field {
	case "form_id":
		return form.FormID, nil
	case "form_slug":
		return form.FormSlug, nil
	case "submission_id":
		return form.SubmissionID, nil
	case "workspace_id":
		return form.WorkspaceID, nil
	case "submitter_email":
		return form.SubmitterEmail, nil
	case "submitter_name":
		return form.SubmitterName, nil
	default:
		// Check in form data
		if form.Data != nil {
			if val, ok := form.Data[field]; ok {
				return val, nil
			}
		}
		return nil, fmt.Errorf("unknown form field: %s", field)
	}
}
