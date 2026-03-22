package servicedomain

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	shared "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

// FormStatus represents the status of a form
type FormStatus string

const (
	FormStatusDraft    FormStatus = "draft"
	FormStatusActive   FormStatus = "active"
	FormStatusInactive FormStatus = "inactive"
	FormStatusArchived FormStatus = "archived"
)

// SubmissionStatus represents the status of a form submission
type SubmissionStatus string

const (
	SubmissionStatusPending    SubmissionStatus = "pending"
	SubmissionStatusProcessing SubmissionStatus = "processing"
	SubmissionStatusCompleted  SubmissionStatus = "completed"
	SubmissionStatusFailed     SubmissionStatus = "failed"
)

// FormSchema represents a public form definition backed by a form spec.
type FormSchema struct {
	ID          string
	WorkspaceID string

	// Basic info
	Name        string
	Description string
	Slug        string // URL-friendly identifier

	// Form configuration
	SchemaData      shared.TypedSchema // Structured field definition data
	UISchema        shared.TypedSchema // UI rendering configuration
	ValidationRules shared.TypedSchema // Custom validation rules

	// Form settings
	Status        FormStatus
	IsPublic      bool // Can be accessed without login
	RequiresAuth  bool // Requires authentication
	AllowMultiple bool // Multiple submissions per user
	CollectEmail  bool // Collect submitter email

	// Auto-actions
	AutoCreateCase   bool     // Create case from submission
	AutoAssignTeamID string   // Auto-assign to team
	AutoAssignUserID string   // Auto-assign to user
	AutoCasePriority string   // Default priority for cases
	AutoCaseType     string   // Default case type
	AutoTags         []string // Tags to add to created cases

	// Notification settings
	NotifyOnSubmission     bool
	NotificationEmails     []string
	NotificationWebhookURL string

	// Appearance
	Theme             string
	CustomCSS         string
	SubmissionMessage string
	RedirectURL       string

	// Analytics
	SubmissionCount  int
	LastSubmissionAt *time.Time
	ConversionRate   float64

	// Access control
	AllowedDomains       []string // Email domain restrictions
	BlockedDomains       []string // Blocked domains
	RequiresCaptcha      bool
	MaxSubmissionsPerDay int

	// Embedding support
	AllowEmbed   bool     // Allow embedding on external sites
	EmbedDomains []string // Allowed domains for embedding
	CryptoID     string   // Public access token for embedded forms

	// Automation integration
	TriggerRuleIDs []string // Rules to trigger on form submission

	// Multi-step workflow support
	HasWorkflow    bool
	WorkflowStates []FormWorkflowState
	Transitions    []FormWorkflowTransition

	// Metadata
	CreatedByID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// GetStartState returns the starting workflow state, or nil if no workflow
func (fs *FormSchema) GetStartState() *FormWorkflowState {
	if !fs.HasWorkflow || len(fs.WorkflowStates) == 0 {
		return nil
	}
	for i := range fs.WorkflowStates {
		if fs.WorkflowStates[i].IsStart {
			return &fs.WorkflowStates[i]
		}
	}
	// Fallback to first state if no explicit start
	return &fs.WorkflowStates[0]
}

// GetStateByID returns a workflow state by ID
func (fs *FormSchema) GetStateByID(stateID string) *FormWorkflowState {
	for i := range fs.WorkflowStates {
		if fs.WorkflowStates[i].ID == stateID {
			return &fs.WorkflowStates[i]
		}
	}
	return nil
}

// GetTransitionsFromState returns all transitions from a given state
func (fs *FormSchema) GetTransitionsFromState(stateID string) []FormWorkflowTransition {
	var transitions []FormWorkflowTransition
	for _, t := range fs.Transitions {
		if t.FromStateID == stateID {
			transitions = append(transitions, t)
		}
	}
	return transitions
}

// FormSubmission represents a submission to a form
type PublicFormSubmission struct {
	ID          string
	WorkspaceID string
	FormID      string

	// Submission data
	Data          shared.Metadata // Form field values
	RawData       string          // Original JSON data
	ProcessedData shared.Metadata // Cleaned/processed data

	// Submitter info
	SubmitterEmail string
	SubmitterName  string
	SubmitterIP    string
	UserAgent      string
	Referrer       string

	// Processing status
	Status          SubmissionStatus
	ProcessingError string
	ProcessingNotes string

	// Auto-created resources
	CaseID    string // If auto-created case
	ContactID string // If auto-created contact

	// Validation
	IsValid          bool
	ValidationErrors []string

	// Spam detection
	SpamScore   float64
	IsSpam      bool
	SpamReasons []string

	// Processing tracking
	ProcessedAt    *time.Time
	ProcessedByID  string
	ProcessingTime int64 // milliseconds

	// Attachments
	AttachmentIDs []string

	// Multi-step workflow tracking
	CurrentStateID  string
	StateHistory    []StateTransition
	CompletionToken string // Token for resuming multi-step form

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TransitionToState moves the submission to a new workflow state
func (fs *PublicFormSubmission) TransitionToState(toStateID string, data shared.Metadata, userID string) {
	transition := StateTransition{
		FromStateID: fs.CurrentStateID,
		ToStateID:   toStateID,
		Timestamp:   time.Now(),
		Data:        data,
		UserID:      userID,
	}
	fs.StateHistory = append(fs.StateHistory, transition)
	fs.CurrentStateID = toStateID

	// Merge step data into submission data
	if !data.IsEmpty() {
		fs.Data.Merge(data)
	}
	fs.UpdatedAt = time.Now()
}

// IsInEndState checks if the submission is in a terminal workflow state
func (fs *PublicFormSubmission) IsInEndState(form *FormSchema) bool {
	if !form.HasWorkflow || fs.CurrentStateID == "" {
		return fs.Status == SubmissionStatusCompleted
	}
	state := form.GetStateByID(fs.CurrentStateID)
	return state != nil && state.IsEnd
}

// GetVisibleFields returns the fields visible in the current workflow state
func (fs *PublicFormSubmission) GetVisibleFields(form *FormSchema) []string {
	if !form.HasWorkflow || fs.CurrentStateID == "" {
		// No workflow - all fields visible
		return nil
	}
	state := form.GetStateByID(fs.CurrentStateID)
	if state == nil {
		return nil
	}
	return state.Fields
}

// FormField represents a field definition in a public form.
type FormField struct {
	Name        string
	Type        string // "text", "email", "textarea", "select", "checkbox", etc.
	Label       string
	Description string
	Required    bool
	Options     []FormFieldOption // For select, radio, checkbox
	Validation  shared.Metadata
	Default     shared.Value
	Placeholder string
	Order       int
}

// FormFieldOption represents an option for select/radio/checkbox fields
type FormFieldOption struct {
	Value string
	Label string
	Order int
}

// ==================== MULTI-STEP WORKFLOW SUPPORT ====================

// FormWorkflowState represents a state in a multi-step form workflow
type FormWorkflowState struct {
	ID          string
	FormID      string
	Name        string
	Description string
	IsStart     bool     // Is this the starting state?
	IsEnd       bool     // Is this a terminal state?
	DataLocked  bool     // Are fields locked in this state?
	Fields      []string // Fields visible in this state
	Order       int      // Display order
}

// FormWorkflowTransition represents a transition between workflow states
type FormWorkflowTransition struct {
	ID          string
	FormID      string
	FromStateID string
	ToStateID   string
	Label       string          // Button text for the transition
	Conditions  shared.Metadata // Optional conditions for transition
}

// StateTransition records a state change in a form submission
type StateTransition struct {
	FromStateID string
	ToStateID   string
	Timestamp   time.Time
	Data        shared.Metadata // Data submitted in this step
	UserID      string
}

// FormAnalytics represents analytics data for a form
type FormAnalytics struct {
	FormID      string
	WorkspaceID string

	// Submission metrics
	TotalSubmissions   int
	ValidSubmissions   int
	InvalidSubmissions int
	SpamSubmissions    int

	// Time-based metrics
	SubmissionsToday int
	SubmissionsWeek  int
	SubmissionsMonth int

	// Performance metrics
	AverageProcessingTime int64 // milliseconds
	ConversionRate        float64
	AbandonmentRate       float64

	// Field analytics
	FieldCompletionRates map[string]float64
	FieldErrorRates      map[string]float64

	// Traffic sources
	ReferrerStats   map[string]int
	DeviceStats     map[string]int
	GeographicStats map[string]int

	// Metadata
	LastUpdated time.Time
}

// NewFormSchema creates a new public form definition.
func NewFormSchema(workspaceID, name, slug string, createdByID string) *FormSchema {
	return &FormSchema{
		WorkspaceID:        workspaceID,
		Name:               name,
		Slug:               slug,
		Status:             FormStatusDraft,
		IsPublic:           false,
		RequiresAuth:       true,
		AllowMultiple:      true,
		CollectEmail:       true,
		AutoCreateCase:     true,
		SchemaData:         shared.NewTypedSchema(),
		UISchema:           shared.NewTypedSchema(),
		ValidationRules:    shared.NewTypedSchema(),
		NotificationEmails: []string{},
		AutoTags:           []string{},
		AllowedDomains:     []string{},
		BlockedDomains:     []string{},
		CreatedByID:        createdByID,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
		CryptoID:           GenerateCryptoID(),
	}
}

func GenerateCryptoID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return id.NewPublicID()
	}
	return hex.EncodeToString(bytes)
}

