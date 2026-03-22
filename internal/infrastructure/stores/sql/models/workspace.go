package models

import (
	"time"
)

// Workspace represents an isolated tenant/client
type Workspace struct {
	ID            string     `db:"id"`
	Name          string     `db:"name"`
	Slug          string     `db:"slug"`
	ShortCode     *string    `db:"short_code"`
	Description   *string    `db:"description"`
	LogoURL       *string    `db:"logo_url"`
	PrimaryColor  *string    `db:"primary_color"`
	AccentColor   *string    `db:"accent_color"`
	Settings      *string    `db:"settings"`
	Features      *string    `db:"features"`
	StorageBucket *string    `db:"storage_bucket"`
	MaxUsers      *int       `db:"max_users"`
	MaxCases      *int       `db:"max_cases"`
	MaxStorage    *int64     `db:"max_storage"`
	IsActive      bool       `db:"is_active"`
	IsSuspended   bool       `db:"is_suspended"`
	SuspendReason *string    `db:"suspend_reason"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at"`
}

func (Workspace) TableName() string {
	return "workspaces"
}

// UserWorkspaceRole links a user to a workspace
type UserWorkspaceRole struct {
	ID          string     `db:"id"`
	UserID      string     `db:"user_id"`
	WorkspaceID string     `db:"workspace_id"`
	Role        string     `db:"role"`
	Permissions string     `db:"permissions"`
	InvitedBy   *string    `db:"invited_by"`
	RevokedAt   *time.Time `db:"revoked_at"`
	ExpiresAt   *time.Time `db:"expires_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

func (UserWorkspaceRole) TableName() string {
	return "user_workspace_roles"
}

// Team represents a team within a workspace
type Team struct {
	ID                  string    `db:"id"`
	WorkspaceID         string    `db:"workspace_id"`
	Name                string    `db:"name"`
	Description         string    `db:"description"`
	EmailAddress        string    `db:"email_address"`
	Settings            string    `db:"settings"`
	ResponseTimeHours   int       `db:"response_time_hours"`
	ResolutionTimeHours int       `db:"resolution_time_hours"`
	AutoAssign          bool      `db:"auto_assign"`
	AutoAssignKeywords  string    `db:"auto_assign_keywords"`
	IsActive            bool      `db:"is_active"`
	CreatedAt           time.Time `db:"created_at"`
	UpdatedAt           time.Time `db:"updated_at"`
}

func (Team) TableName() string {
	return "teams"
}

// TeamMember represents a user in a team
type TeamMember struct {
	ID          string    `db:"id"`
	TeamID      string    `db:"team_id"`
	UserID      string    `db:"user_id"`
	WorkspaceID string    `db:"workspace_id"`
	Role        string    `db:"role"`
	IsActive    bool      `db:"is_active"`
	JoinedAt    time.Time `db:"joined_at"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (TeamMember) TableName() string {
	return "team_members"
}

// Contact represents an external user
type Contact struct {
	ID                string     `db:"id"`
	WorkspaceID       string     `db:"workspace_id"`
	Email             string     `db:"email"`
	Name              string     `db:"name"`
	Phone             string     `db:"phone"`
	Company           string     `db:"company"`
	Tags              string     `db:"tags"`
	Notes             string     `db:"notes"`
	CustomFields      string     `db:"custom_fields"`
	PreferredLanguage string     `db:"preferred_language"`
	Timezone          string     `db:"timezone"`
	IsBlocked         bool       `db:"is_blocked"`
	BlockedReason     string     `db:"blocked_reason"`
	TotalCases        int        `db:"total_cases"`
	LastContactAt     *time.Time `db:"last_contact_at"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
}

func (Contact) TableName() string {
	return "contacts"
}

// WorkspaceSettings represents per-workspace configuration
type WorkspaceSettings struct {
	ID               string    `db:"id"`
	WorkspaceID      string    `db:"workspace_id"`
	EmailFromName    string    `db:"email_from_name"`
	EmailFromAddress string    `db:"email_from_address"`
	SettingsJSON     string    `db:"settings_json"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

func (WorkspaceSettings) TableName() string {
	return "workspace_settings"
}
