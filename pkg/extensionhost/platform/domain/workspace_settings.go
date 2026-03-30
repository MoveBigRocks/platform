package platformdomain

import (
	"time"

	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	shared "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

// SettingType represents the type of a setting
type SettingType string

const (
	SettingTypeString   SettingType = "string"
	SettingTypeNumber   SettingType = "number"
	SettingTypeBoolean  SettingType = "boolean"
	SettingTypeJSON     SettingType = "json"
	SettingTypeArray    SettingType = "array"
	SettingTypeObject   SettingType = "object"
	SettingTypeEnum     SettingType = "enum"
	SettingTypeColor    SettingType = "color"
	SettingTypeFile     SettingType = "file"
	SettingTypePassword SettingType = "password"
)

// SettingCategory represents categories of settings
type SettingCategory string

const (
	SettingCategoryGeneral       SettingCategory = "general"
	SettingCategoryBranding      SettingCategory = "branding"
	SettingCategorySecurity      SettingCategory = "security"
	SettingCategoryNotifications SettingCategory = "notifications"
	SettingCategoryEmail         SettingCategory = "email"
	SettingCategoryIntegrations  SettingCategory = "integrations"
	SettingCategoryWorkflow      SettingCategory = "workflow"
	SettingCategoryPortal        SettingCategory = "portal"
	SettingCategoryAnalytics     SettingCategory = "analytics"
	SettingCategorySupport       SettingCategory = "support"
	SettingCategoryBilling       SettingCategory = "billing"
	SettingCategoryAdvanced      SettingCategory = "advanced"
)

// SettingScope represents who can modify a setting
type SettingScope string

const (
	SettingScopeWorkspaceOwner SettingScope = "workspace_owner" // Only workspace owner
	SettingScopeAdmin          SettingScope = "admin"           // Workspace admins
	SettingScopeManager        SettingScope = "manager"         // Managers and above
	SettingScopeUser           SettingScope = "user"            // Any user
	SettingScopeSystem         SettingScope = "system"          // System only (read-only for users)
)

// WorkspaceSettings represents per-workspace configuration settings
type WorkspaceSettings struct {
	ID          string
	WorkspaceID string

	// General settings
	WorkspaceName        string
	WorkspaceDescription string
	Timezone             string
	Language             string
	DateFormat           string
	TimeFormat           string

	// Business information
	CompanyName    string
	CompanyAddress string
	CompanyPhone   string
	CompanyEmail   string
	CompanyWebsite string

	// Branding
	LogoURL        string
	FaviconURL     string
	BrandColor     string
	SecondaryColor string
	Theme          string // "light", "dark", "auto"
	CustomCSS      string

	// Business hours
	BusinessHours map[string]BusinessHours
	Holidays      []Holiday

	// Case management
	CaseNumberPrefix    string
	CaseNumberFormat    string
	DefaultCaseStatus   servicedomain.CaseStatus
	DefaultCasePriority servicedomain.CasePriority
	AutoAssignCases     bool
	RequireCaseCategory bool

	// SLA settings
	DefaultSLAHours      int
	SLAByPriority        map[string]int
	SLABusinessHoursOnly bool

	// Email settings
	EmailFromName        string
	EmailFromAddress     string
	EmailReplyToAddress  string
	EmailSignature       string
	EmailFooter          string
	AutoResponseEnabled  bool
	AutoResponseTemplate string

	// Notification settings
	NotifyOnNewCase      bool
	NotifyOnCaseUpdate   bool
	NotifyOnAssignment   bool
	NotifyOnEscalation   bool
	NotificationChannels []string // "email", "sms", "slack", "teams"

	// Security settings
	PasswordMinLength        int
	PasswordRequireSpecial   bool
	PasswordRequireNumbers   bool
	PasswordRequireUppercase bool
	SessionTimeoutMinutes    int
	TwoFactorRequired        bool
	IPWhitelist              []string
	IPBlacklist              []string

	// File upload settings
	MaxFileSize      int64 // bytes
	AllowedFileTypes []string
	BlockedFileTypes []string
	VirusScanEnabled bool

	// Integration settings
	SlackWebhookURL string
	TeamsWebhookURL string
	ZapierEnabled   bool
	WebhooksEnabled bool
	APIRateLimit    int // requests per minute

	// Analytics settings
	AnalyticsEnabled  bool
	GoogleAnalyticsID string
	DataRetentionDays int
	ExportDataEnabled bool

	// Portal settings
	PortalEnabled        bool
	PortalDomain         string
	PortalTitle          string
	PortalWelcomeMessage string

	// Workflow settings
	WorkflowsEnabled  bool
	RulesEnabled      bool
	MacrosEnabled     bool
	AutomationEnabled bool

	// Backup and maintenance
	AutoBackupEnabled   bool
	BackupFrequencyDays int
	BackupRetentionDays int
	MaintenanceWindow   MaintenanceWindow

	// Compliance settings
	GDPRCompliant         bool
	CCPACompliant         bool
	DataProcessingConsent bool
	CookieNoticeEnabled   bool

	// Feature flags
	FeatureFlags map[string]bool
	BetaFeatures []string

	// Custom fields
	CustomFields shared.TypedCustomFields

	// Metadata
	CreatedByID string
	UpdatedByID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewWorkspaceSettings creates the default settings record for a workspace.
func NewWorkspaceSettings(workspaceID string) *WorkspaceSettings {
	now := time.Now()
	return &WorkspaceSettings{
		WorkspaceID: workspaceID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// BusinessHours represents business hours for a day
type BusinessHours struct {
	IsBusinessDay bool
	OpenTime      string      // "09:00"
	CloseTime     string      // "17:00"
	BreakTimes    []BreakTime // Lunch breaks, etc.
}

// BreakTime represents a break during business hours
type BreakTime struct {
	StartTime string // "12:00"
	EndTime   string // "13:00"
	Name      string // "Lunch Break"
}

// Holiday represents a holiday
type Holiday struct {
	Date        time.Time
	Name        string
	IsRecurring bool // Recurring annually
	CountryCode string
}

// MaintenanceWindow represents a scheduled maintenance window
type MaintenanceWindow struct {
	DayOfWeek string // "Sunday"
	StartTime string // "02:00"
	EndTime   string // "04:00"
	Timezone  string
}

// SettingDefinition represents the definition of a workspace setting
type SettingDefinition struct {
	ID           string
	Key          string // Setting key (e.g., "workspace_name")
	Name         string // Display name
	Description  string
	Category     SettingCategory
	Type         SettingType
	DefaultValue shared.Value
	Required     bool
	Scope        SettingScope // Who can modify this setting

	// Validation
	MinValue   *float64 // For numbers
	MaxValue   *float64 // For numbers
	MinLength  *int     // For strings
	MaxLength  *int     // For strings
	Pattern    string   // Regex pattern for strings
	EnumValues []string // Valid values for enum

	// UI settings
	Placeholder  string // Placeholder text
	HelpText     string // Help text
	DisplayOrder int    // Display order in UI
	IsVisible    bool   // Whether to show in UI
	IsAdvanced   bool   // Whether this is an advanced setting

	// Dependencies
	DependsOn      string // Show only if other setting has specific value
	DependsOnValue shared.Value

	// Feature flags
	RequiredFeature string // Required feature flag

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SettingValue represents a workspace setting value
type SettingValue struct {
	ID          string
	WorkspaceID string
	SettingKey  string
	Value       shared.Value

	// Change tracking
	PreviousValue shared.Value
	UpdatedByID   string
	UpdatedAt     time.Time

	// Validation
	IsValid         bool
	ValidationError string

	// Metadata
	CreatedAt time.Time
}

// SettingChangeHistory represents the change history of workspace settings
type SettingChangeHistory struct {
	ID          string
	WorkspaceID string

	// Change details
	SettingKey string
	OldValue   shared.Value
	NewValue   shared.Value
	ChangeType string // "create", "update", "delete", "reset"

	// User context
	ChangedByID   string
	ChangedByName string
	UserAgent     string
	IPAddress     string

	// Reason
	Reason string

	// System context
	SystemTriggered bool   // Changed by system vs user
	MigrationID     string // If changed by migration

	// Metadata
	CreatedAt time.Time
}

// UpdateSetting updates a specific setting value
func (ws *WorkspaceSettings) UpdateSetting(key string, value shared.Value, updatedByID string) {
	// Update the specific field based on key using type-safe accessors
	switch key {
	case "workspace_name":
		if value.IsString() {
			ws.WorkspaceName = value.AsString()
		}
	case "timezone":
		if value.IsString() {
			ws.Timezone = value.AsString()
		}
	case "language":
		if value.IsString() {
			ws.Language = value.AsString()
		}
	case "theme":
		if value.IsString() {
			ws.Theme = value.AsString()
		}
	case "auto_assign_cases":
		if value.IsBool() {
			ws.AutoAssignCases = value.AsBool()
		}
	case "default_sla_hours":
		if value.IsInt() {
			ws.DefaultSLAHours = int(value.AsInt())
		}
	case "portal_enabled":
		if value.IsBool() {
			ws.PortalEnabled = value.AsBool()
		}
		// Add more cases as needed
	}

	ws.UpdatedByID = updatedByID
	ws.UpdatedAt = time.Now()
}

// SetBusinessHours sets business hours for a specific day
func (ws *WorkspaceSettings) SetBusinessHours(day string, hours BusinessHours) {
	if ws.BusinessHours == nil {
		ws.BusinessHours = make(map[string]BusinessHours)
	}
	ws.BusinessHours[day] = hours
	ws.UpdatedAt = time.Now()
}

// AddHoliday adds a holiday to the workspace
func (ws *WorkspaceSettings) AddHoliday(holiday Holiday) {
	ws.Holidays = append(ws.Holidays, holiday)
	ws.UpdatedAt = time.Now()
}

// SetFeatureFlag sets a feature flag value
func (ws *WorkspaceSettings) SetFeatureFlag(flag string, enabled bool) {
	if ws.FeatureFlags == nil {
		ws.FeatureFlags = make(map[string]bool)
	}
	ws.FeatureFlags[flag] = enabled
	ws.UpdatedAt = time.Now()
}

// IsFeatureEnabled checks if a feature flag is enabled
func (ws *WorkspaceSettings) IsFeatureEnabled(flag string) bool {
	if ws.FeatureFlags == nil {
		return false
	}
	enabled, exists := ws.FeatureFlags[flag]
	return exists && enabled
}

// IsBetaFeatureEnabled checks if a beta feature is enabled
func (ws *WorkspaceSettings) IsBetaFeatureEnabled(feature string) bool {
	for _, betaFeature := range ws.BetaFeatures {
		if betaFeature == feature {
			return true
		}
	}
	return false
}

// AddBetaFeature adds a beta feature to the workspace
func (ws *WorkspaceSettings) AddBetaFeature(feature string) {
	for _, existing := range ws.BetaFeatures {
		if existing == feature {
			return // Already exists
		}
	}
	ws.BetaFeatures = append(ws.BetaFeatures, feature)
	ws.UpdatedAt = time.Now()
}

// RemoveBetaFeature removes a beta feature from the workspace
func (ws *WorkspaceSettings) RemoveBetaFeature(feature string) {
	for i, existing := range ws.BetaFeatures {
		if existing == feature {
			ws.BetaFeatures = append(ws.BetaFeatures[:i], ws.BetaFeatures[i+1:]...)
			break
		}
	}
	ws.UpdatedAt = time.Now()
}

// IsBusinessDay checks if a specific day is a business day
func (ws *WorkspaceSettings) IsBusinessDay(date time.Time) bool {
	// Check if it's a holiday
	for _, holiday := range ws.Holidays {
		if holiday.Date.Format("2006-01-02") == date.Format("2006-01-02") {
			return false
		}
		if holiday.IsRecurring && holiday.Date.Format("01-02") == date.Format("01-02") {
			return false
		}
	}

	// Check business hours
	dayName := date.Weekday().String()
	if hours, exists := ws.BusinessHours[dayName]; exists {
		return hours.IsBusinessDay
	}

	// Default: weekdays are business days
	weekday := date.Weekday()
	return weekday >= time.Monday && weekday <= time.Friday
}

// GetSLAForPriority gets SLA hours for a specific priority
func (ws *WorkspaceSettings) GetSLAForPriority(priority servicedomain.CasePriority) int {
	if ws.SLAByPriority != nil {
		if sla, exists := ws.SLAByPriority[string(priority)]; exists {
			return sla
		}
	}
	return ws.DefaultSLAHours
}

// IsFileTypeAllowed checks if a file type is allowed
func (ws *WorkspaceSettings) IsFileTypeAllowed(fileType string) bool {
	// Check blocked types first
	for _, blocked := range ws.BlockedFileTypes {
		if blocked == fileType {
			return false
		}
	}

	// If allowed types is empty, allow all (except blocked)
	if len(ws.AllowedFileTypes) == 0 {
		return true
	}

	// Check allowed types
	for _, allowed := range ws.AllowedFileTypes {
		if allowed == fileType {
			return true
		}
	}

	return false
}

// IsFileSizeAllowed checks if a file size is within limits
func (ws *WorkspaceSettings) IsFileSizeAllowed(fileSize int64) bool {
	return fileSize <= ws.MaxFileSize
}

// =============================================================================
// Read-only Accessor Methods
// =============================================================================
// These methods provide grouped access to related settings without changing
// the JSON serialization or storage format. Use these for cleaner code when
// accessing multiple related settings.

// EmailConfig provides read-only access to email configuration settings.
type EmailConfig struct {
	FromName        string
	FromAddress     string
	ReplyToAddress  string
	Signature       string
	Footer          string
	AutoResponse    bool
	AutoResponseTpl string
}

// Email returns the email configuration settings.
func (ws *WorkspaceSettings) Email() EmailConfig {
	return EmailConfig{
		FromName:        ws.EmailFromName,
		FromAddress:     ws.EmailFromAddress,
		ReplyToAddress:  ws.EmailReplyToAddress,
		Signature:       ws.EmailSignature,
		Footer:          ws.EmailFooter,
		AutoResponse:    ws.AutoResponseEnabled,
		AutoResponseTpl: ws.AutoResponseTemplate,
	}
}

// NotificationConfig provides read-only access to notification settings.
type NotificationConfig struct {
	OnNewCase    bool
	OnCaseUpdate bool
	OnAssignment bool
	OnEscalation bool
	Channels     []string
}

// Notifications returns the notification settings.
func (ws *WorkspaceSettings) Notifications() NotificationConfig {
	return NotificationConfig{
		OnNewCase:    ws.NotifyOnNewCase,
		OnCaseUpdate: ws.NotifyOnCaseUpdate,
		OnAssignment: ws.NotifyOnAssignment,
		OnEscalation: ws.NotifyOnEscalation,
		Channels:     ws.NotificationChannels,
	}
}

// CaseConfig provides read-only access to case management settings.
type CaseConfig struct {
	NumberPrefix    string
	NumberFormat    string
	DefaultStatus   servicedomain.CaseStatus
	DefaultPriority servicedomain.CasePriority
	AutoAssign      bool
	RequireCategory bool
}

// Case returns the case management settings.
func (ws *WorkspaceSettings) Case() CaseConfig {
	return CaseConfig{
		NumberPrefix:    ws.CaseNumberPrefix,
		NumberFormat:    ws.CaseNumberFormat,
		DefaultStatus:   ws.DefaultCaseStatus,
		DefaultPriority: ws.DefaultCasePriority,
		AutoAssign:      ws.AutoAssignCases,
		RequireCategory: ws.RequireCaseCategory,
	}
}

// SecurityConfig provides read-only access to security settings.
type SecurityConfig struct {
	PasswordMinLength        int
	PasswordRequireSpecial   bool
	PasswordRequireNumbers   bool
	PasswordRequireUppercase bool
	SessionTimeoutMinutes    int
	TwoFactorRequired        bool
	IPWhitelist              []string
	IPBlacklist              []string
}

// Security returns the security settings.
func (ws *WorkspaceSettings) Security() SecurityConfig {
	return SecurityConfig{
		PasswordMinLength:        ws.PasswordMinLength,
		PasswordRequireSpecial:   ws.PasswordRequireSpecial,
		PasswordRequireNumbers:   ws.PasswordRequireNumbers,
		PasswordRequireUppercase: ws.PasswordRequireUppercase,
		SessionTimeoutMinutes:    ws.SessionTimeoutMinutes,
		TwoFactorRequired:        ws.TwoFactorRequired,
		IPWhitelist:              ws.IPWhitelist,
		IPBlacklist:              ws.IPBlacklist,
	}
}

// SLAConfig provides read-only access to SLA settings.
type SLAConfig struct {
	DefaultHours      int
	ByPriority        map[string]int
	BusinessHoursOnly bool
}

// SLA returns the SLA settings.
func (ws *WorkspaceSettings) SLA() SLAConfig {
	return SLAConfig{
		DefaultHours:      ws.DefaultSLAHours,
		ByPriority:        ws.SLAByPriority,
		BusinessHoursOnly: ws.SLABusinessHoursOnly,
	}
}
