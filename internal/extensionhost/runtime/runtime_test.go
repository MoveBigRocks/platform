package extensionruntime

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/pkg/eventbus"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

func TestRegistryDispatchAndProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := &Registry{}
	registry.Register("test.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "message": "ready"})
	})

	result, err := registry.Probe("test.health", http.MethodGet, "/extensions/test/health", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.Contains(t, string(result.Body), `"status":"healthy"`)
}

func TestRegistryConsumeAndRunJob(t *testing.T) {
	registry := &Registry{}

	var consumed bool
	registry.RegisterEventConsumer("test.consumer", func(context.Context, []byte) error {
		consumed = true
		return nil
	})
	var jobRan bool
	registry.RegisterScheduledJob("test.job", func(context.Context) error {
		jobRan = true
		return nil
	})

	require.NoError(t, registry.Consume("test.consumer", context.Background(), []byte(`{}`)))
	require.NoError(t, registry.RunJob("test.job", context.Background()))
	assert.True(t, consumed)
	assert.True(t, jobRan)
}

func TestNewRegistryIncludesEnterpriseAccessTargets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := NewRegistry(nil)

	health, err := registry.Probe("enterprise-access.runtime.health", http.MethodGet, "/extensions/enterprise-access/health", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, health.StatusCode)
	assert.Contains(t, string(health.Body), `"status":"failed"`)

	settings, err := registry.Probe("enterprise-access.admin.settings", http.MethodGet, "/extensions/enterprise-access", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, settings.StatusCode)
	assert.Contains(t, string(settings.Body), "Enterprise Access")

	start, err := registry.Probe("enterprise-access.auth.oidc.start", http.MethodPost, "/extensions/enterprise-access/oidc/start", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, start.StatusCode)
	assert.Contains(t, string(start.Body), "failed")
}

func TestRuntimeEnsureInstalledExtensionRuntimeRequiresRegisteredTargets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	runtime := NewRuntime(&Registry{})
	extension := &platformdomain.InstalledExtension{
		Manifest: platformdomain.ExtensionManifest{
			RuntimeClass: platformdomain.ExtensionRuntimeClassServiceBacked,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:          "analytics-ingest",
					Class:         platformdomain.ExtensionEndpointClassPublicIngest,
					MountPath:     "/api/analytics/event",
					ServiceTarget: "analytics.ingest.event",
				},
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/extensions/web-analytics/health",
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "analytics.runtime.health",
				},
			},
		},
	}

	err := runtime.EnsureInstalledExtensionRuntime(context.Background(), extension)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "analytics.ingest.event")
}

func TestRuntimeCheckInstalledExtensionHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := &Registry{}
	registry.Register("analytics.runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "message": "analytics ready"})
	})

	runtime := NewRuntime(registry)
	extension := &platformdomain.InstalledExtension{
		Manifest: platformdomain.ExtensionManifest{
			RuntimeClass: platformdomain.ExtensionRuntimeClassServiceBacked,
			Endpoints: []platformdomain.ExtensionEndpoint{
				{
					Name:             "runtime-health",
					Class:            platformdomain.ExtensionEndpointClassHealth,
					MountPath:        "/extensions/web-analytics/health",
					Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
					WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
					ServiceTarget:    "analytics.runtime.health",
				},
			},
		},
	}

	status, message, err := runtime.CheckInstalledExtensionHealth(context.Background(), extension)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionHealthHealthy, status)
	assert.Contains(t, message, "analytics ready")
}

