package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

// TypedConditions wraps conditions with type-safe operator
type TypedConditions struct {
	Conditions []automationdomain.TypedCondition `json:"conditions,omitempty"`
	Operator   shareddomain.LogicalOperator      `json:"operator,omitempty"`
}

// TypedActions wraps actions array
type TypedActions struct {
	Actions []automationdomain.TypedAction `json:"actions,omitempty"`
}

// RuleStore implements shared.RuleStore using sqlx
type RuleStore struct {
	db *SqlxDB
}

// NewRuleStore creates a new rule store
func NewRuleStore(db *SqlxDB) *RuleStore {
	return &RuleStore{db: db}
}

// --- Rules ---

func (s *RuleStore) CreateRule(ctx context.Context, rule *automationdomain.Rule) error {
	normalizePersistedUUID(&rule.ID)
	conditions := TypedConditions{
		Conditions: rule.Conditions.Conditions,
		Operator:   rule.Conditions.Operator,
	}
	actions := TypedActions{
		Actions: rule.Actions.Actions,
	}

	conditionsJSON, err := json.Marshal(conditions)
	if err != nil {
		return err
	}
	actionsJSON, err := json.Marshal(actions)
	if err != nil {
		return err
	}
	muteFor, err := json.Marshal(rule.MuteFor)
	if err != nil {
		return err
	}
	caseTypes, err := json.Marshal(rule.CaseTypes)
	if err != nil {
		return err
	}
	priorities, err := json.Marshal(rule.Priorities)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO core_automation.rules (
			id, workspace_id, title, description, is_active, is_system, system_rule_key, priority,
			conditions, actions, mute_for, max_executions_per_day, max_executions_per_hour,
			team_id, case_types, priorities, total_executions, last_executed_at,
			average_execution_time, success_rate, created_by_id, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?
		)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		rule.ID, rule.WorkspaceID, rule.Title, rule.Description, rule.IsActive, rule.IsSystem, rule.SystemRuleKey, rule.Priority,
		string(conditionsJSON), string(actionsJSON), string(muteFor), rule.MaxExecutionsPerDay, rule.MaxExecutionsPerHour,
		nullableUUIDValue(rule.TeamID), string(caseTypes), string(priorities), rule.TotalExecutions, rule.LastExecutedAt,
		rule.AverageExecutionTime, rule.SuccessRate, nullableLegacyUUIDValue(rule.CreatedByID), rule.CreatedAt, rule.UpdatedAt,
	).Scan(&rule.ID)
	return TranslateSqlxError(err, "rules")
}

