package automationservices

import automationdomain "github.com/movebigrocks/platform/internal/automation/domain"

// SupportedRuleActionTypes returns the action names currently understood by the runtime.
func SupportedRuleActionTypes() []string {
	return automationdomain.SupportedRuleActionTypes()
}

// ValidateRuleActions ensures seeded or imported rule actions are understood by the
// current runtime and are not obviously incompatible with the declared trigger type.
func ValidateRuleActions(conditions automationdomain.TypedConditions, actions automationdomain.TypedActions) error {
	return automationdomain.ValidateRuleActions(conditions, actions)
}
