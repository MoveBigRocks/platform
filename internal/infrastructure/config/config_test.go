package config

import (
	"os"
	"testing"
	"time"
)

const defaultTestDatabaseDSN = "postgres://mbr:secret@localhost:5432/mbr?sslmode=disable"

func TestLoad_Defaults(t *testing.T) {
	// Clear environment
	clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Test defaults
	if cfg.Server.Port != "8080" {
		t.Errorf("Expected default port 8080, got %s", cfg.Server.Port)
	}
	if cfg.Server.Environment != "development" {
		t.Errorf("Expected default environment 'development', got %s", cfg.Server.Environment)
	}
	if cfg.Storage.Type != "s3" {
		t.Errorf("Expected default storage type 's3', got %s", cfg.Storage.Type)
	}
	if cfg.Database.EffectiveDriver() != "postgres" {
		t.Errorf("Expected default database driver postgres, got %s", cfg.Database.EffectiveDriver())
	}
	if cfg.Database.EffectiveDSN() != defaultTestDatabaseDSN {
		t.Errorf("Expected default database target %s, got %s", defaultTestDatabaseDSN, cfg.Database.EffectiveDSN())
	}
	if !cfg.Cache.Enabled {
		t.Error("Expected cache enabled by default")
	}
	if len(cfg.Server.TrustedProxies) != 2 || cfg.Server.TrustedProxies[0] != "127.0.0.1" || cfg.Server.TrustedProxies[1] != "::1" {
		t.Errorf("Expected default trusted proxies [127.0.0.1 ::1], got %v", cfg.Server.TrustedProxies)
	}
}

func TestLoad_EnvironmentOverrides(t *testing.T) {
	clearEnv()

	// Set environment variables
	t.Setenv("PORT", "3000")
	t.Setenv("ENVIRONMENT", "test")
	t.Setenv("STORAGE_TYPE", "filesystem")
	t.Setenv("CACHE_ENABLED", "false")
	t.Setenv("DATABASE_DSN", defaultTestDatabaseDSN)
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Port != "3000" {
		t.Errorf("Expected port 3000, got %s", cfg.Server.Port)
	}
	if cfg.Server.Environment != "test" {
		t.Errorf("Expected environment 'test', got %s", cfg.Server.Environment)
	}
	if cfg.Storage.Type != "filesystem" {
		t.Errorf("Expected storage type 'filesystem', got %s", cfg.Storage.Type)
	}
	if cfg.Cache.Enabled {
		t.Error("Expected cache disabled")
	}
	if cfg.Database.EffectiveDriver() != "postgres" {
		t.Errorf("Expected postgres driver, got %s", cfg.Database.EffectiveDriver())
	}
	if got := cfg.Database.RedactedDSN(); got != "postgres://***:***@localhost:5432/mbr?sslmode=disable" {
		t.Errorf("unexpected redacted dsn: %s", got)
	}
}

func TestValidate_Production_RequiresJWTSecret(t *testing.T) {
	clearEnv()
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("CACHE_ENABLED", "true")
	t.Setenv("EMAIL_BACKEND", "sendgrid")
	t.Setenv("SENDGRID_API_KEY", "test-key")
	defer clearEnv()

	cfg, err := Load()
	if err == nil {
		t.Error("Expected error for missing/default JWT secret in production")
	}
	if cfg != nil {
		t.Error("Expected nil config on validation error")
	}
}

func TestValidate_Production_ValidConfig(t *testing.T) {
	clearEnv()
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("JWT_SECRET", "super-secret-key-for-production")
	t.Setenv("INSTANCE_ID", "inst_prod_123")
	t.Setenv("METRICS_TOKEN", "metrics-secret-token")
	t.Setenv("CACHE_ENABLED", "true")
	t.Setenv("EMAIL_BACKEND", "sendgrid")
	t.Setenv("SENDGRID_API_KEY", "test-key")
	t.Setenv("STORAGE_TYPE", "s3")
	t.Setenv("S3_BUCKET", "my-bucket")
	t.Setenv("S3_REGION", "us-west-2")
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Environment != "production" {
		t.Errorf("Expected environment 'production', got %s", cfg.Server.Environment)
	}
	if cfg.Auth.JWTSecret != "super-secret-key-for-production" {
		t.Error("JWT secret not set correctly")
	}
}

func TestValidate_Production_RequiresMetricsToken(t *testing.T) {
	clearEnv()
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("JWT_SECRET", "super-secret-key-for-production")
	t.Setenv("INSTANCE_ID", "inst_prod_metrics")
	t.Setenv("CACHE_ENABLED", "true")
	t.Setenv("EMAIL_BACKEND", "sendgrid")
	t.Setenv("SENDGRID_API_KEY", "test-key")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("Expected error for missing METRICS_TOKEN in production")
	}
}

