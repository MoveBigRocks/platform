package automationservices

import (
	"context"

	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"

	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
)

// JobService handles all job-related business logic
type JobService struct {
	jobStore shared.JobStore
}

// CreateJob creates a new job
func (s *JobService) CreateJob(ctx context.Context, job *automationdomain.Job) error {
	if job.ID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("id", "required"))
	}
	if job.WorkspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if job.Name == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("name", "required"))
	}

	if err := s.jobStore.CreateJob(ctx, job); err != nil {
		return apierrors.DatabaseError("create job", err)
	}
	return nil
}

// GetJob retrieves a job by ID
func (s *JobService) GetJob(ctx context.Context, jobID string) (*automationdomain.Job, error) {
	if jobID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("job_id", "required"))
	}

	job, err := s.jobStore.GetJob(ctx, jobID)
	if err != nil {
		return nil, apierrors.NotFoundError("job", jobID)
	}
	return job, nil
}

// UpdateJob updates an existing job
func (s *JobService) UpdateJob(ctx context.Context, job *automationdomain.Job) error {
	if job.ID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("id", "required"))
	}
	if job.WorkspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}

	if err := s.jobStore.UpdateJob(ctx, job); err != nil {
		return apierrors.DatabaseError("update job", err)
	}
	return nil
}
