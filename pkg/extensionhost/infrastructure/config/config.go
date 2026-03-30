package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port         string
	Environment  string
	Domain       string // Main domain (e.g., "movebigrocks.com", "staging.movebigrocks.com", "lvh.me")
	BaseURL      string // Public site (e.g., "https://movebigrocks.com")
	AdminBaseURL string // Admin panel (e.g., "https://admin.movebigrocks.com")
	APIBaseURL   string // API endpoints (e.g., "https://api.movebigrocks.com")
	// TrustedProxies controls which reverse proxies are trusted for X-Forwarded-For.
	// Default is loopback-only for local Caddy/nginx deployments.
	TrustedProxies []string
}

// StorageConfig holds all storage-related configuration
type StorageConfig struct {
	Type              string // "s3" or "filesystem"
	EncryptionEnabled bool
	EncryptionKey     []byte

	// Operational storage (workspaces, users, projects, issues, cases)
	Operational S3Config

	// Analytics storage (usage metrics, performance metrics - high volume)
	Analytics PathConfig

	// Audit log storage (compliance, security events - 7-year retention)
	Audit PathConfig

	// Versioned artifact storage (knowledge repos, published extension content)
	Artifacts PathConfig

	// Attachments storage (email attachments, uploads - S3 only)
	Attachments S3BucketConfig

	// Backups storage (filesystem backup destination)
	Backups S3BucketConfig
}

// S3Config holds S3 configuration for operational storage
type S3Config struct {
	Bucket         string
	Region         string
	Endpoint       string // For MinIO
	AccessKey      string
	SecretKey      string
	FileSystemPath string // For filesystem fallback
}

// PathConfig holds path-based storage config (S3 bucket + filesystem path)
type PathConfig struct {
	S3Bucket string
	Path     string
}

// S3BucketConfig holds simple S3 bucket config
type S3BucketConfig struct {
	Bucket string
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret    string
	JWTExpiry    time.Duration
	MagicLinkTTL time.Duration
	CookieDomain string // Domain for session cookies (e.g., ".example.com" for cross-subdomain)
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	DefaultUserRole  string
	RateLimitEnabled bool
	RateLimitPerMin  int
	RateLimitBurst   int
}

// EmailConfig holds email configuration
type EmailConfig struct {
	Backend               string // "sendgrid", "postmark", "ses", "mock", "none"
	SendGridAPIKey        string
	PostmarkServerToken   string
	PostmarkAccountToken  string
	PostmarkWebhookSecret string
	SESRegion             string
	SESAccessKey          string
	SESSecretKey          string
	SMTPHost              string
	SMTPPort              int
	SMTPUsername          string
	SMTPPassword          string
	FromEmail             string
	FromName              string
	MaxRetries            int
	RetryDelay            time.Duration
	MaxEmailSize          int64
	SpamThreshold         float64
}

// NotificationConfig holds notification channel configuration
type NotificationConfig struct {
	Channels          []string
	SlackWebhookURL   string
	SlackBotToken     string
	TeamsWebhookURL   string
	DiscordWebhookURL string
	TwilioAccountSID  string
	TwilioAuthToken   string
	TwilioFromPhone   string
	PushEnabled       bool
	FCMServerKey      string
	APNSKeyPath       string
	RetentionDays     int
}

// AuditConfig holds audit logging configuration
type AuditConfig struct {
	RetentionDays          int
	LogLevel               string
	EnableSecurityEvents   bool
	SecurityEventThreshold int
	ExportFormats          []string
	ComplianceMode         string
}

// LimitsConfig holds performance and limit settings
type LimitsConfig struct {
	MaxRequestSize int64
	MaxUploadSize  int64
	EnableMetrics  bool
	MetricsPort    string
	MetricsToken   string // Bearer token required to access /metrics (empty = require localhost)

	// Custom Fields
	MaxCustomFieldsPerEntity int
	CustomFieldValidation    bool
	CustomFieldEncryption    bool

	// Reporting
	ReportCacheTimeout     time.Duration
	MaxReportRows          int
	ReportExportFormats    []string
	EnableScheduledReports bool
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	Timeout        time.Duration
	BufferSize     int
	MaxConnections int
	OriginCheck    bool
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Enabled bool
	TTL     time.Duration
}