func TestRuntimePrepareInstallValidatesPrivilegedManifest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := &Registry{}
	registry.Register("enterprise-access.runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	registry.Register("enterprise-access.admin.callback", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	runtime := NewRuntime(registry)
	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "enterprise-access",
		Name:          "Enterprise Access",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindIdentity,
		Scope:         platformdomain.ExtensionScopeInstance,
		Risk:          platformdomain.ExtensionRiskPrivileged,
		RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
		Endpoints: []platformdomain.ExtensionEndpoint{
			{
				Name:          "health",
				Class:         platformdomain.ExtensionEndpointClassHealth,
				MountPath:     "/extensions/enterprise-access/health",
				Methods:       []string{"GET"},
				Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
				ServiceTarget: "enterprise-access.runtime.health",
			},
			{
				Name:          "oidc-callback",
				Class:         platformdomain.ExtensionEndpointClassWebhook,
				MountPath:     "/auth/extensions/enterprise-access/callback",
				Methods:       []string{"POST"},
				Auth:          platformdomain.ExtensionEndpointAuthSignedWebhook,
				ServiceTarget: "enterprise-access.admin.callback",
			},
		},
	}

	require.NoError(t, runtime.PrepareInstall(context.Background(), manifest, ""))
}

func TestRuntimePrepareInstallRejectsUnsafePrivilegedManifest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := &Registry{}
	registry.Register("slack.runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	runtime := NewRuntime(registry)
	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "slack-alerts",
		Name:          "Slack Alerts",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindConnector,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskPrivileged,
		RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
		Endpoints: []platformdomain.ExtensionEndpoint{
			{
				Name:          "public-page",
				Class:         platformdomain.ExtensionEndpointClassPublicPage,
				MountPath:     "/connectors/slack",
				Auth:          platformdomain.ExtensionEndpointAuthSession,
				ServiceTarget: "slack.runtime.health",
			},
		},
	}

	err := runtime.PrepareInstall(context.Background(), manifest, "workspace_123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "may not declare public_page endpoints")
}

