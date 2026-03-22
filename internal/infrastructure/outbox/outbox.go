package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/id"
	"github.com/movebigrocks/platform/pkg/logger"
)

// Service implements the Outbox pattern for reliable event publishing
type Service struct {
	store    stores.Store
	eventBus eventbus.Bus
	logger   *logger.Logger

	workerCtx    context.Context
	workerCancel context.CancelFunc
	workerWg     sync.WaitGroup

	// Configurable settings
	pollInterval     time.Duration
	maxRetries       int
	retention        time.Duration
	batchSize        int
	maxBackoff       time.Duration
	healthMaxPending int
	healthMaxAge     time.Duration
}

// NewService creates a new Outbox service with default configuration.
// Deprecated: Use NewServiceWithConfig for configurable settings.
func NewService(store stores.Store, eventBus eventbus.Bus, log *logger.Logger) *Service {
	return NewServiceWithConfig(store, eventBus, log, config.OutboxConfig{
		PollInterval:     5 * time.Second,
		MaxRetries:       10,
		RetentionDays:    7,
		BatchSize:        100,
		MaxBackoff:       5 * time.Minute,
		HealthMaxPending: 100,
		HealthMaxAge:     5 * time.Minute,
	})
}

// NewServiceWithConfig creates a new Outbox service with the provided configuration.
func NewServiceWithConfig(store stores.Store, eventBus eventbus.Bus, log *logger.Logger, cfg config.OutboxConfig) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	if log == nil {
		log = logger.NewNop()
	}

	return &Service{
		store:            store,
		eventBus:         eventBus,
		logger:           log,
		workerCtx:        ctx,
		workerCancel:     cancel,
		pollInterval:     cfg.PollInterval,
		maxRetries:       cfg.MaxRetries,
		retention:        time.Duration(cfg.RetentionDays) * 24 * time.Hour,
		batchSize:        cfg.BatchSize,
		maxBackoff:       cfg.MaxBackoff,
		healthMaxPending: cfg.HealthMaxPending,
		healthMaxAge:     cfg.HealthMaxAge,
	}
}

