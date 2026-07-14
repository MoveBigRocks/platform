package automationservices

import automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"

// ValidateRuleActions ensures seeded or imported rule actions are understood by the
// current runtime and are not obviously incompatible with the declared trigger type.
func ValidateRuleActions(conditions automationdomain.TypedConditions, actions automationdomain.TypedActions) error {
	return automationdomain.ValidateRuleActions(conditions, actions)
}