func TestRuntimeEnsureInstalledExtensionRuntimeRegistersEventConsumersAndJobs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	registry := &Registry{}
	registry.Register("runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	consumerCalls := make(chan struct{}, 1)
	registry.RegisterEventConsumer("test.consumer", func(context.Context, []byte) error {
		select {
		case consumerCalls <- struct{}{}:
		default:
		}
		return nil
	})

	jobCalls := make(chan struct{}, 2)
	registry.RegisterScheduledJob("test.job", func(context.Context) error {
		select {
		case jobCalls <- struct{}{}:
		default:
		}
		return nil
	})

	runtime := NewRuntime(registry, WithBackgroundRuntimeDeps(eventbus.NewInMemoryBus(), store.Extensions(), store.Workspaces(), nil))
	defer runtime.Stop()

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "runtime-pack",
		Name:          "Runtime Pack",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindOperational,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
		StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
		Schema: platformdomain.ExtensionSchemaManifest{
			Name:            "ext_demandops_runtime_pack",
			PackageKey:      "demandops/runtime-pack",
			TargetVersion:   "000001",
			MigrationEngine: "postgres_sql",
		},
		Runtime: platformdomain.ExtensionRuntimeSpec{
			Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []platformdomain.ExtensionEndpoint{
			{
				Name:             "runtime-health",
				Class:            platformdomain.ExtensionEndpointClassHealth,
				MountPath:        "/extensions/runtime-pack/health",
				Methods:          []string{"GET"},
				Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
				WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
				ServiceTarget:    "runtime.health",
			},
		},
		Events: platformdomain.ExtensionEventCatalog{
			Subscribes: []string{"case.created"},
		},
		EventConsumers: []platformdomain.ExtensionEventConsumer{
			{
				Name:          "case-created",
				Stream:        "case-events",
				EventTypes:    []string{"case.created"},
				ServiceTarget: "test.consumer",
			},
		},
		ScheduledJobs: []platformdomain.ExtensionScheduledJob{
			{
				Name:            "maintenance",
				IntervalSeconds: 1,
				ServiceTarget:   "test.job",
			},
		},
	}

	installed, err := platformdomain.NewInstalledExtension(workspace.ID, "user_123", "lic_runtime", manifest, []byte(`{}`))
	require.NoError(t, err)
	installed.Activate()
	require.NoError(t, store.Extensions().CreateInstalledExtension(ctx, installed))

	require.NoError(t, runtime.EnsureInstalledExtensionRuntime(ctx, installed))

	select {
	case <-jobCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("expected scheduled job to run at least once")
	}

	caseCreated := map[string]any{
		"event_type":  map[string]any{"type": "case.created", "version": 1},
		"WorkspaceID": workspace.ID,
	}
	require.NoError(t, runtime.eventBus.Publish(eventbus.StreamCaseEvents, caseCreated))

	select {
	case <-consumerCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("expected event consumer to receive matching event")
	}

	diagnostics, err := runtime.GetInstalledExtensionRuntimeDiagnostics(ctx, installed)
	require.NoError(t, err)
	require.Len(t, diagnostics.Endpoints, 1)
	require.Len(t, diagnostics.EventConsumers, 1)
	require.Len(t, diagnostics.ScheduledJobs, 1)
	assert.Equal(t, "runtime-health", diagnostics.Endpoints[0].Name)
	assert.Equal(t, "healthy", diagnostics.Endpoints[0].Status)
	assert.NotNil(t, diagnostics.Endpoints[0].RegisteredAt)
	assert.NotNil(t, diagnostics.Endpoints[0].LastCheckedAt)
	assert.NotNil(t, diagnostics.Endpoints[0].LastSuccessAt)
	assert.Equal(t, "case-created", diagnostics.EventConsumers[0].Name)
	assert.Equal(t, "healthy", diagnostics.EventConsumers[0].Status)
	assert.NotNil(t, diagnostics.EventConsumers[0].RegisteredAt)
	assert.NotNil(t, diagnostics.EventConsumers[0].LastSuccessAt)

	require.Eventually(t, func() bool {
		diagnostics, err = runtime.GetInstalledExtensionRuntimeDiagnostics(ctx, installed)
		require.NoError(t, err)
		require.Len(t, diagnostics.ScheduledJobs, 1)
		return diagnostics.ScheduledJobs[0].LastSuccessAt != nil &&
			diagnostics.ScheduledJobs[0].LastStartedAt != nil &&
			diagnostics.ScheduledJobs[0].RegisteredAt != nil
	}, 2*time.Second, 25*time.Millisecond)

	assert.Equal(t, "maintenance", diagnostics.ScheduledJobs[0].Name)
	assert.NotNil(t, diagnostics.ScheduledJobs[0].RegisteredAt)
	assert.NotNil(t, diagnostics.ScheduledJobs[0].LastStartedAt)
	assert.NotNil(t, diagnostics.ScheduledJobs[0].LastSuccessAt)
}

