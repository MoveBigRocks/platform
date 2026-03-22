package sql

import (
	"context"
	"database/sql"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
)

// IdempotencyStore implements shared.IdempotencyStore using SQL
type IdempotencyStore struct {
	db *SqlxDB
}

// NewIdempotencyStore creates a new idempotency store
func NewIdempotencyStore(db *SqlxDB) *IdempotencyStore {
	return &IdempotencyStore{db: db}
}

func (s *IdempotencyStore) MarkProcessed(ctx context.Context, eventID, handlerGroup string) error {
	// Use composite key (event_id, handler_group) so each handler group
	// independently tracks which events it has processed
	processedAt := time.Now().UTC()
	query := `INSERT INTO core_infra.processed_events (event_id, handler_group, processed_at)
		VALUES (?, ?, ?)
		ON CONFLICT (event_id, handler_group) DO NOTHING`
	_, err := s.db.Get(ctx).ExecContext(ctx, query, eventID, handlerGroup, processedAt)
	return TranslateSqlxError(err, "processed_events")
}

func (s *IdempotencyStore) IsProcessed(ctx context.Context, eventID, handlerGroup string) (bool, error) {
	// Check if this specific handler group has processed this event
	// Different handler groups must independently process the same event
	query := `SELECT 1 FROM core_infra.processed_events WHERE event_id = ? AND handler_group = ? LIMIT 1`
	var exists int
	err := s.db.Get(ctx).GetContext(ctx, &exists, query, eventID, handlerGroup)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, TranslateSqlxError(err, "processed_events")
	}
	return true, nil
}

var _ shared.IdempotencyStore = (*IdempotencyStore)(nil)
