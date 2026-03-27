// Package resolvers provides GraphQL resolvers for the platform domain.
// This domain owns the User, Agent, Workspace, and WorkspaceMembership API surface.
package resolvers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	"github.com/movebigrocks/platform/internal/graph/model"
	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	serviceresolvers "github.com/movebigrocks/platform/internal/service/resolvers"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
)

// Config holds the dependencies for platform domain resolvers
type Config struct {
	UserService      *platformservices.UserManagementService
	WorkspaceService *platformservices.WorkspaceManagementService
	AgentService     *platformservices.AgentService
	ContactService   *platformservices.ContactService
	ExtensionService *platformservices.ExtensionService
	CaseService      *serviceapp.CaseService
	QueueService     *serviceapp.QueueService
	ServiceGraph     *serviceresolvers.Resolver
}

// Resolver handles all platform domain GraphQL operations
type Resolver struct {
	userService      *platformservices.UserManagementService
	workspaceService *platformservices.WorkspaceManagementService
	agentService     *platformservices.AgentService
	contactService   *platformservices.ContactService
	extensionService *platformservices.ExtensionService
	caseService      *serviceapp.CaseService
	queueService     *serviceapp.QueueService
	serviceGraph     *serviceresolvers.Resolver
}

// NewResolver creates a new platform domain resolver
func NewResolver(cfg Config) *Resolver {
	return &Resolver{
		userService:      cfg.UserService,
		workspaceService: cfg.WorkspaceService,
		agentService:     cfg.AgentService,
		contactService:   cfg.ContactService,
		extensionService: cfg.ExtensionService,
		caseService:      cfg.CaseService,
		queueService:     cfg.QueueService,
		serviceGraph:     cfg.ServiceGraph,
	}
}

// Extension resolves an installed extension by ID.
func (r *Resolver) Extension(ctx context.Context, id string) (*InstalledExtensionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionRead)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	return &InstalledExtensionResolver{extension: extension, r: r}, nil
}

// Extensions resolves all installed extensions for a workspace.
func (r *Resolver) Extensions(ctx context.Context, workspaceID string) ([]*InstalledExtensionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	extensions, err := r.extensionService.ListWorkspaceExtensions(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list extensions: %w", err)
	}
	result := make([]*InstalledExtensionResolver, len(extensions))
	for i, extension := range extensions {
		result[i] = &InstalledExtensionResolver{extension: extension, r: r}
	}
	return result, nil
}

// InstanceExtensions resolves installed instance-scoped extensions.
func (r *Resolver) InstanceExtensions(ctx context.Context) ([]*InstalledExtensionResolver, error) {
	if _, err := graphshared.RequireInstanceAdmin(ctx); err != nil {
		return nil, err
	}
	extensions, err := r.extensionService.ListInstanceExtensions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list instance extensions: %w", err)
	}
	result := make([]*InstalledExtensionResolver, len(extensions))
	for i, extension := range extensions {
		result[i] = &InstalledExtensionResolver{extension: extension, r: r}
	}
	return result, nil
}

// ExtensionEventCatalog resolves the runtime event catalog for a workspace.
func (r *Resolver) ExtensionEventCatalog(ctx context.Context, workspaceID string) ([]*ExtensionRuntimeEventResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	events, err := r.extensionService.ListWorkspaceEventCatalog(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list extension events: %w", err)
	}
	result := make([]*ExtensionRuntimeEventResolver, len(events))
	for i := range events {
		result[i] = &ExtensionRuntimeEventResolver{event: events[i]}
	}
	return result, nil
}

// ExtensionArtifactFiles resolves managed artifact files for an installed extension surface.
func (r *Resolver) ExtensionArtifactFiles(ctx context.Context, id, surface string) ([]*ExtensionArtifactFileResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionRead)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	files, err := r.extensionService.ListExtensionArtifactFiles(ctx, id, surface)
	if err != nil {
		return nil, fmt.Errorf("failed to list extension artifacts: %w", err)
	}
	result := make([]*ExtensionArtifactFileResolver, 0, len(files))
	for _, file := range files {
		if file == nil {
			continue
		}
		result = append(result, &ExtensionArtifactFileResolver{file: *file})
	}
	return result, nil
}

// ExtensionArtifactContent resolves the current or historical content for an installed extension artifact.
func (r *Resolver) ExtensionArtifactContent(ctx context.Context, id, surface, artifactPath string, ref *string) (string, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionRead)
	if err != nil {
		return "", err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return "", fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return "", fmt.Errorf("extension not found")
	}
	return r.extensionService.GetExtensionArtifactContent(ctx, id, surface, artifactPath, derefString(ref))
}

// ExtensionArtifactHistory resolves the revision history for an installed extension artifact.
func (r *Resolver) ExtensionArtifactHistory(ctx context.Context, id, surface, artifactPath string, limit *int32) ([]*ArtifactRevisionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionRead)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	revisions, err := r.extensionService.GetExtensionArtifactHistory(ctx, id, surface, artifactPath, derefInt32OrDefault(limit, 20))
	if err != nil {
		return nil, fmt.Errorf("failed to load extension artifact history: %w", err)
	}
	result := make([]*ArtifactRevisionResolver, 0, len(revisions))
	for _, revision := range revisions {
		result = append(result, &ArtifactRevisionResolver{revision: revision})
	}
	return result, nil
}

// ExtensionArtifactDiff resolves a patch between two extension artifact revisions.
func (r *Resolver) ExtensionArtifactDiff(ctx context.Context, id, surface, artifactPath string, fromRevision, toRevision *string) (*ArtifactDiffResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionRead)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	diff, err := r.extensionService.GetExtensionArtifactDiff(ctx, id, surface, artifactPath, derefString(fromRevision), derefString(toRevision))
	if err != nil {
		return nil, fmt.Errorf("failed to load extension artifact diff: %w", err)
	}
	return &ArtifactDiffResolver{diff: diff}, nil
}

// =============================================================================
// Query Resolvers
// =============================================================================

