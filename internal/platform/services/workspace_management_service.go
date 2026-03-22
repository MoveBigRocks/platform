package platformservices

import (
	"context"
	"fmt"
	"time"

	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// WorkspaceManagementService handles workspace CRUD operations for instance admins
type WorkspaceManagementService struct {
	workspaceStore shared.WorkspaceStore
	caseStore      shared.CaseStore
	userStore      shared.UserStore
	issueChecker   WorkspaceIssueChecker
	rulesSeeder    *automationservices.DefaultRulesSeeder
	logger         *logger.Logger
}

type WorkspaceIssueChecker interface {
	CountOpenWorkspaceIssues(ctx context.Context, workspaceID string) (int, error)
}

// NewWorkspaceManagementService creates a new workspace management service
func NewWorkspaceManagementService(
	workspaceStore shared.WorkspaceStore,
	caseStore shared.CaseStore,
	userStore shared.UserStore,
	ruleStore shared.RuleStore,
) *WorkspaceManagementService {
	return &WorkspaceManagementService{
		workspaceStore: workspaceStore,
		caseStore:      caseStore,
		userStore:      userStore,
		rulesSeeder:    automationservices.NewDefaultRulesSeeder(ruleStore),
		logger:         logger.New().WithField("service", "workspace-management"),
	}
}

func (s *WorkspaceManagementService) SetIssueChecker(checker WorkspaceIssueChecker) {
	s.issueChecker = checker
}

// ListAllWorkspaces lists all workspaces in the instance with additional stats
func (s *WorkspaceManagementService) ListAllWorkspaces(ctx context.Context) ([]*WorkspaceWithStats, error) {
	workspaces, err := s.workspaceStore.ListWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}

	result := make([]*WorkspaceWithStats, 0, len(workspaces))
	for _, ws := range workspaces {
		stats := &WorkspaceWithStats{
			Workspace: ws,
		}

		// Get member count
		members, err := s.workspaceStore.GetWorkspaceUsers(ctx, ws.ID)
		if err == nil {
			stats.MemberCount = len(members)
		}

		// Get case count from case store
		caseCount, err := s.caseStore.GetCaseCount(ctx, ws.ID, shared.CaseFilter{})
		if err == nil {
			stats.CaseCount = caseCount
		}

		result = append(result, stats)
	}

	return result, nil
}

// GetWorkspace gets a workspace by ID
func (s *WorkspaceManagementService) GetWorkspace(ctx context.Context, id string) (*platformdomain.Workspace, error) {
	workspace, err := s.workspaceStore.GetWorkspace(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("workspace not found: %w", err)
	}
	return workspace, nil
}

// GetWorkspaceBySlug gets a workspace by slug
func (s *WorkspaceManagementService) GetWorkspaceBySlug(ctx context.Context, slug string) (*platformdomain.Workspace, error) {
	workspace, err := s.workspaceStore.GetWorkspaceBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("workspace not found: %w", err)
	}
	return workspace, nil
}

// CreateWorkspace creates a new workspace
func (s *WorkspaceManagementService) CreateWorkspace(ctx context.Context, name, slug, description string) (*platformdomain.Workspace, error) {
	// Validate slug is unique
	existing, err := s.workspaceStore.GetWorkspaceBySlug(ctx, slug)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("workspace with slug '%s' already exists", slug)
	}

	workspace := &platformdomain.Workspace{
		CreatedAt: time.Now(),
	}
	workspace.UpdateDetails(name, slug, description, workspace.CreatedAt)

	if workspace.ShortCode == "" {
		workspace.ShortCode = platformdomain.GenerateWorkspaceShortCode(slug)
	}

	if err := s.workspaceStore.CreateWorkspace(ctx, workspace); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	// Create default workspace settings
	settings := platformdomain.NewWorkspaceSettings(workspace.ID)

	if err := s.workspaceStore.CreateWorkspaceSettings(ctx, settings); err != nil {
		// Non-fatal - workspace is created, settings can be created later
		s.logger.Warn("failed to create workspace settings", "error", err)
	}

	// Seed default automation rules
	if s.rulesSeeder != nil {
		rules, err := s.rulesSeeder.SeedDefaultRules(ctx, workspace.ID)
		if err != nil {
			// Non-fatal - workspace is created, rules can be seeded later
			s.logger.Warn("failed to seed default rules", "workspace_id", workspace.ID, "error", err)
		} else {
			s.logger.Info("seeded default automation rules", "workspace_id", workspace.ID, "rule_count", len(rules))
		}
	}

	return workspace, nil
}

