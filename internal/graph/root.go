// Package graph provides the GraphQL API implementation using graph-gophers/graphql-go.
//
// Architecture:
// - root.go: RootResolver that delegates to domain-specific resolvers
// - model/: GraphQL types shared across domains
// - shared/scalars.go: Custom scalar types (DateTime, JSON)
// - Each domain (service, observability, platform, automation) has its own resolvers
package graph

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"

	"github.com/movebigrocks/platform/internal/graph/model"
	"github.com/movebigrocks/platform/internal/graph/shared"
	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"

	// Domain resolvers - each domain owns its API surface
	automationresolvers "github.com/movebigrocks/platform/internal/automation/resolvers"
	platformresolvers "github.com/movebigrocks/platform/internal/platform/resolvers"
	serviceresolvers "github.com/movebigrocks/platform/internal/service/resolvers"
)

// RootResolver is the entry point for all GraphQL operations.
// It delegates to domain-specific resolvers.
type RootResolver struct {
	service    *serviceresolvers.Resolver
	platform   *platformresolvers.Resolver
	automation *automationresolvers.Resolver

	agentUserService *platformservices.UserManagementService
	agentService     *platformservices.AgentService
}

// Config contains all dependencies needed by the GraphQL resolvers
type Config struct {
	Service    *serviceresolvers.Config
	Platform   *platformresolvers.Config
	Automation *automationresolvers.Config
}

// NewRootResolver creates a new root resolver with all domain resolvers
func NewRootResolver(cfg Config) *RootResolver {
	r := &RootResolver{}

	if cfg.Service != nil {
		r.service = serviceresolvers.NewResolver(*cfg.Service)
	}
	if cfg.Platform != nil {
		platformCfg := *cfg.Platform
		platformCfg.ServiceGraph = r.service
		if platformCfg.CaseService == nil && cfg.Service != nil {
			platformCfg.CaseService = cfg.Service.CaseService
		}
		if platformCfg.QueueService == nil && cfg.Service != nil {
			platformCfg.QueueService = cfg.Service.QueueService
		}
		r.platform = platformresolvers.NewResolver(platformCfg)

		r.agentUserService = platformCfg.UserService
		r.agentService = platformCfg.AgentService
	}
	if cfg.Automation != nil {
		r.automation = automationresolvers.NewResolver(*cfg.Automation)
	}

	return r
}

// =============================================================================
// Query Resolvers
// =============================================================================

// Me returns the current authenticated principal
func (r *RootResolver) Me(ctx context.Context) (*PrincipalResolver, error) {
	authCtx, err := shared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if authCtx.IsAgent() {
		agent := authCtx.GetAgent()
		if agent == nil {
			return nil, fmt.Errorf("agent not found in context")
		}

		return &PrincipalResolver{
			agent:            agentFromDomain(agent),
			agentUserService: r.agentUserService,
			agentService:     r.agentService,
		}, nil
	}

	user := authCtx.GetHuman()
	if user == nil {
		return nil, fmt.Errorf("user not found in context")
	}
	return &PrincipalResolver{user: userFromDomain(user)}, nil
}

// --- Service Domain Queries (delegated) ---

// Case delegates to service resolver
func (r *RootResolver) Case(ctx context.Context, args struct{ ID string }) (*serviceresolvers.CaseResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.Case(ctx, args.ID)
}

// CaseByHumanID delegates to service resolver
func (r *RootResolver) CaseByHumanID(ctx context.Context, args struct {
	WorkspaceID string
	CaseID      string
}) (*serviceresolvers.CaseResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.CaseByHumanID(ctx, args.WorkspaceID, args.CaseID)
}

// Cases delegates to service resolver
func (r *RootResolver) Cases(ctx context.Context, args struct {
	WorkspaceID string
	Filter      *model.CaseFilterInput
}) (*serviceresolvers.CaseConnectionResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.Cases(ctx, args.WorkspaceID, args.Filter)
}

// Queue delegates to service resolver.
func (r *RootResolver) Queue(ctx context.Context, args struct{ ID string }) (*serviceresolvers.QueueResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.Queue(ctx, args.ID)
}

// Queues delegates to service resolver.
func (r *RootResolver) Queues(ctx context.Context, args struct{ WorkspaceID string }) ([]*serviceresolvers.QueueResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.Queues(ctx, args.WorkspaceID)
}

// ConversationSession delegates to service resolver.
func (r *RootResolver) ConversationSession(ctx context.Context, args struct{ ID string }) (*serviceresolvers.ConversationSessionResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ConversationSession(ctx, args.ID)
}

// ConversationSessions delegates to service resolver.
func (r *RootResolver) ConversationSessions(ctx context.Context, args struct {
	WorkspaceID string
	Filter      *model.ConversationSessionFilterInput
}) ([]*serviceresolvers.ConversationSessionResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ConversationSessions(ctx, args.WorkspaceID, args.Filter)
}

// KnowledgeResource delegates to service resolver.
func (r *RootResolver) KnowledgeResource(ctx context.Context, args struct{ ID string }) (*serviceresolvers.KnowledgeResourceResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.KnowledgeResource(ctx, args.ID)
}

// KnowledgeResourceBySlug delegates to service resolver.
func (r *RootResolver) KnowledgeResourceBySlug(ctx context.Context, args struct {
	WorkspaceID string
	TeamID      string
	Surface     string
	Slug        string
}) (*serviceresolvers.KnowledgeResourceResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.KnowledgeResourceBySlug(ctx, args.WorkspaceID, args.TeamID, args.Surface, args.Slug)
}

// KnowledgeResources delegates to service resolver.
func (r *RootResolver) KnowledgeResources(ctx context.Context, args struct {
	WorkspaceID string
	Filter      *model.KnowledgeResourceFilterInput
}) ([]*serviceresolvers.KnowledgeResourceResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.KnowledgeResources(ctx, args.WorkspaceID, args.Filter)
}

// KnowledgeResourceHistory delegates to service resolver.
func (r *RootResolver) KnowledgeResourceHistory(ctx context.Context, args struct {
	ID    string
	Limit *int32
}) ([]*serviceresolvers.KnowledgeRevisionResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.KnowledgeResourceHistory(ctx, args.ID, args.Limit)
}

