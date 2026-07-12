package sql

import (
	"context"
	"database/sql"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
)

func (s *IdempotencyStore) ClaimProcessing(ctx context.Context, eventID, handlerGroup string, lease time.Duration) (bool, error) {
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	now := time.Now().UTC()
	expiresAt := now.Add(lease)
	query := `INSERT INTO core_infra.processed_events (
			event_id, handler_group, status, claimed_at, claim_expires_at, processed_at
		) VALUES (?, ?, 'processing', ?, ?, NULL)
		ON CONFLICT (event_id, handler_group) DO UPDATE SET
			status = 'processing',
			claimed_at = EXCLUDED.claimed_at,
			claim_expires_at = EXCLUDED.claim_expires_at,
			processed_at = NULL
		WHERE processed_events.status <> 'processed'
		  AND (processed_events.claim_expires_at IS NULL
		       OR processed_events.claim_expires_at <= EXCLUDED.claimed_at)
		RETURNING event_id`
	var claimedEventID string
	err := s.db.Get(ctx).QueryRowxContext(ctx, query, eventID, handlerGroup, now, expiresAt).Scan(&claimedEventID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, TranslateSqlxError(err, "processed_events")
	}
	return true, nil
}

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
	query := `INSERT INTO core_infra.processed_events (event_id, handler_group, status, processed_at)
		VALUES (?, ?, 'processed', ?)
		ON CONFLICT (event_id, handler_group) DO UPDATE SET
			status = 'processed', processed_at = EXCLUDED.processed_at,
			claimed_at = NULL, claim_expires_at = NULL`
	_, err := s.db.Get(ctx).ExecContext(ctx, query, eventID, handlerGroup, processedAt)
	return TranslateSqlxError(err, "processed_events")
}

func (s *IdempotencyStore) ReleaseProcessingClaim(ctx context.Context, eventID, handlerGroup string) error {
	query := `DELETE FROM core_infra.processed_events
		WHERE event_id = ? AND handler_group = ? AND status = 'processing'`
	_, err := s.db.Get(ctx).ExecContext(ctx, query, eventID, handlerGroup)
	return TranslateSqlxError(err, "processed_events")
}

func (s *IdempotencyStore) IsProcessed(ctx context.Context, eventID, handlerGroup string) (bool, error) {
	// Check if this specific handler group has processed this event
	// Different handler groups must independently process the same event
	query := `SELECT 1 FROM core_infra.processed_events
		WHERE event_id = ? AND handler_group = ? AND status = 'processed' LIMIT 1`
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