// UpdateWorkspace updates an existing workspace
func (s *WorkspaceManagementService) UpdateWorkspace(ctx context.Context, id, name, slug, description string) error {
	workspace, err := s.workspaceStore.GetWorkspace(ctx, id)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	// If slug changed, validate it's unique
	if slug != workspace.Slug {
		existing, err := s.workspaceStore.GetWorkspaceBySlug(ctx, slug)
		if err == nil && existing != nil && existing.ID != id {
			return fmt.Errorf("workspace with slug '%s' already exists", slug)
		}
	}

	workspace.UpdateDetails(name, slug, description, time.Now())

	if err := s.workspaceStore.UpdateWorkspace(ctx, workspace); err != nil {
		return fmt.Errorf("failed to update workspace: %w", err)
	}

	return nil
}

// DeleteWorkspace deletes a workspace and all associated data
func (s *WorkspaceManagementService) DeleteWorkspace(ctx context.Context, id string) error {
	workspace, err := s.workspaceStore.GetWorkspace(ctx, id)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	// Check if workspace has active data (cases, errors, etc.)
	caseCount, err := s.caseStore.GetCaseCount(ctx, id, shared.CaseFilter{})
	if err != nil {
		caseCount = 0
	}

	// Check for active members (besides the owner)
	members, err := s.workspaceStore.GetWorkspaceUsers(ctx, id)
	memberCount := 0
	if err == nil {
		memberCount = len(members)
	}

	// Check for open issues only through the optional observability capability.
	openIssues := 0
	if s.issueChecker != nil {
		if count, err := s.issueChecker.CountOpenWorkspaceIssues(ctx, id); err == nil {
			openIssues = count
		}
	}

	if err := workspace.ValidateDeletion(caseCount, memberCount, openIssues); err != nil {
		return err
	}

	// All checks passed, proceed with deletion
	if err := s.workspaceStore.DeleteWorkspace(ctx, id); err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}

	return nil
}

