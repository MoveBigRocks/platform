// create-test-project creates a test error monitoring project and outputs the DSN.
//
// Usage: go run cmd/tools/create-test-project/main.go [workspace-id]
//
// If workspace-id is not provided, it will list available workspaces.
//
// Environment:
//
//	DATABASE_DSN - Database connection string
//	SENTRY_HOST - Host for DSN (default: api.movebigrocks.com)
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

func main() {
	log := logger.New().WithField("cmd", "create-test-project")

	sentryHost := os.Getenv("SENTRY_HOST")
	if sentryHost == "" {
		sentryHost = "api.movebigrocks.com"
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

	// If no args, list workspaces
	if len(os.Args) < 2 {
		fmt.Println("Available workspaces:")
		listWorkspaces(ctx, store)
		fmt.Println("\nUsage: go run cmd/tools/create-test-project/main.go <workspace-id>")
		return
	}

	workspaceID := os.Args[1]

	// Verify workspace exists
	workspace, err := store.Workspaces().GetWorkspace(ctx, workspaceID)
	if err != nil {
		log.Fatal("Failed to get workspace", "error", err)
		return
	}
	if workspace == nil {
		log.Fatal("Workspace not found", "workspace_id", workspaceID)
		return
	}

	// Create a test project
	project := observabilitydomain.NewProject(
		workspaceID,
		"",                 // No team
		"Test Application", // Name
		"test-app",         // Slug
		"python",           // Platform
	)

	// Override DSN with the actual host and numeric project number expected by the SDK.
	// Modern Sentry SDKs validate that the project ID is numeric
	project.DSN = fmt.Sprintf("https://%s@%s/%d",
		project.PublicKey,
		sentryHost,
		project.ProjectNumber,
	)

	// Save project to database
	if err := store.Projects().CreateProject(ctx, project); err != nil {
		log.Fatal("Failed to create project", "error", err)
	}

	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("TEST PROJECT CREATED")
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Printf("\nProject ID:      %s\n", project.ID)
	fmt.Printf("Project Number:  %d\n", project.ProjectNumber)
	fmt.Printf("Name:            %s\n", project.Name)
	fmt.Printf("Workspace:       %s (%s)\n", workspace.Name, workspaceID)
	fmt.Printf("Public Key:      %s\n", project.PublicKey)
	fmt.Printf("\nDSN:\n  %s\n", project.DSN)
	fmt.Println("\nTo test with Python Sentry SDK:")
	fmt.Println("  pip install sentry-sdk")
	fmt.Println("  python -c \"")
	fmt.Println("import sentry_sdk")
	fmt.Printf("sentry_sdk.init(dsn='%s')\n", project.DSN)
	fmt.Println("sentry_sdk.capture_message('Test from Python')")
	fmt.Println("\"")
}

func listWorkspaces(ctx context.Context, store stores.Store) {
	workspaces, err := store.Workspaces().ListWorkspaces(ctx)
	if err != nil {
		fmt.Printf("  Error reading workspaces: %v\n", err)
		return
	}

	if len(workspaces) == 0 {
		fmt.Println("  No workspaces found")
		return
	}

	for _, ws := range workspaces {
		fmt.Printf("  %s - %s (%s)\n", ws.ID, ws.Name, ws.Slug)
	}
}
