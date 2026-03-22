// Package workers provides the embedded worker manager for event processing.
package workers

import (
	"context"
	"fmt"
	"sync"
	"time"

	automationhandlers "github.com/movebigrocks/platform/internal/automation/handlers"
	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	servicehandlers "github.com/movebigrocks/platform/internal/service/handlers"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// Manager manages embedded worker goroutines for event processing.
// It consolidates all event handlers into a single process, using
// an in-memory event bus for event dispatching.
type Manager struct {
	eventBus         eventbus.EventBus
	logger           *logger.Logger
	idempotencyStore eventbus.IdempotencyStore

	// Handlers
	ruleEvalHandler *automationhandlers.RuleEvaluationHandler
	jobHandler      *automationhandlers.JobEventHandler
	formHandler     *servicehandlers.FormEventHandler
	// Lifecycle
	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// ManagerDeps holds dependencies for creating the worker manager
type ManagerDeps struct {
	EventBus         eventbus.EventBus
	Logger           *logger.Logger
	IdempotencyStore eventbus.IdempotencyStore

	// Services needed by handlers
	RulesEngine *automationservices.RulesEngine
	JobService  *automationservices.JobService
	FormService *automationservices.FormService
	CaseService *serviceapp.CaseService
	Outbox      contracts.OutboxPublisher
	TxRunner    contracts.TransactionRunner
}

// NewManager creates a new embedded worker manager.
func NewManager(deps ManagerDeps) *Manager {
	if deps.Logger == nil {
		deps.Logger = logger.NewNop()
	}

	m := &Manager{
		eventBus:         deps.EventBus,
		logger:           deps.Logger.WithField("component", "worker-manager"),
		idempotencyStore: deps.IdempotencyStore,
		stopCh:           make(chan struct{}),
	}

	// Create rule evaluation handler
	if deps.RulesEngine != nil && deps.CaseService != nil {
		m.ruleEvalHandler = automationhandlers.NewRuleEvaluationHandler(
			deps.RulesEngine,
			deps.CaseService,
			deps.Logger,
		)
	}

	// Create job handler
	if deps.JobService != nil {
		m.jobHandler = automationhandlers.NewJobEventHandler(
			deps.JobService,
			deps.Logger,
		)
	}

	if deps.FormService != nil && deps.CaseService != nil && deps.Outbox != nil && deps.TxRunner != nil {
		m.formHandler = servicehandlers.NewFormEventHandler(
			deps.FormService,
			deps.CaseService,
			deps.Outbox,
			deps.TxRunner,
			deps.Logger,
		)
	}

	return m
}

// Start starts all embedded workers and registers event handlers.
// The outbox worker handles dispatching events to the in-memory EventBus.
// Safe to call multiple times - only the first call takes effect.
func (m *Manager) Start(ctx context.Context) error {
	var startErr error
	m.startOnce.Do(func() {
		m.logger.Info("Starting embedded worker manager")

		// Register all event handlers with the in-memory EventBus
		if err := m.registerHandlers(); err != nil {
			startErr = fmt.Errorf("failed to register handlers: %w", err)
			return
		}

		// Note: The outbox worker handles dispatching events to the EventBus.
		// No polling loop needed here - handlers are invoked when outbox worker
		// calls EventBus.Dispatch() for each pending event.

		m.logger.Info("Embedded worker manager started successfully")
	})
	return startErr
}

// Stop gracefully stops all workers with the given timeout.
func (m *Manager) Stop(timeout time.Duration) error {
	var err error
	m.stopOnce.Do(func() {
		m.logger.Info("Stopping embedded worker manager")
		close(m.stopCh)

		// Wait for workers with timeout
		done := make(chan struct{})
		go func() {
			m.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			m.logger.Info("All workers stopped gracefully")
		case <-time.After(timeout):
			m.logger.Warn("Worker shutdown timed out", "timeout", timeout)
			err = fmt.Errorf("worker shutdown timed out after %v", timeout)
		}
	})
	return err
}

// registerHandlers registers all event handlers with the event bus
func (m *Manager) registerHandlers() error {
	// Wrap subscribe to add database-backed idempotency to all handlers
	subscribe := func(stream eventbus.Stream, group, consumer string, handler func(context.Context, []byte) error) error {
		var idempotentHandler eventbus.Handler
		if m.idempotencyStore != nil {
			idempotentHandler = eventbus.WithDBIdempotency(m.idempotencyStore, group, handler)
		} else {
			idempotentHandler = handler
		}
		return m.eventBus.Subscribe(stream, group, consumer, idempotentHandler)
	}

	// Register rule evaluation handler
	if m.ruleEvalHandler != nil {
		if err := m.ruleEvalHandler.RegisterHandlers(subscribe); err != nil {
			return fmt.Errorf("register rule evaluation handlers: %w", err)
		}
		m.logger.Info("Rule evaluation handlers registered")
	}

	// Register job handler
	if m.jobHandler != nil {
		if err := m.jobHandler.RegisterHandlers(subscribe); err != nil {
			return fmt.Errorf("register job handlers: %w", err)
		}
		m.logger.Info("Job handlers registered")
	}

	if m.formHandler != nil {
		if err := m.formHandler.RegisterHandlers(subscribe); err != nil {
			return fmt.Errorf("register form handlers: %w", err)
		}
		m.logger.Info("Form event handlers registered")
	}

	return nil
}
