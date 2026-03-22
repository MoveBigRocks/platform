package models

import (
	"time"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`
	UserID      string `db:"user_id"`
	UserEmail   string `db:"user_email"`
	UserName    string `db:"user_name"`

	// Action details
	Action       string `db:"action"`
	Resource     string `db:"resource"`
	ResourceID   string `db:"resource_id"`
	ResourceName string `db:"resource_name"`

	// Changes
	OldValue string `db:"old_value"`
	NewValue string `db:"new_value"`
	Changes  string `db:"changes"`

	// Context
	IPAddress string `db:"ip_address"`
	UserAgent string `db:"user_agent"`
	SessionID string `db:"session_id"`
	RequestID string `db:"request_id"`
	APIKeyID  string `db:"api_key_id"`

	// Result
	Success      bool   `db:"success"`
	ErrorMessage string `db:"error_message"`

	// Metadata
	Metadata string `db:"metadata"`
	Tags     string `db:"tags"`

	// Timestamp
	CreatedAt time.Time `db:"created_at"`
}

// AuditConfiguration represents audit configuration for a workspace
type AuditConfiguration struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Configuration settings
	EnabledCategories string `db:"enabled_categories"`
	LogLevel          string `db:"log_level"`
	RetentionDays     int    `db:"retention_days"`

	// Features
	LogAPIRequests  bool `db:"log_api_requests"`
	LogUserActions  bool `db:"log_user_actions"`
	LogSystemEvents bool `db:"log_system_events"`
	LogDataChanges  bool `db:"log_data_changes"`

	// Metadata
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// AlertRule represents an alert rule for audit events
type AlertRule struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`
	Name        string `db:"name"`
	Description string `db:"description"`

	// Rule configuration
	EventType  string `db:"event_type"`
	Severity   string `db:"severity"`
	Conditions string `db:"conditions"`

	// Alert settings
	Enabled    bool   `db:"enabled"`
	Channels   string `db:"channels"`
	Recipients string `db:"recipients"`

	// Throttling
	ThrottleMinutes int        `db:"throttle_minutes"`
	LastTriggered   *time.Time `db:"last_triggered"`

	// Metadata
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	CreatedBy string    `db:"created_by"`
}

// SecurityEvent represents a security-related event
type SecurityEvent struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Event details
	Type        string `db:"type"`
	Severity    string `db:"severity"`
	Description string `db:"description"`

	// Actor
	UserID    string `db:"user_id"`
	IPAddress string `db:"ip_address"`
	UserAgent string `db:"user_agent"`
	Location  string `db:"location"`

	// Target
	Resource   string `db:"resource"`
	ResourceID string `db:"resource_id"`

	// Detection
	DetectionMethod string `db:"detection_method"`
	RiskScore       int    `db:"risk_score"`
	Indicators      string `db:"indicators"`

	// Response
	AutoBlocked    bool       `db:"auto_blocked"`
	RequiresReview bool       `db:"requires_review"`
	ReviewedBy     string     `db:"reviewed_by"`
	ReviewedAt     *time.Time `db:"reviewed_at"`
	ActionTaken    string     `db:"action_taken"`

	// Metadata
	Metadata string `db:"metadata"`

	OccurredAt time.Time `db:"occurred_at"`
	CreatedAt  time.Time `db:"created_at"`
}

// AuditLogRetention represents retention policy for audit logs
type AuditLogRetention struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Retention periods (in days)
	DefaultRetention   int `db:"default_retention"`
	AuthenticationLogs int `db:"authentication_logs"`
	DataAccessLogs     int `db:"data_access_logs"`
	ConfigurationLogs  int `db:"configuration_logs"`
	SecurityLogs       int `db:"security_logs"`

	// Archive settings
	ArchiveEnabled   bool   `db:"archive_enabled"`
	ArchiveLocation  string `db:"archive_location"`
	ArchiveAfterDays int    `db:"archive_after_days"`

	// Compliance
	ComplianceMode     bool   `db:"compliance_mode"`
	ComplianceStandard string `db:"compliance_standard"`

	UpdatedAt time.Time `db:"updated_at"`
	UpdatedBy string    `db:"updated_by"`
}
