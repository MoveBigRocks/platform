package sql

import (
	"context"
	"database/sql"

	"github.com/lib/pq"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

type ServiceCatalogStore struct {
	db *SqlxDB
}

func NewServiceCatalogStore(db *SqlxDB) *ServiceCatalogStore {
	return &ServiceCatalogStore{db: db}
}

func (s *ServiceCatalogStore) CreateServiceCatalogNode(ctx context.Context, node *servicedomain.ServiceCatalogNode) error {
	normalizePersistedUUID(&node.ID)

	routingPolicy, err := marshalJSONString(node.RoutingPolicy, "routing_policy")
	if err != nil {
		return err
	}
	entitlementPolicy, err := marshalJSONString(node.EntitlementPolicy, "entitlement_policy")
	if err != nil {
		return err
	}

	query := `
		INSERT INTO core_service.service_catalog_nodes (
			id, workspace_id, parent_node_id, slug, path_slug, title, description_markdown,
			node_kind, status, visibility, supported_channels, default_case_category,
			default_queue_id, default_priority, routing_policy_json,
			entitlement_policy_json, search_keywords, display_order, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		node.ID,
		node.WorkspaceID,
		nullableUUIDValue(node.ParentNodeID),
		node.Slug,
		node.PathSlug,
		node.Title,
		node.DescriptionMarkdown,
		string(node.NodeKind),
		string(node.Status),
		string(node.Visibility),
		pq.Array(node.SupportedChannels),
		node.DefaultCaseCategory,
		nullableUUIDValue(node.DefaultQueueID),
		node.DefaultPriority,
		routingPolicy,
		entitlementPolicy,
		pq.Array(node.SearchKeywords),
		node.DisplayOrder,
		node.CreatedAt,
		node.UpdatedAt,
	).Scan(&node.ID)
	return TranslateSqlxError(err, "service_catalog_nodes")
}

func (s *ServiceCatalogStore) GetServiceCatalogNode(ctx context.Context, nodeID string) (*servicedomain.ServiceCatalogNode, error) {
	var model models.ServiceCatalogNode
	query := `SELECT * FROM core_service.service_catalog_nodes WHERE id = ?`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, nodeID); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "service_catalog_nodes")
	}
	return mapServiceCatalogNodeToDomain(&model), nil
}

func (s *ServiceCatalogStore) GetServiceCatalogNodeByPath(ctx context.Context, workspaceID, pathSlug string) (*servicedomain.ServiceCatalogNode, error) {
	var model models.ServiceCatalogNode
	query := `SELECT * FROM core_service.service_catalog_nodes WHERE workspace_id = ? AND path_slug = ?`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, workspaceID, pathSlug); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "service_catalog_nodes")
	}
	return mapServiceCatalogNodeToDomain(&model), nil
}

func (s *ServiceCatalogStore) ListWorkspaceServiceCatalogNodes(ctx context.Context, workspaceID string) ([]*servicedomain.ServiceCatalogNode, error) {
	var modelsList []models.ServiceCatalogNode
	query := `SELECT * FROM core_service.service_catalog_nodes WHERE workspace_id = ? ORDER BY path_slug ASC`
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, workspaceID); err != nil {
		return nil, TranslateSqlxError(err, "service_catalog_nodes")
	}

	nodes := make([]*servicedomain.ServiceCatalogNode, len(modelsList))
	for i := range modelsList {
		nodes[i] = mapServiceCatalogNodeToDomain(&modelsList[i])
	}
	return nodes, nil
}

func (s *ServiceCatalogStore) ListChildServiceCatalogNodes(ctx context.Context, workspaceID, parentNodeID string) ([]*servicedomain.ServiceCatalogNode, error) {
	var (
		modelsList []models.ServiceCatalogNode
		err        error
	)

	if parentNodeID == "" {
		err = s.db.Get(ctx).SelectContext(ctx, &modelsList,
			`SELECT * FROM core_service.service_catalog_nodes WHERE workspace_id = ? AND parent_node_id IS NULL ORDER BY display_order, id`,
			workspaceID,
		)
	} else {
		err = s.db.Get(ctx).SelectContext(ctx, &modelsList,
			`SELECT * FROM core_service.service_catalog_nodes WHERE workspace_id = ? AND parent_node_id = ? ORDER BY display_order, id`,
			workspaceID, parentNodeID,
		)
	}
	if err != nil {
		return nil, TranslateSqlxError(err, "service_catalog_nodes")
	}

	nodes := make([]*servicedomain.ServiceCatalogNode, len(modelsList))
	for i := range modelsList {
		nodes[i] = mapServiceCatalogNodeToDomain(&modelsList[i])
	}
	return nodes, nil
}

func (s *ServiceCatalogStore) UpdateServiceCatalogNode(ctx context.Context, node *servicedomain.ServiceCatalogNode) error {
	routingPolicy, err := marshalJSONString(node.RoutingPolicy, "routing_policy")
	if err != nil {
		return err
	}
	entitlementPolicy, err := marshalJSONString(node.EntitlementPolicy, "entitlement_policy")
	if err != nil {
		return err
	}

	query := `
		UPDATE core_service.service_catalog_nodes SET
			parent_node_id = ?, slug = ?, path_slug = ?, title = ?, description_markdown = ?,
			node_kind = ?, status = ?, visibility = ?, supported_channels = ?,
			default_case_category = ?, default_queue_id = ?, default_priority = ?,
			routing_policy_json = ?, entitlement_policy_json = ?, search_keywords = ?,
			display_order = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ?`

	result, err := s.db.Get(ctx).ExecContext(ctx, query,
		nullableUUIDValue(node.ParentNodeID),
		node.Slug,
		node.PathSlug,
		node.Title,
		node.DescriptionMarkdown,
		string(node.NodeKind),
		string(node.Status),
		string(node.Visibility),
		pq.Array(node.SupportedChannels),
		node.DefaultCaseCategory,
		nullableUUIDValue(node.DefaultQueueID),
		node.DefaultPriority,
		routingPolicy,
		entitlementPolicy,
		pq.Array(node.SearchKeywords),
		node.DisplayOrder,
		node.UpdatedAt,
		node.ID,
		node.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "service_catalog_nodes")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *ServiceCatalogStore) CreateServiceCatalogBinding(ctx context.Context, binding *servicedomain.ServiceCatalogBinding) error {
	normalizePersistedUUID(&binding.ID)

	query := `
		INSERT INTO core_service.service_catalog_bindings (
			id, workspace_id, catalog_node_id, target_kind, target_id, binding_kind, confidence, created_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		binding.ID,
		binding.WorkspaceID,
		binding.CatalogNodeID,
		string(binding.TargetKind),
		nullableUUIDValue(binding.TargetID),
		string(binding.BindingKind),
		binding.Confidence,
		binding.CreatedAt,
	).Scan(&binding.ID)
	return TranslateSqlxError(err, "service_catalog_bindings")
}

func (s *ServiceCatalogStore) ListServiceCatalogBindings(ctx context.Context, catalogNodeID string) ([]*servicedomain.ServiceCatalogBinding, error) {
	var modelsList []models.ServiceCatalogBinding
	query := `SELECT * FROM core_service.service_catalog_bindings WHERE catalog_node_id = ? ORDER BY created_at, id`
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, catalogNodeID); err != nil {
		return nil, TranslateSqlxError(err, "service_catalog_bindings")
	}

	bindings := make([]*servicedomain.ServiceCatalogBinding, len(modelsList))
	for i := range modelsList {
		bindings[i] = mapServiceCatalogBindingToDomain(&modelsList[i])
	}
	return bindings, nil
}