// KnowledgeResourceDiff delegates to service resolver.
func (r *RootResolver) KnowledgeResourceDiff(ctx context.Context, args struct {
	ID           string
	FromRevision *string
	ToRevision   *string
}) (*serviceresolvers.KnowledgeDiffResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.KnowledgeResourceDiff(ctx, args.ID, args.FromRevision, args.ToRevision)
}

// ConceptSpec delegates to service resolver.
func (r *RootResolver) ConceptSpec(ctx context.Context, args struct {
	WorkspaceID *string
	Key         string
	Version     *string
}) (*serviceresolvers.ConceptSpecResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ConceptSpec(ctx, valueOrEmpty(args.WorkspaceID), args.Key, args.Version)
}

// ConceptSpecs delegates to service resolver.
func (r *RootResolver) ConceptSpecs(ctx context.Context, args struct {
	WorkspaceID *string
}) ([]*serviceresolvers.ConceptSpecResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ConceptSpecs(ctx, args.WorkspaceID)
}

// ConceptSpecHistory delegates to service resolver.
func (r *RootResolver) ConceptSpecHistory(ctx context.Context, args struct {
	WorkspaceID *string
	Key         string
	Version     *string
	Limit       *int32
}) ([]*serviceresolvers.ConceptSpecRevisionResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ConceptSpecHistory(ctx, valueOrEmpty(args.WorkspaceID), args.Key, args.Version, args.Limit)
}

// ConceptSpecDiff delegates to service resolver.
func (r *RootResolver) ConceptSpecDiff(ctx context.Context, args struct {
	WorkspaceID  *string
	Key          string
	Version      *string
	FromRevision *string
	ToRevision   *string
}) (*serviceresolvers.ConceptSpecDiffResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ConceptSpecDiff(ctx, valueOrEmpty(args.WorkspaceID), args.Key, args.Version, args.FromRevision, args.ToRevision)
}

// ServiceCatalogNode delegates to service resolver.
func (r *RootResolver) ServiceCatalogNode(ctx context.Context, args struct{ ID string }) (*serviceresolvers.ServiceCatalogNodeResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ServiceCatalogNode(ctx, args.ID)
}

// ServiceCatalogNodeByPath delegates to service resolver.
func (r *RootResolver) ServiceCatalogNodeByPath(ctx context.Context, args struct {
	WorkspaceID string
	Path        string
}) (*serviceresolvers.ServiceCatalogNodeResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ServiceCatalogNodeByPath(ctx, args.WorkspaceID, args.Path)
}

// ServiceCatalogNodes delegates to service resolver.
func (r *RootResolver) ServiceCatalogNodes(ctx context.Context, args struct {
	WorkspaceID  string
	ParentNodeID *string
}) ([]*serviceresolvers.ServiceCatalogNodeResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ServiceCatalogNodes(ctx, args.WorkspaceID, args.ParentNodeID)
}

// FormSpec delegates to service resolver.
func (r *RootResolver) FormSpec(ctx context.Context, args struct{ ID string }) (*serviceresolvers.FormSpecResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.FormSpec(ctx, args.ID)
}

// FormSpecBySlug delegates to service resolver.
func (r *RootResolver) FormSpecBySlug(ctx context.Context, args struct {
	WorkspaceID string
	Slug        string
}) (*serviceresolvers.FormSpecResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.FormSpecBySlug(ctx, args.WorkspaceID, args.Slug)
}

// FormSpecs delegates to service resolver.
func (r *RootResolver) FormSpecs(ctx context.Context, args struct{ WorkspaceID string }) ([]*serviceresolvers.FormSpecResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.FormSpecs(ctx, args.WorkspaceID)
}

// FormSubmission delegates to service resolver.
func (r *RootResolver) FormSubmission(ctx context.Context, args struct{ ID string }) (*serviceresolvers.FormSubmissionResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.FormSubmission(ctx, args.ID)
}

// FormSubmissions delegates to service resolver.
func (r *RootResolver) FormSubmissions(ctx context.Context, args struct {
	WorkspaceID string
	Filter      *model.FormSubmissionFilterInput
}) ([]*serviceresolvers.FormSubmissionResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.FormSubmissions(ctx, args.WorkspaceID, args.Filter)
}

// Extension delegates to platform resolver.
func (r *RootResolver) Extension(ctx context.Context, args struct{ ID string }) (*platformresolvers.InstalledExtensionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.Extension(ctx, args.ID)
}

// Extensions delegates to platform resolver.
func (r *RootResolver) Extensions(ctx context.Context, args struct{ WorkspaceID string }) ([]*platformresolvers.InstalledExtensionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.Extensions(ctx, args.WorkspaceID)
}

// InstanceExtensions delegates to the platform resolver.
func (r *RootResolver) InstanceExtensions(ctx context.Context) ([]*platformresolvers.InstalledExtensionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.InstanceExtensions(ctx)
}

// WorkspaceExtensionAdminNavigation delegates to the platform resolver.
func (r *RootResolver) WorkspaceExtensionAdminNavigation(ctx context.Context, args struct{ WorkspaceID string }) ([]*platformresolvers.ResolvedExtensionAdminNavigationItemResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.WorkspaceExtensionAdminNavigation(ctx, args.WorkspaceID)
}

// WorkspaceExtensionDashboardWidgets delegates to the platform resolver.
func (r *RootResolver) WorkspaceExtensionDashboardWidgets(ctx context.Context, args struct{ WorkspaceID string }) ([]*platformresolvers.ResolvedExtensionDashboardWidgetResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.WorkspaceExtensionDashboardWidgets(ctx, args.WorkspaceID)
}

// InstanceExtensionAdminNavigation delegates to the platform resolver.
func (r *RootResolver) InstanceExtensionAdminNavigation(ctx context.Context) ([]*platformresolvers.ResolvedExtensionAdminNavigationItemResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.InstanceExtensionAdminNavigation(ctx)
}

// InstanceExtensionDashboardWidgets delegates to the platform resolver.
func (r *RootResolver) InstanceExtensionDashboardWidgets(ctx context.Context) ([]*platformresolvers.ResolvedExtensionDashboardWidgetResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.InstanceExtensionDashboardWidgets(ctx)
}

