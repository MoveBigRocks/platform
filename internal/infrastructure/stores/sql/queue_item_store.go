package sql

import (
	"context"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

type QueueItemStore struct {
	db *SqlxDB
}

func NewQueueItemStore(db *SqlxDB) *QueueItemStore {
	return &QueueItemStore{db: db}
}

func (s *QueueItemStore) CreateQueueItem(ctx context.Context, item *servicedomain.QueueItem) error {
	normalizePersistedUUID(&item.ID)
	query := `INSERT INTO core_service.queue_items (
		id, workspace_id, queue_id, item_kind, case_id, conversation_session_id, created_at, updated_at
	) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?)
	RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(
		ctx,
		query,
		item.ID,
		item.WorkspaceID,
		item.QueueID,
		string(item.ItemKind),
		nullableQueueItemString(item.CaseID),
		nullableQueueItemString(item.ConversationSessionID),
		item.CreatedAt,
		item.UpdatedAt,
	).Scan(&item.ID)
	return TranslateSqlxError(err, "queue_items")
}

func (s *QueueItemStore) GetQueueItem(ctx context.Context, itemID string) (*servicedomain.QueueItem, error) {
	var model models.QueueItem
	query := `SELECT * FROM core_service.queue_items WHERE id = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, itemID); err != nil {
		return nil, TranslateSqlxError(err, "queue_items")
	}
	return s.mapToDomain(&model), nil
}

func (s *QueueItemStore) GetQueueItemByCaseID(ctx context.Context, caseID string) (*servicedomain.QueueItem, error) {
	var model models.QueueItem
	query := `SELECT * FROM core_service.queue_items WHERE case_id = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, caseID); err != nil {
		return nil, TranslateSqlxError(err, "queue_items")
	}
	return s.mapToDomain(&model), nil
}

func (s *QueueItemStore) GetQueueItemByConversationSessionID(ctx context.Context, sessionID string) (*servicedomain.QueueItem, error) {
	var model models.QueueItem
	query := `SELECT * FROM core_service.queue_items WHERE conversation_session_id = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, sessionID); err != nil {
		return nil, TranslateSqlxError(err, "queue_items")
	}
	return s.mapToDomain(&model), nil
}

func (s *QueueItemStore) ListQueueItems(ctx context.Context, queueID string) ([]*servicedomain.QueueItem, error) {
	var out []models.QueueItem
	query := `SELECT * FROM core_service.queue_items WHERE queue_id = ? AND deleted_at IS NULL ORDER BY updated_at DESC, id DESC`
	if err := s.db.Get(ctx).SelectContext(ctx, &out, query, queueID); err != nil {
		return nil, TranslateSqlxError(err, "queue_items")
	}

	result := make([]*servicedomain.QueueItem, len(out))
	for i := range out {
		result[i] = s.mapToDomain(&out[i])
	}
	return result, nil
}

func (s *QueueItemStore) UpdateQueueItem(ctx context.Context, item *servicedomain.QueueItem) error {
	query := `UPDATE core_service.queue_items
		SET queue_id = ?, item_kind = ?, case_id = ?, conversation_session_id = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`

	result, err := s.db.Get(ctx).ExecContext(
		ctx,
		query,
		item.QueueID,
		string(item.ItemKind),
		nullableQueueItemString(item.CaseID),
		nullableQueueItemString(item.ConversationSessionID),
		item.UpdatedAt,
		item.ID,
		item.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "queue_items")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *QueueItemStore) DeleteQueueItem(ctx context.Context, itemID string) error {
	return s.softDelete(ctx, `UPDATE core_service.queue_items SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, itemID)
}

func (s *QueueItemStore) DeleteQueueItemByCaseID(ctx context.Context, caseID string) error {
	return s.softDelete(ctx, `UPDATE core_service.queue_items SET deleted_at = ?, updated_at = ? WHERE case_id = ? AND deleted_at IS NULL`, caseID)
}

func (s *QueueItemStore) DeleteQueueItemByConversationSessionID(ctx context.Context, sessionID string) error {
	return s.softDelete(ctx, `UPDATE core_service.queue_items SET deleted_at = ?, updated_at = ? WHERE conversation_session_id = ? AND deleted_at IS NULL`, sessionID)
}

func (s *QueueItemStore) softDelete(ctx context.Context, query, arg string) error {
	now := time.Now().UTC()
	result, err := s.db.Get(ctx).ExecContext(ctx, query, now, now, arg)
	if err != nil {
		return TranslateSqlxError(err, "queue_items")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *QueueItemStore) mapToDomain(model *models.QueueItem) *servicedomain.QueueItem {
	item := &servicedomain.QueueItem{
		ID:          model.ID,
		WorkspaceID: model.WorkspaceID,
		QueueID:     model.QueueID,
		ItemKind:    servicedomain.QueueItemKind(model.ItemKind),
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
	if model.CaseID != nil {
		item.CaseID = *model.CaseID
	}
	if model.ConversationSessionID != nil {
		item.ConversationSessionID = *model.ConversationSessionID
	}
	return item
}

func nullableQueueItemString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
