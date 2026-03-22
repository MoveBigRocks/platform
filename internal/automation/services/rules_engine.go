package automationservices

import (
	"context"
	"errors"
	"fmt"
	"time"

	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
	"github.com/movebigrocks/platform/pkg/logger"
)

// RulesEngine manages rule evaluation and execution
//
// Architecture: The rules engine now uses services instead of stores directly:
// - RuleService for fetching rules
// - ContactService for contact lookup
// - CaseService (via action executor) for case mutations
//
// This ensures all operations go through the service layer for proper validation
// and event publishing.
type RulesEngine struct {
	ruleService    *RuleService
	contactService contracts.ContactServiceInterface
	outbox         contracts.OutboxPublisher
	logger         *logger.Logger

	conditionEvaluator *RuleConditionEvaluator
	actionExecutor     *RuleActionExecutor
	rateLimiter        *RuleRateLimiter

	// cancelCleanup stops the rate limiter cleanup worker
	cancelCleanup func()
}

// RuleCondition is the service-layer alias for the domain type.
type RuleCondition = automationdomain.RuleCondition

// RuleAction is the service-layer alias for the domain type.
type RuleAction = automationdomain.RuleAction

// NewRulesEngine creates a new rules engine.
// The engine starts a background cleanup worker for stale rate limiter stats.
// Call Stop() when shutting down to clean up resources.
//
// Architecture:
//   - ruleService: Used to fetch rules (service layer)
//   - caseService: Passed to action executor for case mutations (service layer)
//   - contactService: Used to get contact context for rule evaluation
//   - ruleStore: Used internally by rate limiter for stats persistence
//   - outbox: Used for event publishing
func NewRulesEngine(
	ruleService *RuleService,
	caseService contracts.CaseServiceInterface,
	contactService contracts.ContactServiceInterface,
	ruleStore shared.RuleStore, // Only for rate limiter stats persistence
	outbox contracts.OutboxPublisher,
) *RulesEngine {
	var rateLimiter *RuleRateLimiter
	if ruleStore != nil {
		rateLimiter = NewRuleRateLimiter(ruleStore)
	} else {
		rateLimiter = NewRuleRateLimiter(nil)
	}

	// Start cleanup worker: runs every hour, cleans rules inactive for 7 days
	cancelCleanup := rateLimiter.StartCleanupWorker(time.Hour, 7*24*time.Hour)

	return &RulesEngine{
		ruleService:        ruleService,
		contactService:     contactService,
		outbox:             outbox,
		logger:             logger.New().WithField("service", "rules_engine"),
		conditionEvaluator: NewRuleConditionEvaluator(),
		actionExecutor:     NewRuleActionExecutor(caseService, outbox),
		rateLimiter:        rateLimiter,
		cancelCleanup:      cancelCleanup,
	}
}

func (re *RulesEngine) SetExtensionChecker(checker ActionExtensionChecker) {
	if re == nil || re.actionExecutor == nil {
		return
	}
	re.actionExecutor.extensionChecker = checker
}

// Stop shuts down the rules engine and releases resources.
// This should be called during graceful shutdown.
func (re *RulesEngine) Stop() {
	if re.cancelCleanup != nil {
		re.cancelCleanup()
	}
}

// EvaluateRulesForCase evaluates all active rules for a case.
//
// Deprecated: Use EvaluateRulesForCaseTyped with *FieldChanges for compile-time type safety.
// This method will be removed in a future version.
func (re *RulesEngine) EvaluateRulesForCase(ctx context.Context, caseObj *servicedomain.Case, event string, changes map[string]interface{}) error {
	// Convert changes to typed FieldChanges
	var fc *FieldChanges
	if changes != nil {
		fc = NewFieldChanges()
		for k, v := range changes {
			fc.Set(k, v)
		}
	}

	return re.EvaluateRulesForCaseTyped(ctx, caseObj, event, fc)
}

// EvaluateRulesForCaseTyped evaluates all active rules for a case with type-safe changes.
// This is the preferred method for rule evaluation - provides compile-time type safety.
func (re *RulesEngine) EvaluateRulesForCaseTyped(ctx context.Context, caseObj *servicedomain.Case, event string, changes *FieldChanges) error {
	// Use RuleService to fetch active rules
	rules, err := re.ruleService.ListActiveRules(ctx, caseObj.WorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to get active rules: %w", err)
	}

	// Use ContactService to get contact context
	var contact *platformdomain.Contact
	if caseObj.ContactID != "" && caseObj.WorkspaceID != "" {
		contact, err = re.contactService.GetContact(ctx, caseObj.WorkspaceID, caseObj.ContactID)
		if err != nil {
			re.logger.WithError(err).Warn("Failed to load contact for rule evaluation", "contact_id", caseObj.ContactID)
		}
	}

	// Use provided changes or create empty
	fc := changes
	if fc == nil {
		fc = NewFieldChanges()
	}

	ruleContext := &RuleContext{
		Case:     caseObj,
		Contact:  contact,
		Event:    event,
		Changes:  fc,
		Metadata: NewRuleMetadata(),
	}

	var errs []error
	for _, rule := range rules {
		if err := re.executeRule(ctx, rule, ruleContext); err != nil {
			re.logger.WithError(err).Error("Rule evaluation failed", "rule_id", rule.ID)
			errs = append(errs, fmt.Errorf("rule %s: %w", rule.ID, err))
		}
	}

	return errors.Join(errs...)
}

