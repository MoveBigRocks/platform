package automationdomain

import (
	"fmt"
	"strings"
	"time"

	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

// =============================================================================
// Rule - Type-Safe Automation Rules
// =============================================================================

// Rule represents an automation rule with type-safe conditions and actions
type Rule struct {
	ID          string
	WorkspaceID string

	// Basic info
	Title         string
	Description   string
	IsActive      bool
	IsSystem      bool
	SystemRuleKey string
	Priority      int

	// Rule configuration (type-safe structures)
	Conditions TypedConditions
	Actions    TypedActions

	// Execution control
	MuteFor              []string
	MaxExecutionsPerDay  int
	MaxExecutionsPerHour int

	// Scope
	TeamID     string
	CaseTypes  []shareddomain.CaseChannel
	Priorities []shareddomain.CasePriority

	// Execution tracking
	TotalExecutions  int
	LastExecutedAt   *time.Time
	ExecutionHistory []TypedExecution

	// Performance tracking
	AverageExecutionTime int64
	SuccessRate          float64

	// Metadata
	CreatedByID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// =============================================================================
// Type-Safe Conditions
// =============================================================================

// TypedConditions wraps conditions with type-safe operator
type TypedConditions struct {
	Conditions []TypedCondition
	Operator   shareddomain.LogicalOperator
}

// TypedCondition represents a type-safe rule condition
type TypedCondition struct {
	Type     string
	Field    string
	Operator string
	Value    shareddomain.Value
	Options  shareddomain.Metadata
}

// =============================================================================
// Type-Safe Actions
// =============================================================================

// TypedActions wraps actions array
type TypedActions struct {
	Actions []TypedAction
}

// TypedAction represents a type-safe rule action
type TypedAction struct {
	Type    string
	Target  string
	Value   shareddomain.Value
	Field   string
	Options shareddomain.Metadata
}

// =============================================================================
// Type-Safe Execution
// =============================================================================

// ExecutionStatus represents the status of a rule execution
type ExecutionStatus string

const (
	ExecStatusRunning ExecutionStatus = "running"
	ExecStatusSuccess ExecutionStatus = "success"
	ExecStatusFailed  ExecutionStatus = "failed"
	ExecStatusSkipped ExecutionStatus = "skipped"
)

// TypedExecution represents a type-safe rule execution record
type TypedExecution struct {
	ID          string
	WorkspaceID string
	RuleID      string

	// Execution context
	CaseID      string
	TriggerType shareddomain.TriggerType
	Context     shareddomain.TypedContext

	// Execution details
	Status        ExecutionStatus
	StartedAt     time.Time
	CompletedAt   *time.Time
	ExecutionTime int64

	// Results
	ActionsExecuted []shareddomain.RuleActionType
	Changes         *shareddomain.ChangeSet
	ErrorMessage    string

	// Metadata
	CreatedAt time.Time
}

// =============================================================================
// Constructors
// =============================================================================

// NewRule creates a new rule with type-safe defaults
func NewRule(workspaceID, title string, createdByID string) *Rule {
	return &Rule{
		WorkspaceID: workspaceID,
		Title:       title,
		IsActive:    true,
		Conditions: TypedConditions{
			Operator:   shareddomain.LogicalAnd,
			Conditions: []TypedCondition{},
		},
		Actions: TypedActions{
			Actions: []TypedAction{},
		},
		MuteFor:          []string{},
		CaseTypes:        []shareddomain.CaseChannel{},
		Priorities:       []shareddomain.CasePriority{},
		ExecutionHistory: []TypedExecution{},
		CreatedByID:      createdByID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

// AddCondition adds a type-safe condition to the rule
func (r *Rule) AddCondition(condType shareddomain.RuleConditionType, field string, op shareddomain.Operator, value shareddomain.Value) {
	r.Conditions.Conditions = append(r.Conditions.Conditions, TypedCondition{
		Type:     string(condType),
		Field:    field,
		Operator: string(op),
		Value:    value,
		Options:  shareddomain.NewMetadata(),
	})
	r.UpdatedAt = time.Now()
}

// AddAction adds a type-safe action to the rule
func (r *Rule) AddAction(actionType shareddomain.RuleActionType, value shareddomain.Value) {
	r.Actions.Actions = append(r.Actions.Actions, TypedAction{
		Type:    string(actionType),
		Value:   value,
		Options: shareddomain.NewMetadata(),
	})
	r.UpdatedAt = time.Now()
}

// AddActionWithField adds a type-safe action with a field name (for custom fields)
func (r *Rule) AddActionWithField(actionType shareddomain.RuleActionType, field string, value shareddomain.Value) {
	r.Actions.Actions = append(r.Actions.Actions, TypedAction{
		Type:    string(actionType),
		Field:   field,
		Value:   value,
		Options: shareddomain.NewMetadata(),
	})
	r.UpdatedAt = time.Now()
}

// =============================================================================
// Evaluation Logic
// =============================================================================

// Evaluate checks if rule conditions are met against a case
func (r *Rule) Evaluate(caseObj *servicedomain.Case, oldCase *servicedomain.Case) bool {
	if caseObj == nil {
		return false
	}

	// No conditions = always match
	if len(r.Conditions.Conditions) == 0 {
		return true
	}

	// Check scope restrictions first
	if !r.matchesScope(caseObj) {
		return false
	}

	// Evaluate conditions
	return r.evaluateConditions(caseObj, oldCase)
}

// matchesScope checks if the case matches rule scope restrictions
func (r *Rule) matchesScope(caseObj *servicedomain.Case) bool {
	// Check team scope
	if r.TeamID != "" && caseObj.TeamID != r.TeamID {
		return false
	}

	// Check case type scope (CaseChannel is aliased in both packages)
	if len(r.CaseTypes) > 0 {
		matched := false
		for _, t := range r.CaseTypes {
			if string(caseObj.Channel) == string(t) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check priority scope (CasePriority is aliased in both packages)
	if len(r.Priorities) > 0 {
		matched := false
		for _, p := range r.Priorities {
			if string(caseObj.Priority) == string(p) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// evaluateConditions evaluates all conditions against the case
func (r *Rule) evaluateConditions(caseObj *servicedomain.Case, oldCase *servicedomain.Case) bool {
	useOr := r.Conditions.Operator == shareddomain.LogicalOr

	for _, cond := range r.Conditions.Conditions {
		matches := r.evaluateCondition(cond, caseObj, oldCase)
		if useOr && matches {
			return true
		}
		if !useOr && !matches {
			return false
		}
	}

	return !useOr
}

// evaluateCondition evaluates a single typed condition
func (r *Rule) evaluateCondition(cond TypedCondition, caseObj *servicedomain.Case, oldCase *servicedomain.Case) bool {
	field := cond.Field
	operator := cond.Operator
	if operator == "" {
		operator = string(shareddomain.OpEquals)
	}

	// Handle change detection
	if operator == string(shareddomain.OpChanged) {
		if oldCase == nil {
			return true
		}
		oldValue := r.getFieldValue(field, oldCase)
		newValue := r.getFieldValue(field, caseObj)
		return !oldValue.Equals(newValue)
	}

	actualValue := r.getFieldValue(field, caseObj)
	// cond.Value is already shareddomain.Value, no conversion needed
	return r.compareValues(actualValue, cond.Value, shareddomain.Operator(operator))
}

// getFieldValue extracts a field value from a case as a typed Value
func (r *Rule) getFieldValue(field string, caseObj *servicedomain.Case) shareddomain.Value {
	switch field {
	case "status":
		return shareddomain.StringValue(string(caseObj.Status))
	case "priority":
		return shareddomain.StringValue(string(caseObj.Priority))
	case "channel":
		return shareddomain.StringValue(string(caseObj.Channel))
	case "category":
		return shareddomain.StringValue(caseObj.Category)
	case "subject":
		return shareddomain.StringValue(caseObj.Subject)
	case "description":
		return shareddomain.StringValue(caseObj.Description)
	case "assigned_to_id", "assignee":
		return shareddomain.StringValue(caseObj.AssignedToID)
	case "team_id":
		return shareddomain.StringValue(caseObj.TeamID)
	case "contact_id":
		return shareddomain.StringValue(caseObj.ContactID)
	case "contact_email":
		return shareddomain.StringValue(caseObj.ContactEmail)
	case "contact_name":
		return shareddomain.StringValue(caseObj.ContactName)
	case "tags":
		return shareddomain.StringsValue(caseObj.Tags)
	default:
		// Check custom fields using typed access
		if val, ok := caseObj.CustomFields.GetString(field); ok {
			return shareddomain.StringValue(val)
		}
		if val, ok := caseObj.CustomFields.GetInt(field); ok {
			return shareddomain.IntValue(val)
		}
		if val, ok := caseObj.CustomFields.GetBool(field); ok {
			return shareddomain.BoolValue(val)
		}
		return shareddomain.NullValue()
	}
}

// compareValues compares two typed values using the specified operator
func (r *Rule) compareValues(actual, expected shareddomain.Value, operator shareddomain.Operator) bool {
	switch operator {
	case shareddomain.OpEquals:
		return actual.Equals(expected)
	case shareddomain.OpNotEquals:
		return !actual.Equals(expected)
	case shareddomain.OpContains:
		return strings.Contains(
			strings.ToLower(actual.AsString()),
			strings.ToLower(expected.AsString()),
		)
	case shareddomain.OpNotContains:
		return !strings.Contains(
			strings.ToLower(actual.AsString()),
			strings.ToLower(expected.AsString()),
		)
	case shareddomain.OpStartsWith:
		return strings.HasPrefix(
			strings.ToLower(actual.AsString()),
			strings.ToLower(expected.AsString()),
		)
	case shareddomain.OpEndsWith:
		return strings.HasSuffix(
			strings.ToLower(actual.AsString()),
			strings.ToLower(expected.AsString()),
		)
	case shareddomain.OpGreaterThan:
		return actual.AsFloat() > expected.AsFloat()
	case shareddomain.OpLessThan:
		return actual.AsFloat() < expected.AsFloat()
	case shareddomain.OpGreaterOrEq:
		return actual.AsFloat() >= expected.AsFloat()
	case shareddomain.OpLessOrEq:
		return actual.AsFloat() <= expected.AsFloat()
	case shareddomain.OpIn:
		actualStr := actual.AsString()
		for _, s := range expected.AsStrings() {
			if s == actualStr {
				return true
			}
		}
		// Also check comma-separated string
		if expected.Type() == shareddomain.ValueTypeString {
			for _, part := range strings.Split(expected.AsString(), ",") {
				if strings.TrimSpace(part) == actualStr {
					return true
				}
			}
		}
		return false
	case shareddomain.OpNotIn:
		actualStr := actual.AsString()
		for _, s := range expected.AsStrings() {
			if s == actualStr {
				return false
			}
		}
		return true
	case shareddomain.OpIsEmpty:
		return actual.IsZero() || actual.AsString() == ""
	case shareddomain.OpIsNotEmpty:
		return !actual.IsZero() && actual.AsString() != ""
	case shareddomain.OpExists:
		return !actual.IsZero()
	case shareddomain.OpNotExists:
		return actual.IsZero()
	default:
		return false
	}
}

// =============================================================================
// Execution Logic
// =============================================================================

// Execute runs the rule actions on a case
func (r *Rule) Execute(caseObj *servicedomain.Case) (*TypedExecution, error) {
	if caseObj == nil {
		return nil, fmt.Errorf("case cannot be nil")
	}

	startedAt := time.Now()
	execution := &TypedExecution{
		WorkspaceID:     r.WorkspaceID,
		RuleID:          r.ID,
		CaseID:          caseObj.ID,
		TriggerType:     shareddomain.TriggerTypeCaseUpdated,
		Status:          ExecStatusRunning,
		StartedAt:       startedAt,
		ActionsExecuted: []shareddomain.RuleActionType{},
		Changes:         shareddomain.NewChangeSet(),
		CreatedAt:       startedAt,
	}

	// Execute actions
	actionsExecuted, err := r.executeActions(caseObj, execution.Changes)
	if err != nil {
		execution.Status = ExecStatusFailed
		execution.ErrorMessage = err.Error()
	} else {
		execution.Status = ExecStatusSuccess
		execution.ActionsExecuted = actionsExecuted
	}

	// Record completion
	completedAt := time.Now()
	execution.CompletedAt = &completedAt
	execution.ExecutionTime = completedAt.Sub(startedAt).Milliseconds()

	// Update rule statistics
	r.TotalExecutions++
	r.LastExecutedAt = &completedAt
	r.UpdatedAt = time.Now()

	// Update success rate
	if execution.Status == ExecStatusSuccess {
		r.SuccessRate = (r.SuccessRate*float64(r.TotalExecutions-1) + 1.0) / float64(r.TotalExecutions)
	} else {
		r.SuccessRate = (r.SuccessRate * float64(r.TotalExecutions-1)) / float64(r.TotalExecutions)
	}

	// Update average execution time
	r.AverageExecutionTime = ((r.AverageExecutionTime * int64(r.TotalExecutions-1)) + execution.ExecutionTime) / int64(r.TotalExecutions)

	return execution, nil
}

// executeActions processes all actions defined in the rule
func (r *Rule) executeActions(caseObj *servicedomain.Case, changes *shareddomain.ChangeSet) ([]shareddomain.RuleActionType, error) {
	var executed []shareddomain.RuleActionType

	for _, action := range r.Actions.Actions {
		if action.Type == "" {
			continue
		}

		if err := r.executeAction(action, caseObj, changes); err != nil {
			return executed, fmt.Errorf("action %s failed: %w", action.Type, err)
		}
		executed = append(executed, shareddomain.RuleActionType(action.Type))
	}

	return executed, nil
}

// executeAction executes a single typed action on the case
func (r *Rule) executeAction(action TypedAction, caseObj *servicedomain.Case, changes *shareddomain.ChangeSet) error {
	// action.Value is already shareddomain.Value, no conversion needed
	actionValue := action.Value

	switch shareddomain.RuleActionType(action.Type) {
	case shareddomain.ActionTypeSetStatus:
		oldStatus := string(caseObj.Status)
		newStatus := actionValue.AsString()
		caseObj.Status = servicedomain.CaseStatus(newStatus)
		changes.RecordString("status", oldStatus, newStatus)

	case shareddomain.ActionTypeSetPriority:
		oldPriority := string(caseObj.Priority)
		newPriority := actionValue.AsString()
		caseObj.Priority = servicedomain.CasePriority(newPriority)
		changes.RecordString("priority", oldPriority, newPriority)

	case shareddomain.ActionTypeAssign:
		oldAssignee := caseObj.AssignedToID
		newAssignee := actionValue.AsString()
		caseObj.AssignedToID = newAssignee
		changes.RecordString("assigned_to_id", oldAssignee, newAssignee)

	case shareddomain.ActionTypeSetTeam:
		oldTeam := caseObj.TeamID
		newTeam := actionValue.AsString()
		caseObj.TeamID = newTeam
		changes.RecordString("team_id", oldTeam, newTeam)

	case shareddomain.ActionTypeAddTag:
		tag := actionValue.AsString()
		if !caseObj.HasTag(tag) {
			caseObj.Tags = append(caseObj.Tags, tag)
			changes.RecordString("tags_added", "", tag)
		}

	case shareddomain.ActionTypeRemoveTag:
		tag := actionValue.AsString()
		if err := caseObj.RemoveTag(tag); err != nil {
			return fmt.Errorf("remove tag: %w", err)
		}
		changes.RecordString("tags_removed", tag, "")

	case shareddomain.ActionTypeSetCategory:
		oldCategory := caseObj.Category
		newCategory := actionValue.AsString()
		caseObj.Category = newCategory
		changes.RecordString("category", oldCategory, newCategory)

	case shareddomain.ActionTypeSetCustomField:
		fieldName := action.Field
		if fieldName != "" {
			switch actionValue.Type() {
			case shareddomain.ValueTypeString:
				caseObj.CustomFields.SetString(fieldName, actionValue.AsString())
			case shareddomain.ValueTypeInt:
				caseObj.CustomFields.SetInt(fieldName, actionValue.AsInt())
			case shareddomain.ValueTypeBool:
				caseObj.CustomFields.SetBool(fieldName, actionValue.AsBool())
			case shareddomain.ValueTypeStrings:
				caseObj.CustomFields.SetStrings(fieldName, actionValue.AsStrings())
			default:
				caseObj.CustomFields.SetString(fieldName, actionValue.AsString())
			}
			changes.RecordString("custom_field_"+fieldName, "", actionValue.AsString())
		}

	case shareddomain.ActionTypeMute:
		r.MuteFor = append(r.MuteFor, caseObj.ID)
		changes.RecordBool("muted", false, true)

	// These actions are handled externally - we just record that they were requested
	case shareddomain.ActionTypeSendNotify, shareddomain.ActionTypeSendEmail,
		shareddomain.ActionTypeEscalate, shareddomain.ActionTypeWebhook:
		changes.RecordString(action.Type+"_requested", "", actionValue.AsString())
	}

	caseObj.UpdatedAt = time.Now()
	return nil
}

// =============================================================================
// Helper Methods
// =============================================================================

// IsMuted checks if a case ID is in the mute list for this rule
func (r *Rule) IsMuted(caseID string) bool {
	for _, mutedID := range r.MuteFor {
		if mutedID == caseID {
			return true
		}
	}
	return false
}

// Unmute removes a case from the mute list
func (r *Rule) Unmute(caseID string) {
	newMuteList := make([]string, 0, len(r.MuteFor))
	for _, mutedID := range r.MuteFor {
		if mutedID != caseID {
			newMuteList = append(newMuteList, mutedID)
		}
	}
	r.MuteFor = newMuteList
}

// HasTimeBasedConditions returns true if the rule has any time-based conditions
func (r *Rule) HasTimeBasedConditions() bool {
	for _, cond := range r.Conditions.Conditions {
		if cond.Type == string(shareddomain.ConditionTypeTime) {
			return true
		}
	}
	return false
}

// =============================================================================
// Typed Rule Aliases
// =============================================================================

// RuleConditionsData is the canonical alias for typed condition collections.
type RuleConditionsData = TypedConditions

// RuleCondition is the canonical alias for a typed condition.
type RuleCondition = TypedCondition

// RuleActionsData is the canonical alias for typed action collections.
type RuleActionsData = TypedActions

// RuleAction is the canonical alias for a typed action.
type RuleAction = TypedAction

// RuleExecution is the canonical alias for a typed execution record.
type RuleExecution = TypedExecution

// ParseConditions returns the typed conditions data.
func (r *Rule) ParseConditions() (*TypedConditions, error) {
	return &r.Conditions, nil
}

// ParseActions returns the typed actions data.
func (r *Rule) ParseActions() (*TypedActions, error) {
	return &r.Actions, nil
}
