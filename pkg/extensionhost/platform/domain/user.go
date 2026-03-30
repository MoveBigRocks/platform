package platformdomain

import (
	"strings"
	"time"
)

// InstanceRole represents instance-level administrative roles
type InstanceRole string

const (
	InstanceRoleSuperAdmin InstanceRole = "super_admin" // Full instance access across all workspaces + full operational scope
	InstanceRoleAdmin      InstanceRole = "admin"       // Full operator scope plus user-management permission
	InstanceRoleOperator   InstanceRole = "operator"    // Platform admin operations across all workspaces, excluding user management
)

// CanonicalizeInstanceRole normalizes older and cased values to the canonical instance role.
func CanonicalizeInstanceRole(role InstanceRole) InstanceRole {
	switch InstanceRole(strings.ToLower(strings.TrimSpace(string(role)))) {
	case InstanceRoleSuperAdmin, InstanceRoleAdmin, InstanceRoleOperator:
		return InstanceRole(strings.ToLower(strings.TrimSpace(string(role))))
	}
	return ""
}

// IsValidInstanceRole checks whether the role is a supported instance role after normalization.
func IsValidInstanceRole(role InstanceRole) bool {
	switch CanonicalizeInstanceRole(role) {
	case InstanceRoleSuperAdmin, InstanceRoleAdmin, InstanceRoleOperator:
		return true
	default:
		return false
	}
}

// IsSuperAdmin checks whether this role is super_admin.
func (r InstanceRole) IsSuperAdmin() bool {
	return CanonicalizeInstanceRole(r) == InstanceRoleSuperAdmin
}

// IsAdmin checks whether this role can manage other users.
func (r InstanceRole) IsAdmin() bool {
	canonical := CanonicalizeInstanceRole(r)
	return canonical == InstanceRoleSuperAdmin || canonical == InstanceRoleAdmin
}

// IsOperator checks whether this role is allowed into the instance admin panel.
func (r InstanceRole) IsOperator() bool {
	canonical := CanonicalizeInstanceRole(r)
	return canonical == InstanceRoleSuperAdmin || canonical == InstanceRoleAdmin || canonical == InstanceRoleOperator
}

// WorkspaceRole represents workspace-scoped roles
type WorkspaceRole string

const (
	WorkspaceRoleOwner  WorkspaceRole = "owner"  // Created workspace, billing access
	WorkspaceRoleAdmin  WorkspaceRole = "admin"  // Full workspace management
	WorkspaceRoleMember WorkspaceRole = "member" // Can manage cases, view errors
	WorkspaceRoleViewer WorkspaceRole = "viewer" // Read-only access
)