// ExtensionEventCatalog delegates to platform resolver.
func (r *RootResolver) ExtensionEventCatalog(ctx context.Context, args struct{ WorkspaceID string }) ([]*platformresolvers.ExtensionRuntimeEventResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.ExtensionEventCatalog(ctx, args.WorkspaceID)
}

// ExtensionArtifactFiles delegates to platform resolver.
func (r *RootResolver) ExtensionArtifactFiles(ctx context.Context, args struct {
	ID      string
	Surface string
}) ([]*platformresolvers.ExtensionArtifactFileResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.ExtensionArtifactFiles(ctx, args.ID, args.Surface)
}

// ExtensionArtifactContent delegates to platform resolver.
func (r *RootResolver) ExtensionArtifactContent(ctx context.Context, args struct {
	ID      string
	Surface string
	Path    string
	Ref     *string
}) (string, error) {
	if r.platform == nil {
		return "", fmt.Errorf("platform resolver not configured")
	}
	return r.platform.ExtensionArtifactContent(ctx, args.ID, args.Surface, args.Path, args.Ref)
}

// ExtensionArtifactHistory delegates to platform resolver.
func (r *RootResolver) ExtensionArtifactHistory(ctx context.Context, args struct {
	ID      string
	Surface string
	Path    string
	Limit   *int32
}) ([]*platformresolvers.ArtifactRevisionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.ExtensionArtifactHistory(ctx, args.ID, args.Surface, args.Path, args.Limit)
}

// ExtensionArtifactDiff delegates to platform resolver.
func (r *RootResolver) ExtensionArtifactDiff(ctx context.Context, args struct {
	ID           string
	Surface      string
	Path         string
	FromRevision *string
	ToRevision   *string
}) (*platformresolvers.ArtifactDiffResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.ExtensionArtifactDiff(ctx, args.ID, args.Surface, args.Path, args.FromRevision, args.ToRevision)
}

// --- Platform Domain Queries (delegated) ---

// Workspace delegates to platform resolver
func (r *RootResolver) Workspace(ctx context.Context, args struct{ ID string }) (*platformresolvers.WorkspaceResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.Workspace(ctx, args.ID)
}

// Workspaces returns all workspaces accessible to the current user
func (r *RootResolver) Workspaces(ctx context.Context) ([]*platformresolvers.WorkspaceResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.Workspaces(ctx)
}

// Team delegates to platform resolver.
func (r *RootResolver) Team(ctx context.Context, args struct{ ID string }) (*platformresolvers.TeamResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.Team(ctx, args.ID)
}

// Teams delegates to platform resolver.
func (r *RootResolver) Teams(ctx context.Context, args struct{ WorkspaceID string }) ([]*platformresolvers.TeamResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.Teams(ctx, args.WorkspaceID)
}

// Agent delegates to platform resolver
func (r *RootResolver) Agent(ctx context.Context, args struct{ ID string }) (*platformresolvers.AgentResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.Agent(ctx, args.ID)
}

// Agents delegates to platform resolver
func (r *RootResolver) Agents(ctx context.Context, args struct{ WorkspaceID string }) ([]*platformresolvers.AgentResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.Agents(ctx, args.WorkspaceID)
}

// Contacts delegates to platform resolver.
func (r *RootResolver) Contacts(ctx context.Context, args struct{ WorkspaceID string }) ([]*model.Contact, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.Contacts(ctx, args.WorkspaceID)
}

// --- Admin Queries (delegated) ---

// AdminUsers delegates to platform resolver
func (r *RootResolver) AdminUsers(ctx context.Context, args struct {
	Filter *model.AdminUserFilterInput
}) (*platformresolvers.AdminUserConnectionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.AdminUsers(ctx, args.Filter)
}

// AdminUser delegates to platform resolver
func (r *RootResolver) AdminUser(ctx context.Context, args struct{ ID string }) (*platformresolvers.AdminUserResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.AdminUser(ctx, args.ID)
}

// AdminUserWithWorkspaces delegates to platform resolver
func (r *RootResolver) AdminUserWithWorkspaces(ctx context.Context, args struct{ ID string }) (*platformresolvers.AdminUserWithWorkspacesResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.AdminUserWithWorkspaces(ctx, args.ID)
}

// AdminRules delegates to automation resolver
func (r *RootResolver) AdminRules(ctx context.Context, args struct {
	Filter *model.AdminRuleFilterInput
}) (*automationresolvers.AdminRuleConnectionResolver, error) {
	if r.automation == nil {
		return nil, fmt.Errorf("automation resolver not configured")
	}
	return r.automation.AdminRules(ctx, args.Filter)
}

// AdminRule delegates to automation resolver
func (r *RootResolver) AdminRule(ctx context.Context, args struct{ ID string }) (*automationresolvers.AdminRuleResolver, error) {
	if r.automation == nil {
		return nil, fmt.Errorf("automation resolver not configured")
	}
	return r.automation.AdminRule(ctx, args.ID)
}

// AdminForms delegates to automation resolver
func (r *RootResolver) AdminForms(ctx context.Context, args struct {
	Filter *model.AdminFormFilterInput
}) (*automationresolvers.AdminFormConnectionResolver, error) {
	if r.automation == nil {
		return nil, fmt.Errorf("automation resolver not configured")
	}
	return r.automation.AdminForms(ctx, args.Filter)
}

// AdminForm delegates to automation resolver
func (r *RootResolver) AdminForm(ctx context.Context, args struct{ ID string }) (*automationresolvers.AdminFormResolver, error) {
	if r.automation == nil {
		return nil, fmt.Errorf("automation resolver not configured")
	}
	return r.automation.AdminForm(ctx, args.ID)
}

// =============================================================================
// Mutation Resolvers
// =============================================================================

// --- Service Domain Mutations (delegated) ---

// CreateCase delegates to service resolver.
func (r *RootResolver) CreateCase(ctx context.Context, args struct {
	Input model.CreateCaseInput
}) (*serviceresolvers.CaseResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.CreateCase(ctx, args.Input)
}

// AddCommunication delegates to service resolver
func (r *RootResolver) AddCommunication(ctx context.Context, args struct {
	Input model.AddCommunicationInput
}) (*serviceresolvers.CommunicationResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.AddCommunication(ctx, args.Input)
}

