package servicehandlers

import (
	"context"
	"time"

	"github.com/movebigrocks/platform/pkg/logger"
)

// EventHandlerMiddleware wraps event handlers with logging and error handling.
func EventHandlerMiddleware(log *logger.Logger, handler func(context.Context, []byte) error) func(context.Context, []byte) error {
	return func(ctx context.Context, data []byte) error {
		start := time.Now()
		err := handler(ctx, data)
		duration := time.Since(start)
		if err != nil {
			log.WithError(err).WithField("duration_ms", duration.Milliseconds()).Error("Event handler failed")
			return err
		}
		log.WithField("duration_ms", duration.Milliseconds()).Debug("Event handler completed")
		return nil
	}
}
