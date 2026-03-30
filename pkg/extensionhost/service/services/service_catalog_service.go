package serviceapp

import (
	"context"
	"strings"

	apierrors "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/errors"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

// ServiceCatalogService manages the workspace-scoped service catalog.
type ServiceCatalogService struct {
	catalogStore   shared.ServiceCatalogStore
	workspaceStore shared.WorkspaceStore
}

func NewServiceCatalogService(catalogStore shared.ServiceCatalogStore, workspaceStore shared.WorkspaceStore) *ServiceCatalogService {
	return &ServiceCatalogService{
		catalogStore:   catalogStore,
		workspaceStore: workspaceStore,
	}
}

func (s *ServiceCatalogService) GetServiceCatalogNode(ctx context.Context, nodeID string) (*servicedomain.ServiceCatalogNode, error) {
	if strings.TrimSpace(nodeID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("node_id", "required"))
	}
	node, err := s.catalogStore.GetServiceCatalogNode(ctx, nodeID)
	if err != nil {
		return nil, apierrors.NotFoundError("service catalog node", nodeID)
	}
	return node, nil
}

func (s *ServiceCatalogService) GetServiceCatalogNodeByPath(ctx context.Context, workspaceID, path string) (*servicedomain.ServiceCatalogNode, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if normalizedPath := normalizeCatalogPath(path); normalizedPath == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("path", "required"))
	}

	node, err := s.catalogStore.GetServiceCatalogNodeByPath(ctx, workspaceID, normalizeCatalogPath(path))
	if err != nil {
		return nil, apierrors.NotFoundError("service catalog node", normalizeCatalogPath(path))
	}
	return node, nil
}

func (s *ServiceCatalogService) ListServiceCatalogNodes(ctx context.Context, workspaceID, parentNodeID string) ([]*servicedomain.ServiceCatalogNode, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}

	nodes, err := s.catalogStore.ListChildServiceCatalogNodes(ctx, workspaceID, strings.TrimSpace(parentNodeID))
	if err != nil {
		return nil, apierrors.DatabaseError("list service catalog nodes", err)
	}
	return nodes, nil
}

func (s *ServiceCatalogService) ListServiceCatalogBindings(ctx context.Context, nodeID string) ([]*servicedomain.ServiceCatalogBinding, error) {
	if strings.TrimSpace(nodeID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("node_id", "required"))
	}

	bindings, err := s.catalogStore.ListServiceCatalogBindings(ctx, nodeID)
	if err != nil {
		return nil, apierrors.DatabaseError("list service catalog bindings", err)
	}
	return bindings, nil
}

func normalizeCatalogPath(path string) string {
	return strings.Trim(strings.TrimSpace(path), "/")
}