// AddCaseNote delegates to service resolver.
func (r *RootResolver) AddCaseNote(ctx context.Context, args struct {
	ID   string
	Body string
}) (*serviceresolvers.CommunicationResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.AddCaseNote(ctx, args.ID, args.Body)
}

// ReplyToCase delegates to service resolver.
func (r *RootResolver) ReplyToCase(ctx context.Context, args struct {
	ID    string
	Input model.ReplyToCaseInput
}) (*serviceresolvers.CommunicationResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ReplyToCase(ctx, args.ID, args.Input)
}

// UpdateCaseStatus delegates to service resolver
func (r *RootResolver) UpdateCaseStatus(ctx context.Context, args struct {
	ID     string
	Status string
}) (*serviceresolvers.CaseResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.UpdateCaseStatus(ctx, args.ID, args.Status)
}

// SetCasePriority delegates to service resolver.
func (r *RootResolver) SetCasePriority(ctx context.Context, args struct {
	ID       string
	Priority string
}) (*serviceresolvers.CaseResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.SetCasePriority(ctx, args.ID, args.Priority)
}

// AssignCase delegates to service resolver
func (r *RootResolver) AssignCase(ctx context.Context, args struct {
	ID         string
	AssigneeID *string
}) (*serviceresolvers.CaseResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.AssignCase(ctx, args.ID, args.AssigneeID)
}

// HandoffCase delegates to service resolver.
func (r *RootResolver) HandoffCase(ctx context.Context, args struct {
	ID    string
	Input model.CaseHandoffInput
}) (*serviceresolvers.CaseResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.HandoffCase(ctx, args.ID, args.Input)
}

// CreateQueue delegates to service resolver.
func (r *RootResolver) CreateQueue(ctx context.Context, args struct {
	Input model.CreateQueueInput
}) (*serviceresolvers.QueueResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.CreateQueue(ctx, args.Input)
}

// UpdateQueue delegates to service resolver.
func (r *RootResolver) UpdateQueue(ctx context.Context, args struct {
	ID    string
	Input model.UpdateQueueInput
}) (*serviceresolvers.QueueResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.UpdateQueue(ctx, args.ID, args.Input)
}

// CreateFormSpec delegates to service resolver.
func (r *RootResolver) CreateFormSpec(ctx context.Context, args struct {
	Input model.CreateFormSpecInput
}) (*serviceresolvers.FormSpecResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.CreateFormSpec(ctx, args.Input)
}

// UpdateFormSpec delegates to service resolver.
func (r *RootResolver) UpdateFormSpec(ctx context.Context, args struct {
	ID    string
	Input model.UpdateFormSpecInput
}) (*serviceresolvers.FormSpecResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.UpdateFormSpec(ctx, args.ID, args.Input)
}

// CreateFormSubmission delegates to service resolver.
func (r *RootResolver) CreateFormSubmission(ctx context.Context, args struct {
	Input model.CreateFormSubmissionInput
}) (*serviceresolvers.FormSubmissionResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.CreateFormSubmission(ctx, args.Input)
}

// SetCaseQueue delegates to service resolver.
func (r *RootResolver) SetCaseQueue(ctx context.Context, args struct {
	CaseID  string
	QueueID *string
}) (*serviceresolvers.CaseResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.SetCaseQueue(ctx, args.CaseID, args.QueueID)
}

// AddConversationMessage delegates to service resolver.
func (r *RootResolver) AddConversationMessage(ctx context.Context, args struct {
	SessionID string
	Input     model.AddConversationMessageInput
}) (*serviceresolvers.ConversationMessageResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.AddConversationMessage(ctx, args.SessionID, args.Input)
}

// HandoffConversation delegates to service resolver.
func (r *RootResolver) HandoffConversation(ctx context.Context, args struct {
	SessionID string
	Input     model.ConversationHandoffInput
}) (*serviceresolvers.ConversationSessionResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.HandoffConversation(ctx, args.SessionID, args.Input)
}

// EscalateConversation delegates to service resolver.
func (r *RootResolver) EscalateConversation(ctx context.Context, args struct {
	SessionID string
	Input     model.EscalateConversationInput
}) (*serviceresolvers.CaseResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.EscalateConversation(ctx, args.SessionID, args.Input)
}

// CreateKnowledgeResource delegates to service resolver.
func (r *RootResolver) CreateKnowledgeResource(ctx context.Context, args struct {
	Input model.CreateKnowledgeResourceInput
}) (*serviceresolvers.KnowledgeResourceResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.CreateKnowledgeResource(ctx, args.Input)
}

// UpdateKnowledgeResource delegates to service resolver.
func (r *RootResolver) UpdateKnowledgeResource(ctx context.Context, args struct {
	ID    string
	Input model.UpdateKnowledgeResourceInput
}) (*serviceresolvers.KnowledgeResourceResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.UpdateKnowledgeResource(ctx, args.ID, args.Input)
}

// ReviewKnowledgeResource delegates to service resolver.
func (r *RootResolver) ReviewKnowledgeResource(ctx context.Context, args struct {
	ID     string
	Status *string
}) (*serviceresolvers.KnowledgeResourceResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ReviewKnowledgeResource(ctx, args.ID, args.Status)
}

// PublishKnowledgeResource delegates to service resolver.
func (r *RootResolver) PublishKnowledgeResource(ctx context.Context, args struct {
	ID      string
	Surface *string
}) (*serviceresolvers.KnowledgeResourceResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.PublishKnowledgeResource(ctx, args.ID, args.Surface)
}

// ShareKnowledgeResource delegates to service resolver.
func (r *RootResolver) ShareKnowledgeResource(ctx context.Context, args struct {
	ID    string
	Input model.ShareKnowledgeResourceInput
}) (*serviceresolvers.KnowledgeResourceResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.ShareKnowledgeResource(ctx, args.ID, args.Input)
}

// DeleteKnowledgeResource delegates to service resolver.
func (r *RootResolver) DeleteKnowledgeResource(ctx context.Context, args struct{ ID string }) (*serviceresolvers.KnowledgeResourceResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.DeleteKnowledgeResource(ctx, args.ID)
}