// Workspace resolves a workspace by ID
func (r *Resolver) Workspace(ctx context.Context, id string) (*WorkspaceResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Validate the requested workspace matches auth context (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(id, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	workspace, err := r.workspaceService.GetWorkspace(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	return &WorkspaceResolver{workspace: workspace, r: r}, nil
}

// Workspaces returns all workspaces accessible to the current user
func (r *Resolver) Workspaces(ctx context.Context) ([]*WorkspaceResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Instance admins can see all workspaces
	if authCtx.IsInstanceAdmin() {
		workspaces, err := r.workspaceService.ListWorkspaces(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list workspaces: %w", err)
		}
		resolvers := make([]*WorkspaceResolver, len(workspaces))
		for i, ws := range workspaces {
			resolvers[i] = &WorkspaceResolver{workspace: ws, r: r}
		}
		return resolvers, nil
	}

	// Regular users only see their workspaces
	if authCtx.IsHuman() {
		user := authCtx.Principal.(*platformdomain.User)
		workspaceInfos, err := r.userService.GetUserWorkspaces(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user workspaces: %w", err)
		}
		allowedWorkspaceIDs := make(map[string]struct{}, len(authCtx.WorkspaceIDSet()))
		for _, workspaceID := range authCtx.WorkspaceIDSet() {
			allowedWorkspaceIDs[workspaceID] = struct{}{}
		}

		// Keep only workspaces in the principal's allowed list (for OAuth scope filtering)
		filtered := make([]*platformdomain.Workspace, 0, len(workspaceInfos))
		for _, info := range workspaceInfos {
			if _, ok := allowedWorkspaceIDs[info.Workspace.ID]; !ok {
				continue
			}
			filtered = append(filtered, info.Workspace)
		}

		resolvers := make([]*WorkspaceResolver, len(filtered))
		for i, workspace := range filtered {
			resolvers[i] = &WorkspaceResolver{workspace: workspace, r: r}
		}
		return resolvers, nil
	}

	// Agents only have access to their assigned workspace
	if authCtx.Workspace != nil {
		return []*WorkspaceResolver{{workspace: authCtx.Workspace, r: r}}, nil
	}

	return []*WorkspaceResolver{}, nil
}

// Team resolves a team by ID.
func (r *Resolver) Team(ctx context.Context, id string) (*TeamResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	team, err := r.workspaceService.GetTeam(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("team not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(team.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("team not found")
	}
	return &TeamResolver{team: team, r: r}, nil
}

// Teams resolves all teams for a workspace.
func (r *Resolver) Teams(ctx context.Context, workspaceID string) ([]*TeamResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	teams, err := r.workspaceService.ListWorkspaceTeams(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	result := make([]*TeamResolver, len(teams))
	for i, team := range teams {
		result[i] = &TeamResolver{team: team, r: r}
	}
	return result, nil
}

// Agent resolves an agent by ID
func (r *Resolver) Agent(ctx context.Context, id string) (*AgentResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	agent, err := r.agentService.GetAgent(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	// Layer 2 defense: Validate workspace ownership (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(agent.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	return &AgentResolver{agent: agent, r: r}, nil
}

// Agents resolves all agents for a workspace
func (r *Resolver) Agents(ctx context.Context, workspaceID string) ([]*AgentResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Validate the requested workspace matches auth context (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	agents, err := r.agentService.ListAgents(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	result := make([]*AgentResolver, len(agents))
	for i, agent := range agents {
		result[i] = &AgentResolver{agent: agent, r: r}
	}
	return result, nil
}

// Contacts resolves all contacts for a workspace.
func (r *Resolver) Contacts(ctx context.Context, workspaceID string) ([]*model.Contact, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionContactRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	if r.contactService == nil {
		return []*model.Contact{}, nil
	}

	contacts, err := r.contactService.ListWorkspaceContacts(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list contacts: %w", err)
	}

	result := make([]*model.Contact, len(contacts))
	for i, contact := range contacts {
		result[i] = contactToModel(contact)
	}
	return result, nil
}

// =============================================================================
// Admin Query Resolvers
// =============================================================================

// AdminUsers resolves admin users with filtering
func (r *Resolver) AdminUsers(ctx context.Context, filter *model.AdminUserFilterInput) (*AdminUserConnectionResolver, error) {
	_, err := graphshared.RequireInstanceUserManager(ctx)
	if err != nil {
		return nil, err
	}

	users, err := r.userService.ListAllUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return &AdminUserConnectionResolver{users: users, r: r}, nil
}

// AdminUser resolves a single admin user by ID
func (r *Resolver) AdminUser(ctx context.Context, id string) (*AdminUserResolver, error) {
	_, err := graphshared.RequireInstanceUserManager(ctx)
	if err != nil {
		return nil, err
	}

	user, err := r.userService.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get workspace count
	workspaces, err := r.userService.GetUserWorkspaces(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user workspaces: %w", err)
	}
	workspaceCount := len(workspaces)

	return &AdminUserResolver{user: user, workspaceCount: workspaceCount, r: r}, nil
}

// AdminUserWithWorkspaces resolves a user with their workspace memberships
func (r *Resolver) AdminUserWithWorkspaces(ctx context.Context, id string) (*AdminUserWithWorkspacesResolver, error) {
	_, err := graphshared.RequireInstanceUserManager(ctx)
	if err != nil {
		return nil, err
	}

	userWithWorkspaces, err := r.userService.GetUserWithWorkspaces(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user with workspaces: %w", err)
	}

	return &AdminUserWithWorkspacesResolver{data: userWithWorkspaces, r: r}, nil
}

// =============================================================================
// Mutation Resolvers - Agent Management
// =============================================================================

// CreateAgent creates a new agent
func (r *Resolver) CreateAgent(ctx context.Context, input model.CreateAgentInput) (*AgentResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Only humans can create agents
	if authCtx.IsAgent() {
		return nil, fmt.Errorf("agents cannot create other agents")
	}

	// Instance admins can create agents in any workspace
	// Regular users must validate workspace ownership (ADR-0003)
	if !authCtx.IsInstanceAdmin() {
		if err := graphshared.ValidateWorkspaceOwnership(input.WorkspaceID, authCtx); err != nil {
			return nil, fmt.Errorf("workspace not found")
		}
	}

	description := ""
	if input.Description != nil {
		description = *input.Description
	}

	agent := platformdomain.NewAgent(
		input.WorkspaceID,
		input.Name,
		description,
		authCtx.Principal.GetID(),
		authCtx.Principal.GetID(),
	)

	if err := r.agentService.CreateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return &AgentResolver{agent: agent, r: r}, nil
}

// UpdateAgent updates an existing agent
func (r *Resolver) UpdateAgent(ctx context.Context, id string, input model.UpdateAgentInput) (*AgentResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if authCtx.IsAgent() {
		return nil, fmt.Errorf("agents cannot update other agents")
	}

	agent, err := r.agentService.GetAgent(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	// Layer 2 defense: Validate workspace ownership before modification (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(agent.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	if input.Name != nil {
		agent.Name = *input.Name
	}
	if input.Description != nil {
		agent.Description = *input.Description
	}
	agent.UpdatedAt = time.Now()

	if err := r.agentService.UpdateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	return &AgentResolver{agent: agent, r: r}, nil
}

// SuspendAgent suspends an agent
func (r *Resolver) SuspendAgent(ctx context.Context, id, reason string) (*AgentResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if authCtx.IsAgent() {
		return nil, fmt.Errorf("agents cannot suspend other agents")
	}

	agent, err := r.agentService.GetAgent(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	// Layer 2 defense: Validate workspace ownership before modification (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(agent.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	agent.Suspend(reason)

	if err := r.agentService.UpdateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to suspend agent: %w", err)
	}

	return &AgentResolver{agent: agent, r: r}, nil
}

// ActivateAgent activates a suspended agent
func (r *Resolver) ActivateAgent(ctx context.Context, id string) (*AgentResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if authCtx.IsAgent() {
		return nil, fmt.Errorf("agents cannot activate other agents")
	}

	agent, err := r.agentService.GetAgent(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	// Layer 2 defense: Validate workspace ownership before modification (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(agent.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	agent.Activate()

	if err := r.agentService.UpdateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to activate agent: %w", err)
	}

	return &AgentResolver{agent: agent, r: r}, nil
}

// RevokeAgent permanently revokes an agent
func (r *Resolver) RevokeAgent(ctx context.Context, id, reason string) (*AgentResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if authCtx.IsAgent() {
		return nil, fmt.Errorf("agents cannot revoke other agents")
	}

	agent, err := r.agentService.GetAgent(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	// Layer 2 defense: Validate workspace ownership before modification (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(agent.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	agent.Revoke(reason)

	if err := r.agentService.UpdateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to revoke agent: %w", err)
	}

	return &AgentResolver{agent: agent, r: r}, nil
}

// =============================================================================
// Mutation Resolvers - Agent Token Management
// =============================================================================

// CreateAgentToken creates a new token for an agent
func (r *Resolver) CreateAgentToken(ctx context.Context, input model.CreateAgentTokenInput) (*AgentTokenResultResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if authCtx.IsAgent() {
		return nil, fmt.Errorf("agents cannot create tokens")
	}

	// Validate the agent belongs to the user's workspace (ADR-0003)
	// Instance admins can create tokens for any agent
	agent, err := r.agentService.GetAgent(ctx, input.AgentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found")
	}
	if !authCtx.IsInstanceAdmin() {
		if err := graphshared.ValidateWorkspaceOwnership(agent.WorkspaceID, authCtx); err != nil {
			return nil, fmt.Errorf("agent not found")
		}
	}

	// Calculate expiration
	var expiresAt *time.Time
	if input.ExpiresInDays != nil {
		exp := time.Now().AddDate(0, 0, int(*input.ExpiresInDays))
		expiresAt = &exp
	}

	token, plaintext, err := platformdomain.NewAgentToken(
		input.AgentID,
		input.Name,
		authCtx.Principal.GetID(),
		expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate agent token: %w", err)
	}

	if err := r.agentService.CreateAgentToken(ctx, token); err != nil {
		return nil, fmt.Errorf("failed to create agent token: %w", err)
	}

	return &AgentTokenResultResolver{token: token, plaintext: plaintext, r: r}, nil
}

// RevokeAgentToken revokes an agent token
func (r *Resolver) RevokeAgentToken(ctx context.Context, id string) (*AgentTokenResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if authCtx.IsAgent() {
		return nil, fmt.Errorf("agents cannot revoke tokens")
	}

	// Fetch token first to get agent ID
	token, err := r.agentService.GetAgentToken(ctx, id)
	if err != nil || token == nil {
		return nil, fmt.Errorf("token not found")
	}

	// Fetch agent to validate workspace ownership (ADR-0003)
	// Instance admins can revoke tokens for any agent
	agent, err := r.agentService.GetAgent(ctx, token.AgentID)
	if err != nil {
		return nil, fmt.Errorf("token not found")
	}
	if !authCtx.IsInstanceAdmin() {
		if err := graphshared.ValidateWorkspaceOwnership(agent.WorkspaceID, authCtx); err != nil {
			return nil, fmt.Errorf("token not found") // Same error to prevent enumeration
		}
	}

	if err := r.agentService.RevokeAgentToken(ctx, id, authCtx.Principal.GetID()); err != nil {
		return nil, fmt.Errorf("failed to revoke agent token: %w", err)
	}

	// Return the updated token
	token.Revoke(authCtx.Principal.GetID())
	return &AgentTokenResolver{token: token, r: r}, nil
}

// =============================================================================
// Mutation Resolvers - Workspace Membership
// =============================================================================

// GrantAgentMembership grants workspace membership to an agent
func (r *Resolver) GrantAgentMembership(ctx context.Context, input model.GrantMembershipInput) (*WorkspaceMembershipResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if authCtx.IsAgent() {
		return nil, fmt.Errorf("agents cannot grant memberships")
	}

	// Validate the user can grant memberships in the specified workspace (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(input.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	// Validate the agent exists and belongs to the same workspace
	agent, err := r.agentService.GetAgent(ctx, input.AgentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(agent.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	membership := platformdomain.NewAgentMembership(
		input.WorkspaceID,
		input.AgentID,
		input.Role,
		input.Permissions,
		authCtx.Principal.GetID(),
	)
	membership.Constraints = membershipConstraintsFromInput(input.Constraints)

	// Set expiration if provided
	if input.ExpiresInDays != nil {
		exp := time.Now().AddDate(0, 0, int(*input.ExpiresInDays))
		membership.ExpiresAt = &exp
	}

	if err := r.agentService.GrantWorkspaceMembership(ctx, membership); err != nil {
		return nil, fmt.Errorf("failed to grant membership: %w", err)
	}

	return &WorkspaceMembershipResolver{membership: membership, r: r}, nil
}

// RevokeAgentMembership revokes an agent's workspace membership
func (r *Resolver) RevokeAgentMembership(ctx context.Context, id string) (*WorkspaceMembershipResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if authCtx.IsAgent() {
		return nil, fmt.Errorf("agents cannot revoke memberships")
	}

	// Fetch membership first to validate workspace ownership (ADR-0003)
	membership, err := r.agentService.GetWorkspaceMembershipByID(ctx, id)
	if err != nil || membership == nil {
		return nil, fmt.Errorf("membership not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(membership.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("membership not found") // Same error to prevent enumeration
	}

	if err := r.agentService.RevokeWorkspaceMembership(ctx, id, authCtx.Principal.GetID()); err != nil {
		return nil, fmt.Errorf("failed to revoke membership: %w", err)
	}

	// Return the revoked membership
	now := time.Now()
	membership.RevokedAt = &now
	revokedByID := authCtx.Principal.GetID()
	membership.RevokedByID = &revokedByID
	return &WorkspaceMembershipResolver{membership: membership, r: r}, nil
}

// CreateTeam creates a team within a workspace.
func (r *Resolver) CreateTeam(ctx context.Context, input model.CreateTeamInput) (*TeamResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	if authCtx.IsAgent() {
		return nil, fmt.Errorf("agents cannot create teams")
	}
	if err := graphshared.ValidateWorkspaceOwnership(input.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("team name is required")
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}
	autoAssign := false
	if input.AutoAssign != nil {
		autoAssign = *input.AutoAssign
	}
	responseTimeHours := 0
	if input.ResponseTimeHours != nil {
		responseTimeHours = int(*input.ResponseTimeHours)
	}
	resolutionTimeHours := 0
	if input.ResolutionTimeHours != nil {
		resolutionTimeHours = int(*input.ResolutionTimeHours)
	}

	team := &platformdomain.Team{
		WorkspaceID:         input.WorkspaceID,
		Name:                name,
		Description:         strings.TrimSpace(derefString(input.Description)),
		EmailAddress:        strings.TrimSpace(derefString(input.EmailAddress)),
		ResponseTimeHours:   responseTimeHours,
		ResolutionTimeHours: resolutionTimeHours,
		AutoAssign:          autoAssign,
		AutoAssignKeywords:  derefStringSlice(input.AutoAssignKeywords),
		IsActive:            isActive,
	}
	if err := r.workspaceService.CreateTeam(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}
	return &TeamResolver{team: team, r: r}, nil
}

// AddTeamMember adds a user to a team.
func (r *Resolver) AddTeamMember(ctx context.Context, input model.AddTeamMemberInput) (*TeamMemberResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	if authCtx.IsAgent() {
		return nil, fmt.Errorf("agents cannot modify team membership")
	}

	team, err := r.workspaceService.GetTeam(ctx, input.TeamID)
	if err != nil {
		return nil, fmt.Errorf("team not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(team.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("team not found")
	}

	role, err := normalizeTeamMemberRole(input.Role)
	if err != nil {
		return nil, err
	}

	member := &platformdomain.TeamMember{
		TeamID:      input.TeamID,
		UserID:      input.UserID,
		WorkspaceID: team.WorkspaceID,
		Role:        role,
	}
	if err := r.workspaceService.AddTeamMember(ctx, member); err != nil {
		return nil, fmt.Errorf("failed to add team member: %w", err)
	}
	return &TeamMemberResolver{member: member, r: r}, nil
}

// InstallExtension installs a new extension bundle into a workspace.
func (r *Resolver) InstallExtension(ctx context.Context, input model.InstallExtensionInput) (*InstalledExtensionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionWrite)
	if err != nil {
		return nil, err
	}

	var manifest platformdomain.ExtensionManifest
	manifestBytes, err := json.Marshal(input.Manifest.ToMap())
	if err != nil {
		return nil, fmt.Errorf("invalid manifest")
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest")
	}

	assets := make([]platformservices.ExtensionAssetInput, 0, len(input.Assets))
	for _, asset := range input.Assets {
		isCustomizable := false
		if asset.IsCustomizable != nil {
			isCustomizable = *asset.IsCustomizable
		}
		contentType := ""
		if asset.ContentType != nil {
			contentType = *asset.ContentType
		}
		assets = append(assets, platformservices.ExtensionAssetInput{
			Path:           asset.Path,
			ContentType:    contentType,
			Content:        []byte(asset.Content),
			IsCustomizable: isCustomizable,
		})
	}
	migrations := make([]platformservices.ExtensionMigrationInput, 0, len(input.Migrations))
	for _, migration := range input.Migrations {
		migrations = append(migrations, platformservices.ExtensionMigrationInput{
			Path:    migration.Path,
			Content: []byte(migration.Content),
		})
	}

	installedByID := ""
	if authCtx.Principal != nil {
		installedByID = authCtx.Principal.GetID()
	}

	workspaceID := strings.TrimSpace(derefString(input.WorkspaceID))
	switch manifest.Scope {
	case platformdomain.ExtensionScopeWorkspace:
		if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
			return nil, fmt.Errorf("workspace not found")
		}
	case platformdomain.ExtensionScopeInstance:
		if _, err := graphshared.RequireInstanceAdmin(ctx); err != nil {
			return nil, err
		}
	}

	extension, err := r.extensionService.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:   workspaceID,
		InstalledByID: installedByID,
		LicenseToken:  derefString(input.LicenseToken),
		BundleBase64:  derefString(input.BundleBase64),
		Manifest:      manifest,
		Assets:        assets,
		Migrations:    migrations,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to install extension: %w", err)
	}
	return &InstalledExtensionResolver{extension: extension, r: r}, nil
}

// UpgradeExtension upgrades an installed extension bundle in place.
func (r *Resolver) UpgradeExtension(ctx context.Context, id string, input model.UpgradeExtensionInput) (*InstalledExtensionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionWrite)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}

	var manifest platformdomain.ExtensionManifest
	manifestBytes, err := json.Marshal(input.Manifest.ToMap())
	if err != nil {
		return nil, fmt.Errorf("invalid manifest")
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest")
	}

	assets := make([]platformservices.ExtensionAssetInput, 0, len(input.Assets))
	for _, asset := range input.Assets {
		isCustomizable := false
		if asset.IsCustomizable != nil {
			isCustomizable = *asset.IsCustomizable
		}
		contentType := ""
		if asset.ContentType != nil {
			contentType = *asset.ContentType
		}
		assets = append(assets, platformservices.ExtensionAssetInput{
			Path:           asset.Path,
			ContentType:    contentType,
			Content:        []byte(asset.Content),
			IsCustomizable: isCustomizable,
		})
	}
	migrations := make([]platformservices.ExtensionMigrationInput, 0, len(input.Migrations))
	for _, migration := range input.Migrations {
		migrations = append(migrations, platformservices.ExtensionMigrationInput{
			Path:    migration.Path,
			Content: []byte(migration.Content),
		})
	}

	installedByID := ""
	if authCtx.Principal != nil {
		installedByID = authCtx.Principal.GetID()
	}

	upgraded, err := r.extensionService.UpgradeExtension(ctx, platformservices.UpgradeExtensionParams{
		ExtensionID:   id,
		InstalledByID: installedByID,
		LicenseToken:  derefString(input.LicenseToken),
		BundleBase64:  derefString(input.BundleBase64),
		Manifest:      manifest,
		Assets:        assets,
		Migrations:    migrations,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade extension: %w", err)
	}
	return &InstalledExtensionResolver{extension: upgraded, r: r}, nil
}

// ActivateExtension activates an installed extension for a workspace.
func (r *Resolver) ActivateExtension(ctx context.Context, id string) (*InstalledExtensionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionWrite)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	extension, err = r.extensionService.ActivateExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to activate extension: %w", err)
	}
	return &InstalledExtensionResolver{extension: extension, r: r}, nil
}

// DeactivateExtension deactivates an installed extension.
func (r *Resolver) DeactivateExtension(ctx context.Context, id string, reason *string) (*InstalledExtensionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionWrite)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	extension, err = r.extensionService.DeactivateExtension(ctx, id, derefString(reason))
	if err != nil {
		return nil, fmt.Errorf("failed to deactivate extension: %w", err)
	}
	return &InstalledExtensionResolver{extension: extension, r: r}, nil
}

// UninstallExtension removes a deactivated extension installation.
func (r *Resolver) UninstallExtension(ctx context.Context, id string) (bool, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionWrite)
	if err != nil {
		return false, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return false, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return false, fmt.Errorf("extension not found")
	}
	if err := r.extensionService.UninstallExtension(ctx, id); err != nil {
		return false, fmt.Errorf("failed to uninstall extension: %w", err)
	}
	return true, nil
}

// ValidateExtension validates an installed extension's manifest and assets.
func (r *Resolver) ValidateExtension(ctx context.Context, id string) (*InstalledExtensionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionWrite)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	extension, err = r.extensionService.ValidateExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to validate extension: %w", err)
	}
	return &InstalledExtensionResolver{extension: extension, r: r}, nil
}

// CheckExtensionHealth refreshes and persists runtime health for an installed extension.
func (r *Resolver) CheckExtensionHealth(ctx context.Context, id string) (*InstalledExtensionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionWrite)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	extension, err = r.extensionService.CheckExtensionHealth(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to check extension health: %w", err)
	}
	return &InstalledExtensionResolver{extension: extension, r: r}, nil
}

// UpdateExtensionConfig updates extension configuration values.
func (r *Resolver) UpdateExtensionConfig(ctx context.Context, id string, input model.UpdateExtensionConfigInput) (*InstalledExtensionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionWrite)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	extension, err = r.extensionService.UpdateExtensionConfig(ctx, id, input.Config.ToMap())
	if err != nil {
		return nil, fmt.Errorf("failed to update extension config: %w", err)
	}
	return &InstalledExtensionResolver{extension: extension, r: r}, nil
}

