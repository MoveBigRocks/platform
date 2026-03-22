package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// This script creates test data for testing the context-aware security foundation
func main() {
	// Initialize logger
	log := logger.New().WithField("cmd", "seed-security-test")

	log.Info("Seeding security test data...")

	// Get storage path from environment or use default
	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		storagePath = "./data"
	}
	log.Info("Using filesystem storage", "path", storagePath)

	// Initialize store
	store, err := stores.NewStore(storagePath)
	if err != nil {
		log.Fatal("Failed to initialize store", "error", err)
	}

	ctx := context.Background()

	// Create instance super admin user
	superAdminRole := platformdomain.InstanceRoleSuperAdmin
	adminUser := &platformdomain.User{
		Email:         "admin@mbr.local",
		Name:          "Super Admin",
		InstanceRole:  &superAdminRole,
		IsActive:      true,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := store.Users().CreateUser(ctx, adminUser); err != nil {
		log.Error("Failed to create admin user", "error", err)
		os.Exit(1)
	}
	log.Info("Created instance super admin user", "email", adminUser.Email, "id", adminUser.ID)

	// Create test workspaces
	workspace1 := &platformdomain.Workspace{
		Name:        "Demo Workspace",
		Slug:        "demo",
		Description: "Test workspace for demo",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := store.Workspaces().CreateWorkspace(ctx, workspace1); err != nil {
		log.Error("Failed to create workspace 1", "error", err)
		os.Exit(1)
	}
	log.Info("Created workspace", "name", workspace1.Name, "id", workspace1.ID)

	workspace2 := &platformdomain.Workspace{
		Name:        "Support Team",
		Slug:        "support",
		Description: "Support team workspace",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := store.Workspaces().CreateWorkspace(ctx, workspace2); err != nil {
		log.Error("Failed to create workspace 2", "error", err)
		os.Exit(1)
	}
	log.Info("Created workspace", "name", workspace2.Name, "id", workspace2.ID)

	// Create regular user with workspace roles
	regularUser := &platformdomain.User{
		Email:         "john@example.com",
		Name:          "John Doe",
		InstanceRole:  nil, // No instance role - regular workspace user
		IsActive:      true,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := store.Users().CreateUser(ctx, regularUser); err != nil {
		log.Error("Failed to create regular user", "error", err)
		os.Exit(1)
	}
	log.Info("Created regular user", "email", regularUser.Email, "id", regularUser.ID)

	// Assign regular user as owner of workspace1
	userWorkspaceRole1 := &platformdomain.UserWorkspaceRole{
		UserID:      regularUser.ID,
		WorkspaceID: workspace1.ID,
		Role:        platformdomain.WorkspaceRoleOwner,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := store.Workspaces().CreateUserWorkspaceRole(ctx, userWorkspaceRole1); err != nil {
		log.Error("Failed to create user workspace role 1", "error", err)
		os.Exit(1)
	}
	log.Info("Assigned user to workspace",
		"email", regularUser.Email,
		"role", userWorkspaceRole1.Role,
		"workspace", workspace1.Name)

	// Assign regular user as member of workspace2
	userWorkspaceRole2 := &platformdomain.UserWorkspaceRole{
		UserID:      regularUser.ID,
		WorkspaceID: workspace2.ID,
		Role:        platformdomain.WorkspaceRoleMember,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := store.Workspaces().CreateUserWorkspaceRole(ctx, userWorkspaceRole2); err != nil {
		log.Error("Failed to create user workspace role 2", "error", err)
		os.Exit(1)
	}
	log.Info("Assigned user to workspace",
		"email", regularUser.Email,
		"role", userWorkspaceRole2.Role,
		"workspace", workspace2.Name)

	// Create hybrid user (instance admin + workspace roles)
	instanceAdminRole := platformdomain.InstanceRoleAdmin
	hybridUser := &platformdomain.User{
		Email:         "hybrid@mbr.local",
		Name:          "Hybrid Admin",
		InstanceRole:  &instanceAdminRole,
		IsActive:      true,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := store.Users().CreateUser(ctx, hybridUser); err != nil {
		log.Error("Failed to create hybrid user", "error", err)
		os.Exit(1)
	}
	log.Info("Created hybrid user", "email", hybridUser.Email, "id", hybridUser.ID)

	// Assign hybrid user as admin of workspace1
	hybridWorkspaceRole := &platformdomain.UserWorkspaceRole{
		UserID:      hybridUser.ID,
		WorkspaceID: workspace1.ID,
		Role:        platformdomain.WorkspaceRoleAdmin,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := store.Workspaces().CreateUserWorkspaceRole(ctx, hybridWorkspaceRole); err != nil {
		log.Error("Failed to create hybrid workspace role", "error", err)
		os.Exit(1)
	}
	log.Info("Assigned user to workspace",
		"email", hybridUser.Email,
		"role", hybridWorkspaceRole.Role,
		"workspace", workspace1.Name)

	log.Info("Security test data seeded successfully!")

	fmt.Println("\nTest Users:")
	fmt.Printf("  1. Super Admin: %s (instance admin-only access)\n", adminUser.Email)
	fmt.Printf("  2. Regular User: %s (2 workspaces: owner + member)\n", regularUser.Email)
	fmt.Printf("  3. Hybrid Admin: %s (instance admin + 1 workspace)\n", hybridUser.Email)
	fmt.Println("\nTest Workspaces:")
	fmt.Printf("  1. %s (slug: %s, id: %s)\n", workspace1.Name, workspace1.Slug, workspace1.ID)
	fmt.Printf("  2. %s (slug: %s, id: %s)\n", workspace2.Name, workspace2.Slug, workspace2.ID)
	fmt.Println("\nTo test:")
	fmt.Println("  1. Request magic link for admin@mbr.local -> Should default to instance admin context")
	fmt.Println("  2. Request magic link for john@example.com -> Should default to Demo Workspace (owner)")
	fmt.Println("  3. Request magic link for hybrid@mbr.local -> Should default to instance admin context (super admin priority)")
	fmt.Println("  4. Use /auth/switch-context to switch between contexts")
	fmt.Println("  5. Verify admin.* subdomain routes require instance admin context")
	fmt.Println("  6. Verify /app/:workspace_id/* routes require matching workspace context")

	os.Exit(0)
}
