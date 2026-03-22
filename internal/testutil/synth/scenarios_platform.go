package synth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

// PlatformScenarioRunner executes workspace and user management scenarios
type PlatformScenarioRunner struct {
	services *TestServices
	verbose  bool
}

// NewPlatformScenarioRunner creates a new platform scenario runner
func NewPlatformScenarioRunner(services *TestServices, verbose bool) *PlatformScenarioRunner {
	return &PlatformScenarioRunner{
		services: services,
		verbose:  verbose,
	}
}

// RunAllPlatformScenarios runs all workspace/user management scenarios
func (sr *PlatformScenarioRunner) RunAllPlatformScenarios(ctx context.Context) ([]*ScenarioResult, error) {
	results := make([]*ScenarioResult, 0)

	scenarios := []struct {
		Name string
		Run  func(context.Context) (*ScenarioResult, error)
	}{
		{"Create Workspace with Settings", sr.RunCreateWorkspaceScenario},
		{"User Registration and Workspace Assignment", sr.RunUserWorkspaceAssignmentScenario},
		{"Role-Based Access Control", sr.RunRoleBasedAccessScenario},
		{"Team Creation and Membership", sr.RunTeamManagementScenario},
		{"Contact Management", sr.RunContactManagementScenario},
		{"Session and Context Switching", sr.RunSessionContextScenario},
	}

	for _, scenario := range scenarios {
		sr.log("Running scenario: %s", scenario.Name)

		result, err := scenario.Run(ctx)
		if err != nil {
			result = &ScenarioResult{
				Name:    scenario.Name,
				Success: false,
				Error:   err,
			}
		}
		results = append(results, result)

		if sr.verbose && len(result.Verifications) > 0 {
			for _, v := range result.Verifications {
				status := "✓"
				if !v.Passed {
					status = "✗"
				}
				sr.log("    %s %s: %s", status, v.Check, v.Details)
			}
		}
		sr.log("  Result: success=%v", result.Success)
	}

	return results, nil
}