// UpdateExtensionAsset updates a customizable extension asset.
func (r *Resolver) UpdateExtensionAsset(ctx context.Context, id string, input model.UpdateExtensionAssetInput) (*ExtensionAssetResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionWrite)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	contentType := derefString(input.ContentType)
	asset, err := r.extensionService.UpdateExtensionAsset(ctx, id, input.Path, []byte(input.Content), contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to update extension asset: %w", err)
	}
	return &ExtensionAssetResolver{asset: asset}, nil
}

// PublishExtensionArtifact writes and publishes managed extension artifact content.
func (r *Resolver) PublishExtensionArtifact(ctx context.Context, id string, input model.PublishExtensionArtifactInput) (*ExtensionArtifactPublicationResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionExtensionWrite)
	if err != nil {
		return nil, err
	}
	extension, err := r.extensionService.GetInstalledExtension(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	if err := validateInstalledExtensionAccess(authCtx, extension); err != nil {
		return nil, fmt.Errorf("extension not found")
	}
	publication, err := r.extensionService.PublishExtensionArtifact(ctx, id, input.Surface, input.Path, []byte(input.Content), authActorID(authCtx))
	if err != nil {
		return nil, fmt.Errorf("failed to publish extension artifact: %w", err)
	}
	return &ExtensionArtifactPublicationResolver{publication: publication}, nil
}

// =============================================================================
// Mutation Resolvers - Admin User Management
// =============================================================================

// AdminCreateUser creates a new user (admin only)
func (r *Resolver) AdminCreateUser(ctx context.Context, input model.CreateUserInput) (*AdminUserResolver, error) {
	_, err := graphshared.RequireInstanceUserManager(ctx)
	if err != nil {
		return nil, err
	}

	var instanceRole *platformdomain.InstanceRole
	if input.InstanceRole != nil {
		role := platformdomain.InstanceRole(*input.InstanceRole)
		instanceRole = &role
	}

	user, err := r.userService.CreateUser(ctx, input.Email, input.Name, instanceRole)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &AdminUserResolver{user: user, workspaceCount: 0, r: r}, nil
}

// AdminUpdateUser updates a user (admin only)
func (r *Resolver) AdminUpdateUser(ctx context.Context, id string, input model.UpdateUserInput) (*AdminUserResolver, error) {
	_, err := graphshared.RequireInstanceUserManager(ctx)
	if err != nil {
		return nil, err
	}

	user, err := r.userService.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	email := user.Email
	if input.Email != nil {
		email = *input.Email
	}
	name := user.Name
	if input.Name != nil {
		name = *input.Name
	}
	isActive := user.IsActive
	if input.IsActive != nil {
		isActive = *input.IsActive
	}
	emailVerified := user.EmailVerified
	if input.EmailVerified != nil {
		emailVerified = *input.EmailVerified
	}

	var instanceRole *platformdomain.InstanceRole
	if input.InstanceRole != nil {
		role := platformdomain.InstanceRole(*input.InstanceRole)
		instanceRole = &role
	} else {
		instanceRole = user.InstanceRole
	}

	if err := r.userService.UpdateUser(ctx, id, email, name, instanceRole, isActive, emailVerified); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Get updated user
	user, err = r.userService.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated user: %w", err)
	}

	workspaces, err := r.userService.GetUserWorkspaces(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user workspaces: %w", err)
	}
	return &AdminUserResolver{user: user, workspaceCount: len(workspaces), r: r}, nil
}

// AdminToggleUserStatus activates or deactivates a user
func (r *Resolver) AdminToggleUserStatus(ctx context.Context, id string, isActive bool) (*AdminUserResolver, error) {
	_, err := graphshared.RequireInstanceUserManager(ctx)
	if err != nil {
		return nil, err
	}

	if err := r.userService.ToggleUserStatus(ctx, id, isActive); err != nil {
		return nil, fmt.Errorf("failed to toggle user status: %w", err)
	}

	user, err := r.userService.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	workspaces, err := r.userService.GetUserWorkspaces(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user workspaces: %w", err)
	}
	return &AdminUserResolver{user: user, workspaceCount: len(workspaces), r: r}, nil
}

// AdminDeleteUser deletes a user
func (r *Resolver) AdminDeleteUser(ctx context.Context, id string) (bool, error) {
	_, err := graphshared.RequireInstanceUserManager(ctx)
	if err != nil {
		return false, err
	}

	if err := r.userService.DeleteUser(ctx, id); err != nil {
		return false, fmt.Errorf("failed to delete user: %w", err)
	}

	return true, nil
}

// =============================================================================
// Type Resolvers
// =============================================================================

// WorkspaceResolver resolves Workspace fields
type WorkspaceResolver struct {
	workspace *platformdomain.Workspace
	r         *Resolver
}

// ID returns the workspace ID
func (w *WorkspaceResolver) ID() model.ID {
	return model.ID(w.workspace.ID)
}

// Name returns the workspace name
func (w *WorkspaceResolver) Name() string {
	return w.workspace.Name
}

// ShortCode returns the workspace slug/short code
func (w *WorkspaceResolver) ShortCode() string {
	return w.workspace.Slug
}

