package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/movebigrocks/platform/pkg/logger"
)

// MaxRetries is the maximum number of retry attempts before moving to DLQ
const MaxRetries = 3

// FileEventBus provides fail-safe event publishing and consumption using filesystem
type FileEventBus struct {
	basePath     string
	logger       *logger.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	stopChan     chan struct{}
	workersWg    sync.WaitGroup
	mu           sync.RWMutex
	watchers     map[string]*fsnotify.Watcher
	streamLock   sync.Map // map[string]*sync.Mutex - per-stream processing lock
	shutdownOnce sync.Once
}

// NewFileEventBus creates a new filesystem-based event bus.
// The provided context is used as the parent context for all internal operations,
// enabling proper cancellation propagation and tracing.
func NewFileEventBus(ctx context.Context, basePath string, log *logger.Logger) (*FileEventBus, error) {
	eventsPath := filepath.Join(basePath, "events")
	if err := os.MkdirAll(eventsPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create events directory: %w", err)
	}

	ebCtx, cancel := context.WithCancel(ctx)

	if log == nil {
		log = logger.NewNop()
	}

	log.Info("FileEventBus initialized", "path", eventsPath)

	return &FileEventBus{
		basePath: eventsPath,
		logger:   log,
		ctx:      ebCtx,
		cancel:   cancel,
		stopChan: make(chan struct{}),
		watchers: make(map[string]*fsnotify.Watcher),
	}, nil
}

// EventFile represents the structure of an event file.
// Uses json.RawMessage for Data to preserve the exact JSON bytes during serialization,
// allowing type-safe deserialization with ParseEventData().
type EventFile struct {
	ID        string          `json:"id"`
	Stream    string          `json:"stream"`
	EventType EventType       `json:"event_type"` // Type-safe event type for deserialization
	Data      json.RawMessage `json:"data"`       // Raw JSON bytes, preserves type information
	Timestamp int64           `json:"timestamp"`
	Retries   int             `json:"retries"`
}

// ParseEventData deserializes the event data into the provided typed struct.
// Use this instead of accessing Data directly to maintain type safety:
//
//	var myEvent MyEventType
//	if err := eventFile.ParseEventData(&myEvent); err != nil {
//	    return err
//	}
func (ef *EventFile) ParseEventData(v interface{}) error {
	return json.Unmarshal(ef.Data, v)
}

// For internal use, maintain the unexported alias
type eventFile = EventFile