// RegisterConceptSpec delegates to service resolver.
func (r *RootResolver) RegisterConceptSpec(ctx context.Context, args struct {
	Input model.RegisterConceptSpecInput
}) (*serviceresolvers.ConceptSpecResolver, error) {
	if r.service == nil {
		return nil, fmt.Errorf("service resolver not configured")
	}
	return r.service.RegisterConceptSpec(ctx, args.Input)
}

// InstallExtension delegates to platform resolver.
func (r *RootResolver) InstallExtension(ctx context.Context, args struct {
	Input model.InstallExtensionInput
}) (*platformresolvers.InstalledExtensionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.InstallExtension(ctx, args.Input)
}

// UpgradeExtension delegates to platform resolver.
func (r *RootResolver) UpgradeExtension(ctx context.Context, args struct {
	ID    string
	Input model.UpgradeExtensionInput
}) (*platformresolvers.InstalledExtensionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.UpgradeExtension(ctx, args.ID, args.Input)
}

// ActivateExtension delegates to platform resolver.
func (r *RootResolver) ActivateExtension(ctx context.Context, args struct{ ID string }) (*platformresolvers.InstalledExtensionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.ActivateExtension(ctx, args.ID)
}

// DeactivateExtension delegates to platform resolver.
func (r *RootResolver) DeactivateExtension(ctx context.Context, args struct {
	ID     string
	Reason *string
}) (*platformresolvers.InstalledExtensionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.DeactivateExtension(ctx, args.ID, args.Reason)
}