func TestRuntimeDeactivateInstalledExtensionRuntimeStopsJobsAndAllowsReactivation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	registry := &Registry{}
	registry.Register("runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	consumerCalls := make(chan struct{}, 4)
	registry.RegisterEventConsumer("test.consumer", func(context.Context, []byte) error {
		select {
		case consumerCalls <- struct{}{}:
		default:
		}
		return nil
	})

	jobCalls := make(chan struct{}, 8)
	registry.RegisterScheduledJob("test.job", func(context.Context) error {
		select {
		case jobCalls <- struct{}{}:
		default:
		}
		return nil
	})

	runtime := NewRuntime(registry, WithBackgroundRuntimeDeps(eventbus.NewInMemoryBus(), store.Extensions(), store.Workspaces(), nil))
	defer runtime.Stop()

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "runtime-pack",
		Name:          "Runtime Pack",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindOperational,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
		StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
		Schema: platformdomain.ExtensionSchemaManifest{
			Name:            "ext_demandops_runtime_pack",
			PackageKey:      "demandops/runtime-pack",
			TargetVersion:   "000001",
			MigrationEngine: "postgres_sql",
		},
		Runtime: platformdomain.ExtensionRuntimeSpec{
			Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []platformdomain.ExtensionEndpoint{
			{
				Name:             "runtime-health",
				Class:            platformdomain.ExtensionEndpointClassHealth,
				MountPath:        "/extensions/runtime-pack/health",
				Methods:          []string{"GET"},
				Auth:             platformdomain.ExtensionEndpointAuthInternalOnly,
				WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped,
				ServiceTarget:    "runtime.health",
			},
		},
		Events: platformdomain.ExtensionEventCatalog{
			Subscribes: []string{"case.created"},
		},
		EventConsumers: []platformdomain.ExtensionEventConsumer{
			{
				Name:          "case-created",
				Stream:        "case-events",
				EventTypes:    []string{"case.created"},
				ServiceTarget: "test.consumer",
			},
		},
		ScheduledJobs: []platformdomain.ExtensionScheduledJob{
			{
				Name:            "maintenance",
				IntervalSeconds: 1,
				ServiceTarget:   "test.job",
			},
		},
	}

	installed, err := platformdomain.NewInstalledExtension(workspace.ID, "user_123", "lic_runtime", manifest, []byte(`{}`))
	require.NoError(t, err)
	installed.Activate()
	require.NoError(t, store.Extensions().CreateInstalledExtension(ctx, installed))

	require.NoError(t, runtime.EnsureInstalledExtensionRuntime(ctx, installed))

	select {
	case <-jobCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("expected scheduled job to run at least once")
	}

	caseCreated := map[string]any{
		"event_type":  map[string]any{"type": "case.created", "version": 1},
		"WorkspaceID": workspace.ID,
	}
	require.NoError(t, runtime.eventBus.Publish(eventbus.StreamCaseEvents, caseCreated))

	select {
	case <-consumerCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("expected event consumer to receive matching event")
	}

	installed.Deactivate("paused for maintenance")
	require.NoError(t, store.Extensions().UpdateInstalledExtension(ctx, installed))
	require.NoError(t, runtime.DeactivateInstalledExtensionRuntime(ctx, installed, "paused for maintenance"))

	assert.Eventually(t, func() bool {
		diagnostics, err := runtime.GetInstalledExtensionRuntimeDiagnostics(ctx, installed)
		require.NoError(t, err)
		return diagnostics.Endpoints[0].Status == "inactive" &&
			diagnostics.EventConsumers[0].Status == "inactive" &&
			diagnostics.ScheduledJobs[0].Status == "inactive"
	}, time.Second, 25*time.Millisecond)

	require.NoError(t, runtime.eventBus.Publish(eventbus.StreamCaseEvents, caseCreated))
	select {
	case <-consumerCalls:
		t.Fatal("expected consumer to stay inactive after deactivation")
	case <-time.After(200 * time.Millisecond):
	}

	installed.Activate()
	require.NoError(t, store.Extensions().UpdateInstalledExtension(ctx, installed))
	require.NoError(t, runtime.EnsureInstalledExtensionRuntime(ctx, installed))

	require.NoError(t, runtime.eventBus.Publish(eventbus.StreamCaseEvents, caseCreated))
	select {
	case <-consumerCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("expected consumer to resume after reactivation")
	}

	select {
	case <-jobCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("expected scheduled job to resume after reactivation")
	}
}

func TestRuntimeTracksConsumerFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	registry := &Registry{}
	registry.Register("runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	registry.RegisterEventConsumer("failing.consumer", func(context.Context, []byte) error {
		return fmt.Errorf("consumer boom")
	})

	runtime := NewRuntime(registry, WithBackgroundRuntimeDeps(eventbus.NewInMemoryBus(), store.Extensions(), store.Workspaces(), nil))
	defer runtime.Stop()

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "runtime-pack",
		Name:          "Runtime Pack",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindOperational,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
		StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
		Schema: platformdomain.ExtensionSchemaManifest{
			Name:            "ext_demandops_runtime_pack",
			PackageKey:      "demandops/runtime-pack",
			TargetVersion:   "000001",
			MigrationEngine: "postgres_sql",
		},
		Runtime: platformdomain.ExtensionRuntimeSpec{
			Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []platformdomain.ExtensionEndpoint{
			{
				Name:          "runtime-health",
				Class:         platformdomain.ExtensionEndpointClassHealth,
				MountPath:     "/extensions/runtime-pack/health",
				Methods:       []string{"GET"},
				Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
				ServiceTarget: "runtime.health",
			},
		},
		Events: platformdomain.ExtensionEventCatalog{
			Subscribes: []string{"case.created"},
		},
		EventConsumers: []platformdomain.ExtensionEventConsumer{
			{
				Name:          "case-created",
				Stream:        "case-events",
				EventTypes:    []string{"case.created"},
				ServiceTarget: "failing.consumer",
			},
		},
	}

	installed, err := platformdomain.NewInstalledExtension(workspace.ID, "user_123", "lic_runtime", manifest, []byte(`{}`))
	require.NoError(t, err)
	installed.Activate()
	require.NoError(t, store.Extensions().CreateInstalledExtension(ctx, installed))
	require.NoError(t, runtime.EnsureInstalledExtensionRuntime(ctx, installed))

	caseCreated := map[string]any{
		"event_type":  map[string]any{"type": "case.created", "version": 1},
		"WorkspaceID": workspace.ID,
	}
	err = runtime.eventBus.Publish(eventbus.StreamCaseEvents, caseCreated)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "consumer boom")

	require.Eventually(t, func() bool {
		diagnostics, err := runtime.GetInstalledExtensionRuntimeDiagnostics(ctx, installed)
		require.NoError(t, err)
		return len(diagnostics.EventConsumers) == 1 &&
			diagnostics.EventConsumers[0].ConsecutiveFailures >= 1 &&
			diagnostics.EventConsumers[0].LastFailureAt != nil
	}, 2*time.Second, 50*time.Millisecond)
}

func TestRuntimeIsolatesDiagnosticsPerInstalledExtension(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceOne := testutil.NewIsolatedWorkspace(t)
	workspaceTwo := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceOne))
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceTwo))

	registry := &Registry{}
	registry.Register("runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	registry.RegisterEventConsumer("failing.consumer", func(context.Context, []byte) error {
		return fmt.Errorf("consumer boom")
	})

	runtime := NewRuntime(registry, WithBackgroundRuntimeDeps(eventbus.NewInMemoryBus(), store.Extensions(), store.Workspaces(), nil))
	defer runtime.Stop()

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "runtime-pack",
		Name:          "Runtime Pack",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindOperational,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
		StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
		Schema: platformdomain.ExtensionSchemaManifest{
			Name:            "ext_demandops_runtime_pack",
			PackageKey:      "demandops/runtime-pack",
			TargetVersion:   "000001",
			MigrationEngine: "postgres_sql",
		},
		Runtime: platformdomain.ExtensionRuntimeSpec{
			Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []platformdomain.ExtensionEndpoint{
			{
				Name:          "runtime-health",
				Class:         platformdomain.ExtensionEndpointClassHealth,
				MountPath:     "/extensions/runtime-pack/health",
				Methods:       []string{"GET"},
				Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
				ServiceTarget: "runtime.health",
			},
		},
		Events: platformdomain.ExtensionEventCatalog{
			Subscribes: []string{"case.created"},
		},
		EventConsumers: []platformdomain.ExtensionEventConsumer{
			{
				Name:          "case-created",
				Stream:        "case-events",
				EventTypes:    []string{"case.created"},
				ServiceTarget: "failing.consumer",
			},
		},
	}

	installedOne, err := platformdomain.NewInstalledExtension(workspaceOne.ID, "user_123", "lic_runtime_one", manifest, []byte(`{}`))
	require.NoError(t, err)
	installedOne.Activate()
	require.NoError(t, store.Extensions().CreateInstalledExtension(ctx, installedOne))
	require.NoError(t, runtime.EnsureInstalledExtensionRuntime(ctx, installedOne))

	installedTwo, err := platformdomain.NewInstalledExtension(workspaceTwo.ID, "user_123", "lic_runtime_two", manifest, []byte(`{}`))
	require.NoError(t, err)
	installedTwo.Activate()
	require.NoError(t, store.Extensions().CreateInstalledExtension(ctx, installedTwo))
	require.NoError(t, runtime.EnsureInstalledExtensionRuntime(ctx, installedTwo))

	caseCreated := map[string]any{
		"event_type":  map[string]any{"type": "case.created", "version": 1},
		"WorkspaceID": workspaceOne.ID,
	}
	err = runtime.eventBus.Publish(eventbus.StreamCaseEvents, caseCreated)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "consumer boom")

	require.Eventually(t, func() bool {
		diagnosticsOne, err := runtime.GetInstalledExtensionRuntimeDiagnostics(ctx, installedOne)
		require.NoError(t, err)
		diagnosticsTwo, err := runtime.GetInstalledExtensionRuntimeDiagnostics(ctx, installedTwo)
		require.NoError(t, err)
		return len(diagnosticsOne.EventConsumers) == 1 &&
			len(diagnosticsTwo.EventConsumers) == 1 &&
			diagnosticsOne.EventConsumers[0].ConsecutiveFailures >= 1 &&
			diagnosticsOne.EventConsumers[0].LastFailureAt != nil &&
			diagnosticsTwo.EventConsumers[0].ConsecutiveFailures == 0 &&
			diagnosticsTwo.EventConsumers[0].LastFailureAt == nil
	}, 2*time.Second, 50*time.Millisecond)
}