// Agents resolves the agents for this workspace
func (w *WorkspaceResolver) Agents(ctx context.Context) ([]*AgentResolver, error) {
	agents, err := w.r.agentService.ListAgents(ctx, w.workspace.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	result := make([]*AgentResolver, len(agents))
	for i, agent := range agents {
		result[i] = &AgentResolver{agent: agent, r: w.r}
	}
	return result, nil
}

// Teams resolves the teams for this workspace.
func (w *WorkspaceResolver) Teams(ctx context.Context) ([]*TeamResolver, error) {
	teams, err := w.r.workspaceService.ListWorkspaceTeams(ctx, w.workspace.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	result := make([]*TeamResolver, len(teams))
	for i, team := range teams {
		result[i] = &TeamResolver{team: team, r: w.r}
	}
	return result, nil
}

// Cases resolves linked support cases for the workspace
func (w *WorkspaceResolver) Cases(ctx context.Context, args struct{ Filter *model.CaseFilterInput }) (*serviceresolvers.CaseConnectionResolver, error) {
	if w.r == nil || w.r.caseService == nil || w.r.serviceGraph == nil {
		return nil, nil
	}

	filters := contracts.CaseFilters{
		WorkspaceID: w.workspace.ID,
		Limit:       50,
	}
	if args.Filter != nil {
		if args.Filter.Status != nil && len(*args.Filter.Status) > 0 {
			filters.Status = (*args.Filter.Status)[0]
		}
		if args.Filter.Priority != nil && len(*args.Filter.Priority) > 0 {
			filters.Priority = (*args.Filter.Priority)[0]
		}
		if args.Filter.QueueID != nil {
			filters.QueueID = *args.Filter.QueueID
		}
		if args.Filter.AssigneeID != nil {
			filters.AssignedTo = *args.Filter.AssigneeID
		}
		if args.Filter.First != nil {
			filters.Limit = int(*args.Filter.First)
		}
	}

	cases, total, err := w.r.caseService.ListCases(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list cases: %w", err)
	}
	return w.r.serviceGraph.NewCaseConnectionResolver(cases, total, filters.Limit), nil
}

// Queues resolves linked support queues for the workspace.
func (w *WorkspaceResolver) Queues(ctx context.Context) ([]*serviceresolvers.QueueResolver, error) {
	if w.r == nil || w.r.queueService == nil || w.r.serviceGraph == nil {
		return []*serviceresolvers.QueueResolver{}, nil
	}

	queues, err := w.r.queueService.ListWorkspaceQueues(ctx, w.workspace.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list queues: %w", err)
	}
	return w.r.serviceGraph.NewQueueResolvers(queues), nil
}

// Extensions resolves installed extensions for this workspace.
func (w *WorkspaceResolver) Extensions(ctx context.Context) ([]*InstalledExtensionResolver, error) {
	if w.r == nil || w.r.extensionService == nil {
		return []*InstalledExtensionResolver{}, nil
	}
	return w.r.Extensions(ctx, w.workspace.ID)
}

// TeamResolver resolves Team fields.
type TeamResolver struct {
	team *platformdomain.Team
	r    *Resolver
}

func (t *TeamResolver) ID() model.ID               { return model.ID(t.team.ID) }
func (t *TeamResolver) WorkspaceID() model.ID      { return model.ID(t.team.WorkspaceID) }
func (t *TeamResolver) Name() string               { return t.team.Name }
func (t *TeamResolver) Description() *string       { return optionalString(t.team.Description) }
func (t *TeamResolver) EmailAddress() *string      { return optionalString(t.team.EmailAddress) }
func (t *TeamResolver) ResponseTimeHours() int32   { return int32(t.team.ResponseTimeHours) }
func (t *TeamResolver) ResolutionTimeHours() int32 { return int32(t.team.ResolutionTimeHours) }
func (t *TeamResolver) AutoAssign() bool           { return t.team.AutoAssign }
func (t *TeamResolver) AutoAssignKeywords() []string {
	return append([]string(nil), t.team.AutoAssignKeywords...)
}
func (t *TeamResolver) IsActive() bool { return t.team.IsActive }
func (t *TeamResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: t.team.CreatedAt}
}
func (t *TeamResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: t.team.UpdatedAt}
}

func (t *TeamResolver) Members(ctx context.Context) ([]*TeamMemberResolver, error) {
	members, err := t.r.workspaceService.GetTeamMembers(ctx, t.team.WorkspaceID, t.team.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list team members: %w", err)
	}
	result := make([]*TeamMemberResolver, len(members))
	for i, member := range members {
		result[i] = &TeamMemberResolver{member: member, r: t.r}
	}
	return result, nil
}

// TeamMemberResolver resolves TeamMember fields.
type TeamMemberResolver struct {
	member *platformdomain.TeamMember
	r      *Resolver
}

func (t *TeamMemberResolver) ID() model.ID          { return model.ID(t.member.ID) }
func (t *TeamMemberResolver) TeamID() model.ID      { return model.ID(t.member.TeamID) }
func (t *TeamMemberResolver) UserID() model.ID      { return model.ID(t.member.UserID) }
func (t *TeamMemberResolver) WorkspaceID() model.ID { return model.ID(t.member.WorkspaceID) }
func (t *TeamMemberResolver) Role() string          { return string(t.member.Role) }
func (t *TeamMemberResolver) IsActive() bool        { return t.member.IsActive }
func (t *TeamMemberResolver) JoinedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: t.member.JoinedAt}
}
func (t *TeamMemberResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: t.member.CreatedAt}
}
func (t *TeamMemberResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: t.member.UpdatedAt}
}

// AgentResolver resolves Agent fields
type AgentResolver struct {
	agent *platformdomain.Agent
	r     *Resolver
}

// ID returns the agent ID
func (a *AgentResolver) ID() model.ID {
	return model.ID(a.agent.ID)
}

// WorkspaceID returns the workspace ID
func (a *AgentResolver) WorkspaceID() model.ID {
	return model.ID(a.agent.WorkspaceID)
}

// Name returns the agent name
func (a *AgentResolver) Name() string {
	return a.agent.Name
}

// Description returns the agent description
func (a *AgentResolver) Description() *string {
	if a.agent.Description == "" {
		return nil
	}
	return &a.agent.Description
}

// OwnerID returns the owner ID
func (a *AgentResolver) OwnerID() model.ID {
	return model.ID(a.agent.OwnerID)
}

// Owner resolves the owner user
func (a *AgentResolver) Owner(ctx context.Context) (*model.User, error) {
	if a.r.userService == nil {
		return nil, nil
	}

	user, err := a.r.userService.GetUser(ctx, a.agent.OwnerID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get owner: %w", err)
	}

	return userToModel(user), nil
}

// Status returns the agent status
func (a *AgentResolver) Status() string {
	return string(a.agent.Status)
}

// StatusReason returns the status reason
func (a *AgentResolver) StatusReason() *string {
	if a.agent.StatusReason == "" {
		return nil
	}
	return &a.agent.StatusReason
}

// CreatedAt returns the creation timestamp
func (a *AgentResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: a.agent.CreatedAt}
}

// UpdatedAt returns the update timestamp
func (a *AgentResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: a.agent.UpdatedAt}
}

// CreatedByID returns the creator ID
func (a *AgentResolver) CreatedByID() model.ID {
	return model.ID(a.agent.CreatedByID)
}

// Tokens resolves the agent's tokens
func (a *AgentResolver) Tokens(ctx context.Context) ([]*AgentTokenResolver, error) {
	tokens, err := a.r.agentService.ListAgentTokens(ctx, a.agent.ID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return []*AgentTokenResolver{}, nil
		}
		return nil, fmt.Errorf("failed to list agent tokens: %w", err)
	}

	result := make([]*AgentTokenResolver, len(tokens))
	for i, token := range tokens {
		result[i] = &AgentTokenResolver{token: token, r: a.r}
	}
	return result, nil
}

// Membership resolves the agent's workspace membership
func (a *AgentResolver) Membership(ctx context.Context) (*WorkspaceMembershipResolver, error) {
	if a.r.agentService == nil {
		return nil, nil
	}

	membership, err := a.r.agentService.GetWorkspaceMembership(ctx, a.agent.WorkspaceID, a.agent.ID, platformdomain.PrincipalTypeAgent)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get membership: %w", err)
	}

	return &WorkspaceMembershipResolver{membership: membership, r: a.r}, nil
}

// AgentTokenResolver resolves AgentToken fields
type AgentTokenResolver struct {
	token *platformdomain.AgentToken
	r     *Resolver
}

// ID returns the token ID
func (t *AgentTokenResolver) ID() model.ID {
	return model.ID(t.token.ID)
}

// AgentID returns the agent ID
func (t *AgentTokenResolver) AgentID() model.ID {
	return model.ID(t.token.AgentID)
}

// TokenPrefix returns the token prefix
func (t *AgentTokenResolver) TokenPrefix() string {
	return t.token.TokenPrefix
}

// Name returns the token name
func (t *AgentTokenResolver) Name() string {
	return t.token.Name
}

// ExpiresAt returns the expiration timestamp
func (t *AgentTokenResolver) ExpiresAt() *graphshared.DateTime {
	if t.token.ExpiresAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *t.token.ExpiresAt}
}

// RevokedAt returns the revocation timestamp
func (t *AgentTokenResolver) RevokedAt() *graphshared.DateTime {
	if t.token.RevokedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *t.token.RevokedAt}
}

// LastUsedAt returns the last used timestamp
func (t *AgentTokenResolver) LastUsedAt() *graphshared.DateTime {
	if t.token.LastUsedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *t.token.LastUsedAt}
}

// LastUsedIP returns the last used IP
func (t *AgentTokenResolver) LastUsedIP() *string {
	if t.token.LastUsedIP == "" {
		return nil
	}
	return &t.token.LastUsedIP
}

// UseCount returns the use count
func (t *AgentTokenResolver) UseCount() int32 {
	return int32(t.token.UseCount)
}

// CreatedAt returns the creation timestamp
func (t *AgentTokenResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: t.token.CreatedAt}
}

// CreatedByID returns the creator ID
func (t *AgentTokenResolver) CreatedByID() model.ID {
	return model.ID(t.token.CreatedByID)
}

// AgentTokenResultResolver resolves the result of creating a token
type AgentTokenResultResolver struct {
	token     *platformdomain.AgentToken
	plaintext string
	r         *Resolver
}

// Token returns the token resolver
func (r *AgentTokenResultResolver) Token() *AgentTokenResolver {
	return &AgentTokenResolver{token: r.token, r: r.r}
}

// PlaintextToken returns the plaintext token (only shown once)
func (r *AgentTokenResultResolver) PlaintextToken() string {
	return r.plaintext
}

// WorkspaceMembershipResolver resolves WorkspaceMembership fields
type WorkspaceMembershipResolver struct {
	membership *platformdomain.WorkspaceMembership
	r          *Resolver
}

type InstalledExtensionResolver struct {
	extension *platformdomain.InstalledExtension
	r         *Resolver
}

