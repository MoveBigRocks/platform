package container

import (
	"context"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/outbox"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	observabilityservices "github.com/movebigrocks/platform/internal/observability/services"
)

// ObservabilityContainer holds observability domain services.
// Observability services depend only on infrastructure (no other domain services).
type ObservabilityContainer struct {
	Issue          *observabilityservices.IssueService
	Project        *observabilityservices.ProjectService
	ErrorGrouping  *observabilityservices.ErrorGroupingService
	ErrorProcessor *observabilityservices.ErrorProcessor

	// Store config for StartWorkers
	errorProcessingConfig config.ErrorProcessingConfig
}

// NewObservabilityContainer creates a new observability container with all observability services.
func NewObservabilityContainer(store stores.Store, outbox *outbox.Service, cfg config.ErrorProcessingConfig) *ObservabilityContainer {
	c := &ObservabilityContainer{
		errorProcessingConfig: cfg,
	}

	c.Issue = observabilityservices.NewIssueService(
		store.Issues(),
		store.Projects(),
		store.ErrorEvents(),
		store.Workspaces(),
		outbox,
	)

	c.Project = observabilityservices.NewProjectService(
		store.Projects(),
		store.Workspaces(),
	)

	c.ErrorGrouping = observabilityservices.NewErrorGroupingService(
		store.Issues(),
		store.Projects(),
		outbox,
	)

	c.ErrorProcessor = observabilityservices.NewErrorProcessorFromConfig(c.ErrorGrouping, cfg)

	return c
}

// StartWorkers starts the error processing workers using the configured worker count.
func (o *ObservabilityContainer) StartWorkers(ctx context.Context) error {
	return o.ErrorProcessor.StartWorkers(ctx, o.errorProcessingConfig.WorkerCount)
}

// StopWorkers stops the error processing workers.
func (o *ObservabilityContainer) StopWorkers() error {
	if o.ErrorProcessor != nil {
		return o.ErrorProcessor.StopWorkers()
	}
	return nil
}
