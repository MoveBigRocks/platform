package automationdomain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/pkg/id"
)

// JobStatus represents the status of a background job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCanceled  JobStatus = "canceled"
	JobStatusRetrying  JobStatus = "retrying"
)

// JobPriority represents the priority of a background job
type JobPriority int

const (
	JobPriorityLow      JobPriority = 0
	JobPriorityNormal   JobPriority = 5
	JobPriorityHigh     JobPriority = 10
	JobPriorityCritical JobPriority = 20
)

// String returns the string representation of JobPriority
func (p JobPriority) String() string {
	switch p {
	case JobPriorityLow:
		return "low"
	case JobPriorityNormal:
		return "normal"
	case JobPriorityHigh:
		return "high"
	case JobPriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Job represents a background task/job
type Job struct {
	ID          string
	PublicID    string // Base58-encoded public ID (external ID, no index needed)
	WorkspaceID string // Indexed for ListWorkspaceJobs (admin dashboards)

	// Job definition
	Name     string          // Task name (e.g., "send_email")
	Queue    string          // Indexed for queue-specific workers (job routing)
	Priority JobPriority     // Indexed for priority-based processing (job scheduling)
	Status   JobStatus       // Indexed for GetJobsForProcessing (worker polling - critical)
	Payload  json.RawMessage // Job arguments (type-safe deferred JSON)

	// Execution tracking
	Result      json.RawMessage // Job result (if any)
	Error       string          // Error message if failed
	Attempts    int             // Current attempt count
	MaxAttempts int             // Maximum retry attempts

	// Scheduling
	ScheduledFor *time.Time // When to run the job
	StartedAt    *time.Time // When job execution began
	CompletedAt  *time.Time // When job completed

	// Worker management
	WorkerID    string     // Which worker is processing
	LockedUntil *time.Time // Lock timeout for race prevention

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewJob creates a new workspace-scoped job with default values.
// payload should be a marshallable struct or nil.
func NewJob(workspaceID, name string, payload any) (*Job, error) {
	now := time.Now()
	var rawPayload json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal job payload: %w", err)
		}
		rawPayload = data
	}

	job := &Job{
		WorkspaceID:  workspaceID,
		PublicID:     generatePublicID(),
		Name:         name,
		Queue:        "default",
		Priority:     JobPriorityNormal,
		Status:       JobStatusPending,
		Payload:      rawPayload,
		Attempts:     0,
		MaxAttempts:  3,
		ScheduledFor: &now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := job.Validate(); err != nil {
		return nil, fmt.Errorf("job validation failed: %w", err)
	}

	return job, nil
}

// NewWorkspaceJob creates a new workspace-scoped job
func NewWorkspaceJob(workspaceID, name string, payload any) (*Job, error) {
	return NewJob(workspaceID, name, payload)
}

// Validate validates a job's business rules
// DOMAIN BUSINESS RULES: All job validation rules defined here
func (j *Job) Validate() error {
	// Required fields
	if j.Name == "" {
		return fmt.Errorf("job name is required")
	}
	if j.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}

	// Validate priority is within known range
	if j.Priority < JobPriorityLow || j.Priority > JobPriorityCritical {
		return fmt.Errorf("invalid job priority: %d (must be between %d and %d)",
			j.Priority, JobPriorityLow, JobPriorityCritical)
	}

	return nil
}

// IsGlobal reports whether the job is missing a workspace assignment.
func (j *Job) IsGlobal() bool {
	return j.WorkspaceID == ""
}

// IsReady returns true if the job is ready to be processed
func (j *Job) IsReady() bool {
	return j.Status == JobStatusPending &&
		(j.ScheduledFor == nil || j.ScheduledFor.Before(time.Now()) || j.ScheduledFor.Equal(time.Now()))
}

// CanRetry returns true if the job can be retried
// DOMAIN BUSINESS RULE: Jobs can only be retried if they failed and haven't exceeded max attempts
func (j *Job) CanRetry() bool {
	return j.Status == JobStatusFailed && j.Attempts < j.MaxAttempts
}

// CanCancel returns true if the job can be canceled
// DOMAIN BUSINESS RULE: Jobs can only be canceled if they're pending or retrying (not running, completed, failed, or already canceled)
func (j *Job) CanCancel() bool {
	return j.Status == JobStatusPending || j.Status == JobStatusRetrying
}

// NextRetryTime calculates the next retry time using exponential backoff
func (j *Job) NextRetryTime() time.Time {
	// Exponential backoff: 60 * (2 ^ (attempts - 1)) seconds
	delay := 60 * (1 << uint(j.Attempts))
	return time.Now().Add(time.Duration(delay) * time.Second)
}

// MarkRunning marks the job as running with the given worker ID
func (j *Job) MarkRunning(workerID string) {
	j.Status = JobStatusRunning
	j.WorkerID = workerID
	now := time.Now()
	j.StartedAt = &now
	lockedUntil := now.Add(15 * time.Minute)
	j.LockedUntil = &lockedUntil // 15 minute lock timeout
	j.UpdatedAt = now
}

// MarkCompleted marks the job as completed with optional result
func (j *Job) MarkCompleted(result any) error {
	j.Status = JobStatusCompleted
	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal job result: %w", err)
		}
		j.Result = data
	}
	now := time.Now()
	j.CompletedAt = &now
	j.LockedUntil = nil
	j.WorkerID = ""
	j.UpdatedAt = now
	return nil
}

