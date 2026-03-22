// Package eventbus provides an in-memory event dispatcher.
// The eventbus handles routing events to registered handlers.
// Durability is handled separately by the outbox service.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Handler processes an event payload.
type Handler func(ctx context.Context, data []byte) error

// Logger interface for the event bus.
type Logger interface {
	Info(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// stdLogger wraps the standard log package.
type stdLogger struct{}

func (l stdLogger) Info(msg string, args ...interface{})  { log.Printf("[INFO] "+msg, args...) }
func (l stdLogger) Debug(msg string, args ...interface{}) { log.Printf("[DEBUG] "+msg, args...) }
func (l stdLogger) Warn(msg string, args ...interface{})  { log.Printf("[WARN] "+msg, args...) }
func (l stdLogger) Error(msg string, args ...interface{}) { log.Printf("[ERROR] "+msg, args...) }

// InMemoryBus is a thread-safe in-memory event dispatcher.
// It routes events to registered handlers without any database interaction.
// Durability and retry logic are handled by the outbox service.
type InMemoryBus struct {
	handlers map[string][]Handler // stream -> handlers
	mu       sync.RWMutex
	logger   Logger
	dlq      DLQ

	// Shutdown coordination
	wg         sync.WaitGroup
	shutdownMu sync.RWMutex
	shutdown   bool
}

// NewInMemoryBus creates a new in-memory event bus with default logger.
func NewInMemoryBus() *InMemoryBus {
	return NewInMemoryBusWithLogger(stdLogger{})
}

// NewInMemoryBusWithLogger creates a new in-memory event bus with custom logger.
func NewInMemoryBusWithLogger(logger Logger) *InMemoryBus {
	return &InMemoryBus{
		handlers: make(map[string][]Handler),
		logger:   logger,
	}
}

// SetDLQ sets the dead letter queue for failed events.
func (b *InMemoryBus) SetDLQ(dlq DLQ) {
	b.dlq = dlq
}

// Subscribe registers a handler for a stream.
// Multiple handlers can be registered for the same stream.
func (b *InMemoryBus) Subscribe(stream Stream, group, consumer string, handler func(ctx context.Context, data []byte) error) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	streamKey := stream.String()
	b.handlers[streamKey] = append(b.handlers[streamKey], handler)
	b.logger.Info("Subscribed to stream=%s group=%s consumer=%s handler_count=%d",
		streamKey, group, consumer, len(b.handlers[streamKey]))
	return nil
}

// Dispatch sends an event to all registered handlers for a stream.
// This is called by the outbox worker after fetching events from the database.
// Returns an error if any handler fails (all handlers are still invoked).
func (b *InMemoryBus) Dispatch(ctx context.Context, stream Stream, eventType EventType, payload []byte) error {
	b.shutdownMu.RLock()
	if b.shutdown {
		b.shutdownMu.RUnlock()
		return fmt.Errorf("eventbus is shutting down")
	}
	b.shutdownMu.RUnlock()

	b.mu.RLock()
	handlers := b.handlers[stream.String()]
	if len(handlers) == 0 {
		b.mu.RUnlock()
		b.logger.Debug("No handlers for stream=%s event_type=%s", stream.String(), eventType.String())
		return nil
	}
	// Copy handlers to avoid holding lock during execution
	handlersCopy := make([]Handler, len(handlers))
	copy(handlersCopy, handlers)
	b.mu.RUnlock()

	// Execute all handlers, collecting errors
	var errs []error
	var errsMu sync.Mutex
	var wg sync.WaitGroup

	for i, h := range handlersCopy {
		wg.Add(1)
		b.wg.Add(1)
		go func(idx int, handler Handler) {
			defer wg.Done()
			defer b.wg.Done()

			// Create handler context with timeout
			handlerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Execute with panic recovery
			err := b.executeHandler(handlerCtx, handler, stream, eventType, payload)
			if err != nil {
				errsMu.Lock()
				errs = append(errs, err)
				errsMu.Unlock()
			}
		}(i, h)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("handler(s) failed: %v", errs[0])
	}
	return nil
}

// executeHandler runs a single handler with panic recovery.
func (b *InMemoryBus) executeHandler(ctx context.Context, handler Handler, stream Stream, eventType EventType, payload []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
			b.logger.Error("Handler panicked stream=%s event_type=%s panic=%v",
				stream.String(), eventType.String(), r)
			if b.dlq != nil {
				if dlqErr := b.dlq.SendHandlerError(ctx, stream.String(), eventType.String(), payload, err); dlqErr != nil {
					b.logger.Error("Failed to send to DLQ stream=%s event_type=%s dlq_error=%v",
						stream.String(), eventType.String(), dlqErr)
				}
			}
		}
	}()

	if err = handler(ctx, payload); err != nil {
		b.logger.Error("Handler failed stream=%s event_type=%s error=%v",
			stream.String(), eventType.String(), err)
		if b.dlq != nil {
			if dlqErr := b.dlq.SendHandlerError(ctx, stream.String(), eventType.String(), payload, err); dlqErr != nil {
				b.logger.Error("Failed to send to DLQ stream=%s event_type=%s dlq_error=%v",
					stream.String(), eventType.String(), dlqErr)
			}
		}
		return err
	}
	return nil
}

