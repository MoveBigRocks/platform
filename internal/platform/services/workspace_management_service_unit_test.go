package platformservices

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/testutil"
)

type stubWorkspaceIssueChecker struct {
	count int
	err   error
}

func (s stubWorkspaceIssueChecker) CountOpenWorkspaceIssues(context.Context, string) (int, error) {
	return s.count, s.err
}

func TestWorkspaceManagementService_DeleteWorkspaceBlocksOnOpenIssuesFromCapability(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	service := NewWorkspaceManagementService(store.Workspaces(), store.Cases(), store.Users(), store.Rules())
	service.SetIssueChecker(stubWorkspaceIssueChecker{count: 2})

	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	err := service.DeleteWorkspace(ctx, workspace.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "has 2 open issues")
}
