package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

// QueueStore implements shared.QueueStore using SQLite/sqlx.
type QueueStore struct {
	db *SqlxDB
}

func NewQueueStore(db *SqlxDB) *QueueStore {
	return &QueueStore{db: db}
}

func (s *QueueStore) CreateQueue(ctx context.Context, queue *servicedomain.Queue) error {
	metadata, err := marshalJSONString(queue.Metadata, "metadata")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	normalizePersistedUUID(&queue.ID)
	query := `INSERT INTO core_service.case_queues (
		id, workspace_id, slug, name, description, metadata, created_at, updated_at
	) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?)
	RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(
		ctx,
		query,
		queue.ID,
		queue.WorkspaceID,
		queue.Slug,
		queue.Name,
		queue.Description,
		metadata,
		queue.CreatedAt,
		queue.UpdatedAt,
	).Scan(&queue.ID)
	return TranslateSqlxError(err, "case_queues")
}

func (s *QueueStore) GetQueue(ctx context.Context, queueID string) (*servicedomain.Queue, error) {
	var model models.Queue
	query := `SELECT * FROM core_service.case_queues WHERE id = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, queueID); err != nil {
		return nil, TranslateSqlxError(err, "case_queues")
	}
	return s.mapToDomain(&model), nil
}

func (s *QueueStore) GetQueueBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.Queue, error) {
	var model models.Queue
	query := `SELECT * FROM core_service.case_queues WHERE workspace_id = ? AND slug = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, workspaceID, slug); err != nil {
		return nil, TranslateSqlxError(err, "case_queues")
	}
	return s.mapToDomain(&model), nil
}

func (s *QueueStore) ListWorkspaceQueues(ctx context.Context, workspaceID string) ([]*servicedomain.Queue, error) {
	var out []models.Queue
	query := `SELECT * FROM core_service.case_queues WHERE workspace_id = ? AND deleted_at IS NULL ORDER BY created_at DESC`
	if err := s.db.Get(ctx).SelectContext(ctx, &out, query, workspaceID); err != nil {
		return nil, TranslateSqlxError(err, "case_queues")
	}

	result := make([]*servicedomain.Queue, len(out))
	for i := range out {
		result[i] = s.mapToDomain(&out[i])
	}
	return result, nil
}

func (s *QueueStore) UpdateQueue(ctx context.Context, queue *servicedomain.Queue) error {
	metadata, err := marshalJSONString(queue.Metadata, "metadata")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `UPDATE core_service.case_queues
		SET slug = ?, name = ?, description = ?, metadata = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`

	result, err := s.db.Get(ctx).ExecContext(
		ctx,
		query,
		queue.Slug,
		queue.Name,
		queue.Description,
		metadata,
		time.Now(),
		queue.ID,
		queue.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "case_queues")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *QueueStore) DeleteQueue(ctx context.Context, workspaceID, queueID string) error {
	now := time.Now()
	query := `UPDATE core_service.case_queues SET deleted_at = ?, updated_at = ? WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, now, now, queueID, workspaceID)
	if err != nil {
		return TranslateSqlxError(err, "case_queues")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *QueueStore) mapToDomain(model *models.Queue) *servicedomain.Queue {
	metadata := shareddomain.NewTypedCustomFields()
	if model.Metadata != "" {
		unmarshalJSONField(model.Metadata, &metadata, "case_queues", "metadata")
	}

	return &servicedomain.Queue{
		ID:          model.ID,
		WorkspaceID: model.WorkspaceID,
		Slug:        model.Slug,
		Name:        model.Name,
		Description: model.Description,
		Metadata:    metadata,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
		DeletedAt:   model.DeletedAt,
	}
}