// MarkFailed marks the job as failed with error message
func (j *Job) MarkFailed(errorMsg string) error {
	j.Status = JobStatusFailed
	j.Error = errorMsg
	now := time.Now()
	j.CompletedAt = &now
	j.LockedUntil = nil
	j.WorkerID = ""
	j.UpdatedAt = now
	return nil
}

// MarkRetrying marks the job for retry
func (j *Job) MarkRetrying() {
	j.Status = JobStatusRetrying
	j.Attempts++
	j.ScheduledFor = &[]time.Time{j.NextRetryTime()}[0]
	j.LockedUntil = nil
	j.WorkerID = ""
	j.UpdatedAt = time.Now()
}

// MarkCanceled marks the job as canceled
func (j *Job) MarkCanceled() {
	j.Status = JobStatusCanceled
	now := time.Now()
	j.CompletedAt = &now
	j.LockedUntil = nil
	j.WorkerID = ""
	j.UpdatedAt = now
}

// IsLocked returns true if the job is currently locked by a worker
func (j *Job) IsLocked() bool {
	return j.LockedUntil != nil && j.LockedUntil.After(time.Now())
}

// IsExpired returns true if the job lock has expired
func (j *Job) IsExpired() bool {
	return j.Status == JobStatusRunning &&
		j.LockedUntil != nil &&
		j.LockedUntil.Before(time.Now())
}

// UnmarshalPayload unmarshals the payload into the provided struct
func (j *Job) UnmarshalPayload(v any) error {
	if len(j.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(j.Payload, v)
}

// UnmarshalResult unmarshals the result into the provided struct
func (j *Job) UnmarshalResult(v any) error {
	if len(j.Result) == 0 {
		return nil
	}
	return json.Unmarshal(j.Result, v)
}

// payloadMap returns the payload as a map for helper-style access.
func (j *Job) payloadMap() map[string]any {
	if len(j.Payload) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(j.Payload, &m); err != nil {
		return nil
	}
	return m
}

// GetPayloadString safely gets a string value from payload
func (j *Job) GetPayloadString(key string) string {
	m := j.payloadMap()
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// GetPayloadInt safely gets an int value from payload
func (j *Job) GetPayloadInt(key string) int {
	m := j.payloadMap()
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case json.Number:
			if i, err := v.Int64(); err == nil {
				return int(i)
			}
		}
	}
	return 0
}