// PublishEvent publishes a type-safe event using the outbox pattern.
// This is the preferred method - events must implement eventbus.Event interface.
// The event is saved to the database and dispatched by the background worker.
func (s *Service) PublishEvent(ctx context.Context, stream eventbus.Stream, event eventbus.Event) error {
	// Validate event using the Event interface method
	if err := event.Validate(); err != nil {
		s.logger.WithError(err).Error("Event validation failed before outbox save",
			"stream", stream.String(),
			"event_type", event.GetEventType().String(),
			"event_id", event.GetEventID(),
		)
		return fmt.Errorf("event validation failed: %w", err)
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	outboxEvent := &shared.OutboxEvent{
		ID:        event.GetEventID(), // Use the event's own ID for traceability
		Stream:    stream.String(),    // Store as string for persistence
		EventType: event.GetEventType().String(),
		EventData: eventData,
		Status:    "pending",
		Attempts:  0,
		CreatedAt: time.Now(),
	}

	// Save to durable storage - worker will dispatch
	if err := s.store.Outbox().SaveOutboxEvent(ctx, outboxEvent); err != nil {
		return fmt.Errorf("failed to save outbox event: %w", err)
	}

	s.logger.Debug("Event saved to outbox (pending dispatch)",
		"event_id", outboxEvent.ID,
		"event_type", event.GetEventType().String(),
		"stream", stream.String(),
	)

	return nil
}

// Publish publishes an event using the outbox pattern for dynamic/custom payloads.
// Prefer PublishEvent when compile-time type safety is available.
// The event is saved to the database and dispatched by the background worker.
func (s *Service) Publish(ctx context.Context, stream eventbus.Stream, event interface{}) error {
	// Validate event if it implements Validator interface
	if validator, ok := event.(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			s.logger.WithError(err).Error("Event validation failed before outbox save",
				"stream", stream.String(),
				"event_type", fmt.Sprintf("%T", event),
			)
			return fmt.Errorf("event validation failed: %w", err)
		}
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	outboxEvent := &shared.OutboxEvent{
		ID:        generateEventID(),
		Stream:    stream.String(),
		EventType: fmt.Sprintf("%T", event),
		EventData: eventData,
		Status:    "pending",
		Attempts:  0,
		CreatedAt: time.Now(),
	}

	// Save to durable storage - worker will dispatch
	if err := s.store.Outbox().SaveOutboxEvent(ctx, outboxEvent); err != nil {
		return fmt.Errorf("failed to save outbox event: %w", err)
	}

	s.logger.Debug("Event saved to outbox (pending dispatch)",
		"event_id", outboxEvent.ID,
		"stream", stream.String(),
	)

	return nil
}

// Start starts the background retry worker
func (s *Service) Start() {
	s.workerWg.Add(1)
	go s.retryWorker()
	s.logger.Info("Outbox service started")
}

// Stop gracefully stops the service
func (s *Service) Stop(timeout time.Duration) error {
	s.logger.Info("Outbox service shutting down...")
	s.workerCancel()

	done := make(chan struct{})
	go func() {
		s.workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Outbox service stopped")
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("shutdown timed out after %v", timeout)
	}
}

// HealthCheck checks outbox health and detects lag
func (s *Service) HealthCheck(ctx context.Context) (string, int, error) {
	events, err := s.store.Outbox().GetPendingOutboxEvents(ctx, s.healthMaxPending*10)
	if err != nil {
		return "unhealthy", len(events), fmt.Errorf("failed to check outbox: %w", err)
	}

	pendingCount := len(events)

	// Check for old pending events (lag indicator)
	var oldestPendingAge time.Duration
	if pendingCount > 0 {
		oldestPendingAge = time.Since(events[0].CreatedAt)
	}

	if pendingCount > s.healthMaxPending {
		return "degraded", pendingCount, fmt.Errorf("outbox has %d pending events (threshold: %d)", pendingCount, s.healthMaxPending)
	}

	if oldestPendingAge > s.healthMaxAge {
		return "degraded", pendingCount, fmt.Errorf("oldest pending event is %v old (threshold: %v)", oldestPendingAge.Round(time.Second), s.healthMaxAge)
	}

	return "healthy", pendingCount, nil
}

// retryWorker polls for pending events and retries publishing
func (s *Service) retryWorker() {
	defer s.workerWg.Done()

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.workerCtx.Done():
			return
		case <-ticker.C:
			s.processPendingEvents()
		}
	}
}

// processPendingEvents retrieves and processes pending events.
// SQLite uses a single-writer model which prevents concurrent event processing.
// For multi-instance deployments, only one instance should run the outbox processor.
func (s *Service) processPendingEvents() {
	ctx := context.Background()
	eventsToProcess := make([]*shared.OutboxEvent, 0, s.batchSize)

	// Claim events within a transaction to prevent duplicate dispatch.
	// SQLite uses one writable connection, so we keep the lock window small
	// and dispatch is intentionally done after this transaction commits.
	err := s.store.WithTransaction(ctx, func(txCtx context.Context) error {
		events, err := s.store.Outbox().GetPendingOutboxEvents(txCtx, s.batchSize)
		if err != nil {
			return fmt.Errorf("get pending events: %w", err)
		}

		if len(events) > 0 {
			s.logger.Debug("Processing pending events", "count", len(events))
		}

		for _, event := range events {
			event.Status = "publishing"
			if err := s.store.Outbox().UpdateOutboxEvent(txCtx, event); err != nil {
				return fmt.Errorf("mark event as publishing: %w", err)
			}
			eventsToProcess = append(eventsToProcess, event)
		}

		return nil
	})

	if err != nil {
		s.logger.WithError(err).Error("Failed to process pending events")
		return
	}

	for _, event := range eventsToProcess {
		s.retryPublish(ctx, event)
	}

	// Cleanup old events outside transaction (best-effort, non-critical)
	cutoff := time.Now().Add(-s.retention)
	if err := s.store.Outbox().DeletePublishedOutboxEvents(ctx, cutoff); err != nil {
		s.logger.Warnw("failed to cleanup old outbox events", "error", err)
	}

	// Recover stale "publishing" events (crashed mid-publish). 2 minutes is enough
	// for any publish operation to complete; anything older is considered stale.
	if count, err := s.store.Outbox().RecoverStalePublishingEvents(ctx, 2*time.Minute); err != nil {
		s.logger.Warnw("failed to recover stale publishing events", "error", err)
	} else if count > 0 {
		s.logger.Info("Recovered stale publishing events", "count", count)
	}
}