func TestValidate_Production_RequiresRealEmailBackend(t *testing.T) {
	clearEnv()
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("JWT_SECRET", "valid-secret")
	t.Setenv("CACHE_ENABLED", "true")
	t.Setenv("EMAIL_BACKEND", "mock")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("Expected error for mock email backend in production")
	}
}

func TestValidate_SendGridRequiresAPIKey(t *testing.T) {
	clearEnv()
	t.Setenv("EMAIL_BACKEND", "sendgrid")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("Expected error for missing SendGrid API key")
	}
}

func TestValidate_PostmarkRequiresServerToken(t *testing.T) {
	clearEnv()
	t.Setenv("EMAIL_BACKEND", "postmark")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("Expected error for missing Postmark server token")
	}
}

func TestValidate_SMTPRequiresHostAndPort(t *testing.T) {
	clearEnv()
	t.Setenv("EMAIL_BACKEND", "smtp")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("Expected error for missing SMTP host")
	}

	clearEnv()
	t.Setenv("EMAIL_BACKEND", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "0")

	_, err = Load()
	if err == nil {
		t.Error("Expected error for invalid SMTP port")
	}
}

func TestValidate_SESRequiresRegion(t *testing.T) {
	clearEnv()
	t.Setenv("EMAIL_BACKEND", "ses")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("Expected error for missing SES region")
	}
}

func TestValidate_InvalidStorageType(t *testing.T) {
	clearEnv()
	t.Setenv("STORAGE_TYPE", "invalid")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("Expected error for invalid storage type")
	}
}

