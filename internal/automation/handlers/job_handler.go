package automationhandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// JobEventHandler handles job-related events
type JobEventHandler struct {
	jobService *automationservices.JobService
	logger     *logger.Logger
}

// NewJobEventHandler creates a new job event handler
func NewJobEventHandler(jobService *automationservices.JobService, log *logger.Logger) *JobEventHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &JobEventHandler{
		jobService: jobService,
		logger:     log,
	}
}

// HandleJobEnqueued processes JobEnqueued events
func (h *JobEventHandler) HandleJobEnqueued(ctx context.Context, eventData []byte) error {
	var event shareddomain.JobEnqueued
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal JobEnqueued event: %w", err)
	}

	// Validate this is actually a JobEnqueued event (has EnqueuedAt set)
	if event.EnqueuedAt.IsZero() {
		// Not a JobEnqueued event, silently ignore
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"job_id":       event.JobID,
		"job_type":     event.JobType,
		"workspace_id": event.WorkspaceID,
	}).Info("Processing JobEnqueued event")

	// Construct job from event
	j := &automationdomain.Job{
		ID:          event.JobID,
		Name:        event.JobType,
		WorkspaceID: event.WorkspaceID,
		Priority:    parsePriority(event.Priority),
		Payload:     event.Payload,
		Status:      automationdomain.JobStatusPending,
		CreatedAt:   event.EnqueuedAt,
		UpdatedAt:   event.EnqueuedAt,
	}

	if !event.ScheduledAt.IsZero() {
		j.ScheduledFor = &event.ScheduledAt
	}

	// Store the job
	if err := h.jobService.CreateJob(ctx, j); err != nil {
		return fmt.Errorf("failed to store job: %w", err)
	}

	h.logger.WithField("job_id", event.JobID).Info("Successfully processed JobEnqueued event")
	return nil
}

// HandleJobStarted processes JobStarted events
func (h *JobEventHandler) HandleJobStarted(ctx context.Context, eventData []byte) error {
	var event shareddomain.JobStarted
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal JobStarted event: %w", err)
	}

	// Validate this is actually a JobStarted event (has WorkerID and StartedAt set)
	if event.WorkerID == "" || event.StartedAt.IsZero() {
		// Not a JobStarted event, silently ignore
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"job_id":    event.JobID,
		"worker_id": event.WorkerID,
		"attempt":   event.Attempt,
	}).Info("Processing JobStarted event")

	// Get the job
	j, err := h.jobService.GetJob(ctx, event.JobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Use domain method to transition state
	j.MarkRunning(event.WorkerID)
	j.Attempts = event.Attempt

	// Save updated job
	if err := h.jobService.UpdateJob(ctx, j); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	h.logger.WithField("job_id", event.JobID).Info("Successfully processed JobStarted event")
	return nil
}

// HandleJobCompleted processes JobCompleted events
func (h *JobEventHandler) HandleJobCompleted(ctx context.Context, eventData []byte) error {
	var event shareddomain.JobCompleted
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal JobCompleted event: %w", err)
	}

	// Validate this is actually a JobCompleted event (has CompletedAt set)
	if event.CompletedAt.IsZero() {
		// Not a JobCompleted event, silently ignore
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"job_id":   event.JobID,
		"duration": event.Duration,
	}).Info("Processing JobCompleted event")

	// Get the job
	j, err := h.jobService.GetJob(ctx, event.JobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Use domain method to transition state
	if err := j.MarkCompleted(event.Result); err != nil {
		return fmt.Errorf("failed to mark job completed: %w", err)
	}

	// Save updated job
	if err := h.jobService.UpdateJob(ctx, j); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	h.logger.WithField("job_id", event.JobID).Info("Successfully processed JobCompleted event")
	return nil
}

// HandleJobFailed processes JobFailed events
func (h *JobEventHandler) HandleJobFailed(ctx context.Context, eventData []byte) error {
	var event shareddomain.JobFailed
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal JobFailed event: %w", err)
	}

	// Validate this is actually a JobFailed event (has Error and FailedAt set)
	if event.Error == "" || event.FailedAt.IsZero() {
		// Not a JobFailed event, silently ignore
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"job_id":     event.JobID,
		"error":      event.Error,
		"will_retry": event.WillRetry,
	}).Info("Processing JobFailed event")

	// Get the job
	j, err := h.jobService.GetJob(ctx, event.JobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Use domain methods to transition state
	if event.WillRetry {
		j.MarkRetrying()
	} else {
		if err := j.MarkFailed(event.Error); err != nil {
			return fmt.Errorf("failed to mark job failed: %w", err)
		}
	}
	j.Attempts = event.Attempt

	// Save updated job
	if err := h.jobService.UpdateJob(ctx, j); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	h.logger.WithFields(map[string]interface{}{
		"job_id":     event.JobID,
		"will_retry": event.WillRetry,
	}).Info("Successfully processed JobFailed event")
	return nil
}