// UninstallExtension delegates to platform resolver.
func (r *RootResolver) UninstallExtension(ctx context.Context, args struct{ ID string }) (bool, error) {
	if r.platform == nil {
		return false, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.UninstallExtension(ctx, args.ID)
}

// ValidateExtension delegates to platform resolver.
func (r *RootResolver) ValidateExtension(ctx context.Context, args struct{ ID string }) (*platformresolvers.InstalledExtensionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.ValidateExtension(ctx, args.ID)
}

// CheckExtensionHealth delegates to platform resolver.
func (r *RootResolver) CheckExtensionHealth(ctx context.Context, args struct{ ID string }) (*platformresolvers.InstalledExtensionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.CheckExtensionHealth(ctx, args.ID)
}

// UpdateExtensionConfig delegates to platform resolver.
func (r *RootResolver) UpdateExtensionConfig(ctx context.Context, args struct {
	ID    string
	Input model.UpdateExtensionConfigInput
}) (*platformresolvers.InstalledExtensionResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.UpdateExtensionConfig(ctx, args.ID, args.Input)
}

// UpdateExtensionAsset delegates to platform resolver.
func (r *RootResolver) UpdateExtensionAsset(ctx context.Context, args struct {
	ID    string
	Input model.UpdateExtensionAssetInput
}) (*platformresolvers.ExtensionAssetResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.UpdateExtensionAsset(ctx, args.ID, args.Input)
}

// PublishExtensionArtifact delegates to platform resolver.
func (r *RootResolver) PublishExtensionArtifact(ctx context.Context, args struct {
	ID    string
	Input model.PublishExtensionArtifactInput
}) (*platformresolvers.ExtensionArtifactPublicationResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.PublishExtensionArtifact(ctx, args.ID, args.Input)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// --- Platform Domain Mutations (delegated) ---

// CreateAgent delegates to platform resolver
func (r *RootResolver) CreateAgent(ctx context.Context, args struct {
	Input model.CreateAgentInput
}) (*platformresolvers.AgentResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.CreateAgent(ctx, args.Input)
}

// UpdateAgent delegates to platform resolver
func (r *RootResolver) UpdateAgent(ctx context.Context, args struct {
	ID    string
	Input model.UpdateAgentInput
}) (*platformresolvers.AgentResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.UpdateAgent(ctx, args.ID, args.Input)
}

// SuspendAgent delegates to platform resolver
func (r *RootResolver) SuspendAgent(ctx context.Context, args struct {
	ID     string
	Reason string
}) (*platformresolvers.AgentResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.SuspendAgent(ctx, args.ID, args.Reason)
}

// ActivateAgent delegates to platform resolver
func (r *RootResolver) ActivateAgent(ctx context.Context, args struct{ ID string }) (*platformresolvers.AgentResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.ActivateAgent(ctx, args.ID)
}

// RevokeAgent delegates to platform resolver
func (r *RootResolver) RevokeAgent(ctx context.Context, args struct {
	ID     string
	Reason string
}) (*platformresolvers.AgentResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.RevokeAgent(ctx, args.ID, args.Reason)
}

// CreateAgentToken delegates to platform resolver
func (r *RootResolver) CreateAgentToken(ctx context.Context, args struct {
	Input model.CreateAgentTokenInput
}) (*platformresolvers.AgentTokenResultResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.CreateAgentToken(ctx, args.Input)
}

// RevokeAgentToken delegates to platform resolver
func (r *RootResolver) RevokeAgentToken(ctx context.Context, args struct{ ID string }) (*platformresolvers.AgentTokenResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.RevokeAgentToken(ctx, args.ID)
}

// GrantAgentMembership delegates to platform resolver
func (r *RootResolver) GrantAgentMembership(ctx context.Context, args struct {
	Input model.GrantMembershipInput
}) (*platformresolvers.WorkspaceMembershipResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.GrantAgentMembership(ctx, args.Input)
}

// CreateTeam delegates to platform resolver.
func (r *RootResolver) CreateTeam(ctx context.Context, args struct {
	Input model.CreateTeamInput
}) (*platformresolvers.TeamResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.CreateTeam(ctx, args.Input)
}

// AddTeamMember delegates to platform resolver.
func (r *RootResolver) AddTeamMember(ctx context.Context, args struct {
	Input model.AddTeamMemberInput
}) (*platformresolvers.TeamMemberResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.AddTeamMember(ctx, args.Input)
}

// RevokeAgentMembership delegates to platform resolver
func (r *RootResolver) RevokeAgentMembership(ctx context.Context, args struct{ ID string }) (*platformresolvers.WorkspaceMembershipResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.RevokeAgentMembership(ctx, args.ID)
}

// --- Admin Mutations (delegated) ---

// AdminCreateUser delegates to platform resolver
func (r *RootResolver) AdminCreateUser(ctx context.Context, args struct {
	Input model.CreateUserInput
}) (*platformresolvers.AdminUserResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.AdminCreateUser(ctx, args.Input)
}

// AdminUpdateUser delegates to platform resolver
func (r *RootResolver) AdminUpdateUser(ctx context.Context, args struct {
	ID    string
	Input model.UpdateUserInput
}) (*platformresolvers.AdminUserResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.AdminUpdateUser(ctx, args.ID, args.Input)
}

// AdminToggleUserStatus delegates to platform resolver
func (r *RootResolver) AdminToggleUserStatus(ctx context.Context, args struct {
	ID       string
	IsActive bool
}) (*platformresolvers.AdminUserResolver, error) {
	if r.platform == nil {
		return nil, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.AdminToggleUserStatus(ctx, args.ID, args.IsActive)
}

// AdminDeleteUser delegates to platform resolver
func (r *RootResolver) AdminDeleteUser(ctx context.Context, args struct{ ID string }) (bool, error) {
	if r.platform == nil {
		return false, fmt.Errorf("platform resolver not configured")
	}
	return r.platform.AdminDeleteUser(ctx, args.ID)
}

// AdminCreateRule delegates to automation resolver
func (r *RootResolver) AdminCreateRule(ctx context.Context, args struct {
	Input model.CreateRuleInput
}) (*automationresolvers.AdminRuleResolver, error) {
	if r.automation == nil {
		return nil, fmt.Errorf("automation resolver not configured")
	}
	return r.automation.AdminCreateRule(ctx, args.Input)
}

// AdminUpdateRule delegates to automation resolver
func (r *RootResolver) AdminUpdateRule(ctx context.Context, args struct {
	ID    string
	Input model.UpdateRuleInput
}) (*automationresolvers.AdminRuleResolver, error) {
	if r.automation == nil {
		return nil, fmt.Errorf("automation resolver not configured")
	}
	return r.automation.AdminUpdateRule(ctx, args.ID, args.Input)
}

// AdminDeleteRule delegates to automation resolver
func (r *RootResolver) AdminDeleteRule(ctx context.Context, args struct{ ID string }) (bool, error) {
	if r.automation == nil {
		return false, fmt.Errorf("automation resolver not configured")
	}
	return r.automation.AdminDeleteRule(ctx, args.ID)
}

// AdminCreateForm delegates to automation resolver
func (r *RootResolver) AdminCreateForm(ctx context.Context, args struct {
	Input model.CreateFormInput
}) (*automationresolvers.AdminFormResolver, error) {
	if r.automation == nil {
		return nil, fmt.Errorf("automation resolver not configured")
	}
	return r.automation.AdminCreateForm(ctx, args.Input)
}

// AdminUpdateForm delegates to automation resolver
func (r *RootResolver) AdminUpdateForm(ctx context.Context, args struct {
	ID    string
	Input model.UpdateFormInput
}) (*automationresolvers.AdminFormResolver, error) {
	if r.automation == nil {
		return nil, fmt.Errorf("automation resolver not configured")
	}
	return r.automation.AdminUpdateForm(ctx, args.ID, args.Input)
}

// AdminDeleteForm delegates to automation resolver
func (r *RootResolver) AdminDeleteForm(ctx context.Context, args struct{ ID string }) (bool, error) {
	if r.automation == nil {
		return false, fmt.Errorf("automation resolver not configured")
	}
	return r.automation.AdminDeleteForm(ctx, args.ID)
}

// =============================================================================
// Principal Union Resolver
// =============================================================================

// PrincipalResolver resolves the Principal union type
type PrincipalResolver struct {
	user             *model.User
	agent            *model.Agent
	agentUserService *platformservices.UserManagementService
	agentService     *platformservices.AgentService
}

// ToUser resolves Principal to User
func (r *PrincipalResolver) ToUser() (*UserResolver, bool) {
	if r.user == nil {
		return nil, false
	}
	return &UserResolver{user: r.user}, true
}

// ToAgent resolves Principal to Agent
func (r *PrincipalResolver) ToAgent() (*AgentResolver, bool) {
	if r.agent == nil {
		return nil, false
	}
	return &AgentResolver{
		agent:        r.agent,
		userService:  r.agentUserService,
		agentService: r.agentService,
	}, true
}

// =============================================================================
// User Resolver (for Principal union)
// =============================================================================

// UserResolver resolves User fields
type UserResolver struct {
	user *model.User
}

// ID returns the user ID
func (r *UserResolver) ID() graphql.ID {
	return r.user.ID
}

// Email returns the user email
func (r *UserResolver) Email() string {
	return r.user.Email
}

// Name returns the user name
func (r *UserResolver) Name() string {
	return r.user.Name
}

// AvatarURL returns the user avatar URL
func (r *UserResolver) AvatarURL() *string {
	return r.user.AvatarURL
}

// =============================================================================
// Agent Resolver (for Principal union)
// =============================================================================

// AgentResolver resolves Agent fields for Principal union
type AgentResolver struct {
	agent *model.Agent

	userService  *platformservices.UserManagementService
	agentService *platformservices.AgentService
}

// ID returns the agent ID
func (r *AgentResolver) ID() graphql.ID {
	return r.agent.ID
}

// WorkspaceID returns the workspace ID
func (r *AgentResolver) WorkspaceID() graphql.ID {
	return r.agent.WorkspaceID
}

// Name returns the agent name
func (r *AgentResolver) Name() string {
	return r.agent.Name
}

// Description returns the agent description
func (r *AgentResolver) Description() *string {
	return r.agent.Description
}

// OwnerID returns the owner ID
func (r *AgentResolver) OwnerID() graphql.ID {
	return r.agent.OwnerID
}

// Owner resolves the owner user
func (r *AgentResolver) Owner(ctx context.Context) (*model.User, error) {
	if r.agent == nil {
		return nil, nil
	}

	if r.userService == nil {
		return nil, nil
	}

	owner, err := r.userService.GetUser(ctx, string(r.agent.OwnerID))
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get owner: %w", err)
	}

	return userFromDomain(owner), nil
}

// Status returns the agent status
func (r *AgentResolver) Status() string {
	return r.agent.Status
}

// StatusReason returns the status reason
func (r *AgentResolver) StatusReason() *string {
	return r.agent.StatusReason
}

// CreatedAt returns the creation timestamp
func (r *AgentResolver) CreatedAt() shared.DateTime {
	return shared.DateTime{Time: r.agent.CreatedAt}
}

// UpdatedAt returns the update timestamp
func (r *AgentResolver) UpdatedAt() shared.DateTime {
	return shared.DateTime{Time: r.agent.UpdatedAt}
}

// CreatedByID returns the creator ID
func (r *AgentResolver) CreatedByID() graphql.ID {
	return r.agent.CreatedByID
}

// Tokens resolves the agent's tokens
func (r *AgentResolver) Tokens(ctx context.Context) ([]*AgentTokenResolver, error) {
	if r.agent == nil {
		return []*AgentTokenResolver{}, nil
	}

	if r.userService == nil || r.agentService == nil {
		return []*AgentTokenResolver{}, nil
	}

	tokens, err := r.agentService.ListAgentTokens(ctx, string(r.agent.ID))
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return []*AgentTokenResolver{}, nil
		}
		return nil, fmt.Errorf("failed to list tokens: %w", err)
	}

	result := make([]*AgentTokenResolver, len(tokens))
	for i, token := range tokens {
		result[i] = &AgentTokenResolver{token: agentTokenFromDomain(token)}
	}
	return result, nil
}