// PublishEvent dispatches an event directly to handlers (synchronous).
// For durable publishing, use the outbox service instead.
func (b *InMemoryBus) PublishEvent(stream Stream, event Event) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("event validation failed: %w", err)
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return b.Dispatch(context.Background(), stream, event.GetEventType(), payload)
}

// PublishEventWithRetry dispatches with retry (retries handled in-process).
func (b *InMemoryBus) PublishEventWithRetry(stream Stream, event Event, maxRetries int) error {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if err := b.PublishEvent(stream, event); err != nil {
			lastErr = err
			time.Sleep(time.Duration(1<<i) * 100 * time.Millisecond) // Exponential backoff
			continue
		}
		return nil
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// PublishValidated is an alias for PublishEvent.
func (b *InMemoryBus) PublishValidated(stream Stream, event Event) error {
	return b.PublishEvent(stream, event)
}

// Publish dispatches raw data to handlers.
func (b *InMemoryBus) Publish(stream Stream, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	eventType := EventTypeFromString(ParseEventType(payload))
	return b.Dispatch(context.Background(), stream, eventType, payload)
}

// PublishWithType dispatches raw data with preserved event type.
func (b *InMemoryBus) PublishWithType(stream Stream, eventType EventType, eventID string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	return b.Dispatch(context.Background(), stream, eventType, payload)
}

// GetStreamInfo returns info about a stream (handler count).
func (b *InMemoryBus) GetStreamInfo(stream Stream) (length int64, groups int64, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	handlers := b.handlers[stream.String()]
	return 0, int64(len(handlers)), nil
}

// GetPendingMessages returns 0 for in-memory bus (no queue).
func (b *InMemoryBus) GetPendingMessages(stream Stream, group string) (int64, error) {
	return 0, nil
}

// HealthCheck verifies the event bus is operational.
func (b *InMemoryBus) HealthCheck() error {
	b.shutdownMu.RLock()
	defer b.shutdownMu.RUnlock()

	if b.shutdown {
		return fmt.Errorf("eventbus is shutdown")
	}
	return nil
}

// Shutdown gracefully shuts down the event bus.
func (b *InMemoryBus) Shutdown(timeout time.Duration) error {
	b.shutdownMu.Lock()
	b.shutdown = true
	b.shutdownMu.Unlock()

	// Wait for in-flight handlers with timeout
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		b.logger.Info("EventBus shutdown complete")
		return nil
	case <-time.After(timeout):
		b.logger.Warn("EventBus shutdown timed out timeout=%v", timeout)
		return fmt.Errorf("shutdown timed out after %v", timeout)
	}
}

// Close closes the event bus.
func (b *InMemoryBus) Close() error {
	return b.Shutdown(30 * time.Second)
}

// HandlerCount returns the number of handlers for a stream (for testing).
func (b *InMemoryBus) HandlerCount(stream Stream) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[stream.String()])
}

// Ensure InMemoryBus implements Bus interface
var _ Bus = (*InMemoryBus)(nil)
