package shareddomain

import (
	"encoding/json"
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
)

// Job & Background Processing Events

// JobEnqueued is published when a job is queued
type JobEnqueued struct {
	eventbus.BaseEvent

	// Event payload
	JobID       string
	JobType     string
	WorkspaceID string
	Priority    string
	Payload     json.RawMessage
	ScheduledAt time.Time
	EnqueuedAt  time.Time
}

// JobStarted is published when a job begins processing
type JobStarted struct {
	eventbus.BaseEvent

	// Event payload
	JobID       string
	JobType     string
	WorkspaceID string
	WorkerID    string
	Attempt     int
	StartedAt   time.Time
}

// JobCompleted is published when a job completes successfully
type JobCompleted struct {
	eventbus.BaseEvent

	// Event payload
	JobID       string
	JobType     string
	WorkspaceID string
	WorkerID    string
	Result      json.RawMessage
	Duration    int64
	CompletedAt time.Time
}

// JobFailed is published when a job fails
type JobFailed struct {
	eventbus.BaseEvent

	// Event payload
	JobID       string
	JobType     string
	WorkspaceID string
	WorkerID    string
	Error       string
	Attempt     int
	WillRetry   bool
	Duration    int64
	FailedAt    time.Time
}

// JobRetrying is published when a job is scheduled for retry
type JobRetrying struct {
	eventbus.BaseEvent

	// Event payload
	JobID       string
	JobType     string
	WorkspaceID string
	Attempt     int
	MaxAttempts int
	NextRetryAt time.Time
	LastError   string
	ScheduledAt time.Time
}

// JobCanceled is published when a job is canceled
type JobCanceled struct {
	eventbus.BaseEvent

	// Event payload
	JobID       string
	JobType     string
	WorkspaceID string
	CanceledBy  string
	Reason      string
	CanceledAt  time.Time
}

// Validation Methods

// Validate validates the JobEnqueued event
func (e JobEnqueued) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("job_id", e.JobID); err != nil {
		return err
	}
	if err := validateNonEmpty("job_type", e.JobType); err != nil {
		return err
	}
	if err := validateNonEmpty("workspace_id", e.WorkspaceID); err != nil {
		return err
	}

	if e.EnqueuedAt.IsZero() {
		return ErrRequiredField("enqueued_at")
	}

	return nil
}

// Validate validates the JobStarted event
func (e JobStarted) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("job_id", e.JobID); err != nil {
		return err
	}
	if err := validateNonEmpty("job_type", e.JobType); err != nil {
		return err
	}
	if e.StartedAt.IsZero() {
		return ErrRequiredField("started_at")
	}
	return nil
}

// Validate validates the JobCompleted event
func (e JobCompleted) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("job_id", e.JobID); err != nil {
		return err
	}
	if err := validateNonEmpty("job_type", e.JobType); err != nil {
		return err
	}
	if e.CompletedAt.IsZero() {
		return ErrRequiredField("completed_at")
	}
	if err := validateNonNegativeInt("duration", int(e.Duration)); err != nil {
		return err
	}
	return nil
}

// Validate validates the JobFailed event
func (e JobFailed) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("job_id", e.JobID); err != nil {
		return err
	}
	if err := validateNonEmpty("job_type", e.JobType); err != nil {
		return err
	}
	if err := validateNonEmpty("error", e.Error); err != nil {
		return err
	}
	if e.FailedAt.IsZero() {
		return ErrRequiredField("failed_at")
	}
	return nil
}

// Validate validates the JobRetrying event
func (e JobRetrying) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("job_id", e.JobID); err != nil {
		return err
	}
	if err := validateNonEmpty("job_type", e.JobType); err != nil {
		return err
	}
	if err := validatePositiveInt("attempt", e.Attempt); err != nil {
		return err
	}
	if e.NextRetryAt.IsZero() {
		return ErrRequiredField("next_retry_at")
	}
	return nil
}

// Validate validates the JobCanceled event
func (e JobCanceled) Validate() error {
	if err := validateNonEmpty("event_id", e.EventID); err != nil {
		return err
	}
	if e.EventType.IsZero() {
		return ErrRequiredField("event_type")
	}
	if err := validateNonEmpty("job_id", e.JobID); err != nil {
		return err
	}
	if err := validateNonEmpty("job_type", e.JobType); err != nil {
		return err
	}
	if err := validateNonEmpty("canceled_by", e.CanceledBy); err != nil {
		return err
	}
	if e.CanceledAt.IsZero() {
		return ErrRequiredField("canceled_at")
	}
	return nil
}
