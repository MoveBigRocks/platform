package platformservices

import (
	"context"
	"fmt"
	"time"
)

// System Health Methods for AdminStatsService

// GetSystemHealth checks the health of all system components (Phase 4 Implementation)
func (s *AdminStatsService) GetSystemHealth(ctx context.Context) ([]SystemHealth, error) {
	var healthChecks []SystemHealth

	// Check storage health
	storageHealth := SystemHealth{
		Component: "storage",
		Status:    "healthy",
		Message:   "Storage backend operational",
		Details:   make(map[string]interface{}),
	}

	err := s.store.HealthCheck(ctx)
	if err != nil {
		storageHealth.Status = "down"
		storageHealth.Message = fmt.Sprintf("Storage health check failed: %v", err)
	}
	healthChecks = append(healthChecks, storageHealth)

	// Workers health (Phase 4 Implementation)
	workerHealth := s.getWorkerHealth(ctx)
	healthChecks = append(healthChecks, workerHealth)

	// Job Queue health
	jobQueueHealth := s.getJobQueueHealth(ctx)
	healthChecks = append(healthChecks, jobQueueHealth)

	// Event Bus health
	eventBusHealth := s.getEventBusHealth(ctx)
	healthChecks = append(healthChecks, eventBusHealth)

	// API health (always healthy if we're responding)
	apiHealth := SystemHealth{
		Component: "api",
		Status:    "healthy",
		Message:   "API responding",
		Details:   make(map[string]interface{}),
	}
	healthChecks = append(healthChecks, apiHealth)

	return healthChecks, nil
}

// getWorkerHealth returns the health status of background workers
func (s *AdminStatsService) getWorkerHealth(ctx context.Context) SystemHealth {
	health := SystemHealth{
		Component: "workers",
		Status:    "healthy",
		Message:   "Background workers operational",
		Details:   make(map[string]interface{}),
	}

	// Get job processing metrics
	jobsProcessed := 0
	jobsFailed := 0
	pendingJobs := 0

	if s.usePrometheus {
		// Get from Prometheus
		processed, err := s.prometheusClient.Query(ctx, "increase(mbr_jobs_processed_total{status=\"success\"}[1h])")
		if err == nil {
			if val, err := s.prometheusClient.GetScalarValue(processed); err == nil {
				jobsProcessed = int(val)
			}
		}
		failed, err := s.prometheusClient.Query(ctx, "increase(mbr_jobs_processed_total{status=\"failed\"}[1h])")
		if err == nil {
			if val, err := s.prometheusClient.GetScalarValue(failed); err == nil {
				jobsFailed = int(val)
			}
		}
	} else {
		// Count from storage (ignore errors: continue with 0 counts)
		workspaces, _ := s.store.Workspaces().ListWorkspaces(ctx) //nolint:errcheck
		cutoff := time.Now().Add(-1 * time.Hour)

		limit := len(workspaces)
		if limit > 10 {
			limit = 10
		}

		for i := 0; i < limit; i++ {
			// Count completed jobs (ignore errors: skip workspace on failure)
			completedJobs, _, _ := s.store.Jobs().ListWorkspaceJobs(ctx, workspaces[i].ID, "completed", "", 100, 0) //nolint:errcheck
			for _, job := range completedJobs {
				if job.CompletedAt != nil && job.CompletedAt.After(cutoff) {
					jobsProcessed++
				}
			}

			// Count failed jobs (ignore errors: skip workspace on failure)
			failedJobs, _, _ := s.store.Jobs().ListWorkspaceJobs(ctx, workspaces[i].ID, "failed", "", 100, 0) //nolint:errcheck
			for _, job := range failedJobs {
				if job.UpdatedAt.After(cutoff) {
					jobsFailed++
				}
			}

			// Count pending jobs (ignore errors: skip workspace on failure)
			pendingJobsList, _, _ := s.store.Jobs().ListWorkspaceJobs(ctx, workspaces[i].ID, "pending", "", 100, 0) //nolint:errcheck
			pendingJobs += len(pendingJobsList)
		}
	}

	health.Details["jobs_processed_1h"] = jobsProcessed
	health.Details["jobs_failed_1h"] = jobsFailed
	health.Details["jobs_pending"] = pendingJobs

	// Calculate failure rate
	if jobsProcessed+jobsFailed > 0 {
		failureRate := float64(jobsFailed) / float64(jobsProcessed+jobsFailed) * 100
		health.Details["failure_rate"] = fmt.Sprintf("%.1f%%", failureRate)

		if failureRate > 50 {
			health.Status = "degraded"
			health.Message = fmt.Sprintf("High job failure rate: %.1f%%", failureRate)
		} else if failureRate > 20 {
			health.Status = "degraded"
			health.Message = fmt.Sprintf("Elevated job failure rate: %.1f%%", failureRate)
		}
	}

	// Check for job queue backlog
	if pendingJobs > 1000 {
		health.Status = "degraded"
		health.Message = fmt.Sprintf("Large job queue backlog: %d pending jobs", pendingJobs)
	}

	return health
}

// getJobQueueHealth returns the health status of job queues
func (s *AdminStatsService) getJobQueueHealth(ctx context.Context) SystemHealth {
	health := SystemHealth{
		Component: "job_queue",
		Status:    "healthy",
		Message:   "Job queues operational",
		Details:   make(map[string]interface{}),
	}

	// Count jobs by queue (ignore errors: continue with empty counts)
	queueCounts := make(map[string]int)
	workspaces, _ := s.store.Workspaces().ListWorkspaces(ctx) //nolint:errcheck

	limit := len(workspaces)
	if limit > 10 {
		limit = 10
	}

	for i := 0; i < limit; i++ {
		// Get pending jobs (ignore errors: skip workspace on failure)
		pendingJobs, _, _ := s.store.Jobs().ListWorkspaceJobs(ctx, workspaces[i].ID, "pending", "", 500, 0) //nolint:errcheck
		for _, job := range pendingJobs {
			queueCounts[job.Queue]++
		}
	}

	health.Details["queue_depths"] = queueCounts

	// Check for any queue with too many pending jobs
	for queue, count := range queueCounts {
		if count > 500 {
			health.Status = "degraded"
			health.Message = fmt.Sprintf("Queue '%s' has %d pending jobs", queue, count)
			break
		}
	}

	return health
}

// getEventBusHealth returns the health status of the event bus
func (s *AdminStatsService) getEventBusHealth(ctx context.Context) SystemHealth {
	health := SystemHealth{
		Component: "event_bus",
		Status:    "healthy",
		Message:   "Event bus operational",
		Details:   make(map[string]interface{}),
	}

	// Check outbox for pending events
	pendingEvents, err := s.store.Outbox().GetPendingOutboxEvents(ctx, 100)
	if err != nil {
		health.Status = "degraded"
		health.Message = fmt.Sprintf("Failed to check outbox: %v", err)
		return health
	}

	health.Details["pending_events"] = len(pendingEvents)

	// Check for old pending events (stuck in outbox)
	stuckEvents := 0
	cutoff := time.Now().Add(-5 * time.Minute)
	for _, event := range pendingEvents {
		if event.CreatedAt.Before(cutoff) {
			stuckEvents++
		}
	}

	health.Details["stuck_events"] = stuckEvents

	if stuckEvents > 10 {
		health.Status = "degraded"
		health.Message = fmt.Sprintf("%d events stuck in outbox for over 5 minutes", stuckEvents)
	} else if len(pendingEvents) > 50 {
		health.Status = "degraded"
		health.Message = fmt.Sprintf("Large outbox backlog: %d pending events", len(pendingEvents))
	}

	return health
}
