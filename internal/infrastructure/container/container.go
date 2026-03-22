package container

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/health"
	"github.com/movebigrocks/platform/internal/infrastructure/metrics"
	"github.com/movebigrocks/platform/internal/infrastructure/outbox"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	"github.com/movebigrocks/platform/internal/infrastructure/tracing"
	"github.com/movebigrocks/platform/internal/shared/geoip"
	"github.com/movebigrocks/platform/internal/workers"

	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// Container holds all application dependencies.
// It provides centralized initialization and lifecycle management.
//
// Domain services are organized into sub-containers by bounded context:
//   - Platform: User, workspace, session, agent, contact services
//   - Service: Case, email, attachment services
//   - Automation: Rule, form services (depends on Platform and Service)
//   - Observability: Issue, project, error processing services
type Container struct {
	// Core infrastructure (shared across all domains)
	Config          *config.Config
	Logger          *logger.Logger
	Store           stores.Store
	EventBus        eventbus.EventBus
	Outbox          *outbox.Service
	Tracer          trace.Tracer
	TracingShutdown tracing.ShutdownFunc

	// Database metrics collection
	DBStatsCollector *metrics.DBStatsCollector

	// Shared services
	GeoIP geoip.Service

	// Domain containers (grouped by bounded context)
	Platform      *PlatformContainer
	Service       *ServiceContainer
	Automation    *AutomationContainer
	Observability *ObservabilityContainer

	// Embedded workers
	WorkerManager *workers.Manager

	// Health
	HealthChecker *health.Checker
}

// Options configures the container initialization.
type Options struct {
	Version   string
	GitCommit string
	BuildDate string
}

