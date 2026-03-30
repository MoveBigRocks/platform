package extensionruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

type Runtime struct {
	registry       *Registry
	eventBus       eventbus.EventBus
	extensionStore shared.ExtensionStore
	workspaceStore shared.WorkspaceStore
	logger         *logger.Logger

	mu                sync.Mutex
	bootstrapStates   map[string]runtimeBootstrapState
	endpointStates    map[string]platformdomain.ExtensionRuntimeEndpointState
	consumerKeys      map[string]struct{}
	consumerStates    map[string]platformdomain.ExtensionRuntimeConsumerState
	scheduledJobStops map[string]context.CancelFunc
	jobStates         map[string]platformdomain.ExtensionRuntimeJobState
	rootCtx           context.Context
	rootCancel        context.CancelFunc
}

type runtimeBootstrapState struct {
	Status          string
	LastBootstrapAt *time.Time
	LastError       string
}

type RuntimeOption func(*Runtime)

func WithBackgroundRuntimeDeps(
	eventBus eventbus.EventBus,
	extensionStore shared.ExtensionStore,
	workspaceStore shared.WorkspaceStore,
	log *logger.Logger,
) RuntimeOption {
	return func(runtime *Runtime) {
		runtime.eventBus = eventBus
		runtime.extensionStore = extensionStore
		runtime.workspaceStore = workspaceStore
		runtime.logger = log
	}
}

func NewRuntime(registry *Registry, options ...RuntimeOption) *Runtime {
	rootCtx, cancel := context.WithCancel(context.Background())
	runtime := &Runtime{
		registry:          registry,
		logger:            logger.NewNop(),
		bootstrapStates:   make(map[string]runtimeBootstrapState),
		endpointStates:    make(map[string]platformdomain.ExtensionRuntimeEndpointState),
		consumerKeys:      make(map[string]struct{}),
		consumerStates:    make(map[string]platformdomain.ExtensionRuntimeConsumerState),
		scheduledJobStops: make(map[string]context.CancelFunc),
		jobStates:         make(map[string]platformdomain.ExtensionRuntimeJobState),
		rootCtx:           rootCtx,
		rootCancel:        cancel,
	}
	for _, option := range options {
		if option != nil {
			option(runtime)
		}
	}
	if runtime.logger == nil {
		runtime.logger = logger.NewNop()
	}
	return runtime
}

func (r *Runtime) Start(ctx context.Context) error {
	if r == nil || r.extensionStore == nil || r.workspaceStore == nil {
		return nil
	}

	workspaces, err := r.workspaceStore.ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces for extension runtime bootstrap: %w", err)
	}
	instanceInstalled, err := r.extensionStore.ListInstanceExtensions(ctx)
	if err != nil {
		return fmt.Errorf("list instance extensions for extension runtime bootstrap: %w", err)
	}
	for _, extension := range instanceInstalled {
		if extension == nil || extension.Status != platformdomain.ExtensionStatusActive {
			continue
		}
		if err := r.EnsureInstalledExtensionRuntime(ctx, extension); err != nil {
			if r.logger != nil {
				r.logger.Warn("Failed to bootstrap instance-scoped extension runtime",
					"extension_id", extension.ID,
					"slug", extension.Slug,
					"error", err,
				)
			}
		}
	}
	for _, workspace := range workspaces {
		installed, err := r.extensionStore.ListWorkspaceExtensions(ctx, workspace.ID)
		if err != nil {
			return fmt.Errorf("list workspace extensions for extension runtime bootstrap: %w", err)
		}
		for _, extension := range installed {
			if extension == nil || extension.Status != platformdomain.ExtensionStatusActive {
				continue
			}
			if err := r.EnsureInstalledExtensionRuntime(ctx, extension); err != nil {
				if r.logger != nil {
					r.logger.Warn("Failed to bootstrap workspace-scoped extension runtime",
						"extension_id", extension.ID,
						"workspace_id", extension.WorkspaceID,
						"slug", extension.Slug,
						"error", err,
					)
				}
			}
		}
	}
	return nil
}

func (r *Runtime) Stop() {
	if r == nil || r.rootCancel == nil {
		return
	}
	r.rootCancel()
}

