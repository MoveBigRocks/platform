package platformdomain

import (
	"time"
)

// Permission constants
// Format: resource:action
const (
	// Case permissions
	PermissionCaseRead  = "case:read"
	PermissionCaseWrite = "case:write"

	// Contact permissions (customer PII)
	PermissionContactRead  = "contact:read"
	PermissionContactWrite = "contact:write"

	// Communication permissions
	PermissionCommunicationRead  = "communication:read"
	PermissionCommunicationWrite = "communication:write"

	// Queue permissions
	PermissionQueueRead  = "queue:read"
	PermissionQueueWrite = "queue:write"

	// Conversation permissions
	PermissionConversationRead  = "conversation:read"
	PermissionConversationWrite = "conversation:write"

	// Knowledge permissions
	PermissionKnowledgeRead  = "knowledge:read"
	PermissionKnowledgeWrite = "knowledge:write"

	// Service catalog permissions
	PermissionCatalogRead  = "catalog:read"
	PermissionCatalogWrite = "catalog:write"

	// Form permissions
	PermissionFormRead  = "form:read"
	PermissionFormWrite = "form:write"

	// Attachment permissions
	PermissionAttachmentRead  = "attachment:read"
	PermissionAttachmentWrite = "attachment:write"

	// Issue permissions
	PermissionIssueRead  = "issue:read"
	PermissionIssueWrite = "issue:write"

	// Error event permissions
	PermissionErrorEventRead = "error_event:read"

	// Application permissions
	PermissionApplicationRead  = "application:read"
	PermissionApplicationWrite = "application:write"

	// Extension permissions
	PermissionExtensionRead  = "extension:read"
	PermissionExtensionWrite = "extension:write"

	// Git repo permissions (read = can access the connection details)
	PermissionGitRepoRead = "git_repo:read"
)

// MembershipConstraints restrict what a principal can access within a workspace
type MembershipConstraints struct {
	// Rate limiting
	RateLimitPerMinute *int
	RateLimitPerHour   *int

	// IP restrictions
	AllowedIPs []string

	// Scope restrictions - empty means no restriction
	AllowedProjectIDs []string
	AllowedTeamIDs    []string

	// Delegated routing controls for agent-driven work handoff/escalation.
	AllowDelegatedRouting   bool
	DelegatedRoutingTeamIDs []string

	// Time-based restrictions
	ActiveHoursStart *string // "09:00"
	ActiveHoursEnd   *string // "17:00"
	ActiveTimezone   *string // "America/New_York"
	ActiveDays       []int   // 1=Monday, 7=Sunday
}

// WorkspaceMembership grants a principal (user or agent) access to a workspace
type WorkspaceMembership struct {
	ID          string
	WorkspaceID string

	// The principal - polymorphic
	PrincipalID   string
	PrincipalType PrincipalType

	// Role determines base permissions (for display/grouping)
	Role string

	// Permissions - array of resource:action strings
	Permissions []string

	// Constraints - restrictions on access
	Constraints MembershipConstraints

	// Lifecycle
	GrantedByID string
	GrantedAt   time.Time
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	RevokedByID *string
}

// IsActive returns true if the membership is currently valid
func (m *WorkspaceMembership) IsActive() bool {
	if m.RevokedAt != nil {
		return false
	}
	if m.ExpiresAt != nil && time.Now().After(*m.ExpiresAt) {
		return false
	}
	return true
}

// HasPermission checks if the membership grants a specific permission
func (m *WorkspaceMembership) HasPermission(permission string) bool {
	for _, p := range m.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// HasResourcePermission checks if the membership grants a permission for a resource
func (m *WorkspaceMembership) HasResourcePermission(resource, action string) bool {
	permission := resource + ":" + action
	return m.HasPermission(permission)
}

// Revoke revokes the membership
func (m *WorkspaceMembership) Revoke(revokedByID string) {
	now := time.Now()
	m.RevokedAt = &now
	m.RevokedByID = &revokedByID
}

// IsRateLimited checks if the membership has rate limiting configured
func (m *WorkspaceMembership) IsRateLimited() bool {
	return m.Constraints.RateLimitPerMinute != nil || m.Constraints.RateLimitPerHour != nil
}

// HasRateLimit is an alias for IsRateLimited
func (m *WorkspaceMembership) HasRateLimit() bool {
	return m.IsRateLimited()
}

// HasTimeRestrictions checks if the membership has time-based restrictions
func (m *WorkspaceMembership) HasTimeRestrictions() bool {
	return m.Constraints.ActiveHoursStart != nil || len(m.Constraints.ActiveDays) > 0
}

// IsIPRestricted checks if the membership has IP restrictions
func (m *WorkspaceMembership) IsIPRestricted() bool {
	return len(m.Constraints.AllowedIPs) > 0
}

// IsProjectRestricted checks if the membership is restricted to specific projects
func (m *WorkspaceMembership) IsProjectRestricted() bool {
	return len(m.Constraints.AllowedProjectIDs) > 0
}

// CanAccessProject checks if the membership can access a specific project
func (m *WorkspaceMembership) CanAccessProject(projectID string) bool {
	if !m.IsProjectRestricted() {
		return true
	}
	for _, id := range m.Constraints.AllowedProjectIDs {
		if id == projectID {
			return true
		}
	}
	return false
}

// AllowsDelegatedRouting reports whether this membership permits an agent to
// route work across queues or teams on behalf of its human owner.
func (m *WorkspaceMembership) AllowsDelegatedRouting() bool {
	if m == nil {
		return false
	}
	return m.Constraints.AllowDelegatedRouting
}

// CanDelegateRoutingToTeam checks whether delegated routing is allowed for the
// provided team scope. An empty delegated-routing team list means any allowed
// team in the workspace scope may be targeted.
func (m *WorkspaceMembership) CanDelegateRoutingToTeam(teamID string) bool {
	if !m.AllowsDelegatedRouting() {
		return false
	}
	if teamID == "" || len(m.Constraints.DelegatedRoutingTeamIDs) == 0 {
		return true
	}
	for _, allowedTeamID := range m.Constraints.DelegatedRoutingTeamIDs {
		if allowedTeamID == teamID {
			return true
		}
	}
	return false
}

// NewWorkspaceMembership creates a new membership for a principal
func NewWorkspaceMembership(
	workspaceID string,
	principalID string,
	principalType PrincipalType,
	role string,
	permissions []string,
	grantedByID string,
) *WorkspaceMembership {
	return &WorkspaceMembership{
		WorkspaceID:   workspaceID,
		PrincipalID:   principalID,
		PrincipalType: principalType,
		Role:          role,
		Permissions:   permissions,
		GrantedByID:   grantedByID,
		GrantedAt:     time.Now(),
	}
}

// NewAgentMembership creates a membership for an agent with typical defaults
func NewAgentMembership(
	workspaceID string,
	agentID string,
	role string,
	permissions []string,
	grantedByID string,
) *WorkspaceMembership {
	return NewWorkspaceMembership(
		workspaceID,
		agentID,
		PrincipalTypeAgent,
		role,
		permissions,
		grantedByID,
	)
}