// EvaluateRulesForForm evaluates all active rules for a submitted form event.
func (re *RulesEngine) EvaluateRulesForForm(ctx context.Context, formEvent *contracts.FormSubmittedEvent, event string) error {
	if formEvent == nil {
		return fmt.Errorf("form event is nil")
	}
	if formEvent.WorkspaceID == "" {
		return fmt.Errorf("form event workspace_id is required")
	}

	rules, err := re.ruleService.ListActiveRules(ctx, formEvent.WorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to get active rules: %w", err)
	}

	var contact *platformdomain.Contact
	if re.contactService != nil && formEvent.SubmitterEmail != "" {
		contact, err = re.contactService.GetContactByEmail(ctx, formEvent.WorkspaceID, formEvent.SubmitterEmail)
		if err != nil {
			re.logger.WithError(err).Warn("Failed to load contact for form rule evaluation", "submitter_email", formEvent.SubmitterEmail)
		}
	}

	ruleContext := &RuleContext{
		FormSubmission: formEvent,
		Contact:        contact,
		Event:          event,
		Changes:        NewFieldChanges(),
		Metadata:       NewRuleMetadata(),
	}

	var errs []error
	for _, rule := range rules {
		if err := re.executeRule(ctx, rule, ruleContext); err != nil {
			re.logger.WithError(err).Error("Form rule evaluation failed", "rule_id", rule.ID, "submission_id", formEvent.SubmissionID)
			errs = append(errs, fmt.Errorf("rule %s: %w", rule.ID, err))
		}
	}

	return errors.Join(errs...)
}

// executeRule is the unified method for evaluating and executing a single rule
func (re *RulesEngine) executeRule(ctx context.Context, rule *automationdomain.Rule, ruleContext *RuleContext) error {
	ruleContext.RuleID = rule.ID
	targetID := ruleContext.TargetID()
	targetType := ruleContext.TargetType()

	log := re.logger.WithFields(map[string]interface{}{
		"rule_id":     rule.ID,
		"target_type": targetType,
		"target_id":   targetID,
	})

	// Check rate limiting
	if !re.rateLimiter.CanExecuteRule(rule) {
		log.Debug("Rule rate limited")
		return nil
	}

	// Check if case is muted for this rule (only applies to case-based rules)
	if ruleContext.Case != nil && re.rateLimiter.IsCaseMuted(rule, ruleContext.Case.ID) {
		log.Debug("Case is muted for rule")
		return nil
	}

	// Evaluate conditions
	conditionsMatch, err := re.conditionEvaluator.EvaluateConditions(rule.Conditions, ruleContext)
	if err != nil {
		return fmt.Errorf("failed to evaluate conditions: %w", err)
	}

	if !conditionsMatch {
		log.Debug("Rule conditions did not match")
		return nil
	}

	log.Info("Rule matched, executing actions")

	// Build typed context
	typedContext := shareddomain.NewTypedContext()
	typedContext.EventType = ruleContext.Event
	// Copy metadata to Extra using type-safe merge
	if ruleContext.Metadata != nil {
		typedContext.Extra.Merge(ruleContext.Metadata.ToMetadata())
	}

	// Build execution record
	execution := &automationdomain.RuleExecution{
		ID:          generateRuleID(),
		WorkspaceID: rule.WorkspaceID,
		RuleID:      rule.ID,
		TriggerType: shareddomain.TriggerType(ruleContext.Event),
		Context:     typedContext,
		Status:      "running",
		StartedAt:   time.Now(),
	}

	// Set target-specific fields in execution
	if ruleContext.Case != nil {
		execution.CaseID = ruleContext.Case.ID
		typedContext.CaseID = ruleContext.Case.ID
	}
	if ruleContext.Issue != nil {
		typedContext.Extra.SetString("issue_id", ruleContext.Issue.ID)
	}
	if ruleContext.FormSubmission != nil {
		typedContext.Extra.SetString("form_id", ruleContext.FormSubmission.FormID)
		typedContext.Extra.SetString("submission_id", ruleContext.FormSubmission.SubmissionID)
	}

	if err := re.ruleService.CreateRuleExecution(ctx, execution); err != nil {
		log.WithError(err).Warn("Failed to create rule execution record")
		return fmt.Errorf("failed to create rule execution audit record: %w", err)
	}

	// Execute actions
	actionResults, actionErr := re.actionExecutor.ExecuteActions(ctx, rule.Actions, ruleContext)
	if actionErr != nil {
		execution.Status = "failed"
		execution.ErrorMessage = actionErr.Error()
		log.WithError(actionErr).Error("Rule execution failed")
	} else {
		execution.Status = "success"
		// Convert string slice to RuleActionType slice
		for _, actionType := range actionResults.ExecutedActions {
			execution.ActionsExecuted = append(execution.ActionsExecuted, shareddomain.RuleActionType(actionType))
		}
		// Convert changes to ChangeSet using type-safe access
		if actionResults.Changes != nil {
			execution.Changes = actionResults.Changes.ToChangeSet()
		}
		log.Info("Rule executed successfully")
	}

	now := time.Now()
	execution.CompletedAt = &now
	execution.ExecutionTime = now.Sub(execution.StartedAt).Milliseconds()

	if err := re.ruleService.UpdateRuleExecution(ctx, execution); err != nil {
		log.WithError(err).Warn("Failed to update rule execution record")
		return fmt.Errorf("failed to update rule execution audit record: %w", err)
	}

	log.Debug("Rule execution completed", "execution_id", execution.ID)
	re.rateLimiter.UpdateRuleStats(ctx, rule, execution.Status == "success")

	return actionErr
}

// ActionResult represents the result of executing actions
type ActionResult struct {
	ExecutedActions []string
	Changes         *ActionChanges // Type-safe changes
	Errors          []string
}

func generateRuleID() string {
	return id.NewPublicID()
}
