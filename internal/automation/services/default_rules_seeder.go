package automationservices

import (
	"context"
	"fmt"
	"time"

	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// DefaultRulesSeeder creates default automation rules for new workspaces
type DefaultRulesSeeder struct {
	rules  shared.RuleStore
	logger *logger.Logger
}

// toMetadata creates a shareddomain.Metadata from variadic key-value pairs.
// Uses shareddomain.ValueFromInterface for type conversion.
func toMetadata(kvPairs ...interface{}) shareddomain.Metadata {
	m := shareddomain.NewMetadata()
	for i := 0; i < len(kvPairs)-1; i += 2 {
		key, ok := kvPairs[i].(string)
		if !ok {
			continue
		}
		m.Set(key, shareddomain.ValueFromInterface(kvPairs[i+1]))
	}
	return m
}

// NewDefaultRulesSeeder creates a new default rules seeder
func NewDefaultRulesSeeder(rules shared.RuleStore) *DefaultRulesSeeder {
	return &DefaultRulesSeeder{
		rules:  rules,
		logger: logger.New().WithField("service", "default-rules-seeder"),
	}
}

// SystemRuleKey constants for identifying system rules
const (
	SystemRuleKeyCaseCreatedReceipt        = "case_created_receipt"
	SystemRuleKeyFirstResponseOpen         = "first_response_open"
	SystemRuleKeyPendingReminder           = "pending_reminder"
	SystemRuleKeyAutoCloseResolved         = "auto_close_resolved"
	SystemRuleKeyNoResponseAlert           = "no_response_alert"
	SystemRuleKeyCustomerReplyReopen       = "customer_reply_reopen"
	SystemRuleKeyCustomerReplyReopenClosed = "customer_reply_reopen_closed"
)

// SeedDefaultRules creates default automation rules for a workspace
// Returns the created rules or error if seeding fails
func (s *DefaultRulesSeeder) SeedDefaultRules(ctx context.Context, workspaceID string) ([]*automationdomain.Rule, error) {
	// Check if rules already exist for this workspace (idempotent)
	existingRules, err := s.rules.ListWorkspaceRules(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing rules: %w", err)
	}

	// Build map of existing system rules by key
	existingSystemKeys := make(map[string]bool)
	for _, rule := range existingRules {
		if rule.IsSystem && rule.SystemRuleKey != "" {
			existingSystemKeys[rule.SystemRuleKey] = true
		}
	}

	// Define default rules
	defaultRules := s.getDefaultRules(workspaceID)

	var createdRules []*automationdomain.Rule
	for _, rule := range defaultRules {
		// Skip if this system rule already exists
		if existingSystemKeys[rule.SystemRuleKey] {
			s.logger.Debug("system rule already exists, skipping", "rule_key", rule.SystemRuleKey, "workspace_id", workspaceID)
			continue
		}

		if err := s.rules.CreateRule(ctx, rule); err != nil {
			s.logger.Error("failed to create default rule", "rule_title", rule.Title, "error", err)
			continue
		}

		s.logger.Info("created default rule", "rule_title", rule.Title, "workspace_id", workspaceID)
		createdRules = append(createdRules, rule)
	}

	return createdRules, nil
}

// getDefaultRules returns the default rules for a workspace
func (s *DefaultRulesSeeder) getDefaultRules(workspaceID string) []*automationdomain.Rule {
	now := time.Now()

	return []*automationdomain.Rule{
		// 1. New case receipt - Send acknowledgment email when case is created
		{
			WorkspaceID:   workspaceID,
			Title:         "New Case Acknowledgment",
			Description:   "Send an acknowledgment email to the customer when a new case is created",
			IsActive:      true,
			IsSystem:      true,
			SystemRuleKey: SystemRuleKeyCaseCreatedReceipt,
			Priority:      1,
			Conditions: automationdomain.RuleConditionsData{
				Operator: "and",
				Conditions: []automationdomain.RuleCondition{
					{Type: "event", Field: "trigger", Operator: "equals", Value: shareddomain.ValueFromInterface("case_created")},
					{Type: "field", Field: "contact_email", Operator: "is_not_empty"},
				},
			},
			Actions: automationdomain.RuleActionsData{
				Actions: []automationdomain.RuleAction{
					{
						Type: "send_email",
						Options: toMetadata(
							"template", "case_acknowledgment",
							"to", "{{contact_email}}",
							"subject", "We received your request: {{subject}}",
							"body", "Thank you for contacting us. We have received your request and will respond as soon as possible.\n\nCase Reference: {{human_id}}\nSubject: {{subject}}\n\nOur team will review your request and get back to you shortly.",
						),
					},
				},
			},
			CreatedByID: "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// 2. First response → Pending - Change status when agent first replies
		{
			WorkspaceID:   workspaceID,
			Title:         "First Response - Set Pending",
			Description:   "Automatically change case status to 'pending' when an agent sends the first response (awaiting customer reply)",
			IsActive:      true,
			IsSystem:      true,
			SystemRuleKey: SystemRuleKeyFirstResponseOpen,
			Priority:      1,
			Conditions: automationdomain.RuleConditionsData{
				Operator: "and",
				Conditions: []automationdomain.RuleCondition{
					{Type: "event", Field: "trigger", Operator: "equals", Value: shareddomain.ValueFromInterface("agent_reply")},
					{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.ValueFromInterface(string(servicedomain.CaseStatusNew))},
				},
			},
			Actions: automationdomain.RuleActionsData{
				Actions: []automationdomain.RuleAction{
					{Type: "change_status", Value: shareddomain.ValueFromInterface(string(servicedomain.CaseStatusPending))},
				},
			},
			CreatedByID: "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// 3. Customer reply to pending/resolved → Set Open
		{
			WorkspaceID:   workspaceID,
			Title:         "Customer Reply - Set Open",
			Description:   "Automatically set case to 'open' when customer replies to a pending or resolved case",
			IsActive:      true,
			IsSystem:      true,
			SystemRuleKey: SystemRuleKeyCustomerReplyReopen,
			Priority:      1,
			Conditions: automationdomain.RuleConditionsData{
				Operator: "and",
				Conditions: []automationdomain.RuleCondition{
					{Type: "event", Field: "trigger", Operator: "equals", Value: shareddomain.ValueFromInterface("customer_reply")},
					{Type: "field", Field: "status", Operator: "in", Value: shareddomain.ValueFromInterface([]string{string(servicedomain.CaseStatusResolved), string(servicedomain.CaseStatusPending)})},
				},
			},
			Actions: automationdomain.RuleActionsData{
				Actions: []automationdomain.RuleAction{
					{Type: "change_status", Value: shareddomain.ValueFromInterface(string(servicedomain.CaseStatusOpen))},
				},
			},
			CreatedByID: "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// 3b. Customer reply to closed → Reopen case with tag
		{
			WorkspaceID:   workspaceID,
			Title:         "Customer Reply - Reopen Closed Case",
			Description:   "Automatically reopen a closed case when the customer replies, adding 'reopened' tag",
			IsActive:      true,
			IsSystem:      true,
			SystemRuleKey: SystemRuleKeyCustomerReplyReopenClosed,
			Priority:      1,
			Conditions: automationdomain.RuleConditionsData{
				Operator: "and",
				Conditions: []automationdomain.RuleCondition{
					{Type: "event", Field: "trigger", Operator: "equals", Value: shareddomain.ValueFromInterface("customer_reply")},
					{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.ValueFromInterface(string(servicedomain.CaseStatusClosed))},
				},
			},
			Actions: automationdomain.RuleActionsData{
				Actions: []automationdomain.RuleAction{
					{Type: "change_status", Value: shareddomain.ValueFromInterface(string(servicedomain.CaseStatusOpen))},
					{Type: "add_tags", Value: shareddomain.ValueFromInterface("reopened")},
					{Type: "add_communication", Value: shareddomain.ValueFromInterface("Case automatically reopened due to customer reply")},
				},
			},
			CreatedByID: "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// 4. Pending reminder - Remind customer after 3 days
		{
			WorkspaceID:   workspaceID,
			Title:         "Pending Case Reminder",
			Description:   "Send a reminder email to customer if case has been pending for 3 days",
			IsActive:      true,
			IsSystem:      true,
			SystemRuleKey: SystemRuleKeyPendingReminder,
			Priority:      2,
			Conditions: automationdomain.RuleConditionsData{
				Operator: "and",
				Conditions: []automationdomain.RuleCondition{
					{Type: "event", Field: "trigger", Operator: "equals", Value: shareddomain.ValueFromInterface("scheduled")},
					{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.ValueFromInterface(string(servicedomain.CaseStatusPending))},
					{Type: "time", Field: "status_changed_at", Operator: "older_than", Value: shareddomain.ValueFromInterface("3d")},
				},
			},
			Actions: automationdomain.RuleActionsData{
				Actions: []automationdomain.RuleAction{
					{
						Type: "send_email",
						Options: toMetadata(
							"template", "pending_reminder",
							"to", "{{contact_email}}",
							"subject", "Reminder: We're waiting for your response - {{subject}}",
							"body", "Hi,\n\nWe're following up on your support request. We're waiting for additional information from you to proceed.\n\nCase Reference: {{human_id}}\nSubject: {{subject}}\n\nPlease reply to this email with the requested information. If we don't hear back within 4 more days, we'll assume the issue is resolved and close the case.\n\nThank you!",
						),
					},
					{Type: "add_tags", Value: shareddomain.ValueFromInterface("reminder-sent")},
				},
			},
			CreatedByID: "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// 5. Auto-close resolved cases - Close after 7 days
		{
			WorkspaceID:   workspaceID,
			Title:         "Auto-Close Resolved Cases",
			Description:   "Automatically close resolved cases after 7 days of inactivity",
			IsActive:      true,
			IsSystem:      true,
			SystemRuleKey: SystemRuleKeyAutoCloseResolved,
			Priority:      3,
			Conditions: automationdomain.RuleConditionsData{
				Operator: "and",
				Conditions: []automationdomain.RuleCondition{
					{Type: "event", Field: "trigger", Operator: "equals", Value: shareddomain.ValueFromInterface("scheduled")},
					{Type: "field", Field: "status", Operator: "equals", Value: shareddomain.ValueFromInterface(string(servicedomain.CaseStatusResolved))},
					{Type: "time", Field: "resolved_at", Operator: "older_than", Value: shareddomain.ValueFromInterface("7d")},
				},
			},
			Actions: automationdomain.RuleActionsData{
				Actions: []automationdomain.RuleAction{
					{Type: "close_case", Value: shareddomain.ValueFromInterface(true)},
					{Type: "add_communication", Value: shareddomain.ValueFromInterface("Case automatically closed after 7 days in resolved status with no activity")},
				},
			},
			CreatedByID: "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// 6. No response alert - Notify team when case has no response for 24h
		{
			WorkspaceID:   workspaceID,
			Title:         "No Response Alert (24h)",
			Description:   "Notify workspace members and admins when a case hasn't received a response within 24 hours",
			IsActive:      true,
			IsSystem:      true,
			SystemRuleKey: SystemRuleKeyNoResponseAlert,
			Priority:      1,
			Conditions: automationdomain.RuleConditionsData{
				Operator: "and",
				Conditions: []automationdomain.RuleCondition{
					{Type: "event", Field: "trigger", Operator: "equals", Value: shareddomain.ValueFromInterface("scheduled")},
					{Type: "field", Field: "status", Operator: "in", Value: shareddomain.ValueFromInterface([]string{string(servicedomain.CaseStatusNew), string(servicedomain.CaseStatusOpen)})},
					{Type: "field", Field: "first_response_at", Operator: "is_empty"},
					{Type: "time", Field: "created_at", Operator: "older_than", Value: shareddomain.ValueFromInterface("24h")},
				},
			},
			Actions: automationdomain.RuleActionsData{
				Actions: []automationdomain.RuleAction{
					{
						Type: "send_email",
						Options: toMetadata(
							"to", "{{workspace_members}}",
							"subject", "Alert: Case awaiting response for 24+ hours - {{subject}}",
							"body", "A support case has been waiting for a response for more than 24 hours.\n\nCase Reference: {{human_id}}\nSubject: {{subject}}\nPriority: {{priority}}\nCreated: {{created_at}}\n\nPlease review and respond to this case as soon as possible.",
						),
					},
					{Type: "add_tags", Value: shareddomain.ValueFromInterface("needs-attention")},
					{Type: "add_communication", Value: shareddomain.ValueFromInterface("Alert: Case has been awaiting response for more than 24 hours. Team notified.")},
				},
			},
			CreatedByID: "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}
