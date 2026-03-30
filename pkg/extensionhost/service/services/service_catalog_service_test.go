package serviceapp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

func TestServiceCatalogService_GetByPathAndList(t *testing.T) {
	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	workspaceID := testutil.CreateTestWorkspace(t, store, "catalog-list")
	service := NewServiceCatalogService(store.ServiceCatalog(), store.Workspaces())
	ctx := context.Background()

	rootNode := servicedomain.NewServiceCatalogNode(workspaceID, "support", "Support")
	rootNode.NodeKind = servicedomain.ServiceCatalogNodeKindDomain
	rootNode.PathSlug = "support"
	require.NoError(t, store.ServiceCatalog().CreateServiceCatalogNode(ctx, rootNode))

	childNode := servicedomain.NewServiceCatalogNode(workspaceID, "refunds", "Refund Requests")
	childNode.ParentNodeID = rootNode.ID
	childNode.PathSlug = "support/refunds"
	childNode.NodeKind = servicedomain.ServiceCatalogNodeKindRequestType
	require.NoError(t, store.ServiceCatalog().CreateServiceCatalogNode(ctx, childNode))

	loadedNode, err := service.GetServiceCatalogNodeByPath(ctx, workspaceID, "/support/refunds/")
	require.NoError(t, err)
	assert.Equal(t, childNode.ID, loadedNode.ID)

	children, err := service.ListServiceCatalogNodes(ctx, workspaceID, rootNode.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, childNode.ID, children[0].ID)

	roots, err := service.ListServiceCatalogNodes(ctx, workspaceID, "")
	require.NoError(t, err)
	require.Len(t, roots, 1)
	assert.Equal(t, rootNode.ID, roots[0].ID)
}

func TestServiceCatalogService_ListBindings(t *testing.T) {
	store, cleanup := testutil.SetupTestSQLStore(t)
	defer cleanup()

	workspaceID := testutil.CreateTestWorkspace(t, store, "catalog-bindings")
	service := NewServiceCatalogService(store.ServiceCatalog(), store.Workspaces())
	ctx := context.Background()

	queue := servicedomain.NewQueue(workspaceID, "Billing", "billing", "Billing queue")
	require.NoError(t, store.Queues().CreateQueue(ctx, queue))

	node := servicedomain.NewServiceCatalogNode(workspaceID, "billing", "Billing")
	node.PathSlug = "billing"
	require.NoError(t, store.ServiceCatalog().CreateServiceCatalogNode(ctx, node))

	binding := servicedomain.NewServiceCatalogBinding(workspaceID, node.ID, servicedomain.ServiceCatalogBindingTargetKindQueue, queue.ID)
	binding.BindingKind = servicedomain.ServiceCatalogBindingKindDefault
	require.NoError(t, store.ServiceCatalog().CreateServiceCatalogBinding(ctx, binding))

	bindings, err := service.ListServiceCatalogBindings(ctx, node.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	assert.Equal(t, node.ID, bindings[0].CatalogNodeID)
	assert.Equal(t, queue.ID, bindings[0].TargetID)
	assert.Equal(t, servicedomain.ServiceCatalogBindingKindDefault, bindings[0].BindingKind)
}
