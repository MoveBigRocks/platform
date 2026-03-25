package servicedomain

import (
	"strings"
	"time"

	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

type ServiceCatalogNodeKind string

const (
	ServiceCatalogNodeKindDomain      ServiceCatalogNodeKind = "domain"
	ServiceCatalogNodeKindService     ServiceCatalogNodeKind = "service"
	ServiceCatalogNodeKindRequestType ServiceCatalogNodeKind = "request_type"
	ServiceCatalogNodeKindIssueType   ServiceCatalogNodeKind = "issue_type"
)

type ServiceCatalogNodeStatus string

const (
	ServiceCatalogNodeStatusDraft    ServiceCatalogNodeStatus = "draft"
	ServiceCatalogNodeStatusActive   ServiceCatalogNodeStatus = "active"
	ServiceCatalogNodeStatusArchived ServiceCatalogNodeStatus = "archived"
)

type ServiceCatalogNodeVisibility string

const (
	ServiceCatalogNodeVisibilityWorkspace  ServiceCatalogNodeVisibility = "workspace"
	ServiceCatalogNodeVisibilityCustomer   ServiceCatalogNodeVisibility = "customer"
	ServiceCatalogNodeVisibilityRestricted ServiceCatalogNodeVisibility = "restricted"
)

type ServiceCatalogNode struct {
	ID          string
	WorkspaceID string

	ParentNodeID string
	Slug         string
	PathSlug     string
	Title        string

	DescriptionMarkdown string
	NodeKind            ServiceCatalogNodeKind
	Status              ServiceCatalogNodeStatus
	Visibility          ServiceCatalogNodeVisibility

	SupportedChannels   []string
	DefaultCaseCategory string
	DefaultQueueID      string
	DefaultPriority     string

	RoutingPolicy     shareddomain.TypedSchema
	EntitlementPolicy shareddomain.TypedSchema
	SearchKeywords    []string
	DisplayOrder      int

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewServiceCatalogNode(workspaceID, slug, title string) *ServiceCatalogNode {
	now := time.Now().UTC()
	slug = strings.TrimSpace(slug)

	return &ServiceCatalogNode{
		WorkspaceID:       workspaceID,
		Slug:              slug,
		PathSlug:          slug,
		Title:             title,
		NodeKind:          ServiceCatalogNodeKindService,
		Status:            ServiceCatalogNodeStatusActive,
		Visibility:        ServiceCatalogNodeVisibilityWorkspace,
		SupportedChannels: []string{},
		RoutingPolicy:     shareddomain.NewTypedSchema(),
		EntitlementPolicy: shareddomain.NewTypedSchema(),
		SearchKeywords:    []string{},
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

type ServiceCatalogBindingTargetKind string

const (
	ServiceCatalogBindingTargetKindKnowledgeResource ServiceCatalogBindingTargetKind = "knowledge_resource"
	ServiceCatalogBindingTargetKindFormSpec          ServiceCatalogBindingTargetKind = "form_spec"
	ServiceCatalogBindingTargetKindQueue             ServiceCatalogBindingTargetKind = "queue"
	ServiceCatalogBindingTargetKindPolicyProfile     ServiceCatalogBindingTargetKind = "policy_profile"
)

type ServiceCatalogBindingKind string

const (
	ServiceCatalogBindingKindPrimary   ServiceCatalogBindingKind = "primary"
	ServiceCatalogBindingKindSuggested ServiceCatalogBindingKind = "suggested"
	ServiceCatalogBindingKindDefault   ServiceCatalogBindingKind = "default"
)

type ServiceCatalogBinding struct {
	ID            string
	WorkspaceID   string
	CatalogNodeID string
	TargetKind    ServiceCatalogBindingTargetKind
	TargetID      string
	BindingKind   ServiceCatalogBindingKind
	Confidence    *float64
	CreatedAt     time.Time
}

func NewServiceCatalogBinding(workspaceID, catalogNodeID string, targetKind ServiceCatalogBindingTargetKind, targetID string) *ServiceCatalogBinding {
	return &ServiceCatalogBinding{
		WorkspaceID:   workspaceID,
		CatalogNodeID: catalogNodeID,
		TargetKind:    targetKind,
		TargetID:      targetID,
		BindingKind:   ServiceCatalogBindingKindPrimary,
		CreatedAt:     time.Now().UTC(),
	}
}
