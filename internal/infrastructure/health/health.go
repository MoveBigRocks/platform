package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	"github.com/movebigrocks/platform/pkg/logger"
)

// ComponentStatus represents the health status of a component
type ComponentStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "healthy", "degraded", "unhealthy"
	Message string `json:"message,omitempty"`
	Latency int64  `json:"latency_ms,omitempty"`
}

// BuildInfo contains version and build information
type BuildInfo struct {
	Version    string `json:"version"`
	GitCommit  string `json:"git_commit,omitempty"`
	BuildDate  string `json:"build_date,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
}

// HealthResponse represents the overall health check response
type HealthResponse struct {
	Status     string            `json:"status"` // "healthy", "degraded", "unhealthy"
	Timestamp  time.Time         `json:"timestamp"`
	Components []ComponentStatus `json:"components"`
	Version    string            `json:"version,omitempty"`
	Build      *BuildInfo        `json:"build,omitempty"`
}

// Checker performs health checks on system components
type Checker struct {
	store     stores.Store
	eventBus  eventbus.EventBus
	logger    *logger.Logger
	buildInfo *BuildInfo
}

// NewCheckerWithBuildInfo creates a health checker with full build information
func NewCheckerWithBuildInfo(store stores.Store, eventBus eventbus.EventBus, log *logger.Logger, version, gitCommit, buildDate, instanceID string) *Checker {
	return &Checker{
		store:    store,
		eventBus: eventBus,
		logger:   log,
		buildInfo: &BuildInfo{
			Version:    version,
			GitCommit:  gitCommit,
			BuildDate:  buildDate,
			InstanceID: instanceID,
		},
	}
}

// Check performs a comprehensive health check
func (h *Checker) Check(ctx context.Context) *HealthResponse {
	components := make([]ComponentStatus, 0, 4)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Check storage
	wg.Add(1)
	go func() {
		defer wg.Done()
		status := h.checkStorage(ctx)
		mu.Lock()
		components = append(components, status)
		mu.Unlock()
	}()

	// Check event bus
	if h.eventBus != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := h.checkEventBus(ctx)
			mu.Lock()
			components = append(components, status)
			mu.Unlock()
		}()
	}

	// Check outbox
	wg.Add(1)
	go func() {
		defer wg.Done()
		status := h.checkOutbox(ctx)
		mu.Lock()
		components = append(components, status)
		mu.Unlock()
	}()

	wg.Wait()

	// Determine overall status
	overallStatus := "healthy"
	for _, c := range components {
		if c.Status == "unhealthy" {
			overallStatus = "unhealthy"
			break
		} else if c.Status == "degraded" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
		}
	}

	return &HealthResponse{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Components: components,
		Version:    h.buildInfo.Version,
		Build:      h.buildInfo,
	}
}

// CheckReadiness performs a readiness check
func (h *Checker) CheckReadiness(ctx context.Context) *HealthResponse {
	// For readiness, we just check critical components
	components := make([]ComponentStatus, 0, 2)

	// Storage must be ready
	components = append(components, h.checkStorage(ctx))

	overallStatus := "healthy"
	for _, c := range components {
		if c.Status == "unhealthy" {
			overallStatus = "unhealthy"
			break
		}
	}

	return &HealthResponse{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Components: components,
		Version:    h.buildInfo.Version,
		Build:      h.buildInfo,
	}
}

func (h *Checker) checkStorage(ctx context.Context) ComponentStatus {
	start := time.Now()
	status := ComponentStatus{
		Name:   "storage",
		Status: "healthy",
	}

	if h.store == nil {
		status.Status = "unhealthy"
		status.Message = "storage not initialized"
		return status
	}

	err := h.store.HealthCheck(ctx)
	status.Latency = time.Since(start).Milliseconds()

	if err != nil {
		status.Status = "unhealthy"
		status.Message = err.Error()
		return status
	}

	// Check if latency is high
	if status.Latency > 100 {
		status.Status = "degraded"
		status.Message = "high latency"
	}

	return status
}

func (h *Checker) checkEventBus(ctx context.Context) ComponentStatus {
	start := time.Now()
	status := ComponentStatus{
		Name:   "eventbus",
		Status: "healthy",
	}

	if h.eventBus == nil {
		status.Status = "unhealthy"
		status.Message = "event bus not initialized"
		return status
	}

	err := h.eventBus.HealthCheck()
	status.Latency = time.Since(start).Milliseconds()

	if err != nil {
		status.Status = "unhealthy"
		status.Message = err.Error()
		return status
	}

	return status
}

func (h *Checker) checkOutbox(ctx context.Context) ComponentStatus {
	start := time.Now()
	status := ComponentStatus{
		Name:   "outbox",
		Status: "healthy",
	}

	if h.store == nil || h.store.Outbox() == nil {
		status.Status = "unhealthy"
		status.Message = "outbox store not initialized"
		return status
	}

	// Check pending outbox events
	events, err := h.store.Outbox().GetPendingOutboxEvents(ctx, 1000)
	status.Latency = time.Since(start).Milliseconds()

	if err != nil {
		status.Status = "unhealthy"
		status.Message = fmt.Sprintf("failed to check outbox: %v", err)
		return status
	}

	// Warn if too many pending events
	if len(events) > 500 {
		status.Status = "degraded"
		status.Message = fmt.Sprintf("%d events pending in outbox", len(events))
	} else if len(events) > 100 {
		status.Message = fmt.Sprintf("%d events pending", len(events))
	}

	return status
}