// retryPublish attempts to publish with exponential backoff using two-phase commit.
// Phase 1: Mark as "publishing" to prevent duplicate processing
// Phase 2: Publish event, then mark as "published" or revert to "pending" on failure
func (s *Service) retryPublish(ctx context.Context, event *shared.OutboxEvent) {
	if event.Attempts >= s.maxRetries {
		event.Status = "failed"
		event.LastError = fmt.Sprintf("max retries (%d) exceeded", s.maxRetries)
		if err := s.store.Outbox().UpdateOutboxEvent(ctx, event); err != nil {
			s.logger.Warnw("failed to update failed outbox event", "error", err, "event_id", event.ID)
		}
		s.logger.Error("Max retries exceeded", "event_id", event.ID)
		return
	}

	if event.NextRetry != nil && time.Now().Before(*event.NextRetry) {
		return
	}

	var eventData interface{}
	if err := json.Unmarshal(event.EventData, &eventData); err != nil {
		event.Status = "failed"
		event.LastError = fmt.Sprintf("unmarshal error: %v", err)
		if err := s.store.Outbox().UpdateOutboxEvent(ctx, event); err != nil {
			s.logger.Warnw("failed to update failed outbox event", "error", err, "event_id", event.ID)
		}
		return
	}

	// Phase 1: Mark as "publishing" to claim the event and prevent duplicates
	event.Status = "publishing"
	if err := s.store.Outbox().UpdateOutboxEvent(ctx, event); err != nil {
		s.logger.Warnw("failed to mark event as publishing", "error", err, "event_id", event.ID)
		return
	}

	stream := eventbus.StreamFromString(event.Stream)
	eventType := eventbus.EventTypeFromString(event.EventType)

	// Phase 2: Publish the event
	if err := s.eventBus.PublishWithType(stream, eventType, event.ID, eventData); err != nil {
		// Revert to pending with backoff
		event.Status = "pending"
		event.Attempts++
		event.LastError = err.Error()

		backoff := time.Duration(1<<uint(event.Attempts)) * time.Second
		if backoff > s.maxBackoff {
			backoff = s.maxBackoff
		}
		nextRetry := time.Now().Add(backoff)
		event.NextRetry = &nextRetry

		if err := s.store.Outbox().UpdateOutboxEvent(ctx, event); err != nil {
			s.logger.Warnw("failed to revert event to pending", "error", err, "event_id", event.ID)
		}
		s.logger.Warn("Retry failed", "event_id", event.ID, "event_type", event.EventType, "attempts", event.Attempts)
		return
	}

	// Phase 2 complete: Mark as published
	now := time.Now()
	event.Status = "published"
	event.PublishedAt = &now
	if err := s.store.Outbox().UpdateOutboxEvent(ctx, event); err != nil {
		// Event was published but we couldn't update status. It will be picked up
		// as "publishing" by cleanup and retried. Handlers must be idempotent.
		s.logger.Errorw("event published but status update failed - handlers must be idempotent",
			"error", err, "event_id", event.ID, "event_type", event.EventType)
		return
	}
	s.logger.Info("Event published", "event_id", event.ID, "event_type", event.EventType, "attempts", event.Attempts)
}

func generateEventID() string {
	return id.New()
}

// GetPendingEvents retrieves pending events (exposed for worker)
func (s *Service) GetPendingEvents(ctx context.Context, limit int) ([]*shared.OutboxEvent, error) {
	return s.store.Outbox().GetPendingOutboxEvents(ctx, limit)
}

// ProcessPendingEvent processes a single pending event (exposed for worker)
func (s *Service) ProcessPendingEvent(ctx context.Context, event *shared.OutboxEvent) bool {
	s.retryPublish(ctx, event)
	// Check if it was successfully published
	return event.Status == "published"
}
