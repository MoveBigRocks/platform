package shareddomain

import (
	"encoding/json"
	"time"
)

// FieldMetadata represents typed metadata for field options
type FieldMetadata json.RawMessage

// CustomField represents a custom field definition
type CustomField struct {
	ID          string
	WorkspaceID string
	Name        string
	Label       string
	Description string

	// Field configuration
	Type       FieldType
	DataType   DataType
	Required   bool
	Unique     bool
	Searchable bool

	// Display settings
	DisplayOrder int
	GroupName    string
	Placeholder  string
	HelpText     string
	Icon         string
	Hidden       bool
	ReadOnly     bool

	// Validation
	Validation FieldValidation

	// Options (for select, multiselect, radio, checkbox)
	Options []FieldOption

	// Default value (JSON-encoded for type flexibility)
	DefaultValue json.RawMessage

	// Computed field settings
	IsComputed   bool
	Formula      string
	Dependencies []string // Field IDs this field depends on

	// Permissions
	ViewRoles []string
	EditRoles []string

	// Metadata
	Tags     []string
	IsSystem bool // System fields cannot be deleted
	IsActive bool

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
}

// FieldType represents the type of custom field
type FieldType string

const (
	FieldTypeText        FieldType = "text"
	FieldTypeTextArea    FieldType = "textarea"
	FieldTypeNumber      FieldType = "number"
	FieldTypeDate        FieldType = "date"
	FieldTypeDateTime    FieldType = "datetime"
	FieldTypeTime        FieldType = "time"
	FieldTypeSelect      FieldType = "select"
	FieldTypeMultiSelect FieldType = "multiselect"
	FieldTypeRadio       FieldType = "radio"
	FieldTypeCheckbox    FieldType = "checkbox"
	FieldTypeToggle      FieldType = "toggle"
	FieldTypeEmail       FieldType = "email"
	FieldTypePhone       FieldType = "phone"
	FieldTypeURL         FieldType = "url"
	FieldTypeCurrency    FieldType = "currency"
	FieldTypePercentage  FieldType = "percentage"
	FieldTypeRating      FieldType = "rating"
	FieldTypeFile        FieldType = "file"
	FieldTypeImage       FieldType = "image"
	FieldTypeColor       FieldType = "color"
	FieldTypeRichText    FieldType = "richtext"
	FieldTypeJSON        FieldType = "json"
	FieldTypeLookup      FieldType = "lookup"
	FieldTypeFormula     FieldType = "formula"
	FieldTypeRelation    FieldType = "relation"
	FieldTypeUser        FieldType = "user"
	FieldTypeLocation    FieldType = "location"
	FieldTypeSignature   FieldType = "signature"
)

// DataType represents the data type of a field value
type DataType string

const (
	DataTypeString  DataType = "string"
	DataTypeNumber  DataType = "number"
	DataTypeBoolean DataType = "boolean"
	DataTypeDate    DataType = "date"
	DataTypeArray   DataType = "array"
	DataTypeObject  DataType = "object"
	DataTypeBinary  DataType = "binary"
)

// FieldValidation represents validation rules for a field
type FieldValidation struct {
	// Text validation
	MinLength    *int
	MaxLength    *int
	Pattern      string // Regex pattern
	AllowedChars string

	// Number validation
	MinValue  *float64
	MaxValue  *float64
	Step      *float64
	Precision *int

	// Date validation
	MinDate     *time.Time
	MaxDate     *time.Time
	AllowPast   *bool
	AllowFuture *bool

	// File validation
	MaxFileSize  *int64   // bytes
	AllowedTypes []string // MIME types

	// Array validation
	MinItems    *int
	MaxItems    *int
	UniqueItems bool

	// Custom validation
	CustomValidator string // JavaScript function
	ErrorMessage    string
}

// FieldOption represents an option for select/radio fields
type FieldOption struct {
	Value       string
	Label       string
	Color       string
	Icon        string
	Description string
	IsDefault   bool
	IsDisabled  bool
	Order       int
	Metadata    json.RawMessage
}

// CustomFieldValue represents a value for a custom field
type CustomFieldValue struct {
	ID          string
	FieldID     string
	EntityType  string // case, contact, user, etc.
	EntityID    string
	WorkspaceID string

	// Value storage (use appropriate field based on data type)
	StringValue  string
	NumberValue  *float64
	BooleanValue *bool
	DateValue    *time.Time
	ArrayValue   []string
	ObjectValue  json.RawMessage
	BinaryValue  []byte

	// Formatted value for display
	DisplayValue string

	// File/Image specific
	FileURL      string
	FileName     string
	FileSize     int64
	FileMimeType string

	// Relation specific
	RelatedEntity string
	RelatedID     string

	// Metadata
	IsValid         bool
	ValidationError string

	CreatedAt time.Time
	UpdatedAt time.Time
	UpdatedBy string
}

