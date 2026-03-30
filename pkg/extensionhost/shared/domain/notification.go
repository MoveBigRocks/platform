package shareddomain

import (
	"encoding/json"
	"time"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationTypeEmail   NotificationType = "email"
	NotificationTypeInApp   NotificationType = "in_app"
	NotificationTypePush    NotificationType = "push"
	NotificationTypeSMS     NotificationType = "sms"
	NotificationTypeWebhook NotificationType = "webhook"
	NotificationTypeSlack   NotificationType = "slack"
	NotificationTypeTeams   NotificationType = "teams"
	NotificationTypeDiscord NotificationType = "discord"
)

// NotificationCategory represents the category of notification
type NotificationCategory string

const (
	NotificationCategoryCaseUpdate     NotificationCategory = "case_update"
	NotificationCategoryCaseAssignment NotificationCategory = "case_assignment"
	NotificationCategoryCaseMention    NotificationCategory = "case_mention"
	NotificationCategorySLAWarning     NotificationCategory = "sla_warning"
	NotificationCategorySLABreach      NotificationCategory = "sla_breach"
	NotificationCategoryEscalation     NotificationCategory = "escalation"
	NotificationCategorySystemAlert    NotificationCategory = "system_alert"
	NotificationCategoryReport         NotificationCategory = "report"
	NotificationCategoryDigest         NotificationCategory = "digest"
	NotificationCategoryWorkflow       NotificationCategory = "workflow"
	NotificationCategoryCollaboration  NotificationCategory = "collaboration"
)

// NotificationPriority represents the priority of a notification
type NotificationPriority string

const (
	NotificationPriorityLow      NotificationPriority = "low"
	NotificationPriorityMedium   NotificationPriority = "medium"
	NotificationPriorityHigh     NotificationPriority = "high"
	NotificationPriorityUrgent   NotificationPriority = "urgent"
	NotificationPriorityCritical NotificationPriority = "critical"
)

// NotificationStatus represents the status of a notification
type NotificationStatus string

const (
	NotificationStatusPending    NotificationStatus = "pending"
	NotificationStatusQueued     NotificationStatus = "queued"
	NotificationStatusSending    NotificationStatus = "sending"
	NotificationStatusSent       NotificationStatus = "sent"
	NotificationStatusDelivered  NotificationStatus = "delivered"
	NotificationStatusRead       NotificationStatus = "read"
	NotificationStatusFailed     NotificationStatus = "failed"
	NotificationStatusBounced    NotificationStatus = "bounced"
	NotificationStatusSuppressed NotificationStatus = "suppressed"
)

// Notification represents a notification in the system
type Notification struct {
	ID             string
	WorkspaceID    string
	RecipientID    string
	RecipientEmail string
	RecipientPhone string
	Type           NotificationType
	Category       NotificationCategory
	Priority       NotificationPriority
	Status         NotificationStatus

	// Content
	Subject      string
	Body         string
	HTMLBody     string
	TemplateID   string
	TemplateData json.RawMessage

	// Context
	EntityType string // case, workflow, report, etc.
	EntityID   string
	ActionURL  string
	ImageURL   string

	// Delivery
	ChannelConfig json.RawMessage
	ScheduledFor  *time.Time
	SentAt        *time.Time
	DeliveredAt   *time.Time
	ReadAt        *time.Time
	FailedAt      *time.Time
	RetryCount    int
	MaxRetries    int
	LastError     string

	// Tracking
	MessageID      string // External provider message ID
	ConversationID string
	BatchID        string
	TrackingData   json.RawMessage

	// Settings
	RequireDeliveryConfirmation bool
	AllowDuplicate              bool
	ExpiresAt                   *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NotificationTemplate represents a reusable notification template
type NotificationTemplate struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	Type        NotificationType
	Category    NotificationCategory

	// Content templates
	SubjectTemplate string
	BodyTemplate    string
	HTMLTemplate    string

	// Variables
	Variables     []TemplateVariable
	DefaultValues json.RawMessage

	// Settings
	IsActive bool
	IsSystem bool // System templates cannot be deleted
	Priority NotificationPriority

	// Channel-specific settings
	ChannelSettings json.RawMessage

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
}