// New creates a new container with all dependencies wired.
// Initialization order enforces the dependency graph:
//  1. Infrastructure (tracing, database, store, event bus, outbox)
//  2. Platform (no domain dependencies)
//  3. Service (no domain dependencies)
//  4. Automation (depends on Platform.Contact and Service.Case)
//  5. Observability (no domain dependencies)
//  6. Health checker
//
// Architecture: EventBus is purely in-memory for dispatch. The outbox handles
// durability by saving events to the database, and its worker dispatches to
// the in-memory EventBus.
func New(cfg *config.Config, opts Options) (*Container, error) {
	c := &Container{
		Config: cfg,
		Logger: logger.New().WithField("service", "api"),
	}

	// Initialize infrastructure first
	if err := c.initTracing(); err != nil {
		c.Logger.Warn("Failed to init tracing, continuing without it", "error", err)
	}

	// Database first - independent of EventBus
	if err := c.initStore(); err != nil {
		return nil, fmt.Errorf("failed to init store: %w", err)
	}

	// EventBus is in-memory only - outbox worker handles dispatch
	c.initEventBus()

	c.initOutbox()

	// Initialize domain containers in dependency order
	// Platform has no domain dependencies
	c.Platform = NewPlatformContainer(c.Store, c.Config, c.Logger)

	// Service has no domain dependencies
	service, err := NewServiceContainer(ServiceContainerDeps{
		Store:  c.Store,
		Outbox: c.Outbox,
		Config: c.Config,
		Logger: c.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init service container: %w", err)
	}
	c.Service = service

	// Automation depends on Platform.Contact and Service.Case (explicit in deps)
	c.Automation = NewAutomationContainer(AutomationContainerDeps{
		Store:     c.Store,
		Outbox:    c.Outbox,
		Case:      c.Service.Case,     // Dependency on Service domain
		Contact:   c.Platform.Contact, // Dependency on Platform domain
		Extension: c.Platform.Extension,
	})

	// Observability has no domain dependencies
	c.Observability = NewObservabilityContainer(c.Store, c.Outbox, c.Config.ErrorProcessing)
	if c.Platform != nil && c.Platform.Workspace != nil && c.Observability != nil {
		c.Platform.Workspace.SetIssueChecker(c.Observability.Issue)
	}
	if c.Platform != nil && c.Platform.Stats != nil && c.Observability != nil {
		c.Platform.Stats.SetIssueMetricsProvider(c.Observability.Issue)
	}

	// GeoIP service (shared, used by analytics and potentially other modules)
	var geo geoip.Service
	geoIPPath := cfg.GeoIPDBPath
	if geoIPPath != "" {
		geo, err = geoip.NewMaxMindService(geoIPPath)
		if err != nil {
			c.Logger.Warn("Failed to load GeoIP database, falling back to noop", "path", geoIPPath, "error", err)
			geo = geoip.NewNoopService()
		} else {
			c.Logger.Info("GeoIP database loaded", "path", geoIPPath)
		}
	} else {
		geo = geoip.NewNoopService()
	}
	c.GeoIP = geo

	// Initialize embedded worker manager
	c.WorkerManager = workers.NewManager(workers.ManagerDeps{
		EventBus:         c.EventBus,
		Logger:           c.Logger,
		IdempotencyStore: c.Store.Idempotency(),
		RulesEngine:      c.Automation.Engine,
		FormService:      c.Automation.Form,
		CaseService:      c.Service.Case,
		Outbox:           c.Outbox,
		TxRunner:         c.Store,
	})

	c.initHealth(opts)

	return c, nil
}

func (c *Container) initTracing() error {
	tracer, shutdown, err := tracing.Init(c.Config.Tracing)
	if err != nil {
		return fmt.Errorf("failed to initialize tracing: %w", err)
	}
	c.Tracer = tracer
	c.TracingShutdown = shutdown

	if c.Config.Tracing.Enabled {
		c.Logger.Info("Distributed tracing initialized",
			"exporter", c.Config.Tracing.Exporter,
			"sample_rate", c.Config.Tracing.SampleRate,
		)
	}

	return nil
}

func (c *Container) initStore() error {
	c.Logger.Info("Creating database",
		"driver", c.Config.Database.EffectiveDriver(),
		"dsn", c.Config.Database.RedactedDSN(),
	)

	if c.Config.DatabasePool.MaxOpenConns <= 1 {
		c.Logger.Warn("Outbox is enabled; single DB connection can cause event/deadlock contention",
			"max_open_conns", c.Config.DatabasePool.MaxOpenConns,
			"recommendation", "set DATABASE_MAX_OPEN_CONNS > 1")
	}

	// Create database connection with configured connection pool settings
	db, err := sql.NewDBWithConfig(sql.DBConfig{
		DSN:             c.Config.Database.EffectiveDSN(),
		MaxOpenConns:    c.Config.DatabasePool.MaxOpenConns,
		MaxIdleConns:    c.Config.DatabasePool.MaxIdleConns,
		ConnMaxLifetime: c.Config.DatabasePool.ConnMaxLifetime,
		ConnMaxIdleTime: c.Config.DatabasePool.ConnMaxIdleTime,
	})
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	store, err := sql.NewStore(db)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	c.Store = store
	c.Logger.Info("Store initialized")

	// Set up database connection pool metrics collector
	sqlDB, _ := db.GetSQLDB()
	c.DBStatsCollector = metrics.NewDBStatsCollector(sqlDB, "api", 15*time.Second)
	metrics.RegisterDBStatsCollector(c.DBStatsCollector)
	c.Logger.Info("Database metrics collector initialized", "interval", "15s")

	return nil
}

func (c *Container) initEventBus() {
	// Create in-memory EventBus - purely for dispatch
	// Durability is handled by the outbox service
	c.EventBus = eventbus.NewInMemoryBus()
	c.Logger.Info("In-memory EventBus initialized")
}

func (c *Container) initOutbox() {
	c.Outbox = outbox.NewServiceWithConfig(c.Store, c.EventBus, c.Logger, c.Config.Outbox)
	c.Logger.Info("Outbox service initialized",
		"poll_interval", c.Config.Outbox.PollInterval,
		"max_retries", c.Config.Outbox.MaxRetries,
		"batch_size", c.Config.Outbox.BatchSize,
	)
}

func (c *Container) initHealth(opts Options) {
	c.HealthChecker = health.NewCheckerWithBuildInfo(
		c.Store,
		c.EventBus,
		c.Logger,
		opts.Version,
		opts.GitCommit,
		opts.BuildDate,
		c.Config.InstanceID,
	)
}

// Start starts all background services.
// Use StartWithHealthCheck for production to verify health after startup.
func (c *Container) Start(ctx context.Context) error {
	// Start outbox
	c.Outbox.Start()
	c.Logger.Info("Outbox service started")

	// Start database metrics collector
	if c.DBStatsCollector != nil {
		c.DBStatsCollector.Start()
		c.Logger.Info("Database metrics collector started")
	}

	// Start error processing workers
	if err := c.Observability.StartWorkers(ctx); err != nil {
		c.Logger.Warn("Failed to start error processing workers", "error", err)
	} else {
		c.Logger.Info("Started error monitoring processing workers",
			"worker_count", c.Config.ErrorProcessing.WorkerCount,
			"queue_size", c.Config.ErrorProcessing.QueueSize,
		)
	}

	// Start embedded worker manager (event handlers)
	if c.WorkerManager != nil {
		if err := c.WorkerManager.Start(ctx); err != nil {
			return fmt.Errorf("failed to start worker manager: %w", err)
		}
		c.Logger.Info("Embedded worker manager started")
	}

	return nil
}

// StartWithHealthCheck starts all background services and verifies health.
// This should be used in production to catch startup issues before accepting traffic.
// Returns an error if critical services (database, event bus) are unhealthy.
func (c *Container) StartWithHealthCheck(ctx context.Context) error {
	// Start all services
	if err := c.Start(ctx); err != nil {
		return fmt.Errorf("start services: %w", err)
	}

	// Allow services a moment to initialize
	time.Sleep(100 * time.Millisecond)

	// Validate critical services are healthy
	if c.HealthChecker != nil {
		healthStatus := c.HealthChecker.Check(ctx)

		// Log health status
		c.Logger.Info("Post-startup health check",
			"status", healthStatus.Status,
			"components", len(healthStatus.Components))

		// Fail startup if critical services are unhealthy
		if healthStatus.Status == "unhealthy" {
			var failedServices []string
			for _, component := range healthStatus.Components {
				if component.Status == "unhealthy" {
					failedServices = append(failedServices, component.Name)
					c.Logger.Error("Service unhealthy after startup",
						"service", component.Name,
						"status", component.Status,
						"message", component.Message)
				}
			}
			return fmt.Errorf("critical services unhealthy after startup: %v", failedServices)
		}
	}

	c.Logger.Info("All services started and healthy")
	return nil
}

// Stop gracefully shuts down all services.
func (c *Container) Stop(timeout time.Duration) error {
	c.Logger.Info("Stopping services...")

	// Stop embedded worker manager first (event handlers)
	if c.WorkerManager != nil {
		if err := c.WorkerManager.Stop(timeout); err != nil {
			c.Logger.Error("Error stopping worker manager", "error", err)
		} else {
			c.Logger.Info("Embedded worker manager stopped")
		}
	}

	// Close GeoIP database
	if c.GeoIP != nil {
		if err := c.GeoIP.Close(); err != nil {
			c.Logger.Error("Error closing GeoIP database", "error", err)
		}
	}

	// Stop observability workers
	if err := c.Observability.StopWorkers(); err != nil {
		c.Logger.Error("Error stopping error processing workers", "error", err)
	}

	// Stop database metrics collector
	if c.DBStatsCollector != nil {
		c.DBStatsCollector.Stop()
		metrics.UnregisterDBStatsCollector("api")
		c.Logger.Info("Database metrics collector stopped")
	}

	// Stop outbox
	if err := c.Outbox.Stop(timeout); err != nil {
		c.Logger.Error("Error stopping outbox service", "error", err)
	} else {
		c.Logger.Info("Outbox service stopped")
	}

	// Close event bus
	if err := c.EventBus.Close(); err != nil {
		c.Logger.Error("Error closing event bus", "error", err)
	} else {
		c.Logger.Info("Event bus closed")
	}

	// Close platform services (stops background cleanup goroutine)
	c.Platform.Close()
	c.Logger.Info("Platform services closed")

	// Stop automation services (stops rate limiter cleanup worker)
	c.Automation.Stop()
	c.Logger.Info("Automation services stopped")

	// Close database connection pool to prevent connection leaks
	if err := c.Store.Close(); err != nil {
		c.Logger.Error("Error closing database connection pool", "error", err)
	} else {
		c.Logger.Info("Database connection pool closed")
	}

	// Shutdown tracing to flush remaining spans
	if c.TracingShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.TracingShutdown(ctx); err != nil {
			c.Logger.Error("Error shutting down tracing", "error", err)
		} else {
			c.Logger.Info("Tracing shutdown complete")
		}
	}

	return nil
}
