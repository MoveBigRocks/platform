package automationservices

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/graph/model"
	automationdomain "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
)

// RuleService handles CRUD operations for automation rules
type RuleService struct {
	ruleStore shared.RuleStore
}

// NewRuleService creates a new RuleService
func NewRuleService(ruleStore shared.RuleStore) *RuleService {
	return &RuleService{ruleStore: ruleStore}
}

// CreateRuleParams contains parameters for creating a rule
type CreateRuleParams struct {
	WorkspaceID          string
	Title                string
	Description          string
	IsActive             bool
	Priority             int
	MaxExecutionsPerHour int
	MaxExecutionsPerDay  int
	Conditions           []automationdomain.RuleCondition
	Actions              []automationdomain.RuleAction
	CreatedByID          string
}

// Validate validates the create rule parameters
func (p *CreateRuleParams) Validate() error {
	if p.Title == "" {
		return fmt.Errorf("Title is required")
	}
	if p.WorkspaceID == "" {
		return fmt.Errorf("Workspace ID is required")
	}
	return nil
}

// CreateRule creates a new automation rule
func (s *RuleService) CreateRule(ctx context.Context, params CreateRuleParams) (*automationdomain.Rule, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	now := time.Now()
	rule := &automationdomain.Rule{
		WorkspaceID: params.WorkspaceID,
		Title:       params.Title,
		Description: params.Description,
		IsActive:    params.IsActive,
		Priority:    params.Priority,
		Conditions: automationdomain.TypedConditions{
			Operator:   "and",
			Conditions: params.Conditions,
		},
		Actions: automationdomain.TypedActions{
			Actions: params.Actions,
		},
		MaxExecutionsPerHour: params.MaxExecutionsPerHour,
		MaxExecutionsPerDay:  params.MaxExecutionsPerDay,
		CreatedByID:          params.CreatedByID,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := s.ruleStore.CreateRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("create rule: %w", err)
	}

	return rule, nil
}

// UpdateRuleParams contains parameters for updating a rule
type UpdateRuleParams struct {
	Title                string
	Description          string
	IsActive             bool
	Priority             int
	MaxExecutionsPerHour int
	MaxExecutionsPerDay  int
	Conditions           []automationdomain.RuleCondition
	Actions              []automationdomain.RuleAction
}

// UpdateRule updates an existing rule
func (s *RuleService) UpdateRule(ctx context.Context, ruleID string, params UpdateRuleParams) (*automationdomain.Rule, error) {
	rule, err := s.ruleStore.GetRule(ctx, ruleID)
	if err != nil {
		return nil, fmt.Errorf("get rule: %w", err)
	}

	rule.Title = params.Title
	rule.Description = params.Description
	rule.IsActive = params.IsActive
	rule.Priority = params.Priority
	rule.MaxExecutionsPerHour = params.MaxExecutionsPerHour
	rule.MaxExecutionsPerDay = params.MaxExecutionsPerDay
	rule.Conditions = automationdomain.TypedConditions{
		Operator:   "and",
		Conditions: params.Conditions,
	}
	rule.Actions = automationdomain.TypedActions{
		Actions: params.Actions,
	}
	rule.UpdatedAt = time.Now()

	if err := s.ruleStore.UpdateRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("update rule: %w", err)
	}

	return rule, nil
}

// GetRule retrieves a rule by ID
func (s *RuleService) GetRule(ctx context.Context, ruleID string) (*automationdomain.Rule, error) {
	rule, err := s.ruleStore.GetRule(ctx, ruleID)
	if err != nil {
		return nil, fmt.Errorf("get rule: %w", err)
	}
	return rule, nil
}

// DeleteRule deletes a rule by ID
func (s *RuleService) DeleteRule(ctx context.Context, ruleID string) error {
	// Fetch rule to get its workspaceID for scoped deletion
	rule, err := s.ruleStore.GetRule(ctx, ruleID)
	if err != nil {
		return fmt.Errorf("get rule for delete: %w", err)
	}

	if err := s.ruleStore.DeleteRule(ctx, rule.WorkspaceID, ruleID); err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}
	return nil
}

// ListWorkspaceRules lists all rules for a workspace
func (s *RuleService) ListWorkspaceRules(ctx context.Context, workspaceID string) ([]*automationdomain.Rule, error) {
	rules, err := s.ruleStore.ListWorkspaceRules(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	return rules, nil
}

// ListActiveRules lists all active rules for a workspace
func (s *RuleService) ListActiveRules(ctx context.Context, workspaceID string) ([]*automationdomain.Rule, error) {
	rules, err := s.ruleStore.ListActiveRules(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list active rules: %w", err)
	}
	return rules, nil
}

// ToggleRule enables or disables a rule
func (s *RuleService) ToggleRule(ctx context.Context, ruleID string, isActive bool) error {
	rule, err := s.ruleStore.GetRule(ctx, ruleID)
	if err != nil {
		return fmt.Errorf("get rule: %w", err)
	}

	rule.IsActive = isActive
	rule.UpdatedAt = time.Now()

	if err := s.ruleStore.UpdateRule(ctx, rule); err != nil {
		return fmt.Errorf("update rule: %w", err)
	}

	return nil
}

// ListAllRules lists all rules across workspaces (requires admin context)
func (s *RuleService) ListAllRules(ctx context.Context) ([]*automationdomain.Rule, error) {
	rules, err := s.ruleStore.ListAllRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all rules: %w", err)
	}
	return rules, nil
}

// ListAllRulesFiltered lists admin rules with GraphQL filter translation kept out of resolvers.
func (s *RuleService) ListAllRulesFiltered(ctx context.Context, filter *model.AdminRuleFilterInput) ([]*automationdomain.Rule, error) {
	rules, err := s.ListAllRules(ctx)
	if err != nil {
		return nil, err
	}
	if filter == nil {
		return rules, nil
	}

	result := make([]*automationdomain.Rule, 0, len(rules))
	for _, rule := range rules {
		if filter.WorkspaceID != nil && rule.WorkspaceID != *filter.WorkspaceID {
			continue
		}
		if filter.IsActive != nil && rule.IsActive != *filter.IsActive {
			continue
		}
		result = append(result, rule)
	}
	if filter.First != nil && len(result) > int(*filter.First) {
		result = result[:int(*filter.First)]
	}
	return result, nil
}

// CreateRuleExecution records a rule execution for auditing/logging purposes
func (s *RuleService) CreateRuleExecution(ctx context.Context, execution *automationdomain.RuleExecution) error {
	if err := s.ruleStore.CreateRuleExecution(ctx, execution); err != nil {
		return fmt.Errorf("create rule execution: %w", err)
	}
	return nil
}

// UpdateRuleExecution updates an existing rule execution record with final status
func (s *RuleService) UpdateRuleExecution(ctx context.Context, execution *automationdomain.RuleExecution) error {
	if err := s.ruleStore.UpdateRuleExecution(ctx, execution); err != nil {
		return fmt.Errorf("update rule execution: %w", err)
	}
	return nil
}