func (e *InstalledExtensionResolver) ID() model.ID { return model.ID(e.extension.ID) }
func (e *InstalledExtensionResolver) WorkspaceID() *model.ID {
	if strings.TrimSpace(e.extension.WorkspaceID) == "" {
		return nil
	}
	workspaceID := model.ID(e.extension.WorkspaceID)
	return &workspaceID
}
func (e *InstalledExtensionResolver) Slug() string      { return e.extension.Slug }
func (e *InstalledExtensionResolver) Name() string      { return e.extension.Name }
func (e *InstalledExtensionResolver) Publisher() string { return e.extension.Publisher }
func (e *InstalledExtensionResolver) Version() string   { return e.extension.Version }
func (e *InstalledExtensionResolver) Description() *string {
	if e.extension.Description == "" {
		return nil
	}
	return &e.extension.Description
}
func (e *InstalledExtensionResolver) Kind() string  { return string(e.extension.Manifest.Kind) }
func (e *InstalledExtensionResolver) Scope() string { return string(e.extension.Manifest.Scope) }
func (e *InstalledExtensionResolver) Risk() string  { return string(e.extension.Manifest.Risk) }
func (e *InstalledExtensionResolver) RuntimeClass() string {
	return string(e.extension.Manifest.RuntimeClass)
}
func (e *InstalledExtensionResolver) StorageClass() string {
	return string(e.extension.Manifest.StorageClass)
}
func (e *InstalledExtensionResolver) Schema() *ExtensionSchemaResolver {
	if e.extension.Manifest.Schema.Name == "" &&
		e.extension.Manifest.Schema.PackageKey == "" &&
		e.extension.Manifest.Schema.TargetVersion == "" &&
		e.extension.Manifest.Schema.MigrationEngine == "" {
		return nil
	}
	return &ExtensionSchemaResolver{schema: e.extension.Manifest.Schema}
}
func (e *InstalledExtensionResolver) WorkspacePlan() *ExtensionWorkspacePlanResolver {
	if e.extension.Manifest.WorkspacePlan.Mode == "" &&
		e.extension.Manifest.WorkspacePlan.Name == "" &&
		e.extension.Manifest.WorkspacePlan.Slug == "" &&
		e.extension.Manifest.WorkspacePlan.Description == "" {
		return nil
	}
	return &ExtensionWorkspacePlanResolver{plan: e.extension.Manifest.WorkspacePlan}
}
func (e *InstalledExtensionResolver) Permissions() []string {
	return e.extension.Manifest.Permissions
}
func (e *InstalledExtensionResolver) SeedQueues() []*ExtensionQueueSeedResolver {
	result := make([]*ExtensionQueueSeedResolver, len(e.extension.Manifest.Queues))
	for i := range e.extension.Manifest.Queues {
		result[i] = &ExtensionQueueSeedResolver{seed: e.extension.Manifest.Queues[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) SeedForms() []*ExtensionFormSeedResolver {
	result := make([]*ExtensionFormSeedResolver, len(e.extension.Manifest.Forms))
	for i := range e.extension.Manifest.Forms {
		result[i] = &ExtensionFormSeedResolver{seed: e.extension.Manifest.Forms[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) SeedAutomationRules() []*ExtensionAutomationSeedResolver {
	result := make([]*ExtensionAutomationSeedResolver, len(e.extension.Manifest.AutomationRules))
	for i := range e.extension.Manifest.AutomationRules {
		result[i] = &ExtensionAutomationSeedResolver{seed: e.extension.Manifest.AutomationRules[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) ArtifactSurfaces() []*ExtensionArtifactSurfaceResolver {
	result := make([]*ExtensionArtifactSurfaceResolver, len(e.extension.Manifest.ArtifactSurfaces))
	for i := range e.extension.Manifest.ArtifactSurfaces {
		result[i] = &ExtensionArtifactSurfaceResolver{surface: e.extension.Manifest.ArtifactSurfaces[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) PublicRoutes() []*ExtensionRouteResolver {
	result := make([]*ExtensionRouteResolver, len(e.extension.Manifest.PublicRoutes))
	for i := range e.extension.Manifest.PublicRoutes {
		result[i] = &ExtensionRouteResolver{route: e.extension.Manifest.PublicRoutes[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) AdminRoutes() []*ExtensionRouteResolver {
	result := make([]*ExtensionRouteResolver, len(e.extension.Manifest.AdminRoutes))
	for i := range e.extension.Manifest.AdminRoutes {
		result[i] = &ExtensionRouteResolver{route: e.extension.Manifest.AdminRoutes[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) Endpoints() []*ExtensionEndpointResolver {
	result := make([]*ExtensionEndpointResolver, len(e.extension.Manifest.Endpoints))
	for i := range e.extension.Manifest.Endpoints {
		result[i] = &ExtensionEndpointResolver{endpoint: e.extension.Manifest.Endpoints[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) AdminNavigation() []*ExtensionAdminNavigationItemResolver {
	result := make([]*ExtensionAdminNavigationItemResolver, len(e.extension.Manifest.AdminNavigation))
	for i := range e.extension.Manifest.AdminNavigation {
		result[i] = &ExtensionAdminNavigationItemResolver{item: e.extension.Manifest.AdminNavigation[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) DashboardWidgets() []*ExtensionDashboardWidgetResolver {
	result := make([]*ExtensionDashboardWidgetResolver, len(e.extension.Manifest.DashboardWidgets))
	for i := range e.extension.Manifest.DashboardWidgets {
		result[i] = &ExtensionDashboardWidgetResolver{widget: e.extension.Manifest.DashboardWidgets[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) Events() *ExtensionEventCatalogResolver {
	return &ExtensionEventCatalogResolver{catalog: e.extension.Manifest.Events}
}
func (e *InstalledExtensionResolver) EventConsumers() []*ExtensionEventConsumerResolver {
	result := make([]*ExtensionEventConsumerResolver, len(e.extension.Manifest.EventConsumers))
	for i := range e.extension.Manifest.EventConsumers {
		result[i] = &ExtensionEventConsumerResolver{consumer: e.extension.Manifest.EventConsumers[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) ScheduledJobs() []*ExtensionScheduledJobResolver {
	result := make([]*ExtensionScheduledJobResolver, len(e.extension.Manifest.ScheduledJobs))
	for i := range e.extension.Manifest.ScheduledJobs {
		result[i] = &ExtensionScheduledJobResolver{job: e.extension.Manifest.ScheduledJobs[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) Commands() []*ExtensionCommandResolver {
	result := make([]*ExtensionCommandResolver, len(e.extension.Manifest.Commands))
	for i := range e.extension.Manifest.Commands {
		result[i] = &ExtensionCommandResolver{command: e.extension.Manifest.Commands[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) AgentSkills() []*ExtensionAgentSkillResolver {
	result := make([]*ExtensionAgentSkillResolver, len(e.extension.Manifest.AgentSkills))
	for i := range e.extension.Manifest.AgentSkills {
		result[i] = &ExtensionAgentSkillResolver{skill: e.extension.Manifest.AgentSkills[i]}
	}
	return result
}
func (e *InstalledExtensionResolver) CustomizableAssets() []string {
	return e.extension.Manifest.CustomizableAssets
}
func (e *InstalledExtensionResolver) Config() graphshared.JSON {
	return graphshared.JSON(e.extension.Config.ToMap())
}
func (e *InstalledExtensionResolver) Status() string { return string(e.extension.Status) }
func (e *InstalledExtensionResolver) ValidationStatus() string {
	return string(e.extension.ValidationStatus)
}
func (e *InstalledExtensionResolver) ValidationMessage() *string {
	if e.extension.ValidationMessage == "" {
		return nil
	}
	return &e.extension.ValidationMessage
}
func (e *InstalledExtensionResolver) HealthStatus() string { return string(e.extension.HealthStatus) }
func (e *InstalledExtensionResolver) HealthMessage() *string {
	if e.extension.HealthMessage == "" {
		return nil
	}
	return &e.extension.HealthMessage
}
func (e *InstalledExtensionResolver) BundleSHA256() string { return e.extension.BundleSHA256 }
func (e *InstalledExtensionResolver) BundleSize() int32    { return int32(e.extension.BundleSize) }
func (e *InstalledExtensionResolver) InstalledByID() *model.ID {
	if e.extension.InstalledByID == "" {
		return nil
	}
	id := model.ID(e.extension.InstalledByID)
	return &id
}
func (e *InstalledExtensionResolver) InstalledAt() graphshared.DateTime {
	return graphshared.DateTime{Time: e.extension.InstalledAt}
}
func (e *InstalledExtensionResolver) ActivatedAt() *graphshared.DateTime {
	if e.extension.ActivatedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *e.extension.ActivatedAt}
}
func (e *InstalledExtensionResolver) DeactivatedAt() *graphshared.DateTime {
	if e.extension.DeactivatedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *e.extension.DeactivatedAt}
}
func (e *InstalledExtensionResolver) ValidatedAt() *graphshared.DateTime {
	if e.extension.ValidatedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *e.extension.ValidatedAt}
}
func (e *InstalledExtensionResolver) LastHealthCheckAt() *graphshared.DateTime {
	if e.extension.LastHealthCheckAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *e.extension.LastHealthCheckAt}
}
func (e *InstalledExtensionResolver) RuntimeDiagnostics(ctx context.Context) (*ExtensionRuntimeDiagnosticsResolver, error) {
	if e.r == nil || e.r.extensionService == nil {
		return &ExtensionRuntimeDiagnosticsResolver{}, nil
	}
	diagnostics, err := e.r.extensionService.GetExtensionRuntimeDiagnostics(ctx, e.extension.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve extension runtime diagnostics: %w", err)
	}
	return &ExtensionRuntimeDiagnosticsResolver{diagnostics: diagnostics}, nil
}
func (e *InstalledExtensionResolver) Assets(ctx context.Context) ([]*ExtensionAssetResolver, error) {
	if e.r == nil || e.r.extensionService == nil {
		return []*ExtensionAssetResolver{}, nil
	}
	assets, err := e.r.extensionService.ListExtensionAssets(ctx, e.extension.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list extension assets: %w", err)
	}
	result := make([]*ExtensionAssetResolver, len(assets))
	for i := range assets {
		result[i] = &ExtensionAssetResolver{asset: assets[i]}
	}
	return result, nil
}

type ExtensionQueueSeedResolver struct {
	seed platformdomain.ExtensionQueueSeed
}

func (e *ExtensionQueueSeedResolver) Slug() string { return e.seed.Slug }
func (e *ExtensionQueueSeedResolver) Name() string { return e.seed.Name }
func (e *ExtensionQueueSeedResolver) Description() *string {
	if e.seed.Description == "" {
		return nil
	}
	return &e.seed.Description
}

type ExtensionWorkspacePlanResolver struct {
	plan platformdomain.ExtensionWorkspacePlan
}

type ExtensionSchemaResolver struct {
	schema platformdomain.ExtensionSchemaManifest
}

func (e *ExtensionSchemaResolver) Name() string            { return e.schema.Name }
func (e *ExtensionSchemaResolver) PackageKey() string      { return e.schema.PackageKey }
func (e *ExtensionSchemaResolver) TargetVersion() string   { return e.schema.TargetVersion }
func (e *ExtensionSchemaResolver) MigrationEngine() string { return e.schema.MigrationEngine }

func (e *ExtensionWorkspacePlanResolver) Mode() *string {
	if e.plan.Mode == "" {
		return nil
	}
	value := string(e.plan.Mode)
	return &value
}
func (e *ExtensionWorkspacePlanResolver) Name() *string {
	if e.plan.Name == "" {
		return nil
	}
	return &e.plan.Name
}
func (e *ExtensionWorkspacePlanResolver) Slug() *string {
	if e.plan.Slug == "" {
		return nil
	}
	return &e.plan.Slug
}
func (e *ExtensionWorkspacePlanResolver) Description() *string {
	if e.plan.Description == "" {
		return nil
	}
	return &e.plan.Description
}

type ExtensionFormSeedResolver struct {
	seed platformdomain.ExtensionFormSeed
}

func (e *ExtensionFormSeedResolver) Slug() string { return e.seed.Slug }
func (e *ExtensionFormSeedResolver) Name() string { return e.seed.Name }
func (e *ExtensionFormSeedResolver) Description() *string {
	if e.seed.Description == "" {
		return nil
	}
	return &e.seed.Description
}
func (e *ExtensionFormSeedResolver) Status() *string {
	if e.seed.Status == "" {
		return nil
	}
	return &e.seed.Status
}
func (e *ExtensionFormSeedResolver) IsPublic() bool       { return e.seed.IsPublic }
func (e *ExtensionFormSeedResolver) RequiresAuth() bool   { return e.seed.RequiresAuth }
func (e *ExtensionFormSeedResolver) AllowMultiple() bool  { return e.seed.AllowMultiple }
func (e *ExtensionFormSeedResolver) CollectEmail() bool   { return e.seed.CollectEmail }
func (e *ExtensionFormSeedResolver) AutoCreateCase() bool { return e.seed.AutoCreateCase }
func (e *ExtensionFormSeedResolver) AutoCasePriority() *string {
	if e.seed.AutoCasePriority == "" {
		return nil
	}
	return &e.seed.AutoCasePriority
}
func (e *ExtensionFormSeedResolver) AutoCaseType() *string {
	if e.seed.AutoCaseType == "" {
		return nil
	}
	return &e.seed.AutoCaseType
}
func (e *ExtensionFormSeedResolver) AutoTags() []string { return e.seed.AutoTags }
func (e *ExtensionFormSeedResolver) SubmissionMessage() *string {
	if e.seed.SubmissionMessage == "" {
		return nil
	}
	return &e.seed.SubmissionMessage
}
func (e *ExtensionFormSeedResolver) RedirectURL() *string {
	if e.seed.RedirectURL == "" {
		return nil
	}
	return &e.seed.RedirectURL
}
func (e *ExtensionFormSeedResolver) Theme() *string {
	if e.seed.Theme == "" {
		return nil
	}
	return &e.seed.Theme
}
func (e *ExtensionFormSeedResolver) Schema() graphshared.JSON {
	return graphshared.JSON(e.seed.Schema.ToMap())
}
func (e *ExtensionFormSeedResolver) UISchema() graphshared.JSON {
	return graphshared.JSON(e.seed.UISchema.ToMap())
}
func (e *ExtensionFormSeedResolver) ValidationRules() graphshared.JSON {
	return graphshared.JSON(e.seed.ValidationRules.ToMap())
}

type ExtensionAutomationSeedResolver struct {
	seed platformdomain.ExtensionAutomationSeed
}

func (e *ExtensionAutomationSeedResolver) Key() string   { return e.seed.Key }
func (e *ExtensionAutomationSeedResolver) Title() string { return e.seed.Title }
func (e *ExtensionAutomationSeedResolver) Description() *string {
	if e.seed.Description == "" {
		return nil
	}
	return &e.seed.Description
}
func (e *ExtensionAutomationSeedResolver) IsActive() bool  { return e.seed.IsActive }
func (e *ExtensionAutomationSeedResolver) Priority() int32 { return int32(e.seed.Priority) }
func (e *ExtensionAutomationSeedResolver) MaxExecutionsPerHour() int32 {
	return int32(e.seed.MaxExecutionsPerHour)
}
func (e *ExtensionAutomationSeedResolver) MaxExecutionsPerDay() int32 {
	return int32(e.seed.MaxExecutionsPerDay)
}
func (e *ExtensionAutomationSeedResolver) Conditions() graphshared.JSON {
	return graphshared.JSON(e.seed.Conditions.ToMap())
}
func (e *ExtensionAutomationSeedResolver) Actions() graphshared.JSON {
	return graphshared.JSON(e.seed.Actions.ToMap())
}

type ExtensionRouteResolver struct {
	route platformdomain.ExtensionRoute
}

func (e *ExtensionRouteResolver) PathPrefix() string { return e.route.PathPrefix }
func (e *ExtensionRouteResolver) AssetPath() *string { return optionalString(e.route.AssetPath) }
func (e *ExtensionRouteResolver) ArtifactSurface() *string {
	return optionalString(e.route.ArtifactSurface)
}
func (e *ExtensionRouteResolver) ArtifactPath() *string {
	return optionalString(e.route.ArtifactPath)
}

type ExtensionEndpointResolver struct {
	endpoint platformdomain.ExtensionEndpoint
}

func (e *ExtensionEndpointResolver) Name() string           { return e.endpoint.Name }
func (e *ExtensionEndpointResolver) Class() string          { return string(e.endpoint.Class) }
func (e *ExtensionEndpointResolver) MountPath() string      { return e.endpoint.MountPath }
func (e *ExtensionEndpointResolver) Methods() []string      { return e.endpoint.Methods }
func (e *ExtensionEndpointResolver) Auth() string           { return string(e.endpoint.Auth) }
func (e *ExtensionEndpointResolver) ContentTypes() []string { return e.endpoint.ContentTypes }
func (e *ExtensionEndpointResolver) MaxBodyBytes() int32    { return int32(e.endpoint.MaxBodyBytes) }
func (e *ExtensionEndpointResolver) RateLimitPerMinute() int32 {
	return int32(e.endpoint.RateLimitPerMin)
}
func (e *ExtensionEndpointResolver) WorkspaceBinding() string {
	return string(e.endpoint.WorkspaceBinding)
}
func (e *ExtensionEndpointResolver) AssetPath() *string {
	if e.endpoint.AssetPath == "" {
		return nil
	}
	return &e.endpoint.AssetPath
}
func (e *ExtensionEndpointResolver) ArtifactSurface() *string {
	return optionalString(e.endpoint.ArtifactSurface)
}
func (e *ExtensionEndpointResolver) ArtifactPath() *string {
	return optionalString(e.endpoint.ArtifactPath)
}
func (e *ExtensionEndpointResolver) ServiceTarget() *string {
	if e.endpoint.ServiceTarget == "" {
		return nil
	}
	return &e.endpoint.ServiceTarget
}

type ExtensionArtifactSurfaceResolver struct {
	surface platformdomain.ExtensionArtifactSurface
}

func (e *ExtensionArtifactSurfaceResolver) Name() string { return e.surface.Name }
func (e *ExtensionArtifactSurfaceResolver) Description() *string {
	return optionalString(e.surface.Description)
}
func (e *ExtensionArtifactSurfaceResolver) SeedAssetPath() *string {
	return optionalString(e.surface.SeedAssetPath)
}

type ExtensionArtifactFileResolver struct {
	file platformdomain.ExtensionArtifactFile
}

func (e *ExtensionArtifactFileResolver) Surface() string { return e.file.Surface }
func (e *ExtensionArtifactFileResolver) Path() string    { return e.file.Path }

type ArtifactRevisionResolver struct {
	revision artifactservices.Revision
}

func (a *ArtifactRevisionResolver) Ref() string { return a.revision.Ref }
func (a *ArtifactRevisionResolver) CommittedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: a.revision.CommittedAt}
}
func (a *ArtifactRevisionResolver) Subject() string { return a.revision.Subject }

type ArtifactDiffResolver struct {
	diff *platformdomain.ExtensionArtifactDiff
}

func (a *ArtifactDiffResolver) FromRevision() *string {
	if a == nil || a.diff == nil {
		return nil
	}
	return optionalString(a.diff.FromRevision)
}
func (a *ArtifactDiffResolver) ToRevision() string {
	if a == nil || a.diff == nil {
		return ""
	}
	return a.diff.ToRevision
}
func (a *ArtifactDiffResolver) Patch() string {
	if a == nil || a.diff == nil {
		return ""
	}
	return a.diff.Patch
}

type ExtensionArtifactPublicationResolver struct {
	publication *platformdomain.ExtensionArtifactPublication
}

func (e *ExtensionArtifactPublicationResolver) Surface() string {
	if e == nil || e.publication == nil {
		return ""
	}
	return e.publication.Surface
}
func (e *ExtensionArtifactPublicationResolver) Path() string {
	if e == nil || e.publication == nil {
		return ""
	}
	return e.publication.Path
}
func (e *ExtensionArtifactPublicationResolver) RevisionRef() string {
	if e == nil || e.publication == nil {
		return ""
	}
	return e.publication.RevisionRef
}

type ExtensionAdminNavigationItemResolver struct {
	item platformdomain.ExtensionAdminNavigationItem
}

func (e *ExtensionAdminNavigationItemResolver) Name() string { return e.item.Name }
func (e *ExtensionAdminNavigationItemResolver) Section() *string {
	if e.item.Section == "" {
		return nil
	}
	return &e.item.Section
}
func (e *ExtensionAdminNavigationItemResolver) Title() string { return e.item.Title }
func (e *ExtensionAdminNavigationItemResolver) Icon() *string {
	if e.item.Icon == "" {
		return nil
	}
	return &e.item.Icon
}
func (e *ExtensionAdminNavigationItemResolver) Endpoint() string { return e.item.Endpoint }
func (e *ExtensionAdminNavigationItemResolver) ActivePage() *string {
	if e.item.ActivePage == "" {
		return nil
	}
	return &e.item.ActivePage
}

type ExtensionDashboardWidgetResolver struct {
	widget platformdomain.ExtensionDashboardWidget
}

func (e *ExtensionDashboardWidgetResolver) Name() string  { return e.widget.Name }
func (e *ExtensionDashboardWidgetResolver) Title() string { return e.widget.Title }
func (e *ExtensionDashboardWidgetResolver) Description() *string {
	if e.widget.Description == "" {
		return nil
	}
	return &e.widget.Description
}
func (e *ExtensionDashboardWidgetResolver) Icon() *string {
	if e.widget.Icon == "" {
		return nil
	}
	return &e.widget.Icon
}
func (e *ExtensionDashboardWidgetResolver) Endpoint() string { return e.widget.Endpoint }

type ExtensionCommandResolver struct {
	command platformdomain.ExtensionCommand
}

func (e *ExtensionCommandResolver) Name() string { return e.command.Name }
func (e *ExtensionCommandResolver) Description() *string {
	if e.command.Description == "" {
		return nil
	}
	return &e.command.Description
}

type ExtensionAgentSkillResolver struct {
	skill platformdomain.ExtensionAgentSkill
}

func (e *ExtensionAgentSkillResolver) Name() string { return e.skill.Name }
func (e *ExtensionAgentSkillResolver) Description() *string {
	if e.skill.Description == "" {
		return nil
	}
	return &e.skill.Description
}
func (e *ExtensionAgentSkillResolver) AssetPath() string { return e.skill.AssetPath }

type ExtensionEventCatalogResolver struct {
	catalog platformdomain.ExtensionEventCatalog
}

func (e *ExtensionEventCatalogResolver) Publishes() []*ExtensionEventDefinitionResolver {
	result := make([]*ExtensionEventDefinitionResolver, len(e.catalog.Publishes))
	for i := range e.catalog.Publishes {
		result[i] = &ExtensionEventDefinitionResolver{event: e.catalog.Publishes[i]}
	}
	return result
}
func (e *ExtensionEventCatalogResolver) Subscribes() []string { return e.catalog.Subscribes }

type ExtensionEventDefinitionResolver struct {
	event platformdomain.ExtensionEventDefinition
}

func (e *ExtensionEventDefinitionResolver) Type() string { return e.event.Type }
func (e *ExtensionEventDefinitionResolver) Description() *string {
	if e.event.Description == "" {
		return nil
	}
	return &e.event.Description
}
func (e *ExtensionEventDefinitionResolver) SchemaVersion() int32 {
	return int32(e.event.SchemaVersion)
}

type ExtensionEventConsumerResolver struct {
	consumer platformdomain.ExtensionEventConsumer
}

func (e *ExtensionEventConsumerResolver) Name() string { return e.consumer.Name }
func (e *ExtensionEventConsumerResolver) Description() *string {
	if e.consumer.Description == "" {
		return nil
	}
	return &e.consumer.Description
}
func (e *ExtensionEventConsumerResolver) Stream() string { return e.consumer.Stream }
func (e *ExtensionEventConsumerResolver) EventTypes() []string {
	return e.consumer.EventTypes
}
func (e *ExtensionEventConsumerResolver) ConsumerGroup() *string {
	if e.consumer.ConsumerGroup == "" {
		return nil
	}
	return &e.consumer.ConsumerGroup
}
func (e *ExtensionEventConsumerResolver) ServiceTarget() string { return e.consumer.ServiceTarget }

type ExtensionScheduledJobResolver struct {
	job platformdomain.ExtensionScheduledJob
}

func (e *ExtensionScheduledJobResolver) Name() string { return e.job.Name }
func (e *ExtensionScheduledJobResolver) Description() *string {
	if e.job.Description == "" {
		return nil
	}
	return &e.job.Description
}
func (e *ExtensionScheduledJobResolver) IntervalSeconds() int32 {
	return int32(e.job.IntervalSeconds)
}
func (e *ExtensionScheduledJobResolver) ServiceTarget() string { return e.job.ServiceTarget }

type ExtensionRuntimeDiagnosticsResolver struct {
	diagnostics platformdomain.ExtensionRuntimeDiagnostics
}

func (r *ExtensionRuntimeDiagnosticsResolver) BootstrapStatus() string {
	return r.diagnostics.BootstrapStatus
}

func (r *ExtensionRuntimeDiagnosticsResolver) LastBootstrapAt() *graphshared.DateTime {
	if r.diagnostics.LastBootstrapAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.diagnostics.LastBootstrapAt}
}

func (r *ExtensionRuntimeDiagnosticsResolver) LastBootstrapError() *string {
	if r.diagnostics.LastBootstrapError == "" {
		return nil
	}
	return &r.diagnostics.LastBootstrapError
}

func (r *ExtensionRuntimeDiagnosticsResolver) EventConsumers() []*ExtensionRuntimeConsumerStateResolver {
	result := make([]*ExtensionRuntimeConsumerStateResolver, len(r.diagnostics.EventConsumers))
	for i := range r.diagnostics.EventConsumers {
		result[i] = &ExtensionRuntimeConsumerStateResolver{state: r.diagnostics.EventConsumers[i]}
	}
	return result
}

func (r *ExtensionRuntimeDiagnosticsResolver) Endpoints() []*ExtensionRuntimeEndpointStateResolver {
	result := make([]*ExtensionRuntimeEndpointStateResolver, len(r.diagnostics.Endpoints))
	for i := range r.diagnostics.Endpoints {
		result[i] = &ExtensionRuntimeEndpointStateResolver{state: r.diagnostics.Endpoints[i]}
	}
	return result
}

func (r *ExtensionRuntimeDiagnosticsResolver) ScheduledJobs() []*ExtensionRuntimeJobStateResolver {
	result := make([]*ExtensionRuntimeJobStateResolver, len(r.diagnostics.ScheduledJobs))
	for i := range r.diagnostics.ScheduledJobs {
		result[i] = &ExtensionRuntimeJobStateResolver{state: r.diagnostics.ScheduledJobs[i]}
	}
	return result
}

type ExtensionRuntimeConsumerStateResolver struct {
	state platformdomain.ExtensionRuntimeConsumerState
}

type ExtensionRuntimeEndpointStateResolver struct {
	state platformdomain.ExtensionRuntimeEndpointState
}

func (r *ExtensionRuntimeEndpointStateResolver) Name() string      { return r.state.Name }
func (r *ExtensionRuntimeEndpointStateResolver) Class() string     { return r.state.Class }
func (r *ExtensionRuntimeEndpointStateResolver) MountPath() string { return r.state.MountPath }
func (r *ExtensionRuntimeEndpointStateResolver) Status() string    { return r.state.Status }
func (r *ExtensionRuntimeEndpointStateResolver) ConsecutiveFailures() int32 {
	return int32(r.state.ConsecutiveFailures)
}
func (r *ExtensionRuntimeEndpointStateResolver) ServiceTarget() *string {
	if r.state.ServiceTarget == "" {
		return nil
	}
	return &r.state.ServiceTarget
}
func (r *ExtensionRuntimeEndpointStateResolver) RegisteredAt() *graphshared.DateTime {
	if r.state.RegisteredAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.RegisteredAt}
}
func (r *ExtensionRuntimeEndpointStateResolver) LastCheckedAt() *graphshared.DateTime {
	if r.state.LastCheckedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.LastCheckedAt}
}
func (r *ExtensionRuntimeEndpointStateResolver) LastSuccessAt() *graphshared.DateTime {
	if r.state.LastSuccessAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.LastSuccessAt}
}
func (r *ExtensionRuntimeEndpointStateResolver) LastFailureAt() *graphshared.DateTime {
	if r.state.LastFailureAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.LastFailureAt}
}
func (r *ExtensionRuntimeEndpointStateResolver) LastError() *string {
	if r.state.LastError == "" {
		return nil
	}
	return &r.state.LastError
}

func (r *ExtensionRuntimeConsumerStateResolver) Name() string          { return r.state.Name }
func (r *ExtensionRuntimeConsumerStateResolver) Stream() string        { return r.state.Stream }
func (r *ExtensionRuntimeConsumerStateResolver) ServiceTarget() string { return r.state.ServiceTarget }
func (r *ExtensionRuntimeConsumerStateResolver) Status() string        { return r.state.Status }
func (r *ExtensionRuntimeConsumerStateResolver) ConsecutiveFailures() int32 {
	return int32(r.state.ConsecutiveFailures)
}
func (r *ExtensionRuntimeConsumerStateResolver) ConsumerGroup() *string {
	if r.state.ConsumerGroup == "" {
		return nil
	}
	return &r.state.ConsumerGroup
}
func (r *ExtensionRuntimeConsumerStateResolver) RegisteredAt() *graphshared.DateTime {
	if r.state.RegisteredAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.RegisteredAt}
}
func (r *ExtensionRuntimeConsumerStateResolver) LastDeliveredAt() *graphshared.DateTime {
	if r.state.LastDeliveredAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.LastDeliveredAt}
}
func (r *ExtensionRuntimeConsumerStateResolver) LastSuccessAt() *graphshared.DateTime {
	if r.state.LastSuccessAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.LastSuccessAt}
}
func (r *ExtensionRuntimeConsumerStateResolver) LastFailureAt() *graphshared.DateTime {
	if r.state.LastFailureAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.LastFailureAt}
}
func (r *ExtensionRuntimeConsumerStateResolver) LastError() *string {
	if r.state.LastError == "" {
		return nil
	}
	return &r.state.LastError
}

type ExtensionRuntimeJobStateResolver struct {
	state platformdomain.ExtensionRuntimeJobState
}

func (r *ExtensionRuntimeJobStateResolver) Name() string { return r.state.Name }
func (r *ExtensionRuntimeJobStateResolver) IntervalSeconds() int32 {
	return int32(r.state.IntervalSeconds)
}
func (r *ExtensionRuntimeJobStateResolver) ServiceTarget() string { return r.state.ServiceTarget }
func (r *ExtensionRuntimeJobStateResolver) Status() string        { return r.state.Status }
func (r *ExtensionRuntimeJobStateResolver) ConsecutiveFailures() int32 {
	return int32(r.state.ConsecutiveFailures)
}
func (r *ExtensionRuntimeJobStateResolver) RegisteredAt() *graphshared.DateTime {
	if r.state.RegisteredAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.RegisteredAt}
}
func (r *ExtensionRuntimeJobStateResolver) LastStartedAt() *graphshared.DateTime {
	if r.state.LastStartedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.LastStartedAt}
}
func (r *ExtensionRuntimeJobStateResolver) LastSuccessAt() *graphshared.DateTime {
	if r.state.LastSuccessAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.LastSuccessAt}
}
func (r *ExtensionRuntimeJobStateResolver) LastFailureAt() *graphshared.DateTime {
	if r.state.LastFailureAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.LastFailureAt}
}
func (r *ExtensionRuntimeJobStateResolver) BackoffUntil() *graphshared.DateTime {
	if r.state.BackoffUntil == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *r.state.BackoffUntil}
}
func (r *ExtensionRuntimeJobStateResolver) LastError() *string {
	if r.state.LastError == "" {
		return nil
	}
	return &r.state.LastError
}

type ExtensionRuntimeEventResolver struct {
	event platformdomain.ExtensionRuntimeEvent
}

func (e *ExtensionRuntimeEventResolver) Type() string { return e.event.Type }
func (e *ExtensionRuntimeEventResolver) Description() *string {
	if e.event.Description == "" {
		return nil
	}
	return &e.event.Description
}
func (e *ExtensionRuntimeEventResolver) SchemaVersion() int32  { return int32(e.event.SchemaVersion) }
func (e *ExtensionRuntimeEventResolver) Core() bool            { return e.event.Core }
func (e *ExtensionRuntimeEventResolver) Publishers() []string  { return e.event.Publishers }
func (e *ExtensionRuntimeEventResolver) Subscribers() []string { return e.event.Subscribers }

type ExtensionAssetResolver struct {
	asset *platformdomain.ExtensionAsset
}

func (e *ExtensionAssetResolver) ID() model.ID         { return model.ID(e.asset.ID) }
func (e *ExtensionAssetResolver) Path() string         { return e.asset.Path }
func (e *ExtensionAssetResolver) Kind() string         { return string(e.asset.Kind) }
func (e *ExtensionAssetResolver) ContentType() string  { return e.asset.ContentType }
func (e *ExtensionAssetResolver) IsCustomizable() bool { return e.asset.IsCustomizable }
func (e *ExtensionAssetResolver) Checksum() string     { return e.asset.Checksum }
func (e *ExtensionAssetResolver) Size() int32          { return int32(e.asset.Size) }
func (e *ExtensionAssetResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: e.asset.UpdatedAt}
}
func (e *ExtensionAssetResolver) TextContent() *string {
	if len(e.asset.Content) == 0 || !utf8.Valid(e.asset.Content) {
		return nil
	}
	text := string(e.asset.Content)
	return &text
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefStringSlice(value *[]string) []string {
	if value == nil {
		return nil
	}
	return append([]string(nil), (*value)...)
}

func derefInt32OrDefault(value *int32, fallback int) int {
	if value == nil {
		return fallback
	}
	return int(*value)
}

func authActorID(authCtx *platformdomain.AuthContext) string {
	if authCtx == nil {
		return ""
	}
	if agent := authCtx.GetAgent(); agent != nil {
		return agent.ID
	}
	if user := authCtx.GetHuman(); user != nil {
		return user.ID
	}
	return ""
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func optionalTrimmedString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func optionalInt32(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func membershipConstraintsFromInput(input *model.MembershipConstraintsInput) platformdomain.MembershipConstraints {
	if input == nil {
		return platformdomain.MembershipConstraints{}
	}

	constraints := platformdomain.MembershipConstraints{
		AllowedIPs:              derefStringSlice(input.AllowedIPs),
		AllowedProjectIDs:       derefStringSlice(input.AllowedProjectIDs),
		AllowedTeamIDs:          derefStringSlice(input.AllowedTeamIDs),
		DelegatedRoutingTeamIDs: derefStringSlice(input.DelegatedRoutingTeamIDs),
		AllowDelegatedRouting:   input.AllowDelegatedRouting != nil && *input.AllowDelegatedRouting,
		ActiveHoursStart:        optionalTrimmedString(input.ActiveHoursStart),
		ActiveHoursEnd:          optionalTrimmedString(input.ActiveHoursEnd),
		ActiveTimezone:          optionalTrimmedString(input.ActiveTimezone),
	}
	if input.RateLimitPerMinute != nil {
		value := int(*input.RateLimitPerMinute)
		constraints.RateLimitPerMinute = &value
	}
	if input.RateLimitPerHour != nil {
		value := int(*input.RateLimitPerHour)
		constraints.RateLimitPerHour = &value
	}
	if input.ActiveDays != nil {
		constraints.ActiveDays = make([]int, len(*input.ActiveDays))
		for i, day := range *input.ActiveDays {
			constraints.ActiveDays[i] = int(day)
		}
	}
	return constraints
}

func normalizeTeamMemberRole(value string) (platformdomain.TeamMemberRole, error) {
	switch platformdomain.TeamMemberRole(strings.ToLower(strings.TrimSpace(value))) {
	case platformdomain.TeamMemberRoleLead:
		return platformdomain.TeamMemberRoleLead, nil
	case platformdomain.TeamMemberRoleMember:
		return platformdomain.TeamMemberRoleMember, nil
	default:
		return "", fmt.Errorf("team member role must be one of: %s, %s", platformdomain.TeamMemberRoleLead, platformdomain.TeamMemberRoleMember)
	}
}

func validateInstalledExtensionAccess(authCtx *platformdomain.AuthContext, extension *platformdomain.InstalledExtension) error {
	if extension == nil {
		return graphshared.ErrNotFound
	}
	if strings.TrimSpace(extension.WorkspaceID) == "" {
		if authCtx == nil || !authCtx.IsInstanceAdmin() {
			return graphshared.ErrNotFound
		}
		return nil
	}
	return graphshared.ValidateWorkspaceOwnership(extension.WorkspaceID, authCtx)
}

// ID returns the membership ID
func (m *WorkspaceMembershipResolver) ID() model.ID {
	return model.ID(m.membership.ID)
}

// WorkspaceID returns the workspace ID
func (m *WorkspaceMembershipResolver) WorkspaceID() model.ID {
	return model.ID(m.membership.WorkspaceID)
}

// PrincipalID returns the principal ID
func (m *WorkspaceMembershipResolver) PrincipalID() model.ID {
	return model.ID(m.membership.PrincipalID)
}

// PrincipalType returns the principal type
func (m *WorkspaceMembershipResolver) PrincipalType() string {
	return string(m.membership.PrincipalType)
}

// Role returns the role
func (m *WorkspaceMembershipResolver) Role() string {
	return m.membership.Role
}

// Permissions returns the permissions
func (m *WorkspaceMembershipResolver) Permissions() []string {
	return m.membership.Permissions
}

func (m *WorkspaceMembershipResolver) Constraints() *MembershipConstraintsResolver {
	return &MembershipConstraintsResolver{constraints: m.membership.Constraints}
}

// GrantedAt returns the grant timestamp
func (m *WorkspaceMembershipResolver) GrantedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: m.membership.GrantedAt}
}

// ExpiresAt returns the expiration timestamp
func (m *WorkspaceMembershipResolver) ExpiresAt() *graphshared.DateTime {
	if m.membership.ExpiresAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *m.membership.ExpiresAt}
}

// RevokedAt returns the revocation timestamp
func (m *WorkspaceMembershipResolver) RevokedAt() *graphshared.DateTime {
	if m.membership.RevokedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *m.membership.RevokedAt}
}

type MembershipConstraintsResolver struct {
	constraints platformdomain.MembershipConstraints
}

func (m *MembershipConstraintsResolver) RateLimitPerMinute() *int32 {
	return optionalInt32(m.constraints.RateLimitPerMinute)
}

func (m *MembershipConstraintsResolver) RateLimitPerHour() *int32 {
	return optionalInt32(m.constraints.RateLimitPerHour)
}

func (m *MembershipConstraintsResolver) AllowedIPs() []string {
	return append([]string(nil), m.constraints.AllowedIPs...)
}

func (m *MembershipConstraintsResolver) AllowedProjectIDs() []model.ID {
	result := make([]model.ID, len(m.constraints.AllowedProjectIDs))
	for i, projectID := range m.constraints.AllowedProjectIDs {
		result[i] = model.ID(projectID)
	}
	return result
}

func (m *MembershipConstraintsResolver) AllowedTeamIDs() []model.ID {
	result := make([]model.ID, len(m.constraints.AllowedTeamIDs))
	for i, teamID := range m.constraints.AllowedTeamIDs {
		result[i] = model.ID(teamID)
	}
	return result
}

func (m *MembershipConstraintsResolver) AllowDelegatedRouting() bool {
	return m.constraints.AllowDelegatedRouting
}

func (m *MembershipConstraintsResolver) DelegatedRoutingTeamIDs() []model.ID {
	result := make([]model.ID, len(m.constraints.DelegatedRoutingTeamIDs))
	for i, teamID := range m.constraints.DelegatedRoutingTeamIDs {
		result[i] = model.ID(teamID)
	}
	return result
}

func (m *MembershipConstraintsResolver) ActiveHoursStart() *string {
	return optionalTrimmedString(m.constraints.ActiveHoursStart)
}

func (m *MembershipConstraintsResolver) ActiveHoursEnd() *string {
	return optionalTrimmedString(m.constraints.ActiveHoursEnd)
}

func (m *MembershipConstraintsResolver) ActiveTimezone() *string {
	return optionalTrimmedString(m.constraints.ActiveTimezone)
}

func (m *MembershipConstraintsResolver) ActiveDays() []int32 {
	result := make([]int32, len(m.constraints.ActiveDays))
	for i, day := range m.constraints.ActiveDays {
		result[i] = int32(day)
	}
	return result
}

// =============================================================================
// Admin Type Resolvers
// =============================================================================

// AdminUserResolver resolves AdminUser fields
type AdminUserResolver struct {
	user           *platformdomain.User
	workspaceCount int
	r              *Resolver
}

// ID returns the user ID
func (u *AdminUserResolver) ID() model.ID {
	return model.ID(u.user.ID)
}

// Email returns the email
func (u *AdminUserResolver) Email() string {
	return u.user.Email
}

// Name returns the name
func (u *AdminUserResolver) Name() string {
	return u.user.Name
}

// AvatarURL returns the avatar URL
func (u *AdminUserResolver) AvatarURL() *string {
	if u.user.Avatar == "" {
		return nil
	}
	return &u.user.Avatar
}

// InstanceRole returns the instance role
func (u *AdminUserResolver) InstanceRole() *string {
	if u.user.InstanceRole == nil {
		return nil
	}
	role := string(*u.user.InstanceRole)
	return &role
}

// IsActive returns whether the user is active
func (u *AdminUserResolver) IsActive() bool {
	return u.user.IsActive
}

// EmailVerified returns whether the email is verified
func (u *AdminUserResolver) EmailVerified() bool {
	return u.user.EmailVerified
}

// CreatedAt returns the creation timestamp
func (u *AdminUserResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: u.user.CreatedAt}
}

// UpdatedAt returns the update timestamp
func (u *AdminUserResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: u.user.UpdatedAt}
}

// WorkspaceCount returns the workspace count
func (u *AdminUserResolver) WorkspaceCount() int32 {
	return int32(u.workspaceCount)
}

// AdminUserConnectionResolver resolves AdminUserConnection
type AdminUserConnectionResolver struct {
	users []*platformservices.UserWithStats
	r     *Resolver
}

// Edges returns the user edges
func (c *AdminUserConnectionResolver) Edges() []*AdminUserEdgeResolver {
	edges := make([]*AdminUserEdgeResolver, len(c.users))
	for i, user := range c.users {
		edges[i] = &AdminUserEdgeResolver{user: user, r: c.r}
	}
	return edges
}

// PageInfo returns pagination info
func (c *AdminUserConnectionResolver) PageInfo() *PageInfoResolver {
	return &PageInfoResolver{}
}

// TotalCount returns the total count
func (c *AdminUserConnectionResolver) TotalCount() int32 {
	return int32(len(c.users))
}

// AdminUserEdgeResolver resolves AdminUserEdge
type AdminUserEdgeResolver struct {
	user *platformservices.UserWithStats
	r    *Resolver
}

// Node returns the admin user
func (e *AdminUserEdgeResolver) Node() *AdminUserResolver {
	return &AdminUserResolver{
		user:           e.user.User,
		workspaceCount: e.user.WorkspaceCount,
		r:              e.r,
	}
}

// Cursor returns the cursor
func (e *AdminUserEdgeResolver) Cursor() string {
	return e.user.ID
}

// AdminUserWithWorkspacesResolver resolves AdminUserWithWorkspaces
type AdminUserWithWorkspacesResolver struct {
	data *platformservices.UserWithWorkspaces
	r    *Resolver
}

// User returns the admin user
func (r *AdminUserWithWorkspacesResolver) User() *AdminUserResolver {
	return &AdminUserResolver{
		user:           r.data.User,
		workspaceCount: len(r.data.Workspaces),
		r:              r.r,
	}
}

// Workspaces returns the workspace role info list
func (r *AdminUserWithWorkspacesResolver) Workspaces() []*WorkspaceRoleInfoResolver {
	result := make([]*WorkspaceRoleInfoResolver, len(r.data.Workspaces))
	for i, ws := range r.data.Workspaces {
		result[i] = &WorkspaceRoleInfoResolver{info: ws, r: r.r}
	}
	return result
}

// WorkspaceRoleInfoResolver resolves WorkspaceRoleInfo
type WorkspaceRoleInfoResolver struct {
	info *platformservices.WorkspaceRoleInfo
	r    *Resolver
}

// Workspace returns the workspace
func (r *WorkspaceRoleInfoResolver) Workspace() *WorkspaceResolver {
	return &WorkspaceResolver{workspace: r.info.Workspace, r: r.r}
}

// Role returns the role
func (r *WorkspaceRoleInfoResolver) Role() string {
	return string(r.info.Role)
}

// JoinedAt returns when the user joined
func (r *WorkspaceRoleInfoResolver) JoinedAt() string {
	return r.info.JoinedAt
}

// PageInfoResolver resolves PageInfo
type PageInfoResolver struct {
	hasNextPage     bool
	hasPreviousPage bool
	startCursor     *string
	endCursor       *string
}

// HasNextPage returns whether there's a next page
func (p *PageInfoResolver) HasNextPage() bool {
	return p.hasNextPage
}

// HasPreviousPage returns whether there's a previous page
func (p *PageInfoResolver) HasPreviousPage() bool {
	return p.hasPreviousPage
}

// StartCursor returns the start cursor
func (p *PageInfoResolver) StartCursor() *string {
	return p.startCursor
}

// EndCursor returns the end cursor
func (p *PageInfoResolver) EndCursor() *string {
	return p.endCursor
}

// =============================================================================
// Helper Converters
// =============================================================================

func userToModel(u *platformdomain.User) *model.User {
	if u == nil {
		return nil
	}
	var avatarURL *string
	if u.Avatar != "" {
		avatarURL = &u.Avatar
	}
	return &model.User{
		ID:        model.ID(u.ID),
		Email:     u.Email,
		Name:      u.Name,
		AvatarURL: avatarURL,
	}
}

func contactToModel(c *platformdomain.Contact) *model.Contact {
	if c == nil {
		return nil
	}
	var name *string
	if c.Name != "" {
		name = &c.Name
	}
	return &model.Contact{
		ID:    model.ID(c.ID),
		Email: c.Email,
		Name:  name,
	}
}
