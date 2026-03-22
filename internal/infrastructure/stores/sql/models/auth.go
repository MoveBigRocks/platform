package models

import (
	"time"
)

// Session represents a user session
type Session struct {
	ID        string `db:"id"`
	TokenHash string `db:"token_hash"`
	UserID    string `db:"user_id"`

	// User info cached for performance
	Email string `db:"email"`
	Name  string `db:"name"`

	// Current context
	CurrentContextType        string  `db:"current_context_type"`
	CurrentContextRole        string  `db:"current_context_role"`
	CurrentContextWorkspaceID *string `db:"current_context_workspace_id"`

	// Available contexts (JSON array)
	AvailableContexts string `db:"available_contexts"`

	// Device info
	UserAgent string `db:"user_agent"`
	IPAddress string `db:"ip_address"`

	// Session lifecycle
	CreatedAt      time.Time  `db:"created_at"`
	ExpiresAt      time.Time  `db:"expires_at"`
	LastActivityAt time.Time  `db:"last_activity_at"`
	RevokedAt      *time.Time `db:"revoked_at"`
}

func (Session) TableName() string {
	return "sessions"
}

// MagicLinkToken represents a magic link for authentication
type MagicLinkToken struct {
	Token     string     `db:"token"`
	Email     string     `db:"email"`
	UserID    *string    `db:"user_id"`
	ExpiresAt time.Time  `db:"expires_at"`
	Used      bool       `db:"used"`
	UsedAt    *time.Time `db:"used_at"`
	CreatedAt time.Time  `db:"created_at"`
}

func (MagicLinkToken) TableName() string {
	return "magic_links"
}

// RateLimitEntry tracks rate limiting for various operations
type RateLimitEntry struct {
	Key       string     `db:"key"`
	Count     int        `db:"count"`
	FirstAt   time.Time  `db:"first_at"`
	LastAt    time.Time  `db:"last_at"`
	Blocked   bool       `db:"blocked"`
	BlockedAt *time.Time `db:"blocked_at"`
	ExpiresAt time.Time  `db:"expires_at"`
}

func (RateLimitEntry) TableName() string {
	return "rate_limit_entries"
}

// =============================================================================
// Role & Permission Models
// =============================================================================

// Role represents a workspace role with permissions
type Role struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`

	// Role details
	Name        string `db:"name"`
	Description string `db:"description"`
	Type        string `db:"type"`
	IsSystem    bool   `db:"is_system"`
	IsDefault   bool   `db:"is_default"`

	// Permissions (JSON)
	Permissions string `db:"permissions"`

	// Metadata
	CreatedByID string    `db:"created_by_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// UserRole links users to roles
type UserRole struct {
	ID          string `db:"id"`
	UserID      string `db:"user_id"`
	WorkspaceID string `db:"workspace_id"`
	RoleID      string `db:"role_id"`

	// Assignment context
	AssignedByID string     `db:"assigned_by_id"`
	AssignedAt   time.Time  `db:"assigned_at"`
	ExpiresAt    *time.Time `db:"expires_at"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// =============================================================================
// Notification Models
// =============================================================================

// Notification represents a user notification
type Notification struct {
	ID          string `db:"id"`
	WorkspaceID string `db:"workspace_id"`
	UserID      string `db:"user_id"`

	// Notification content
	Type    string `db:"type"`
	Title   string `db:"title"`
	Body    string `db:"body"`
	IconURL string `db:"icon_url"`

	// Target
	TargetType string `db:"target_type"`
	TargetID   string `db:"target_id"`

	// Action
	ActionURL   string `db:"action_url"`
	ActionLabel string `db:"action_label"`

	// Status
	IsRead     bool       `db:"is_read"`
	ReadAt     *time.Time `db:"read_at"`
	IsArchived bool       `db:"is_archived"`
	ArchivedAt *time.Time `db:"archived_at"`

	// Priority
	Priority string `db:"priority"`

	// Delivery
	DeliveryMethods string     `db:"delivery_methods"`
	EmailSentAt     *time.Time `db:"email_sent_at"`
	PushSentAt      *time.Time `db:"push_sent_at"`
	SMSSentAt       *time.Time `db:"sms_sent_at"`

	// Expiration
	ExpiresAt *time.Time `db:"expires_at"`

	// Metadata
	Metadata  string    `db:"metadata"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// =============================================================================
// Portal Access Models
// =============================================================================

// PortalAccessToken grants temporary portal access to contacts
type PortalAccessToken struct {
	Token       string `db:"token"`
	WorkspaceID string `db:"workspace_id"`

	// Target
	ContactID string `db:"contact_id"`
	Email     string `db:"email"`

	// Token details
	Type string `db:"type"`

	// Status
	Used   bool       `db:"used"`
	UsedAt *time.Time `db:"used_at"`

	// Scope (JSON)
	Scopes string `db:"scopes"`

	// Timing
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}
