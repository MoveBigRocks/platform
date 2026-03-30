package platformdomain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// PrincipalType distinguishes users from agents
type PrincipalType string

const (
	PrincipalTypeUser  PrincipalType = "user"
	PrincipalTypeAgent PrincipalType = "agent"
)

// Principal is anything that can authenticate and act
type Principal interface {
	GetID() string
	GetName() string
	GetPrincipalType() PrincipalType
}

// Ensure User implements Principal
func (u *User) GetID() string                   { return u.ID }
func (u *User) GetName() string                 { return u.Name }
func (u *User) GetPrincipalType() PrincipalType { return PrincipalTypeUser }

// AgentStatus tracks agent lifecycle
type AgentStatus string

const (
	AgentStatusActive    AgentStatus = "active"
	AgentStatusSuspended AgentStatus = "suspended"
	AgentStatusRevoked   AgentStatus = "revoked"
)

// Agent is a non-human principal that can act in a workspace
type Agent struct {
	ID          string
	WorkspaceID string

	// Identity
	Name        string
	Description string

	// Accountability - every agent has an owner
	OwnerID string

	// Lifecycle
	Status       AgentStatus
	StatusReason string

	// Timestamps
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedByID string
	DeletedAt   *time.Time
}

// Ensure Agent implements Principal
func (a *Agent) GetID() string                   { return a.ID }
func (a *Agent) GetName() string                 { return a.Name }
func (a *Agent) GetPrincipalType() PrincipalType { return PrincipalTypeAgent }

// IsActive returns true if the agent can authenticate and act
func (a *Agent) IsActive() bool {
	return a.Status == AgentStatusActive && a.DeletedAt == nil
}

// NewAgent creates a new agent
func NewAgent(workspaceID, name, description, ownerID, createdByID string) *Agent {
	now := time.Now()
	return &Agent{
		WorkspaceID: workspaceID,
		Name:        name,
		Description: description,
		OwnerID:     ownerID,
		Status:      AgentStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedByID: createdByID,
	}
}

// Suspend suspends the agent with a reason
func (a *Agent) Suspend(reason string) {
	a.Status = AgentStatusSuspended
	a.StatusReason = reason
	a.UpdatedAt = time.Now()
}

// Activate activates a suspended agent
func (a *Agent) Activate() {
	a.Status = AgentStatusActive
	a.StatusReason = ""
	a.UpdatedAt = time.Now()
}

// Revoke permanently revokes the agent
func (a *Agent) Revoke(reason string) {
	a.Status = AgentStatusRevoked
	a.StatusReason = reason
	a.UpdatedAt = time.Now()
}

// AgentToken is an authentication credential for an agent
type AgentToken struct {
	ID      string
	AgentID string

	// Token - only the hash is stored
	TokenHash   string
	TokenPrefix string

	// Human-readable name for this token
	Name string

	// Lifecycle
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	RevokedByID *string

	// Usage tracking
	LastUsedAt *time.Time
	LastUsedIP string
	UseCount   int64

	// Audit
	CreatedAt   time.Time
	CreatedByID string
}

// IsValid returns true if the token can be used
func (t *AgentToken) IsValid() bool {
	if t.RevokedAt != nil {
		return false
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return false
	}
	return true
}

// Revoke revokes the token
func (t *AgentToken) Revoke(revokedByID string) {
	now := time.Now()
	t.RevokedAt = &now
	t.RevokedByID = &revokedByID
}

// RecordUsage updates usage tracking
func (t *AgentToken) RecordUsage(ip string) {
	now := time.Now()
	t.LastUsedAt = &now
	t.LastUsedIP = ip
	t.UseCount++
}

// GenerateAgentToken creates a new agent token
// Returns: plaintext token (show once), hash (store), prefix (for identification), error
func GenerateAgentToken() (plaintext, hash, prefix string, err error) {
	// 32 bytes = 256 bits of entropy
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", "", fmt.Errorf("crypto/rand failed: %w", err)
	}

	// Format: hat_{random} (Move Big Rocks agent token)
	plaintext = fmt.Sprintf("hat_%s", hex.EncodeToString(randomBytes))

	// Store SHA-256 hash
	hashBytes := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(hashBytes[:])

	// Prefix for identification (first 16 chars)
	prefix = plaintext[:16]

	return plaintext, hash, prefix, nil
}

// HashAgentToken computes the hash of a plaintext token
func HashAgentToken(plaintext string) string {
	hashBytes := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(hashBytes[:])
}

// NewAgentToken creates a new token for an agent
// Returns the token struct, the plaintext token (only returned once), and any error
func NewAgentToken(agentID, name, createdByID string, expiresAt *time.Time) (*AgentToken, string, error) {
	plaintext, hash, prefix, err := GenerateAgentToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate agent token: %w", err)
	}

	return &AgentToken{
		AgentID:     agentID,
		TokenHash:   hash,
		TokenPrefix: prefix,
		Name:        name,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
		CreatedByID: createdByID,
	}, plaintext, nil
}

// AuthMethod indicates how the principal authenticated
type AuthMethod string

const (
	AuthMethodSession    AuthMethod = "session"
	AuthMethodAgentToken AuthMethod = "agent_token"
	AuthMethodInternal   AuthMethod = "internal" // Service-to-service auth within the platform
)

