package models

import (
	"time"
)

// Rule represents an automation rule
type Rule struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Basic info
	Title         string `db:"title"`
	Description   string `db:"description"`
	IsActive      bool   `db:"is_active"`
	IsSystem      bool   `db:"is_system"`
	SystemRuleKey string `db:"system_rule_key"`
	Priority      int    `db:"priority"`

	// Rule configuration
	Conditions string `db:"conditions"`
	Actions    string `db:"actions"`

	// Execution control
	MuteFor              string `db:"mute_for"`
	MaxExecutionsPerDay  int    `db:"max_executions_per_day"`
	MaxExecutionsPerHour int    `db:"max_executions_per_hour"`

	// Scope
	TeamID     *string `db:"team_id"`
	CaseTypes  string  `db:"case_types"`
	Priorities string  `db:"priorities"`

	// Execution tracking
	TotalExecutions int        `db:"total_executions"`
	LastExecutedAt  *time.Time `db:"last_executed_at"`

	// Performance tracking
	AverageExecutionTime int64   `db:"average_execution_time"`
	SuccessRate          float64 `db:"success_rate"`

	// Metadata
	CreatedByID *string    `db:"created_by_id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}

func (Rule) TableName() string {
	return "rules"
}

// RuleExecution represents a rule execution record
type RuleExecution struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`
	RuleID      string `db:"rule_id"`

	// Execution context
	CaseID      *string `db:"case_id"`
	TriggerType string  `db:"trigger_type"`
	Context     string  `db:"context"`

	// Execution details
	Status        string     `db:"status"`
	StartedAt     time.Time  `db:"started_at"`
	CompletedAt   *time.Time `db:"completed_at"`
	ExecutionTime int64      `db:"execution_time"`

	// Results
	ActionsExecuted string `db:"actions_executed"`
	Changes         string `db:"changes"`
	ErrorMessage    string `db:"error_message"`

	// Metadata
	CreatedAt time.Time `db:"created_at"`
}

func (RuleExecution) TableName() string {
	return "rule_executions"
}

// Workflow represents a multi-step business process
type Workflow struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Basic info
	Name        string `db:"name"`
	Description string `db:"description"`
	IsActive    bool   `db:"is_active"`
	Version     int    `db:"version"`

	// Workflow definition
	Steps     string `db:"steps"`
	Triggers  string `db:"triggers"`
	Variables string `db:"variables"`

	// Execution settings
	TimeoutMinutes    int  `db:"timeout_minutes"`
	MaxRetries        int  `db:"max_retries"`
	ParallelExecution bool `db:"parallel_execution"`

	// Statistics
	TotalExecutions int   `db:"total_executions"`
	SuccessfulRuns  int   `db:"successful_runs"`
	FailedRuns      int   `db:"failed_runs"`
	AverageRuntime  int64 `db:"average_runtime"`

	// Metadata
	CreatedByID string     `db:"created_by_id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}

// WorkflowInstance represents a running instance of a workflow
type WorkflowInstance struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`
	WorkflowID  string `db:"workflow_id"`

	// Instance info
	Name        string `db:"name"`
	Status      string `db:"status"`
	CurrentStep string `db:"current_step"`

	// Context
	CaseID    string `db:"case_id"`
	ContactID string `db:"contact_id"`
	UserID    string `db:"user_id"`
	Context   string `db:"context"`
	Variables string `db:"variables"`

	// Execution tracking
	StartedAt     time.Time  `db:"started_at"`
	CompletedAt   *time.Time `db:"completed_at"`
	ExecutionTime int64      `db:"execution_time"`

	// Steps completed
	CompletedSteps string `db:"completed_steps"`
	FailedSteps    string `db:"failed_steps"`

	// Error handling
	ErrorMessage string `db:"error_message"`
	RetryCount   int    `db:"retry_count"`

	// Metadata
	CreatedByID string    `db:"created_by_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}