// PublishEvent publishes a type-safe event to a stream with compile-time enforcement.
// This is the preferred method - events must implement the Event interface.
// The event is validated before publishing.
// The EventType is preserved in the file for type-safe deserialization.
func (eb *FileEventBus) PublishEvent(stream Stream, event Event) error {
	// Validate the event
	if err := event.Validate(); err != nil {
		eb.logger.WithError(err).Error("Event validation failed",
			"stream", stream.String(),
			"event_type", event.GetEventType().String(),
			"event_id", event.GetEventID(),
		)
		return fmt.Errorf("event validation failed: %w", err)
	}

	streamPath := filepath.Join(eb.basePath, stream.String(), "pending")
	if err := os.MkdirAll(streamPath, 0755); err != nil {
		return fmt.Errorf("failed to create stream directory: %w", err)
	}

	// Use the event's own ID for traceability
	eventID := event.GetEventID()
	timestamp := time.Now().UnixNano()

	// Marshal event data to json.RawMessage to preserve type information
	eventData, err := json.Marshal(event)
	if err != nil {
		eb.logger.WithError(err).Error("Failed to marshal event data",
			"stream", stream.String(),
			"event_type", event.GetEventType().String(),
		)
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	ef := eventFile{
		ID:        eventID,
		Stream:    stream.String(),
		EventType: event.GetEventType(), // Preserve event type for deserialization
		Data:      json.RawMessage(eventData),
		Timestamp: timestamp,
		Retries:   0,
	}

	data, err := json.MarshalIndent(ef, "", "  ")
	if err != nil {
		eb.logger.WithError(err).Error("Failed to marshal event file",
			"stream", stream.String(),
			"event_type", event.GetEventType().String(),
		)
		return fmt.Errorf("failed to marshal event for stream %s: %w", stream.String(), err)
	}

	filename := fmt.Sprintf("%d-%s.json", timestamp, eventID)
	eventPath := filepath.Join(streamPath, filename)

	if err := os.WriteFile(eventPath, data, 0644); err != nil {
		eb.logger.WithError(err).Error("Failed to write event file",
			"stream", stream.String(),
			"path", eventPath,
			"event_type", event.GetEventType().String(),
		)
		return fmt.Errorf("failed to write event to stream %s: %w", stream.String(), err)
	}

	eb.logger.Debug("Event published",
		"stream", stream.String(),
		"id", eventID,
		"event_type", event.GetEventType().String(),
	)
	return nil
}

// PublishEventWithRetry publishes a type-safe event with automatic retry for critical events.
func (eb *FileEventBus) PublishEventWithRetry(stream Stream, event Event, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := eb.PublishEvent(stream, event); err == nil {
			return nil
		} else {
			lastErr = err
			backoff := time.Duration(attempt+1) * 100 * time.Millisecond
			eb.logger.Warn("Publish retry",
				"stream", stream.String(),
				"event_type", event.GetEventType().String(),
				"event_id", event.GetEventID(),
				"attempt", attempt+1,
				"backoff", backoff,
				"error", err,
			)
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("failed to publish after %d retries: %w", maxRetries, lastErr)
}

// PublishValidated is an alias for PublishEvent for backwards compatibility with outbox pattern
func (eb *FileEventBus) PublishValidated(stream Stream, event Event) error {
	return eb.PublishEvent(stream, event)
}

// Publish publishes raw data to a stream (for backwards compatibility with outbox pattern).
// Prefer PublishEvent for type-safe events with compile-time enforcement.
// When using this method, EventType will be set to TypeUnknown since type information
// cannot be inferred from raw data.
func (eb *FileEventBus) Publish(stream Stream, data interface{}) error {
	return eb.PublishWithType(stream, TypeUnknown, "", data)
}

// PublishWithType publishes raw data to a stream with preserved type information.
// This is used by the outbox retry mechanism to preserve EventType across retries.
// The eventID parameter allows preserving the original event ID for traceability.
// For new events, pass an empty eventID and one will be generated.
func (eb *FileEventBus) PublishWithType(stream Stream, eventType EventType, eventID string, data interface{}) error {
	streamPath := filepath.Join(eb.basePath, stream.String(), "pending")
	if err := os.MkdirAll(streamPath, 0755); err != nil {
		return fmt.Errorf("failed to create stream directory: %w", err)
	}

	if eventID == "" {
		eventID = NewEventID()
	}
	timestamp := time.Now().UnixNano()

	// Marshal data to json.RawMessage
	rawData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	ef := eventFile{
		ID:        eventID,
		Stream:    stream.String(),
		EventType: eventType, // Preserve type information for deserialization
		Data:      json.RawMessage(rawData),
		Timestamp: timestamp,
		Retries:   0,
	}

	jsonData, err := json.MarshalIndent(ef, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal event file: %w", err)
	}

	filename := fmt.Sprintf("%d-%s.json", timestamp, eventID)
	eventPath := filepath.Join(streamPath, filename)

	if err := os.WriteFile(eventPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}

// Subscribe subscribes to a stream using fsnotify for immediate notification.
// IMPORTANT: This method BLOCKS until the event bus is shut down. Callers should
// typically run this in a goroutine:
//
//	go func() {
//	    if err := bus.Subscribe(stream, group, consumer, handler); err != nil {
//	        log.Error("subscription error", err)
//	    }
//	}()
//
// For a non-blocking alternative, use SubscribeAsync().
// Handler receives context for cancellation and timeout control.
func (eb *FileEventBus) Subscribe(stream Stream, group, consumer string, handler func(ctx context.Context, data []byte) error) error {
	streamPath := filepath.Join(eb.basePath, stream.String())
	pendingPath := filepath.Join(streamPath, "pending")
	processedPath := filepath.Join(streamPath, "processed")
	dlqPath := filepath.Join(streamPath, "dlq")

	for _, p := range []string{pendingPath, processedPath, dlqPath} {
		if err := os.MkdirAll(p, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", p, err)
		}
	}

	// Create fsnotify watcher for this stream
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	if err := watcher.Add(pendingPath); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	streamKey := stream.String()

	eb.mu.Lock()
	eb.watchers[streamKey] = watcher
	eb.mu.Unlock()

	eb.logger.Info("Consumer subscribed",
		"stream", streamKey,
		"group", group,
		"consumer", consumer,
	)

	eb.workersWg.Add(1)
	defer eb.workersWg.Done()

	// Process any pending events on startup
	if err := eb.processPendingEvents(pendingPath, processedPath, dlqPath, streamKey, handler); err != nil {
		eb.logger.WithError(err).Warn("Error processing pending events on startup", "stream", streamKey)
	}

	// Fallback poll interval for reliability
	fallbackTicker := time.NewTicker(30 * time.Second)
	defer fallbackTicker.Stop()

	// Debounce timer to batch rapid file events
	var debounceTimer *time.Timer
	debounceDuration := 50 * time.Millisecond

	// Cleanup function to properly stop timer and close watcher
	cleanup := func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		watcher.Close()
	}

	for {
		select {
		case <-eb.stopChan:
			eb.logger.Info("Consumer shutting down", "stream", streamKey, "consumer", consumer)
			cleanup()
			return nil

		case <-eb.ctx.Done():
			eb.logger.Info("Consumer context canceled", "stream", streamKey, "consumer", consumer)
			cleanup()
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				cleanup()
				return nil
			}

			// Only process create events for .json files
			if event.Op&fsnotify.Create == fsnotify.Create && filepath.Ext(event.Name) == ".json" {
				// Debounce: reset timer on each event
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounceDuration, func() {
					// Track this callback so Shutdown waits for it
					eb.workersWg.Add(1)
					defer eb.workersWg.Done()

					// Check if we're still running before processing
					select {
					case <-eb.ctx.Done():
						return // Don't process, we're shutting down
					case <-eb.stopChan:
						return // Don't process, we're shutting down
					default:
					}
					if err := eb.processPendingEvents(pendingPath, processedPath, dlqPath, streamKey, handler); err != nil {
						eb.logger.WithError(err).Error("Error processing events", "stream", streamKey)
					}
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				cleanup()
				return nil
			}
			eb.logger.WithError(err).Warn("Watcher error", "stream", streamKey)

		case <-fallbackTicker.C:
			// Fallback poll for reliability
			if err := eb.processPendingEvents(pendingPath, processedPath, dlqPath, streamKey, handler); err != nil {
				eb.logger.WithError(err).Error("Error processing events (fallback poll)", "stream", streamKey)
			}
		}
	}
}

// SubscribeAsync is a non-blocking version of Subscribe that runs the subscription
// in a goroutine. It returns immediately after starting the subscription.
// Errors from the subscription (e.g., during setup) are sent to the returned channel.
// The channel is closed when the subscription ends (either normally or due to shutdown).
func (eb *FileEventBus) SubscribeAsync(stream Stream, group, consumer string, handler func(ctx context.Context, data []byte) error) <-chan error {
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)
		if err := eb.Subscribe(stream, group, consumer, handler); err != nil {
			errChan <- err
		}
	}()

	return errChan
}

