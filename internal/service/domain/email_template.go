package servicedomain

import (
	"time"
)

// EmailTemplate represents an email template
type EmailTemplate struct {
	ID          string
	WorkspaceID string

	// Template info
	Name        string
	Subject     string
	Description string

	// Template content
	HTMLContent string
	TextContent string

	// Template variables
	Variables  []EmailTemplateVariable
	SampleData map[string]interface{}

	// Settings
	IsActive bool
	Category string
	Language string

	// Usage tracking
	TimesUsed  int
	LastUsedAt *time.Time

	// Version control
	Version  int
	ParentID string // For versioning

	// Metadata
	CreatedByID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// EmailTemplateVariable represents a variable in an email template
type EmailTemplateVariable struct {
	Name         string
	Type         string // "string", "number", "date", "boolean"
	Description  string
	Required     bool
	DefaultValue string
}

// NewEmailTemplate creates a new email template
func NewEmailTemplate(workspaceID, name, subject string, createdByID string) *EmailTemplate {
	return &EmailTemplate{
		WorkspaceID: workspaceID,
		Name:        name,
		Subject:     subject,
		IsActive:    true,
		Language:    "en",
		Version:     1,
		Variables:   []EmailTemplateVariable{},
		SampleData:  make(map[string]interface{}),
		CreatedByID: createdByID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