// IntegrationsConfig holds external integration configuration
type IntegrationsConfig struct {
	OpenAIAPIKey     string
	AnthropicAPIKey  string
	ElasticsearchURL string
	ClamAVAddr       string
	ClamAVTimeout    time.Duration
	PrometheusURL    string
	GrafanaURL       string
}

// TracingConfig holds distributed tracing configuration
type TracingConfig struct {
	Enabled      bool
	Exporter     string  // "jaeger", "otlp", "stdout", or "none"
	JaegerURL    string  // Jaeger collector URL (e.g., http://jaeger:14268/api/traces)
	OTLPEndpoint string  // OTLP endpoint (e.g., localhost:4317)
	OTLPInsecure bool    // Use insecure connection for OTLP
	ServiceName  string  // Service name for traces
	SampleRate   float64 // Sampling rate (0.0-1.0)
	Environment  string  // Environment tag (production, staging, development)
}

// FeaturesConfig holds feature flags
type FeaturesConfig struct {
	EnableSignup    bool
	EnableMagicLink bool
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	DSN string // PostgreSQL connection string
}

// EventBusConfig holds event bus configuration
type EventBusConfig struct {
	BufferSize   int           // Event notification buffer size (default: 10000)
	PollInterval time.Duration // Polling fallback interval (default: 5s)
	EnableDLQ    bool          // Enable dead letter queue (default: true)
}

// OutboxConfig holds outbox service configuration
type OutboxConfig struct {
	PollInterval     time.Duration // How often to poll for pending events (default: 5s)
	MaxRetries       int           // Maximum retry attempts before giving up (default: 10)
	RetentionDays    int           // How long to keep published events (default: 7)
	BatchSize        int           // Number of events to process per batch (default: 100)
	MaxBackoff       time.Duration // Maximum backoff between retries (default: 5m)
	HealthMaxPending int           // Max pending events before unhealthy (default: 100)
	HealthMaxAge     time.Duration // Max age of oldest event before unhealthy (default: 5m)
}

// ErrorProcessingConfig holds error processor configuration
type ErrorProcessingConfig struct {
	WorkerCount     int           // Number of error processing workers (default: 4)
	QueueSize       int           // Event queue buffer size (default: 1000)
	RateLimitWindow time.Duration // Rate limiting window (default: 1h)
	RateLimitBlock  time.Duration // Block duration after exceeding limit (default: 5m)
}

// DatabasePoolConfig holds database connection pool configuration
type DatabasePoolConfig struct {
	MaxOpenConns    int           // Maximum open connections (default: 25)
	MaxIdleConns    int           // Maximum idle connections (default: 5)
	ConnMaxLifetime time.Duration // Maximum connection lifetime (default: 5m)
	ConnMaxIdleTime time.Duration // Maximum idle time before closing (default: 5m)
}

// AdminConfig holds admin configuration
type AdminConfig struct {
	Emails []string
}

// ExtensionTrustConfig holds extension bundle trust policy configuration.
type ExtensionTrustConfig struct {
	RequireVerification bool
	TrustedPublishers   map[string]map[string]string
}

// EnterpriseAccessConfig holds privileged identity-provider policy controls.
type EnterpriseAccessConfig struct {
	AllowedHosts       []string
	AllowEnvSecretRefs bool
}

