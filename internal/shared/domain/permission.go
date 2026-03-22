package shareddomain

import (
	"encoding/json"
	"time"
)

// Role represents a role in the RBAC system
type Role struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	IsSystem    bool // System roles cannot be deleted
	IsDefault   bool // Automatically assigned to new users
	Priority    int  // Higher priority roles override lower ones

	// Permissions
	Permissions []Permission

	// Inheritance
	ParentRoleID string
	ParentRole   *Role

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
}

// Permission represents a specific permission
type Permission struct {
	ID          string
	Resource    string          // e.g., "case", "user", "workspace"
	Action      string          // e.g., "create", "read", "update", "delete"
	Scope       string          // e.g., "own", "team", "workspace", "all"
	Constraints json.RawMessage // Additional constraints as JSON
}

// PermissionSet represents a collection of permissions
type PermissionSet struct {
	ID          string
	Name        string
	Description string
	Permissions []Permission
	IsSystem    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UserRole represents the assignment of a role to a user
type UserRole struct {
	ID          string
	UserID      string
	RoleID      string
	WorkspaceID string
	TeamID      string // Optional team-specific role

	// Role details (populated when fetched)
	Role *Role

	// Time-based restrictions
	ValidFrom  *time.Time
	ValidUntil *time.Time

	// Assignment details
	AssignedBy string
	AssignedAt time.Time
	Reason     string

	// Status
	IsActive  bool
	RevokedAt *time.Time
	RevokedBy string
}

// ResourcePermission represents permissions for a specific resource instance
type ResourcePermission struct {
	ID           string
	ResourceType string // e.g., "case", "knowledge_resource"
	ResourceID   string // Specific resource instance ID
	UserID       string
	TeamID       string
	RoleID       string

	// Permissions
	Permissions []string // e.g., ["read", "update"]

	// Grant details
	GrantedBy string
	GrantedAt time.Time
	ExpiresAt *time.Time
	Reason    string

	// Inheritance
	IsInherited   bool
	InheritedFrom string // Parent resource ID
}

// PermissionCheck represents a permission check request
type PermissionCheck struct {
	UserID      string
	Resource    string
	ResourceID  string
	Action      string
	WorkspaceID string
	TeamID      string
	Context     Metadata
}

// PermissionResult represents the result of a permission check
type PermissionResult struct {
	Allowed      bool
	Reason       string
	MatchedRoles []string
	MatchedPerms []string
	Constraints  Metadata
}

// PolicyRule represents a policy-based access control rule
type PolicyRule struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string

	// Rule definition
	Resource string
	Action   string
	Effect   PolicyEffect // Allow or Deny
	Priority int

	// Conditions
	Conditions []PolicyCondition

	// Target
	TargetUsers []string
	TargetTeams []string
	TargetRoles []string

	// Status
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
}

// PolicyEffect represents the effect of a policy rule
type PolicyEffect string

const (
	PolicyEffectAllow PolicyEffect = "allow"
	PolicyEffectDeny  PolicyEffect = "deny"
)

// PolicyCondition represents a condition for a policy rule
type PolicyCondition struct {
	Type     string // e.g., "time", "ip", "attribute"
	Operator string // e.g., "equals", "contains", "between"
	Value    Value
	Field    string
}

// AccessToken represents an API access token with specific permissions
type AccessToken struct {
	ID          string
	UserID      string
	WorkspaceID string
	Name        string
	Token       string // Only returned on creation
	TokenHash   string // Stored hash of the token

	// Permissions
	Scopes      []string     // API scopes
	Permissions []Permission // Specific permissions

	// Restrictions
	IPWhitelist []string
	Origins     []string
	RateLimit   int // Requests per minute

	// Validity
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	LastUsedIP string
	UsageCount int

	// Status
	IsActive     bool
	RevokedAt    *time.Time
	RevokedBy    string
	RevokeReason string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// PermissionTemplate represents a template for common permission sets
type PermissionTemplate struct {
	ID          string
	Name        string
	Description string
	Category    string // e.g., "agent", "admin", "viewer"

	// Template permissions
	Permissions []Permission

	// Usage
	IsDefault bool
	IsSystem  bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// RoleHierarchy represents the hierarchy of roles
type RoleHierarchy struct {
	RoleID   string
	ParentID string
	Children []string
	Level    int
	Path     string // e.g., "/owner/manager/agent"
}

// PermissionAuditLog represents an audit log entry for permission changes
type PermissionAuditLog struct {
	ID          string
	WorkspaceID string
	UserID      string
	TargetID    string // User/Role/Resource being modified
	TargetType  string // "user", "role", "resource"
	Action      string // "grant", "revoke", "modify"

	// Changes
	OldPermissions []Permission
	NewPermissions []Permission
	ChangeSummary  string

	// Context
	Reason    string
	IPAddress string
	UserAgent string
	Metadata  Metadata

	CreatedAt time.Time
}

// Common system roles
const (
	RoleSystemAdmin    = "system_admin"
	RoleWorkspaceAdmin = "workspace_admin"
	RoleTeamLead       = "team_lead"
	RoleAgent          = "agent"
	RoleViewer         = "viewer"
	RoleCustomer       = "customer"
)

// Common resources
const (
	ResourceWorkspace   = "workspace"
	ResourceTeam        = "team"
	ResourceUser        = "user"
	ResourceCase        = "case"
	ResourceContact     = "contact"
	ResourceKnowledge   = "knowledge_resource"
	ResourceReport      = "report"
	ResourceSettings    = "settings"
	ResourceIntegration = "integration"
	ResourceBilling     = "billing"
	ResourceAuditLog    = "audit_log"
)

// Common actions
const (
	ActionCreate  = "create"
	ActionRead    = "read"
	ActionUpdate  = "update"
	ActionDelete  = "delete"
	ActionList    = "list"
	ActionAssign  = "assign"
	ActionExport  = "export"
	ActionImport  = "import"
	ActionExecute = "execute"
	ActionManage  = "manage"
)

// Common scopes
const (
	ScopeOwn       = "own"       // Only own resources
	ScopeTeam      = "team"      // Team resources
	ScopeWorkspace = "workspace" // All workspace resources
	ScopeSystem    = "system"    // System-wide
)
