package platformdomain

import (
	"time"

	shared "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

// AuditConfiguration represents audit configuration for a workspace
type AuditConfiguration struct {
	ID          string
	WorkspaceID string

	// Configuration settings
	EnabledCategories []string
	LogLevel          string
	RetentionDays     int

	// Features
	LogAPIRequests  bool
	LogUserActions  bool
	LogSystemEvents bool
	LogDataChanges  bool

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AlertRule represents an alert rule for audit events
type AlertRule struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string

	// Rule configuration
	EventType  string
	Severity   string
	Conditions shared.Metadata

	// Alert settings
	Enabled    bool
	Channels   []string // email, slack, webhook
	Recipients []string

	// Throttling
	ThrottleMinutes int
	LastTriggered   *time.Time

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
}
