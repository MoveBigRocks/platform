package platformdomain

import (
	"encoding/json"
	"time"

	shared "github.com/movebigrocks/platform/internal/shared/domain"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID          string
	WorkspaceID string
	UserID      string
	UserEmail   string
	UserName    string

	// Action details
	Action       AuditAction
	Resource     string
	ResourceID   string
	ResourceName string

	// Changes
	OldValue json.RawMessage
	NewValue json.RawMessage
	Changes  []FieldChange

	// Context
	IPAddress string
	UserAgent string
	SessionID string
	RequestID string
	APIKeyID  string

	// Result
	Success      bool
	ErrorMessage string

	// Metadata
	Metadata shared.Metadata
	Tags     []string

	// Timestamp
	CreatedAt time.Time
}

// AuditAction represents the type of action being audited
type AuditAction string

const (
	// Authentication actions
	AuditActionLogin         AuditAction = "login"
	AuditActionLogout        AuditAction = "logout"
	AuditActionLoginFailed   AuditAction = "login_failed"
	AuditActionPasswordReset AuditAction = "password_reset"
	AuditActionMFAEnabled    AuditAction = "mfa_enabled"
	AuditActionMFADisabled   AuditAction = "mfa_disabled"
	AuditActionAPIKeyCreated AuditAction = "api_key_created"
	AuditActionAPIKeyRevoked AuditAction = "api_key_revoked"

	// CRUD actions
	AuditActionCreate  AuditAction = "create"
	AuditActionRead    AuditAction = "read"
	AuditActionUpdate  AuditAction = "update"
	AuditActionDelete  AuditAction = "delete"
	AuditActionRestore AuditAction = "restore"
	AuditActionArchive AuditAction = "archive"

	// Case actions
	AuditActionCaseAssigned  AuditAction = "case_assigned"
	AuditActionCaseResolved  AuditAction = "case_resolved"
	AuditActionCaseMerged    AuditAction = "case_merged"
	AuditActionCaseEscalated AuditAction = "case_escalated"

	// Permission actions
	AuditActionPermissionGranted AuditAction = "permission_granted"
	AuditActionPermissionRevoked AuditAction = "permission_revoked"
	AuditActionRoleAssigned      AuditAction = "role_assigned"
	AuditActionRoleRemoved       AuditAction = "role_removed"

	// Data actions
	AuditActionExport     AuditAction = "export"
	AuditActionImport     AuditAction = "import"
	AuditActionBulkUpdate AuditAction = "bulk_update"
	AuditActionBulkDelete AuditAction = "bulk_delete"

	// System actions
	AuditActionSettingsChanged     AuditAction = "settings_changed"
	AuditActionIntegrationEnabled  AuditAction = "integration_enabled"
	AuditActionIntegrationDisabled AuditAction = "integration_disabled"
	AuditActionWebhookCreated      AuditAction = "webhook_created"
	AuditActionWebhookDeleted      AuditAction = "webhook_deleted"
)

// FieldChange represents a single field change
type FieldChange struct {
	Field      string
	OldValue   shared.Value
	NewValue   shared.Value
	ChangeType string // added, removed, modified
}

// AuditLogFilter represents filters for querying audit logs
type AuditLogFilter struct {
	WorkspaceID string
	UserID      string
	Action      string
	Resource    string
	ResourceID  string
	StartDate   *time.Time
	EndDate     *time.Time
	Success     *bool
	IPAddress   string
	Tags        []string
	Limit       int
	Offset      int
}

// AuditLogRetention represents retention policy for audit logs
type AuditLogRetention struct {
	ID          string
	WorkspaceID string

	// Retention periods (in days)
	DefaultRetention   int
	AuthenticationLogs int
	DataAccessLogs     int
	ConfigurationLogs  int
	SecurityLogs       int

	// Archive settings
	ArchiveEnabled   bool
	ArchiveLocation  string // S3, GCS, etc.
	ArchiveAfterDays int

	// Compliance
	ComplianceMode     bool   // Prevents deletion
	ComplianceStandard string // GDPR, HIPAA, etc.

	UpdatedAt time.Time
	UpdatedBy string
}

// AuditLogExport represents an audit log export request
type AuditLogExport struct {
	ID          string
	WorkspaceID string
	RequestedBy string

	// Export parameters
	Filter AuditLogFilter
	Format ExportFormat

	// Status
	Status      ExportStatus
	RecordCount int
	FileSize    int64

	// Output
	DownloadURL string
	ExpiresAt   *time.Time

	// Processing
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       string

	CreatedAt time.Time
}

// ExportFormat represents the format for exports
type ExportFormat string

const (
	ExportFormatJSON ExportFormat = "json"
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatPDF  ExportFormat = "pdf"
	ExportFormatXLSX ExportFormat = "xlsx"
)

// ExportStatus represents the status of an export
type ExportStatus string

const (
	ExportStatusPending    ExportStatus = "pending"
	ExportStatusProcessing ExportStatus = "processing"
	ExportStatusCompleted  ExportStatus = "completed"
	ExportStatusFailed     ExportStatus = "failed"
	ExportStatusExpired    ExportStatus = "expired"
)