// GetPayloadBool safely gets a bool value from payload
func (j *Job) GetPayloadBool(key string) bool {
	m := j.payloadMap()
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// GetPayloadSlice safely gets a slice value from payload
func (j *Job) GetPayloadSlice(key string) []any {
	m := j.payloadMap()
	if val, ok := m[key]; ok {
		if slice, ok := val.([]any); ok {
			return slice
		}
	}
	return nil
}

// JobQueue represents a job queue configuration
type JobQueue struct {
	ID          string
	WorkspaceID string

	// Queue configuration
	Name        string // Queue name
	Description string // Queue description
	IsActive    bool   // Whether queue is processing jobs

	// Processing configuration
	MaxWorkers     int // Maximum concurrent workers
	MaxRetries     int // Default max retries for jobs
	RetryDelay     int // Base retry delay in seconds
	ProcessTimeout int // Job processing timeout in seconds

	// Statistics
	PendingJobs   int // Current pending job count
	RunningJobs   int // Current running job count
	CompletedJobs int // Total completed jobs
	FailedJobs    int // Total failed jobs

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// JobTemplate represents a reusable job template
type JobTemplate struct {
	ID          string
	WorkspaceID string

	// Template definition
	Name        string      // Template name
	Description string      // Template description
	JobName     string      // Actual job name to execute
	Queue       string      // Default queue
	Priority    JobPriority // Default priority
	MaxAttempts int         // Default max attempts

	// Template payload with variable placeholders
	PayloadTemplate json.RawMessage

	// Validation schema for template variables
	VariableSchema json.RawMessage

	// Usage statistics
	TimesUsed  int
	LastUsedAt *time.Time

	// Metadata
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RecurringJob represents a job that runs on a schedule
type RecurringJob struct {
	ID          string
	WorkspaceID string

	// Job configuration
	Name        string          // Recurring job name
	Description string          // Description
	JobName     string          // Actual job to execute
	Queue       string          // Queue for generated jobs
	Priority    JobPriority     // Priority for generated jobs
	Payload     json.RawMessage // Payload for generated jobs

	// Schedule configuration
	CronExpression string // Cron expression for scheduling
	Timezone       string // Timezone for schedule
	IsActive       bool   // Whether job is active

	// Execution tracking
	NextRunAt  *time.Time // Next scheduled run
	LastRunAt  *time.Time // Last execution time
	LastJobID  string     // ID of last generated job
	RunCount   int        // Total runs
	FailedRuns int        // Failed run count

	// Configuration
	MaxRuns         int        // Maximum runs (0 = unlimited)
	StopAfter       *time.Time // Stop after this date
	MissedRunPolicy string     // "skip", "run_once", "catch_up"
	OverlapPolicy   string     // "skip", "queue", "terminate"

	// Metadata
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// JobExecution represents the execution history of a job
type JobExecution struct {
	ID          string
	WorkspaceID string
	JobID       string

	// Execution details
	WorkerID    string    // Worker that executed the job
	StartedAt   time.Time // Execution start time
	CompletedAt time.Time // Execution completion time
	Duration    int       // Execution duration in milliseconds
	Status      JobStatus // Final execution status

	// Results
	Result json.RawMessage // Execution result
	Error  string          // Error message if failed

	// System metrics
	CPUUsage    float64 // CPU usage during execution
	MemoryUsage int64   // Memory usage in bytes

	// Metadata
	CreatedAt time.Time
}

// ScheduledJob represents a recurring scheduled job
type ScheduledJob struct {
	ID              string
	WorkspaceID     string
	Type            string
	Name            string
	Description     string
	Payload         json.RawMessage
	Priority        JobPriority
	CronExpression  string
	IntervalSeconds int
	ScheduledAt     *time.Time
	LastRunAt       *time.Time
	NextRunAt       *time.Time
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Helper function to generate public IDs
func generatePublicID() string {
	return id.NewPublicID()
}