func (r *Runtime) EnsureInstalledExtensionRuntime(
	ctx context.Context,
	extension *platformdomain.InstalledExtension,
) (err error) {
	if extension == nil {
		return fmt.Errorf("extension not found")
	}
	if extension.Manifest.RuntimeClass != platformdomain.ExtensionRuntimeClassServiceBacked {
		return nil
	}
	if r == nil || r.registry == nil {
		return fmt.Errorf("service target registry is not configured")
	}
	defer func() {
		if err != nil {
			r.recordBootstrapFailure(extension, err)
			return
		}
		r.recordBootstrapSuccess(extension)
	}()

	for _, endpoint := range extension.Manifest.Endpoints {
		serviceTarget := strings.TrimSpace(endpoint.ServiceTarget)
		r.recordEndpointDeclared(extension, endpoint)
		if serviceTarget == "" {
			continue
		}
		if !r.registry.SupportsServiceTarget(extension.Manifest.Runtime.Protocol, serviceTarget) {
			r.recordEndpointFailure(extension, endpoint, fmt.Errorf("service target %s is not registered", serviceTarget))
			return fmt.Errorf("service target %s is not registered", serviceTarget)
		}
		r.recordEndpointRegistered(extension, endpoint)
	}
	for _, consumer := range extension.Manifest.EventConsumers {
		serviceTarget := strings.TrimSpace(consumer.ServiceTarget)
		if serviceTarget == "" {
			continue
		}
		if !r.registry.SupportsServiceTarget(extension.Manifest.Runtime.Protocol, serviceTarget) {
			return fmt.Errorf("service target %s is not registered", serviceTarget)
		}
	}
	for _, job := range extension.Manifest.ScheduledJobs {
		serviceTarget := strings.TrimSpace(job.ServiceTarget)
		if serviceTarget == "" {
			continue
		}
		if !r.registry.SupportsServiceTarget(extension.Manifest.Runtime.Protocol, serviceTarget) {
			return fmt.Errorf("service target %s is not registered", serviceTarget)
		}
	}
	if err := r.ensureBackgroundRuntime(ctx, extension); err != nil {
		return err
	}

	status, message, err := r.CheckInstalledExtensionHealth(ctx, extension)
	if err != nil {
		return err
	}
	if status != platformdomain.ExtensionHealthHealthy {
		message = strings.TrimSpace(message)
		if message == "" {
			message = "service runtime is not healthy"
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}

func (r *Runtime) PrepareInstall(_ context.Context, manifest platformdomain.ExtensionManifest, _ string) error {
	if manifest.RuntimeClass != platformdomain.ExtensionRuntimeClassServiceBacked {
		return fmt.Errorf("privileged extensions require service-backed runtime")
	}
	if r == nil || r.registry == nil {
		return fmt.Errorf("service target registry is not configured")
	}

	hasHealthEndpoint := false
	for _, endpoint := range manifest.Endpoints {
		if endpoint.Class == platformdomain.ExtensionEndpointClassHealth {
			hasHealthEndpoint = true
		}
		if endpoint.Class == platformdomain.ExtensionEndpointClassPublicPage || endpoint.Class == platformdomain.ExtensionEndpointClassPublicAsset {
			return fmt.Errorf("privileged extensions may not declare %s endpoints", endpoint.Class)
		}
		if endpoint.Auth == platformdomain.ExtensionEndpointAuthPublic {
			return fmt.Errorf("privileged endpoint %s may not use public auth", endpoint.Name)
		}
		serviceTarget := strings.TrimSpace(endpoint.ServiceTarget)
		if serviceTarget != "" && !r.registry.SupportsServiceTarget(manifest.Runtime.Protocol, serviceTarget) {
			return fmt.Errorf("service target %s is not registered", serviceTarget)
		}
	}
	if !hasHealthEndpoint {
		return fmt.Errorf("privileged extensions must declare a health endpoint")
	}
	for _, consumer := range manifest.EventConsumers {
		serviceTarget := strings.TrimSpace(consumer.ServiceTarget)
		if serviceTarget == "" {
			continue
		}
		if !r.registry.SupportsServiceTarget(manifest.Runtime.Protocol, serviceTarget) {
			return fmt.Errorf("service target %s is not registered", serviceTarget)
		}
	}
	for _, job := range manifest.ScheduledJobs {
		serviceTarget := strings.TrimSpace(job.ServiceTarget)
		if serviceTarget == "" {
			continue
		}
		if !r.registry.SupportsServiceTarget(manifest.Runtime.Protocol, serviceTarget) {
			return fmt.Errorf("service target %s is not registered", serviceTarget)
		}
	}
	return nil
}

func (r *Runtime) DeactivateInstalledExtensionRuntime(
	ctx context.Context,
	extension *platformdomain.InstalledExtension,
	reason string,
) error {
	if extension == nil || extension.Manifest.RuntimeClass != platformdomain.ExtensionRuntimeClassServiceBacked {
		return nil
	}
	if r == nil {
		return fmt.Errorf("extension runtime is not configured")
	}

	active, err := r.isExtensionInstallActive(ctx, extension)
	if err != nil {
		return err
	}
	if active {
		return nil
	}

	r.markRuntimeDraining(extension, reason)
	r.cancelScheduledJobs(extension, reason)
	r.markRuntimeInactive(extension, reason)
	return nil
}

func (r *Runtime) CheckInstalledExtensionHealth(
	_ context.Context,
	extension *platformdomain.InstalledExtension,
) (platformdomain.ExtensionHealthStatus, string, error) {
	if extension == nil {
		return platformdomain.ExtensionHealthFailed, "", fmt.Errorf("extension not found")
	}
	if extension.Manifest.RuntimeClass != platformdomain.ExtensionRuntimeClassServiceBacked {
		return platformdomain.ExtensionHealthHealthy, "bundle runtime healthy", nil
	}

	healthEndpoints := make([]platformdomain.ExtensionEndpoint, 0)
	for _, endpoint := range extension.Manifest.Endpoints {
		if endpoint.Class == platformdomain.ExtensionEndpointClassHealth {
			healthEndpoints = append(healthEndpoints, endpoint)
		}
	}
	if len(healthEndpoints) == 0 {
		return platformdomain.ExtensionHealthDegraded, "service-backed extension does not declare a health endpoint", nil
	}
	if r == nil || r.registry == nil {
		return platformdomain.ExtensionHealthFailed, "", fmt.Errorf("service target registry is not configured")
	}

	overallStatus := platformdomain.ExtensionHealthHealthy
	messages := make([]string, 0, len(healthEndpoints))
	for _, endpoint := range healthEndpoints {
		status, message, err := r.checkHealthEndpoint(extension, endpoint)
		if err != nil {
			r.recordEndpointFailure(extension, endpoint, err)
			return platformdomain.ExtensionHealthFailed, "", err
		}
		r.recordEndpointCheck(extension, endpoint, status, message)
		overallStatus = worseExtensionHealthStatus(overallStatus, status)
		if strings.TrimSpace(message) != "" {
			messages = append(messages, fmt.Sprintf("%s: %s", endpoint.Name, strings.TrimSpace(message)))
		}
	}
	if len(messages) == 0 {
		messages = append(messages, "service runtime healthy")
	}
	return overallStatus, strings.Join(messages, "; "), nil
}

func (r *Runtime) checkHealthEndpoint(extension *platformdomain.InstalledExtension, endpoint platformdomain.ExtensionEndpoint) (platformdomain.ExtensionHealthStatus, string, error) {
	result, err := r.registry.ProbeEndpoint(extension, endpoint)
	if err != nil {
		return platformdomain.ExtensionHealthFailed, "", fmt.Errorf("dispatch health target %s: %w", endpoint.ServiceTarget, err)
	}

	status, message := decodeHealthResponse(result.StatusCode, result.Body)
	return status, message, nil
}

func decodeHealthResponse(statusCode int, body []byte) (platformdomain.ExtensionHealthStatus, string) {
	status := platformdomain.ExtensionHealthUnknown
	message := ""

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if len(body) > 0 && json.Unmarshal(body, &payload) == nil {
		message = strings.TrimSpace(payload.Message)
		switch strings.TrimSpace(strings.ToLower(payload.Status)) {
		case "ok", "healthy":
			status = platformdomain.ExtensionHealthHealthy
		case "degraded", "warning":
			status = platformdomain.ExtensionHealthDegraded
		case "failed", "down", "error", "unhealthy":
			status = platformdomain.ExtensionHealthFailed
		}
	}

	switch {
	case statusCode >= 500:
		status = platformdomain.ExtensionHealthFailed
	case statusCode >= 400 && status == platformdomain.ExtensionHealthUnknown:
		status = platformdomain.ExtensionHealthFailed
	case statusCode >= 300 && status == platformdomain.ExtensionHealthUnknown:
		status = platformdomain.ExtensionHealthDegraded
	case statusCode >= 200 && status == platformdomain.ExtensionHealthUnknown:
		status = platformdomain.ExtensionHealthHealthy
	}

	if message == "" {
		switch status {
		case platformdomain.ExtensionHealthHealthy, platformdomain.ExtensionHealthDegraded, platformdomain.ExtensionHealthFailed:
			message = fmt.Sprintf("returned HTTP %d", statusCode)
		default:
			message = "health status unknown"
		}
	}

	return status, message
}

func worseExtensionHealthStatus(left, right platformdomain.ExtensionHealthStatus) platformdomain.ExtensionHealthStatus {
	if extensionHealthSeverity(right) > extensionHealthSeverity(left) {
		return right
	}
	return left
}

func extensionHealthSeverity(status platformdomain.ExtensionHealthStatus) int {
	switch status {
	case platformdomain.ExtensionHealthHealthy:
		return 0
	case platformdomain.ExtensionHealthUnknown:
		return 1
	case platformdomain.ExtensionHealthDegraded:
		return 2
	case platformdomain.ExtensionHealthFailed:
		return 3
	default:
		return 1
	}
}

func (r *Runtime) GetInstalledExtensionRuntimeDiagnostics(_ context.Context, extension *platformdomain.InstalledExtension) (platformdomain.ExtensionRuntimeDiagnostics, error) {
	if extension == nil {
		return platformdomain.ExtensionRuntimeDiagnostics{}, fmt.Errorf("extension not found")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	diagnostics := platformdomain.ExtensionRuntimeDiagnostics{
		BootstrapStatus: "not_started",
		Endpoints:       make([]platformdomain.ExtensionRuntimeEndpointState, 0, len(extension.Manifest.Endpoints)),
		EventConsumers:  make([]platformdomain.ExtensionRuntimeConsumerState, 0, len(extension.Manifest.EventConsumers)),
		ScheduledJobs:   make([]platformdomain.ExtensionRuntimeJobState, 0, len(extension.Manifest.ScheduledJobs)),
	}
	if state, ok := r.bootstrapStates[r.runtimeInstallKey(extension)]; ok {
		diagnostics.BootstrapStatus = state.Status
		diagnostics.LastBootstrapAt = cloneTime(state.LastBootstrapAt)
		diagnostics.LastBootstrapError = state.LastError
	}

	for _, endpoint := range extension.Manifest.Endpoints {
		key := r.backgroundKey(extension, "endpoint", endpoint.Name)
		state, ok := r.endpointStates[key]
		if !ok {
			state = platformdomain.ExtensionRuntimeEndpointState{
				Name:          endpoint.Name,
				Class:         string(endpoint.Class),
				MountPath:     endpoint.MountPath,
				ServiceTarget: endpoint.ServiceTarget,
				Status:        "declared",
			}
		}
		diagnostics.Endpoints = append(diagnostics.Endpoints, cloneEndpointState(state))
	}

	for _, consumer := range extension.Manifest.EventConsumers {
		key := r.backgroundKey(extension, "consumer", consumer.Name)
		state, ok := r.consumerStates[key]
		if !ok {
			state = platformdomain.ExtensionRuntimeConsumerState{
				Name:          consumer.Name,
				Stream:        consumer.Stream,
				ConsumerGroup: strings.TrimSpace(consumer.ConsumerGroup),
				ServiceTarget: consumer.ServiceTarget,
				Status:        "not_registered",
			}
		}
		diagnostics.EventConsumers = append(diagnostics.EventConsumers, cloneConsumerState(state))
	}

	for _, job := range extension.Manifest.ScheduledJobs {
		key := r.backgroundKey(extension, "job", job.Name)
		state, ok := r.jobStates[key]
		if !ok {
			state = platformdomain.ExtensionRuntimeJobState{
				Name:            job.Name,
				IntervalSeconds: job.IntervalSeconds,
				ServiceTarget:   job.ServiceTarget,
				Status:          "not_registered",
			}
		}
		diagnostics.ScheduledJobs = append(diagnostics.ScheduledJobs, cloneJobState(state))
	}

	if extension.Status != platformdomain.ExtensionStatusActive {
		markDiagnosticsInactive(&diagnostics, extension.HealthMessage)
	}

	return diagnostics, nil
}

func (r *Runtime) recordBootstrapSuccess(extension *platformdomain.InstalledExtension) {
	if r == nil || extension == nil {
		return
	}
	key := r.runtimeInstallKey(extension)
	if key == "" {
		return
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bootstrapStates[key] = runtimeBootstrapState{
		Status:          "bootstrapped",
		LastBootstrapAt: &now,
	}
}

func (r *Runtime) recordBootstrapFailure(extension *platformdomain.InstalledExtension, err error) {
	if r == nil || extension == nil {
		return
	}
	key := r.runtimeInstallKey(extension)
	if key == "" {
		return
	}
	now := time.Now()
	state := runtimeBootstrapState{
		Status:          "failed",
		LastBootstrapAt: &now,
	}
	if err != nil {
		state.LastError = err.Error()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bootstrapStates[key] = state
}

func cloneEndpointState(state platformdomain.ExtensionRuntimeEndpointState) platformdomain.ExtensionRuntimeEndpointState {
	state.RegisteredAt = cloneTime(state.RegisteredAt)
	state.LastCheckedAt = cloneTime(state.LastCheckedAt)
	state.LastSuccessAt = cloneTime(state.LastSuccessAt)
	state.LastFailureAt = cloneTime(state.LastFailureAt)
	return state
}

func cloneConsumerState(state platformdomain.ExtensionRuntimeConsumerState) platformdomain.ExtensionRuntimeConsumerState {
	state.RegisteredAt = cloneTime(state.RegisteredAt)
	state.LastDeliveredAt = cloneTime(state.LastDeliveredAt)
	state.LastSuccessAt = cloneTime(state.LastSuccessAt)
	state.LastFailureAt = cloneTime(state.LastFailureAt)
	return state
}

func cloneJobState(state platformdomain.ExtensionRuntimeJobState) platformdomain.ExtensionRuntimeJobState {
	state.RegisteredAt = cloneTime(state.RegisteredAt)
	state.LastStartedAt = cloneTime(state.LastStartedAt)
	state.LastSuccessAt = cloneTime(state.LastSuccessAt)
	state.LastFailureAt = cloneTime(state.LastFailureAt)
	state.BackoffUntil = cloneTime(state.BackoffUntil)
	return state
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func markDiagnosticsInactive(diagnostics *platformdomain.ExtensionRuntimeDiagnostics, reason string) {
	if diagnostics == nil {
		return
	}
	reason = strings.TrimSpace(reason)
	for idx := range diagnostics.Endpoints {
		diagnostics.Endpoints[idx].Status = "inactive"
		if diagnostics.Endpoints[idx].LastError == "" {
			diagnostics.Endpoints[idx].LastError = reason
		}
	}
	for idx := range diagnostics.EventConsumers {
		diagnostics.EventConsumers[idx].Status = "inactive"
		if diagnostics.EventConsumers[idx].LastError == "" {
			diagnostics.EventConsumers[idx].LastError = reason
		}
	}
	for idx := range diagnostics.ScheduledJobs {
		diagnostics.ScheduledJobs[idx].Status = "inactive"
		if diagnostics.ScheduledJobs[idx].LastError == "" {
			diagnostics.ScheduledJobs[idx].LastError = reason
		}
	}
}

func (r *Runtime) recordEndpointDeclared(extension *platformdomain.InstalledExtension, endpoint platformdomain.ExtensionEndpoint) {
	if r == nil {
		return
	}
	key := r.backgroundKey(extension, "endpoint", endpoint.Name)
	if key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.endpointStates[key]; exists {
		return
	}
	r.endpointStates[key] = platformdomain.ExtensionRuntimeEndpointState{
		Name:          endpoint.Name,
		Class:         string(endpoint.Class),
		MountPath:     endpoint.MountPath,
		ServiceTarget: endpoint.ServiceTarget,
		Status:        "declared",
	}
}

func (r *Runtime) recordEndpointRegistered(extension *platformdomain.InstalledExtension, endpoint platformdomain.ExtensionEndpoint) {
	if r == nil {
		return
	}
	key := r.backgroundKey(extension, "endpoint", endpoint.Name)
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.endpointStates[key]
	state.Name = endpoint.Name
	state.Class = string(endpoint.Class)
	state.MountPath = endpoint.MountPath
	state.ServiceTarget = endpoint.ServiceTarget
	state.Status = "registered"
	if state.RegisteredAt == nil {
		state.RegisteredAt = &now
	}
	r.endpointStates[key] = state
}

func (r *Runtime) recordEndpointCheck(extension *platformdomain.InstalledExtension, endpoint platformdomain.ExtensionEndpoint, health platformdomain.ExtensionHealthStatus, message string) {
	if r == nil {
		return
	}
	key := r.backgroundKey(extension, "endpoint", endpoint.Name)
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.endpointStates[key]
	state.Name = endpoint.Name
	state.Class = string(endpoint.Class)
	state.MountPath = endpoint.MountPath
	state.ServiceTarget = endpoint.ServiceTarget
	state.LastCheckedAt = &now
	switch health {
	case platformdomain.ExtensionHealthHealthy:
		state.Status = "healthy"
		state.LastSuccessAt = &now
		state.ConsecutiveFailures = 0
		state.LastError = ""
	case platformdomain.ExtensionHealthDegraded:
		state.Status = "degraded"
		state.LastSuccessAt = &now
		state.ConsecutiveFailures = 0
		state.LastError = strings.TrimSpace(message)
	case platformdomain.ExtensionHealthFailed:
		state.Status = "failed"
		state.LastFailureAt = &now
		state.ConsecutiveFailures++
		state.LastError = strings.TrimSpace(message)
	default:
		state.Status = "unknown"
	}
	r.endpointStates[key] = state
}

func (r *Runtime) recordEndpointFailure(extension *platformdomain.InstalledExtension, endpoint platformdomain.ExtensionEndpoint, err error) {
	if r == nil {
		return
	}
	key := r.backgroundKey(extension, "endpoint", endpoint.Name)
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.endpointStates[key]
	state.Name = endpoint.Name
	state.Class = string(endpoint.Class)
	state.MountPath = endpoint.MountPath
	state.ServiceTarget = endpoint.ServiceTarget
	state.Status = "failed"
	state.LastCheckedAt = &now
	state.LastFailureAt = &now
	state.ConsecutiveFailures++
	if err != nil {
		state.LastError = err.Error()
	}
	r.endpointStates[key] = state
}

func (r *Runtime) markRuntimeDraining(extension *platformdomain.InstalledExtension, reason string) {
	if r == nil || extension == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	message := strings.TrimSpace(reason)
	for _, endpoint := range extension.Manifest.Endpoints {
		key := r.backgroundKey(extension, "endpoint", endpoint.Name)
		state := r.endpointStates[key]
		state.Name = endpoint.Name
		state.Class = string(endpoint.Class)
		state.MountPath = endpoint.MountPath
		state.ServiceTarget = endpoint.ServiceTarget
		state.Status = "draining"
		if message != "" {
			state.LastError = message
		}
		r.endpointStates[key] = state
	}
	for _, consumer := range extension.Manifest.EventConsumers {
		key := r.backgroundKey(extension, "consumer", consumer.Name)
		state := r.consumerStates[key]
		state.Name = consumer.Name
		state.Stream = consumer.Stream
		state.ConsumerGroup = strings.TrimSpace(consumer.ConsumerGroup)
		state.ServiceTarget = consumer.ServiceTarget
		state.Status = "draining"
		if message != "" {
			state.LastError = message
		}
		r.consumerStates[key] = state
	}
	for _, job := range extension.Manifest.ScheduledJobs {
		key := r.backgroundKey(extension, "job", job.Name)
		state := r.jobStates[key]
		state.Name = job.Name
		state.IntervalSeconds = job.IntervalSeconds
		state.ServiceTarget = job.ServiceTarget
		state.Status = "draining"
		if message != "" {
			state.LastError = message
		}
		r.jobStates[key] = state
	}
}

func (r *Runtime) markRuntimeInactive(extension *platformdomain.InstalledExtension, reason string) {
	if r == nil || extension == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	message := strings.TrimSpace(reason)
	for _, endpoint := range extension.Manifest.Endpoints {
		key := r.backgroundKey(extension, "endpoint", endpoint.Name)
		state := r.endpointStates[key]
		state.Name = endpoint.Name
		state.Class = string(endpoint.Class)
		state.MountPath = endpoint.MountPath
		state.ServiceTarget = endpoint.ServiceTarget
		state.Status = "inactive"
		if message != "" {
			state.LastError = message
		}
		r.endpointStates[key] = state
	}
	for _, consumer := range extension.Manifest.EventConsumers {
		key := r.backgroundKey(extension, "consumer", consumer.Name)
		state := r.consumerStates[key]
		state.Name = consumer.Name
		state.Stream = consumer.Stream
		state.ConsumerGroup = strings.TrimSpace(consumer.ConsumerGroup)
		state.ServiceTarget = consumer.ServiceTarget
		state.Status = "inactive"
		if message != "" {
			state.LastError = message
		}
		r.consumerStates[key] = state
	}
	for _, job := range extension.Manifest.ScheduledJobs {
		key := r.backgroundKey(extension, "job", job.Name)
		state := r.jobStates[key]
		state.Name = job.Name
		state.IntervalSeconds = job.IntervalSeconds
		state.ServiceTarget = job.ServiceTarget
		state.Status = "inactive"
		if message != "" {
			state.LastError = message
		}
		r.jobStates[key] = state
	}
}