// Membership resolves the agent's workspace membership
func (r *AgentResolver) Membership(ctx context.Context) (*WorkspaceMembershipResolver, error) {
	if r.agent == nil {
		return nil, nil
	}

	if r.agentService == nil {
		return nil, nil
	}

	membership, err := r.agentService.GetWorkspaceMembership(ctx, string(r.agent.WorkspaceID), string(r.agent.ID), platformdomain.PrincipalTypeAgent)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get membership: %w", err)
	}

	return &WorkspaceMembershipResolver{membership: workspaceMembershipFromDomain(membership)}, nil
}

// =============================================================================
// AgentToken Resolver (for Principal union -> Agent -> Tokens)
// =============================================================================

// AgentTokenResolver resolves AgentToken fields
type AgentTokenResolver struct {
	token *model.AgentToken
}

// ID returns the token ID
func (r *AgentTokenResolver) ID() graphql.ID {
	return r.token.ID
}

// AgentID returns the agent ID
func (r *AgentTokenResolver) AgentID() graphql.ID {
	return r.token.AgentID
}

// TokenPrefix returns the token prefix
func (r *AgentTokenResolver) TokenPrefix() string {
	return r.token.TokenPrefix
}

// Name returns the token name
func (r *AgentTokenResolver) Name() string {
	return r.token.Name
}

// ExpiresAt returns the expiration timestamp
func (r *AgentTokenResolver) ExpiresAt() *shared.DateTime {
	if r.token.ExpiresAt == nil {
		return nil
	}
	return &shared.DateTime{Time: *r.token.ExpiresAt}
}

// RevokedAt returns the revocation timestamp
func (r *AgentTokenResolver) RevokedAt() *shared.DateTime {
	if r.token.RevokedAt == nil {
		return nil
	}
	return &shared.DateTime{Time: *r.token.RevokedAt}
}

// LastUsedAt returns the last used timestamp
func (r *AgentTokenResolver) LastUsedAt() *shared.DateTime {
	if r.token.LastUsedAt == nil {
		return nil
	}
	return &shared.DateTime{Time: *r.token.LastUsedAt}
}

// LastUsedIP returns the last used IP
func (r *AgentTokenResolver) LastUsedIP() *string {
	if r.token.LastUsedIP == nil {
		return nil
	}
	return r.token.LastUsedIP
}

// UseCount returns the use count
func (r *AgentTokenResolver) UseCount() int32 {
	return r.token.UseCount
}

// CreatedAt returns the creation timestamp
func (r *AgentTokenResolver) CreatedAt() shared.DateTime {
	return shared.DateTime{Time: r.token.CreatedAt}
}

// CreatedByID returns the creator ID
func (r *AgentTokenResolver) CreatedByID() graphql.ID {
	return r.token.CreatedByID
}

// =============================================================================
// WorkspaceMembership Resolver (for Principal union -> Agent -> Membership)
// =============================================================================

// WorkspaceMembershipResolver resolves WorkspaceMembership fields
type WorkspaceMembershipResolver struct {
	membership *model.WorkspaceMembership
}

// MembershipConstraintsResolver resolves scoped membership constraints.
type MembershipConstraintsResolver struct {
	constraints model.MembershipConstraints
}

// ID returns the membership ID
func (r *WorkspaceMembershipResolver) ID() graphql.ID {
	return r.membership.ID
}

// WorkspaceID returns the workspace ID
func (r *WorkspaceMembershipResolver) WorkspaceID() graphql.ID {
	return r.membership.WorkspaceID
}

// PrincipalID returns the principal ID
func (r *WorkspaceMembershipResolver) PrincipalID() graphql.ID {
	return r.membership.PrincipalID
}

// PrincipalType returns the principal type
func (r *WorkspaceMembershipResolver) PrincipalType() string {
	return r.membership.PrincipalType
}

// Role returns the role
func (r *WorkspaceMembershipResolver) Role() string {
	return r.membership.Role
}

// Permissions returns the permissions
func (r *WorkspaceMembershipResolver) Permissions() []string {
	return r.membership.Permissions
}

// Constraints returns the membership constraints.
func (r *WorkspaceMembershipResolver) Constraints() *MembershipConstraintsResolver {
	return &MembershipConstraintsResolver{constraints: r.membership.Constraints}
}

// GrantedAt returns the grant timestamp
func (r *WorkspaceMembershipResolver) GrantedAt() shared.DateTime {
	return shared.DateTime{Time: r.membership.GrantedAt}
}

// ExpiresAt returns the expiration timestamp
func (r *WorkspaceMembershipResolver) ExpiresAt() *shared.DateTime {
	if r.membership.ExpiresAt == nil {
		return nil
	}
	return &shared.DateTime{Time: *r.membership.ExpiresAt}
}

// RevokedAt returns the revocation timestamp
func (r *WorkspaceMembershipResolver) RevokedAt() *shared.DateTime {
	if r.membership.RevokedAt == nil {
		return nil
	}
	return &shared.DateTime{Time: *r.membership.RevokedAt}
}

