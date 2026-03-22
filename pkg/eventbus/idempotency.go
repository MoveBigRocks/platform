package eventbus

import (
	"context"

	"github.com/movebigrocks/platform/internal/infrastructure/metrics"
)

// IdempotencyStore interface for database-backed idempotency.
type IdempotencyStore interface {
	MarkProcessed(ctx context.Context, eventID, handlerGroup string) error
	IsProcessed(ctx context.Context, eventID, handlerGroup string) (bool, error)
}

// WithDBIdempotency wraps a handler with database-backed duplicate detection.
func WithDBIdempotency(store IdempotencyStore, handlerGroup string, handler Handler) Handler {
	return func(ctx context.Context, data []byte) error {
		hdr, err := ParseEventHeader(data)
		if err != nil {
			return err
		}

		processed, err := store.IsProcessed(ctx, hdr.EventID, handlerGroup)
		if err != nil {
			metrics.IdempotencyCheckErrors.WithLabelValues(handlerGroup).Inc()
			// Log error but continue - better to potentially duplicate than drop
			return handler(ctx, data)
		}

		if processed {
			metrics.EventsDeduplicated.WithLabelValues(handlerGroup).Inc()
			return nil // Already processed, skip
		}

		if err := handler(ctx, data); err != nil {
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
