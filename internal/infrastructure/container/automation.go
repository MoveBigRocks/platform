package container

import (
	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	"github.com/movebigrocks/platform/internal/infrastructure/outbox"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
)

// AutomationContainer holds automation domain services.
// Automation services depend on Service (CaseService) and Platform (ContactService).
type AutomationContainer struct {
	Rule   *automationservices.RuleService
	Engine *automationservices.RulesEngine
	Form   *automationservices.FormService
}

// AutomationContainerDeps contains the dependencies for creating an AutomationContainer.
// Dependencies from other domains are explicit in the struct.
type AutomationContainerDeps struct {
	Store     stores.Store
	Outbox    *outbox.Service
	Case      contracts.CaseServiceInterface   // From Service domain
	Contact   *platformservices.ContactService // From Platform domain
	Extension *platformservices.ExtensionService
}

// NewAutomationContainer creates a new automation container with all automation services.
// Dependencies on other domains (Case, Contact) are explicit in the deps struct.
func NewAutomationContainer(deps AutomationContainerDeps) *AutomationContainer {
	c := &AutomationContainer{}

	// RuleService first (RulesEngine depends on it)
	c.Rule = automationservices.NewRuleService(deps.Store.Rules())

	// RulesEngine takes services from other domains explicitly
	// This ensures all case mutations go through the service layer for proper
	// validation and event publishing
	c.Engine = automationservices.NewRulesEngine(
		c.Rule,             // For fetching rules
		deps.Case,          // For case mutations (via action executor)
		deps.Contact,       // For contact lookup
		deps.Store.Rules(), // Only for rate limiter stats persistence
		deps.Outbox,
	)
	c.Engine.SetExtensionChecker(deps.Extension)

	// FormService with full dependencies for public form handling
	c.Form = automationservices.NewFormServiceWithDeps(
		deps.Store.Forms(),
		deps.Store.Users(), // RateLimitStore
		deps.Store,         // TransactionRunner
		deps.Store,         // TenantContextSetter
		deps.Outbox,
	)

	return c
}

// Stop stops any background goroutines in automation services.
func (a *AutomationContainer) Stop() {
	if a.Engine != nil {
		a.Engine.Stop()
	}
}
