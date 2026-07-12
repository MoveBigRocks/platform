package eventbus

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/metrics"
)

// IdempotencyStore interface for database-backed idempotency.
type IdempotencyStore interface {
	ClaimProcessing(ctx context.Context, eventID, handlerGroup string, lease time.Duration) (bool, error)
	MarkProcessed(ctx context.Context, eventID, handlerGroup string) error
	ReleaseProcessingClaim(ctx context.Context, eventID, handlerGroup string) error
	IsProcessed(ctx context.Context, eventID, handlerGroup string) (bool, error)
}

// WithDBIdempotency wraps a handler with database-backed duplicate detection.
func WithDBIdempotency(store IdempotencyStore, handlerGroup string, handler Handler) Handler {
	return func(ctx context.Context, data []byte) error {
		hdr, err := ParseEventHeader(data)
		if err != nil {
			return err
		}

		claimed, err := store.ClaimProcessing(ctx, hdr.EventID, handlerGroup, 2*time.Minute)
		if err != nil {
			metrics.IdempotencyCheckErrors.WithLabelValues(handlerGroup).Inc()
			// Fail closed so a transient database outage cannot turn into concurrent
			// duplicate side effects. Returning an error lets the bus retry safely.
			return fmt.Errorf("claim idempotency lease: %w", err)
		}
		if !claimed {
			metrics.EventsDeduplicated.WithLabelValues(handlerGroup).Inc()
			return nil // Already processed or leased by another worker.
		}

		if err := handler(ctx, data); err != nil {
			_ = store.ReleaseProcessingClaim(ctx, hdr.EventID, handlerGroup)
			return err
		}

		metrics.EventsHandled.WithLabelValues(handlerGroup).Inc()

		// Mark as processed after successful handling
		if err := store.MarkProcessed(ctx, hdr.EventID, handlerGroup); err != nil {
			// Log error but don't fail - event was processed successfully
			// On replay, handler will re-execute (handlers must be idempotent)
			metrics.MarkProcessedErrors.WithLabelValues(handlerGroup).Inc()
		}
		return nil
	}
}