// User represents a system user (global entity)
type User struct {
	ID     string // UUIDv7
	Email  string // Indexed for GetUserByEmail in auth flow
	Name   string
	Avatar string

	// Instance-level role (optional - most users won't have this)
	// Only users who need to access the instance admin panel should have this
	InstanceRole *InstanceRole

	// Authentication & Security
	IsActive      bool
	EmailVerified bool
	LockedUntil   *time.Time
	LastLoginAt   *time.Time
	LastLoginIP   string

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (u *User) CanAccessAdminPanel() bool {
	return u.InstanceRole != nil && u.InstanceRole.IsOperator() && u.IsActive && u.EmailVerified && !u.IsLocked()
}

func (u *User) IsSuperAdmin() bool {
	return u.InstanceRole != nil && u.InstanceRole.IsSuperAdmin()
}

func (u *User) IsInstanceAdmin() bool {
	return u.InstanceRole != nil && u.InstanceRole.IsOperator()
}

func (u *User) CanManageUsers() bool {
	return u.InstanceRole != nil && u.InstanceRole.IsAdmin()
}

// CanonicalizeRole returns the normalized role to keep storage and session data aligned.
func (u *User) CanonicalizeRole() {
	if u.InstanceRole == nil {
		return
	}

	canonical := CanonicalizeInstanceRole(*u.InstanceRole)
	if canonical != "" && IsValidInstanceRole(canonical) {
		u.InstanceRole = &canonical
	} else {
		u.InstanceRole = nil
	}
}

func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// UserWorkspaceRole links a user to a workspace with a specific role
type UserWorkspaceRole struct {
	ID          string
	UserID      string        // Indexed for GetUserWorkspaceRoles (authorization checks)
	WorkspaceID string        // Indexed for GetWorkspaceUsers (workspace member lists)
	Role        WorkspaceRole // Indexed for role-based queries

	// Permissions can be customized per workspace
	Permissions []string

	// Invitation & lifecycle
	InvitedBy *string    // User who invited this member (nil for workspace creators)
	RevokedAt *time.Time // When membership was revoked

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt *time.Time // Optional expiration for temporary access
}

func (uwr *UserWorkspaceRole) IsActive() bool {
	if uwr.RevokedAt != nil {
		return false
	}
	if uwr.ExpiresAt != nil && time.Now().After(*uwr.ExpiresAt) {
		return false
	}
	return true
}

// MagicLinkToken represents a passwordless authentication token
type MagicLinkToken struct {
	ID     string
	Token  string // Indexed for GetMagicLink in passwordless auth
	Email  string // Indexed for ListTokensByEmail (admin, cleanup)
	UserID string // Empty for new users

	// Security
	IPAddress string
	UserAgent string
	Used      bool
	UsedAt    *time.Time

	// Metadata
	CreatedAt time.Time
	ExpiresAt time.Time
}

// ContextType represents the type of context (instance admin or workspace)
type ContextType string

const (
	ContextTypeInstance  ContextType = "instance"  // Instance Admin Panel context
	ContextTypeWorkspace ContextType = "workspace" // Workspace Portal context
)

// Context represents what the user is currently viewing/acting as
type Context struct {
	Type          ContextType // "instance" or "workspace"
	WorkspaceID   *string     // Set if type == "workspace"
	WorkspaceName *string     // Workspace name for display
	WorkspaceSlug *string     // Workspace slug for subdomain routing
	Role          string      // Instance role or workspace role
	// Note: Permissions are computed on-the-fly, not stored
}

// Session represents an active user session with context switching support
type Session struct {
	ID        string
	TokenHash string // SHA-256 hash of the session token (indexed for lookup)
	UserID    string // Indexed for ListUserSessions (session cleanup)

	// User info (cached for performance)
	Email string
	Name  string

	// Current context (what the user is viewing NOW)
	CurrentContext Context

	// Available contexts (computed at login, refreshed on context switch)
	// This is cached to avoid repeated database queries
	AvailableContexts []Context

	// Device info
	IPAddress string
	UserAgent string

	// Session lifecycle
	CreatedAt      time.Time
	ExpiresAt      time.Time
	LastActivityAt time.Time // Updated on each request
	RevokedAt      *time.Time
}

func (s *Session) IsValid() bool {
	if s.RevokedAt != nil {
		return false
	}
	if time.Now().After(s.ExpiresAt) {
		return false
	}
	return true
}

func (s *Session) IsIdle(maxIdleDuration time.Duration) bool {
	return time.Since(s.LastActivityAt) > maxIdleDuration
}

func (s *Session) UpdateActivity() {
	s.LastActivityAt = time.Now()
}

func (s *Session) IsInstanceContext() bool {
	return s.CurrentContext.Type == ContextTypeInstance
}

func (s *Session) IsWorkspaceContext() bool {
	return s.CurrentContext.Type == ContextTypeWorkspace
}

func (s *Session) GetCurrentWorkspaceID() *string {
	if s.CurrentContext.Type == ContextTypeWorkspace {
		return s.CurrentContext.WorkspaceID
	}
	return nil
}

func (s *Session) HasInstanceAccess() bool {
	for _, ctx := range s.AvailableContexts {
		if ctx.Type == ContextTypeInstance {
			return true
		}
	}
	return false
}

func NewUser(email, name string) *User {
	return &User{
		Email:     email,
		Name:      name,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (u *User) HasWorkspaceRole(workspaceID string, role WorkspaceRole, roles []*UserWorkspaceRole) bool {
	for _, r := range roles {
		if r.UserID == u.ID && r.WorkspaceID == workspaceID && r.Role == role && r.IsActive() {
			return true
		}
	}
	return false
}

func (u *User) GetWorkspaceRole(workspaceID string, roles []*UserWorkspaceRole) *WorkspaceRole {
	for _, r := range roles {
		if r.UserID == u.ID && r.WorkspaceID == workspaceID && r.IsActive() {
			return &r.Role
		}
	}
	return nil
}

func (u *User) GetWorkspaces(roles []*UserWorkspaceRole) []string {
	workspaceIDs := []string{}
	for _, r := range roles {
		if r.UserID == u.ID && r.IsActive() {
			workspaceIDs = append(workspaceIDs, r.WorkspaceID)
		}
	}
	return workspaceIDs
}

// CanAccessWorkspace returns true if user has a role in the workspace OR is an instance admin
func (u *User) CanAccessWorkspace(workspaceID string, roles []*UserWorkspaceRole) bool {
	for _, r := range roles {
		if r.UserID == u.ID && r.WorkspaceID == workspaceID && r.IsActive() {
			return true
		}
	}
	// Instance admins can access any workspace
	return u.IsInstanceAdmin()
}
