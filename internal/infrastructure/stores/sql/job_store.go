package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	automationdomain "github.com/movebigrocks/platform/internal/automation/domain"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
)

// JobStore implements shared.JobStore using sqlx
type JobStore struct {
	db *SqlxDB
}

// NewJobStore creates a new job store
func NewJobStore(db *SqlxDB) *JobStore {
	return &JobStore{db: db}
}

// Jobs

func (s *JobStore) CreateJob(ctx context.Context, job *automationdomain.Job) error {
	if err := job.Validate(); err != nil {
		return err
	}
	normalizePersistedUUID(&job.ID)
	payload, err := marshalJSONString(job.Payload, "payload")
	if err != nil {
		return err
	}
	result, err := marshalJSONString(job.Result, "result")
	if err != nil {
		return err
	}

	query := `
		INSERT INTO core_automation.jobs (
			id, public_id, workspace_id, name, queue, priority, status, payload,
			result, error, attempts, max_attempts, scheduled_for, started_at,
			completed_at, worker_id, locked_until, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?
		)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		job.ID, job.PublicID, job.WorkspaceID, job.Name, job.Queue, int(job.Priority), string(job.Status), payload,
		result, job.Error, job.Attempts, job.MaxAttempts, job.ScheduledFor, job.StartedAt,
		job.CompletedAt, job.WorkerID, job.LockedUntil, job.CreatedAt, job.UpdatedAt,
	).Scan(&job.ID)
	return TranslateSqlxError(err, "jobs")
}

func (s *JobStore) GetJob(ctx context.Context, jobID string) (*automationdomain.Job, error) {
	var model models.Job
	query := `SELECT * FROM core_automation.jobs WHERE id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &model, query, jobID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "jobs")
	}
	return s.mapJobToDomain(&model)
}

func (s *JobStore) UpdateJob(ctx context.Context, job *automationdomain.Job) error {
	if err := job.Validate(); err != nil {
		return err
	}
	payload, err := marshalJSONString(job.Payload, "payload")
	if err != nil {
		return err
	}
	result, err := marshalJSONString(job.Result, "result")
	if err != nil {
		return err
	}

	query := `
		UPDATE core_automation.jobs SET
			name = ?, queue = ?, priority = ?, status = ?, payload = ?,
			result = ?, error = ?, attempts = ?, max_attempts = ?,
			scheduled_for = ?, started_at = ?, completed_at = ?,
			worker_id = ?, locked_until = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`

	res, err := s.db.Get(ctx).ExecContext(ctx, query,
		job.Name, job.Queue, int(job.Priority), string(job.Status), payload,
		result, job.Error, job.Attempts, job.MaxAttempts,
		job.ScheduledFor, job.StartedAt, job.CompletedAt,
		job.WorkerID, job.LockedUntil, job.UpdatedAt,
		job.ID, job.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "jobs")
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return TranslateSqlxError(err, "jobs")
	}
	if rowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *JobStore) ListWorkspaceJobs(ctx context.Context, workspaceID string, status automationdomain.JobStatus, queue string, limit, offset int) ([]*automationdomain.Job, int, error) {
	// Build query dynamically based on filters
	var conditions []string
	args := []interface{}{workspaceID}

	conditions = append(conditions, "workspace_id = ?")
	conditions = append(conditions, "deleted_at IS NULL")

	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(status))
	}
	if queue != "" {
		conditions = append(conditions, "queue = ?")
		args = append(args, queue)
	}

	whereClause := strings.Join(conditions, " AND ")
	countQuery := `SELECT COUNT(*) FROM core_automation.jobs WHERE ` + whereClause
	selectQuery := `SELECT * FROM core_automation.jobs WHERE ` + whereClause

	// Get count
	var total int
	if err := s.db.Get(ctx).GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, TranslateSqlxError(err, "jobs")
	}

	// Get rows with pagination
	selectQuery += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	var dbModels []models.Job
	if err := s.db.Get(ctx).SelectContext(ctx, &dbModels, selectQuery, args...); err != nil {
		return nil, 0, TranslateSqlxError(err, "jobs")
	}

	result := make([]*automationdomain.Job, len(dbModels))
	for i, m := range dbModels {
		domainJob, err := s.mapJobToDomain(&m)
		if err != nil {
			return nil, 0, err
		}
		result[i] = domainJob
	}
	return result, total, nil
}

// Helpers

func (s *JobStore) mapJobToDomain(m *models.Job) (*automationdomain.Job, error) {
	var payload json.RawMessage
	if m.Payload != "" {
		if err := json.Unmarshal([]byte(m.Payload), &payload); err != nil {
			return nil, err
		}
	}

	var result json.RawMessage
	if m.Result != "" {
		if err := json.Unmarshal([]byte(m.Result), &result); err != nil {
			return nil, err
		}
	}

	return &automationdomain.Job{
		ID:           m.ID,
		PublicID:     m.PublicID,
		WorkspaceID:  m.WorkspaceID,
		Name:         m.Name,
		Queue:        m.Queue,
		Priority:     automationdomain.JobPriority(m.Priority),
		Status:       automationdomain.JobStatus(m.Status),
		Payload:      payload,
		Result:       result,
		Error:        m.Error,
		Attempts:     m.Attempts,
		MaxAttempts:  m.MaxAttempts,
		ScheduledFor: m.ScheduledFor,
		StartedAt:    m.StartedAt,
		CompletedAt:  m.CompletedAt,
		WorkerID:     m.WorkerID,
		LockedUntil:  m.LockedUntil,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}, nil
}
