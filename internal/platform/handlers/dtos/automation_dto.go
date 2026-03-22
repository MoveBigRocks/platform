package dtos

import (
	"encoding/json"
	"time"
)

// RuleResponse represents a rule in API responses
type RuleResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`

	// Basic info
	Title         string `json:"title"`
	Description   string `json:"description,omitempty"`
	IsActive      bool   `json:"is_active"`
	IsSystem      bool   `json:"is_system"`
	SystemRuleKey string `json:"system_rule_key,omitempty"`
	Priority      int    `json:"priority"`

	// Rule configuration
	Conditions TypedConditionsResponse `json:"conditions"`
	Actions    TypedActionsResponse    `json:"actions"`

	// Execution control
	MuteFor              []string `json:"mute_for,omitempty"`
	MaxExecutionsPerDay  int      `json:"max_executions_per_day,omitempty"`
	MaxExecutionsPerHour int      `json:"max_executions_per_hour,omitempty"`

	// Scope
	TeamID     string   `json:"team_id,omitempty"`
	CaseTypes  []string `json:"case_types,omitempty"`
	Priorities []string `json:"priorities,omitempty"`

	// Execution tracking
	TotalExecutions int        `json:"total_executions"`
	LastExecutedAt  *time.Time `json:"last_executed_at,omitempty"`

	// Performance tracking
	AverageExecutionTime int64   `json:"average_execution_time"`
	SuccessRate          float64 `json:"success_rate"`

	// Metadata
	CreatedByID string    `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TypedConditionsResponse wraps conditions for API
type TypedConditionsResponse struct {
	Conditions []TypedConditionResponse `json:"conditions,omitempty"`
	Operator   string                   `json:"operator,omitempty"`
}

// TypedConditionResponse represents a rule condition for API
type TypedConditionResponse struct {
	Type     string            `json:"type"`
	Field    string            `json:"field,omitempty"`
	Operator string            `json:"operator"`
	Value    json.RawMessage   `json:"value"`
	Options  map[string]string `json:"options,omitempty"`
}

// TypedActionsResponse wraps actions for API
type TypedActionsResponse struct {
	Actions []TypedActionResponse `json:"actions,omitempty"`
}

// TypedActionResponse represents a rule action for API
type TypedActionResponse struct {
	Type    string            `json:"type"`
	Target  string            `json:"target,omitempty"`
	Value   json.RawMessage   `json:"value"`
	Field   string            `json:"field,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

// RuleConditionRequest represents a rule condition in API requests
type RuleConditionRequest struct {
	Type     string          `json:"type"`
	Field    string          `json:"field,omitempty"`
	Operator string          `json:"operator"`
	Value    json.RawMessage `json:"value"`
	Options  json.RawMessage `json:"options,omitempty"`
}

// RuleActionRequest represents a rule action in API requests
type RuleActionRequest struct {
	Type    string          `json:"type"`
	Target  string          `json:"target,omitempty"`
	Value   json.RawMessage `json:"value"`
	Field   string          `json:"field,omitempty"`
	Options json.RawMessage `json:"options,omitempty"`
}

// CreateRuleRequest represents request to create a rule
type CreateRuleRequest struct {
	Title                string                 `json:"title" binding:"required"`
	Description          string                 `json:"description"`
	WorkspaceID          string                 `json:"workspace_id" binding:"required"`
	IsActive             bool                   `json:"is_active"`
	Priority             int                    `json:"priority"`
	MaxExecutionsPerHour int                    `json:"max_executions_per_hour"`
	MaxExecutionsPerDay  int                    `json:"max_executions_per_day"`
	Conditions           []RuleConditionRequest `json:"conditions"`
	Actions              []RuleActionRequest    `json:"actions"`
}

// UpdateRuleRequest represents request to update a rule
type UpdateRuleRequest struct {
	Title                string                 `json:"title"`
	Description          string                 `json:"description"`
	IsActive             bool                   `json:"is_active"`
	Priority             int                    `json:"priority"`
	MaxExecutionsPerHour int                    `json:"max_executions_per_hour"`
	MaxExecutionsPerDay  int                    `json:"max_executions_per_day"`
	Conditions           []RuleConditionRequest `json:"conditions"`
	Actions              []RuleActionRequest    `json:"actions"`
}

// =============================================================================
// Workflow DTOs
// =============================================================================

// WorkflowTriggerRequest represents a workflow trigger in API requests
type WorkflowTriggerRequest struct {
	Type     string `json:"type"`
	Event    string `json:"event"`
	Schedule string `json:"schedule"`
}

// WorkflowStepRequest represents a workflow step in API requests
type WorkflowStepRequest struct {
	Name   string                 `json:"name"`
	Type   string                 `json:"type"`
	Order  int                    `json:"order"`
	Config map[string]interface{} `json:"config"`
}

// CreateWorkflowRequest represents request to create a workflow
type CreateWorkflowRequest struct {
	Name           string                   `json:"name" binding:"required"`
	Description    string                   `json:"description"`
	WorkspaceID    string                   `json:"workspace_id" binding:"required"`
	IsActive       bool                     `json:"is_active"`
	TimeoutMinutes int                      `json:"timeout_minutes"`
	Triggers       []WorkflowTriggerRequest `json:"triggers"`
	Steps          []WorkflowStepRequest    `json:"steps"`
}

// UpdateWorkflowRequest represents request to update a workflow
type UpdateWorkflowRequest struct {
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	IsActive       bool                     `json:"is_active"`
	TimeoutMinutes int                      `json:"timeout_minutes"`
	Triggers       []WorkflowTriggerRequest `json:"triggers"`
	Steps          []WorkflowStepRequest    `json:"steps"`
}

// =============================================================================
// Job DTOs
// =============================================================================

// CreateJobRequest represents request to create a job
type CreateJobRequest struct {
	Name            string                 `json:"name" binding:"required"`
	Description     string                 `json:"description"`
	Schedule        string                 `json:"schedule"`
	Type            string                 `json:"type" binding:"required"`
	ActionType      string                 `json:"action_type" binding:"required"`
	Configuration   map[string]interface{} `json:"configuration"`
	RetryCount      int                    `json:"retry_count"`
	TimeoutSeconds  int                    `json:"timeout_seconds"`
	NotifyOnFailure bool                   `json:"notify_on_failure"`
	WorkspaceID     string                 `json:"workspace_id" binding:"required"`
}

// UpdateJobRequest represents request to update a job
type UpdateJobRequest struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Schedule        string                 `json:"schedule"`
	Type            string                 `json:"type"`
	ActionType      string                 `json:"action_type"`
	Configuration   map[string]interface{} `json:"configuration"`
	RetryCount      int                    `json:"retry_count"`
	TimeoutSeconds  int                    `json:"timeout_seconds"`
	NotifyOnFailure bool                   `json:"notify_on_failure"`
	IsActive        bool                   `json:"is_active"`
}

// ToggleJobActiveRequest represents request to toggle job active status
type ToggleJobActiveRequest struct {
	Active bool `json:"active"`
}

// TriggerJobRequest represents request to manually trigger a job
type TriggerJobRequest struct {
	Parameters map[string]interface{} `json:"parameters"`
}