// HandleJobRetrying processes JobRetrying events
func (h *JobEventHandler) HandleJobRetrying(ctx context.Context, eventData []byte) error {
	var event shareddomain.JobRetrying
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal JobRetrying event: %w", err)
	}

	// Validate this is actually a JobRetrying event (has MaxAttempts and NextRetryAt set)
	if event.MaxAttempts == 0 || event.NextRetryAt.IsZero() {
		// Not a JobRetrying event, silently ignore
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"job_id":       event.JobID,
		"attempt":      event.Attempt,
		"max_attempts": event.MaxAttempts,
	}).Info("Processing JobRetrying event")

	// Get the job
	j, err := h.jobService.GetJob(ctx, event.JobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Use domain method to mark for retry
	j.MarkRetrying()
	j.Attempts = event.Attempt
	j.Error = event.LastError

	// Save updated job
	if err := h.jobService.UpdateJob(ctx, j); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	h.logger.WithField("job_id", event.JobID).Info("Successfully processed JobRetrying event")
	return nil
}

// HandleJobCanceled processes JobCanceled events
func (h *JobEventHandler) HandleJobCanceled(ctx context.Context, eventData []byte) error {
	var event shareddomain.JobCanceled
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal JobCanceled event: %w", err)
	}

	// Validate this is actually a JobCanceled event (has CanceledBy and CanceledAt set)
	if event.CanceledBy == "" || event.CanceledAt.IsZero() {
		// Not a JobCanceled event, silently ignore
		return nil
	}

	h.logger.WithFields(map[string]interface{}{
		"job_id":      event.JobID,
		"canceled_by": event.CanceledBy,
		"reason":      event.Reason,
	}).Info("Processing JobCanceled event")

	// Get the job
	j, err := h.jobService.GetJob(ctx, event.JobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Use domain method to mark as canceled
	j.MarkCanceled()

	// Save updated job
	if err := h.jobService.UpdateJob(ctx, j); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	h.logger.WithField("job_id", event.JobID).Info("Successfully processed JobCanceled event")
	return nil
}

// RegisterHandlers registers all job event handlers with the event bus
func (h *JobEventHandler) RegisterHandlers(subscribe func(stream eventbus.Stream, group, consumer string, handler func(context.Context, []byte) error) error) error {
	// Wrap handlers with middleware
	jobEnqueuedHandler := EventHandlerMiddleware(h.logger, h.HandleJobEnqueued)
	jobStartedHandler := EventHandlerMiddleware(h.logger, h.HandleJobStarted)
	jobCompletedHandler := EventHandlerMiddleware(h.logger, h.HandleJobCompleted)
	jobFailedHandler := EventHandlerMiddleware(h.logger, h.HandleJobFailed)
	jobRetryingHandler := EventHandlerMiddleware(h.logger, h.HandleJobRetrying)
	jobCanceledHandler := EventHandlerMiddleware(h.logger, h.HandleJobCanceled)

	// Register all job event handlers
	// Each event type gets its own consumer group to ensure all handlers receive all events
	stream := eventbus.StreamJobEvents

	if err := subscribe(stream, "job-enqueued", "consumer", jobEnqueuedHandler); err != nil {
		return fmt.Errorf("failed to register JobEnqueued handler: %w", err)
	}

	if err := subscribe(stream, "job-started", "consumer", jobStartedHandler); err != nil {
		return fmt.Errorf("failed to register JobStarted handler: %w", err)
	}

	if err := subscribe(stream, "job-completed", "consumer", jobCompletedHandler); err != nil {
		return fmt.Errorf("failed to register JobCompleted handler: %w", err)
	}

	if err := subscribe(stream, "job-failed", "consumer", jobFailedHandler); err != nil {
		return fmt.Errorf("failed to register JobFailed handler: %w", err)
	}

	if err := subscribe(stream, "job-retrying", "consumer", jobRetryingHandler); err != nil {
		return fmt.Errorf("failed to register JobRetrying handler: %w", err)
	}

	if err := subscribe(stream, "job-canceled", "consumer", jobCanceledHandler); err != nil {
		return fmt.Errorf("failed to register JobCanceled handler: %w", err)
	}

	h.logger.Info("Job event handlers registered successfully")
	return nil
}

// Helper function to parse job priority from string
func parsePriority(priority string) automationdomain.JobPriority {
	switch priority {
	case "low":
		return automationdomain.JobPriorityLow
	case "normal":
		return automationdomain.JobPriorityNormal
	case "high":
		return automationdomain.JobPriorityHigh
	case "critical":
		return automationdomain.JobPriorityCritical
	default:
		return automationdomain.JobPriorityNormal
	}
}

// EventHandlerMiddleware wraps an event handler with logging and error handling
func EventHandlerMiddleware(logger *logger.Logger, handler func(context.Context, []byte) error) func(context.Context, []byte) error {
	return func(ctx context.Context, data []byte) error {
		start := time.Now()

		err := handler(ctx, data)

		duration := time.Since(start)
		if err != nil {
			logger.WithError(err).WithField("duration_ms", duration.Milliseconds()).Error("Event handler failed")
			return err
		}

		logger.WithField("duration_ms", duration.Milliseconds()).Debug("Event handler completed")
		return nil
	}
}
