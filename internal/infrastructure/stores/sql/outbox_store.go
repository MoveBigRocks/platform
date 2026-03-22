package sql

import (
	"context"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
)

// OutboxStore implements shared.OutboxStore using sqlx
type OutboxStore struct {
	db *SqlxDB
}

// NewOutboxStore creates a new OutboxStore
func NewOutboxStore(db *SqlxDB) *OutboxStore {
	return &OutboxStore{db: db}
}

func (s *OutboxStore) SaveOutboxEvent(ctx context.Context, event *shared.OutboxEvent) error {
	normalizePersistedUUID(&event.ID)
	query := `INSERT INTO core_infra.outbox_events (
		id, stream, event_type, event_data, status, attempts, created_at,
		published_at, last_error, next_retry
	) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?)
	RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		event.ID, event.Stream, event.EventType, event.EventData, event.Status,
		event.Attempts, event.CreatedAt, event.PublishedAt, event.LastError,
		event.NextRetry,
	).Scan(&event.ID)
	return TranslateSqlxError(err, "outbox_events")
}

func (s *OutboxStore) GetOutboxEvent(ctx context.Context, eventID string) (*shared.OutboxEvent, error) {
	var dbEvent models.OutboxEvent
	query := `SELECT * FROM core_infra.outbox_events WHERE id = ?`
	err := s.db.Get(ctx).GetContext(ctx, &dbEvent, query, eventID)
	if err != nil {
		return nil, TranslateSqlxError(err, "outbox_events")
	}
	return s.mapToShared(&dbEvent), nil
}

func (s *OutboxStore) GetPendingOutboxEvents(ctx context.Context, limit int) ([]*shared.OutboxEvent, error) {
	var dbEvents []models.OutboxEvent
	now := time.Now().UTC()

	query := `SELECT * FROM core_infra.outbox_events
		WHERE status = 'pending'
		  AND (next_retry IS NULL OR next_retry <= ?)
		ORDER BY created_at ASC
		LIMIT ?`

	err := s.db.Get(ctx).SelectContext(ctx, &dbEvents, query, now, limit)
	if err != nil {
		return nil, TranslateSqlxError(err, "outbox_events")
	}

	events := make([]*shared.OutboxEvent, len(dbEvents))
	for i, e := range dbEvents {
		events[i] = s.mapToShared(&e)
	}
	return events, nil
}

func (s *OutboxStore) UpdateOutboxEvent(ctx context.Context, event *shared.OutboxEvent) error {
	query := `UPDATE core_infra.outbox_events SET
		stream = ?, event_type = ?, event_data = ?, status = ?, attempts = ?,
		published_at = ?, last_error = ?, next_retry = ?
		WHERE id = ?`

	_, err := s.db.Get(ctx).ExecContext(ctx, query,
		event.Stream, event.EventType, event.EventData, event.Status,
		event.Attempts, event.PublishedAt, event.LastError, event.NextRetry, event.ID,
	)
	return TranslateSqlxError(err, "outbox_events")
}

func (s *OutboxStore) DeletePublishedOutboxEvents(ctx context.Context, before time.Time) error {
	query := `DELETE FROM core_infra.outbox_events WHERE status = 'published' AND published_at < ?`
	_, err := s.db.Get(ctx).ExecContext(ctx, query, before)
	return TranslateSqlxError(err, "outbox_events")
}

func (s *OutboxStore) RecoverStalePublishingEvents(ctx context.Context, staleThreshold time.Duration) (int, error) {
	cutoff := time.Now().Add(-staleThreshold)
	query := `UPDATE core_infra.outbox_events SET status = 'pending', attempts = attempts + 1,
		last_error = 'recovered from stale publishing state'
		WHERE status = 'publishing' AND created_at < ?`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, TranslateSqlxError(err, "outbox_events")
	}
	count, err := result.RowsAffected()
	// ExecContext succeeded; if row count is unavailable, return 0 recovered.
	if err == nil {
		return int(count), nil
	}
	return 0, nil
}

func (s *OutboxStore) mapToShared(e *models.OutboxEvent) *shared.OutboxEvent {
	return &shared.OutboxEvent{
		ID:          e.ID,
		Stream:      e.Stream,
		EventType:   e.EventType,
		EventData:   e.EventData,
		Status:      e.Status,
		Attempts:    e.Attempts,
		CreatedAt:   e.CreatedAt,
		PublishedAt: e.PublishedAt,
		LastError:   e.LastError,
		NextRetry:   e.NextRetry,
	}
}

// CountOutboxEventsByStream returns the total count of events for a stream
func (s *OutboxStore) CountOutboxEventsByStream(ctx context.Context, stream string) (int64, error) {
	var count int64
	var query string
	var args []interface{}

	if stream != "" {
		query = `SELECT COUNT(*) FROM core_infra.outbox_events WHERE stream = ?`
		args = []interface{}{stream}
	} else {
		query = `SELECT COUNT(*) FROM core_infra.outbox_events`
	}

	err := s.db.Get(ctx).GetContext(ctx, &count, query, args...)
	return count, TranslateSqlxError(err, "outbox_events")
}

// CountPendingOutboxEvents returns the count of pending events for a stream
func (s *OutboxStore) CountPendingOutboxEvents(ctx context.Context, stream string) (int64, error) {
	var count int64
	var query string
	var args []interface{}

	if stream != "" {
		query = `SELECT COUNT(*) FROM core_infra.outbox_events WHERE status = 'pending' AND stream = ?`
		args = []interface{}{stream}
	} else {
		query = `SELECT COUNT(*) FROM core_infra.outbox_events WHERE status = 'pending'`
	}

	err := s.db.Get(ctx).GetContext(ctx, &count, query, args...)
	return count, TranslateSqlxError(err, "outbox_events")
}