// TemplateVariable represents a variable in a template
type TemplateVariable struct {
	Name         string
	Type         string // string, number, date, boolean, object
	Required     bool
	DefaultValue string
	Description  string
	Format       string // For dates, numbers
}

// NotificationPreferences represents user notification preferences
type NotificationPreferences struct {
	ID          string
	UserID      string
	WorkspaceID string

	// Channel preferences
	EmailEnabled bool
	InAppEnabled bool
	PushEnabled  bool
	SMSEnabled   bool

	// Category preferences
	CategorySettings map[NotificationCategory]CategoryPreference

	// Quiet hours
	QuietHoursEnabled  bool
	QuietHoursStart    string // HH:MM format
	QuietHoursEnd      string
	QuietHoursTimezone string
	QuietHoursDays     []string // ["monday", "tuesday", ...]

	// Digest preferences
	DigestEnabled   bool
	DigestFrequency string // daily, weekly, monthly
	DigestTime      string // HH:MM format
	DigestDay       string // For weekly digest

	// Filtering
	MinimumPriority NotificationPriority
	BlockedSenders  []string
	AllowedSenders  []string

	// Delivery preferences
	PreferredLanguage string
	PreferredChannels []string // Ordered by preference

	CreatedAt time.Time
	UpdatedAt time.Time
}

// CategoryPreference represents preferences for a notification category
type CategoryPreference struct {
	Enabled         bool
	Channels        []NotificationType
	MinimumPriority NotificationPriority
	InstantDelivery bool // Override digest settings
	CustomSettings  json.RawMessage
}

// NotificationBatch represents a batch of notifications
type NotificationBatch struct {
	ID          string
	WorkspaceID string
	Name        string
	Type        NotificationType
	Status      NotificationStatus

	// Recipients
	RecipientCount  int
	RecipientListID string
	RecipientFilter string

	// Content
	TemplateID   string
	TemplateData json.RawMessage

	// Delivery
	ScheduledFor *time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time

	// Statistics
	SentCount      int
	DeliveredCount int
	FailedCount    int
	ReadCount      int

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
}

// NotificationLog represents a log entry for notification delivery
type NotificationLog struct {
	ID             string
	NotificationID string
	BatchID        string
	Status         NotificationStatus
	Channel        NotificationType

	// Delivery details
	Provider       string // sendgrid, twilio, etc.
	MessageID      string
	DeliveryStatus string
	DeliveryTime   *time.Time

	// Response data
	ResponseCode    int
	ResponseBody    string
	ResponseHeaders map[string]string

	// Error handling
	ErrorMessage string
	ErrorCode    string
	RetryAttempt int
	NextRetryAt  *time.Time

	// Tracking
	OpenedAt       *time.Time
	ClickedAt      *time.Time
	UnsubscribedAt *time.Time
	BouncedAt      *time.Time
	ComplainedAt   *time.Time

	// Metadata
	IPAddress  string
	UserAgent  string
	Location   string
	DeviceType string
	Metadata   json.RawMessage

	CreatedAt time.Time
}

// DigestNotification represents a digest notification configuration
type DigestNotification struct {
	ID          string
	UserID      string
	WorkspaceID string

	// Configuration
	Frequency string // daily, weekly, monthly
	NextRunAt time.Time
	LastRunAt *time.Time

	// Content settings
	IncludeCategories []NotificationCategory
	ExcludeCategories []NotificationCategory
	MinimumPriority   NotificationPriority
	MaxItems          int

	// Delivery
	DeliveryChannel NotificationType
	TemplateID      string

	// Statistics
	LastItemCount  int
	TotalSentCount int

	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NotificationFilter represents filters for querying notifications
type NotificationFilter struct {
	UserID      string
	WorkspaceID string
	Type        string
	Status      NotificationStatus
	Priority    NotificationPriority
	StartDate   *time.Time
	EndDate     *time.Time
	Limit       int
	Offset      int
}

// NotificationStats represents notification statistics
type NotificationStats struct {
	Total      int
	Unread     int
	Read       int
	Sent       int
	Failed     int
	ByType     map[string]int
	ByChannel  map[string]int
	ByPriority map[string]int
}