func (s *ServiceCatalogStore) ListServiceCatalogBindingsForTarget(ctx context.Context, workspaceID, targetKind, targetID string) ([]*servicedomain.ServiceCatalogBinding, error) {
	var modelsList []models.ServiceCatalogBinding
	query := `
		SELECT * FROM core_service.service_catalog_bindings
		WHERE workspace_id = ? AND target_kind = ? AND target_id = ?
		ORDER BY created_at, id`
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, workspaceID, targetKind, targetID); err != nil {
		return nil, TranslateSqlxError(err, "service_catalog_bindings")
	}

	bindings := make([]*servicedomain.ServiceCatalogBinding, len(modelsList))
	for i := range modelsList {
		bindings[i] = mapServiceCatalogBindingToDomain(&modelsList[i])
	}
	return bindings, nil
}

func mapServiceCatalogNodeToDomain(model *models.ServiceCatalogNode) *servicedomain.ServiceCatalogNode {
	return &servicedomain.ServiceCatalogNode{
		ID:                  model.ID,
		WorkspaceID:         model.WorkspaceID,
		ParentNodeID:        valueOrEmpty(model.ParentNodeID),
		Slug:                model.Slug,
		PathSlug:            model.PathSlug,
		Title:               model.Title,
		DescriptionMarkdown: model.DescriptionMarkdown,
		NodeKind:            servicedomain.ServiceCatalogNodeKind(model.NodeKind),
		Status:              servicedomain.ServiceCatalogNodeStatus(model.Status),
		Visibility:          servicedomain.ServiceCatalogNodeVisibility(model.Visibility),
		SupportedChannels:   unmarshalStringArrayField(model.SupportedChannels, "service_catalog_nodes", "supported_channels"),
		DefaultCaseCategory: valueOrEmpty(model.DefaultCaseCategory),
		DefaultQueueID:      valueOrEmpty(model.DefaultQueueID),
		DefaultPriority:     valueOrEmpty(model.DefaultPriority),
		RoutingPolicy:       unmarshalTypedSchemaOrEmpty(model.RoutingPolicyJSON, "service_catalog_nodes", "routing_policy_json"),
		EntitlementPolicy:   unmarshalTypedSchemaOrEmpty(model.EntitlementPolicyJSON, "service_catalog_nodes", "entitlement_policy_json"),
		SearchKeywords:      unmarshalStringArrayField(model.SearchKeywords, "service_catalog_nodes", "search_keywords"),
		DisplayOrder:        model.DisplayOrder,
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
	}
}

func mapServiceCatalogBindingToDomain(model *models.ServiceCatalogBinding) *servicedomain.ServiceCatalogBinding {
	return &servicedomain.ServiceCatalogBinding{
		ID:            model.ID,
		WorkspaceID:   model.WorkspaceID,
		CatalogNodeID: model.CatalogNodeID,
		TargetKind:    servicedomain.ServiceCatalogBindingTargetKind(model.TargetKind),
		TargetID:      model.TargetID,
		BindingKind:   servicedomain.ServiceCatalogBindingKind(model.BindingKind),
		Confidence:    model.Confidence,
		CreatedAt:     model.CreatedAt,
	}
}

var _ shared.ServiceCatalogStore = (*ServiceCatalogStore)(nil)