// FieldMapping represents how custom fields map to entities
type FieldMapping struct {
	ID          string
	WorkspaceID string
	FieldID     string
	EntityType  string

	// Display settings for this entity type
	Section    string // Which section of the UI
	Order      int
	IsRequired bool
	IsVisible  bool

	// Conditional display
	ShowWhen    []FieldCondition
	RequireWhen []FieldCondition

	CreatedAt time.Time
	UpdatedAt time.Time
}

// FieldCondition represents a condition for field display/requirement
type FieldCondition struct {
	FieldID   string
	Operator  string // equals, not_equals, contains, etc.
	Value     json.RawMessage
	LogicalOp string // AND, OR for multiple conditions
}

// FieldGroup represents a group of related custom fields
type FieldGroup struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	Icon        string

	// Fields in this group
	FieldIDs []string

	// Display settings
	Order       int
	Collapsible bool
	Collapsed   bool

	// Permissions
	ViewRoles []string
	EditRoles []string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// FieldTemplate represents a template for custom fields
type FieldTemplate struct {
	ID          string
	Name        string
	Description string
	Category    string

	// Template fields
	Fields []CustomField
	Groups []FieldGroup

	// Usage
	EntityTypes []string // Which entities this template applies to
	Industry    string
	UseCase     string

	// Metadata
	IsPublic bool
	IsSystem bool
	Tags     []string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
}

// FieldHistory represents the history of changes to a custom field value
type FieldHistory struct {
	ID         string
	FieldID    string
	EntityType string
	EntityID   string

	// Change details
	OldValue   json.RawMessage
	NewValue   json.RawMessage
	ChangeType string // created, updated, deleted

	// Context
	ChangedBy    string
	ChangedAt    time.Time
	ChangeReason string

	// Audit
	IPAddress string
	UserAgent string
}

// FieldMigration represents a migration for custom fields
type FieldMigration struct {
	ID          string
	WorkspaceID string

	// Migration details
	Type        MigrationType
	SourceField string
	TargetField string

	// Transformation
	Transform string // JavaScript function
	Mapping   json.RawMessage

	// Status
	Status           MigrationStatus
	Progress         int // Percentage
	TotalRecords     int
	ProcessedRecords int
	FailedRecords    int

	// Execution
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       string

	// Rollback
	CanRollback  bool
	RollbackData json.RawMessage

	CreatedAt time.Time
	CreatedBy string
}

// MigrationType represents the type of field migration
type MigrationType string

const (
	MigrationTypeRename    MigrationType = "rename"
	MigrationTypeTransform MigrationType = "transform"
	MigrationTypeMerge     MigrationType = "merge"
	MigrationTypeSplit     MigrationType = "split"
	MigrationTypeDelete    MigrationType = "delete"
	MigrationTypeCopy      MigrationType = "copy"
)

// MigrationStatus represents the status of a migration
type MigrationStatus string

const (
	MigrationStatusPending    MigrationStatus = "pending"
	MigrationStatusRunning    MigrationStatus = "running"
	MigrationStatusCompleted  MigrationStatus = "completed"
	MigrationStatusFailed     MigrationStatus = "failed"
	MigrationStatusRolledBack MigrationStatus = "rolled_back"
)

// FieldStatistics represents usage statistics for a custom field
type FieldStatistics struct {
	FieldID     string
	WorkspaceID string

	// Usage metrics
	TotalEntities int
	FilledCount   int
	EmptyCount    int
	FillRate      float64 // Percentage

	// Value distribution (for select/multiselect)
	ValueDistribution map[string]int

	// Statistics (for number fields)
	MinValue    *float64
	MaxValue    *float64
	AvgValue    *float64
	MedianValue *float64

	// Update frequency
	UpdatesLastDay   int
	UpdatesLastWeek  int
	UpdatesLastMonth int

	// Last activity
	LastUpdated   time.Time
	LastUpdatedBy string

	ComputedAt time.Time
}

// CustomFieldGroup represents a group of custom fields
type CustomFieldGroup struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	EntityType  string   // "case", "contact", "company", etc.
	Fields      []string // Field IDs in this group
	Order       int
	IsCollapsed bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
