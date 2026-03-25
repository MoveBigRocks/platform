package models

import "time"

type ServiceCatalogNode struct {
	ID                    string    `db:"id"`
	WorkspaceID           string    `db:"workspace_id"`
	ParentNodeID          *string   `db:"parent_node_id"`
	Slug                  string    `db:"slug"`
	PathSlug              string    `db:"path_slug"`
	Title                 string    `db:"title"`
	DescriptionMarkdown   string    `db:"description_markdown"`
	NodeKind              string    `db:"node_kind"`
	Status                string    `db:"status"`
	Visibility            string    `db:"visibility"`
	SupportedChannels     string    `db:"supported_channels"`
	DefaultCaseCategory   *string   `db:"default_case_category"`
	DefaultQueueID        *string   `db:"default_queue_id"`
	DefaultPriority       *string   `db:"default_priority"`
	RoutingPolicyJSON     string    `db:"routing_policy_json"`
	EntitlementPolicyJSON string    `db:"entitlement_policy_json"`
	SearchKeywords        string    `db:"search_keywords"`
	SearchVector          string    `db:"search_vector"`
	DisplayOrder          int       `db:"display_order"`
	CreatedAt             time.Time `db:"created_at"`
	UpdatedAt             time.Time `db:"updated_at"`
}

func (ServiceCatalogNode) TableName() string { return "service_catalog_nodes" }

type ServiceCatalogBinding struct {
	ID            string    `db:"id"`
	WorkspaceID   string    `db:"workspace_id"`
	CatalogNodeID string    `db:"catalog_node_id"`
	TargetKind    string    `db:"target_kind"`
	TargetID      string    `db:"target_id"`
	BindingKind   string    `db:"binding_kind"`
	Confidence    *float64  `db:"confidence"`
	CreatedAt     time.Time `db:"created_at"`
}

func (ServiceCatalogBinding) TableName() string { return "service_catalog_bindings" }
