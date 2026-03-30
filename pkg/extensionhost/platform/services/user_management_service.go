package platformservices

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

// UserManagementService handles user CRUD operations for instance admins
type UserManagementService struct {
	userStore      shared.UserStore
	workspaceStore shared.WorkspaceStore
}

// NewUserManagementService creates a new user management service
func NewUserManagementService(userStore shared.UserStore, workspaceStore shared.WorkspaceStore) *UserManagementService {
	return &UserManagementService{
		userStore:      userStore,
		workspaceStore: workspaceStore,
	}
}

// ListAllUsers lists all users in the instance with additional stats
func (s *UserManagementService) ListAllUsers(ctx context.Context) ([]*UserWithStats, error) {
	users, err := s.userStore.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	result := make([]*UserWithStats, 0, len(users))
	for _, user := range users {
		stats := &UserWithStats{
			User: user,
		}

		// Get workspace count
		roles, err := s.workspaceStore.GetUserWorkspaceRoles(ctx, user.ID)
		if err == nil {
			activeCount := 0
			for _, role := range roles {
				if role.IsActive() {
					activeCount++
				}
			}
			stats.WorkspaceCount = activeCount
		}

		result = append(result, stats)
	}

	return result, nil
}

// CreateUser creates a new user with optional instance role
func (s *UserManagementService) CreateUser(ctx context.Context, email, name string, instanceRole *platformdomain.InstanceRole) (*platformdomain.User, error) {
	// Check if user with email already exists
	existing, err := s.userStore.GetUserByEmail(ctx, email)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("user with email '%s' already exists", email)
	}

	user, err := platformdomain.NewManagedUser(email, name, instanceRole, time.Now())
	if err != nil {
		return nil, err
	}

	if err := s.userStore.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// UpdateUser updates an existing user
func (s *UserManagementService) UpdateUser(ctx context.Context, id, email, name string, instanceRole *platformdomain.InstanceRole, isActive, emailVerified bool) error {
	user, err := s.userStore.GetUser(ctx, id)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// If email changed, validate it's unique
	if email != user.Email {
		existing, err := s.userStore.GetUserByEmail(ctx, email)
		if err == nil && existing != nil && existing.ID != id {
			return fmt.Errorf("user with email '%s' already exists", email)
		}
	}

	if err := user.UpdateManagedProfile(email, name, instanceRole, isActive, emailVerified, time.Now()); err != nil {
		return err
	}

	if err := s.userStore.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// DeleteUser deletes a user and all associated data
func (s *UserManagementService) DeleteUser(ctx context.Context, id string) error {
	user, err := s.userStore.GetUser(ctx, id)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Prevent deleting last super admin
	if user.IsSuperAdmin() {
		allUsers, err := s.userStore.ListUsers(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate deletion: %w", err)
		}

		if err := platformdomain.EnsureAnotherActiveSuperAdmin(allUsers, id, "delete"); err != nil {
			return err
		}
	}

	if err := s.userStore.DeleteUser(ctx, id); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// ToggleUserStatus activates or deactivates a user
func (s *UserManagementService) ToggleUserStatus(ctx context.Context, id string, isActive bool) error {
	user, err := s.userStore.GetUser(ctx, id)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Prevent deactivating last super admin
	if !isActive && user.IsSuperAdmin() {
		allUsers, err := s.userStore.ListUsers(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate status change: %w", err)
		}

		if err := platformdomain.EnsureAnotherActiveSuperAdmin(allUsers, id, "deactivate"); err != nil {
			return err
		}
	}

	user.IsActive = isActive
	user.UpdatedAt = time.Now()

	if err := s.userStore.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	return nil
}

// GetUser gets a user by ID
func (s *UserManagementService) GetUser(ctx context.Context, id string) (*platformdomain.User, error) {
	user, err := s.userStore.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return user, nil
}

// GetUserWorkspaces gets all workspaces a user belongs to
func (s *UserManagementService) GetUserWorkspaces(ctx context.Context, id string) ([]*WorkspaceRoleInfo, error) {
	roles, err := s.workspaceStore.GetUserWorkspaceRoles(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user workspaces: %w", err)
	}

	workspaces := make([]*WorkspaceRoleInfo, 0, len(roles))
	for _, role := range roles {
		if !role.IsActive() {
			continue
		}

		workspace, err := s.workspaceStore.GetWorkspace(ctx, role.WorkspaceID)
		if err != nil {
			continue // Skip if workspace not found
		}

		workspaces = append(workspaces, &WorkspaceRoleInfo{
			Workspace: workspace,
			Role:      role.Role,
			JoinedAt:  role.CreatedAt.Format("2006-01-02"),
		})
	}

	return workspaces, nil
}

// GetUserWithWorkspaces gets a user with their workspace memberships
func (s *UserManagementService) GetUserWithWorkspaces(ctx context.Context, id string) (*UserWithWorkspaces, error) {
	user, err := s.userStore.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	roles, err := s.workspaceStore.GetUserWorkspaceRoles(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user workspaces: %w", err)
	}

	workspaces := make([]*WorkspaceRoleInfo, 0, len(roles))
	for _, role := range roles {
		if !role.IsActive() {
			continue
		}

		workspace, err := s.workspaceStore.GetWorkspace(ctx, role.WorkspaceID)
		if err != nil {
			continue // Skip if workspace not found
		}

		workspaces = append(workspaces, &WorkspaceRoleInfo{
			Workspace: workspace,
			Role:      role.Role,
			JoinedAt:  role.CreatedAt.Format("2006-01-02"),
		})
	}

	return &UserWithWorkspaces{
		User:       user,
		Workspaces: workspaces,
	}, nil
}

// UserWithStats contains user data with additional statistics
type UserWithStats struct {
	*platformdomain.User
	WorkspaceCount int
}

// UserWithWorkspaces contains user data with their workspace memberships
type UserWithWorkspaces struct {
	*platformdomain.User
	Workspaces []*WorkspaceRoleInfo
}

// WorkspaceRoleInfo contains workspace info with user's role
type WorkspaceRoleInfo struct {
	Workspace *platformdomain.Workspace
	Role      platformdomain.WorkspaceRole
	JoinedAt  string
}

// ListUsers returns all users (without stats, for simple listings)
func (s *UserManagementService) ListUsers(ctx context.Context) ([]*platformdomain.User, error) {
	return s.userStore.ListUsers(ctx)
}

// GetUsersByIDs returns multiple users by their IDs in a single batch query
func (s *UserManagementService) GetUsersByIDs(ctx context.Context, ids []string) ([]*platformdomain.User, error) {
	if len(ids) == 0 {
		return []*platformdomain.User{}, nil
	}
	return s.userStore.GetUsersByIDs(ctx, ids)
}
