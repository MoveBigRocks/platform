package extensionruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/movebigrocks/platform/pkg/eventbus"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

func (r *Runtime) ensureBackgroundRuntime(ctx context.Context, extension *platformdomain.InstalledExtension) error {
	if r == nil || extension == nil {
		return nil
	}
	if extension.Manifest.RuntimeClass != platformdomain.ExtensionRuntimeClassServiceBacked {
		return nil
	}

	for _, consumer := range extension.Manifest.EventConsumers {
		if err := r.ensureEventConsumer(ctx, extension, consumer); err != nil {
			return err
		}
	}
	for _, job := range extension.Manifest.ScheduledJobs {
		if err := r.ensureScheduledJob(extension, job); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) ensureEventConsumer(ctx context.Context, extension *platformdomain.InstalledExtension, consumer platformdomain.ExtensionEventConsumer) error {
	if r == nil || r.eventBus == nil || r.registry == nil || extension == nil {
		return nil
	}
	key := r.backgroundKey(extension, "consumer", consumer.Name)
	if key == "" {
		return nil
	}

	r.mu.Lock()
	if _, exists := r.consumerKeys[key]; exists {
		state := r.consumerStates[key]
		state.Name = consumer.Name
		state.Stream = consumer.Stream
		state.ConsumerGroup = strings.TrimSpace(consumer.ConsumerGroup)
		state.ServiceTarget = consumer.ServiceTarget
		if state.Status == "" || state.Status == "inactive" || state.Status == "draining" {
			state.Status = "registered"
			state.LastError = ""
			if state.RegisteredAt == nil {
				registeredAt := time.Now()
				state.RegisteredAt = &registeredAt
			}
			r.consumerStates[key] = state
		}
		r.mu.Unlock()
		return nil
	}
	r.consumerKeys[key] = struct{}{}
	registeredAt := time.Now()
	r.consumerStates[key] = platformdomain.ExtensionRuntimeConsumerState{
		Name:          consumer.Name,
		Stream:        consumer.Stream,
		ConsumerGroup: strings.TrimSpace(consumer.ConsumerGroup),
		ServiceTarget: consumer.ServiceTarget,
		Status:        "registered",
		RegisteredAt:  &registeredAt,
	}
	r.mu.Unlock()

	subscribeErr := r.eventBus.Subscribe(
		eventbus.StreamFromString(consumer.Stream),
		r.subscriptionGroup(extension, consumer.ConsumerGroup),
		consumer.Name,
		func(handlerCtx context.Context, data []byte) error {
			if !matchesConsumerEventType(consumer, data) {
				return nil
			}
			active, err := r.extensionActiveForPayload(handlerCtx, extension, data)
			if err != nil {
				r.recordConsumerFailure(key, err)
				return err
			}
			if !active {
				return nil
			}
			if err := r.registry.ConsumeExtension(extension, consumer, handlerCtx, data); err != nil {
				r.recordConsumerFailure(key, err)
				return err
			}
			r.recordConsumerSuccess(key)
			return nil
		},
	)
	if subscribeErr != nil {
		return fmt.Errorf("subscribe extension consumer %s: %w", consumer.Name, subscribeErr)
	}

	if r.logger != nil {
		r.logger.Info("Extension event consumer registered",
			"extension", extension.Slug,
			"consumer", consumer.Name,
			"stream", consumer.Stream,
			"service_target", consumer.ServiceTarget,
		)
	}
	return nil
}

func (r *Runtime) ensureScheduledJob(extension *platformdomain.InstalledExtension, job platformdomain.ExtensionScheduledJob) error {
	if r == nil || r.registry == nil || extension == nil {
		return nil
	}
	key := r.backgroundKey(extension, "job", job.Name)
	if key == "" {
		return nil
	}

	r.mu.Lock()
	if _, exists := r.scheduledJobStops[key]; exists {
		r.mu.Unlock()
		return nil
	}
	jobCtx, cancel := context.WithCancel(r.rootCtx)
	r.scheduledJobStops[key] = cancel
	registeredAt := time.Now()
	r.jobStates[key] = platformdomain.ExtensionRuntimeJobState{
		Name:            job.Name,
		IntervalSeconds: job.IntervalSeconds,
		ServiceTarget:   job.ServiceTarget,
		Status:          "registered",
		RegisteredAt:    &registeredAt,
	}
	r.mu.Unlock()

	go r.runScheduledJobLoop(jobCtx, extension, job)
	return nil
}

func (r *Runtime) cancelScheduledJobs(extension *platformdomain.InstalledExtension, reason string) {
	if r == nil || extension == nil {
		return
	}
	message := strings.TrimSpace(reason)
	for _, job := range extension.Manifest.ScheduledJobs {
		key := r.backgroundKey(extension, "job", job.Name)
		r.mu.Lock()
		cancel, exists := r.scheduledJobStops[key]
		if exists {
			delete(r.scheduledJobStops, key)
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
		r.mu.Unlock()
		if exists && cancel != nil {
			cancel()
		}
	}
}

func (r *Runtime) runScheduledJobLoop(ctx context.Context, extension *platformdomain.InstalledExtension, job platformdomain.ExtensionScheduledJob) {
	runOnce := func() {
		if wait, ok := r.jobBackoffRemaining(extension, job); ok {
			r.recordJobBackoff(extension, job, wait)
			return
		}
		r.recordJobStarted(extension, job)
		active, err := r.isExtensionInstallActive(ctx, extension)
		if err != nil {
			r.recordJobFailure(extension, job, err)
			if r.logger != nil {
				r.logger.Warn("Failed to resolve active installs for extension scheduled job",
					"extension", extension.Slug,
					"job", job.Name,
					"error", err,
				)
			}
			return
		}
		if !active {
			r.recordJobIdle(extension, job)
			return
		}
		if err := r.registry.RunExtensionJob(extension, job, ctx); err != nil {
			r.recordJobFailure(extension, job, err)
			if r.logger != nil {
				r.logger.Warn("Extension scheduled job failed",
					"extension", extension.Slug,
					"job", job.Name,
					"service_target", job.ServiceTarget,
					"error", err,
				)
			}
			return
		}
		r.recordJobSuccess(extension, job)
	}

	runOnce()

	ticker := time.NewTicker(time.Duration(job.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.recordJobInactive(extension, job)
			return
		case <-ticker.C:
			runOnce()
		}
	}
}

func (r *Runtime) recordConsumerSuccess(key string) {
	if r == nil {
		return
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.consumerStates[key]
	state.Status = "healthy"
	state.ConsecutiveFailures = 0
	state.LastDeliveredAt = &now
	state.LastSuccessAt = &now
	state.LastError = ""
	r.consumerStates[key] = state
}

func (r *Runtime) recordConsumerFailure(key string, err error) {
	if r == nil {
		return
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.consumerStates[key]
	state.Status = "failed"
	state.ConsecutiveFailures++
	state.LastDeliveredAt = &now
	state.LastFailureAt = &now
	if err != nil {
		state.LastError = err.Error()
	}
	r.consumerStates[key] = state
}

func (r *Runtime) recordJobStarted(extension *platformdomain.InstalledExtension, job platformdomain.ExtensionScheduledJob) {
	if r == nil {
		return
	}
	key := r.backgroundKey(extension, "job", job.Name)
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.jobStates[key]
	if state.Name == "" {
		state.Name = job.Name
		state.IntervalSeconds = job.IntervalSeconds
		state.ServiceTarget = job.ServiceTarget
		state.RegisteredAt = &now
	}
	state.Status = "running"
	state.LastStartedAt = &now
	r.jobStates[key] = state
}

func (r *Runtime) recordJobSuccess(extension *platformdomain.InstalledExtension, job platformdomain.ExtensionScheduledJob) {
	if r == nil {
		return
	}
	key := r.backgroundKey(extension, "job", job.Name)
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.jobStates[key]
	state.Status = "healthy"
	state.ConsecutiveFailures = 0
	state.BackoffUntil = nil
	state.LastSuccessAt = &now
	state.LastError = ""
	r.jobStates[key] = state
}

func (r *Runtime) recordJobIdle(extension *platformdomain.InstalledExtension, job platformdomain.ExtensionScheduledJob) {
	if r == nil {
		return
	}
	key := r.backgroundKey(extension, "job", job.Name)
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.jobStates[key]
	if state.Name == "" {
		state.Name = job.Name
		state.IntervalSeconds = job.IntervalSeconds
		state.ServiceTarget = job.ServiceTarget
	}
	state.Status = "idle"
	state.BackoffUntil = nil
	r.jobStates[key] = state
}

func (r *Runtime) recordJobFailure(extension *platformdomain.InstalledExtension, job platformdomain.ExtensionScheduledJob, err error) {
	if r == nil {
		return
	}
	key := r.backgroundKey(extension, "job", job.Name)
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.jobStates[key]
	state.ConsecutiveFailures++
	state.Status = "failed"
	state.LastFailureAt = &now
	backoffUntil := now.Add(computeRuntimeBackoff(job.IntervalSeconds, state.ConsecutiveFailures))
	state.BackoffUntil = &backoffUntil
	if err != nil {
		state.LastError = err.Error()
	}
	r.jobStates[key] = state
}

func (r *Runtime) recordJobInactive(extension *platformdomain.InstalledExtension, job platformdomain.ExtensionScheduledJob) {
	if r == nil {
		return
	}
	key := r.backgroundKey(extension, "job", job.Name)
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.jobStates[key]
	if state.Name == "" {
		state.Name = job.Name
		state.IntervalSeconds = job.IntervalSeconds
		state.ServiceTarget = job.ServiceTarget
	}
	state.Status = "inactive"
	state.BackoffUntil = nil
	r.jobStates[key] = state
}

func (r *Runtime) recordJobBackoff(extension *platformdomain.InstalledExtension, job platformdomain.ExtensionScheduledJob, remaining time.Duration) {
	if r == nil {
		return
	}
	key := r.backgroundKey(extension, "job", job.Name)
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.jobStates[key]
	if state.Name == "" {
		state.Name = job.Name
		state.IntervalSeconds = job.IntervalSeconds
		state.ServiceTarget = job.ServiceTarget
	}
	state.Status = "backoff"
	if state.BackoffUntil == nil {
		until := time.Now().Add(remaining)
		state.BackoffUntil = &until
	}
	r.jobStates[key] = state
}

func (r *Runtime) jobBackoffRemaining(extension *platformdomain.InstalledExtension, job platformdomain.ExtensionScheduledJob) (time.Duration, bool) {
	if r == nil {
		return 0, false
	}
	key := r.backgroundKey(extension, "job", job.Name)
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.jobStates[key]
	if !ok || state.BackoffUntil == nil {
		return 0, false
	}
	remaining := time.Until(*state.BackoffUntil)
	if remaining <= 0 {
		state.BackoffUntil = nil
		if state.Status == "backoff" {
			state.Status = "registered"
		}
		r.jobStates[key] = state
		return 0, false
	}
	return remaining, true
}

func computeRuntimeBackoff(intervalSeconds, consecutiveFailures int) time.Duration {
	base := time.Second
	if intervalSeconds > 0 {
		base = time.Duration(intervalSeconds) * time.Second
	}
	if consecutiveFailures < 1 {
		consecutiveFailures = 1
	}
	multiplier := math.Pow(2, float64(consecutiveFailures-1))
	backoff := time.Duration(float64(base) * multiplier)
	maxBackoff := 5 * time.Minute
	if backoff > maxBackoff {
		return maxBackoff
	}
	return backoff
}

func (r *Runtime) extensionActiveForPayload(ctx context.Context, extension *platformdomain.InstalledExtension, data []byte) (bool, error) {
	if extension == nil {
		return false, nil
	}
	if extension.Manifest.Scope == platformdomain.ExtensionScopeInstance {
		return r.isExtensionInstallActive(ctx, extension)
	}

	workspaceID := payloadWorkspaceID(data)
	if strings.TrimSpace(workspaceID) == "" {
		return r.isExtensionInstallActive(ctx, extension)
	}
	if strings.TrimSpace(extension.WorkspaceID) == "" || strings.TrimSpace(extension.WorkspaceID) != strings.TrimSpace(workspaceID) {
		return false, nil
	}
	return r.isExtensionInstallActive(ctx, extension)
}

func (r *Runtime) isExtensionInstallActive(ctx context.Context, extension *platformdomain.InstalledExtension) (bool, error) {
	if extension == nil {
		return false, nil
	}
	if r.extensionStore == nil {
		return extension.Status == platformdomain.ExtensionStatusActive, nil
	}
	if extensionID := strings.TrimSpace(extension.ID); extensionID != "" {
		current, err := r.extensionStore.GetInstalledExtension(ctx, extensionID)
		if err == nil && current != nil {
			return current.Status == platformdomain.ExtensionStatusActive, nil
		}
	}
	if extension.Manifest.Scope == platformdomain.ExtensionScopeInstance {
		installed, err := r.extensionStore.GetInstanceExtensionBySlug(ctx, extension.Slug)
		if err != nil || installed == nil {
			return false, nil //nolint:nilerr // not found = not active
		}
		return installed.Status == platformdomain.ExtensionStatusActive, nil
	}
	if strings.TrimSpace(extension.WorkspaceID) == "" {
		return extension.Status == platformdomain.ExtensionStatusActive, nil
	}
	installed, err := r.extensionStore.GetInstalledExtensionBySlug(ctx, extension.WorkspaceID, extension.Slug)
	if err != nil || installed == nil {
		return false, nil //nolint:nilerr // not found = not active
	}
	return installed.Status == platformdomain.ExtensionStatusActive, nil
}

func (r *Runtime) backgroundKey(extension *platformdomain.InstalledExtension, kind, name string) string {
	if extension == nil {
		return ""
	}
	installKey := r.runtimeInstallKey(extension)
	name = strings.TrimSpace(name)
	if installKey == "" || name == "" {
		return ""
	}
	return installKey + ":" + kind + ":" + name
}

func (r *Runtime) subscriptionGroup(extension *platformdomain.InstalledExtension, group string) string {
	group = strings.TrimSpace(group)
	if group == "" {
		group = "consumer"
	}
	key := sanitizeRuntimeIdentifier(r.runtimeInstallKey(extension))
	if key == "" {
		key = "extension"
	}
	return key + "-" + sanitizeRuntimeIdentifier(group)
}

func (r *Runtime) runtimeInstallKey(extension *platformdomain.InstalledExtension) string {
	if extension == nil {
		return ""
	}
	if extensionID := strings.TrimSpace(extension.ID); extensionID != "" {
		return extensionID
	}
	packageKey := strings.TrimSpace(extension.Manifest.PackageKey())
	if packageKey == "" {
		packageKey = strings.TrimSpace(extension.Slug)
	}
	workspaceID := strings.TrimSpace(extension.WorkspaceID)
	switch {
	case workspaceID != "" && packageKey != "":
		return workspaceID + ":" + packageKey
	case workspaceID != "":
		return workspaceID
	default:
		return packageKey
	}
}

func sanitizeRuntimeIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"/", "-",
		":", "-",
		"_", "-",
		" ", "-",
	)
	return replacer.Replace(value)
}

func matchesConsumerEventType(consumer platformdomain.ExtensionEventConsumer, data []byte) bool {
	if len(consumer.EventTypes) == 0 {
		return true
	}
	eventType := strings.TrimSpace(eventbus.ParseEventType(data))
	if eventType == "" {
		return false
	}
	for _, allowed := range consumer.EventTypes {
		if allowed == eventType {
			return true
		}
	}
	return false
}

func payloadWorkspaceID(data []byte) string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}

	for _, key := range []string{"workspace_id", "workspaceID", "WorkspaceID"} {
		value, ok := payload[key]
		if !ok {
			continue
		}
		var workspaceID string
		if json.Unmarshal(value, &workspaceID) == nil {
			return strings.TrimSpace(workspaceID)
		}
	}
	return ""
}
