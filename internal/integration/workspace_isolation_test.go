//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/refext"
	"github.com/movebigrocks/platform/pkg/id"
)

// =============================================================================
// PROJECT STORE WORKSPACE ISOLATION TESTS
// These tests verify the core workspace isolation pattern at the store layer.
// =============================================================================

func TestWorkspaceIsolation_ProjectStore(t *testing.T) {
	testutil.SetupTestEnv(t)

	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create two workspaces
	ws1ID := testutil.CreateTestWorkspace(t, store, "workspace-1")
	ws2ID := testutil.CreateTestWorkspace(t, store, "workspace-2")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, ws1ID, "error-tracking")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, ws2ID, "error-tracking")

	// Create projects in both workspaces
	var project1, project2 *observabilitydomain.Project
	err := store.WithAdminContext(ctx, func(adminCtx context.Context) error {
		project1 = testutil.NewIsolatedProject(t, ws1ID)
		if err := store.Projects().CreateProject(adminCtx, project1); err != nil {
			return err
		}
		project2 = testutil.NewIsolatedProject(t, ws2ID)
		return store.Projects().CreateProject(adminCtx, project2)
	})
	require.NoError(t, err)

	t.Run("GetProjectInWorkspace returns project for correct workspace", func(t *testing.T) {
		result, err := store.Projects().GetProjectInWorkspace(ctx, ws1ID, project1.ID)
		require.NoError(t, err)
		assert.Equal(t, project1.ID, result.ID)
		assert.Equal(t, ws1ID, result.WorkspaceID)
	})

	t.Run("GetProjectInWorkspace returns ErrNotFound for wrong workspace", func(t *testing.T) {
		_, err := store.Projects().GetProjectInWorkspace(ctx, ws2ID, project1.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, shared.ErrNotFound)
	})

	t.Run("GetProjectInWorkspace returns ErrNotFound for non-existent project", func(t *testing.T) {
		_, err := store.Projects().GetProjectInWorkspace(ctx, ws1ID, "non-existent-id")
		require.Error(t, err)
		assert.ErrorIs(t, err, shared.ErrNotFound)
	})

	t.Run("Cross-workspace and non-existent return same error type", func(t *testing.T) {
		_, crossWorkspaceErr := store.Projects().GetProjectInWorkspace(ctx, ws2ID, project1.ID)
		_, nonExistentErr := store.Projects().GetProjectInWorkspace(ctx, ws1ID, "non-existent-id")

		// Both should be ErrNotFound - preventing enumeration attacks
		assert.ErrorIs(t, crossWorkspaceErr, shared.ErrNotFound)
		assert.ErrorIs(t, nonExistentErr, shared.ErrNotFound)
	})

	t.Run("ListWorkspaceProjects only returns workspace projects", func(t *testing.T) {
		projects, err := store.Projects().ListWorkspaceProjects(ctx, ws1ID)
		require.NoError(t, err)

		projectIDs := extractProjectIDs(projects)
		assert.Contains(t, projectIDs, project1.ID, "should contain own project")
		assert.NotContains(t, projectIDs, project2.ID, "should not contain other workspace project")
	})
}

// =============================================================================
// ISSUE STORE WORKSPACE ISOLATION TESTS
// Tests use minimal fixtures to avoid schema/mapping issues.
// =============================================================================

