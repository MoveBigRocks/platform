package models

import (
	"time"
)

// Job represents a background task/job
type Job struct {
	ID          string `db:"id"`
	PublicID    string `db:"public_id"`
	WorkspaceID string `db:"workspace_id"`

	// Job definition
	Name     string `db:"name"`
	Queue    string `db:"queue"`
	Priority int    `db:"priority"`
	Status   string `db:"status"`
	Payload  string `db:"payload"`

	// Execution tracking
	Result      string `db:"result"`
	Error       string `db:"error"`
	Attempts    int    `db:"attempts"`
	MaxAttempts int    `db:"max_attempts"`

	// Scheduling
	ScheduledFor *time.Time `db:"scheduled_for"`
	StartedAt    *time.Time `db:"started_at"`
	CompletedAt  *time.Time `db:"completed_at"`

	// Worker management
	WorkerID    string     `db:"worker_id"`
	LockedUntil *time.Time `db:"locked_until"`

	// Metadata
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func (Job) TableName() string {
	return "jobs"
}

// JobQueue represents a job queue configuration
type JobQueue struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Queue configuration
	Name        string `db:"name"`
	Description string `db:"description"`
	IsActive    bool   `db:"is_active"`

	// Processing configuration
	MaxWorkers     int `db:"max_workers"`
	MaxRetries     int `db:"max_retries"`
	RetryDelay     int `db:"retry_delay"`
	ProcessTimeout int `db:"process_timeout"`

	// Statistics
	PendingJobs   int `db:"pending_jobs"`
	RunningJobs   int `db:"running_jobs"`
	CompletedJobs int `db:"completed_jobs"`
	FailedJobs    int `db:"failed_jobs"`

	// Metadata
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// JobTemplate represents a reusable job template
type JobTemplate struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Template definition
	Name        string `db:"name"`
	Description string `db:"description"`
	JobName     string `db:"job_name"`
	Queue       string `db:"queue"`
	Priority    int    `db:"priority"`
	MaxAttempts int    `db:"max_attempts"`

	// Template payload
	PayloadTemplate string `db:"payload_template"`

	// Validation schema
	VariableSchema string `db:"variable_schema"`

	// Usage statistics
	TimesUsed  int        `db:"times_used"`
	LastUsedAt *time.Time `db:"last_used_at"`

	// Metadata
	CreatedBy string    `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// RecurringJob represents a job that runs on a schedule
type RecurringJob struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Job configuration
	Name        string `db:"name"`
	Description string `db:"description"`
	JobName     string `db:"job_name"`
	Queue       string `db:"queue"`
	Priority    int    `db:"priority"`
	Payload     string `db:"payload"`

	// Schedule configuration
	CronExpression string `db:"cron_expression"`
	Timezone       string `db:"timezone"`
	IsActive       bool   `db:"is_active"`

	// Execution tracking
	NextRunAt  *time.Time `db:"next_run_at"`
	LastRunAt  *time.Time `db:"last_run_at"`
	LastJobID  string     `db:"last_job_id"`
	RunCount   int        `db:"run_count"`
	FailedRuns int        `db:"failed_runs"`

	// Configuration
	MaxRuns         int        `db:"max_runs"`
	StopAfter       *time.Time `db:"stop_after"`
	MissedRunPolicy string     `db:"missed_run_policy"`
	OverlapPolicy   string     `db:"overlap_policy"`

	// Metadata
	CreatedBy string    `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// JobExecution represents the execution history of a job
type JobExecution struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`
	JobID       string `db:"job_id"`

	// Execution details
	WorkerID    string    `db:"worker_id"`
	StartedAt   time.Time `db:"started_at"`
	CompletedAt time.Time `db:"completed_at"`
	Duration    int       `db:"duration"`
	Status      string    `db:"status"`

	// Results
	Result string `db:"result"`
	Error  string `db:"error"`

	// System metrics
	CPUUsage    float64 `db:"cpu_usage"`
	MemoryUsage int64   `db:"memory_usage"`

	// Metadata
	CreatedAt time.Time `db:"created_at"`
}
