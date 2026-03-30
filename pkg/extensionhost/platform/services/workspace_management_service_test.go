//go:build integration

package platformservices

import (
	"context"
	"testing"

	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceManagementService_CreateWorkspace_SeedsDefaultRules(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	service := NewWorkspaceManagementService(store.Workspaces(), store.Cases(), store.Users(), store.Rules())

	slug := testutil.UniqueID("ws")
	// Create a workspace
	workspace, err := service.CreateWorkspace(ctx, "Test Workspace", slug, "A test workspace")
	require.NoError(t, err)
	require.NotNil(t, workspace)

	// Verify default rules were seeded
	rules, err := store.Rules().ListWorkspaceRules(ctx, workspace.ID)
	require.NoError(t, err)

	assert.Len(t, rules, 7, "Should have seeded 7 default rules")

	// Verify all rules are system rules
	for _, rule := range rules {
		assert.True(t, rule.IsSystem, "Rule should be marked as system: %s", rule.Title)
		assert.True(t, rule.IsActive, "Rule should be active: %s", rule.Title)
		assert.NotEmpty(t, rule.SystemRuleKey, "Rule should have system key: %s", rule.Title)
	}

	// Verify all expected system keys are present
	systemKeys := make(map[string]bool)
	for _, rule := range rules {
		systemKeys[rule.SystemRuleKey] = true
	}

	expectedKeys := []string{
		automationservices.SystemRuleKeyCaseCreatedReceipt,
		automationservices.SystemRuleKeyFirstResponseOpen,
		automationservices.SystemRuleKeyCustomerReplyReopen,
		automationservices.SystemRuleKeyCustomerReplyReopenClosed,
		automationservices.SystemRuleKeyPendingReminder,
		automationservices.SystemRuleKeyAutoCloseResolved,
		automationservices.SystemRuleKeyNoResponseAlert,
	}
	for _, key := range expectedKeys {
		assert.True(t, systemKeys[key], "Missing system rule: %s", key)
	}
}

func TestWorkspaceManagementService_CreateWorkspace_MultipleWorkspaces(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	service := NewWorkspaceManagementService(store.Workspaces(), store.Cases(), store.Users(), store.Rules())

	// Create first workspace
	slug1 := testutil.UniqueID("ws")
	ws1, err := service.CreateWorkspace(ctx, "Workspace One", slug1, "First workspace")
	require.NoError(t, err)

	// Create second workspace
	slug2 := testutil.UniqueID("ws")
	ws2, err := service.CreateWorkspace(ctx, "Workspace Two", slug2, "Second workspace")
	require.NoError(t, err)

	// Each workspace should have its own rules
	rules1, err := store.Rules().ListWorkspaceRules(ctx, ws1.ID)
	require.NoError(t, err)
	assert.Len(t, rules1, 7, "Workspace 1 should have 7 rules")

	rules2, err := store.Rules().ListWorkspaceRules(ctx, ws2.ID)
	require.NoError(t, err)
	assert.Len(t, rules2, 7, "Workspace 2 should have 7 rules")

	// Rules should have different IDs
	ids1 := make(map[string]bool)
	for _, r := range rules1 {
		ids1[r.ID] = true
	}

	for _, r := range rules2 {
		assert.False(t, ids1[r.ID], "Workspace 2 rules should have different IDs than workspace 1")
	}
}

func TestWorkspaceManagementService_CreateWorkspace_DuplicateSlugFails(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	service := NewWorkspaceManagementService(store.Workspaces(), store.Cases(), store.Users(), store.Rules())

	slug := testutil.UniqueID("ws")
	// Create first workspace
	_, err := service.CreateWorkspace(ctx, "Workspace One", slug, "First workspace")
	require.NoError(t, err)

	// Try to create second workspace with same slug
	_, err = service.CreateWorkspace(ctx, "Workspace Two", slug, "Second workspace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestWorkspaceManagementService_CreateWorkspace_SettingsCreated(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	service := NewWorkspaceManagementService(store.Workspaces(), store.Cases(), store.Users(), store.Rules())

	slug := testutil.UniqueID("ws")
	// Create a workspace
	workspace, err := service.CreateWorkspace(ctx, "Settings Test", slug, "Testing settings creation")
	require.NoError(t, err)

	// Verify settings were created
	settings, err := store.Workspaces().GetWorkspaceSettings(ctx, workspace.ID)
	require.NoError(t, err)
	require.NotNil(t, settings)
	assert.Equal(t, workspace.ID, settings.WorkspaceID)
}

func TestWorkspaceManagementService_DeleteWorkspace_WithRules(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	service := NewWorkspaceManagementService(store.Workspaces(), store.Cases(), store.Users(), store.Rules())

	slug := testutil.UniqueID("ws")
	// Create a workspace (which will seed rules)
	workspace, err := service.CreateWorkspace(ctx, "Delete Test", slug, "Testing deletion")
	require.NoError(t, err)

	// Verify rules exist
	rules, err := store.Rules().ListWorkspaceRules(ctx, workspace.ID)
	require.NoError(t, err)
	assert.Len(t, rules, 7, "Should have 7 rules before deletion")

	// Delete the workspace
	err = service.DeleteWorkspace(ctx, workspace.ID)
	require.NoError(t, err)

	// Verify workspace is deleted
	_, err = store.Workspaces().GetWorkspace(ctx, workspace.ID)
	require.Error(t, err, "Workspace should be deleted")
}

func TestWorkspaceManagementService_RulesSeederIntegration(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	service := NewWorkspaceManagementService(store.Workspaces(), store.Cases(), store.Users(), store.Rules())

	// Verify the seeder is initialized
	assert.NotNil(t, service.rulesSeeder, "Rules seeder should be initialized")

	slug := testutil.UniqueID("ws")
	// Create workspace and verify rules
	workspace, err := service.CreateWorkspace(ctx, "Integration Test", slug, "Testing full integration")
	require.NoError(t, err)

	rules, err := store.Rules().ListWorkspaceRules(ctx, workspace.ID)
	require.NoError(t, err)

	// Verify specific rules exist with correct structure
	rulesByKey := make(map[string]bool)
	for _, rule := range rules {
		rulesByKey[rule.SystemRuleKey] = true
	}

	// All system rules should be present
	assert.True(t, rulesByKey[automationservices.SystemRuleKeyCaseCreatedReceipt], "Should have case created receipt rule")
	assert.True(t, rulesByKey[automationservices.SystemRuleKeyFirstResponseOpen], "Should have first response open rule")
	assert.True(t, rulesByKey[automationservices.SystemRuleKeyCustomerReplyReopen], "Should have customer reply reopen rule")
	assert.True(t, rulesByKey[automationservices.SystemRuleKeyPendingReminder], "Should have pending reminder rule")
	assert.True(t, rulesByKey[automationservices.SystemRuleKeyAutoCloseResolved], "Should have auto close resolved rule")
	assert.True(t, rulesByKey[automationservices.SystemRuleKeyNoResponseAlert], "Should have no response alert rule")
}

func TestWorkspaceManagementService_CreateTeamAndAddMember(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	service := NewWorkspaceManagementService(store.Workspaces(), store.Cases(), store.Users(), store.Rules())

	workspace, err := service.CreateWorkspace(ctx, "Team Workspace", testutil.UniqueID("ws"), "Team test workspace")
	require.NoError(t, err)

	user := platformdomain.NewUser("team.member@example.com", "Team Member")
	require.NoError(t, store.Users().CreateUser(ctx, user))
	require.NoError(t, service.AddUserToWorkspace(ctx, user.ID, workspace.ID, platformdomain.WorkspaceRoleMember))

	team := &platformdomain.Team{
		WorkspaceID: workspace.ID,
		Name:        "Marketing",
		IsActive:    true,
	}
	require.NoError(t, service.CreateTeam(ctx, team))
	require.NotEmpty(t, team.ID)

	membersBefore, err := service.GetTeamMembers(ctx, workspace.ID, team.ID)
	require.NoError(t, err)
	assert.Len(t, membersBefore, 0)

	member := &platformdomain.TeamMember{
		TeamID: team.ID,
		UserID: user.ID,
		Role:   platformdomain.TeamMemberRoleLead,
	}
	require.NoError(t, service.AddTeamMember(ctx, member))
	require.NotEmpty(t, member.ID)

	membersAfter, err := service.GetTeamMembers(ctx, workspace.ID, team.ID)
	require.NoError(t, err)
	require.Len(t, membersAfter, 1)
	assert.Equal(t, user.ID, membersAfter[0].UserID)
	assert.Equal(t, platformdomain.TeamMemberRoleLead, membersAfter[0].Role)
}

func TestWorkspaceManagementService_AddTeamMemberRequiresWorkspaceMembership(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	service := NewWorkspaceManagementService(store.Workspaces(), store.Cases(), store.Users(), store.Rules())

	workspace, err := service.CreateWorkspace(ctx, "Restricted Workspace", testutil.UniqueID("ws"), "Restricted team membership test")
	require.NoError(t, err)

	user := platformdomain.NewUser("outsider@example.com", "Outside User")
	require.NoError(t, store.Users().CreateUser(ctx, user))

	team := &platformdomain.Team{
		WorkspaceID: workspace.ID,
		Name:        "Support",
		IsActive:    true,
	}
	require.NoError(t, service.CreateTeam(ctx, team))

	err = service.AddTeamMember(ctx, &platformdomain.TeamMember{
		TeamID: team.ID,
		UserID: user.ID,
		Role:   platformdomain.TeamMemberRoleMember,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user is not a member of workspace")
}
