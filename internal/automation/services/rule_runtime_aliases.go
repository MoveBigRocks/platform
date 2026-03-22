package automationservices

import automationdomain "github.com/movebigrocks/platform/internal/automation/domain"

type FieldChanges = automationdomain.FieldChanges

func NewFieldChanges() *FieldChanges {
	return automationdomain.NewFieldChanges()
}

type ActionChanges = automationdomain.ActionChanges

func NewActionChanges() *ActionChanges {
	return automationdomain.NewActionChanges()
}

type RuleMetadata = automationdomain.RuleMetadata

func NewRuleMetadata() *RuleMetadata {
	return automationdomain.NewRuleMetadata()
}

type IssueContextData = automationdomain.IssueContextData
type RuleContext = automationdomain.RuleContext
type RuleConditionEvaluator = automationdomain.RuleConditionEvaluator

func NewRuleConditionEvaluator() *RuleConditionEvaluator {
	return automationdomain.NewRuleConditionEvaluator()
}
