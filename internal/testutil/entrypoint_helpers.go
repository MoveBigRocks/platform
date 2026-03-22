package testutil

import (
	"os"
	"strings"
	"testing"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
)

// NewTestConfig creates a test configuration for entry point tests.
// Sets up minimal required environment variables and loads config.
func NewTestConfig(t *testing.T) *config.Config {
	t.Helper()

	// Create temp directory for test data
	tmpDir := t.TempDir()
	testDSN := ensureEntrypointTestDatabase(t)

	setTestEnv(t, map[string]string{
		"STORAGE_PATH":    tmpDir,
		"FILESYSTEM_PATH": tmpDir,
		"JWT_SECRET":      "test-secret-at-least-32-chars-long-for-testing",
		"ENVIRONMENT":     "test",
		"EMAIL_BACKEND":   "mock",
		"STORAGE_TYPE":    "filesystem",
		"DATABASE_DSN":    testDSN,
	})

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}

	return cfg
}

// SetupTestEnv sets up common test environment variables.
// Call this at the beginning of entry point tests to ensure proper isolation.
func SetupTestEnv(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	testDSN := ensureEntrypointTestDatabase(t)
	setTestEnv(t, map[string]string{
		"ENVIRONMENT":     "test",
		"STORAGE_PATH":    tmpDir,
		"FILESYSTEM_PATH": tmpDir,
		"JWT_SECRET":      "test-secret-at-least-32-chars-long-for-testing",
		"EMAIL_BACKEND":   "mock",
		"STORAGE_TYPE":    "filesystem",
		"DATABASE_DSN":    testDSN,
		"TRACING_ENABLED": "false",
		"ENABLE_METRICS":  "false",
		"CLAMAV_ADDR":     "",
	})
}

func ensureEntrypointTestDatabase(t *testing.T) string {
	t.Helper()

	if dsn := strings.TrimSpace(os.Getenv("DATABASE_DSN")); dsn != "" {
		return dsn
	}

	dsn, cleanup := SetupTestPostgresDatabase(t)
	t.Cleanup(cleanup)
	return dsn
}

func setTestEnv(t *testing.T, env map[string]string) {
	t.Helper()

	for key, value := range env {
		t.Setenv(key, value)
	}
}

// ValidateConfig performs basic validation that config loaded correctly.
func ValidateConfig(t *testing.T, cfg *config.Config) {
	t.Helper()

	if cfg == nil {
		t.Fatal("config should not be nil")
	}

	if cfg.Server.Environment != "test" {
		t.Errorf("expected environment to be 'test', got %q", cfg.Server.Environment)
	}

	if cfg.Auth.JWTSecret == "" {
		t.Error("JWT secret should not be empty")
	}

	if cfg.Storage.Operational.FileSystemPath == "" {
		t.Error("storage path should not be empty")
	}
}