func TestWorkspaceIsolation_IssueStore(t *testing.T) {
	testutil.SetupTestEnv(t)

	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create two workspaces
	ws1ID := testutil.CreateTestWorkspace(t, store, "workspace-1")
	ws2ID := testutil.CreateTestWorkspace(t, store, "workspace-2")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, ws1ID, "error-tracking")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, ws2ID, "error-tracking")

	// Create projects first (issues need projects)
	var project1, project2 *observabilitydomain.Project
	err := store.WithAdminContext(ctx, func(adminCtx context.Context) error {
		project1 = testutil.NewIsolatedProject(t, ws1ID)
		if err := store.Projects().CreateProject(adminCtx, project1); err != nil {
			return err
		}
		project2 = testutil.NewIsolatedProject(t, ws2ID)
		return store.Projects().CreateProject(adminCtx, project2)
	})
	require.NoError(t, err)

	// Create issues using store directly with proper time handling
	var issue1ID, issue2ID string
	err = store.WithAdminContext(ctx, func(adminCtx context.Context) error {
		now := time.Now().UTC()
		issue1ID = id.New()
		issue1 := &observabilitydomain.Issue{
			ID:          issue1ID,
			ProjectID:   project1.ID,
			WorkspaceID: ws1ID,
			Title:       "Test Issue 1",
			Level:       observabilitydomain.ErrorLevelError,
			Status:      observabilitydomain.IssueStatusUnresolved,
			Fingerprint: "fp-" + issue1ID[:8],
			FirstSeen:   now,
			LastSeen:    now,
			EventCount:  1,
			UserCount:   1,
			Platform:    "javascript",
		}
		if err := store.Issues().CreateIssue(adminCtx, issue1); err != nil {
			return err
		}

		issue2ID = id.New()
		issue2 := &observabilitydomain.Issue{
			ID:          issue2ID,
			ProjectID:   project2.ID,
			WorkspaceID: ws2ID,
			Title:       "Test Issue 2",
			Level:       observabilitydomain.ErrorLevelError,
			Status:      observabilitydomain.IssueStatusUnresolved,
			Fingerprint: "fp-" + issue2ID[:8],
			FirstSeen:   now,
			LastSeen:    now,
			EventCount:  1,
			UserCount:   1,
			Platform:    "javascript",
		}
		return store.Issues().CreateIssue(adminCtx, issue2)
	})
	require.NoError(t, err)

	// Test workspace isolation - these verify the SQL WHERE clause includes workspace_id
	t.Run("GetIssueInWorkspace returns ErrNotFound for wrong workspace", func(t *testing.T) {
		// Try to get issue1 (belongs to ws1) using ws2
		_, err := store.Issues().GetIssueInWorkspace(ctx, ws2ID, issue1ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, shared.ErrNotFound, "should return ErrNotFound for cross-workspace access")
	})

	t.Run("GetIssueInWorkspace returns ErrNotFound for non-existent issue", func(t *testing.T) {
		_, err := store.Issues().GetIssueInWorkspace(ctx, ws1ID, "non-existent-id")
		require.Error(t, err)
		assert.ErrorIs(t, err, shared.ErrNotFound)
	})

	t.Run("Cross-workspace and non-existent return same error type", func(t *testing.T) {
		_, crossWorkspaceErr := store.Issues().GetIssueInWorkspace(ctx, ws2ID, issue1ID)
		_, nonExistentErr := store.Issues().GetIssueInWorkspace(ctx, ws1ID, "non-existent-id")

		// Both should be ErrNotFound - this prevents enumeration attacks
		assert.ErrorIs(t, crossWorkspaceErr, shared.ErrNotFound)
		assert.ErrorIs(t, nonExistentErr, shared.ErrNotFound)
	})
}

func TestWorkspaceIsolation_FormStore(t *testing.T) {
	testutil.SetupTestEnv(t)

	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create two workspaces
	ws1ID := testutil.CreateTestWorkspace(t, store, "workspace-1")
	ws2ID := testutil.CreateTestWorkspace(t, store, "workspace-2")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, ws1ID, "error-tracking")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, ws2ID, "error-tracking")

	const sharedSlug = "support-form"
	var form1ID, form2ID string

	err := store.WithAdminContext(ctx, func(adminCtx context.Context) error {
		form1 := servicedomain.NewFormSchema(ws1ID, "Workspace 1 Form", sharedSlug, "seed-user")
		form2 := servicedomain.NewFormSchema(ws2ID, "Workspace 2 Form", sharedSlug, "seed-user")
		form1ID = form1.ID
		form2ID = form2.ID

		if err := store.Forms().CreateFormSchema(adminCtx, form1); err != nil {
			return err
		}
		return store.Forms().CreateFormSchema(adminCtx, form2)
	})
	require.NoError(t, err)

	t.Run("GetFormBySlug is isolated by workspace", func(t *testing.T) {
		ws1Form, err := store.Forms().GetFormBySlug(ctx, ws1ID, sharedSlug)
		require.NoError(t, err)
		assert.Equal(t, form1ID, ws1Form.ID)
		assert.Equal(t, ws1ID, ws1Form.WorkspaceID)

		ws2Form, err := store.Forms().GetFormBySlug(ctx, ws2ID, sharedSlug)
		require.NoError(t, err)
		assert.Equal(t, form2ID, ws2Form.ID)
		assert.Equal(t, ws2ID, ws2Form.WorkspaceID)

		assert.NotEqual(t, ws1Form.ID, ws2Form.ID)
		assert.NotEqual(t, ws1Form.WorkspaceID, ws2Form.WorkspaceID)
	})

	t.Run("Shared slugs are rejected without workspace filter", func(t *testing.T) {
		_, err := store.Forms().GetFormBySlug(ctx, "", sharedSlug)
		require.Error(t, err)
		assert.ErrorIs(t, err, shared.ErrNotFound)
	})
}