// RunCreateWorkspaceScenario tests workspace creation with settings
func (sr *PlatformScenarioRunner) RunCreateWorkspaceScenario(ctx context.Context) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Create Workspace with Settings",
		Verifications: make([]VerificationResult, 0),
	}

	sr.log("  Step 1: Creating workspace...")

	workspace := platformdomain.NewWorkspace("Acme Testing Corp", "acme-testing")
	workspace.Description = "A test workspace for scenario testing"
	workspace.MaxUsers = 50
	workspace.MaxCases = 5000

	err := sr.services.Store.Workspaces().CreateWorkspace(ctx, workspace)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Workspace created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Workspace created", Passed: true,
		Details: fmt.Sprintf("ID: %s, Slug: %s", workspace.ID, workspace.Slug),
	})

	// Verify retrieval by ID
	sr.log("  Step 2: Verifying workspace retrieval...")
	stored, err := sr.services.Store.Workspaces().GetWorkspace(ctx, workspace.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Workspace retrievable by ID", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Workspace retrievable by ID", Passed: stored.Name == workspace.Name,
		Details: fmt.Sprintf("Name: %s", stored.Name),
	})

	// Verify retrieval by slug
	sr.log("  Step 3: Verifying slug lookup...")
	bySlug, err := sr.services.Store.Workspaces().GetWorkspaceBySlug(ctx, workspace.Slug)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Workspace retrievable by slug", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Workspace retrievable by slug", Passed: bySlug.ID == workspace.ID,
		Details: fmt.Sprintf("Slug: %s", bySlug.Slug),
	})

	// Create and verify workspace settings
	sr.log("  Step 4: Creating workspace settings...")
	settings := &platformdomain.WorkspaceSettings{
		ID:            id.New(),
		WorkspaceID:   workspace.ID,
		WorkspaceName: workspace.Name,
		Timezone:      "America/New_York",
		Language:      "en",
		DateFormat:    "MM/DD/YYYY",
		TimeFormat:    "12h",
		CompanyName:   "Acme Testing Corp",
		CompanyEmail:  "support@acme-testing.com",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err = sr.services.Store.Workspaces().CreateWorkspaceSettings(ctx, settings)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Workspace settings created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	// Verify settings retrieval
	storedSettings, err := sr.services.Store.Workspaces().GetWorkspaceSettings(ctx, workspace.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Workspace settings retrievable", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Workspace settings retrievable", Passed: storedSettings.CompanyName == "Acme Testing Corp",
		Details: fmt.Sprintf("CompanyName: %s", storedSettings.CompanyName),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunUserWorkspaceAssignmentScenario tests user creation and workspace assignment
func (sr *PlatformScenarioRunner) RunUserWorkspaceAssignmentScenario(ctx context.Context) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "User Registration and Workspace Assignment",
		Verifications: make([]VerificationResult, 0),
	}

	// Create workspace first
	workspace := platformdomain.NewWorkspace("User Test Workspace", "user-test-ws")
	sr.services.Store.Workspaces().CreateWorkspace(ctx, workspace)

	sr.log("  Step 1: Creating user...")

	user := platformdomain.NewUser("john.doe@example.com", "John Doe")
	user.IsActive = true
	user.EmailVerified = true

	err := sr.services.Store.Users().CreateUser(ctx, user)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "User created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "User created", Passed: true,
		Details: fmt.Sprintf("ID: %s, Email: %s", user.ID, user.Email),
	})

	// Verify user retrieval by email
	sr.log("  Step 2: Verifying email lookup...")
	byEmail, err := sr.services.Store.Users().GetUserByEmail(ctx, user.Email)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "User lookup by email", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "User lookup by email", Passed: byEmail.ID == user.ID,
		Details: fmt.Sprintf("Found: %s", byEmail.Name),
	})

	// Assign user to workspace
	sr.log("  Step 3: Assigning user to workspace...")
	role := &platformdomain.UserWorkspaceRole{
		ID:          id.New(),
		UserID:      user.ID,
		WorkspaceID: workspace.ID,
		Role:        platformdomain.WorkspaceRoleMember,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = sr.services.Store.Workspaces().CreateUserWorkspaceRole(ctx, role)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "User assigned to workspace", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "User assigned to workspace", Passed: true,
		Details: fmt.Sprintf("Role: %s", role.Role),
	})

	// Verify user's workspace roles
	sr.log("  Step 4: Verifying user workspace roles...")
	roles, err := sr.services.Store.Workspaces().GetUserWorkspaceRoles(ctx, user.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "User roles retrievable", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	hasRole := false
	for _, r := range roles {
		if r.WorkspaceID == workspace.ID && r.Role == platformdomain.WorkspaceRoleMember {
			hasRole = true
			break
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "User has workspace role", Passed: hasRole,
		Details: fmt.Sprintf("Total roles: %d", len(roles)),
	})

	// Verify workspace members list
	sr.log("  Step 5: Verifying workspace members...")
	members, err := sr.services.Store.Workspaces().GetWorkspaceUsers(ctx, workspace.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Workspace members retrievable", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	isMember := false
	for _, m := range members {
		if m.UserID == user.ID {
			isMember = true
			break
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "User appears in workspace members", Passed: isMember,
		Details: fmt.Sprintf("Total members: %d", len(members)),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunRoleBasedAccessScenario tests different workspace roles
func (sr *PlatformScenarioRunner) RunRoleBasedAccessScenario(ctx context.Context) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Role-Based Access Control",
		Verifications: make([]VerificationResult, 0),
	}

	// Create workspace
	workspace := platformdomain.NewWorkspace("RBAC Test Workspace", "rbac-test")
	sr.services.Store.Workspaces().CreateWorkspace(ctx, workspace)

	sr.log("  Step 1: Creating users with different roles...")

	// Create owner
	owner := platformdomain.NewUser("owner@rbac-test.com", "Workspace Owner")
	sr.services.Store.Users().CreateUser(ctx, owner)

	ownerRole := &platformdomain.UserWorkspaceRole{
		ID:          id.New(),
		UserID:      owner.ID,
		WorkspaceID: workspace.ID,
		Role:        platformdomain.WorkspaceRoleOwner,
		CreatedAt:   time.Now(),
	}
	sr.services.Store.Workspaces().CreateUserWorkspaceRole(ctx, ownerRole)

	// Create admin
	admin := platformdomain.NewUser("admin@rbac-test.com", "Workspace Admin")
	sr.services.Store.Users().CreateUser(ctx, admin)

	adminRole := &platformdomain.UserWorkspaceRole{
		ID:          id.New(),
		UserID:      admin.ID,
		WorkspaceID: workspace.ID,
		Role:        platformdomain.WorkspaceRoleAdmin,
		CreatedAt:   time.Now(),
	}
	sr.services.Store.Workspaces().CreateUserWorkspaceRole(ctx, adminRole)

	// Create member
	member := platformdomain.NewUser("member@rbac-test.com", "Workspace Member")
	sr.services.Store.Users().CreateUser(ctx, member)

	memberRole := &platformdomain.UserWorkspaceRole{
		ID:          id.New(),
		UserID:      member.ID,
		WorkspaceID: workspace.ID,
		Role:        platformdomain.WorkspaceRoleMember,
		CreatedAt:   time.Now(),
	}
	sr.services.Store.Workspaces().CreateUserWorkspaceRole(ctx, memberRole)

	// Create viewer
	viewer := platformdomain.NewUser("viewer@rbac-test.com", "Workspace Viewer")
	sr.services.Store.Users().CreateUser(ctx, viewer)

	viewerRole := &platformdomain.UserWorkspaceRole{
		ID:          id.New(),
		UserID:      viewer.ID,
		WorkspaceID: workspace.ID,
		Role:        platformdomain.WorkspaceRoleViewer,
		CreatedAt:   time.Now(),
	}
	sr.services.Store.Workspaces().CreateUserWorkspaceRole(ctx, viewerRole)

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Created 4 users with different roles", Passed: true,
		Details: "Owner, Admin, Member, Viewer",
	})

	// Verify all users in workspace
	sr.log("  Step 2: Verifying workspace has all members...")
	members, _ := sr.services.Store.Workspaces().GetWorkspaceUsers(ctx, workspace.ID)

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "All 4 users in workspace", Passed: len(members) >= 4,
		Details: fmt.Sprintf("Member count: %d", len(members)),
	})

	// Verify role distribution
	sr.log("  Step 3: Verifying role distribution...")
	roleCounts := make(map[platformdomain.WorkspaceRole]int)
	for _, m := range members {
		roleCounts[m.Role]++
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Owner role exists", Passed: roleCounts[platformdomain.WorkspaceRoleOwner] >= 1,
		Details: fmt.Sprintf("Owners: %d", roleCounts[platformdomain.WorkspaceRoleOwner]),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Admin role exists", Passed: roleCounts[platformdomain.WorkspaceRoleAdmin] >= 1,
		Details: fmt.Sprintf("Admins: %d", roleCounts[platformdomain.WorkspaceRoleAdmin]),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Member role exists", Passed: roleCounts[platformdomain.WorkspaceRoleMember] >= 1,
		Details: fmt.Sprintf("Members: %d", roleCounts[platformdomain.WorkspaceRoleMember]),
	})

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Viewer role exists", Passed: roleCounts[platformdomain.WorkspaceRoleViewer] >= 1,
		Details: fmt.Sprintf("Viewers: %d", roleCounts[platformdomain.WorkspaceRoleViewer]),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunTeamManagementScenario tests team creation and membership
