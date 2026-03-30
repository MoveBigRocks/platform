package platformservices

import (
	"time"

	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

// Parameter objects for AuditService methods
// These replace long parameter lists with clean, self-documenting structs

// LogActivityRequest contains parameters for logging an activity
type LogActivityRequest struct {
	WorkspaceID  string
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	Details      shareddomain.Metadata
	Outcome      string // "success" or "failure"
	UserAgent    string
	IPAddress    string
}

// LogSecurityEventRequest contains parameters for logging a security event
type LogSecurityEventRequest struct {
	WorkspaceID string
	ActorID     string
	EventType   string
	Severity    string // "critical", "high", "medium", "low", "info"
	Description string
	Details     shareddomain.Metadata
	UserAgent   string
	IPAddress   string
}

// AuditLogQuery contains filter parameters for querying audit logs
type AuditLogQuery struct {
	WorkspaceID  string
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	Outcome      string
	Category     string
	StartTime    *time.Time
	EndTime      *time.Time
	Limit        int
	Offset       int
}

// SecurityEventQuery contains filter parameters for querying security events
type SecurityEventQuery struct {
	WorkspaceID string
	EventType   string
	Severity    string
	ActorID     string
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int
	Offset      int
}

// SearchAuditLogsRequest contains parameters for searching audit logs
type SearchAuditLogsRequest struct {
	WorkspaceID string
	Query       string
	Filters     shareddomain.Metadata
	SortBy      string
	SortOrder   string
	Limit       int
	Offset      int
}

// AlertRuleCondition contains typed parameters for alert rule conditions
type AlertRuleCondition struct {
	Field    string             // Field to evaluate (e.g., "severity", "event_type")
	Operator string             // Comparison operator (e.g., "equals", "contains", "greater_than")
	Value    shareddomain.Value // Value to compare against
	LogicOp  string             // Logical operator for combining with other conditions ("and", "or")
}

// AlertRuleAction contains typed parameters for alert rule actions
type AlertRuleAction struct {
	Type       string                // Action type (e.g., "email", "webhook", "slack")
	Recipients []string              // Recipients for notifications
	Template   string                // Template ID or name
	Config     shareddomain.Metadata // Additional action configuration
}

// AlertRuleRequest contains parameters for creating/updating alert rules
type AlertRuleRequest struct {
	WorkspaceID string
	RuleID      string // Empty for create, set for update
	Name        string
	Description string
	Conditions  []AlertRuleCondition
	Actions     []AlertRuleAction
}
