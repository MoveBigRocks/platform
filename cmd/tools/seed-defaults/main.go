// seed-defaults seeds default automation rules for all existing workspaces.
// This is useful for production environments where workspaces were created
// before the default rules feature was added.
//
// The seeder is idempotent - it will skip rules that already exist.
//
// Usage: go run cmd/tools/seed-defaults/main.go [-workspace <id>]
//
// Options:
//
//	-workspace  Seed only a specific workspace (optional, seeds all if not specified)
//
// Environment:
//
//	DATABASE_DSN - Database connection string
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	"github.com/movebigrocks/platform/pkg/logger"
)

func main() {
	workspaceID := flag.String("workspace", "", "Seed only a specific workspace (optional)")
	flag.Parse()

	log := logger.New().WithField("cmd", "seed-defaults")

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

	// Create seeder
	seeder := automationservices.NewDefaultRulesSeeder(store.Rules())

	var workspacesToSeed []string

	if *workspaceID != "" {
		// Seed specific workspace
		workspace, err := store.Workspaces().GetWorkspace(ctx, *workspaceID)
		if err != nil {
			log.Error("Workspace not found", "workspace_id", *workspaceID, "error", err)
			os.Exit(1)
		}
		workspacesToSeed = append(workspacesToSeed, workspace.ID)
		log.Info("Seeding specific workspace", "workspace_id", workspace.ID, "name", workspace.Name)
	} else {
		// Get all workspaces
		workspaces, err := store.Workspaces().ListWorkspaces(ctx)
		if err != nil {
			log.Error("Failed to list workspaces", "error", err)
			os.Exit(1)
		}

		if len(workspaces) == 0 {
			fmt.Println("No workspaces found. Nothing to seed.")
			os.Exit(0)
		}

		for _, ws := range workspaces {
			workspacesToSeed = append(workspacesToSeed, ws.ID)
		}
		log.Info("Found workspaces to seed", "count", len(workspacesToSeed))
	}

	// Seed each workspace
	totalCreated := 0
	for _, wsID := range workspacesToSeed {
		workspace, _ := store.Workspaces().GetWorkspace(ctx, wsID) //nolint:errcheck
		wsName := wsID
		if workspace != nil {
			wsName = workspace.Name
		}

		rules, err := seeder.SeedDefaultRules(ctx, wsID)
		if err != nil {
			log.Error("Failed to seed workspace", "workspace_id", wsID, "name", wsName, "error", err)
			continue
		}

		if len(rules) > 0 {
			log.Info("Seeded workspace", "workspace_id", wsID, "name", wsName, "rules_created", len(rules))
			totalCreated += len(rules)
		} else {
			log.Info("Workspace already has all default rules", "workspace_id", wsID, "name", wsName)
		}
	}

	fmt.Printf("\n✓ Seeding complete!\n")
	fmt.Printf("  Workspaces processed: %d\n", len(workspacesToSeed))
	fmt.Printf("  Rules created: %d\n", totalCreated)

	if totalCreated == 0 {
		fmt.Println("\nAll workspaces already have default rules.")
	}
}
