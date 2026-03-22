package platformhandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	obsdomain "github.com/movebigrocks/platform/internal/observability/domain"
	observabilityservices "github.com/movebigrocks/platform/internal/observability/services"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/refext"
)

type testWorkspaceExtensionChecker struct {
	enabled bool
	err     error
}

func (t testWorkspaceExtensionChecker) HasActiveExtensionInWorkspace(_ context.Context, _, _ string) (bool, error) {
	return t.enabled, t.err
}

func TestWorkspaceAPIHandler_ListIssuesReturnsNotFoundWhenErrorTrackingInactive(t *testing.T) {
	t.Parallel()

	handler := NewWorkspaceAPIHandler(nil, nil, testWorkspaceExtensionChecker{enabled: false})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("workspace_id", "ws_test")
		c.Next()
	})
	router.GET("/api/issues", handler.ListIssues)

	req := httptest.NewRequest(http.MethodGet, "/api/issues", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWorkspaceAPIHandler_ResolveIssueRequiresIssueWritePermission(t *testing.T) {
	t.Parallel()

	store, cleanup := testutil.SetupTestSQLStore(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	refext.InstallAndActivateReferenceExtension(t, ctx, store, workspace.ID, "error-tracking")

	project := testutil.NewIsolatedProject(t, workspace.ID)
	require.NoError(t, store.Projects().CreateProject(ctx, project))

	issue := &obsdomain.Issue{
		ID:          "issue-no-permission",
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		Title:       "No permission case",
		Fingerprint: "fp-no-permission",
		Status:      obsdomain.IssueStatusUnresolved,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
	}
	require.NoError(t, store.Issues().CreateIssue(ctx, issue))

	issueService := observabilityservices.NewIssueService(
		store.Issues(),
		store.Projects(),
		store.ErrorEvents(),
		store.Workspaces(),
		nil,
	)
	handler := NewWorkspaceAPIHandler(nil, issueService, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		authCtx := graphshared.SetAuthContext(c.Request.Context(), &platformdomain.AuthContext{
			Permissions: []string{},
		})
		c.Request = c.Request.WithContext(authCtx)
		c.Set("workspace_id", workspace.ID)
		c.Set("auth_context", &platformdomain.AuthContext{})
		c.Next()
	})
	router.POST("/api/issues/:id/resolve", handler.ResolveIssue)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/issues/"+issue.ID+"/resolve",
		strings.NewReader(`{"resolution":"fixed"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "issue:write permission required", response["error"])

	updated, err := store.Issues().GetIssue(ctx, issue.ID)
	require.NoError(t, err)
	assert.Equal(t, obsdomain.IssueStatusUnresolved, updated.Status)
	assert.Nil(t, updated.ResolvedAt)
}

func TestWorkspaceAPIHandler_ResolveIssueNotFoundWhenWrongWorkspaceContext(t *testing.T) {
	t.Parallel()

	store, cleanup := testutil.SetupTestSQLStore(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	otherWorkspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, otherWorkspace))
	refext.InstallAndActivateReferenceExtension(t, ctx, store, otherWorkspace.ID, "error-tracking")

	project := testutil.NewIsolatedProject(t, otherWorkspace.ID)
	require.NoError(t, store.Projects().CreateProject(ctx, project))

	issue := &obsdomain.Issue{
		ID:          "issue-wrong-workspace",
		WorkspaceID: otherWorkspace.ID,
		ProjectID:   project.ID,
		Title:       "Wrong workspace issue",
		Fingerprint: "fp-wrong-workspace",
		Status:      obsdomain.IssueStatusUnresolved,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
	}
	require.NoError(t, store.Issues().CreateIssue(ctx, issue))

	issueService := observabilityservices.NewIssueService(
		store.Issues(),
		store.Projects(),
		store.ErrorEvents(),
		store.Workspaces(),
		nil,
	)
	handler := NewWorkspaceAPIHandler(nil, issueService, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		authCtx := graphshared.SetAuthContext(c.Request.Context(), &platformdomain.AuthContext{
			Permissions: []string{platformdomain.PermissionIssueWrite},
		})
		c.Request = c.Request.WithContext(authCtx)
		c.Set("workspace_id", workspace.ID)
		c.Set("auth_context", &platformdomain.AuthContext{
			Permissions: []string{platformdomain.PermissionIssueWrite},
		})
		c.Next()
	})
	router.POST("/api/issues/:id/resolve", handler.ResolveIssue)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/issues/"+issue.ID+"/resolve",
		strings.NewReader(`{"resolution":"fixed"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	updated, err := store.Issues().GetIssue(ctx, issue.ID)
	require.NoError(t, err)
	assert.Equal(t, obsdomain.IssueStatusUnresolved, updated.Status)
	assert.Nil(t, updated.ResolvedAt)
}

func TestWorkspaceAPIHandler_UpdateIssueRequiresIssueWritePermission(t *testing.T) {
	t.Parallel()

	store, cleanup := testutil.SetupTestSQLStore(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	refext.InstallAndActivateReferenceExtension(t, ctx, store, workspace.ID, "error-tracking")

	project := testutil.NewIsolatedProject(t, workspace.ID)
	require.NoError(t, store.Projects().CreateProject(ctx, project))

	issue := &obsdomain.Issue{
		ID:          "issue-update-no-permission",
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		Title:       "No permission issue",
		Fingerprint: "fp-no-permission-update",
		Status:      obsdomain.IssueStatusUnresolved,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
	}
	require.NoError(t, store.Issues().CreateIssue(ctx, issue))

	issueService := observabilityservices.NewIssueService(
		store.Issues(),
		store.Projects(),
		store.ErrorEvents(),
		store.Workspaces(),
		nil,
	)
	handler := NewWorkspaceAPIHandler(nil, issueService, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		authCtx := graphshared.SetAuthContext(c.Request.Context(), &platformdomain.AuthContext{
			Permissions: []string{},
		})
		c.Request = c.Request.WithContext(authCtx)
		c.Set("workspace_id", workspace.ID)
		c.Set("auth_context", &platformdomain.AuthContext{})
		c.Next()
	})
	router.PUT("/api/issues/:id", handler.UpdateIssue)

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/issues/"+issue.ID,
		strings.NewReader(`{"status":"resolved"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "issue:write permission required", response["error"])

	updated, err := store.Issues().GetIssue(ctx, issue.ID)
	require.NoError(t, err)
	assert.Equal(t, obsdomain.IssueStatusUnresolved, updated.Status)
	assert.Equal(t, "", updated.AssignedTo)
}

func TestWorkspaceAPIHandler_UpdateIssueNotFoundWhenWrongWorkspaceContext(t *testing.T) {
	t.Parallel()

	store, cleanup := testutil.SetupTestSQLStore(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	otherWorkspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, otherWorkspace))
	refext.InstallAndActivateReferenceExtension(t, ctx, store, otherWorkspace.ID, "error-tracking")

	project := testutil.NewIsolatedProject(t, otherWorkspace.ID)
	require.NoError(t, store.Projects().CreateProject(ctx, project))

	issue := &obsdomain.Issue{
		ID:          "issue-update-wrong-workspace",
		WorkspaceID: otherWorkspace.ID,
		ProjectID:   project.ID,
		Title:       "Wrong workspace issue",
		Fingerprint: "fp-wrong-workspace-update",
		Status:      obsdomain.IssueStatusUnresolved,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
	}
	require.NoError(t, store.Issues().CreateIssue(ctx, issue))

	issueService := observabilityservices.NewIssueService(
		store.Issues(),
		store.Projects(),
		store.ErrorEvents(),
		store.Workspaces(),
		nil,
	)
	handler := NewWorkspaceAPIHandler(nil, issueService, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		authCtx := graphshared.SetAuthContext(c.Request.Context(), &platformdomain.AuthContext{
			Permissions: []string{platformdomain.PermissionIssueWrite},
		})
		c.Request = c.Request.WithContext(authCtx)
		c.Set("workspace_id", workspace.ID)
		c.Set("auth_context", &platformdomain.AuthContext{
			Permissions: []string{platformdomain.PermissionIssueWrite},
		})
		c.Next()
	})
	router.PUT("/api/issues/:id", handler.UpdateIssue)

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/issues/"+issue.ID,
		strings.NewReader(`{"status":"resolved"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	updated, err := store.Issues().GetIssue(ctx, issue.ID)
	require.NoError(t, err)
	assert.Equal(t, obsdomain.IssueStatusUnresolved, updated.Status)
	assert.Equal(t, "", updated.AssignedTo)
}
