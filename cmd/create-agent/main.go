package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

func main() {
	// Parse flags
	workspaceSlug := flag.String("workspace", "", "Workspace slug (required)")
	agentName := flag.String("name", "", "Agent name (required)")
	agentDesc := flag.String("description", "", "Agent description")
	ownerEmail := flag.String("owner", "", "Owner email (required)")
	tokenName := flag.String("token-name", "default", "Token name")
	permissions := flag.String("permissions", "case:read,case:write,issue:read", "Comma-separated permissions")
	expiresIn := flag.Duration("expires", 0, "Token expiry duration (e.g., 720h for 30 days, 0 for no expiry)")
	apiEndpoint := flag.String("api-endpoint", "", "API endpoint for generated examples (e.g., https://api.movebigrocks.com)")

	flag.Parse()

	// Initialize logger
	log := logger.New().WithField("cmd", "create-agent")

	// Validate required flags
	if *workspaceSlug == "" || *agentName == "" || *ownerEmail == "" {
		fmt.Fprintln(os.Stderr, "Usage: create-agent -workspace SLUG -name NAME -owner EMAIL")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Required flags:")
		fmt.Fprintln(os.Stderr, "  -workspace    Workspace slug")
		fmt.Fprintln(os.Stderr, "  -name         Agent name")
		fmt.Fprintln(os.Stderr, "  -owner        Owner email address")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Optional flags:")
		fmt.Fprintln(os.Stderr, "  -description    Agent description")
		fmt.Fprintln(os.Stderr, "  -token-name     Token name (default: 'default')")
		fmt.Fprintln(os.Stderr, "  -permissions    Comma-separated permissions (default: case:read,case:write,issue:read)")
		fmt.Fprintln(os.Stderr, "  -expires        Token expiry (e.g., 720h for 30 days, 0 for no expiry)")
		fmt.Fprintln(os.Stderr, "  -api-endpoint   API endpoint for generated examples")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Environment:")
		fmt.Fprintln(os.Stderr, "  DATABASE_DSN - Database connection string")
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config", "error", err)
	}

	// Connect to database
	log.Info("Connecting to database", "driver", cfg.Database.EffectiveDriver(), "dsn", cfg.Database.RedactedDSN())
	store, err := stores.NewStoreFromConfig(cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx := context.Background()

	// Use admin context for cross-tenant access
	err = store.WithAdminContext(ctx, func(ctx context.Context) error {
		// Find workspace
		workspace, err := store.Workspaces().GetWorkspaceBySlug(ctx, *workspaceSlug)
		if err != nil {
			return fmt.Errorf("failed to find workspace: %w", err)
		}
		if workspace == nil {
			return fmt.Errorf("workspace not found: %s", *workspaceSlug)
		}

		// Find owner
		owner, err := store.Users().GetUserByEmail(ctx, *ownerEmail)
		if err != nil {
			return fmt.Errorf("failed to find owner: %w", err)
		}
		if owner == nil {
			return fmt.Errorf("owner not found: %s", *ownerEmail)
		}

		// Set tenant context for RLS
		if err := store.SetTenantContext(ctx, workspace.ID); err != nil {
			return fmt.Errorf("failed to set tenant context: %w", err)
		}

		// Create agent
		agent := &platformdomain.Agent{
			WorkspaceID: workspace.ID,
			Name:        *agentName,
			Description: *agentDesc,
			OwnerID:     owner.ID,
			Status:      platformdomain.AgentStatusActive,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			CreatedByID: owner.ID,
		}

		if err := store.Agents().CreateAgent(ctx, agent); err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		fmt.Printf("Created agent: %s (ID: %s)\n", agent.Name, agent.ID)

		// Generate token
		var expiresAt *time.Time
		if *expiresIn > 0 {
			t := time.Now().Add(*expiresIn)
			expiresAt = &t
		}

		token, plainToken, err := platformdomain.NewAgentToken(agent.ID, *tokenName, owner.ID, expiresAt)
		if err != nil {
			return fmt.Errorf("failed to generate token: %w", err)
		}

		if err := store.Agents().CreateAgentToken(ctx, token); err != nil {
			return fmt.Errorf("failed to create token: %w", err)
		}

		// Create workspace membership
		permList := strings.Split(*permissions, ",")
		for i := range permList {
			permList[i] = strings.TrimSpace(permList[i])
		}

		membership := &platformdomain.WorkspaceMembership{
			WorkspaceID:   workspace.ID,
			PrincipalID:   agent.ID,
			PrincipalType: platformdomain.PrincipalTypeAgent,
			Role:          "agent",
			Permissions:   permList,
			GrantedByID:   owner.ID,
			GrantedAt:     time.Now(),
		}

		if err := store.Agents().CreateWorkspaceMembership(ctx, membership); err != nil {
			return fmt.Errorf("failed to create membership: %w", err)
		}

		fmt.Printf("Created token: %s\n", *tokenName)
		fmt.Printf("Permissions: %v\n", permList)
		fmt.Println("")
		fmt.Println("=================================================")
		fmt.Println("IMPORTANT: Save this token now - it cannot be shown again!")
		fmt.Println("=================================================")
		fmt.Println("")
		fmt.Printf("  MBR_AGENT_TOKEN=%s\n", plainToken)
		fmt.Println("")
		fmt.Println("=================================================")

		// Show a simple verification example for the GraphQL API
		endpoint := "https://api.movebigrocks.com"
		if *apiEndpoint != "" {
			endpoint = *apiEndpoint
		}
		fmt.Println("")
		fmt.Println("To verify the token against GraphQL:")
		fmt.Println("")
		fmt.Printf("  curl -sS %s/graphql \\\n", endpoint)
		fmt.Printf("    -H \"Authorization: Bearer %s\" \\\n", plainToken)
		fmt.Printf("    -H \"Content-Type: application/json\" \\\n")
		fmt.Printf("    -d '{\"query\":\"query { __typename }\"}'\n")
		fmt.Println("")

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