func (r *MembershipConstraintsResolver) RateLimitPerMinute() *int32 {
	return r.constraints.RateLimitPerMinute
}

func (r *MembershipConstraintsResolver) RateLimitPerHour() *int32 {
	return r.constraints.RateLimitPerHour
}

func (r *MembershipConstraintsResolver) AllowedIPs() []string {
	return append([]string(nil), r.constraints.AllowedIPs...)
}

func (r *MembershipConstraintsResolver) AllowedProjectIDs() []graphql.ID {
	result := make([]graphql.ID, len(r.constraints.AllowedProjectIDs))
	copy(result, r.constraints.AllowedProjectIDs)
	return result
}

func (r *MembershipConstraintsResolver) AllowedTeamIDs() []graphql.ID {
	result := make([]graphql.ID, len(r.constraints.AllowedTeamIDs))
	copy(result, r.constraints.AllowedTeamIDs)
	return result
}

func (r *MembershipConstraintsResolver) AllowDelegatedRouting() bool {
	return r.constraints.AllowDelegatedRouting
}

func (r *MembershipConstraintsResolver) DelegatedRoutingTeamIDs() []graphql.ID {
	result := make([]graphql.ID, len(r.constraints.DelegatedRoutingTeamIDs))
	copy(result, r.constraints.DelegatedRoutingTeamIDs)
	return result
}

func (r *MembershipConstraintsResolver) ActiveHoursStart() *string {
	return r.constraints.ActiveHoursStart
}

func (r *MembershipConstraintsResolver) ActiveHoursEnd() *string {
	return r.constraints.ActiveHoursEnd
}

func (r *MembershipConstraintsResolver) ActiveTimezone() *string {
	return r.constraints.ActiveTimezone
}

func (r *MembershipConstraintsResolver) ActiveDays() []int32 {
	return append([]int32(nil), r.constraints.ActiveDays...)
}

// =============================================================================
// Helper Converters
// =============================================================================

func userFromDomain(u *platformdomain.User) *model.User {
	if u == nil {
		return nil
	}
	var avatarURL *string
	if u.Avatar != "" {
		avatarURL = &u.Avatar
	}
	return &model.User{
		ID:        model.ID(u.ID),
		Email:     u.Email,
		Name:      u.Name,
		AvatarURL: avatarURL,
	}
}

func agentFromDomain(a *platformdomain.Agent) *model.Agent {
	if a == nil {
		return nil
	}
	var description, statusReason *string
	if a.Description != "" {
		description = &a.Description
	}
	if a.StatusReason != "" {
		statusReason = &a.StatusReason
	}
	return &model.Agent{
		ID:           model.ID(a.ID),
		WorkspaceID:  model.ID(a.WorkspaceID),
		Name:         a.Name,
		Description:  description,
		OwnerID:      model.ID(a.OwnerID),
		Status:       string(a.Status),
		StatusReason: statusReason,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
		CreatedByID:  model.ID(a.CreatedByID),
	}
}

func agentTokenFromDomain(token *platformdomain.AgentToken) *model.AgentToken {
	if token == nil {
		return nil
	}

	var lastUsedIP *string
	if token.LastUsedIP != "" {
		lastUsedIP = &token.LastUsedIP
	}

	return &model.AgentToken{
		ID:          model.ID(token.ID),
		AgentID:     model.ID(token.AgentID),
		TokenPrefix: token.TokenPrefix,
		Name:        token.Name,
		ExpiresAt:   token.ExpiresAt,
		RevokedAt:   token.RevokedAt,
		LastUsedAt:  token.LastUsedAt,
		LastUsedIP:  lastUsedIP,
		UseCount:    int32(token.UseCount),
		CreatedAt:   token.CreatedAt,
		CreatedByID: model.ID(token.CreatedByID),
	}
}

func workspaceMembershipFromDomain(membership *platformdomain.WorkspaceMembership) *model.WorkspaceMembership {
	if membership == nil {
		return nil
	}

	var expiresAt *time.Time
	if membership.ExpiresAt != nil {
		t := *membership.ExpiresAt
		expiresAt = &t
	}

	var revokedAt *time.Time
	if membership.RevokedAt != nil {
		t := *membership.RevokedAt
		revokedAt = &t
	}

	return &model.WorkspaceMembership{
		ID:            model.ID(membership.ID),
		WorkspaceID:   model.ID(membership.WorkspaceID),
		PrincipalID:   model.ID(membership.PrincipalID),
		PrincipalType: string(membership.PrincipalType),
		Role:          membership.Role,
		Permissions:   membership.Permissions,
		Constraints:   membershipConstraintsFromDomain(membership.Constraints),
		GrantedAt:     membership.GrantedAt,
		ExpiresAt:     expiresAt,
		RevokedAt:     revokedAt,
	}
}

func membershipConstraintsFromDomain(constraints platformdomain.MembershipConstraints) model.MembershipConstraints {
	return model.MembershipConstraints{
		RateLimitPerMinute:      optionalInt32(constraints.RateLimitPerMinute),
		RateLimitPerHour:        optionalInt32(constraints.RateLimitPerHour),
		AllowedIPs:              append([]string(nil), constraints.AllowedIPs...),
		AllowedProjectIDs:       toModelIDs(constraints.AllowedProjectIDs),
		AllowedTeamIDs:          toModelIDs(constraints.AllowedTeamIDs),
		AllowDelegatedRouting:   constraints.AllowDelegatedRouting,
		DelegatedRoutingTeamIDs: toModelIDs(constraints.DelegatedRoutingTeamIDs),
		ActiveHoursStart:        optionalTrimmedString(constraints.ActiveHoursStart),
		ActiveHoursEnd:          optionalTrimmedString(constraints.ActiveHoursEnd),
		ActiveTimezone:          optionalTrimmedString(constraints.ActiveTimezone),
		ActiveDays:              toInt32Slice(constraints.ActiveDays),
	}
}

func toModelIDs(values []string) []model.ID {
	result := make([]model.ID, len(values))
	for i, value := range values {
		result[i] = model.ID(value)
	}
	return result
}

func optionalInt32(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func optionalTrimmedString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func toInt32Slice(values []int) []int32 {
	result := make([]int32, len(values))
	for i, value := range values {
		result[i] = int32(value)
	}
	return result
}