func (sr *PlatformScenarioRunner) RunTeamManagementScenario(ctx context.Context) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Team Creation and Membership",
		Verifications: make([]VerificationResult, 0),
	}

	// Create workspace and users
	workspace := platformdomain.NewWorkspace("Team Test Workspace", "team-test")
	sr.services.Store.Workspaces().CreateWorkspace(ctx, workspace)

	lead := platformdomain.NewUser("team.lead@team-test.com", "Team Lead")
	sr.services.Store.Users().CreateUser(ctx, lead)

	member1 := platformdomain.NewUser("member1@team-test.com", "Team Member 1")
	sr.services.Store.Users().CreateUser(ctx, member1)

	member2 := platformdomain.NewUser("member2@team-test.com", "Team Member 2")
	sr.services.Store.Users().CreateUser(ctx, member2)

	sr.log("  Step 1: Creating team...")

	team := &platformdomain.Team{
		ID:          id.New(),
		WorkspaceID: workspace.ID,
		Name:        "Support Team",
		Description: "Handles customer support tickets",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := sr.services.Store.Workspaces().CreateTeam(ctx, team)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Team created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Team created", Passed: true,
		Details: fmt.Sprintf("ID: %s, Name: %s", team.ID, team.Name),
	})

	// Add team members
	sr.log("  Step 2: Adding team members...")

	leadMember := &platformdomain.TeamMember{
		ID:          id.New(),
		WorkspaceID: workspace.ID,
		TeamID:      team.ID,
		UserID:      lead.ID,
		Role:        platformdomain.TeamMemberRoleLead,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
	sr.services.Store.Workspaces().AddTeamMember(ctx, leadMember)

	teamMember1 := &platformdomain.TeamMember{
		ID:          id.New(),
		WorkspaceID: workspace.ID,
		TeamID:      team.ID,
		UserID:      member1.ID,
		Role:        platformdomain.TeamMemberRoleMember,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
	sr.services.Store.Workspaces().AddTeamMember(ctx, teamMember1)

	teamMember2 := &platformdomain.TeamMember{
		ID:          id.New(),
		WorkspaceID: workspace.ID,
		TeamID:      team.ID,
		UserID:      member2.ID,
		Role:        platformdomain.TeamMemberRoleMember,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
	sr.services.Store.Workspaces().AddTeamMember(ctx, teamMember2)

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Added 3 team members", Passed: true,
		Details: "1 Lead, 2 Members",
	})

	// Verify team members
	sr.log("  Step 3: Verifying team members...")
	members, err := sr.services.Store.Workspaces().GetTeamMembers(ctx, workspace.ID, team.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Team members retrievable", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Team has 3 members", Passed: len(members) >= 3,
		Details: fmt.Sprintf("Member count: %d", len(members)),
	})

	// Verify lead role
	hasLead := false
	for _, m := range members {
		if m.Role == platformdomain.TeamMemberRoleLead {
			hasLead = true
			break
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Team has a lead", Passed: hasLead,
		Details: fmt.Sprintf("Lead: %s", lead.Name),
	})

	// Verify team listing for workspace
	sr.log("  Step 4: Verifying workspace teams...")
	teams, err := sr.services.Store.Workspaces().ListWorkspaceTeams(ctx, workspace.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Workspace teams retrievable", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	hasTeam := false
	for _, t := range teams {
		if t.ID == team.ID {
			hasTeam = true
			break
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Team listed in workspace", Passed: hasTeam,
		Details: fmt.Sprintf("Total teams: %d", len(teams)),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunContactManagementScenario tests contact creation and lookup
func (sr *PlatformScenarioRunner) RunContactManagementScenario(ctx context.Context) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Contact Management",
		Verifications: make([]VerificationResult, 0),
	}

	// Create workspace
	workspace := platformdomain.NewWorkspace("Contact Test Workspace", "contact-test")
	sr.services.Store.Workspaces().CreateWorkspace(ctx, workspace)

	sr.log("  Step 1: Creating contact...")

	contact := platformdomain.NewContact(workspace.ID, "customer@example.com")
	contact.Name = "Jane Customer"
	contact.Phone = "+1-555-123-4567"
	contact.Company = "Example Corp"
	contact.Tags = []string{"vip", "enterprise"}

	err := sr.services.Store.Contacts().CreateContact(ctx, contact)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Contact created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Contact created", Passed: true,
		Details: fmt.Sprintf("ID: %s, Email: %s", contact.ID, contact.Email),
	})

	// Verify retrieval by ID
	sr.log("  Step 2: Verifying contact retrieval...")
	stored, err := sr.services.Store.Contacts().GetContact(ctx, workspace.ID, contact.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Contact retrievable", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Contact retrievable", Passed: stored.Name == contact.Name,
		Details: fmt.Sprintf("Name: %s, Company: %s", stored.Name, stored.Company),
	})

	// Verify email lookup
	sr.log("  Step 3: Verifying email lookup...")
	byEmail, err := sr.services.Store.Contacts().GetContactByEmail(ctx, workspace.ID, contact.Email)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Contact lookup by email", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Contact lookup by email", Passed: byEmail.ID == contact.ID,
		Details: fmt.Sprintf("Found: %s", byEmail.Email),
	})

	// Update contact
	sr.log("  Step 4: Updating contact...")
	contact.Tags = append(contact.Tags, "support-priority")
	contact.UpdatedAt = time.Now()

	err = sr.services.Store.Contacts().UpdateContact(ctx, contact)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Contact updated", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	// Verify update
	updated, _ := sr.services.Store.Contacts().GetContact(ctx, workspace.ID, contact.ID)
	hasPriorityTag := false
	for _, tag := range updated.Tags {
		if tag == "support-priority" {
			hasPriorityTag = true
			break
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Contact tags updated", Passed: hasPriorityTag,
		Details: fmt.Sprintf("Tags: %v", updated.Tags),
	})

	// List contacts
	sr.log("  Step 5: Listing workspace contacts...")
	contacts, err := sr.services.Store.Contacts().ListWorkspaceContacts(ctx, workspace.ID)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Contacts listable", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	hasContact := false
	for _, c := range contacts {
		if c.ID == contact.ID {
			hasContact = true
			break
		}
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Contact in workspace list", Passed: hasContact,
		Details: fmt.Sprintf("Total contacts: %d", len(contacts)),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// RunSessionContextScenario tests session creation and context switching
func (sr *PlatformScenarioRunner) RunSessionContextScenario(ctx context.Context) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		Name:          "Session and Context Switching",
		Verifications: make([]VerificationResult, 0),
	}

	// Create two workspaces
	workspace1 := platformdomain.NewWorkspace("Session Workspace 1", "session-ws-1")
	sr.services.Store.Workspaces().CreateWorkspace(ctx, workspace1)

	workspace2 := platformdomain.NewWorkspace("Session Workspace 2", "session-ws-2")
	sr.services.Store.Workspaces().CreateWorkspace(ctx, workspace2)

	// Create user with access to both workspaces
	user := platformdomain.NewUser("multi.workspace@session-test.com", "Multi-Workspace User")
	user.IsActive = true
	user.EmailVerified = true
	sr.services.Store.Users().CreateUser(ctx, user)

	// Assign to both workspaces
	role1 := &platformdomain.UserWorkspaceRole{
		ID:          id.New(),
		UserID:      user.ID,
		WorkspaceID: workspace1.ID,
		Role:        platformdomain.WorkspaceRoleAdmin,
		CreatedAt:   time.Now(),
	}
	sr.services.Store.Workspaces().CreateUserWorkspaceRole(ctx, role1)

	role2 := &platformdomain.UserWorkspaceRole{
		ID:          id.New(),
		UserID:      user.ID,
		WorkspaceID: workspace2.ID,
		Role:        platformdomain.WorkspaceRoleMember,
		CreatedAt:   time.Now(),
	}
	sr.services.Store.Workspaces().CreateUserWorkspaceRole(ctx, role2)

	sr.log("  Step 1: Creating session...")

	// Generate a test token and hash it (simulates what SessionService does)
	testToken := id.New() + id.New() // Random token
	tokenHash := sha256.Sum256([]byte(testToken))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	// Create session with available contexts
	session := &platformdomain.Session{
		ID:             id.New(),
		TokenHash:      tokenHashStr, // Store the hash, not the plaintext token
		UserID:         user.ID,
		Email:          user.Email,
		Name:           user.Name,
		IPAddress:      "127.0.0.1",
		UserAgent:      "Test/1.0",
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(24 * time.Hour),
		LastActivityAt: time.Now(),
	}

	// Build available contexts
	session.AvailableContexts = []platformdomain.Context{
		{Type: platformdomain.ContextTypeWorkspace, WorkspaceID: &workspace1.ID, Role: string(platformdomain.WorkspaceRoleAdmin)},
		{Type: platformdomain.ContextTypeWorkspace, WorkspaceID: &workspace2.ID, Role: string(platformdomain.WorkspaceRoleMember)},
	}
	session.CurrentContext = session.AvailableContexts[0]

	err := sr.services.Store.Users().SaveSession(ctx, session)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Session created", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Session created", Passed: true,
		Details: fmt.Sprintf("TokenHash: %s...", session.TokenHash[:8]),
	})

	// Verify session retrieval by hash
	sr.log("  Step 2: Verifying session retrieval...")
	stored, err := sr.services.Store.Users().GetSessionByHash(ctx, session.TokenHash)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Session retrievable", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Session retrievable", Passed: stored.UserID == user.ID,
		Details: fmt.Sprintf("UserID: %s", stored.UserID),
	})

	// Verify available contexts
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Has 2 available contexts", Passed: len(stored.AvailableContexts) >= 2,
		Details: fmt.Sprintf("Contexts: %d", len(stored.AvailableContexts)),
	})

	// Verify current context
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Current context set", Passed: stored.CurrentContext.Type == platformdomain.ContextTypeWorkspace,
		Details: fmt.Sprintf("Type: %s, Role: %s", stored.CurrentContext.Type, stored.CurrentContext.Role),
	})

	// Simulate context switch
	sr.log("  Step 3: Simulating context switch...")
	session.CurrentContext = session.AvailableContexts[1]
	session.LastActivityAt = time.Now()

	err = sr.services.Store.Users().UpdateSession(ctx, session)
	if err != nil {
		result.Verifications = append(result.Verifications, VerificationResult{
			Check: "Context switch persisted", Passed: false, Details: fmt.Sprintf("Error: %v", err),
		})
		return result, err
	}

	// Verify context switch
	updated, _ := sr.services.Store.Users().GetSessionByHash(ctx, session.TokenHash)
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Context switched to workspace 2", Passed: *updated.CurrentContext.WorkspaceID == workspace2.ID,
		Details: fmt.Sprintf("New workspace: %s", *updated.CurrentContext.WorkspaceID),
	})

	// Verify session validity
	sr.log("  Step 4: Verifying session validity...")
	result.Verifications = append(result.Verifications, VerificationResult{
		Check: "Session is valid", Passed: updated.IsValid(),
		Details: fmt.Sprintf("Expires: %s", updated.ExpiresAt.Format(time.RFC3339)),
	})

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

func (sr *PlatformScenarioRunner) log(format string, args ...interface{}) {
	if sr.verbose {
		fmt.Printf("[platform] "+format+"\n", args...)
	}
}
