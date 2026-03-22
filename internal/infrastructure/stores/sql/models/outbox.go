package models

import (
	"time"
)

// OutboxEvent represents an event in the outbox table
type OutboxEvent struct {
	ID            string     `db:"id"`
	Stream        string     `db:"stream"`
	AggregateType *string    `db:"aggregate_type"` // Domain entity type (e.g., 'case', 'contact', 'issue')
	AggregateID   *string    `db:"aggregate_id"`   // Domain entity ID for event sourcing queries
	EventType     string     `db:"event_type"`
	EventData     []byte     `db:"event_data"`
	CorrelationID *string    `db:"correlation_id"` // Request ID for distributed tracing
	Status        string     `db:"status"`
	Attempts      int        `db:"attempts"`
	CreatedAt     time.Time  `db:"created_at"`
	PublishedAt   *time.Time `db:"published_at"`
	LastError     string     `db:"last_error"`
	NextRetry     *time.Time `db:"next_retry"`
}

func (OutboxEvent) TableName() string {
	return "outbox_events"
}
