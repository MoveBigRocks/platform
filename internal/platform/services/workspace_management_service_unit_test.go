package platformservices

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/testutil"
)

func TestWorkspaceManagementService_DeleteWorkspaceDoesNotDependOnExtensionCapabilities(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	service := NewWorkspaceManagementService(store.Workspaces(), store.Cases(), store.Users(), store.Rules())

	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	require.NoError(t, service.DeleteWorkspace(ctx, workspace.ID))
}