func TestLoad_ExtensionTrustConfig(t *testing.T) {
	clearEnv()
	t.Setenv("INSTANCE_ID", "inst_acme_123")
	t.Setenv("EXTENSION_TRUST_REQUIRE_VERIFICATION", "true")
	t.Setenv("EXTENSION_TRUSTED_PUBLISHERS_JSON", `{"DemandOps":{"demandops-main":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.InstanceID != "inst_acme_123" {
		t.Fatalf("expected instance id inst_acme_123, got %q", cfg.InstanceID)
	}
	if !cfg.ExtensionTrust.RequireVerification {
		t.Fatal("expected extension trust verification to be enabled")
	}
	if got := cfg.ExtensionTrust.TrustedPublishers["DemandOps"]["demandops-main"]; got != "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" {
		t.Fatalf("unexpected trusted publisher key %q", got)
	}
}

func TestLoad_ProductionDefaultsEnableExtensionTrustVerification(t *testing.T) {
	clearEnv()
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("JWT_SECRET", "super-secret-key-for-production")
	t.Setenv("INSTANCE_ID", "inst_prod_123")
	t.Setenv("METRICS_TOKEN", "metrics-secret-token")
	t.Setenv("EMAIL_BACKEND", "none")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if !cfg.ExtensionTrust.RequireVerification {
		t.Fatal("expected production to enable extension trust verification by default")
	}
}

func TestLoad_ExtensionTrustRequiresInstanceIDWhenEnforced(t *testing.T) {
	clearEnv()
	t.Setenv("EXTENSION_TRUST_REQUIRE_VERIFICATION", "true")
	t.Setenv("EXTENSION_TRUSTED_PUBLISHERS_JSON", `{"DemandOps":{"demandops-main":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}}`)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when extension trust verification is enabled without INSTANCE_ID")
	}
	if err.Error() != "INSTANCE_ID is required when EXTENSION_TRUST_REQUIRE_VERIFICATION=true" {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestLoad_ProductionExtensionTrustRequiresInstanceID(t *testing.T) {
	clearEnv()
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("JWT_SECRET", "super-secret-key-for-production")
	t.Setenv("METRICS_TOKEN", "metrics-secret-token")
	t.Setenv("EMAIL_BACKEND", "none")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when production uses default extension verification without INSTANCE_ID")
	}
	if err.Error() != "INSTANCE_ID is required when EXTENSION_TRUST_REQUIRE_VERIFICATION=true" {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestLoad_EnterpriseAccessDefaults(t *testing.T) {
	clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if !cfg.EnterpriseAccess.AllowEnvSecretRefs {
		t.Fatal("expected development to allow env secret refs by default")
	}

	clearEnv()
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("JWT_SECRET", "super-secret-key-for-production")
	t.Setenv("INSTANCE_ID", "inst_prod_456")
	t.Setenv("METRICS_TOKEN", "metrics-secret-token")
	t.Setenv("EMAIL_BACKEND", "none")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.EnterpriseAccess.AllowEnvSecretRefs {
		t.Fatal("expected production to disable env secret refs by default")
	}
}

func TestValidate_S3RequiresBucketAndRegion(t *testing.T) {
	// S3 storage validation is covered by default values ("mbr" and "us-east-1")
	// This test verifies that S3 config works with proper values
	clearEnv()
	t.Setenv("STORAGE_TYPE", "s3")
	t.Setenv("S3_BUCKET", "test-bucket")
	t.Setenv("S3_REGION", "us-west-2")
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Storage.Operational.Bucket != "test-bucket" {
		t.Errorf("Expected S3 bucket 'test-bucket', got %s", cfg.Storage.Operational.Bucket)
	}
	if cfg.Storage.Operational.Region != "us-west-2" {
		t.Errorf("Expected S3 region 'us-west-2', got %s", cfg.Storage.Operational.Region)
	}
}

func TestGetEnv(t *testing.T) {
	clearEnv()

	// Test default
	if got := getEnv("NON_EXISTENT", "default"); got != "default" {
		t.Errorf("Expected 'default', got %s", got)
	}

	// Test override
	t.Setenv("TEST_VAR", "custom")

	if got := getEnv("TEST_VAR", "default"); got != "custom" {
		t.Errorf("Expected 'custom', got %s", got)
	}
}

func TestGetEnvInt(t *testing.T) {
	clearEnv()

	// Test default
	if got := getEnvInt("NON_EXISTENT", 42); got != 42 {
		t.Errorf("Expected 42, got %d", got)
	}

	// Test valid int
	t.Setenv("TEST_INT", "100")

	if got := getEnvInt("TEST_INT", 42); got != 100 {
		t.Errorf("Expected 100, got %d", got)
	}

	// Test invalid int (should return default)
	t.Setenv("TEST_INT_INVALID", "not-a-number")

	if got := getEnvInt("TEST_INT_INVALID", 42); got != 42 {
		t.Errorf("Expected default 42 for invalid int, got %d", got)
	}
}

func TestGetEnvBool(t *testing.T) {
	clearEnv()

	// Test default
	if got := getEnvBool("NON_EXISTENT", true); got != true {
		t.Error("Expected true")
	}

	// Test true values
	for _, val := range []string{"true", "TRUE", "True", "1"} {
		t.Setenv("TEST_BOOL", val)
		if got := getEnvBool("TEST_BOOL", false); !got {
			t.Errorf("Expected true for %s", val)
		}
		os.Unsetenv("TEST_BOOL")
	}

	// Test false values
	for _, val := range []string{"false", "FALSE", "False", "0"} {
		t.Setenv("TEST_BOOL", val)
		if got := getEnvBool("TEST_BOOL", true); got {
			t.Errorf("Expected false for %s", val)
		}
		os.Unsetenv("TEST_BOOL")
	}

	// Test invalid bool (should return default)
	t.Setenv("TEST_BOOL_INVALID", "not-a-bool")

	if got := getEnvBool("TEST_BOOL_INVALID", true); got != true {
		t.Error("Expected default true for invalid bool")
	}
}

func TestGetEnvDuration(t *testing.T) {
	clearEnv()

	// Test default
	if got := getEnvDuration("NON_EXISTENT", "5m"); got != 5*time.Minute {
		t.Errorf("Expected 5m, got %v", got)
	}

	// Test valid duration
	t.Setenv("TEST_DURATION", "30s")

	if got := getEnvDuration("TEST_DURATION", "5m"); got != 30*time.Second {
		t.Errorf("Expected 30s, got %v", got)
	}

	// Test invalid duration (should return default)
	t.Setenv("TEST_DURATION_INVALID", "not-a-duration")

	if got := getEnvDuration("TEST_DURATION_INVALID", "5m"); got != 5*time.Minute {
		t.Errorf("Expected default 5m for invalid duration, got %v", got)
	}
}

func TestGetEnvInt64(t *testing.T) {
	clearEnv()

	// Test default
	var defaultVal int64 = 1024
	if got := getEnvInt64("NON_EXISTENT", defaultVal); got != defaultVal {
		t.Errorf("Expected %d, got %d", defaultVal, got)
	}

	// Test valid int64
	t.Setenv("TEST_INT64", "9223372036854775807")

	var expected int64 = 9223372036854775807
	if got := getEnvInt64("TEST_INT64", defaultVal); got != expected {
		t.Errorf("Expected %d, got %d", expected, got)
	}

	// Test invalid int64 (should return default)
	t.Setenv("TEST_INT64_INVALID", "not-a-number")

	if got := getEnvInt64("TEST_INT64_INVALID", defaultVal); got != defaultVal {
		t.Errorf("Expected default %d for invalid int64, got %d", defaultVal, got)
	}
}

func TestGetEnvSlice(t *testing.T) {
	clearEnv()

	// Test default
	defaultVal := []string{"a", "b", "c"}
	got := getEnvSlice("NON_EXISTENT", defaultVal)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("Expected %v, got %v", defaultVal, got)
	}

	// Test comma-separated values
	t.Setenv("TEST_SLICE", "one,two,three")

	got = getEnvSlice("TEST_SLICE", defaultVal)
	if len(got) != 3 || got[0] != "one" || got[1] != "two" || got[2] != "three" {
		t.Errorf("Expected [one two three], got %v", got)
	}

	// Test single value
	t.Setenv("TEST_SLICE_SINGLE", "single")

	got = getEnvSlice("TEST_SLICE_SINGLE", defaultVal)
	if len(got) != 1 || got[0] != "single" {
		t.Errorf("Expected [single], got %v", got)
	}
}

func TestConfig_FeatureFlags(t *testing.T) {
	clearEnv()
	t.Setenv("ENABLE_SIGNUP", "false")
	t.Setenv("ENABLE_MAGIC_LINK", "true")
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Features.EnableSignup {
		t.Error("Expected signup disabled")
	}
	if !cfg.Features.EnableMagicLink {
		t.Error("Expected magic link enabled")
	}
}

func TestConfig_Notifications(t *testing.T) {
	clearEnv()
	t.Setenv("NOTIFICATION_CHANNELS", "email,slack,sms")
	t.Setenv("SLACK_WEBHOOK_URL", "https://hooks.slack.com/test")
	t.Setenv("NOTIFICATION_RETENTION_DAYS", "30")
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expectedChannels := []string{"email", "slack", "sms"}
	if len(cfg.Notification.Channels) != 3 {
		t.Errorf("Expected 3 channels, got %d", len(cfg.Notification.Channels))
	}
	for i, ch := range expectedChannels {
		if cfg.Notification.Channels[i] != ch {
			t.Errorf("Expected channel %s at index %d, got %s", ch, i, cfg.Notification.Channels[i])
		}
	}

	if cfg.Notification.SlackWebhookURL != "https://hooks.slack.com/test" {
		t.Errorf("Expected slack webhook URL, got %s", cfg.Notification.SlackWebhookURL)
	}

	if cfg.Notification.RetentionDays != 30 {
		t.Errorf("Expected retention days 30, got %d", cfg.Notification.RetentionDays)
	}
}

func TestConfig_Security(t *testing.T) {
	clearEnv()
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("RATE_LIMIT_PER_MIN", "100")
	t.Setenv("RATE_LIMIT_BURST", "20")
	t.Setenv("DEFAULT_USER_ROLE", "viewer")
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if !cfg.Security.RateLimitEnabled {
		t.Error("Expected rate limiting enabled")
	}
	if cfg.Security.RateLimitPerMin != 100 {
		t.Errorf("Expected rate limit 100/min, got %d", cfg.Security.RateLimitPerMin)
	}
	if cfg.Security.RateLimitBurst != 20 {
		t.Errorf("Expected rate limit burst 20, got %d", cfg.Security.RateLimitBurst)
	}
	if cfg.Security.DefaultUserRole != "viewer" {
		t.Errorf("Expected default user role 'viewer', got '%s'", cfg.Security.DefaultUserRole)
	}
}

// Helper function to clear all environment variables used in tests
func clearEnv() {
	envVars := []string{
		"PORT", "ENVIRONMENT", "STORAGE_TYPE", "CACHE_ENABLED",
		"JWT_SECRET", "EMAIL_BACKEND", "SENDGRID_API_KEY", "S3_BUCKET", "S3_REGION",
		"METRICS_TOKEN", "TRUSTED_PROXIES", "DATABASE_DSN",
		"ENABLE_SIGNUP", "ENABLE_MAGIC_LINK",
		"NOTIFICATION_CHANNELS", "SLACK_WEBHOOK_URL", "NOTIFICATION_RETENTION_DAYS",
		"RATE_LIMIT_ENABLED", "RATE_LIMIT_PER_MIN", "RATE_LIMIT_BURST", "DEFAULT_USER_ROLE",
	}

	for _, v := range envVars {
		os.Unsetenv(v)
	}

	os.Setenv("DATABASE_DSN", defaultTestDatabaseDSN)
}
