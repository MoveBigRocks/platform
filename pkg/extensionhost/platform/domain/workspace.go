package platformdomain

import (
	"fmt"
	"strings"
	"time"

	shared "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

// Workspace represents an isolated tenant/client
type Workspace struct {
	ID          string // UUIDv7
	Name        string
	Slug        string // Indexed for GetWorkspaceBySlug (public-facing URLs)
	ShortCode   string // 2-4 char code for case IDs (e.g., "ac" for Acme)
	Description string

	// Branding
	LogoURL      string
	PrimaryColor string
	AccentColor  string

	// Configuration
	Settings      shared.Metadata
	Features      []string // Enabled features
	StorageBucket string   // S3 bucket or prefix

	// Limits (can be customized per workspace)
	MaxUsers   int
	MaxCases   int
	MaxStorage int64

	// Status
	IsActive      bool
	IsSuspended   bool
	SuspendReason string

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// Team represents an operational unit within a workspace
type Team struct {
	ID          string
	WorkspaceID string // Indexed for ListWorkspaceTeams

	// Basic info
	Name        string
	Description string

	// Configuration
	EmailAddress string // Team inbox
	Settings     shared.Metadata

	// SLA Configuration
	ResponseTimeHours   int
	ResolutionTimeHours int

	// Auto-assignment rules
	AutoAssign         bool
	AutoAssignKeywords []string

	// Status
	IsActive bool

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TeamMemberRole defines the role a user has within a team
type TeamMemberRole string

const (
	TeamMemberRoleLead   TeamMemberRole = "lead"
	TeamMemberRoleMember TeamMemberRole = "member"
)

// TeamMember represents a user's membership in a team
type TeamMember struct {
	ID          string
	TeamID      string
	UserID      string
	WorkspaceID string
	Role        TeamMemberRole
	IsActive    bool
	JoinedAt    time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Contact represents an external user (customer)
type Contact struct {
	ID          string
	WorkspaceID string // Indexed for ListWorkspaceContacts

	// Basic info
	Email   string // Indexed for GetContactByEmail (email processing, de-duplication)
	Name    string
	Phone   string
	Company string

	// Additional info
	Tags         []string
	Notes        string
	CustomFields shared.TypedCustomFields

	// Preferences
	PreferredLanguage string
	Timezone          string

	// Status
	IsBlocked     bool
	BlockedReason string

	// Statistics
	TotalCases    int
	LastContactAt *time.Time

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewWorkspace creates a new workspace with default settings.
func NewWorkspace(name, slug string) *Workspace {
	return &Workspace{
		Name:       name,
		Slug:       slug,
		IsActive:   true,
		Settings:   shared.NewMetadata(),
		Features:   []string{},
		MaxUsers:   10,
		MaxCases:   1000,
		MaxStorage: 10 * 1024 * 1024 * 1024, // 10GB default
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// GenerateShortCode derives the human-facing workspace short code from a slug.
func GenerateWorkspaceShortCode(slug string) string {
	normalized := strings.ToLower(slug)
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, " ", "")

	if len(normalized) >= 4 {
		return normalized[:4]
	}
	if len(normalized) >= 2 {
		return normalized
	}
	if len(normalized) == 1 {
		return normalized + "1"
	}
	return id.New()[0:4]
}

// UpdateDetails applies the mutable workspace profile fields.
func (w *Workspace) UpdateDetails(name, slug, description string, updatedAt time.Time) {
	w.Name = name
	w.Slug = slug
	w.ShortCode = GenerateWorkspaceShortCode(slug)
	w.Description = description
	w.UpdatedAt = updatedAt
}

// ValidateDeletion enforces deletion preconditions for managed workspaces.
func (w *Workspace) ValidateDeletion(caseCount, memberCount, openIssueCount int) error {
	if caseCount > 0 {
		return fmt.Errorf("cannot delete workspace '%s': has %d active cases. Please close or transfer cases first", w.Name, caseCount)
	}
	if memberCount > 1 {
		return fmt.Errorf("cannot delete workspace '%s': has %d active members. Please remove members first", w.Name, memberCount)
	}
	if openIssueCount > 0 {
		return fmt.Errorf("cannot delete workspace '%s': has %d open issues. Please resolve issues first", w.Name, openIssueCount)
	}
	return nil
}

// NewContact creates a new contact with default values.
func NewContact(workspaceID, email string) *Contact {
	return &Contact{
		WorkspaceID:  workspaceID,
		Email:        NormalizeContactEmail(email),
		Tags:         []string{},
		CustomFields: shared.NewTypedCustomFields(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// GetStoragePath returns the S3 prefix for this workspace
func (w *Workspace) GetStoragePath() string {
	if w.StorageBucket != "" {
		return w.StorageBucket
	}
	return "workspaces/" + w.Slug
}

// IsAccessible checks if the workspace can be accessed
func (w *Workspace) IsAccessible() bool {
	return w.IsActive && !w.IsSuspended && w.DeletedAt == nil
}