func (s *RuleStore) GetRule(ctx context.Context, ruleID string) (*automationdomain.Rule, error) {
	var m models.Rule
	query := `SELECT * FROM core_automation.rules WHERE id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &m, query, ruleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "rules")
	}
	return s.mapRuleToDomain(&m)
}

func (s *RuleStore) UpdateRule(ctx context.Context, rule *automationdomain.Rule) error {
	conditions := TypedConditions{
		Conditions: rule.Conditions.Conditions,
		Operator:   rule.Conditions.Operator,
	}
	actions := TypedActions{
		Actions: rule.Actions.Actions,
	}

	conditionsJSON, err := json.Marshal(conditions)
	if err != nil {
		return err
	}
	actionsJSON, err := json.Marshal(actions)
	if err != nil {
		return err
	}
	muteFor, err := json.Marshal(rule.MuteFor)
	if err != nil {
		return err
	}
	caseTypes, err := json.Marshal(rule.CaseTypes)
	if err != nil {
		return err
	}
	priorities, err := json.Marshal(rule.Priorities)
	if err != nil {
		return err
	}

	query := `
		UPDATE core_automation.rules SET
			title = ?, description = ?, is_active = ?, is_system = ?, system_rule_key = ?, priority = ?,
			conditions = ?, actions = ?, mute_for = ?, max_executions_per_day = ?, max_executions_per_hour = ?,
			team_id = ?, case_types = ?, priorities = ?, total_executions = ?, last_executed_at = ?,
			average_execution_time = ?, success_rate = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`

	res, err := s.db.Get(ctx).ExecContext(ctx, query,
		rule.Title, rule.Description, rule.IsActive, rule.IsSystem, rule.SystemRuleKey, rule.Priority,
		string(conditionsJSON), string(actionsJSON), string(muteFor), rule.MaxExecutionsPerDay, rule.MaxExecutionsPerHour,
		nullableUUIDValue(rule.TeamID), string(caseTypes), string(priorities), rule.TotalExecutions, rule.LastExecutedAt,
		rule.AverageExecutionTime, rule.SuccessRate, time.Now(),
		rule.ID, rule.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "rules")
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return TranslateSqlxError(err, "rules")
	}
	if rowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *RuleStore) ListWorkspaceRules(ctx context.Context, workspaceID string) ([]*automationdomain.Rule, error) {
	var rules []models.Rule
	query := `SELECT * FROM core_automation.rules WHERE workspace_id = ? AND deleted_at IS NULL ORDER BY priority ASC`
	if err := s.db.Get(ctx).SelectContext(ctx, &rules, query, workspaceID); err != nil {
		return nil, TranslateSqlxError(err, "rules")
	}

	result := make([]*automationdomain.Rule, len(rules))
	for i, r := range rules {
		domainR, err := s.mapRuleToDomain(&r)
		if err != nil {
			return nil, err
		}
		result[i] = domainR
	}
	return result, nil
}

func (s *RuleStore) ListActiveRules(ctx context.Context, workspaceID string) ([]*automationdomain.Rule, error) {
	var rules []models.Rule
	query := `SELECT * FROM core_automation.rules WHERE workspace_id = ? AND is_active = TRUE AND deleted_at IS NULL ORDER BY priority ASC`
	if err := s.db.Get(ctx).SelectContext(ctx, &rules, query, workspaceID); err != nil {
		return nil, TranslateSqlxError(err, "rules")
	}

	result := make([]*automationdomain.Rule, len(rules))
	for i, r := range rules {
		domainR, err := s.mapRuleToDomain(&r)
		if err != nil {
			return nil, err
		}
		result[i] = domainR
	}
	return result, nil
}

func (s *RuleStore) ListAllRules(ctx context.Context) ([]*automationdomain.Rule, error) {
	var rules []models.Rule
	query := `SELECT * FROM core_automation.rules WHERE deleted_at IS NULL ORDER BY workspace_id, priority ASC`
	if err := s.db.Get(ctx).SelectContext(ctx, &rules, query); err != nil {
		return nil, TranslateSqlxError(err, "rules")
	}

	result := make([]*automationdomain.Rule, len(rules))
	for i, r := range rules {
		domainR, err := s.mapRuleToDomain(&r)
		if err != nil {
			return nil, err
		}
		result[i] = domainR
	}
	return result, nil
}

func (s *RuleStore) DeleteRule(ctx context.Context, workspaceID, ruleID string) error {
	query := `UPDATE core_automation.rules SET deleted_at = ? WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`
	res, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now(), ruleID, workspaceID)
	if err != nil {
		return TranslateSqlxError(err, "rules")
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return TranslateSqlxError(err, "rules")
	}
	if rowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *RuleStore) mapRuleToDomain(m *models.Rule) (*automationdomain.Rule, error) {
	var conditions TypedConditions
	var actions TypedActions
	var muteFor []string
	var caseTypes []shareddomain.CaseChannel
	var priorities []shareddomain.CasePriority

	if m.Conditions != "" {
		if err := json.Unmarshal([]byte(m.Conditions), &conditions); err != nil {
			return nil, err
		}
	}
	if m.Actions != "" {
		if err := json.Unmarshal([]byte(m.Actions), &actions); err != nil {
			return nil, err
		}
	}
	if m.MuteFor != "" {
		if err := json.Unmarshal([]byte(m.MuteFor), &muteFor); err != nil {
			return nil, err
		}
	}
	if m.CaseTypes != "" {
		if err := json.Unmarshal([]byte(m.CaseTypes), &caseTypes); err != nil {
			return nil, err
		}
	}
	if m.Priorities != "" {
		if err := json.Unmarshal([]byte(m.Priorities), &priorities); err != nil {
			return nil, err
		}
	}

	domainConditions := automationdomain.TypedConditions{
		Conditions: conditions.Conditions,
		Operator:   conditions.Operator,
	}
	domainActions := automationdomain.TypedActions{
		Actions: actions.Actions,
	}

	return &automationdomain.Rule{
		ID:                   m.ID,
		WorkspaceID:          m.WorkspaceID,
		Title:                m.Title,
		Description:          m.Description,
		IsActive:             m.IsActive,
		IsSystem:             m.IsSystem,
		SystemRuleKey:        m.SystemRuleKey,
		Priority:             m.Priority,
		Conditions:           domainConditions,
		Actions:              domainActions,
		MuteFor:              muteFor,
		MaxExecutionsPerDay:  m.MaxExecutionsPerDay,
		MaxExecutionsPerHour: m.MaxExecutionsPerHour,
		TeamID:               derefStringPtr(m.TeamID),
		CaseTypes:            caseTypes,
		Priorities:           priorities,
		TotalExecutions:      m.TotalExecutions,
		LastExecutedAt:       m.LastExecutedAt,
		AverageExecutionTime: m.AverageExecutionTime,
		SuccessRate:          m.SuccessRate,
		CreatedByID:          derefStringPtr(m.CreatedByID),
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
	}, nil
}

// --- Rule Executions ---

func (s *RuleStore) CreateRuleExecution(ctx context.Context, execution *automationdomain.RuleExecution) error {
	normalizePersistedUUID(&execution.ID)
	contextJSON, err := json.Marshal(execution.Context)
	if err != nil {
		return err
	}
	actionsExecuted, err := json.Marshal(execution.ActionsExecuted)
	if err != nil {
		return err
	}
	changes, err := json.Marshal(execution.Changes)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO core_automation.rule_executions (
			id, workspace_id, rule_id, case_id, trigger_type, context, status,
			started_at, completed_at, execution_time, actions_executed, changes,
			error_message, created_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?
		)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		execution.ID, execution.WorkspaceID, execution.RuleID, nullableUUIDValue(execution.CaseID),
		string(execution.TriggerType), string(contextJSON), string(execution.Status),
		execution.StartedAt, execution.CompletedAt, execution.ExecutionTime,
		string(actionsExecuted), string(changes), execution.ErrorMessage, execution.CreatedAt,
	).Scan(&execution.ID)
	return TranslateSqlxError(err, "rule_executions")
}

func (s *RuleStore) ListRuleExecutions(ctx context.Context, ruleID string) ([]*automationdomain.RuleExecution, error) {
	var executions []models.RuleExecution
	query := `SELECT * FROM core_automation.rule_executions WHERE rule_id = ? ORDER BY created_at DESC`
	if err := s.db.Get(ctx).SelectContext(ctx, &executions, query, ruleID); err != nil {
		return nil, TranslateSqlxError(err, "rule_executions")
	}

	result := make([]*automationdomain.RuleExecution, len(executions))
	for i, e := range executions {
		domainE, err := s.mapExecutionToDomain(&e)
		if err != nil {
			return nil, err
		}
		result[i] = domainE
	}
	return result, nil
}

func (s *RuleStore) UpdateRuleExecution(ctx context.Context, execution *automationdomain.RuleExecution) error {
	actionsExecuted, err := json.Marshal(execution.ActionsExecuted)
	if err != nil {
		return err
	}
	changes, err := json.Marshal(execution.Changes)
	if err != nil {
		return err
	}

	query := `
		UPDATE core_automation.rule_executions SET
			status = ?,
			completed_at = ?,
			execution_time = ?,
			actions_executed = ?,
			changes = ?,
			error_message = ?
		WHERE id = ?`

	_, err = s.db.Get(ctx).ExecContext(ctx, query,
		string(execution.Status), execution.CompletedAt, execution.ExecutionTime,
		string(actionsExecuted), string(changes), execution.ErrorMessage,
		execution.ID,
	)
	return TranslateSqlxError(err, "rule_executions")
}

func (s *RuleStore) IncrementRuleStats(ctx context.Context, workspaceID, ruleID string, success bool, executedAt time.Time) error {
	successIncrement := 0.0
	if success {
		successIncrement = 1.0
	}
	updatedAt := time.Now().UTC()

	query := `
		UPDATE core_automation.rules
		SET
			total_executions = total_executions + 1,
			success_rate = CASE
			WHEN total_executions = 0 THEN ?
				ELSE (success_rate * total_executions + ?) / (total_executions + 1)
			END,
			last_executed_at = ?,
			updated_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`

	_, err := s.db.Get(ctx).ExecContext(ctx, query,
		successIncrement, successIncrement, executedAt, updatedAt, ruleID, workspaceID,
	)
	return TranslateSqlxError(err, "rules")
}

func (s *RuleStore) mapExecutionToDomain(m *models.RuleExecution) (*automationdomain.RuleExecution, error) {
	var execContext shareddomain.TypedContext
	var actionsExecuted []shareddomain.RuleActionType
	var changes shareddomain.ChangeSet

	if m.Context != "" {
		if err := json.Unmarshal([]byte(m.Context), &execContext); err != nil {
			return nil, err
		}
	}
	if m.ActionsExecuted != "" {
		if err := json.Unmarshal([]byte(m.ActionsExecuted), &actionsExecuted); err != nil {
			return nil, err
		}
	}
	if m.Changes != "" {
		if err := json.Unmarshal([]byte(m.Changes), &changes); err != nil {
			return nil, err
		}
	}

	return &automationdomain.RuleExecution{
		ID:              m.ID,
		WorkspaceID:     m.WorkspaceID,
		RuleID:          m.RuleID,
		CaseID:          derefStringPtr(m.CaseID),
		TriggerType:     shareddomain.TriggerType(m.TriggerType),
		Context:         execContext,
		Status:          automationdomain.ExecutionStatus(m.Status),
		StartedAt:       m.StartedAt,
		CompletedAt:     m.CompletedAt,
		ExecutionTime:   m.ExecutionTime,
		ActionsExecuted: actionsExecuted,
		Changes:         &changes,
		ErrorMessage:    m.ErrorMessage,
		CreatedAt:       m.CreatedAt,
	}, nil
}