func TestRuntimeStartContinuesAfterBootstrapFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspaceOne := testutil.NewIsolatedWorkspace(t)
	workspaceTwo := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceOne))
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspaceTwo))

	registry := &Registry{}
	registry.Register("runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	runtime := NewRuntime(registry, WithBackgroundRuntimeDeps(eventbus.NewInMemoryBus(), store.Extensions(), store.Workspaces(), nil))
	defer runtime.Stop()

	healthyManifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "healthy-pack",
		Name:          "Healthy Pack",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindOperational,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
		StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
		Schema: platformdomain.ExtensionSchemaManifest{
			Name:            "ext_demandops_healthy_pack",
			PackageKey:      "demandops/healthy-pack",
			TargetVersion:   "000001",
			MigrationEngine: "postgres_sql",
		},
		Runtime: platformdomain.ExtensionRuntimeSpec{
			Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []platformdomain.ExtensionEndpoint{
			{
				Name:          "runtime-health",
				Class:         platformdomain.ExtensionEndpointClassHealth,
				MountPath:     "/extensions/healthy-pack/health",
				Methods:       []string{"GET"},
				Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
				ServiceTarget: "runtime.health",
			},
		},
	}

	failingManifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "broken-pack",
		Name:          "Broken Pack",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindOperational,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
		StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
		Schema: platformdomain.ExtensionSchemaManifest{
			Name:            "ext_demandops_broken_pack",
			PackageKey:      "demandops/broken-pack",
			TargetVersion:   "000001",
			MigrationEngine: "postgres_sql",
		},
		Runtime: platformdomain.ExtensionRuntimeSpec{
			Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []platformdomain.ExtensionEndpoint{
			{
				Name:          "runtime-health",
				Class:         platformdomain.ExtensionEndpointClassHealth,
				MountPath:     "/extensions/broken-pack/health",
				Methods:       []string{"GET"},
				Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
				ServiceTarget: "missing.runtime.health",
			},
		},
	}

	healthyInstalled, err := platformdomain.NewInstalledExtension(workspaceOne.ID, "user_123", "lic_healthy", healthyManifest, []byte(`{}`))
	require.NoError(t, err)
	healthyInstalled.Activate()
	require.NoError(t, store.Extensions().CreateInstalledExtension(ctx, healthyInstalled))

	failingInstalled, err := platformdomain.NewInstalledExtension(workspaceTwo.ID, "user_123", "lic_broken", failingManifest, []byte(`{}`))
	require.NoError(t, err)
	failingInstalled.Activate()
	require.NoError(t, store.Extensions().CreateInstalledExtension(ctx, failingInstalled))

	require.NoError(t, runtime.Start(ctx))

	healthyDiagnostics, err := runtime.GetInstalledExtensionRuntimeDiagnostics(ctx, healthyInstalled)
	require.NoError(t, err)
	assert.Equal(t, "bootstrapped", healthyDiagnostics.BootstrapStatus)
	assert.NotNil(t, healthyDiagnostics.LastBootstrapAt)
	require.Len(t, healthyDiagnostics.Endpoints, 1)
	assert.Equal(t, "healthy", healthyDiagnostics.Endpoints[0].Status)

	failingDiagnostics, err := runtime.GetInstalledExtensionRuntimeDiagnostics(ctx, failingInstalled)
	require.NoError(t, err)
	assert.Equal(t, "failed", failingDiagnostics.BootstrapStatus)
	assert.NotNil(t, failingDiagnostics.LastBootstrapAt)
	require.NotNil(t, failingDiagnostics.Endpoints)
	assert.Contains(t, failingDiagnostics.LastBootstrapError, "missing.runtime.health")
}