// getStreamLock returns the mutex for a stream, creating it if necessary
func (eb *FileEventBus) getStreamLock(stream string) *sync.Mutex {
	lock, _ := eb.streamLock.LoadOrStore(stream, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// processPendingEvents processes all pending events in order
func (eb *FileEventBus) processPendingEvents(pendingPath, processedPath, dlqPath, stream string, handler func(ctx context.Context, data []byte) error) error {
	// Acquire stream lock to prevent concurrent processing
	lock := eb.getStreamLock(stream)
	lock.Lock()
	defer lock.Unlock()
	entries, err := os.ReadDir(pendingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Sort by filename (timestamp prefix ensures order)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		eventPath := filepath.Join(pendingPath, entry.Name())

		data, err := os.ReadFile(eventPath)
		if err != nil {
			eb.logger.WithError(err).Error("Failed to read event file", "path", eventPath)
			continue
		}

		var ef eventFile
		if err := json.Unmarshal(data, &ef); err != nil {
			eb.logger.WithError(err).Error("Failed to unmarshal event file", "path", eventPath)
			// Move to DLQ on unmarshal failure
			if moveErr := eb.moveFile(eventPath, filepath.Join(dlqPath, entry.Name())); moveErr != nil {
				eb.logger.WithError(moveErr).Error("Failed to move corrupt event to DLQ", "path", eventPath)
			}
			continue
		}

		if ef.Retries >= MaxRetries {
			eb.logger.Warn("Message moved to DLQ - max retries exceeded",
				"stream", stream,
				"id", ef.ID,
				"retries", ef.Retries,
			)
			// Move to DLQ on max retries
			if moveErr := eb.moveFile(eventPath, filepath.Join(dlqPath, entry.Name())); moveErr != nil {
				eb.logger.WithError(moveErr).Error("Failed to move exhausted event to DLQ",
					"stream", stream,
					"id", ef.ID,
				)
			}
			continue
		}

		eventData, err := json.Marshal(ef.Data)
		if err != nil {
			eb.logger.WithError(err).Error("Failed to marshal event data", "id", ef.ID)
			continue
		}

		// Create context with timeout - this context is passed to the handler
		processingCtx, cancel := context.WithTimeout(eb.ctx, 30*time.Second)
		err = eb.processWithContext(processingCtx, handler, eventData)
		cancel()

		if err != nil {
			eb.logger.WithError(err).Error("Event handler failed",
				"stream", stream,
				"id", ef.ID,
				"retries", ef.Retries,
			)

			ef.Retries++
			updatedData, marshalErr := json.MarshalIndent(ef, "", "  ")
			if marshalErr != nil {
				eb.logger.WithError(marshalErr).Error("Failed to marshal retry data", "id", ef.ID)
				continue
			}
			if writeErr := os.WriteFile(eventPath, updatedData, 0644); writeErr != nil {
				eb.logger.WithError(writeErr).Error("Failed to write retry data", "id", ef.ID)
			}
			continue
		}

		// Move to processed folder
		if moveErr := eb.moveFile(eventPath, filepath.Join(processedPath, entry.Name())); moveErr != nil {
			eb.logger.WithError(moveErr).Error("Failed to move processed event",
				"stream", stream,
				"id", ef.ID,
			)
		}
		eb.logger.Debug("Event processed", "stream", stream, "id", ef.ID)
	}

	return nil
}

// moveFile moves a file from src to dst atomically
func (eb *FileEventBus) moveFile(src, dst string) error {
	// Check if source exists first
	if _, err := os.Stat(src); os.IsNotExist(err) {
		// File already moved (race condition with another processor)
		return nil
	}

	if err := os.Rename(src, dst); err != nil {
		// If rename fails, check if source still exists
		if os.IsNotExist(err) {
			return nil // Already moved
		}
		// Fallback to copy+delete for cross-filesystem moves
		data, err := os.ReadFile(src)
		if err != nil {
			if os.IsNotExist(err) {
				return nil // Already moved
			}
			return err
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return err
		}
		if err := os.Remove(src); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// processWithContext runs handler with the provided context for cancellation/timeout.
// Handler is responsible for checking ctx.Done() and returning promptly on cancellation.
// This synchronous design avoids goroutine leaks - handlers that ignore context will block,
// but that's a handler bug, not an eventbus bug.
func (eb *FileEventBus) processWithContext(ctx context.Context, handler func(ctx context.Context, data []byte) error, data []byte) error {
	return handler(ctx, data)
}

// GetStreamInfo returns info about a stream
func (eb *FileEventBus) GetStreamInfo(stream Stream) (length int64, groups int64, err error) {
	pendingPath := filepath.Join(eb.basePath, stream.String(), "pending")
	entries, err := os.ReadDir(pendingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	count := int64(0)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			count++
		}
	}

	return count, 1, nil
}

// GetPendingMessages returns number of pending messages
func (eb *FileEventBus) GetPendingMessages(stream Stream, group string) (int64, error) {
	length, _, err := eb.GetStreamInfo(stream)
	return length, err
}

// TrimStream removes old processed events
func (eb *FileEventBus) TrimStream(stream string, maxAge time.Duration) error {
	processedPath := filepath.Join(eb.basePath, stream, "processed")
	entries, err := os.ReadDir(processedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(processedPath, entry.Name())); err != nil {
				eb.logger.WithError(err).Warn("Failed to remove old processed event", "file", entry.Name())
			}
		}
	}

	return nil
}

// HealthCheck verifies FileEventBus is operational
func (eb *FileEventBus) HealthCheck() error {
	_, err := os.Stat(eb.basePath)
	return err
}

// DLQStats represents dead letter queue statistics
type DLQStats struct {
	Stream        string     `json:"stream"`
	MessageCount  int        `json:"message_count"`
	OldestMessage *time.Time `json:"oldest_message,omitempty"`
}

// GetDLQStats returns DLQ statistics for all streams
func (eb *FileEventBus) GetDLQStats() ([]DLQStats, error) {
	var stats []DLQStats

	entries, err := os.ReadDir(eb.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil
		}
		return nil, fmt.Errorf("failed to read events directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		stream := entry.Name()
		dlqPath := filepath.Join(eb.basePath, stream, "dlq")

		dlqEntries, err := os.ReadDir(dlqPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			eb.logger.WithError(err).Warn("Failed to read DLQ directory", "stream", stream)
			continue
		}

		if len(dlqEntries) == 0 {
			continue
		}

		stat := DLQStats{
			Stream:       stream,
			MessageCount: len(dlqEntries),
		}

		// Find oldest message
		var oldestTime time.Time
		for _, dlqEntry := range dlqEntries {
			info, err := dlqEntry.Info()
			if err != nil {
				continue
			}
			if oldestTime.IsZero() || info.ModTime().Before(oldestTime) {
				oldestTime = info.ModTime()
			}
		}
		if !oldestTime.IsZero() {
			stat.OldestMessage = &oldestTime
		}

		stats = append(stats, stat)
	}

	return stats, nil
}

// GetDLQMessages returns messages in the DLQ for a specific stream
func (eb *FileEventBus) GetDLQMessages(stream string, limit int) ([]*eventFile, error) {
	dlqPath := filepath.Join(eb.basePath, stream, "dlq")

	entries, err := os.ReadDir(dlqPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*eventFile{}, nil
		}
		return nil, fmt.Errorf("failed to read DLQ directory: %w", err)
	}

	var messages []*eventFile
	for i, entry := range entries {
		if i >= limit {
			break
		}

		if entry.IsDir() {
			continue
		}

		eventPath := filepath.Join(dlqPath, entry.Name())
		data, err := os.ReadFile(eventPath)
		if err != nil {
			eb.logger.WithError(err).Warn("Failed to read DLQ message", "file", entry.Name())
			continue
		}

		var ef eventFile
		if err := json.Unmarshal(data, &ef); err != nil {
			eb.logger.WithError(err).Warn("Failed to parse DLQ message", "file", entry.Name())
			continue
		}

		messages = append(messages, &ef)
	}

	return messages, nil
}

