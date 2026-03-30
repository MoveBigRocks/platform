package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

func main() {
	email := flag.String("email", "", "Admin user email (required)")
	name := flag.String("name", "", "Admin user name (defaults to email prefix)")
	role := flag.String("role", "super_admin", "Instance role: super_admin, admin, operator")
	flag.Parse()

	// Initialize logger
	log := logger.New().WithField("cmd", "create-admin")

	if *email == "" {
		fmt.Println("Usage: create-admin -email <email> [-name <name>] [-role <role>]")
		fmt.Println("\nOptions:")
		fmt.Println("  -email    Admin user email (required)")
		fmt.Println("  -name     Admin user name (defaults to email prefix)")
		fmt.Println("  -role     Instance role: super_admin, admin, operator (default: super_admin)")
		fmt.Println("\nEnvironment:")
		fmt.Println("  DATABASE_DSN - Database connection string")
		os.Exit(1)
	}

	// Default name to email prefix
	userName := *name
	if userName == "" {
		for i, c := range *email {
			if c == '@' {
				userName = (*email)[:i]
				break
			}
		}
	}

	// Validate role
	var instanceRole platformdomain.InstanceRole
	switch *role {
	case "super_admin":
		instanceRole = platformdomain.InstanceRoleSuperAdmin
	case "admin":
		instanceRole = platformdomain.InstanceRoleAdmin
	case "operator":
		instanceRole = platformdomain.InstanceRoleOperator
	default:
		log.Error("Invalid role (must be super_admin, admin, or operator)", "role", *role)
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config", "error", err)
	}

	// Initialize store from database config
	log.Info("Connecting to database", "driver", cfg.Database.EffectiveDriver(), "dsn", cfg.Database.RedactedDSN())
	store, err := stores.NewStoreFromConfig(cfg.Database)
	if err != nil {
		log.Fatal("Failed to initialize store", "error", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Check if user already exists
	existing, err := store.Users().GetUserByEmail(ctx, *email)
	if err == nil && existing != nil {
		log.Info("User with email already exists",
			"email", *email,
			"id", existing.ID,
			"name", existing.Name,
			"is_active", existing.IsActive,
			"email_verified", existing.EmailVerified)
		if existing.InstanceRole != nil {
			log.Info("Existing user instance role", "role", *existing.InstanceRole)
		} else {
			log.Info("Existing user has no instance role (regular user)")
		}
		// Still ensure a default workspace exists
		ensureDefaultWorkspace(ctx, store, log)
		os.Exit(0)
	}

	// Create admin user
	adminUser := &platformdomain.User{
		Email:         *email,
		Name:          userName,
		InstanceRole:  &instanceRole,
		IsActive:      true,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := store.Users().CreateUser(ctx, adminUser); err != nil {
		log.Error("Failed to create admin user", "error", err)
		os.Exit(1)
	}

	log.Info("Created admin user successfully",
		"id", adminUser.ID,
		"email", adminUser.Email,
		"name", adminUser.Name,
		"instance_role", instanceRole)

	fmt.Printf("\n✓ Created admin user successfully!\n")
	fmt.Printf("  ID: %s\n", adminUser.ID)
	fmt.Printf("  Email: %s\n", adminUser.Email)
	fmt.Printf("  Name: %s\n", adminUser.Name)
	fmt.Printf("  Instance Role: %s\n", instanceRole)

	// Auto-create a default workspace if none exist
	ensureDefaultWorkspace(ctx, store, log)

	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Request a magic link: POST /auth/magic-link with {\"email\": \"%s\"}\n", *email)
	fmt.Printf("  2. Check your email for the magic link\n")
	fmt.Printf("  3. Click the link to login to the admin panel\n")
}

func ensureDefaultWorkspace(ctx context.Context, store stores.Store, log *logger.Logger) {
	workspaces, err := store.Workspaces().ListWorkspaces(ctx)
	if err != nil {
		log.Error("Failed to list workspaces", "error", err)
		return
	}
	if len(workspaces) > 0 {
		log.Info("Workspaces already exist, skipping default creation", "count", len(workspaces))
		return
	}

	now := time.Now()
	ws := &platformdomain.Workspace{
		Name:      "Default",
		Slug:      "default",
		ShortCode: "DF",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.Workspaces().CreateWorkspace(ctx, ws); err != nil {
		log.Error("Failed to create default workspace", "error", err)
		return
	}

	fmt.Printf("\n✓ Created default workspace\n")
	fmt.Printf("  ID: %s\n", ws.ID)
	fmt.Printf("  Name: %s\n", ws.Name)
	fmt.Printf("  Short Code: %s\n", ws.ShortCode)
}