func TestRuntimeBacksOffFailingJobs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	registry := &Registry{}
	registry.Register("runtime.health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	registry.RegisterScheduledJob("failing.job", func(context.Context) error {
		return fmt.Errorf("job boom")
	})

	runtime := NewRuntime(registry, WithBackgroundRuntimeDeps(eventbus.NewInMemoryBus(), store.Extensions(), store.Workspaces(), nil))
	defer runtime.Stop()

	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "runtime-pack",
		Name:          "Runtime Pack",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindOperational,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		RuntimeClass:  platformdomain.ExtensionRuntimeClassServiceBacked,
		StorageClass:  platformdomain.ExtensionStorageClassOwnedSchema,
		Schema: platformdomain.ExtensionSchemaManifest{
			Name:            "ext_demandops_runtime_pack",
			PackageKey:      "demandops/runtime-pack",
			TargetVersion:   "000001",
			MigrationEngine: "postgres_sql",
		},
		Runtime: platformdomain.ExtensionRuntimeSpec{
			Protocol: platformdomain.ExtensionRuntimeProtocolInProcessHTTP,
		},
		Endpoints: []platformdomain.ExtensionEndpoint{
			{
				Name:          "runtime-health",
				Class:         platformdomain.ExtensionEndpointClassHealth,
				MountPath:     "/extensions/runtime-pack/health",
				Methods:       []string{"GET"},
				Auth:          platformdomain.ExtensionEndpointAuthInternalOnly,
				ServiceTarget: "runtime.health",
			},
		},
		ScheduledJobs: []platformdomain.ExtensionScheduledJob{
			{
				Name:            "maintenance",
				IntervalSeconds: 1,
				ServiceTarget:   "failing.job",
			},
		},
	}

	installed, err := platformdomain.NewInstalledExtension(workspace.ID, "user_123", "lic_runtime", manifest, []byte(`{}`))
	require.NoError(t, err)
	installed.Activate()
	require.NoError(t, store.Extensions().CreateInstalledExtension(ctx, installed))
	require.NoError(t, runtime.EnsureInstalledExtensionRuntime(ctx, installed))

	require.Eventually(t, func() bool {
		diagnostics, err := runtime.GetInstalledExtensionRuntimeDiagnostics(ctx, installed)
		require.NoError(t, err)
		return len(diagnostics.ScheduledJobs) == 1 &&
			diagnostics.ScheduledJobs[0].ConsecutiveFailures >= 1 &&
			diagnostics.ScheduledJobs[0].BackoffUntil != nil
	}, 2*time.Second, 50*time.Millisecond)
}