// ReprocessDLQMessage moves a message from DLQ back to pending for reprocessing
func (eb *FileEventBus) ReprocessDLQMessage(stream, messageID string) error {
	dlqPath := filepath.Join(eb.basePath, stream, "dlq")
	pendingPath := filepath.Join(eb.basePath, stream, "pending")

	// Find the message file
	entries, err := os.ReadDir(dlqPath)
	if err != nil {
		return fmt.Errorf("failed to read DLQ directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		eventPath := filepath.Join(dlqPath, entry.Name())
		data, err := os.ReadFile(eventPath)
		if err != nil {
			continue
		}

		var ef eventFile
		if err := json.Unmarshal(data, &ef); err != nil {
			continue
		}

		if ef.ID == messageID {
			// Reset retry count and move back to pending
			ef.Retries = 0
			ef.Timestamp = time.Now().UnixNano()

			newData, err := json.Marshal(ef)
			if err != nil {
				return fmt.Errorf("failed to marshal event: %w", err)
			}

			newPath := filepath.Join(pendingPath, entry.Name())
			if err := os.WriteFile(newPath, newData, 0644); err != nil {
				return fmt.Errorf("failed to write to pending: %w", err)
			}

			if err := os.Remove(eventPath); err != nil {
				eb.logger.WithError(err).Warn("Failed to remove DLQ message after requeue", "file", entry.Name())
			}

			eb.logger.Info("Reprocessed DLQ message", "stream", stream, "id", messageID)
			return nil
		}
	}

	return fmt.Errorf("message not found in DLQ: %s", messageID)
}

// Shutdown gracefully shuts down the EventBus
func (eb *FileEventBus) Shutdown(timeout time.Duration) error {
	var shutdownErr error
	eb.shutdownOnce.Do(func() {
		eb.logger.Info("FileEventBus shutting down...")

		eb.cancel()
		close(eb.stopChan)

		// Close all watchers
		eb.mu.Lock()
		for _, w := range eb.watchers {
			w.Close()
		}
		eb.watchers = make(map[string]*fsnotify.Watcher)
		eb.mu.Unlock()

		done := make(chan struct{})
		go func() {
			eb.workersWg.Wait()
			close(done)
		}()

		select {
		case <-done:
			eb.logger.Info("FileEventBus shutdown complete")
		case <-time.After(timeout):
			eb.logger.Warn("FileEventBus shutdown timed out, forcing close")
			shutdownErr = fmt.Errorf("shutdown timed out after %v", timeout)
		}
	})
	return shutdownErr
}

// Close closes the event bus with a default timeout
// This provides compatibility with io.Closer interface
func (eb *FileEventBus) Close() error {
	return eb.Shutdown(30 * time.Second)
}