// AuthContext represents the authenticated principal and their permissions
type AuthContext struct {
	// Who
	Principal     Principal
	PrincipalType PrincipalType

	// Where (workspace context, if applicable)
	WorkspaceID  string
	Workspace    *Workspace
	WorkspaceIDs []string // All authorized workspace IDs for this principal (for multi-workspace OAuth tokens)

	// What they can do
	Membership  *WorkspaceMembership
	Permissions []string

	// Instance-level access (users only)
	InstanceRole *InstanceRole

	// How they authenticated
	AuthMethod AuthMethod

	// Request context
	RequestID string
	IPAddress string
	UserAgent string
}

// IsHuman returns true if the principal is a human
func (ac *AuthContext) IsHuman() bool {
	return ac.PrincipalType == PrincipalTypeUser
}

// IsAgent returns true if the principal is an agent
func (ac *AuthContext) IsAgent() bool {
	return ac.PrincipalType == PrincipalTypeAgent
}

// WorkspaceIDSet returns the full set of workspace IDs on the auth context.
func (ac *AuthContext) WorkspaceIDSet() []string {
	if ac == nil {
		return nil
	}

	workspaceSet := map[string]struct{}{}
	if ac.WorkspaceID != "" {
		workspaceSet[ac.WorkspaceID] = struct{}{}
	}
	for _, workspaceID := range ac.WorkspaceIDs {
		if workspaceID != "" {
			workspaceSet[workspaceID] = struct{}{}
		}
	}

	ids := make([]string, 0, len(workspaceSet))
	for id := range workspaceSet {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// HasWorkspaceAccess returns true if the principal is allowed into the workspace.
// Instance admins are allowed into all workspaces.
func (ac *AuthContext) HasWorkspaceAccess(workspaceID string) bool {
	if ac == nil {
		return false
	}
	if ac.IsInstanceAdmin() {
		return true
	}
	if workspaceID == "" {
		return false
	}
	for _, allowedWorkspaceID := range ac.WorkspaceIDSet() {
		if allowedWorkspaceID == workspaceID {
			return true
		}
	}
	return false
}

// CanAccessTeam returns true when the principal can access the team context.
// Empty team scopes mean unrestricted team access within the workspace.
func (ac *AuthContext) CanAccessTeam(teamID string) bool {
	if ac == nil {
		return false
	}
	if ac.IsInstanceAdmin() {
		return true
	}
	if strings.TrimSpace(teamID) == "" {
		return false
	}
	if ac.Membership == nil || len(ac.Membership.Constraints.AllowedTeamIDs) == 0 {
		return true
	}
	for _, allowedTeamID := range ac.Membership.Constraints.AllowedTeamIDs {
		if allowedTeamID == teamID {
			return true
		}
	}
	return false
}

// AllowsDelegatedRouting reports whether this principal may hand off or
// escalate work through the delegated agent-routing path.
func (ac *AuthContext) AllowsDelegatedRouting() bool {
	if ac == nil {
		return false
	}
	if !ac.IsAgent() {
		return true
	}
	if ac.Membership == nil {
		return false
	}
	return ac.Membership.AllowsDelegatedRouting()
}

// CanDelegateRoutingToTeam checks whether delegated routing is allowed for the
// requested target team after workspace team-scope restrictions are applied.
func (ac *AuthContext) CanDelegateRoutingToTeam(teamID string) bool {
	if ac == nil {
		return false
	}
	if !ac.IsAgent() {
		return ac.CanAccessTeam(teamID)
	}
	if strings.TrimSpace(teamID) != "" && !ac.CanAccessTeam(teamID) {
		return false
	}
	if ac.Membership == nil {
		return false
	}
	return ac.Membership.CanDelegateRoutingToTeam(strings.TrimSpace(teamID))
}

// IsInstanceAdmin returns true if the principal has instance-level admin access
func (ac *AuthContext) IsInstanceAdmin() bool {
	return ac.IsHuman() && ac.InstanceRole != nil && ac.InstanceRole.IsOperator()
}

// CanManageUsers returns true when the principal can create/update/delete users.
func (ac *AuthContext) CanManageUsers() bool {
	return ac.IsHuman() && ac.InstanceRole != nil && ac.InstanceRole.IsAdmin()
}

// CanAccessInstancePanel returns true when principal can enter instance admin panel.
func (ac *AuthContext) CanAccessInstancePanel() bool {
	return ac.IsInstanceAdmin()
}

// HasPermission checks if the principal has a specific permission
// Permission format: "resource:action" e.g., "case:read", "issue:write"
func (ac *AuthContext) HasPermission(permission string) bool {
	for _, p := range ac.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// HasResourcePermission checks if the principal has a permission for a resource
func (ac *AuthContext) HasResourcePermission(resource, action string) bool {
	return ac.HasPermission(fmt.Sprintf("%s:%s", resource, action))
}

// GetAgent returns the agent if the principal is an agent, nil otherwise
func (ac *AuthContext) GetAgent() *Agent {
	if ac.IsAgent() {
		if agent, ok := ac.Principal.(*Agent); ok {
			return agent
		}
	}
	return nil
}

// GetHuman returns the user if the principal is a human, nil otherwise
func (ac *AuthContext) GetHuman() *User {
	if ac.IsHuman() {
		if user, ok := ac.Principal.(*User); ok {
			return user
		}
	}
	return nil
}