// Config holds all application configuration with nested sub-configs
type Config struct {
	Server               ServerConfig
	InstanceID           string
	Database             DatabaseConfig
	TestDatabaseAdminDSN string
	ExtensionRuntimeDir  string
	DatabasePool         DatabasePoolConfig
	Storage              StorageConfig
	Auth                 AuthConfig
	Security             SecurityConfig
	Email                EmailConfig
	Notification         NotificationConfig
	Audit                AuditConfig
	Limits               LimitsConfig
	WebSocket            WebSocketConfig
	Cache                CacheConfig
	Integrations         IntegrationsConfig
	Tracing              TracingConfig
	Features             FeaturesConfig
	Admin                AdminConfig
	ExtensionTrust       ExtensionTrustConfig
	EnterpriseAccess     EnterpriseAccessConfig
	EventBus             EventBusConfig
	Outbox               OutboxConfig
	ErrorProcessing      ErrorProcessingConfig
	GeoIPDBPath          string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Ignore error: .env file is optional
	_ = godotenv.Load() //nolint:errcheck // .env file is optional

	// First, determine environment and domain for URL generation
	environment := getEnv("ENVIRONMENT", "development")
	domain := getEnv("DOMAIN", "movebigrocks.com")
	port := getEnv("PORT", "8080")

	// Protocol selection based on environment
	protocol := "https"
	if environment == "development" {
		protocol = "http"
		// Auto-switch to lvh.me for local subdomain testing
		if domain == "movebigrocks.com" {
			domain = "lvh.me"
		}
	}

	// Generate subdomain URLs from domain
	defaultBaseURL := fmt.Sprintf("%s://%s", protocol, domain)
	defaultAdminURL := fmt.Sprintf("%s://admin.%s", protocol, domain)
	defaultAPIURL := fmt.Sprintf("%s://api.%s", protocol, domain)

	// Add port suffix in development
	if environment == "development" {
		defaultBaseURL = fmt.Sprintf("%s:%s", defaultBaseURL, port)
		defaultAdminURL = fmt.Sprintf("%s:%s", defaultAdminURL, port)
		defaultAPIURL = fmt.Sprintf("%s:%s", defaultAPIURL, port)
	}

	// Default storage paths
	defaultStoragePath := getEnv("FILESYSTEM_PATH", "./data")
	defaultS3Bucket := getEnv("S3_BUCKET", "mbr")

	cfg := &Config{
		Server: ServerConfig{
			Port:         port,
			Environment:  environment,
			Domain:       domain,
			BaseURL:      getEnv("BASE_URL", defaultBaseURL),
			AdminBaseURL: getEnv("ADMIN_BASE_URL", defaultAdminURL),
			APIBaseURL:   getEnv("API_BASE_URL", defaultAPIURL),
			TrustedProxies: getEnvSlice("TRUSTED_PROXIES", []string{
				"127.0.0.1",
				"::1",
			}),
		},
		InstanceID: getEnv("INSTANCE_ID", ""),

		Database: DatabaseConfig{
			DSN: strings.TrimSpace(getEnv("DATABASE_DSN", "")),
		},
		TestDatabaseAdminDSN: getEnv("TEST_DATABASE_ADMIN_DSN", ""),
		ExtensionRuntimeDir:  getEnv("EXTENSION_RUNTIME_DIR", "./tmp/extensions"),

		Storage: StorageConfig{
			Type:              getEnv("STORAGE_TYPE", "s3"),
			EncryptionEnabled: getEnvBool("STORAGE_ENCRYPTION_ENABLED", false),
			EncryptionKey:     nil, // Will be set below if encryption is enabled

			Operational: S3Config{
				Bucket:         defaultS3Bucket,
				Region:         getEnv("S3_REGION", "us-east-1"),
				Endpoint:       getEnv("S3_ENDPOINT", ""),
				AccessKey:      getEnv("AWS_ACCESS_KEY_ID", ""),
				SecretKey:      getEnv("AWS_SECRET_ACCESS_KEY", ""),
				FileSystemPath: getEnv("STORAGE_PATH", defaultStoragePath),
			},

			Analytics: PathConfig{
				S3Bucket: getEnv("ANALYTICS_S3_BUCKET", defaultS3Bucket+"-analytics"),
				Path:     getEnv("ANALYTICS_PATH", defaultStoragePath+"/analytics"),
			},

			Audit: PathConfig{
				S3Bucket: getEnv("AUDIT_S3_BUCKET", defaultS3Bucket+"-audit"),
				Path:     getEnv("AUDIT_PATH", defaultStoragePath+"/audit"),
			},

			Artifacts: PathConfig{
				S3Bucket: getEnv("ARTIFACTS_S3_BUCKET", defaultS3Bucket+"-artifacts"),
				Path:     getEnv("ARTIFACTS_PATH", defaultStoragePath+"/artifacts"),
			},

			Attachments: S3BucketConfig{
				Bucket: getEnv("S3_ATTACHMENTS_BUCKET", defaultS3Bucket+"-attachments"),
			},

			Backups: S3BucketConfig{
				Bucket: getEnv("S3_BACKUPS_BUCKET", defaultS3Bucket+"-backups"),
			},
		},

		Auth: AuthConfig{
			JWTSecret:    getEnv("JWT_SECRET", "change-me-in-production"),
			JWTExpiry:    getEnvDuration("JWT_EXPIRY", "168h"), // 7 days
			MagicLinkTTL: getEnvDuration("MAGIC_LINK_TTL", "15m"),
			CookieDomain: getEnv("COOKIE_DOMAIN", ""), // Empty for host-only, ".example.com" for cross-subdomain
		},

		Security: SecurityConfig{
			DefaultUserRole:  getEnv("DEFAULT_USER_ROLE", "user"),
			RateLimitEnabled: getEnvBool("RATE_LIMIT_ENABLED", true),
			RateLimitPerMin:  getEnvInt("RATE_LIMIT_PER_MIN", 60),
			RateLimitBurst:   getEnvInt("RATE_LIMIT_BURST", 10),
		},

		Email: EmailConfig{
			Backend:               getEnv("EMAIL_BACKEND", "mock"),
			SendGridAPIKey:        getEnv("SENDGRID_API_KEY", ""),
			PostmarkServerToken:   getEnv("POSTMARK_SERVER_TOKEN", ""),
			PostmarkAccountToken:  getEnv("POSTMARK_ACCOUNT_TOKEN", ""),
			PostmarkWebhookSecret: getEnv("POSTMARK_WEBHOOK_SECRET", ""),
			SESRegion:             getEnv("SES_REGION", ""),
			SESAccessKey:          getEnv("SES_ACCESS_KEY", ""),
			SESSecretKey:          getEnv("SES_SECRET_KEY", ""),
			SMTPHost:              getEnv("SMTP_HOST", ""),
			SMTPPort:              getEnvInt("SMTP_PORT", 587),
			SMTPUsername:          getEnv("SMTP_USERNAME", ""),
			SMTPPassword:          getEnv("SMTP_PASSWORD", ""),
			FromEmail:             getEnv("FROM_EMAIL", fmt.Sprintf("noreply@%s", domain)),
			FromName:              getEnv("FROM_NAME", "Move Big Rocks"),
			MaxRetries:            getEnvInt("EMAIL_MAX_RETRIES", 3),
			RetryDelay:            getEnvDuration("EMAIL_RETRY_DELAY", "5s"),
			MaxEmailSize:          getEnvInt64("EMAIL_MAX_SIZE", 25*1024*1024),
			SpamThreshold:         getEnvFloat64("EMAIL_SPAM_THRESHOLD", 0.7),
		},

		Notification: NotificationConfig{
			Channels:          getEnvSlice("NOTIFICATION_CHANNELS", []string{"email", "slack"}),
			SlackWebhookURL:   getEnv("SLACK_WEBHOOK_URL", ""),
			SlackBotToken:     getEnv("SLACK_BOT_TOKEN", ""),
			TeamsWebhookURL:   getEnv("TEAMS_WEBHOOK_URL", ""),
			DiscordWebhookURL: getEnv("DISCORD_WEBHOOK_URL", ""),
			TwilioAccountSID:  getEnv("TWILIO_ACCOUNT_SID", ""),
			TwilioAuthToken:   getEnv("TWILIO_AUTH_TOKEN", ""),
			TwilioFromPhone:   getEnv("TWILIO_FROM_PHONE", ""),
			PushEnabled:       getEnvBool("PUSH_NOTIFICATIONS_ENABLED", false),
			FCMServerKey:      getEnv("FCM_SERVER_KEY", ""),
			APNSKeyPath:       getEnv("APNS_KEY_PATH", ""),
			RetentionDays:     getEnvInt("NOTIFICATION_RETENTION_DAYS", 90),
		},

		Audit: AuditConfig{
			RetentionDays:          getEnvInt("AUDIT_LOG_RETENTION_DAYS", 365),
			LogLevel:               getEnv("AUDIT_LOG_LEVEL", "INFO"),
			EnableSecurityEvents:   getEnvBool("ENABLE_SECURITY_EVENTS", true),
			SecurityEventThreshold: getEnvInt("SECURITY_EVENT_THRESHOLD", 10),
			ExportFormats:          getEnvSlice("AUDIT_EXPORT_FORMATS", []string{"csv", "json"}),
			ComplianceMode:         getEnv("COMPLIANCE_MODE", "general"),
		},

		Limits: LimitsConfig{
			MaxRequestSize: getEnvInt64("MAX_REQUEST_SIZE", 32*1024*1024),
			MaxUploadSize:  getEnvInt64("MAX_UPLOAD_SIZE", 100*1024*1024),
			EnableMetrics:  getEnvBool("ENABLE_METRICS", true),
			MetricsPort:    getEnv("METRICS_PORT", "9090"),
			MetricsToken:   getEnv("METRICS_TOKEN", ""), // Empty = localhost only

			MaxCustomFieldsPerEntity: getEnvInt("MAX_CUSTOM_FIELDS_PER_ENTITY", 50),
			CustomFieldValidation:    getEnvBool("CUSTOM_FIELD_VALIDATION", true),
			CustomFieldEncryption:    getEnvBool("CUSTOM_FIELD_ENCRYPTION", false),

			ReportCacheTimeout:     getEnvDuration("REPORT_CACHE_TIMEOUT", "5m"),
			MaxReportRows:          getEnvInt("MAX_REPORT_ROWS", 10000),
			ReportExportFormats:    getEnvSlice("REPORT_EXPORT_FORMATS", []string{"csv", "pdf", "json"}),
			EnableScheduledReports: getEnvBool("ENABLE_SCHEDULED_REPORTS", true),
		},

		WebSocket: WebSocketConfig{
			Timeout:        getEnvDuration("WEBSOCKET_TIMEOUT", "30s"),
			BufferSize:     getEnvInt("WEBSOCKET_BUFFER_SIZE", 1024),
			MaxConnections: getEnvInt("MAX_WEBSOCKET_CONNECTIONS", 1000),
			OriginCheck:    getEnvBool("WEBSOCKET_ORIGIN_CHECK", true),
		},

		Cache: CacheConfig{
			Enabled: getEnvBool("CACHE_ENABLED", true),
			TTL:     getEnvDuration("CACHE_TTL", "1h"),
		},

		Integrations: IntegrationsConfig{
			OpenAIAPIKey:     getEnv("OPENAI_API_KEY", ""),
			AnthropicAPIKey:  getEnv("ANTHROPIC_API_KEY", ""),
			ElasticsearchURL: getEnv("ELASTICSEARCH_URL", ""),
			ClamAVAddr:       getEnv("CLAMAV_ADDR", ""),
			ClamAVTimeout:    getEnvDuration("CLAMAV_TIMEOUT", "30s"),
			PrometheusURL:    getEnv("PROMETHEUS_URL", "http://127.0.0.1:9090"),
			GrafanaURL:       getEnv("GRAFANA_URL", "http://127.0.0.1:3000"),
		},

		Tracing: TracingConfig{
			Enabled:      getEnvBool("TRACING_ENABLED", false),
			Exporter:     getEnv("TRACING_EXPORTER", "jaeger"),
			JaegerURL:    getEnv("JAEGER_URL", "http://jaeger:14268/api/traces"),
			OTLPEndpoint: getEnv("OTLP_ENDPOINT", "localhost:4317"),
			OTLPInsecure: getEnvBool("OTLP_INSECURE", true),
			ServiceName:  getEnv("TRACING_SERVICE_NAME", "mbr"),
			SampleRate:   getEnvFloat64("TRACING_SAMPLE_RATE", 0.1),
			Environment:  environment,
		},

		Features: FeaturesConfig{
			EnableSignup:    getEnvBool("ENABLE_SIGNUP", true),
			EnableMagicLink: getEnvBool("ENABLE_MAGIC_LINK", true),
		},

		Admin: AdminConfig{
			Emails: getEnvSlice("ADMIN_EMAILS", []string{}),
		},

		ExtensionTrust: ExtensionTrustConfig{
			RequireVerification: getEnvBool("EXTENSION_TRUST_REQUIRE_VERIFICATION", environment == "production"),
			TrustedPublishers:   map[string]map[string]string{},
		},

		EnterpriseAccess: EnterpriseAccessConfig{
			AllowedHosts:       getEnvSlice("ENTERPRISE_ACCESS_ALLOWED_HOSTS", []string{}),
			AllowEnvSecretRefs: getEnvBool("ENTERPRISE_ACCESS_ALLOW_ENV_SECRET_REFS", environment != "production"),
		},

		EventBus: EventBusConfig{
			BufferSize:   getEnvInt("EVENTBUS_BUFFER_SIZE", 10000),
			PollInterval: getEnvDuration("EVENTBUS_POLL_INTERVAL", "5s"),
			EnableDLQ:    getEnvBool("EVENTBUS_ENABLE_DLQ", true),
		},

		Outbox: OutboxConfig{
			PollInterval:     getEnvDuration("OUTBOX_POLL_INTERVAL", "5s"),
			MaxRetries:       getEnvInt("OUTBOX_MAX_RETRIES", 10),
			RetentionDays:    getEnvInt("OUTBOX_RETENTION_DAYS", 7),
			BatchSize:        getEnvInt("OUTBOX_BATCH_SIZE", 100),
			MaxBackoff:       getEnvDuration("OUTBOX_MAX_BACKOFF", "5m"),
			HealthMaxPending: getEnvInt("OUTBOX_HEALTH_MAX_PENDING", 100),
			HealthMaxAge:     getEnvDuration("OUTBOX_HEALTH_MAX_AGE", "5m"),
		},

		ErrorProcessing: ErrorProcessingConfig{
			WorkerCount:     getEnvInt("ERROR_WORKER_COUNT", 4),
			QueueSize:       getEnvInt("ERROR_QUEUE_SIZE", 1000),
			RateLimitWindow: getEnvDuration("ERROR_RATE_LIMIT_WINDOW", "1h"),
			RateLimitBlock:  getEnvDuration("ERROR_RATE_LIMIT_BLOCK", "5m"),
		},

		DatabasePool: DatabasePoolConfig{
			MaxOpenConns:    getEnvInt("DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DATABASE_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DATABASE_CONN_MAX_LIFETIME", "5m"),
			ConnMaxIdleTime: getEnvDuration("DATABASE_CONN_MAX_IDLE_TIME", "5m"),
		},

		GeoIPDBPath: getEnv("GEOIP_DB_PATH", ""),
	}

	if trustedPublisherJSON := strings.TrimSpace(getEnv("EXTENSION_TRUSTED_PUBLISHERS_JSON", "")); trustedPublisherJSON != "" {
		if err := json.Unmarshal([]byte(trustedPublisherJSON), &cfg.ExtensionTrust.TrustedPublishers); err != nil {
			return nil, fmt.Errorf("invalid EXTENSION_TRUSTED_PUBLISHERS_JSON: %w", err)
		}
	}

	// Storage Encryption Configuration
	if cfg.Storage.EncryptionEnabled {
		encKeyStr := getEnv("STORAGE_ENCRYPTION_KEY", "")
		if encKeyStr == "" {
			return nil, fmt.Errorf("STORAGE_ENCRYPTION_KEY is required when STORAGE_ENCRYPTION_ENABLED=true")
		}
		encKey, err := parseEncryptionKeyFromEnv(encKeyStr)
		if err != nil {
			return nil, fmt.Errorf("invalid STORAGE_ENCRYPTION_KEY: %w", err)
		}
		cfg.Storage.EncryptionKey = encKey
	}

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// parseEncryptionKeyFromEnv parses and validates an encryption key from environment variable.
func parseEncryptionKeyFromEnv(keyStr string) ([]byte, error) {
	if keyStr == "" {
		return nil, fmt.Errorf("encryption key is empty")
	}

	// Try base64 decode first
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err == nil && len(key) == 32 {
		return key, nil
	}

	// Try raw string if it's exactly 32 bytes
	if len(keyStr) == 32 {
		return []byte(keyStr), nil
	}

	return nil, fmt.Errorf("must be 32 bytes base64-encoded or 32-byte string, got %d bytes after decode", len(keyStr))
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue string) time.Duration {
	value := getEnv(key, defaultValue)
	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}
	// Fallback to default - if this also fails, return 0
	duration, err := time.ParseDuration(defaultValue)
	if err != nil {
		return 0
	}
	return duration
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if int64Value, err := strconv.ParseInt(value, 10, 64); err == nil {
			return int64Value
		}
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

func getEnvFloat64(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// EffectiveDriver returns the resolved database driver for the current config.
func (c DatabaseConfig) EffectiveDriver() string {
	return "postgres"
}

// EffectiveDSN returns the connection string used to open the configured database.
func (c DatabaseConfig) EffectiveDSN() string {
	return strings.TrimSpace(c.DSN)
}

// RedactedDSN returns the configured DSN with credentials removed for logs and UI.
func (c DatabaseConfig) RedactedDSN() string {
	dsn := c.EffectiveDSN()
	if dsn == "" {
		return ""
	}

	switch {
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		parts := strings.SplitN(dsn, "@", 2)
		if len(parts) != 2 {
			return dsn
		}
		schemeParts := strings.SplitN(parts[0], "://", 2)
		if len(schemeParts) != 2 {
			return dsn
		}
		return schemeParts[0] + "://***:***@" + parts[1]
	default:
		return dsn
	}
}

// validate checks that required configuration is present
func (c *Config) validate() error {
	// JWT secret is required for production
	if c.Server.Environment == "production" {
		if c.Auth.JWTSecret == "" || c.Auth.JWTSecret == "change-me-in-production" {
			return fmt.Errorf("JWT_SECRET is required and must be changed in production")
		}
		if c.Limits.MetricsToken == "" {
			return fmt.Errorf("METRICS_TOKEN is required in production")
		}
	}

	// Validate database connection settings.
	if c.Database.EffectiveDSN() == "" {
		return fmt.Errorf("DATABASE_DSN must be set")
	}
	if !strings.HasPrefix(c.Database.EffectiveDSN(), "postgres://") &&
		!strings.HasPrefix(c.Database.EffectiveDSN(), "postgresql://") {
		return fmt.Errorf("DATABASE_DSN must be a postgres connection string")
	}

	// Email backend must be configured for production
	if c.Server.Environment == "production" && c.Email.Backend == "mock" {
		return fmt.Errorf("Email backend must be configured in production (not 'mock'). Use 'none' for workers that don't send email")
	}

	// SendGrid API key required if using SendGrid
	if c.Email.Backend == "sendgrid" && c.Email.SendGridAPIKey == "" {
		return fmt.Errorf("SENDGRID_API_KEY is required when using SendGrid email backend")
	}
	if c.Email.Backend == "postmark" && c.Email.PostmarkServerToken == "" {
		return fmt.Errorf("POSTMARK_SERVER_TOKEN is required when using Postmark email backend")
	}
	if c.Email.Backend == "ses" && c.Email.SESRegion == "" {
		return fmt.Errorf("SES_REGION is required when using SES email backend")
	}
	if c.Email.Backend == "smtp" {
		if c.Email.SMTPHost == "" {
			return fmt.Errorf("SMTP_HOST is required when using SMTP email backend")
		}
		if c.Email.SMTPPort <= 0 {
			return fmt.Errorf("SMTP_PORT must be greater than zero when using SMTP email backend")
		}
	}

	// Storage validation
	if c.Storage.Type != "s3" && c.Storage.Type != "filesystem" {
		return fmt.Errorf("Invalid STORAGE_TYPE: %s (must be 's3' or 'filesystem')", c.Storage.Type)
	}

	// S3 credentials required if using S3
	if c.Storage.Type == "s3" {
		if c.Storage.Operational.Bucket == "" {
			return fmt.Errorf("S3_BUCKET is required when using S3 storage")
		}
		if c.Storage.Operational.Region == "" {
			return fmt.Errorf("S3_REGION is required when using S3 storage")
		}
	}

	for publisher, keyMap := range c.ExtensionTrust.TrustedPublishers {
		if strings.TrimSpace(publisher) == "" {
			return fmt.Errorf("EXTENSION_TRUSTED_PUBLISHERS_JSON contains an empty publisher")
		}
		for keyID, encodedKey := range keyMap {
			if strings.TrimSpace(keyID) == "" {
				return fmt.Errorf("EXTENSION_TRUSTED_PUBLISHERS_JSON contains an empty key id for %s", publisher)
			}
			publicKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedKey))
			if err != nil {
				return fmt.Errorf("EXTENSION_TRUSTED_PUBLISHERS_JSON contains an invalid base64 key for %s/%s: %w", publisher, keyID, err)
			}
			if len(publicKey) != ed25519.PublicKeySize {
				return fmt.Errorf("EXTENSION_TRUSTED_PUBLISHERS_JSON key for %s/%s must be %d bytes", publisher, keyID, ed25519.PublicKeySize)
			}
		}
	}
	if c.ExtensionTrust.RequireVerification && strings.TrimSpace(c.InstanceID) == "" {
		return fmt.Errorf("INSTANCE_ID is required when EXTENSION_TRUST_REQUIRE_VERIFICATION=true")
	}
	for _, host := range c.EnterpriseAccess.AllowedHosts {
		if strings.TrimSpace(host) == "" {
			return fmt.Errorf("ENTERPRISE_ACCESS_ALLOWED_HOSTS contains an empty host")
		}
	}

	return nil
}
