package automationservices

import (
	"context"
	"fmt"

	automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	"github.com/movebigrocks/platform/pkg/logger"
)

type ActionExtensionChecker interface {
	HasActiveExtensionInWorkspace(ctx context.Context, workspaceID, slug string) (bool, error)
}

type RuleActionExecutorOption func(*RuleActionExecutor)

// RuleActionExecutor executes rule actions using the handler registry.
// Uses the Strategy pattern with registered handlers for different action types.
//
// Architecture: All action handlers receive CaseServiceInterface instead of stores,
// ensuring that all case mutations go through the service layer for:
// - Domain validation
// - Event publishing (CaseAssigned, CaseStatusChanged, etc.)
// - Consistent business logic
type RuleActionExecutor struct {
	registry *ActionHandlerRegistry
	logger   *logger.Logger

	// Dependencies for action handlers
	caseService      contracts.CaseServiceInterface
	outbox           contracts.OutboxPublisher
	extensionChecker ActionExtensionChecker
}

// NewRuleActionExecutor creates a new action executor with the Strategy pattern
func NewRuleActionExecutor(caseService contracts.CaseServiceInterface, outbox contracts.OutboxPublisher, options ...RuleActionExecutorOption) *RuleActionExecutor {
	executor := &RuleActionExecutor{
		registry:    NewActionHandlerRegistry(),
		logger:      logger.New().WithField("service", "rule_action_executor"),
		caseService: caseService,
		outbox:      outbox,
	}
	for _, option := range options {
		if option != nil {
			option(executor)
		}
	}

	// Register default handlers
	executor.registerDefaultHandlers()

	return executor
}

func WithRuleActionExecutorExtensionChecker(checker ActionExtensionChecker) RuleActionExecutorOption {
	return func(executor *RuleActionExecutor) {
		executor.extensionChecker = checker
	}
}

func (rae *RuleActionExecutor) registerDefaultHandlers() {
	// Case actions (assign, status, priority, tags, close, custom fields, communication)
	// All handlers receive CaseService - never direct store access
	rae.registry.Register(NewCaseActionHandler(rae.caseService))

	// Escalation actions
	rae.registry.Register(NewEscalationActionHandler(rae.caseService))

	// Notification actions (email, event publishing)
	rae.registry.Register(NewNotificationActionHandler(rae.outbox))

	// Case creation actions (from issues, forms)
	rae.registry.Register(NewCaseCreationActionHandler(rae.caseService))
}

// ExecuteActions executes all rule actions from TypedActions
func (rae *RuleActionExecutor) ExecuteActions(ctx context.Context, typedActions automationdomain.TypedActions, ruleContext *RuleContext) (*ActionResult, error) {
	result := &ActionResult{
		ExecutedActions: make([]string, 0),
		Changes:         NewActionChanges(),
		Errors:          make([]string, 0),
	}

	// Convert TypedActions to RuleActions
	actions := rae.convertTypedActions(typedActions)

	// Execute each action using the registry
	for _, action := range actions {
		if err := rae.executeAction(ctx, action, ruleContext, result); err != nil {
			rae.logger.WithError(err).Warn("Action failed", "action_type", action.Type)
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", action.Type, err))
		} else {
			result.ExecutedActions = append(result.ExecutedActions, action.Type)
		}
	}

	// Report results
	if len(result.Errors) > 0 && len(result.ExecutedActions) > 0 {
		return result, fmt.Errorf("partial execution: %d errors occurred", len(result.Errors))
	} else if len(result.Errors) > 0 {
		return result, fmt.Errorf("execution failed: %d errors occurred", len(result.Errors))
	}

	return result, nil
}

// convertTypedActions converts domain TypedActions to internal RuleActions
func (rae *RuleActionExecutor) convertTypedActions(typedActions automationdomain.TypedActions) []RuleAction {
	actions := make([]RuleAction, len(typedActions.Actions))
	for i, ta := range typedActions.Actions {
		actions[i] = RuleAction{
			Type:    ta.Type,
			Target:  ta.Target,
			Value:   ta.Value,
			Options: ta.Options,
		}
	}
	return actions
}

// executeAction dispatches to the appropriate handler
func (rae *RuleActionExecutor) executeAction(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	if err := rae.validateExtensionRequirement(ctx, action, ruleContext); err != nil {
		return err
	}
	handler, ok := rae.registry.Get(action.Type)
	if !ok {
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
	return handler.Handle(ctx, action, ruleContext, result)
}

func (rae *RuleActionExecutor) validateExtensionRequirement(ctx context.Context, action RuleAction, ruleContext *RuleContext) error {
	requiredExtension := automationdomain.RequiredExtensionForAction(action.Type)
	if requiredExtension == "" || rae == nil || rae.extensionChecker == nil {
		return nil
	}

	workspaceID, err := workspaceIDFromRuleContext(ruleContext)
	if err != nil {
		return err
	}
	enabled, err := rae.extensionChecker.HasActiveExtensionInWorkspace(ctx, workspaceID, requiredExtension)
	if err != nil {
		return fmt.Errorf("failed to resolve %s extension state: %w", requiredExtension, err)
	}
	if !enabled {
		return fmt.Errorf("%s action requires %s to be active in workspace", action.Type, requiredExtension)
	}
	return nil
}
