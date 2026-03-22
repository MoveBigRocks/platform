package models

import (
	"time"
)

// CustomField represents a custom field definition
type CustomField struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`
	Name        string `db:"name"`
	Label       string `db:"label"`
	Description string `db:"description"`

	// Field configuration
	Type       string `db:"type"`
	DataType   string `db:"data_type"`
	Required   bool   `db:"required"`
	Unique     bool   `db:"unique"`
	Searchable bool   `db:"searchable"`

	// Display settings
	DisplayOrder int    `db:"display_order"`
	GroupName    string `db:"group_name"`
	Placeholder  string `db:"placeholder"`
	HelpText     string `db:"help_text"`
	Icon         string `db:"icon"`
	Hidden       bool   `db:"hidden"`
	ReadOnly     bool   `db:"read_only"`

	// Validation
	Validation string `db:"validation"`

	// Options
	Options string `db:"options"`

	// Default value
	DefaultValue string `db:"default_value"`

	// Computed field settings
	IsComputed   bool   `db:"is_computed"`
	Formula      string `db:"formula"`
	Dependencies string `db:"dependencies"`

	// Permissions
	ViewRoles string `db:"view_roles"`
	EditRoles string `db:"edit_roles"`

	// Metadata
	Tags      string    `db:"tags"`
	IsSystem  bool      `db:"is_system"`
	IsActive  bool      `db:"is_active"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	CreatedBy string    `db:"created_by"`
}