// AuditLogSummary represents aggregated audit log data
type AuditLogSummary struct {
	WorkspaceID string
	Period      string // daily, weekly, monthly
	Date        time.Time

	// Action counts
	ActionCounts map[AuditAction]int

	// Resource counts
	ResourceCounts map[string]int

	// User activity
	ActiveUsers int
	TopUsers    []UserActivity

	// Security metrics
	FailedLogins         int
	PermissionDenials    int
	SuspiciousActivities int

	// Data metrics
	RecordsCreated int
	RecordsUpdated int
	RecordsDeleted int
	DataExported   int

	ComputedAt time.Time
}

// UserActivity represents user activity metrics
type UserActivity struct {
	UserID      string
	UserEmail   string
	ActionCount int
	LastActive  time.Time
}

// ComplianceReport represents a compliance audit report
type ComplianceReport struct {
	ID          string
	WorkspaceID string

	// Report details
	Type     ComplianceType
	Period   shared.DateRange
	Standard string // GDPR, HIPAA, SOC2, etc.

	// Findings
	TotalEvents     int
	CompliantEvents int
	Violations      []ComplianceViolation
	Warnings        []ComplianceWarning

	// Recommendations
	Recommendations []string

	// Status
	Status     ReportStatus
	ReviewedBy string
	ReviewedAt *time.Time
	ApprovedBy string
	ApprovedAt *time.Time

	// Files
	ReportURL string

	GeneratedAt time.Time
	GeneratedBy string
}

// ComplianceType represents the type of compliance check
type ComplianceType string

const (
	ComplianceTypeDataAccess      ComplianceType = "data_access"
	ComplianceTypeDataRetention   ComplianceType = "data_retention"
	ComplianceTypeUserConsent     ComplianceType = "user_consent"
	ComplianceTypeDataPortability ComplianceType = "data_portability"
	ComplianceTypeRightToErasure  ComplianceType = "right_to_erasure"
	ComplianceTypeSecurityAudit   ComplianceType = "security_audit"
)

// ReportStatus represents the status of a report
type ReportStatus string

const (
	ReportStatusDraft     ReportStatus = "draft"
	ReportStatusGenerated ReportStatus = "generated"
	ReportStatusReviewed  ReportStatus = "reviewed"
	ReportStatusApproved  ReportStatus = "approved"
	ReportStatusArchived  ReportStatus = "archived"
)

// ComplianceViolation represents a compliance violation
type ComplianceViolation struct {
	Type        string
	Severity    string // critical, high, medium, low
	Description string
	Resource    string
	ResourceID  string
	UserID      string
	OccurredAt  time.Time
	Details     shared.Metadata
}

// ComplianceWarning represents a compliance warning
type ComplianceWarning struct {
	Type           string
	Message        string
	Resource       string
	Count          int
	Recommendation string
}

// SecurityEvent represents a security-related event
type SecurityEvent struct {
	ID          string
	WorkspaceID string

	// Event details
	Type        SecurityEventType
	Severity    SecuritySeverity
	Description string

	// Actor
	UserID    string
	IPAddress string
	UserAgent string
	Location  string

	// Target
	Resource   string
	ResourceID string

	// Detection
	DetectionMethod string
	RiskScore       int
	Indicators      []string

	// Response
	AutoBlocked    bool
	RequiresReview bool
	ReviewedBy     string
	ReviewedAt     *time.Time
	ActionTaken    string

	// Metadata
	Metadata shared.Metadata

	OccurredAt time.Time
	CreatedAt  time.Time
}

// SecurityEventType represents the type of security event
type SecurityEventType string

const (
	SecurityEventTypeBruteForce          SecurityEventType = "brute_force"
	SecurityEventTypeUnauthorizedAccess  SecurityEventType = "unauthorized_access"
	SecurityEventTypeSuspiciousActivity  SecurityEventType = "suspicious_activity"
	SecurityEventTypeDataExfiltration    SecurityEventType = "data_exfiltration"
	SecurityEventTypeMaliciousPayload    SecurityEventType = "malicious_payload"
	SecurityEventTypeAccountTakeover     SecurityEventType = "account_takeover"
	SecurityEventTypePrivilegeEscalation SecurityEventType = "privilege_escalation"
)

// SecuritySeverity represents the severity of a security event
type SecuritySeverity string

const (
	SecuritySeverityInfo     SecuritySeverity = "info"
	SecuritySeverityLow      SecuritySeverity = "low"
	SecuritySeverityMedium   SecuritySeverity = "medium"
	SecuritySeverityHigh     SecuritySeverity = "high"
	SecuritySeverityCritical SecuritySeverity = "critical"
)

// ActivityLog represents user activity tracking
type ActivityLog struct {
	ID          string
	WorkspaceID string
	UserID      string

	// Activity details
	Type     ActivityType
	Entity   string
	EntityID string
	Action   string

	// Context
	Context  shared.Metadata
	Duration int64 // milliseconds

	// Session
	SessionID string
	IPAddress string

	Timestamp time.Time
}

// ActivityType represents the type of activity
type ActivityType string

const (
	ActivityTypePageView     ActivityType = "page_view"
	ActivityTypeSearch       ActivityType = "search"
	ActivityTypeClick        ActivityType = "click"
	ActivityTypeFormSubmit   ActivityType = "form_submit"
	ActivityTypeFileDownload ActivityType = "file_download"
	ActivityTypeFileUpload   ActivityType = "file_upload"
	ActivityTypeAPICall      ActivityType = "api_call"
)