// NewFormSubmission creates a new form submission
func NewPublicFormSubmission(workspaceID, formID string, data shared.Metadata) *PublicFormSubmission {
	return &PublicFormSubmission{
		WorkspaceID:      workspaceID,
		FormID:           formID,
		Data:             data,
		ProcessedData:    shared.NewMetadata(),
		Status:           SubmissionStatusPending,
		IsValid:          true,
		ValidationErrors: []string{},
		SpamReasons:      []string{},
		AttachmentIDs:    []string{},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

// IsExpired checks if the form submission processing has expired
func (fs *PublicFormSubmission) IsExpired() bool {
	// Consider submissions older than 24 hours as expired if still processing
	return fs.Status == SubmissionStatusProcessing &&
		time.Since(fs.CreatedAt) > 24*time.Hour
}

// GetFieldValue safely gets a field value from the submission data
func (fs *PublicFormSubmission) GetFieldValue(fieldName string) (shared.Value, bool) {
	return fs.Data.Get(fieldName)
}

// GetStringField gets a string field value
func (fs *PublicFormSubmission) GetStringField(fieldName string) string {
	return fs.Data.GetString(fieldName)
}

// MarshalSchemaData marshals structured field definition data to JSON.
func (fs *FormSchema) MarshalSchemaData() ([]byte, error) {
	return fs.SchemaData.MarshalJSON()
}

// UnmarshalSchemaData unmarshals JSON into structured field definition data.
func (fs *FormSchema) UnmarshalSchemaData(data []byte) error {
	return fs.SchemaData.UnmarshalJSON(data)
}

// JSONSchemaData represents typed field definition data for form validation.
// This provides type-safe access to field properties.
type JSONSchemaData struct {
	Type        string // "object" for forms
	Title       string
	Description string
	Properties  map[string]JSONFieldSchema
	Required    []string
}

// JSONFieldSchema represents the definition for a single form field.
type JSONFieldSchema struct {
	Type        string // "string", "number", "boolean", "array"
	Title       string
	Description string
	Format      string   // "email", "uri", "date", etc.
	Enum        []string // Allowed values
	Default     any
	MinLength   int
	MaxLength   int
	Minimum     *float64
	Maximum     *float64
	Pattern     string // Regex pattern
	ReadOnly    bool
}

// ParseSchemaData parses the SchemaData map into typed JSONSchemaData.
// This provides type-safe access to the underlying field definition data.
func (fs *FormSchema) ParseSchemaData() (*JSONSchemaData, error) {
	if fs.SchemaData.IsEmpty() {
		return &JSONSchemaData{}, nil
	}

	// Re-marshal and unmarshal for type conversion
	data, err := fs.SchemaData.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var schema JSONSchemaData
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}

	return &schema, nil
}

// GetRequiredFields returns the list of required field names from the schema.
func (fs *FormSchema) GetRequiredFields() []string {
	schema, err := fs.ParseSchemaData()
	if err != nil {
		return nil
	}
	return schema.Required
}

// GetFieldSchema returns the schema for a specific field.
func (fs *FormSchema) GetFieldSchema(fieldName string) (*JSONFieldSchema, bool) {
	schema, err := fs.ParseSchemaData()
	if err != nil {
		return nil, false
	}

	if schema.Properties == nil {
		return nil, false
	}

	fieldSchema, exists := schema.Properties[fieldName]
	return &fieldSchema, exists
}