// AddUserToWorkspace adds a user to a workspace with a specific role
func (s *WorkspaceManagementService) AddUserToWorkspace(ctx context.Context, userID, workspaceID string, role platformdomain.WorkspaceRole) error {
	// Validate user exists
	_, err := s.userStore.GetUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Validate workspace exists
	_, err = s.workspaceStore.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	// Check if user already has a role in this workspace
	roles, err := s.workspaceStore.GetUserWorkspaceRoles(ctx, userID)
	if err == nil {
		for _, r := range roles {
			if r.WorkspaceID == workspaceID && r.IsActive() {
				return fmt.Errorf("user already has a role in this workspace")
			}
		}
	}

	// Create workspace role
	userWorkspaceRole := &platformdomain.UserWorkspaceRole{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Role:        role,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.workspaceStore.CreateUserWorkspaceRole(ctx, userWorkspaceRole); err != nil {
		return fmt.Errorf("failed to add user to workspace: %w", err)
	}

	return nil
}

// RemoveUserFromWorkspace removes a user from a workspace
func (s *WorkspaceManagementService) RemoveUserFromWorkspace(ctx context.Context, userID, workspaceID string) error {
	if err := s.workspaceStore.DeleteUserWorkspaceRole(ctx, userID, workspaceID); err != nil {
		return fmt.Errorf("failed to remove user from workspace: %w", err)
	}

	return nil
}

// WorkspaceWithStats contains workspace data with additional statistics
type WorkspaceWithStats struct {
	*platformdomain.Workspace
	MemberCount int
	CaseCount   int
}

// ListWorkspaces returns all workspaces (without stats, for simple listings)
func (s *WorkspaceManagementService) ListWorkspaces(ctx context.Context) ([]*platformdomain.Workspace, error) {
	return s.workspaceStore.ListWorkspaces(ctx)
}

// GetWorkspacesByIDs returns workspaces by their IDs
func (s *WorkspaceManagementService) GetWorkspacesByIDs(ctx context.Context, ids []string) ([]*platformdomain.Workspace, error) {
	if len(ids) == 0 {
		return []*platformdomain.Workspace{}, nil
	}
	return s.workspaceStore.GetWorkspacesByIDs(ctx, ids)
}

// ListWorkspaceTeams returns all teams for a workspace
func (s *WorkspaceManagementService) ListWorkspaceTeams(ctx context.Context, workspaceID string) ([]*platformdomain.Team, error) {
	return s.workspaceStore.ListWorkspaceTeams(ctx, workspaceID)
}

// GetTeam returns a team by ID.
func (s *WorkspaceManagementService) GetTeam(ctx context.Context, teamID string) (*platformdomain.Team, error) {
	return s.workspaceStore.GetTeam(ctx, teamID)
}

// CreateTeam creates a team within a workspace.
func (s *WorkspaceManagementService) CreateTeam(ctx context.Context, team *platformdomain.Team) error {
	if team == nil {
		return fmt.Errorf("team is required")
	}
	if _, err := s.workspaceStore.GetWorkspace(ctx, team.WorkspaceID); err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	now := time.Now()
	if team.CreatedAt.IsZero() {
		team.CreatedAt = now
	}
	team.UpdatedAt = now

	if err := s.workspaceStore.CreateTeam(ctx, team); err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}
	return nil
}

// AddTeamMember adds a user to a team after validating workspace membership.
func (s *WorkspaceManagementService) AddTeamMember(ctx context.Context, member *platformdomain.TeamMember) error {
	if member == nil {
		return fmt.Errorf("team member is required")
	}
	if _, err := s.userStore.GetUser(ctx, member.UserID); err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	team, err := s.workspaceStore.GetTeam(ctx, member.TeamID)
	if err != nil {
		return fmt.Errorf("team not found: %w", err)
	}
	member.WorkspaceID = team.WorkspaceID

	roles, err := s.workspaceStore.GetUserWorkspaceRoles(ctx, member.UserID)
	if err != nil {
		return fmt.Errorf("failed to load user workspace roles: %w", err)
	}
	allowed := false
	for _, role := range roles {
		if role.WorkspaceID == member.WorkspaceID && role.IsActive() {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("user is not a member of workspace %s", member.WorkspaceID)
	}

	existingMembers, err := s.workspaceStore.GetTeamMembers(ctx, member.WorkspaceID, member.TeamID)
	if err != nil {
		return fmt.Errorf("failed to load existing team members: %w", err)
	}
	for _, existing := range existingMembers {
		if existing.UserID == member.UserID && existing.IsActive {
			return fmt.Errorf("user already belongs to team")
		}
	}

	now := time.Now()
	if member.JoinedAt.IsZero() {
		member.JoinedAt = now
	}
	if member.CreatedAt.IsZero() {
		member.CreatedAt = now
	}
	member.UpdatedAt = now
	member.IsActive = true

	if err := s.workspaceStore.AddTeamMember(ctx, member); err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}
	return nil
}

// GetTeamMembers returns active members for a team.
func (s *WorkspaceManagementService) GetTeamMembers(ctx context.Context, workspaceID, teamID string) ([]*platformdomain.TeamMember, error) {
	return s.workspaceStore.GetTeamMembers(ctx, workspaceID, teamID)
}