// =============================================================================
// CROSS-CUTTING CONSISTENCY TESTS
// Verify all stores behave consistently for workspace isolation.
// =============================================================================

func TestWorkspaceIsolation_ConsistentErrorResponses(t *testing.T) {
	testutil.SetupTestEnv(t)

	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create two workspaces
	ws1ID := testutil.CreateTestWorkspace(t, store, "workspace-1")
	ws2ID := testutil.CreateTestWorkspace(t, store, "workspace-2")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, ws1ID, "error-tracking")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, ws2ID, "error-tracking")

	// Create project in workspace 1
	var project1 *observabilitydomain.Project
	err := store.WithAdminContext(ctx, func(adminCtx context.Context) error {
		project1 = testutil.NewIsolatedProject(t, ws1ID)
		return store.Projects().CreateProject(adminCtx, project1)
	})
	require.NoError(t, err)

	// Test that stores return ErrNotFound consistently for cross-workspace access
	t.Run("Project store returns ErrNotFound for cross-workspace access", func(t *testing.T) {
		_, projectErr := store.Projects().GetProjectInWorkspace(ctx, ws2ID, project1.ID)
		assert.ErrorIs(t, projectErr, shared.ErrNotFound, "Project store should return ErrNotFound")
	})

	// Test that non-existent resources also return ErrNotFound
	t.Run("Project store returns ErrNotFound for non-existent resources", func(t *testing.T) {
		_, projectErr := store.Projects().GetProjectInWorkspace(ctx, ws1ID, "fake-project-id")
		assert.ErrorIs(t, projectErr, shared.ErrNotFound, "Project store should return ErrNotFound for non-existent")
	})
}

// =============================================================================
// MULTI-WORKSPACE SCENARIO TESTS
// =============================================================================

func TestWorkspaceIsolation_ThreeWorkspaces(t *testing.T) {
	testutil.SetupTestEnv(t)

	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create three workspaces to test isolation with more than 2
	ws1ID := testutil.CreateTestWorkspace(t, store, "workspace-1")
	ws2ID := testutil.CreateTestWorkspace(t, store, "workspace-2")
	ws3ID := testutil.CreateTestWorkspace(t, store, "workspace-3")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, ws1ID, "error-tracking")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, ws2ID, "error-tracking")
	refext.InstallAndActivateReferenceExtension(t, ctx, store, ws3ID, "error-tracking")

	// Create one project per workspace
	var projects []*observabilitydomain.Project
	err := store.WithAdminContext(ctx, func(adminCtx context.Context) error {
		for _, wsID := range []string{ws1ID, ws2ID, ws3ID} {
			p := testutil.NewIsolatedProject(t, wsID)
			if err := store.Projects().CreateProject(adminCtx, p); err != nil {
				return err
			}
			projects = append(projects, p)
		}
		return nil
	})
	require.NoError(t, err)

	t.Run("Each workspace sees only its own projects", func(t *testing.T) {
		for i, wsID := range []string{ws1ID, ws2ID, ws3ID} {
			wsProjects, err := store.Projects().ListWorkspaceProjects(ctx, wsID)
			require.NoError(t, err)
			assert.Len(t, wsProjects, 1, "each workspace should have 1 project")
			assert.Equal(t, projects[i].ID, wsProjects[0].ID)
			assert.Equal(t, wsID, wsProjects[0].WorkspaceID)
		}
	})

	t.Run("GetProjectInWorkspace fails for all other workspaces", func(t *testing.T) {
		project1 := projects[0]

		// Should succeed for workspace 1
		result, err := store.Projects().GetProjectInWorkspace(ctx, ws1ID, project1.ID)
		require.NoError(t, err)
		assert.Equal(t, project1.ID, result.ID)

		// Should fail for workspace 2 and 3
		_, err = store.Projects().GetProjectInWorkspace(ctx, ws2ID, project1.ID)
		assert.ErrorIs(t, err, shared.ErrNotFound)

		_, err = store.Projects().GetProjectInWorkspace(ctx, ws3ID, project1.ID)
		assert.ErrorIs(t, err, shared.ErrNotFound)
	})
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func extractProjectIDs(projects []*observabilitydomain.Project) []string {
	ids := make([]string, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}
	return ids
}
