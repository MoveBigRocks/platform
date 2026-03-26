//go:build integration

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/container"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestConfigLoad(t *testing.T) {
	testutil.SetupTestEnv(t)

	cfg, err := config.Load()
	require.NoError(t, err, "config should load without error")
	assert.NotNil(t, cfg, "config should not be nil")

	// Validate basic config structure
	testutil.ValidateConfig(t, cfg)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		jwtSecret   string
		wantErr     bool
	}{
		{
			name:        "test environment with test secret",
			environment: "test",
			jwtSecret:   "test-secret-at-least-32-chars-long",
			wantErr:     false,
		},
		{
			name:        "development environment with default secret",
			environment: "development",
			jwtSecret:   "change-me-in-production",
			wantErr:     false,
		},
		{
			name:        "production environment requires proper JWT secret",
			environment: "production",
			jwtSecret:   "change-me-in-production",
			wantErr:     true,
		},
		{
			name:        "production environment with proper secret",
			environment: "production",
			jwtSecret:   "proper-production-secret-that-is-long-enough",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.SetupTestEnv(t)

			// Override environment and JWT secret for this test
			t.Setenv("ENVIRONMENT", tt.environment)
			t.Setenv("JWT_SECRET", tt.jwtSecret)

			// Production also requires non-mock email backend
			if tt.environment == "production" {
				t.Setenv("EMAIL_BACKEND", "postmark")
				t.Setenv("POSTMARK_SERVER_TOKEN", "test-token")
				t.Setenv("METRICS_TOKEN", "test-metrics-token")
			}

			cfg, err := config.Load()

			if tt.wantErr {
				assert.Error(t, err, "expected config validation to fail")
			} else {
				assert.NoError(t, err, "expected config validation to pass")
				assert.NotNil(t, cfg, "config should not be nil on success")
			}
		})
	}
}

func TestContainerInitialization(t *testing.T) {
	testutil.SetupTestEnv(t)

	cfg := testutil.NewTestConfig(t)
	require.NotNil(t, cfg)

	// Initialize container with all services
	c, err := container.New(cfg, container.Options{
		Version:   "test",
		GitCommit: "test-commit",
		BuildDate: "2024-01-01",
	})
	require.NoError(t, err, "container should initialize without error")
	require.NotNil(t, c, "container should not be nil")

	// Verify core components are initialized
	assert.NotNil(t, c.Logger, "logger should be initialized")
	assert.NotNil(t, c.Store, "store should be initialized")
	assert.NotNil(t, c.EventBus, "event bus should be initialized")
	assert.NotNil(t, c.Outbox, "outbox should be initialized")

	// Verify domain containers are initialized
	assert.NotNil(t, c.Platform, "platform container should be initialized")
	assert.NotNil(t, c.Service, "service container should be initialized")
	assert.NotNil(t, c.Automation, "automation container should be initialized")

	// Verify platform services
	assert.NotNil(t, c.Platform.Session, "session service should be initialized")
	assert.NotNil(t, c.Platform.Stats, "admin stats service should be initialized")
	assert.NotNil(t, c.Platform.Workspace, "workspace service should be initialized")
	assert.NotNil(t, c.Platform.User, "user service should be initialized")
	assert.NotNil(t, c.Platform.Agent, "agent service should be initialized")
	assert.NotNil(t, c.Platform.Contact, "contact service should be initialized")

	// Verify support services
	assert.NotNil(t, c.Service.Case, "case service should be initialized")
	assert.NotNil(t, c.Service.Email, "email service should be initialized")
	// Attachment service is optional (only initialized if S3 is configured)
	if c.Service.Attachment != nil {
		assert.NotNil(t, c.Service.Attachment, "attachment service should be initialized when dependencies are configured")
	}

	// Verify automation services
	assert.NotNil(t, c.Automation.Rule, "rule service should be initialized")
	assert.NotNil(t, c.Automation.Engine, "rules engine should be initialized")
	assert.NotNil(t, c.Automation.Form, "form service should be initialized")

	// Verify health checker
	assert.NotNil(t, c.HealthChecker, "health checker should be initialized")
}

func TestContainerLifecycle(t *testing.T) {
	testutil.SetupTestEnv(t)

	cfg := testutil.NewTestConfig(t)
	require.NotNil(t, cfg)

	c, err := container.New(cfg, container.Options{
		Version:   "test",
		GitCommit: "test-commit",
		BuildDate: "2024-01-01",
	})
	require.NoError(t, err)
	require.NotNil(t, c)

	// Test starting background services
	ctx := t.Context()
	err = c.Start(ctx)
	assert.NoError(t, err, "container should start without error")

	// Test stopping background services
	err = c.Stop(0) // Immediate stop for tests
	assert.NoError(t, err, "container should stop without error")
}

func TestInitializeMetrics(t *testing.T) {
	testutil.SetupTestEnv(t)

	// Setup test store
	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	// Create test workspace
	workspaceID := testutil.CreateTestWorkspace(t, store, "test-workspace")

	// Initialize metrics should not panic
	assert.NotPanics(t, func() {
		initializeMetrics(t.Context(), store, nil)
	}, "initializeMetrics should not panic")

	// Verify workspace was counted
	// (metrics are set but we can't easily verify prometheus gauges in tests)
	workspace, err := store.Workspaces().GetWorkspace(t.Context(), workspaceID)
	require.NoError(t, err)
	assert.True(t, workspace.IsAccessible())
}
