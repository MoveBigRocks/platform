package observabilityservices

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
)

// ErrorProcessor handles background processing of error events
// Uses injected services for all store operations
type ErrorProcessor struct {
	groupingService *ErrorGroupingService
	logger          *log.Logger

	// Worker pool management
	workerCount int
	workers     []*EventWorker
	eventQueue  chan *observabilitydomain.ErrorEvent
	stopChan    chan struct{}
	wg          sync.WaitGroup
	isRunning   atomic.Bool // Thread-safe running state
	mu          sync.Mutex  // Protects workers slice and workerCount during start/stop

	// In-memory processing stats
	successCount int64
	failedCount  int64
}

// EventWorker represents a background worker for processing events
type EventWorker struct {
	id        int
	processor *ErrorProcessor
	stopChan  chan struct{}
	logger    *log.Logger
}

// ProcessingResult represents the result of event processing
type ProcessingResult struct {
	EventID     string        `json:"event_id"`
	IssueID     string        `json:"issue_id"`
	IsNewIssue  bool          `json:"is_new_issue"`
	ProcessTime time.Duration `json:"process_time"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
}

// ProcessingStats represents processing statistics
type ProcessingStats struct {
	TotalProcessed        int64         `json:"total_processed"`
	SuccessfullyProcessed int64         `json:"successfully_processed"`
	Failed                int64         `json:"failed"`
	AverageProcessTime    time.Duration `json:"average_process_time"`
	QueueDepth            int           `json:"queue_depth"`
	WorkerCount           int           `json:"worker_count"`
	IsRunning             bool          `json:"is_running"`
}

// NewErrorProcessorFromConfig creates an error processor with configuration from config package.
func NewErrorProcessorFromConfig(
	groupingService *ErrorGroupingService,
	cfg config.ErrorProcessingConfig,
) *ErrorProcessor {
	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = 4
	}
	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 1000
	}

	return &ErrorProcessor{
		groupingService: groupingService,
		logger:          log.New(log.Writer(), "[ErrorProcessor] ", log.LstdFlags),
		workerCount:     workerCount,
		eventQueue:      make(chan *observabilitydomain.ErrorEvent, queueSize),
		stopChan:        make(chan struct{}),
	}
}

// StartWorkers starts the background worker pool
func (e *ErrorProcessor) StartWorkers(ctx context.Context, workerCount int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isRunning.Load() {
		return fmt.Errorf("workers already running")
	}

	// Recreate stopChan to support restart after StopWorkers
	// (closed channels cannot be reused)
	e.stopChan = make(chan struct{})

	e.workerCount = workerCount
	e.workers = make([]*EventWorker, workerCount)

	// Start workers
	for i := 0; i < workerCount; i++ {
		worker := &EventWorker{
			id:        i,
			processor: e,
			stopChan:  make(chan struct{}),
			logger:    log.New(log.Writer(), fmt.Sprintf("[Worker-%d] ", i), log.LstdFlags),
		}
		e.workers[i] = worker

		e.wg.Add(1)
		go worker.Start(ctx)
	}

	// Start queue monitor
	e.wg.Add(1)
	go e.queueMonitor(ctx)

	e.isRunning.Store(true)
	e.logger.Printf("Started %d workers for event processing", workerCount)

	return nil
}

// StopWorkers stops all background workers
func (e *ErrorProcessor) StopWorkers() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning.Load() {
		return fmt.Errorf("workers not running")
	}

	// Signal all workers to stop
	close(e.stopChan)

	// Stop individual workers
	for _, worker := range e.workers {
		close(worker.stopChan)
	}

	// Wait for all workers to finish
	e.wg.Wait()

	e.isRunning.Store(false)
	e.logger.Printf("Stopped all workers")

	return nil
}

// ProcessEvent queues an event for background processing
func (e *ErrorProcessor) ProcessEvent(ctx context.Context, event *observabilitydomain.ErrorEvent) error {
	if !e.isRunning.Load() {
		// Process synchronously if workers not running
		return e.processEventSync(ctx, event)
	}

	select {
	case e.eventQueue <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue is full, process synchronously or drop
		e.logger.Printf("Event queue full, processing synchronously: %s", event.EventID)
		return e.processEventSync(ctx, event)
	}
}

// processEventSync processes an event synchronously
func (e *ErrorProcessor) processEventSync(ctx context.Context, event *observabilitydomain.ErrorEvent) error {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		e.logger.Printf("Processed event %s in %v", event.EventID, duration)
	}()

	// Mark event as being processed
	event.ProcessedAt = &startTime

	// 1. Group the event into an issue
	_, _, err := e.groupingService.GroupEvent(ctx, event)
	if err != nil {
		e.logger.Printf("Failed to group event %s: %v", event.EventID, err)
		return fmt.Errorf("grouping failed: %w", err)
	}

	// Mark event as grouped
	now := time.Now()
	event.GroupedAt = &now

	// 2. Metrics are now event-driven (handled by event handlers)

	// 3. Update processing cache for statistics
	e.updateProcessingStats(ctx, true, time.Since(startTime))

	return nil
}

// updateProcessingStats updates processing statistics using atomic counters
func (e *ErrorProcessor) updateProcessingStats(ctx context.Context, success bool, duration time.Duration) {
	if success {
		atomic.AddInt64(&e.successCount, 1)
	} else {
		atomic.AddInt64(&e.failedCount, 1)
	}
}

// GetProcessingStats returns current processing statistics
func (e *ErrorProcessor) GetProcessingStats(ctx context.Context) (*ProcessingStats, error) {
	successCount := atomic.LoadInt64(&e.successCount)
	failedCount := atomic.LoadInt64(&e.failedCount)

	stats := &ProcessingStats{
		QueueDepth:            len(e.eventQueue),
		WorkerCount:           e.workerCount,
		IsRunning:             e.isRunning.Load(),
		SuccessfullyProcessed: successCount,
		Failed:                failedCount,
		TotalProcessed:        successCount + failedCount,
	}

	return stats, nil
}

// queueMonitor monitors the event queue and logs statistics
func (e *ErrorProcessor) queueMonitor(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats, err := e.GetProcessingStats(ctx)
			if err == nil {
				e.logger.Printf("Queue depth: %d, Workers: %d, Processed: %d (Success: %d, Failed: %d)",
					stats.QueueDepth, stats.WorkerCount, stats.TotalProcessed,
					stats.SuccessfullyProcessed, stats.Failed)
			}
		case <-e.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// EventWorker methods

// Start starts the worker processing loop
func (w *EventWorker) Start(ctx context.Context) {
	defer w.processor.wg.Done()
	w.logger.Printf("Worker started")

	for {
		select {
		case event := <-w.processor.eventQueue:
			w.processEvent(ctx, event)
		case <-w.stopChan:
			w.logger.Printf("Worker stopping")
			return
		case <-ctx.Done():
			w.logger.Printf("Worker stopping due to context cancellation")
			return
		}
	}
}

// processEvent processes a single event
func (w *EventWorker) processEvent(ctx context.Context, event *observabilitydomain.ErrorEvent) {
	startTime := time.Now()

	defer func() {
		if r := recover(); r != nil {
			w.logger.Printf("Panic while processing event %s: %v", event.EventID, r)
			w.processor.updateProcessingStats(ctx, false, time.Since(startTime))
		}
	}()

	if err := w.processor.processEventSync(ctx, event); err != nil {
		w.logger.Printf("Failed to process event %s: %v", event.EventID, err)
		w.processor.updateProcessingStats(ctx, false, time.Since(startTime))
	}
}
