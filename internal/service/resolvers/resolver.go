// Package resolvers provides GraphQL resolvers for the service domain.
// This domain owns the Case, Communication, and Form API surface.
package resolvers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	"github.com/movebigrocks/platform/internal/graph/model"
	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	knowledgeservices "github.com/movebigrocks/platform/internal/knowledge/services"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

// Config holds the dependencies for service domain resolvers
type Config struct {
	QueueService        *serviceapp.QueueService
	ConversationService *serviceapp.ConversationService
	CatalogService      *serviceapp.ServiceCatalogService
	FormSpecService     *serviceapp.FormSpecService
	ConceptService      *knowledgeservices.ConceptSpecService
	KnowledgeService    *knowledgeservices.KnowledgeService
	CaseService         *serviceapp.CaseService
	UserService         *platformservices.UserManagementService
	ContactService      *platformservices.ContactService
	AgentService        *platformservices.AgentService
}

// Resolver handles all service domain GraphQL operations
type Resolver struct {
	queueService        *serviceapp.QueueService
	conversationService *serviceapp.ConversationService
	catalogService      *serviceapp.ServiceCatalogService
	formSpecService     *serviceapp.FormSpecService
	conceptService      *knowledgeservices.ConceptSpecService
	knowledgeService    *knowledgeservices.KnowledgeService
	caseService         *serviceapp.CaseService
	userService         *platformservices.UserManagementService
	contactService      *platformservices.ContactService
	agentService        *platformservices.AgentService
}

// NewResolver creates a new service domain resolver
func NewResolver(cfg Config) *Resolver {
	return &Resolver{
		queueService:        cfg.QueueService,
		conversationService: cfg.ConversationService,
		catalogService:      cfg.CatalogService,
		formSpecService:     cfg.FormSpecService,
		conceptService:      cfg.ConceptService,
		knowledgeService:    cfg.KnowledgeService,
		caseService:         cfg.CaseService,
		userService:         cfg.UserService,
		contactService:      cfg.ContactService,
		agentService:        cfg.AgentService,
	}
}

// NewKnowledgeResourceResolver wraps a knowledge resource for GraphQL.
func (r *Resolver) NewKnowledgeResourceResolver(resource *knowledgedomain.KnowledgeResource) *KnowledgeResourceResolver {
	if resource == nil {
		return nil
	}
	return &KnowledgeResourceResolver{resource: resource, r: r}
}

// NewKnowledgeResourceResolvers wraps a knowledge resource slice for GraphQL.
func (r *Resolver) NewKnowledgeResourceResolvers(resources []*knowledgedomain.KnowledgeResource) []*KnowledgeResourceResolver {
	result := make([]*KnowledgeResourceResolver, 0, len(resources))
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		result = append(result, r.NewKnowledgeResourceResolver(resource))
	}
	return result
}

// NewConceptSpecResolver wraps a concept spec for GraphQL.
func (r *Resolver) NewConceptSpecResolver(spec *knowledgedomain.ConceptSpec) *ConceptSpecResolver {
	if spec == nil {
		return nil
	}
	return &ConceptSpecResolver{spec: spec}
}

// NewConceptSpecResolvers wraps a concept spec slice for GraphQL.
func (r *Resolver) NewConceptSpecResolvers(specs []*knowledgedomain.ConceptSpec) []*ConceptSpecResolver {
	result := make([]*ConceptSpecResolver, 0, len(specs))
	for _, spec := range specs {
		if spec == nil {
			continue
		}
		result = append(result, r.NewConceptSpecResolver(spec))
	}
	return result
}

// NewServiceCatalogNodeResolver wraps a service catalog node for GraphQL.
func (r *Resolver) NewServiceCatalogNodeResolver(node *servicedomain.ServiceCatalogNode) *ServiceCatalogNodeResolver {
	if node == nil {
		return nil
	}
	return &ServiceCatalogNodeResolver{node: node, r: r}
}

// NewServiceCatalogNodeResolvers wraps a service catalog node slice for GraphQL.
func (r *Resolver) NewServiceCatalogNodeResolvers(nodes []*servicedomain.ServiceCatalogNode) []*ServiceCatalogNodeResolver {
	result := make([]*ServiceCatalogNodeResolver, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		result = append(result, r.NewServiceCatalogNodeResolver(node))
	}
	return result
}

// NewFormSpecResolver wraps a form spec for GraphQL.
func (r *Resolver) NewFormSpecResolver(spec *servicedomain.FormSpec) *FormSpecResolver {
	if spec == nil {
		return nil
	}
	return &FormSpecResolver{spec: spec}
}

// NewFormSpecResolvers wraps a form spec slice for GraphQL.
func (r *Resolver) NewFormSpecResolvers(specs []*servicedomain.FormSpec) []*FormSpecResolver {
	result := make([]*FormSpecResolver, 0, len(specs))
	for _, spec := range specs {
		if spec == nil {
			continue
		}
		result = append(result, r.NewFormSpecResolver(spec))
	}
	return result
}

// NewFormSubmissionResolver wraps a form submission for GraphQL.
func (r *Resolver) NewFormSubmissionResolver(submission *servicedomain.FormSubmission) *FormSubmissionResolver {
	if submission == nil {
		return nil
	}
	return &FormSubmissionResolver{submission: submission}
}

// NewFormSubmissionResolvers wraps a form submission slice for GraphQL.
func (r *Resolver) NewFormSubmissionResolvers(submissions []*servicedomain.FormSubmission) []*FormSubmissionResolver {
	result := make([]*FormSubmissionResolver, 0, len(submissions))
	for _, submission := range submissions {
		if submission == nil {
			continue
		}
		result = append(result, r.NewFormSubmissionResolver(submission))
	}
	return result
}

// NewQueueResolver wraps a queue domain object for GraphQL.
func (r *Resolver) NewQueueResolver(queue *servicedomain.Queue) *QueueResolver {
	if queue == nil {
		return nil
	}
	return &QueueResolver{queue: queue, r: r}
}

// NewQueueResolvers wraps a queue slice for GraphQL.
func (r *Resolver) NewQueueResolvers(queues []*servicedomain.Queue) []*QueueResolver {
	result := make([]*QueueResolver, 0, len(queues))
	for _, queue := range queues {
		if queue == nil {
			continue
		}
		result = append(result, r.NewQueueResolver(queue))
	}
	return result
}

func (r *Resolver) NewQueueItemResolver(item *servicedomain.QueueItem) *QueueItemResolver {
	if item == nil {
		return nil
	}
	return &QueueItemResolver{item: item, r: r}
}

func (r *Resolver) NewQueueItemResolvers(items []*servicedomain.QueueItem) []*QueueItemResolver {
	result := make([]*QueueItemResolver, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, r.NewQueueItemResolver(item))
	}
	return result
}

// NewConversationSessionResolver wraps a conversation session for GraphQL.
func (r *Resolver) NewConversationSessionResolver(session *servicedomain.ConversationSession) *ConversationSessionResolver {
	if session == nil {
		return nil
	}
	return &ConversationSessionResolver{session: session, r: r}
}

// NewConversationSessionResolvers wraps conversation sessions for GraphQL.
func (r *Resolver) NewConversationSessionResolvers(sessions []*servicedomain.ConversationSession) []*ConversationSessionResolver {
	result := make([]*ConversationSessionResolver, 0, len(sessions))
	for _, session := range sessions {
		if session == nil {
			continue
		}
		result = append(result, r.NewConversationSessionResolver(session))
	}
	return result
}

// NewCaseResolver wraps a case domain object for GraphQL.
func (r *Resolver) NewCaseResolver(caseObj *servicedomain.Case) *CaseResolver {
	if caseObj == nil {
		return nil
	}
	return &CaseResolver{case_: caseObj, r: r}
}

// NewCaseConnectionResolver wraps a case slice for GraphQL relay connections.
func (r *Resolver) NewCaseConnectionResolver(cases []*servicedomain.Case, total, limit int) *CaseConnectionResolver {
	return &CaseConnectionResolver{
		cases: cases,
		total: total,
		limit: limit,
		r:     r,
	}
}

// =============================================================================
// Query Resolvers
// =============================================================================

// Queue resolves a queue by ID.
func (r *Resolver) Queue(ctx context.Context, id string) (*QueueResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionQueueRead)
	if err != nil {
		return nil, err
	}

	queue, err := r.queueService.GetQueue(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("queue not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(queue.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("queue not found")
	}
	return r.NewQueueResolver(queue), nil
}

// Queues resolves queues for a workspace.
func (r *Resolver) Queues(ctx context.Context, workspaceID string) ([]*QueueResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionQueueRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	queues, err := r.queueService.ListWorkspaceQueues(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list queues: %w", err)
	}

	result := make([]*QueueResolver, len(queues))
	for i, queue := range queues {
		result[i] = r.NewQueueResolver(queue)
	}
	return result, nil
}

// ConversationSession resolves a conversation by ID.
func (r *Resolver) ConversationSession(ctx context.Context, id string) (*ConversationSessionResolver, error) {
	if r.conversationService == nil {
		return nil, fmt.Errorf("conversation service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionConversationRead)
	if err != nil {
		return nil, err
	}
	session, err := r.conversationService.GetConversationSession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("conversation session not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(session.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("conversation session not found")
	}
	return r.NewConversationSessionResolver(session), nil
}

// ConversationSessions resolves conversations in a workspace.
func (r *Resolver) ConversationSessions(ctx context.Context, workspaceID string, filter *model.ConversationSessionFilterInput) ([]*ConversationSessionResolver, error) {
	if r.conversationService == nil {
		return nil, fmt.Errorf("conversation service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionConversationRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	params := servicedomain.ConversationSessionFilter{}
	if filter != nil {
		if filter.Status != nil {
			status, err := serviceapp.NormalizeConversationStatus(*filter.Status)
			if err != nil {
				return nil, err
			}
			params.Status = status
		}
		if filter.Channel != nil {
			channel, err := serviceapp.NormalizeConversationChannel(*filter.Channel)
			if err != nil {
				return nil, err
			}
			params.Channel = channel
		}
		if filter.PrimaryCatalogNodeID != nil {
			params.PrimaryCatalogNodeID = strings.TrimSpace(*filter.PrimaryCatalogNodeID)
		}
		if filter.PrimaryContactID != nil {
			params.PrimaryContactID = strings.TrimSpace(*filter.PrimaryContactID)
		}
		if filter.LinkedCaseID != nil {
			params.LinkedCaseID = strings.TrimSpace(*filter.LinkedCaseID)
		}
		if filter.Limit != nil {
			params.Limit = int(*filter.Limit)
		}
		if filter.Offset != nil {
			params.Offset = int(*filter.Offset)
		}
	}
	sessions, err := r.conversationService.ListWorkspaceConversationSessions(ctx, workspaceID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversation sessions: %w", err)
	}
	return r.NewConversationSessionResolvers(sessions), nil
}

// KnowledgeResource resolves a knowledge resource by ID.
func (r *Resolver) KnowledgeResource(ctx context.Context, id string) (*KnowledgeResourceResolver, error) {
	if r.knowledgeService == nil {
		return nil, fmt.Errorf("knowledge service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeRead)
	if err != nil {
		return nil, err
	}
	resource, err := r.knowledgeService.GetKnowledgeResource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(resource.WorkspaceID, authCtx); err != nil || !canAccessKnowledgeResource(authCtx, resource) {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	return r.NewKnowledgeResourceResolver(resource), nil
}

// KnowledgeResourceBySlug resolves a knowledge resource by workspace slug.
func (r *Resolver) KnowledgeResourceBySlug(ctx context.Context, workspaceID, teamID, surface, slug string) (*KnowledgeResourceResolver, error) {
	if r.knowledgeService == nil {
		return nil, fmt.Errorf("knowledge service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	normalizedSurface, err := normalizeKnowledgeSurface(surface)
	if err != nil {
		return nil, err
	}
	resource, err := r.knowledgeService.GetKnowledgeResourceBySlug(ctx, workspaceID, teamID, normalizedSurface, slug)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	if !canAccessKnowledgeResource(authCtx, resource) {
		return nil, nil
	}
	return r.NewKnowledgeResourceResolver(resource), nil
}

// KnowledgeResources resolves knowledge resources for a workspace.
func (r *Resolver) KnowledgeResources(ctx context.Context, workspaceID string, filter *model.KnowledgeResourceFilterInput) ([]*KnowledgeResourceResolver, error) {
	if r.knowledgeService == nil {
		return nil, fmt.Errorf("knowledge service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	params := knowledgedomain.KnowledgeResourceFilter{}
	if filter != nil {
		if filter.TeamID != nil {
			params.TeamID = *filter.TeamID
		}
		if filter.Kind != nil {
			params.Kind = knowledgedomain.KnowledgeResourceKind(*filter.Kind)
		}
		if filter.Status != nil {
			params.Status = knowledgedomain.KnowledgeResourceStatus(*filter.Status)
		}
		if filter.Surface != nil {
			surface, err := normalizeKnowledgeSurface(*filter.Surface)
			if err != nil {
				return nil, err
			}
			params.Surface = surface
		}
		if filter.ReviewStatus != nil {
			status, err := normalizeKnowledgeReviewStatus(*filter.ReviewStatus)
			if err != nil {
				return nil, err
			}
			params.ReviewStatus = status
		}
		if filter.Search != nil {
			params.Search = *filter.Search
		}
		if filter.Limit != nil {
			params.Limit = int(*filter.Limit)
		}
		if filter.Offset != nil {
			params.Offset = int(*filter.Offset)
		}
	}

	resources, _, err := r.knowledgeService.ListWorkspaceKnowledgeResources(ctx, workspaceID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list knowledge resources: %w", err)
	}
	filtered := make([]*knowledgedomain.KnowledgeResource, 0, len(resources))
	for _, resource := range resources {
		if canAccessKnowledgeResource(authCtx, resource) {
			filtered = append(filtered, resource)
		}
	}
	return r.NewKnowledgeResourceResolvers(filtered), nil
}

// KnowledgeResourceHistory resolves git-backed revision history for a knowledge resource.
func (r *Resolver) KnowledgeResourceHistory(ctx context.Context, id string, limit *int32) ([]*KnowledgeRevisionResolver, error) {
	resourceResolver, err := r.KnowledgeResource(ctx, id)
	if err != nil {
		return nil, err
	}
	if resourceResolver == nil {
		return nil, nil
	}

	historyLimit := 20
	if limit != nil {
		historyLimit = int(*limit)
	}
	revisions, err := r.knowledgeService.KnowledgeHistory(ctx, id, historyLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to load knowledge history: %w", err)
	}
	result := make([]*KnowledgeRevisionResolver, len(revisions))
	for i := range revisions {
		revision := revisions[i]
		result[i] = &KnowledgeRevisionResolver{revision: revision}
	}
	return result, nil
}

// KnowledgeResourceDiff resolves a patch between knowledge revisions.
func (r *Resolver) KnowledgeResourceDiff(ctx context.Context, id string, fromRevision, toRevision *string) (*KnowledgeDiffResolver, error) {
	resourceResolver, err := r.KnowledgeResource(ctx, id)
	if err != nil {
		return nil, err
	}
	if resourceResolver == nil {
		return nil, nil
	}

	diff, err := r.knowledgeService.KnowledgeDiff(ctx, id, valueOrEmpty(fromRevision), valueOrEmpty(toRevision))
	if err != nil {
		return nil, fmt.Errorf("failed to load knowledge diff: %w", err)
	}
	return &KnowledgeDiffResolver{diff: diff}, nil
}

// ConceptSpecHistory resolves git-backed concept spec revision history.
func (r *Resolver) ConceptSpecHistory(ctx context.Context, workspaceID, key string, version *string, limit *int32) ([]*ConceptSpecRevisionResolver, error) {
	if r.conceptService == nil {
		return nil, fmt.Errorf("concept service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeRead)
	if err != nil {
		return nil, err
	}
	workspaceValue := strings.TrimSpace(workspaceID)
	if workspaceValue != "" {
		if err := graphshared.ValidateWorkspaceOwnership(workspaceValue, authCtx); err != nil {
			return nil, fmt.Errorf("workspace not found")
		}
	}
	historyLimit := 20
	if limit != nil {
		historyLimit = int(*limit)
	}
	revisions, err := r.conceptService.ConceptSpecHistory(ctx, workspaceValue, key, valueOrEmpty(version), historyLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to load concept spec history: %w", err)
	}
	result := make([]*ConceptSpecRevisionResolver, 0, len(revisions))
	for _, revision := range revisions {
		result = append(result, &ConceptSpecRevisionResolver{revision: revision})
	}
	return result, nil
}

// ConceptSpecDiff resolves a patch between concept spec revisions.
func (r *Resolver) ConceptSpecDiff(ctx context.Context, workspaceID, key string, version, fromRevision, toRevision *string) (*ConceptSpecDiffResolver, error) {
	if r.conceptService == nil {
		return nil, fmt.Errorf("concept service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeRead)
	if err != nil {
		return nil, err
	}
	workspaceValue := strings.TrimSpace(workspaceID)
	if workspaceValue != "" {
		if err := graphshared.ValidateWorkspaceOwnership(workspaceValue, authCtx); err != nil {
			return nil, fmt.Errorf("workspace not found")
		}
	}
	diff, err := r.conceptService.ConceptSpecDiff(ctx, workspaceValue, key, valueOrEmpty(version), valueOrEmpty(fromRevision), valueOrEmpty(toRevision))
	if err != nil {
		return nil, fmt.Errorf("failed to load concept spec diff: %w", err)
	}
	return &ConceptSpecDiffResolver{diff: diff}, nil
}

// ConceptSpec resolves a concept spec by key and version.
func (r *Resolver) ConceptSpec(ctx context.Context, workspaceID, key string, version *string) (*ConceptSpecResolver, error) {
	if r.conceptService == nil {
		return nil, fmt.Errorf("concept service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeRead)
	if err != nil {
		return nil, err
	}
	workspaceValue := strings.TrimSpace(workspaceID)
	if workspaceValue != "" {
		if err := graphshared.ValidateWorkspaceOwnership(workspaceValue, authCtx); err != nil {
			return nil, fmt.Errorf("workspace not found")
		}
	}
	spec, err := r.conceptService.GetConceptSpec(ctx, workspaceValue, key, valueOrEmpty(version))
	if err != nil {
		return nil, fmt.Errorf("concept spec not found")
	}
	return r.NewConceptSpecResolver(spec), nil
}

// ConceptSpecs resolves built-in and workspace concept specs.
func (r *Resolver) ConceptSpecs(ctx context.Context, workspaceID *string) ([]*ConceptSpecResolver, error) {
	if r.conceptService == nil {
		return nil, fmt.Errorf("concept service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeRead)
	if err != nil {
		return nil, err
	}
	workspaceValue := valueOrEmpty(workspaceID)
	if workspaceValue != "" {
		if err := graphshared.ValidateWorkspaceOwnership(workspaceValue, authCtx); err != nil {
			return nil, fmt.Errorf("workspace not found")
		}
	}
	specs, err := r.conceptService.ListConceptSpecs(ctx, workspaceValue)
	if err != nil {
		return nil, fmt.Errorf("failed to list concept specs: %w", err)
	}
	return r.NewConceptSpecResolvers(specs), nil
}

// ServiceCatalogNode resolves a service catalog node by ID.
func (r *Resolver) ServiceCatalogNode(ctx context.Context, id string) (*ServiceCatalogNodeResolver, error) {
	if r.catalogService == nil {
		return nil, fmt.Errorf("catalog service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCatalogRead)
	if err != nil {
		return nil, err
	}
	node, err := r.catalogService.GetServiceCatalogNode(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service catalog node not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(node.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("service catalog node not found")
	}
	return r.NewServiceCatalogNodeResolver(node), nil
}

// ServiceCatalogNodeByPath resolves a service catalog node by workspace path.
func (r *Resolver) ServiceCatalogNodeByPath(ctx context.Context, workspaceID, path string) (*ServiceCatalogNodeResolver, error) {
	if r.catalogService == nil {
		return nil, fmt.Errorf("catalog service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCatalogRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	node, err := r.catalogService.GetServiceCatalogNodeByPath(ctx, workspaceID, path)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return r.NewServiceCatalogNodeResolver(node), nil
}

// ServiceCatalogNodes resolves service catalog nodes for a workspace and parent node.
func (r *Resolver) ServiceCatalogNodes(ctx context.Context, workspaceID string, parentNodeID *string) ([]*ServiceCatalogNodeResolver, error) {
	if r.catalogService == nil {
		return nil, fmt.Errorf("catalog service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCatalogRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	nodes, err := r.catalogService.ListServiceCatalogNodes(ctx, workspaceID, valueOrEmpty(parentNodeID))
	if err != nil {
		return nil, fmt.Errorf("failed to list service catalog nodes: %w", err)
	}
	return r.NewServiceCatalogNodeResolvers(nodes), nil
}

// FormSpec resolves a form spec by ID.
func (r *Resolver) FormSpec(ctx context.Context, id string) (*FormSpecResolver, error) {
	if r.formSpecService == nil {
		return nil, fmt.Errorf("form service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionFormRead)
	if err != nil {
		return nil, err
	}
	spec, err := r.formSpecService.GetFormSpec(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("form spec not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(spec.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("form spec not found")
	}
	return r.NewFormSpecResolver(spec), nil
}

// FormSpecBySlug resolves a form spec by workspace slug.
func (r *Resolver) FormSpecBySlug(ctx context.Context, workspaceID, slug string) (*FormSpecResolver, error) {
	if r.formSpecService == nil {
		return nil, fmt.Errorf("form service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionFormRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	spec, err := r.formSpecService.GetFormSpecBySlug(ctx, workspaceID, slug)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return r.NewFormSpecResolver(spec), nil
}

// FormSpecs resolves form specs for a workspace.
func (r *Resolver) FormSpecs(ctx context.Context, workspaceID string) ([]*FormSpecResolver, error) {
	if r.formSpecService == nil {
		return nil, fmt.Errorf("form service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionFormRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	specs, err := r.formSpecService.ListWorkspaceFormSpecs(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list form specs: %w", err)
	}
	return r.NewFormSpecResolvers(specs), nil
}

// FormSubmission resolves a form submission by ID.
func (r *Resolver) FormSubmission(ctx context.Context, id string) (*FormSubmissionResolver, error) {
	if r.formSpecService == nil {
		return nil, fmt.Errorf("form service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionFormRead)
	if err != nil {
		return nil, err
	}
	submission, err := r.formSpecService.GetFormSubmission(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("form submission not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(submission.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("form submission not found")
	}
	return r.NewFormSubmissionResolver(submission), nil
}

// FormSubmissions resolves form submissions for a workspace.
func (r *Resolver) FormSubmissions(ctx context.Context, workspaceID string, filter *model.FormSubmissionFilterInput) ([]*FormSubmissionResolver, error) {
	if r.formSpecService == nil {
		return nil, fmt.Errorf("form service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionFormRead)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	params := servicedomain.FormSubmissionFilter{}
	if filter != nil {
		if filter.FormSpecID != nil {
			params.FormSpecID = *filter.FormSpecID
		}
		if filter.ConversationSessionID != nil {
			params.ConversationSessionID = *filter.ConversationSessionID
		}
		if filter.CaseID != nil {
			params.CaseID = *filter.CaseID
		}
		if filter.ContactID != nil {
			params.ContactID = *filter.ContactID
		}
		if filter.Status != nil {
			params.Status = servicedomain.FormSubmissionStatus(*filter.Status)
		}
		if filter.Limit != nil {
			params.Limit = int(*filter.Limit)
		}
		if filter.Offset != nil {
			params.Offset = int(*filter.Offset)
		}
	}

	submissions, err := r.formSpecService.ListFormSubmissions(ctx, workspaceID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list form submissions: %w", err)
	}
	return r.NewFormSubmissionResolvers(submissions), nil
}

// Case resolves a case by ID
func (r *Resolver) Case(ctx context.Context, id string) (*CaseResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseRead)
	if err != nil {
		return nil, err
	}

	caseObj, err := r.caseService.GetCase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}

	// Layer 2 defense: Validate workspace ownership (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(caseObj.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("case not found") // Same error to prevent enumeration
	}

	return r.NewCaseResolver(caseObj), nil
}

// CaseByHumanID resolves a case by its human-readable ID
func (r *Resolver) CaseByHumanID(ctx context.Context, workspaceID, caseID string) (*CaseResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseRead)
	if err != nil {
		return nil, err
	}

	// First verify the requested workspace matches the auth context
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("case not found")
	}

	caseObj, err := r.caseService.GetCaseByHumanID(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}

	// Layer 2 defense: Validate workspace ownership (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(caseObj.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("case not found") // Same error to prevent enumeration
	}

	return r.NewCaseResolver(caseObj), nil
}

// Cases resolves cases for a workspace with filters
func (r *Resolver) Cases(ctx context.Context, workspaceID string, filter *model.CaseFilterInput) (*CaseConnectionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseRead)
	if err != nil {
		return nil, err
	}

	// Validate the requested workspace matches auth context (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	filters := contracts.CaseFilters{
		WorkspaceID: workspaceID,
		Limit:       50,
	}

	if filter != nil {
		if filter.Status != nil && len(*filter.Status) > 0 {
			filters.Status = (*filter.Status)[0]
		}
		if filter.Priority != nil && len(*filter.Priority) > 0 {
			filters.Priority = (*filter.Priority)[0]
		}
		if filter.QueueID != nil {
			filters.QueueID = *filter.QueueID
		}
		if filter.AssigneeID != nil {
			filters.AssignedTo = *filter.AssigneeID
		}
		if filter.First != nil {
			filters.Limit = int(*filter.First)
		}
	}

	cases, total, err := r.caseService.ListCases(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list cases: %w", err)
	}

	return r.NewCaseConnectionResolver(cases, total, filters.Limit), nil
}

// =============================================================================
// Mutation Resolvers
// =============================================================================

// CreateCase creates a case through the supported service-domain mutation
// surface used by the CLI and agent workflows.
func (r *Resolver) CreateCase(ctx context.Context, input model.CreateCaseInput) (*CaseResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseWrite)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(input.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	params := serviceapp.CreateCaseParams{
		WorkspaceID:  input.WorkspaceID,
		Subject:      strings.TrimSpace(input.Subject),
		Description:  strings.TrimSpace(valueOrEmpty(input.Description)),
		Category:     strings.TrimSpace(valueOrEmpty(input.Category)),
		QueueID:      strings.TrimSpace(valueOrEmpty(input.QueueID)),
		ContactID:    strings.TrimSpace(valueOrEmpty(input.ContactID)),
		ContactEmail: strings.TrimSpace(valueOrEmpty(input.ContactEmail)),
		ContactName:  strings.TrimSpace(valueOrEmpty(input.ContactName)),
		Channel:      servicedomain.CaseChannelAPI,
	}
	if priority := strings.TrimSpace(valueOrEmpty(input.Priority)); priority != "" {
		params.Priority = servicedomain.CasePriority(priority)
	}

	caseObj, err := r.caseService.CreateCase(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create case: %w", err)
	}
	return &CaseResolver{case_: caseObj, r: r}, nil
}

// AddCommunication adds a communication to a case
func (r *Resolver) AddCommunication(ctx context.Context, input model.AddCommunicationInput) (*CommunicationResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseWrite)
	if err != nil {
		return nil, err
	}

	caseObj, err := r.caseService.GetCase(ctx, input.CaseID)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}

	// Layer 2 defense: Validate workspace ownership before modification (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(caseObj.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("case not found")
	}

	var comm *servicedomain.Communication
	if authCtx.IsAgent() {
		comm = servicedomain.NewAgentCommunication(
			input.CaseID,
			caseObj.WorkspaceID,
			authCtx.Principal.GetID(),
			shareddomain.CommTypeNote,
			input.Body,
		)
	} else {
		comm = servicedomain.NewCommunication(
			input.CaseID,
			caseObj.WorkspaceID,
			shareddomain.CommTypeNote,
			input.Body,
		)
		comm.FromUserID = authCtx.Principal.GetID()
	}

	if input.BodyHTML != nil {
		comm.BodyHTML = *input.BodyHTML
	}
	if input.IsInternal != nil {
		comm.IsInternal = *input.IsInternal
	}

	if err := r.caseService.CreateCommunication(ctx, comm); err != nil {
		return nil, fmt.Errorf("failed to create communication: %w", err)
	}

	return &CommunicationResolver{comm: comm, r: r}, nil
}

// AddCaseNote adds an internal note while preserving the acting principal.
func (r *Resolver) AddCaseNote(ctx context.Context, id, body string) (*CommunicationResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseWrite)
	if err != nil {
		return nil, err
	}

	caseObj, err := r.caseService.GetCase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(caseObj.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("case not found")
	}

	var comm *servicedomain.Communication
	if authCtx.IsAgent() {
		comm = servicedomain.NewAgentCommunication(id, caseObj.WorkspaceID, authCtx.Principal.GetID(), shareddomain.CommTypeNote, body)
	} else {
		comm = servicedomain.NewCommunication(id, caseObj.WorkspaceID, shareddomain.CommTypeNote, body)
		comm.FromUserID = authCtx.Principal.GetID()
	}
	comm.FromName = authCtx.Principal.GetName()
	comm.Direction = shareddomain.DirectionInternal
	comm.IsInternal = true

	if err := r.caseService.CreateCommunication(ctx, comm); err != nil {
		return nil, fmt.Errorf("failed to add case note: %w", err)
	}
	return &CommunicationResolver{comm: comm, r: r}, nil
}

// ReplyToCase sends a case reply through the same outbox-backed runtime used in
// production, deriving the sending identity from the authenticated principal.
func (r *Resolver) ReplyToCase(ctx context.Context, id string, input model.ReplyToCaseInput) (*CommunicationResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseWrite)
	if err != nil {
		return nil, err
	}

	caseObj, err := r.caseService.GetCase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(caseObj.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("case not found")
	}

	actor, err := r.resolveCaseReplyActor(ctx, authCtx)
	if err != nil {
		return nil, err
	}

	toEmails := stringSliceOrEmpty(input.ToEmails)
	if len(toEmails) == 0 && strings.TrimSpace(caseObj.ContactEmail) != "" {
		toEmails = []string{strings.TrimSpace(caseObj.ContactEmail)}
	}
	if len(toEmails) == 0 {
		return nil, fmt.Errorf("failed to queue case reply: at least one recipient email is required")
	}

	subject := strings.TrimSpace(valueOrEmpty(input.Subject))
	if subject == "" {
		subject = "Re: " + caseObj.Subject
	}

	reply, err := r.caseService.ReplyToCase(ctx, serviceapp.ReplyToCaseParams{
		CaseID:      id,
		WorkspaceID: caseObj.WorkspaceID,
		UserID:      actor.userID,
		UserName:    actor.displayName,
		UserEmail:   actor.userEmail,
		AgentID:     actor.agentID,
		Body:        input.Body,
		BodyHTML:    strings.TrimSpace(valueOrEmpty(input.BodyHTML)),
		ToEmails:    toEmails,
		CCEmails:    stringSliceOrEmpty(input.CcEmails),
		Subject:     subject,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to queue case reply: %w", err)
	}
	return &CommunicationResolver{comm: reply, r: r}, nil
}

// UpdateCaseStatus updates the status of a case
func (r *Resolver) UpdateCaseStatus(ctx context.Context, id, status string) (*CaseResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseWrite)
	if err != nil {
		return nil, err
	}

	// Fetch case first to validate workspace ownership (ADR-0003)
	caseObj, err := r.caseService.GetCase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}

	// Layer 2 defense: Validate workspace ownership before modification
	if err := graphshared.ValidateWorkspaceOwnership(caseObj.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("case not found")
	}

	if err := r.caseService.SetCaseStatus(ctx, id, servicedomain.CaseStatus(status)); err != nil {
		return nil, fmt.Errorf("failed to update case status: %w", err)
	}

	// Re-fetch to get updated state
	caseObj, err = r.caseService.GetCase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}

	return &CaseResolver{case_: caseObj, r: r}, nil
}

// SetCasePriority updates the priority of a case.
func (r *Resolver) SetCasePriority(ctx context.Context, id, priority string) (*CaseResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseWrite)
	if err != nil {
		return nil, err
	}

	caseObj, err := r.caseService.GetCase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(caseObj.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("case not found")
	}

	if err := r.caseService.SetCasePriority(ctx, id, servicedomain.CasePriority(priority)); err != nil {
		return nil, fmt.Errorf("failed to update case priority: %w", err)
	}
	caseObj, err = r.caseService.GetCase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}
	return &CaseResolver{case_: caseObj, r: r}, nil
}

// AssignCase assigns a case to a user
func (r *Resolver) AssignCase(ctx context.Context, id string, assigneeID *string) (*CaseResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseWrite)
	if err != nil {
		return nil, err
	}

	// Fetch case first to validate workspace ownership (ADR-0003)
	caseObj, err := r.caseService.GetCase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}

	// Layer 2 defense: Validate workspace ownership before modification
	if err := graphshared.ValidateWorkspaceOwnership(caseObj.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("case not found")
	}

	userID := ""
	if assigneeID != nil {
		userID = *assigneeID
	}

	if err := r.caseService.AssignCase(ctx, id, userID, ""); err != nil {
		return nil, fmt.Errorf("failed to assign case: %w", err)
	}

	// Re-fetch to get updated state
	caseObj, err = r.caseService.GetCase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}

	return &CaseResolver{case_: caseObj, r: r}, nil
}

// HandoffCase moves durable work between teams and queues.
func (r *Resolver) HandoffCase(ctx context.Context, id string, input model.CaseHandoffInput) (*CaseResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseWrite)
	if err != nil {
		return nil, err
	}

	caseObj, err := r.caseService.GetCase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(caseObj.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("case not found")
	}
	if err := validateSourceTeamAccess(authCtx, caseObj.TeamID, "case not found"); err != nil {
		return nil, err
	}

	targetTeamID := strings.TrimSpace(valueOrEmpty(input.TeamID))
	if err := validateDelegatedRouting(authCtx, targetTeamID); err != nil {
		return nil, err
	}
	if targetTeamID != "" && !authCtx.CanAccessTeam(targetTeamID) {
		return nil, fmt.Errorf("team not found")
	}

	performedByID := authCtx.Principal.GetID()
	performedByName := authCtx.Principal.GetName()
	performedByType := "user"
	if authCtx.IsAgent() {
		performedByType = "agent"
	}
	onBehalfOfUserID := agentOwnerID(authCtx)

	if err := r.caseService.HandoffCase(ctx, id, serviceapp.CaseHandoffParams{
		QueueID:          strings.TrimSpace(input.QueueID),
		TeamID:           targetTeamID,
		AssigneeID:       strings.TrimSpace(valueOrEmpty(input.AssigneeID)),
		Reason:           strings.TrimSpace(valueOrEmpty(input.Reason)),
		PerformedByID:    performedByID,
		PerformedByName:  performedByName,
		PerformedByType:  performedByType,
		OnBehalfOfUserID: onBehalfOfUserID,
	}); err != nil {
		return nil, fmt.Errorf("failed to hand off case: %w", err)
	}

	caseObj, err = r.caseService.GetCase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}
	return &CaseResolver{case_: caseObj, r: r}, nil
}

// CreateQueue creates a queue in a workspace.
func (r *Resolver) CreateQueue(ctx context.Context, input model.CreateQueueInput) (*QueueResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionQueueWrite)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(input.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	var slug string
	if input.Slug != nil {
		slug = *input.Slug
	}
	var description string
	if input.Description != nil {
		description = *input.Description
	}

	queue, err := r.queueService.CreateQueue(ctx, serviceapp.CreateQueueParams{
		WorkspaceID: input.WorkspaceID,
		Name:        input.Name,
		Slug:        slug,
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create queue: %w", err)
	}
	return r.NewQueueResolver(queue), nil
}

// AddConversationMessage appends a message to a conversation session.
func (r *Resolver) AddConversationMessage(ctx context.Context, sessionID string, input model.AddConversationMessageInput) (*ConversationMessageResolver, error) {
	if r.conversationService == nil {
		return nil, fmt.Errorf("conversation service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionConversationWrite)
	if err != nil {
		return nil, err
	}
	session, err := r.conversationService.GetConversationSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("conversation session not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(session.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("conversation session not found")
	}
	role, err := serviceapp.NormalizeConversationMessageRole(valueOrEmpty(input.Role))
	if err != nil {
		return nil, err
	}
	kind, err := serviceapp.NormalizeConversationMessageKind(valueOrEmpty(input.Kind))
	if err != nil {
		return nil, err
	}
	visibility, err := serviceapp.NormalizeConversationMessageVisibility(valueOrEmpty(input.Visibility))
	if err != nil {
		return nil, err
	}
	message, err := r.conversationService.AddConversationMessage(ctx, sessionID, serviceapp.AddConversationMessageParams{
		ParticipantID: valueOrEmpty(input.ParticipantID),
		Role:          role,
		Kind:          kind,
		Visibility:    visibility,
		ContentText:   valueOrEmpty(input.ContentText),
		Content:       typedSchemaFromJSON(input.Content),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add conversation message: %w", err)
	}
	return &ConversationMessageResolver{message: message}, nil
}

func (r *Resolver) HandoffConversation(ctx context.Context, sessionID string, input model.ConversationHandoffInput) (*ConversationSessionResolver, error) {
	if r.conversationService == nil {
		return nil, fmt.Errorf("conversation service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionConversationWrite)
	if err != nil {
		return nil, err
	}
	session, err := r.conversationService.GetConversationSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("conversation session not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(session.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("conversation session not found")
	}
	if err := validateSourceTeamAccess(authCtx, session.HandlingTeamID, "conversation session not found"); err != nil {
		return nil, err
	}
	targetTeamID := valueOrEmpty(input.TeamID)
	if err := validateDelegatedRouting(authCtx, targetTeamID); err != nil {
		return nil, err
	}
	if targetTeamID != "" && !authCtx.CanAccessTeam(targetTeamID) {
		return nil, fmt.Errorf("team not found")
	}
	performedByID := authCtx.Principal.GetID()
	performedByName := authCtx.Principal.GetName()
	performedByType := "user"
	if authCtx.IsAgent() {
		performedByType = "agent"
	}
	updated, err := r.conversationService.HandoffConversation(ctx, sessionID, serviceapp.HandoffConversationParams{
		TeamID:           targetTeamID,
		QueueID:          strings.TrimSpace(input.QueueID),
		OperatorUserID:   valueOrEmpty(input.OperatorUserID),
		Reason:           valueOrEmpty(input.Reason),
		PerformedByID:    performedByID,
		PerformedByName:  performedByName,
		PerformedByType:  performedByType,
		OnBehalfOfUserID: agentOwnerID(authCtx),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to hand off conversation: %w", err)
	}
	return r.NewConversationSessionResolver(updated), nil
}

func (r *Resolver) EscalateConversation(ctx context.Context, sessionID string, input model.EscalateConversationInput) (*CaseResolver, error) {
	if r.conversationService == nil || r.caseService == nil {
		return nil, fmt.Errorf("conversation escalation is not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionConversationWrite)
	if err != nil {
		return nil, err
	}
	if _, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseWrite); err != nil {
		return nil, err
	}
	session, err := r.conversationService.GetConversationSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("conversation session not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(session.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("conversation session not found")
	}
	if err := validateSourceTeamAccess(authCtx, session.HandlingTeamID, "conversation session not found"); err != nil {
		return nil, err
	}
	targetTeamID := valueOrEmpty(input.TeamID)
	if err := validateDelegatedRouting(authCtx, targetTeamID); err != nil {
		return nil, err
	}
	if targetTeamID != "" && !authCtx.CanAccessTeam(targetTeamID) {
		return nil, fmt.Errorf("team not found")
	}
	performedByID := authCtx.Principal.GetID()
	performedByName := authCtx.Principal.GetName()
	performedByType := "user"
	if authCtx.IsAgent() {
		performedByType = "agent"
	}
	priority := servicedomain.CasePriorityMedium
	if input.Priority != nil {
		priority = shareddomain.CasePriority(strings.TrimSpace(*input.Priority))
	}
	caseObj, err := r.conversationService.EscalateConversation(ctx, sessionID, serviceapp.EscalateConversationParams{
		TeamID:           targetTeamID,
		QueueID:          strings.TrimSpace(input.QueueID),
		OperatorUserID:   valueOrEmpty(input.OperatorUserID),
		Subject:          valueOrEmpty(input.Subject),
		Description:      valueOrEmpty(input.Description),
		Priority:         priority,
		Category:         valueOrEmpty(input.Category),
		Reason:           valueOrEmpty(input.Reason),
		PerformedByID:    performedByID,
		PerformedByName:  performedByName,
		PerformedByType:  performedByType,
		OnBehalfOfUserID: agentOwnerID(authCtx),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to escalate conversation: %w", err)
	}
	return &CaseResolver{case_: caseObj, r: r}, nil
}

// CreateKnowledgeResource creates a knowledge resource.
func (r *Resolver) CreateKnowledgeResource(ctx context.Context, input model.CreateKnowledgeResourceInput) (*KnowledgeResourceResolver, error) {
	if r.knowledgeService == nil {
		return nil, fmt.Errorf("knowledge service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeWrite)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(input.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	if !authCtx.CanAccessTeam(input.TeamID) {
		return nil, fmt.Errorf("team not found")
	}

	params := knowledgeservices.CreateKnowledgeResourceParams{
		WorkspaceID:        input.WorkspaceID,
		TeamID:             input.TeamID,
		Slug:               input.Slug,
		Title:              input.Title,
		Summary:            valueOrEmpty(input.Summary),
		BodyMarkdown:       valueOrEmpty(input.BodyMarkdown),
		ConceptSpecKey:     valueOrEmpty(input.ConceptSpecKey),
		ConceptSpecVersion: valueOrEmpty(input.ConceptSpecVersion),
		SourceRef:          valueOrEmpty(input.SourceRef),
		PathRef:            valueOrEmpty(input.PathRef),
		CreatedBy:          authCtx.Principal.GetID(),
	}
	if input.Kind != nil {
		params.Kind = knowledgedomain.KnowledgeResourceKind(*input.Kind)
	}
	if input.SourceKind != nil {
		params.SourceKind = knowledgedomain.KnowledgeResourceSourceKind(*input.SourceKind)
	}
	if input.Status != nil {
		params.Status = knowledgedomain.KnowledgeResourceStatus(*input.Status)
	}
	if input.Frontmatter != nil {
		params.Frontmatter = shareddomain.TypedSchemaFromMap(input.Frontmatter.ToMap())
	} else {
		params.Frontmatter = shareddomain.NewTypedSchema()
	}
	if input.SupportedChannels != nil {
		params.SupportedChannels = *input.SupportedChannels
	}
	if input.SharedWithTeamIDs != nil {
		params.SharedWithTeamIDs = *input.SharedWithTeamIDs
	}
	if input.SearchKeywords != nil {
		params.SearchKeywords = *input.SearchKeywords
	}
	if input.Surface != nil {
		surface, err := normalizeKnowledgeSurface(*input.Surface)
		if err != nil {
			return nil, err
		}
		params.Surface = surface
	}

	resource, err := r.knowledgeService.CreateKnowledgeResource(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge resource: %w", err)
	}
	return r.NewKnowledgeResourceResolver(resource), nil
}

// UpdateKnowledgeResource updates a knowledge resource.
func (r *Resolver) UpdateKnowledgeResource(ctx context.Context, id string, input model.UpdateKnowledgeResourceInput) (*KnowledgeResourceResolver, error) {
	if r.knowledgeService == nil {
		return nil, fmt.Errorf("knowledge service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeWrite)
	if err != nil {
		return nil, err
	}

	resource, err := r.knowledgeService.GetKnowledgeResource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(resource.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	if !authCtx.CanAccessTeam(resource.OwnerTeamID) {
		return nil, fmt.Errorf("knowledge resource not found")
	}

	params := knowledgeservices.UpdateKnowledgeResourceParams{
		Slug:               input.Slug,
		Title:              input.Title,
		Summary:            input.Summary,
		BodyMarkdown:       input.BodyMarkdown,
		ConceptSpecKey:     input.ConceptSpecKey,
		ConceptSpecVersion: input.ConceptSpecVersion,
		SourceRef:          input.SourceRef,
		PathRef:            input.PathRef,
		SupportedChannels:  input.SupportedChannels,
		SearchKeywords:     input.SearchKeywords,
	}
	if input.Kind != nil {
		kind := knowledgedomain.KnowledgeResourceKind(*input.Kind)
		params.Kind = &kind
	}
	if input.SourceKind != nil {
		sourceKind := knowledgedomain.KnowledgeResourceSourceKind(*input.SourceKind)
		params.SourceKind = &sourceKind
	}
	if input.Status != nil {
		status := knowledgedomain.KnowledgeResourceStatus(*input.Status)
		params.Status = &status
	}
	if input.Frontmatter != nil {
		frontmatter := shareddomain.TypedSchemaFromMap(input.Frontmatter.ToMap())
		params.Frontmatter = &frontmatter
	}

	updated, err := r.knowledgeService.UpdateKnowledgeResource(ctx, id, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update knowledge resource: %w", err)
	}
	return r.NewKnowledgeResourceResolver(updated), nil
}

// DeleteKnowledgeResource removes a knowledge resource and its git-backed artifact.
func (r *Resolver) DeleteKnowledgeResource(ctx context.Context, id string) (*KnowledgeResourceResolver, error) {
	if r.knowledgeService == nil {
		return nil, fmt.Errorf("knowledge service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeWrite)
	if err != nil {
		return nil, err
	}
	resource, err := r.knowledgeService.GetKnowledgeResource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(resource.WorkspaceID, authCtx); err != nil || !authCtx.CanAccessTeam(resource.OwnerTeamID) {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	deleted, err := r.knowledgeService.DeleteKnowledgeResource(ctx, id, authCtx.Principal.GetID())
	if err != nil {
		return nil, fmt.Errorf("failed to delete knowledge resource: %w", err)
	}
	return r.NewKnowledgeResourceResolver(deleted), nil
}

// ReviewKnowledgeResource updates the review status for a knowledge resource.
func (r *Resolver) ReviewKnowledgeResource(ctx context.Context, id string, status *string) (*KnowledgeResourceResolver, error) {
	if r.knowledgeService == nil {
		return nil, fmt.Errorf("knowledge service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeWrite)
	if err != nil {
		return nil, err
	}
	resource, err := r.knowledgeService.GetKnowledgeResource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(resource.WorkspaceID, authCtx); err != nil || !authCtx.CanAccessTeam(resource.OwnerTeamID) {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	reviewStatus := knowledgedomain.KnowledgeReviewStatusReviewed
	if status != nil {
		reviewStatus, err = normalizeKnowledgeReviewStatus(*status)
		if err != nil {
			return nil, err
		}
	}
	updated, err := r.knowledgeService.ReviewKnowledgeResource(ctx, id, authCtx.Principal.GetID(), reviewStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to review knowledge resource: %w", err)
	}
	return r.NewKnowledgeResourceResolver(updated), nil
}

// PublishKnowledgeResource moves a knowledge resource onto a published surface.
func (r *Resolver) PublishKnowledgeResource(ctx context.Context, id string, surface *string) (*KnowledgeResourceResolver, error) {
	if r.knowledgeService == nil {
		return nil, fmt.Errorf("knowledge service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeWrite)
	if err != nil {
		return nil, err
	}
	resource, err := r.knowledgeService.GetKnowledgeResource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(resource.WorkspaceID, authCtx); err != nil || !authCtx.CanAccessTeam(resource.OwnerTeamID) {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	targetSurface := knowledgedomain.KnowledgeSurfacePublished
	if surface != nil {
		targetSurface, err = normalizeKnowledgeSurface(*surface)
		if err != nil {
			return nil, err
		}
	}
	updated, err := r.knowledgeService.PublishKnowledgeResource(ctx, id, authCtx.Principal.GetID(), targetSurface)
	if err != nil {
		return nil, fmt.Errorf("failed to publish knowledge resource: %w", err)
	}
	return r.NewKnowledgeResourceResolver(updated), nil
}

// ShareKnowledgeResource updates team sharing for a knowledge resource.
func (r *Resolver) ShareKnowledgeResource(ctx context.Context, id string, input model.ShareKnowledgeResourceInput) (*KnowledgeResourceResolver, error) {
	if r.knowledgeService == nil {
		return nil, fmt.Errorf("knowledge service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeWrite)
	if err != nil {
		return nil, err
	}
	resource, err := r.knowledgeService.GetKnowledgeResource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(resource.WorkspaceID, authCtx); err != nil || !authCtx.CanAccessTeam(resource.OwnerTeamID) {
		return nil, fmt.Errorf("knowledge resource not found")
	}
	updated, err := r.knowledgeService.ShareKnowledgeResource(ctx, id, input.TeamIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to share knowledge resource: %w", err)
	}
	return r.NewKnowledgeResourceResolver(updated), nil
}

// RegisterConceptSpec registers a versioned concept spec definition.
func (r *Resolver) RegisterConceptSpec(ctx context.Context, input model.RegisterConceptSpecInput) (*ConceptSpecResolver, error) {
	if r.conceptService == nil {
		return nil, fmt.Errorf("concept service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionKnowledgeWrite)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(input.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	if input.OwnerTeamID != nil && strings.TrimSpace(*input.OwnerTeamID) != "" && !authCtx.CanAccessTeam(*input.OwnerTeamID) {
		return nil, fmt.Errorf("team not found")
	}

	instanceKind, err := normalizeKnowledgeKindValue(input.InstanceKind)
	if err != nil {
		return nil, err
	}
	params := knowledgeservices.RegisterConceptSpecParams{
		WorkspaceID:     input.WorkspaceID,
		OwnerTeamID:     valueOrEmpty(input.OwnerTeamID),
		Key:             input.Key,
		Version:         valueOrEmpty(input.Version),
		Name:            input.Name,
		Description:     valueOrEmpty(input.Description),
		ExtendsKey:      valueOrEmpty(input.ExtendsKey),
		ExtendsVersion:  valueOrEmpty(input.ExtendsVersion),
		InstanceKind:    instanceKind,
		AgentGuidanceMD: valueOrEmpty(input.AgentGuidanceMarkdown),
		SourceRef:       valueOrEmpty(input.SourceRef),
		CreatedBy:       authCtx.Principal.GetID(),
	}
	if input.MetadataSchema != nil {
		params.MetadataSchema = shareddomain.TypedSchemaFromMap(input.MetadataSchema.ToMap())
	} else {
		params.MetadataSchema = shareddomain.NewTypedSchema()
	}
	if input.SectionsSchema != nil {
		params.SectionsSchema = shareddomain.TypedSchemaFromMap(input.SectionsSchema.ToMap())
	} else {
		params.SectionsSchema = shareddomain.NewTypedSchema()
	}
	if input.WorkflowSchema != nil {
		params.WorkflowSchema = shareddomain.TypedSchemaFromMap(input.WorkflowSchema.ToMap())
	} else {
		params.WorkflowSchema = shareddomain.NewTypedSchema()
	}
	if input.SourceKind != nil {
		params.SourceKind = knowledgedomain.ConceptSpecSourceKind(*input.SourceKind)
	}
	if input.Status != nil {
		params.Status = knowledgedomain.ConceptSpecStatus(*input.Status)
	}
	spec, err := r.conceptService.RegisterConceptSpec(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to register concept spec: %w", err)
	}
	return r.NewConceptSpecResolver(spec), nil
}

// UpdateQueue updates a queue.
func (r *Resolver) UpdateQueue(ctx context.Context, id string, input model.UpdateQueueInput) (*QueueResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionQueueWrite)
	if err != nil {
		return nil, err
	}

	queue, err := r.queueService.GetQueue(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("queue not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(queue.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("queue not found")
	}

	updated, err := r.queueService.UpdateQueue(ctx, id, serviceapp.UpdateQueueParams{
		Name:        input.Name,
		Slug:        input.Slug,
		Description: input.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update queue: %w", err)
	}
	return r.NewQueueResolver(updated), nil
}

// CreateFormSpec creates a form spec in a workspace.
func (r *Resolver) CreateFormSpec(ctx context.Context, input model.CreateFormSpecInput) (*FormSpecResolver, error) {
	if r.formSpecService == nil {
		return nil, fmt.Errorf("form service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionFormWrite)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(input.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	status, err := serviceapp.NormalizeFormSpecStatus(valueOrEmpty(input.Status))
	if err != nil {
		return nil, err
	}
	spec, err := r.formSpecService.CreateFormSpec(ctx, serviceapp.CreateFormSpecParams{
		WorkspaceID:          input.WorkspaceID,
		Name:                 input.Name,
		Slug:                 valueOrEmpty(input.Slug),
		PublicKey:            valueOrEmpty(input.PublicKey),
		DescriptionMarkdown:  valueOrEmpty(input.DescriptionMarkdown),
		FieldSpec:            typedSchemaFromJSON(input.FieldSpec),
		EvidenceRequirements: typedSchemaSliceOrEmpty(input.EvidenceRequirements),
		InferenceRules:       typedSchemaSliceOrEmpty(input.InferenceRules),
		ApprovalPolicy:       typedSchemaFromJSON(input.ApprovalPolicy),
		SubmissionPolicy:     typedSchemaFromJSON(input.SubmissionPolicy),
		DestinationPolicy:    typedSchemaFromJSON(input.DestinationPolicy),
		SupportedChannels:    stringSliceOrEmpty(input.SupportedChannels),
		IsPublic:             boolOrDefault(input.IsPublic, false),
		Status:               status,
		Metadata:             typedSchemaFromJSON(input.Metadata),
		CreatedBy:            authCtx.Principal.GetID(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create form spec: %w", err)
	}
	return r.NewFormSpecResolver(spec), nil
}

// UpdateFormSpec updates a form spec.
func (r *Resolver) UpdateFormSpec(ctx context.Context, id string, input model.UpdateFormSpecInput) (*FormSpecResolver, error) {
	if r.formSpecService == nil {
		return nil, fmt.Errorf("form service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionFormWrite)
	if err != nil {
		return nil, err
	}
	spec, err := r.formSpecService.GetFormSpec(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("form spec not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(spec.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("form spec not found")
	}
	params := serviceapp.UpdateFormSpecParams{
		Name:                input.Name,
		Slug:                input.Slug,
		PublicKey:           input.PublicKey,
		DescriptionMarkdown: input.DescriptionMarkdown,
		FieldSpec:           typedSchemaPointerFromJSON(input.FieldSpec),
		EvidenceRequirements: typedSchemaSlicePointerFromJSON(
			input.EvidenceRequirements,
		),
		InferenceRules: typedSchemaSlicePointerFromJSON(input.InferenceRules),
		ApprovalPolicy: typedSchemaPointerFromJSON(input.ApprovalPolicy),
		SubmissionPolicy: typedSchemaPointerFromJSON(
			input.SubmissionPolicy,
		),
		DestinationPolicy: typedSchemaPointerFromJSON(input.DestinationPolicy),
		SupportedChannels: input.SupportedChannels,
		IsPublic:          input.IsPublic,
		Metadata:          typedSchemaPointerFromJSON(input.Metadata),
	}
	if input.Status != nil {
		status, err := serviceapp.NormalizeFormSpecStatus(*input.Status)
		if err != nil {
			return nil, err
		}
		params.Status = &status
	}
	updated, err := r.formSpecService.UpdateFormSpec(ctx, id, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update form spec: %w", err)
	}
	return r.NewFormSpecResolver(updated), nil
}

// CreateFormSubmission creates a new form submission from an operator or agent workflow.
func (r *Resolver) CreateFormSubmission(ctx context.Context, input model.CreateFormSubmissionInput) (*FormSubmissionResolver, error) {
	if r.formSpecService == nil {
		return nil, fmt.Errorf("form service not configured")
	}
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionFormWrite)
	if err != nil {
		return nil, err
	}
	spec, err := r.formSpecService.GetFormSpec(ctx, input.FormSpecID)
	if err != nil {
		return nil, fmt.Errorf("form spec not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(spec.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("form spec not found")
	}
	status, err := serviceapp.NormalizeFormSubmissionStatus(valueOrEmpty(input.Status))
	if err != nil {
		return nil, err
	}
	submittedAt := dateTimePtrToTime(input.SubmittedAt)
	submission, err := r.formSpecService.CreateFormSubmission(ctx, serviceapp.CreateFormSubmissionParams{
		FormSpecID:            input.FormSpecID,
		ConversationSessionID: valueOrEmpty(input.ConversationSessionID),
		CaseID:                valueOrEmpty(input.CaseID),
		ContactID:             valueOrEmpty(input.ContactID),
		Status:                status,
		Channel:               valueOrEmpty(input.Channel),
		SubmitterEmail:        valueOrEmpty(input.SubmitterEmail),
		SubmitterName:         valueOrEmpty(input.SubmitterName),
		CompletionToken:       valueOrEmpty(input.CompletionToken),
		CollectedFields:       typedSchemaFromJSON(input.CollectedFields),
		MissingFields:         typedSchemaFromJSON(input.MissingFields),
		Evidence:              typedSchemaSliceOrEmpty(input.Evidence),
		ValidationErrors:      stringSliceOrEmpty(input.ValidationErrors),
		Metadata:              typedSchemaFromJSON(input.Metadata),
		SubmittedAt:           submittedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create form submission: %w", err)
	}
	return r.NewFormSubmissionResolver(submission), nil
}

// SetCaseQueue assigns or clears a queue on a case.
func (r *Resolver) SetCaseQueue(ctx context.Context, caseID string, queueID *string) (*CaseResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionCaseWrite)
	if err != nil {
		return nil, err
	}

	caseObj, err := r.caseService.GetCase(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(caseObj.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("case not found")
	}

	targetQueueID := ""
	if queueID != nil {
		targetQueueID = *queueID
		if targetQueueID != "" {
			queue, err := r.queueService.GetQueue(ctx, targetQueueID)
			if err != nil || queue.WorkspaceID != caseObj.WorkspaceID {
				return nil, fmt.Errorf("queue not found")
			}
		}
	}

	if err := r.caseService.SetCaseQueue(ctx, caseID, targetQueueID); err != nil {
		return nil, fmt.Errorf("failed to update case queue: %w", err)
	}

	caseObj, err = r.caseService.GetCase(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("case not found")
	}
	return &CaseResolver{case_: caseObj, r: r}, nil
}

// =============================================================================
// Type Resolvers
// =============================================================================

// QueueResolver resolves Queue fields.
type QueueResolver struct {
	queue *servicedomain.Queue
	r     *Resolver
}

func (c *QueueResolver) ID() model.ID {
	return model.ID(c.queue.ID)
}

func (c *QueueResolver) WorkspaceID() model.ID {
	return model.ID(c.queue.WorkspaceID)
}

func (c *QueueResolver) Slug() string {
	return c.queue.Slug
}

func (c *QueueResolver) Name() string {
	return c.queue.Name
}

func (c *QueueResolver) Description() *string {
	if c.queue.Description == "" {
		return nil
	}
	description := c.queue.Description
	return &description
}

func (c *QueueResolver) Items(ctx context.Context) ([]*QueueItemResolver, error) {
	if c.r == nil || c.r.queueService == nil {
		return []*QueueItemResolver{}, nil
	}
	items, err := c.r.queueService.ListQueueItems(ctx, c.queue.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list queue items: %w", err)
	}
	return c.r.NewQueueItemResolvers(items), nil
}

func (c *QueueResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.queue.CreatedAt}
}

func (c *QueueResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.queue.UpdatedAt}
}

type QueueItemResolver struct {
	item *servicedomain.QueueItem
	r    *Resolver
}

func (q *QueueItemResolver) ID() model.ID {
	return model.ID(q.item.ID)
}

func (q *QueueItemResolver) WorkspaceID() model.ID {
	return model.ID(q.item.WorkspaceID)
}

func (q *QueueItemResolver) QueueID() model.ID {
	return model.ID(q.item.QueueID)
}

func (q *QueueItemResolver) ItemKind() string {
	return string(q.item.ItemKind)
}

func (q *QueueItemResolver) CaseID() *model.ID {
	return optionalModelID(q.item.CaseID)
}

func (q *QueueItemResolver) ConversationSessionID() *model.ID {
	return optionalModelID(q.item.ConversationSessionID)
}

func (q *QueueItemResolver) Case(ctx context.Context) (*CaseResolver, error) {
	if q.item.CaseID == "" || q.r == nil || q.r.caseService == nil {
		return nil, nil
	}
	caseObj, err := q.r.caseService.GetCase(ctx, q.item.CaseID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get queue item case: %w", err)
	}
	if caseObj.WorkspaceID != q.item.WorkspaceID {
		return nil, nil
	}
	return &CaseResolver{case_: caseObj, r: q.r}, nil
}

func (q *QueueItemResolver) ConversationSession(ctx context.Context) (*ConversationSessionResolver, error) {
	if q.item.ConversationSessionID == "" || q.r == nil || q.r.conversationService == nil {
		return nil, nil
	}
	session, err := q.r.conversationService.GetConversationSession(ctx, q.item.ConversationSessionID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get queue item conversation: %w", err)
	}
	if session.WorkspaceID != q.item.WorkspaceID {
		return nil, nil
	}
	return &ConversationSessionResolver{session: session, r: q.r}, nil
}

func (q *QueueItemResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: q.item.CreatedAt}
}

func (q *QueueItemResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: q.item.UpdatedAt}
}

// ConversationSessionResolver resolves ConversationSession fields.
type ConversationSessionResolver struct {
	session *servicedomain.ConversationSession
	r       *Resolver
}

func (c *ConversationSessionResolver) ID() model.ID { return model.ID(c.session.ID) }
func (c *ConversationSessionResolver) WorkspaceID() model.ID {
	return model.ID(c.session.WorkspaceID)
}
func (c *ConversationSessionResolver) Channel() string { return string(c.session.Channel) }
func (c *ConversationSessionResolver) Status() string  { return string(c.session.Status) }
func (c *ConversationSessionResolver) Title() *string {
	return optionalStringValue(c.session.Title)
}
func (c *ConversationSessionResolver) LanguageCode() *string {
	return optionalStringValue(c.session.LanguageCode)
}
func (c *ConversationSessionResolver) SourceRef() *string {
	return optionalStringValue(c.session.SourceRef)
}
func (c *ConversationSessionResolver) ExternalSessionKey() *string {
	return optionalStringValue(c.session.ExternalSessionKey)
}
func (c *ConversationSessionResolver) PrimaryContactID() *model.ID {
	return optionalModelID(c.session.PrimaryContactID)
}
func (c *ConversationSessionResolver) PrimaryCatalogNodeID() *model.ID {
	return optionalModelID(c.session.PrimaryCatalogNodeID)
}
func (c *ConversationSessionResolver) ActiveFormSpecID() *model.ID {
	return optionalModelID(c.session.ActiveFormSpecID)
}
func (c *ConversationSessionResolver) ActiveFormSubmissionID() *model.ID {
	return optionalModelID(c.session.ActiveFormSubmissionID)
}
func (c *ConversationSessionResolver) LinkedCaseID() *model.ID {
	return optionalModelID(c.session.LinkedCaseID)
}
func (c *ConversationSessionResolver) HandlingTeamID() *model.ID {
	return optionalModelID(c.session.HandlingTeamID)
}
func (c *ConversationSessionResolver) AssignedOperatorUserID() *model.ID {
	return optionalModelID(c.session.AssignedOperatorUserID)
}
func (c *ConversationSessionResolver) DelegatedRuntimeConnectorID() *string {
	return optionalStringValue(c.session.DelegatedRuntimeConnectorID)
}
func (c *ConversationSessionResolver) OpenedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.session.OpenedAt}
}
func (c *ConversationSessionResolver) LastActivityAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.session.LastActivityAt}
}
func (c *ConversationSessionResolver) ClosedAt() *graphshared.DateTime {
	if c.session.ClosedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *c.session.ClosedAt}
}
func (c *ConversationSessionResolver) Metadata() graphshared.JSON {
	return graphshared.JSON(c.session.Metadata.ToMap())
}
func (c *ConversationSessionResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.session.CreatedAt}
}
func (c *ConversationSessionResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.session.UpdatedAt}
}
func (c *ConversationSessionResolver) Participants(ctx context.Context) ([]*ConversationParticipantResolver, error) {
	participants, err := c.r.conversationService.ListConversationParticipants(ctx, c.session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversation participants: %w", err)
	}
	result := make([]*ConversationParticipantResolver, 0, len(participants))
	for _, participant := range participants {
		if participant == nil {
			continue
		}
		result = append(result, &ConversationParticipantResolver{participant: participant})
	}
	return result, nil
}
func (c *ConversationSessionResolver) Messages(ctx context.Context, args struct{ Visibility *string }) ([]*ConversationMessageResolver, error) {
	filter := servicedomain.ConversationMessageVisibility("")
	if args.Visibility != nil {
		normalized, err := serviceapp.NormalizeConversationMessageVisibility(*args.Visibility)
		if err != nil {
			return nil, err
		}
		filter = normalized
	}
	messages, err := c.r.conversationService.ListConversationMessages(ctx, c.session.ID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversation messages: %w", err)
	}
	result := make([]*ConversationMessageResolver, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		result = append(result, &ConversationMessageResolver{message: message})
	}
	return result, nil
}
func (c *ConversationSessionResolver) WorkingState(ctx context.Context) (*ConversationWorkingStateResolver, error) {
	state, err := c.r.conversationService.GetConversationWorkingState(ctx, c.session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation working state: %w", err)
	}
	if state == nil {
		return nil, nil
	}
	return &ConversationWorkingStateResolver{state: state}, nil
}
func (c *ConversationSessionResolver) Outcomes(ctx context.Context) ([]*ConversationOutcomeResolver, error) {
	outcomes, err := c.r.conversationService.ListConversationOutcomes(ctx, c.session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversation outcomes: %w", err)
	}
	result := make([]*ConversationOutcomeResolver, 0, len(outcomes))
	for _, outcome := range outcomes {
		if outcome == nil {
			continue
		}
		result = append(result, &ConversationOutcomeResolver{outcome: outcome})
	}
	return result, nil
}

type ConversationParticipantResolver struct {
	participant *servicedomain.ConversationParticipant
}

func (c *ConversationParticipantResolver) ID() model.ID { return model.ID(c.participant.ID) }
func (c *ConversationParticipantResolver) WorkspaceID() model.ID {
	return model.ID(c.participant.WorkspaceID)
}
func (c *ConversationParticipantResolver) ConversationSessionID() model.ID {
	return model.ID(c.participant.ConversationSessionID)
}
func (c *ConversationParticipantResolver) ParticipantKind() string {
	return string(c.participant.ParticipantKind)
}
func (c *ConversationParticipantResolver) ParticipantRef() string {
	return c.participant.ParticipantRef
}
func (c *ConversationParticipantResolver) RoleInSession() string {
	return string(c.participant.RoleInSession)
}
func (c *ConversationParticipantResolver) DisplayName() *string {
	return optionalStringValue(c.participant.DisplayName)
}
func (c *ConversationParticipantResolver) JoinedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.participant.JoinedAt}
}
func (c *ConversationParticipantResolver) LeftAt() *graphshared.DateTime {
	if c.participant.LeftAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *c.participant.LeftAt}
}
func (c *ConversationParticipantResolver) Metadata() graphshared.JSON {
	return graphshared.JSON(c.participant.Metadata.ToMap())
}
func (c *ConversationParticipantResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.participant.CreatedAt}
}

type ConversationMessageResolver struct {
	message *servicedomain.ConversationMessage
}

func (c *ConversationMessageResolver) ID() model.ID { return model.ID(c.message.ID) }
func (c *ConversationMessageResolver) WorkspaceID() model.ID {
	return model.ID(c.message.WorkspaceID)
}
func (c *ConversationMessageResolver) ConversationSessionID() model.ID {
	return model.ID(c.message.ConversationSessionID)
}
func (c *ConversationMessageResolver) ParticipantID() *model.ID {
	return optionalModelID(c.message.ParticipantID)
}
func (c *ConversationMessageResolver) Role() string       { return string(c.message.Role) }
func (c *ConversationMessageResolver) Kind() string       { return string(c.message.Kind) }
func (c *ConversationMessageResolver) Visibility() string { return string(c.message.Visibility) }
func (c *ConversationMessageResolver) ContentText() *string {
	return optionalStringValue(c.message.ContentText)
}
func (c *ConversationMessageResolver) Content() graphshared.JSON {
	return graphshared.JSON(c.message.Content.ToMap())
}
func (c *ConversationMessageResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.message.CreatedAt}
}

type ConversationWorkingStateResolver struct {
	state *servicedomain.ConversationWorkingState
}

func (c *ConversationWorkingStateResolver) ConversationSessionID() model.ID {
	return model.ID(c.state.ConversationSessionID)
}
func (c *ConversationWorkingStateResolver) WorkspaceID() model.ID {
	return model.ID(c.state.WorkspaceID)
}
func (c *ConversationWorkingStateResolver) PrimaryCatalogNodeID() *model.ID {
	return optionalModelID(c.state.PrimaryCatalogNodeID)
}
func (c *ConversationWorkingStateResolver) SuggestedCatalogNodes() []*ConversationCatalogSuggestionResolver {
	result := make([]*ConversationCatalogSuggestionResolver, 0, len(c.state.SuggestedCatalogNodes))
	for _, suggestion := range c.state.SuggestedCatalogNodes {
		copySuggestion := suggestion
		result = append(result, &ConversationCatalogSuggestionResolver{suggestion: &copySuggestion})
	}
	return result
}
func (c *ConversationWorkingStateResolver) ClassificationConfidence() *float64 {
	return c.state.ClassificationConfidence
}
func (c *ConversationWorkingStateResolver) ActivePolicyProfileRef() *string {
	return optionalStringValue(c.state.ActivePolicyProfileRef)
}
func (c *ConversationWorkingStateResolver) ActiveFormSpecID() *model.ID {
	return optionalModelID(c.state.ActiveFormSpecID)
}
func (c *ConversationWorkingStateResolver) ActiveFormSubmissionID() *model.ID {
	return optionalModelID(c.state.ActiveFormSubmissionID)
}
func (c *ConversationWorkingStateResolver) CollectedFields() graphshared.JSON {
	return graphshared.JSON(c.state.CollectedFields.ToMap())
}
func (c *ConversationWorkingStateResolver) MissingFields() graphshared.JSON {
	return graphshared.JSON(c.state.MissingFields.ToMap())
}
func (c *ConversationWorkingStateResolver) RequiresOperatorReview() bool {
	return c.state.RequiresOperatorReview
}
func (c *ConversationWorkingStateResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.state.UpdatedAt}
}

type ConversationCatalogSuggestionResolver struct {
	suggestion *servicedomain.ConversationCatalogSuggestion
}

func (c *ConversationCatalogSuggestionResolver) CatalogNodeID() model.ID {
	return model.ID(c.suggestion.CatalogNodeID)
}
func (c *ConversationCatalogSuggestionResolver) Reason() *string {
	return optionalStringValue(c.suggestion.Reason)
}
func (c *ConversationCatalogSuggestionResolver) Confidence() float64 {
	return c.suggestion.Confidence
}

type ConversationOutcomeResolver struct {
	outcome *servicedomain.ConversationOutcome
}

func (c *ConversationOutcomeResolver) ID() model.ID { return model.ID(c.outcome.ID) }
func (c *ConversationOutcomeResolver) WorkspaceID() model.ID {
	return model.ID(c.outcome.WorkspaceID)
}
func (c *ConversationOutcomeResolver) ConversationSessionID() model.ID {
	return model.ID(c.outcome.ConversationSessionID)
}
func (c *ConversationOutcomeResolver) Kind() string { return string(c.outcome.Kind) }
func (c *ConversationOutcomeResolver) ResultRef() graphshared.JSON {
	return graphshared.JSON(c.outcome.ResultRef.ToMap())
}
func (c *ConversationOutcomeResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.outcome.CreatedAt}
}

// KnowledgeResourceResolver resolves KnowledgeResource fields.
type KnowledgeResourceResolver struct {
	resource *knowledgedomain.KnowledgeResource
	r        *Resolver
}

func (r *KnowledgeResourceResolver) ID() model.ID {
	return model.ID(r.resource.ID)
}

func (r *KnowledgeResourceResolver) WorkspaceID() model.ID {
	return model.ID(r.resource.WorkspaceID)
}

func (r *KnowledgeResourceResolver) OwnerTeamID() model.ID {
	return model.ID(r.resource.OwnerTeamID)
}

func (r *KnowledgeResourceResolver) Slug() string {
	return r.resource.Slug
}

func (r *KnowledgeResourceResolver) Title() string {
	return r.resource.Title
}

func (r *KnowledgeResourceResolver) Kind() string {
	return string(r.resource.Kind)
}

func (r *KnowledgeResourceResolver) ConceptSpecKey() string {
	return r.resource.ConceptSpecKey
}

func (r *KnowledgeResourceResolver) ConceptSpecVersion() string {
	return r.resource.ConceptSpecVersion
}

func (r *KnowledgeResourceResolver) ConceptSpec() *ConceptSpecResolver {
	if r.r == nil || r.r.conceptService == nil {
		return nil
	}
	spec, err := r.r.conceptService.GetConceptSpec(context.Background(), r.resource.WorkspaceID, r.resource.ConceptSpecKey, r.resource.ConceptSpecVersion)
	if err != nil {
		return nil
	}
	return r.r.NewConceptSpecResolver(spec)
}

func (r *KnowledgeResourceResolver) SourceKind() string {
	return string(r.resource.SourceKind)
}

func (r *KnowledgeResourceResolver) SourceRef() *string {
	if r.resource.SourceRef == "" {
		return nil
	}
	value := r.resource.SourceRef
	return &value
}

func (r *KnowledgeResourceResolver) PathRef() *string {
	if r.resource.PathRef == "" {
		return nil
	}
	value := r.resource.PathRef
	return &value
}

func (r *KnowledgeResourceResolver) ArtifactPath() string {
	return r.resource.ArtifactPath
}

func (r *KnowledgeResourceResolver) Summary() *string {
	if r.resource.Summary == "" {
		return nil
	}
	value := r.resource.Summary
	return &value
}

func (r *KnowledgeResourceResolver) BodyMarkdown() string {
	return r.resource.BodyMarkdown
}

func (r *KnowledgeResourceResolver) Frontmatter() graphshared.JSON {
	return graphshared.JSON(r.resource.Frontmatter.ToMap())
}

func (r *KnowledgeResourceResolver) SupportedChannels() []string {
	return r.resource.SupportedChannels
}

func (r *KnowledgeResourceResolver) SharedWithTeamIDs() []model.ID {
	result := make([]model.ID, len(r.resource.SharedWithTeamIDs))
	for i, teamID := range r.resource.SharedWithTeamIDs {
		result[i] = model.ID(teamID)
	}
	return result
}

func (r *KnowledgeResourceResolver) Surface() string {
	return string(r.resource.Surface)
}

func (r *KnowledgeResourceResolver) TrustLevel() string {
	return string(r.resource.TrustLevel)
}

func (r *KnowledgeResourceResolver) SearchKeywords() []string {
	return r.resource.SearchKeywords
}

func (r *KnowledgeResourceResolver) Status() string {
	return string(r.resource.Status)
}

func (r *KnowledgeResourceResolver) ReviewStatus() string {
	return string(r.resource.ReviewStatus)
}

func (r *KnowledgeResourceResolver) ContentHash() string {
	return r.resource.ContentHash
}

func (r *KnowledgeResourceResolver) RevisionRef() string {
	return r.resource.RevisionRef
}

func (r *KnowledgeResourceResolver) PublishedRevision() *string {
	if r.resource.PublishedRevision == "" {
		return nil
	}
	value := r.resource.PublishedRevision
	return &value
}

func (r *KnowledgeResourceResolver) ReviewedAt() *graphshared.DateTime {
	if r.resource.ReviewedAt == nil {
		return nil
	}
	value := graphshared.DateTime{Time: *r.resource.ReviewedAt}
	return &value
}

func (r *KnowledgeResourceResolver) PublishedAt() *graphshared.DateTime {
	if r.resource.PublishedAt == nil {
		return nil
	}
	value := graphshared.DateTime{Time: *r.resource.PublishedAt}
	return &value
}

func (r *KnowledgeResourceResolver) PublishedByID() *model.ID {
	if r.resource.PublishedBy == "" {
		return nil
	}
	value := model.ID(r.resource.PublishedBy)
	return &value
}

func (r *KnowledgeResourceResolver) CreatedByID() *model.ID {
	if r.resource.CreatedBy == "" {
		return nil
	}
	value := model.ID(r.resource.CreatedBy)
	return &value
}

func (r *KnowledgeResourceResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.resource.CreatedAt}
}

func (r *KnowledgeResourceResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.resource.UpdatedAt}
}

// ConceptSpecResolver resolves ConceptSpec fields.
type ConceptSpecResolver struct {
	spec *knowledgedomain.ConceptSpec
}

func (r *ConceptSpecResolver) ID() model.ID {
	if strings.TrimSpace(r.spec.ID) != "" {
		return model.ID(r.spec.ID)
	}
	return model.ID(r.spec.Key + "@" + r.spec.Version)
}
func (r *ConceptSpecResolver) WorkspaceID() *model.ID {
	if strings.TrimSpace(r.spec.WorkspaceID) == "" {
		return nil
	}
	value := model.ID(r.spec.WorkspaceID)
	return &value
}
func (r *ConceptSpecResolver) OwnerTeamID() *model.ID {
	if strings.TrimSpace(r.spec.OwnerTeamID) == "" {
		return nil
	}
	value := model.ID(r.spec.OwnerTeamID)
	return &value
}
func (r *ConceptSpecResolver) Key() string          { return r.spec.Key }
func (r *ConceptSpecResolver) Version() string      { return r.spec.Version }
func (r *ConceptSpecResolver) Name() string         { return r.spec.Name }
func (r *ConceptSpecResolver) Description() string  { return r.spec.Description }
func (r *ConceptSpecResolver) InstanceKind() string { return string(r.spec.InstanceKind) }
func (r *ConceptSpecResolver) MetadataSchema() graphshared.JSON {
	return graphshared.JSON(r.spec.MetadataSchema.ToMap())
}
func (r *ConceptSpecResolver) SectionsSchema() graphshared.JSON {
	return graphshared.JSON(r.spec.SectionsSchema.ToMap())
}
func (r *ConceptSpecResolver) WorkflowSchema() graphshared.JSON {
	return graphshared.JSON(r.spec.WorkflowSchema.ToMap())
}
func (r *ConceptSpecResolver) AgentGuidanceMarkdown() string { return r.spec.AgentGuidanceMD }
func (r *ConceptSpecResolver) ArtifactPath() string          { return r.spec.ArtifactPath }
func (r *ConceptSpecResolver) SourceKind() string            { return string(r.spec.SourceKind) }
func (r *ConceptSpecResolver) Status() string                { return string(r.spec.Status) }
func (r *ConceptSpecResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.spec.CreatedAt}
}
func (r *ConceptSpecResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.spec.UpdatedAt}
}
func (r *ConceptSpecResolver) ExtendsKey() *string {
	if strings.TrimSpace(r.spec.ExtendsKey) == "" {
		return nil
	}
	value := r.spec.ExtendsKey
	return &value
}
func (r *ConceptSpecResolver) ExtendsVersion() *string {
	if strings.TrimSpace(r.spec.ExtendsVersion) == "" {
		return nil
	}
	value := r.spec.ExtendsVersion
	return &value
}
func (r *ConceptSpecResolver) RevisionRef() *string {
	if strings.TrimSpace(r.spec.RevisionRef) == "" {
		return nil
	}
	value := r.spec.RevisionRef
	return &value
}
func (r *ConceptSpecResolver) SourceRef() *string {
	if strings.TrimSpace(r.spec.SourceRef) == "" {
		return nil
	}
	value := r.spec.SourceRef
	return &value
}
func (r *ConceptSpecResolver) CreatedByID() *model.ID {
	if strings.TrimSpace(r.spec.CreatedBy) == "" {
		return nil
	}
	value := model.ID(r.spec.CreatedBy)
	return &value
}

// KnowledgeRevisionResolver resolves git-backed knowledge revision fields.
type KnowledgeRevisionResolver struct {
	revision artifactservices.Revision
}

func (r *KnowledgeRevisionResolver) Ref() string {
	return r.revision.Ref
}

func (r *KnowledgeRevisionResolver) CommittedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.revision.CommittedAt}
}

func (r *KnowledgeRevisionResolver) Subject() string {
	return r.revision.Subject
}

// KnowledgeDiffResolver resolves a knowledge patch response.
type KnowledgeDiffResolver struct {
	diff *knowledgeservices.KnowledgeDiff
}

func (r *KnowledgeDiffResolver) Path() string {
	return r.diff.Path
}

func (r *KnowledgeDiffResolver) FromRevision() *string {
	if r.diff.FromRevision == "" {
		return nil
	}
	value := r.diff.FromRevision
	return &value
}

func (r *KnowledgeDiffResolver) ToRevision() string {
	return r.diff.ToRevision
}

func (r *KnowledgeDiffResolver) Patch() string {
	return r.diff.Patch
}

// ConceptSpecRevisionResolver resolves git-backed concept spec revision fields.
type ConceptSpecRevisionResolver struct {
	revision artifactservices.Revision
}

func (r *ConceptSpecRevisionResolver) Ref() string {
	return r.revision.Ref
}

func (r *ConceptSpecRevisionResolver) CommittedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.revision.CommittedAt}
}

func (r *ConceptSpecRevisionResolver) Subject() string {
	return r.revision.Subject
}

// ConceptSpecDiffResolver resolves a concept spec patch response.
type ConceptSpecDiffResolver struct {
	diff *knowledgeservices.ConceptSpecDiff
}

func (r *ConceptSpecDiffResolver) Path() string {
	return r.diff.Path
}

func (r *ConceptSpecDiffResolver) FromRevision() *string {
	if r.diff.FromRevision == "" {
		return nil
	}
	value := r.diff.FromRevision
	return &value
}

func (r *ConceptSpecDiffResolver) ToRevision() string {
	return r.diff.ToRevision
}

func (r *ConceptSpecDiffResolver) Patch() string {
	return r.diff.Patch
}

// ServiceCatalogNodeResolver resolves ServiceCatalogNode fields.
type ServiceCatalogNodeResolver struct {
	node *servicedomain.ServiceCatalogNode
	r    *Resolver
}

func (r *ServiceCatalogNodeResolver) ID() model.ID {
	return model.ID(r.node.ID)
}

func (r *ServiceCatalogNodeResolver) WorkspaceID() model.ID {
	return model.ID(r.node.WorkspaceID)
}

func (r *ServiceCatalogNodeResolver) ParentNodeID() *model.ID {
	if r.node.ParentNodeID == "" {
		return nil
	}
	value := model.ID(r.node.ParentNodeID)
	return &value
}

func (r *ServiceCatalogNodeResolver) Slug() string {
	return r.node.Slug
}

func (r *ServiceCatalogNodeResolver) PathSlug() string {
	return r.node.PathSlug
}

func (r *ServiceCatalogNodeResolver) Title() string {
	return r.node.Title
}

func (r *ServiceCatalogNodeResolver) DescriptionMarkdown() *string {
	if r.node.DescriptionMarkdown == "" {
		return nil
	}
	value := r.node.DescriptionMarkdown
	return &value
}

func (r *ServiceCatalogNodeResolver) NodeKind() string {
	return string(r.node.NodeKind)
}

func (r *ServiceCatalogNodeResolver) Status() string {
	return string(r.node.Status)
}

func (r *ServiceCatalogNodeResolver) Visibility() string {
	return string(r.node.Visibility)
}

func (r *ServiceCatalogNodeResolver) SupportedChannels() []string {
	return append([]string(nil), r.node.SupportedChannels...)
}

func (r *ServiceCatalogNodeResolver) DefaultCaseCategory() *string {
	if r.node.DefaultCaseCategory == "" {
		return nil
	}
	value := r.node.DefaultCaseCategory
	return &value
}

func (r *ServiceCatalogNodeResolver) DefaultQueueID() *model.ID {
	if r.node.DefaultQueueID == "" {
		return nil
	}
	value := model.ID(r.node.DefaultQueueID)
	return &value
}

func (r *ServiceCatalogNodeResolver) DefaultPriority() *string {
	if r.node.DefaultPriority == "" {
		return nil
	}
	value := r.node.DefaultPriority
	return &value
}

func (r *ServiceCatalogNodeResolver) SearchKeywords() []string {
	return append([]string(nil), r.node.SearchKeywords...)
}

func (r *ServiceCatalogNodeResolver) DisplayOrder() int32 {
	return int32(r.node.DisplayOrder)
}

func (r *ServiceCatalogNodeResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.node.CreatedAt}
}

func (r *ServiceCatalogNodeResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.node.UpdatedAt}
}

func (r *ServiceCatalogNodeResolver) Bindings(ctx context.Context) ([]*ServiceCatalogBindingResolver, error) {
	if r.r == nil || r.r.catalogService == nil {
		return []*ServiceCatalogBindingResolver{}, nil
	}
	bindings, err := r.r.catalogService.ListServiceCatalogBindings(ctx, r.node.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list service catalog bindings: %w", err)
	}
	result := make([]*ServiceCatalogBindingResolver, 0, len(bindings))
	for _, binding := range bindings {
		if binding == nil {
			continue
		}
		result = append(result, &ServiceCatalogBindingResolver{binding: binding})
	}
	return result, nil
}

// ServiceCatalogBindingResolver resolves ServiceCatalogBinding fields.
type ServiceCatalogBindingResolver struct {
	binding *servicedomain.ServiceCatalogBinding
}

func (r *ServiceCatalogBindingResolver) ID() model.ID {
	return model.ID(r.binding.ID)
}

func (r *ServiceCatalogBindingResolver) WorkspaceID() model.ID {
	return model.ID(r.binding.WorkspaceID)
}

func (r *ServiceCatalogBindingResolver) CatalogNodeID() model.ID {
	return model.ID(r.binding.CatalogNodeID)
}

func (r *ServiceCatalogBindingResolver) TargetKind() string {
	return string(r.binding.TargetKind)
}

func (r *ServiceCatalogBindingResolver) TargetID() model.ID {
	return model.ID(r.binding.TargetID)
}

func (r *ServiceCatalogBindingResolver) BindingKind() string {
	return string(r.binding.BindingKind)
}

func (r *ServiceCatalogBindingResolver) Confidence() *float64 {
	return r.binding.Confidence
}

func (r *ServiceCatalogBindingResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.binding.CreatedAt}
}

// FormSpecResolver resolves FormSpec fields.
type FormSpecResolver struct {
	spec *servicedomain.FormSpec
}

func (r *FormSpecResolver) ID() model.ID {
	return model.ID(r.spec.ID)
}

func (r *FormSpecResolver) WorkspaceID() model.ID {
	return model.ID(r.spec.WorkspaceID)
}

func (r *FormSpecResolver) Name() string {
	return r.spec.Name
}

func (r *FormSpecResolver) Slug() string {
	return r.spec.Slug
}

func (r *FormSpecResolver) PublicKey() *string {
	if r.spec.PublicKey == "" {
		return nil
	}
	value := r.spec.PublicKey
	return &value
}

func (r *FormSpecResolver) DescriptionMarkdown() *string {
	if r.spec.DescriptionMarkdown == "" {
		return nil
	}
	value := r.spec.DescriptionMarkdown
	return &value
}

func (r *FormSpecResolver) FieldSpec() graphshared.JSON {
	return graphshared.JSON(r.spec.FieldSpec.ToMap())
}

func (r *FormSpecResolver) EvidenceRequirements() []graphshared.JSON {
	return typedSchemaSliceToJSON(r.spec.EvidenceRequirements)
}

func (r *FormSpecResolver) InferenceRules() []graphshared.JSON {
	return typedSchemaSliceToJSON(r.spec.InferenceRules)
}

func (r *FormSpecResolver) ApprovalPolicy() graphshared.JSON {
	return graphshared.JSON(r.spec.ApprovalPolicy.ToMap())
}

func (r *FormSpecResolver) SubmissionPolicy() graphshared.JSON {
	return graphshared.JSON(r.spec.SubmissionPolicy.ToMap())
}

func (r *FormSpecResolver) DestinationPolicy() graphshared.JSON {
	return graphshared.JSON(r.spec.DestinationPolicy.ToMap())
}

func (r *FormSpecResolver) SupportedChannels() []string {
	return append([]string(nil), r.spec.SupportedChannels...)
}

func (r *FormSpecResolver) IsPublic() bool {
	return r.spec.IsPublic
}

func (r *FormSpecResolver) Status() string {
	return string(r.spec.Status)
}

func (r *FormSpecResolver) Metadata() graphshared.JSON {
	return graphshared.JSON(r.spec.Metadata.ToMap())
}

func (r *FormSpecResolver) CreatedByID() *model.ID {
	if r.spec.CreatedBy == "" {
		return nil
	}
	value := model.ID(r.spec.CreatedBy)
	return &value
}

func (r *FormSpecResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.spec.CreatedAt}
}

func (r *FormSpecResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.spec.UpdatedAt}
}

func (r *FormSpecResolver) DeletedAt() *graphshared.DateTime {
	if r.spec.DeletedAt == nil {
		return nil
	}
	value := graphshared.DateTime{Time: *r.spec.DeletedAt}
	return &value
}

// FormSubmissionResolver resolves FormSubmission fields.
type FormSubmissionResolver struct {
	submission *servicedomain.FormSubmission
}

func (r *FormSubmissionResolver) ID() model.ID {
	return model.ID(r.submission.ID)
}

func (r *FormSubmissionResolver) WorkspaceID() model.ID {
	return model.ID(r.submission.WorkspaceID)
}

func (r *FormSubmissionResolver) FormSpecID() model.ID {
	return model.ID(r.submission.FormSpecID)
}

func (r *FormSubmissionResolver) ConversationSessionID() *model.ID {
	if r.submission.ConversationSessionID == "" {
		return nil
	}
	value := model.ID(r.submission.ConversationSessionID)
	return &value
}

func (r *FormSubmissionResolver) CaseID() *model.ID {
	if r.submission.CaseID == "" {
		return nil
	}
	value := model.ID(r.submission.CaseID)
	return &value
}

func (r *FormSubmissionResolver) ContactID() *model.ID {
	if r.submission.ContactID == "" {
		return nil
	}
	value := model.ID(r.submission.ContactID)
	return &value
}

func (r *FormSubmissionResolver) Status() string {
	return string(r.submission.Status)
}

func (r *FormSubmissionResolver) Channel() string {
	return r.submission.Channel
}

func (r *FormSubmissionResolver) SubmitterEmail() *string {
	if r.submission.SubmitterEmail == "" {
		return nil
	}
	value := r.submission.SubmitterEmail
	return &value
}

func (r *FormSubmissionResolver) SubmitterName() *string {
	if r.submission.SubmitterName == "" {
		return nil
	}
	value := r.submission.SubmitterName
	return &value
}

func (r *FormSubmissionResolver) CompletionToken() *string {
	if r.submission.CompletionToken == "" {
		return nil
	}
	value := r.submission.CompletionToken
	return &value
}

func (r *FormSubmissionResolver) CollectedFields() graphshared.JSON {
	return graphshared.JSON(r.submission.CollectedFields.ToMap())
}

func (r *FormSubmissionResolver) MissingFields() graphshared.JSON {
	return graphshared.JSON(r.submission.MissingFields.ToMap())
}

func (r *FormSubmissionResolver) Evidence() []graphshared.JSON {
	return typedSchemaSliceToJSON(r.submission.Evidence)
}

func (r *FormSubmissionResolver) ValidationErrors() []string {
	return append([]string(nil), r.submission.ValidationErrors...)
}

func (r *FormSubmissionResolver) Metadata() graphshared.JSON {
	return graphshared.JSON(r.submission.Metadata.ToMap())
}

func (r *FormSubmissionResolver) SubmittedAt() *graphshared.DateTime {
	if r.submission.SubmittedAt == nil {
		return nil
	}
	value := graphshared.DateTime{Time: *r.submission.SubmittedAt}
	return &value
}

func (r *FormSubmissionResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.submission.CreatedAt}
}

func (r *FormSubmissionResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: r.submission.UpdatedAt}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalStringValue(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copyValue := value
	return &copyValue
}

func optionalModelID(value string) *model.ID {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	id := model.ID(value)
	return &id
}

func typedSchemaFromJSON(value *graphshared.JSON) shareddomain.TypedSchema {
	if value == nil {
		return shareddomain.NewTypedSchema()
	}
	if *value == nil {
		return shareddomain.NewTypedSchema()
	}
	return shareddomain.TypedSchemaFromMap(map[string]interface{}(*value))
}

func typedSchemaPointerFromJSON(value *graphshared.JSON) *shareddomain.TypedSchema {
	if value == nil {
		return nil
	}
	schema := typedSchemaFromJSON(value)
	return &schema
}

func typedSchemaSliceFromJSON(values []graphshared.JSON) []shareddomain.TypedSchema {
	if len(values) == 0 {
		return []shareddomain.TypedSchema{}
	}
	result := make([]shareddomain.TypedSchema, 0, len(values))
	for _, value := range values {
		copyValue := value
		result = append(result, typedSchemaFromJSON(&copyValue))
	}
	return result
}

func typedSchemaSlicePointerFromJSON(values *[]graphshared.JSON) *[]shareddomain.TypedSchema {
	if values == nil {
		return nil
	}
	result := typedSchemaSliceFromJSON(*values)
	return &result
}

func typedSchemaSliceOrEmpty(values *[]graphshared.JSON) []shareddomain.TypedSchema {
	if values == nil {
		return []shareddomain.TypedSchema{}
	}
	return typedSchemaSliceFromJSON(*values)
}

func typedSchemaSliceToJSON(items []shareddomain.TypedSchema) []graphshared.JSON {
	if len(items) == 0 {
		return []graphshared.JSON{}
	}
	result := make([]graphshared.JSON, 0, len(items))
	for _, item := range items {
		result = append(result, graphshared.JSON(item.ToMap()))
	}
	return result
}

func stringSliceOrEmpty(values *[]string) []string {
	if values == nil {
		return []string{}
	}
	result := make([]string, 0, len(*values))
	for _, value := range *values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func boolOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func dateTimePtrToTime(value *graphshared.DateTime) *time.Time {
	if value == nil {
		return nil
	}
	converted := value.Time
	return &converted
}

func agentOwnerID(authCtx *platformdomain.AuthContext) string {
	if authCtx == nil || !authCtx.IsAgent() {
		return ""
	}
	agent, ok := authCtx.Principal.(*platformdomain.Agent)
	if !ok {
		return ""
	}
	return strings.TrimSpace(agent.OwnerID)
}

type caseReplyActor struct {
	userID      string
	userEmail   string
	displayName string
	agentID     string
}

func (r *Resolver) resolveCaseReplyActor(ctx context.Context, authCtx *platformdomain.AuthContext) (caseReplyActor, error) {
	if authCtx == nil || authCtx.Principal == nil {
		return caseReplyActor{}, fmt.Errorf("not authenticated")
	}

	if authCtx.IsAgent() {
		ownerID := agentOwnerID(authCtx)
		if ownerID == "" {
			return caseReplyActor{}, fmt.Errorf("agent replies require an owner user")
		}
		if r.userService == nil {
			return caseReplyActor{}, fmt.Errorf("agent replies require user service access")
		}
		owner, err := r.userService.GetUser(ctx, ownerID)
		if err != nil {
			return caseReplyActor{}, fmt.Errorf("load agent owner: %w", err)
		}
		return caseReplyActor{
			userID:      owner.ID,
			userEmail:   strings.TrimSpace(owner.Email),
			displayName: strings.TrimSpace(authCtx.Principal.GetName()),
			agentID:     strings.TrimSpace(authCtx.Principal.GetID()),
		}, nil
	}

	if user, ok := authCtx.Principal.(*platformdomain.User); ok {
		return caseReplyActor{
			userID:      user.ID,
			userEmail:   strings.TrimSpace(user.Email),
			displayName: strings.TrimSpace(user.Name),
		}, nil
	}
	if r.userService == nil {
		return caseReplyActor{}, fmt.Errorf("reply actor lookup requires user service access")
	}
	user, err := r.userService.GetUser(ctx, authCtx.Principal.GetID())
	if err != nil {
		return caseReplyActor{}, fmt.Errorf("load reply actor: %w", err)
	}
	return caseReplyActor{
		userID:      user.ID,
		userEmail:   strings.TrimSpace(user.Email),
		displayName: strings.TrimSpace(user.Name),
	}, nil
}

func validateSourceTeamAccess(authCtx *platformdomain.AuthContext, teamID string, notFoundMessage string) error {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil
	}
	if authCtx == nil || !authCtx.CanAccessTeam(teamID) {
		return fmt.Errorf("%s", notFoundMessage)
	}
	return nil
}

func validateDelegatedRouting(authCtx *platformdomain.AuthContext, targetTeamID string) error {
	targetTeamID = strings.TrimSpace(targetTeamID)
	if authCtx == nil || !authCtx.IsAgent() {
		return nil
	}
	if !authCtx.AllowsDelegatedRouting() {
		return fmt.Errorf("delegated routing is not enabled for this agent membership")
	}
	if targetTeamID != "" && !authCtx.CanDelegateRoutingToTeam(targetTeamID) {
		return fmt.Errorf("delegated routing is not allowed for team %s", targetTeamID)
	}
	return nil
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

func optionalInt32(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func isNotFoundError(err error) bool {
	var apiErr *apierrors.APIError
	return errors.As(err, &apiErr) && apiErr.Type == apierrors.ErrorTypeNotFound
}

// CaseResolver resolves Case fields
type CaseResolver struct {
	case_ *servicedomain.Case
	r     *Resolver
}

// ID returns the case ID
func (c *CaseResolver) ID() model.ID {
	return model.ID(c.case_.ID)
}

// CaseID returns the human-readable case ID
func (c *CaseResolver) CaseID() string {
	return c.case_.HumanID
}

// WorkspaceID returns the workspace ID
func (c *CaseResolver) WorkspaceID() model.ID {
	return model.ID(c.case_.WorkspaceID)
}

// Subject returns the case subject
func (c *CaseResolver) Subject() string {
	return c.case_.Subject
}

// Description returns the case description when present.
func (c *CaseResolver) Description() *string {
	return optionalStringValue(c.case_.Description)
}

// Status returns the case status
func (c *CaseResolver) Status() string {
	return string(c.case_.Status)
}

// Priority returns the case priority
func (c *CaseResolver) Priority() string {
	return string(c.case_.Priority)
}

// Category returns the case category when present.
func (c *CaseResolver) Category() *string {
	return optionalStringValue(c.case_.Category)
}

// Channel returns the case channel.
func (c *CaseResolver) Channel() string {
	return string(c.case_.Channel)
}

// TeamID returns the team ID if set.
func (c *CaseResolver) TeamID() *model.ID {
	if c.case_.TeamID == "" {
		return nil
	}
	id := model.ID(c.case_.TeamID)
	return &id
}

// QueueID returns the queue ID if set.
func (c *CaseResolver) QueueID() *model.ID {
	if c.case_.QueueID == "" {
		return nil
	}
	id := model.ID(c.case_.QueueID)
	return &id
}

// Queue resolves the queue field.
func (c *CaseResolver) Queue(ctx context.Context) (*QueueResolver, error) {
	if c.case_.QueueID == "" || c.r.queueService == nil {
		return nil, nil
	}
	queue, err := c.r.queueService.GetQueue(ctx, c.case_.QueueID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get queue: %w", err)
	}
	if queue.WorkspaceID != c.case_.WorkspaceID {
		return nil, nil
	}
	return c.r.NewQueueResolver(queue), nil
}

// ContactID returns the contact ID if set
func (c *CaseResolver) ContactID() *model.ID {
	if c.case_.ContactID == "" {
		return nil
	}
	id := model.ID(c.case_.ContactID)
	return &id
}

// ContactEmail returns the case contact email when present.
func (c *CaseResolver) ContactEmail() *string {
	return optionalStringValue(c.case_.ContactEmail)
}

// ContactName returns the case contact name when present.
func (c *CaseResolver) ContactName() *string {
	return optionalStringValue(c.case_.ContactName)
}

// AssigneeID returns the assignee ID if set
func (c *CaseResolver) AssigneeID() *model.ID {
	if c.case_.AssignedToID == "" {
		return nil
	}
	id := model.ID(c.case_.AssignedToID)
	return &id
}

// OriginatingConversationID returns the originating conversation ID if set.
func (c *CaseResolver) OriginatingConversationID() *model.ID {
	if c.case_.OriginatingConversationID == "" {
		return nil
	}
	id := model.ID(c.case_.OriginatingConversationID)
	return &id
}

// CreatedAt returns the creation timestamp
func (c *CaseResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.case_.CreatedAt}
}

// UpdatedAt returns the update timestamp
func (c *CaseResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.case_.UpdatedAt}
}

// ResolvedAt returns the resolution timestamp if set
func (c *CaseResolver) ResolvedAt() *graphshared.DateTime {
	if c.case_.ResolvedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *c.case_.ResolvedAt}
}

// Contact resolves the contact field (lazy loading)
func (c *CaseResolver) Contact(ctx context.Context) (*model.Contact, error) {
	if c.case_.ContactID == "" || c.r.contactService == nil {
		return nil, nil
	}

	contact, err := c.r.contactService.GetContact(ctx, c.case_.WorkspaceID, c.case_.ContactID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get contact: %w", err)
	}

	return contactToModel(contact), nil
}

// Assignee resolves the assignee field (lazy loading)
func (c *CaseResolver) Assignee(ctx context.Context) (*model.User, error) {
	if c.case_.AssignedToID == "" || c.r.userService == nil {
		return nil, nil
	}

	user, err := c.r.userService.GetUser(ctx, c.case_.AssignedToID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get assignee: %w", err)
	}

	return userToModel(user), nil
}

// OriginatingConversation resolves the source conversation for an escalated case.
func (c *CaseResolver) OriginatingConversation(ctx context.Context) (*ConversationSessionResolver, error) {
	if c.case_.OriginatingConversationID == "" || c.r.conversationService == nil {
		return nil, nil
	}
	session, err := c.r.conversationService.GetConversationSession(ctx, c.case_.OriginatingConversationID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get originating conversation: %w", err)
	}
	if session.WorkspaceID != c.case_.WorkspaceID {
		return nil, nil
	}
	return &ConversationSessionResolver{session: session, r: c.r}, nil
}

// Communications resolves the communications field (lazy loading)
func (c *CaseResolver) Communications(ctx context.Context) ([]*CommunicationResolver, error) {
	if c.r.caseService == nil {
		return []*CommunicationResolver{}, nil
	}

	comms, err := c.r.caseService.ListCaseCommunications(ctx, c.case_.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list communications: %w", err)
	}

	result := make([]*CommunicationResolver, len(comms))
	for i, comm := range comms {
		result[i] = &CommunicationResolver{comm: comm, r: c.r}
	}
	return result, nil
}

// WorkThread resolves the durable case-owned work thread, including linked
// source-conversation messages when present.
func (c *CaseResolver) WorkThread(ctx context.Context) ([]*CaseWorkThreadEntryResolver, error) {
	entries := make([]*CaseWorkThreadEntryResolver, 0)

	if c.r.caseService != nil {
		comms, err := c.r.caseService.ListCaseCommunications(ctx, c.case_.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list case communications: %w", err)
		}
		for _, comm := range comms {
			if comm == nil {
				continue
			}
			entries = append(entries, &CaseWorkThreadEntryResolver{
				caseID:        c.case_.ID,
				kind:          "case_communication",
				communication: comm,
				createdAt:     comm.CreatedAt,
			})
		}
	}

	if c.case_.OriginatingConversationID != "" && c.r.conversationService != nil {
		messages, err := c.r.conversationService.ListConversationMessages(ctx, c.case_.OriginatingConversationID, "")
		if err != nil {
			return nil, fmt.Errorf("failed to list originating conversation messages: %w", err)
		}
		for _, message := range messages {
			if message == nil {
				continue
			}
			entries = append(entries, &CaseWorkThreadEntryResolver{
				caseID:              c.case_.ID,
				kind:                "conversation_message",
				conversationMessage: message,
				createdAt:           message.CreatedAt,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].createdAt.Equal(entries[j].createdAt) {
			return entries[i].ID() < entries[j].ID()
		}
		return entries[i].createdAt.Before(entries[j].createdAt)
	})
	return entries, nil
}

// =============================================================================
// CommunicationResolver
// =============================================================================

// CommunicationResolver resolves Communication fields
type CommunicationResolver struct {
	comm *servicedomain.Communication
	r    *Resolver
}

// CaseWorkThreadEntryResolver resolves durable case-thread entries.
type CaseWorkThreadEntryResolver struct {
	caseID              string
	kind                string
	communication       *servicedomain.Communication
	conversationMessage *servicedomain.ConversationMessage
	createdAt           time.Time
}

func (c *CaseWorkThreadEntryResolver) ID() model.ID {
	switch {
	case c.communication != nil:
		return model.ID("comm:" + c.communication.ID)
	case c.conversationMessage != nil:
		return model.ID("msg:" + c.conversationMessage.ID)
	default:
		return model.ID("thread:unknown")
	}
}

func (c *CaseWorkThreadEntryResolver) CaseID() model.ID {
	return model.ID(c.caseID)
}

func (c *CaseWorkThreadEntryResolver) Kind() string {
	return c.kind
}

func (c *CaseWorkThreadEntryResolver) CommunicationID() *model.ID {
	if c.communication == nil {
		return nil
	}
	id := model.ID(c.communication.ID)
	return &id
}

func (c *CaseWorkThreadEntryResolver) ConversationMessageID() *model.ID {
	if c.conversationMessage == nil {
		return nil
	}
	id := model.ID(c.conversationMessage.ID)
	return &id
}

func (c *CaseWorkThreadEntryResolver) ConversationSessionID() *model.ID {
	if c.conversationMessage == nil {
		return nil
	}
	id := model.ID(c.conversationMessage.ConversationSessionID)
	return &id
}

func (c *CaseWorkThreadEntryResolver) Channel() *string {
	if c.communication == nil {
		return nil
	}
	value := string(c.communication.Type)
	return &value
}

func (c *CaseWorkThreadEntryResolver) Direction() *string {
	if c.communication == nil {
		return nil
	}
	value := string(c.communication.Direction)
	return &value
}

func (c *CaseWorkThreadEntryResolver) Role() *string {
	if c.conversationMessage == nil {
		return nil
	}
	value := string(c.conversationMessage.Role)
	return &value
}

func (c *CaseWorkThreadEntryResolver) Visibility() *string {
	if c.conversationMessage == nil {
		return nil
	}
	value := string(c.conversationMessage.Visibility)
	return &value
}

func (c *CaseWorkThreadEntryResolver) Subject() *string {
	if c.communication == nil || strings.TrimSpace(c.communication.Subject) == "" {
		return nil
	}
	value := c.communication.Subject
	return &value
}

func (c *CaseWorkThreadEntryResolver) Body() string {
	switch {
	case c.communication != nil:
		return c.communication.Body
	case c.conversationMessage != nil:
		if text := strings.TrimSpace(c.conversationMessage.ContentText); text != "" {
			return text
		}
		if !c.conversationMessage.Content.IsEmpty() {
			if data, err := json.Marshal(c.conversationMessage.Content.ToMap()); err == nil {
				return string(data)
			}
		}
	}
	return ""
}

func (c *CaseWorkThreadEntryResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.createdAt}
}

// ID returns the communication ID
func (c *CommunicationResolver) ID() model.ID {
	return model.ID(c.comm.ID)
}

// CaseID returns the case ID
func (c *CommunicationResolver) CaseID() model.ID {
	return model.ID(c.comm.CaseID)
}

// Direction returns the communication direction
func (c *CommunicationResolver) Direction() string {
	return string(c.comm.Direction)
}

// Channel returns the communication channel/type
func (c *CommunicationResolver) Channel() string {
	return string(c.comm.Type)
}

// Subject returns the subject if set
func (c *CommunicationResolver) Subject() *string {
	if c.comm.Subject == "" {
		return nil
	}
	return &c.comm.Subject
}

// Body returns the body
func (c *CommunicationResolver) Body() string {
	return c.comm.Body
}

// BodyHTML returns the HTML body if set
func (c *CommunicationResolver) BodyHTML() *string {
	if c.comm.BodyHTML == "" {
		return nil
	}
	return &c.comm.BodyHTML
}

// FromEmail returns the sender email if set
func (c *CommunicationResolver) FromEmail() *string {
	if c.comm.FromEmail == "" {
		return nil
	}
	return &c.comm.FromEmail
}

// FromName returns the sender name if set
func (c *CommunicationResolver) FromName() *string {
	if c.comm.FromName == "" {
		return nil
	}
	return &c.comm.FromName
}

// FromUserID returns the sender user ID if set
func (c *CommunicationResolver) FromUserID() *model.ID {
	if c.comm.FromUserID == "" {
		return nil
	}
	id := model.ID(c.comm.FromUserID)
	return &id
}

// FromAgentID returns the sender agent ID if set
func (c *CommunicationResolver) FromAgentID() *model.ID {
	if c.comm.FromAgentID == "" {
		return nil
	}
	id := model.ID(c.comm.FromAgentID)
	return &id
}

// IsInternal returns whether this is an internal note
func (c *CommunicationResolver) IsInternal() bool {
	return c.comm.IsInternal
}

// CreatedAt returns the creation timestamp
func (c *CommunicationResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: c.comm.CreatedAt}
}

// FromUser resolves the sender user (lazy loading)
func (c *CommunicationResolver) FromUser(ctx context.Context) (*model.User, error) {
	if c.comm.FromUserID == "" || c.r.userService == nil {
		return nil, nil
	}

	user, err := c.r.userService.GetUser(ctx, c.comm.FromUserID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return userToModel(user), nil
}

// FromAgent resolves the sender agent (lazy loading)
func (c *CommunicationResolver) FromAgent(ctx context.Context) (*ServiceAgentResolver, error) {
	if c.comm.FromAgentID == "" || c.r.agentService == nil {
		return nil, nil
	}

	agent, err := c.r.agentService.GetAgent(ctx, c.comm.FromAgentID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return &ServiceAgentResolver{agent: agent, r: c.r}, nil
}

// =============================================================================
// Connection Resolvers (Relay-style pagination)
// =============================================================================

// CaseConnectionResolver resolves CaseConnection
type CaseConnectionResolver struct {
	cases []*servicedomain.Case
	total int
	limit int
	r     *Resolver
}

// Edges returns the case edges
func (c *CaseConnectionResolver) Edges() []*CaseEdgeResolver {
	edges := make([]*CaseEdgeResolver, len(c.cases))
	for i, caseObj := range c.cases {
		edges[i] = &CaseEdgeResolver{case_: caseObj, r: c.r}
	}
	return edges
}

// PageInfo returns pagination info
func (c *CaseConnectionResolver) PageInfo() *PageInfoResolver {
	hasNext := len(c.cases) == c.limit && c.total > c.limit
	return &PageInfoResolver{hasNextPage: hasNext}
}

// TotalCount returns the total count
func (c *CaseConnectionResolver) TotalCount() int32 {
	return int32(c.total)
}

// CaseEdgeResolver resolves CaseEdge
type CaseEdgeResolver struct {
	case_ *servicedomain.Case
	r     *Resolver
}

// Node returns the case
func (e *CaseEdgeResolver) Node() *CaseResolver {
	return &CaseResolver{case_: e.case_, r: e.r}
}

// Cursor returns the cursor
func (e *CaseEdgeResolver) Cursor() string {
	return e.case_.ID
}

// PageInfoResolver resolves PageInfo
type PageInfoResolver struct {
	hasNextPage     bool
	hasPreviousPage bool
	startCursor     *string
	endCursor       *string
}

// HasNextPage returns whether there's a next page
func (p *PageInfoResolver) HasNextPage() bool {
	return p.hasNextPage
}

// HasPreviousPage returns whether there's a previous page
func (p *PageInfoResolver) HasPreviousPage() bool {
	return p.hasPreviousPage
}

// StartCursor returns the start cursor
func (p *PageInfoResolver) StartCursor() *string {
	return p.startCursor
}

// EndCursor returns the end cursor
func (p *PageInfoResolver) EndCursor() *string {
	return p.endCursor
}

// =============================================================================
// Helper Converters
// =============================================================================

func userToModel(u *platformdomain.User) *model.User {
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

func contactToModel(c *platformdomain.Contact) *model.Contact {
	if c == nil {
		return nil
	}
	var name *string
	if c.Name != "" {
		name = &c.Name
	}
	return &model.Contact{
		ID:    model.ID(c.ID),
		Email: c.Email,
		Name:  name,
	}
}

// =============================================================================
// Agent Resolver (for Communication.FromAgent)
// =============================================================================

// ServiceAgentResolver resolves Agent fields in service context
type ServiceAgentResolver struct {
	agent *platformdomain.Agent
	r     *Resolver
}

// ID returns the agent ID
func (a *ServiceAgentResolver) ID() model.ID { return model.ID(a.agent.ID) }

// WorkspaceID returns the workspace ID
func (a *ServiceAgentResolver) WorkspaceID() model.ID { return model.ID(a.agent.WorkspaceID) }

// Name returns the agent name
func (a *ServiceAgentResolver) Name() string { return a.agent.Name }

// Description returns the agent description
func (a *ServiceAgentResolver) Description() *string {
	if a.agent.Description == "" {
		return nil
	}
	return &a.agent.Description
}

// OwnerID returns the owner ID
func (a *ServiceAgentResolver) OwnerID() model.ID { return model.ID(a.agent.OwnerID) }

// Owner resolves the owner user (lazy loading)
func (a *ServiceAgentResolver) Owner(ctx context.Context) (*model.User, error) {
	if a.r.userService == nil {
		return nil, nil
	}
	user, err := a.r.userService.GetUser(ctx, a.agent.OwnerID)
	if err != nil {
		return nil, nil //nolint:nilerr
	}
	return userToModel(user), nil
}

// Status returns the agent status
func (a *ServiceAgentResolver) Status() string { return string(a.agent.Status) }

// StatusReason returns the status reason
func (a *ServiceAgentResolver) StatusReason() *string {
	if a.agent.StatusReason == "" {
		return nil
	}
	return &a.agent.StatusReason
}

// CreatedAt returns the creation timestamp
func (a *ServiceAgentResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: a.agent.CreatedAt}
}

// UpdatedAt returns the update timestamp
func (a *ServiceAgentResolver) UpdatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: a.agent.UpdatedAt}
}

// CreatedByID returns the creator ID
func (a *ServiceAgentResolver) CreatedByID() model.ID { return model.ID(a.agent.CreatedByID) }

// Tokens returns agent tokens
func (a *ServiceAgentResolver) Tokens(ctx context.Context) ([]*ServiceAgentTokenResolver, error) {
	if a.r == nil || a.r.agentService == nil {
		return []*ServiceAgentTokenResolver{}, nil
	}

	tokens, err := a.r.agentService.ListAgentTokens(ctx, a.agent.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent tokens: %w", err)
	}

	result := make([]*ServiceAgentTokenResolver, len(tokens))
	for i, token := range tokens {
		result[i] = &ServiceAgentTokenResolver{token: token}
	}
	return result, nil
}

// Membership returns the workspace membership for this agent
func (a *ServiceAgentResolver) Membership(ctx context.Context) (*ServiceWorkspaceMembershipResolver, error) {
	if a.r == nil || a.r.agentService == nil {
		return nil, nil
	}

	membership, err := a.r.agentService.GetWorkspaceMembership(ctx, a.agent.WorkspaceID, a.agent.ID, platformdomain.PrincipalTypeAgent)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil //nolint:nilerr
		}
		return nil, nil //nolint:nilerr
	}
	return &ServiceWorkspaceMembershipResolver{membership: membership}, nil
}

// ServiceAgentTokenResolver resolves AgentToken in service context
type ServiceAgentTokenResolver struct {
	token *platformdomain.AgentToken
}

// ID returns the token ID
func (t *ServiceAgentTokenResolver) ID() model.ID { return model.ID(t.token.ID) }

// AgentID returns the agent ID
func (t *ServiceAgentTokenResolver) AgentID() model.ID { return model.ID(t.token.AgentID) }

// TokenPrefix returns the token prefix
func (t *ServiceAgentTokenResolver) TokenPrefix() string { return t.token.TokenPrefix }

// Name returns the token name
func (t *ServiceAgentTokenResolver) Name() string { return t.token.Name }

// ExpiresAt returns token expiry
func (t *ServiceAgentTokenResolver) ExpiresAt() *graphshared.DateTime {
	if t.token.ExpiresAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *t.token.ExpiresAt}
}

// RevokedAt returns token revocation timestamp
func (t *ServiceAgentTokenResolver) RevokedAt() *graphshared.DateTime {
	if t.token.RevokedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *t.token.RevokedAt}
}

// LastUsedAt returns the last usage timestamp
func (t *ServiceAgentTokenResolver) LastUsedAt() *graphshared.DateTime {
	if t.token.LastUsedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *t.token.LastUsedAt}
}

// LastUsedIP returns the last usage IP
func (t *ServiceAgentTokenResolver) LastUsedIP() *string {
	if t.token.LastUsedIP == "" {
		return nil
	}
	return &t.token.LastUsedIP
}

// UseCount returns the use count
func (t *ServiceAgentTokenResolver) UseCount() int32 { return int32(t.token.UseCount) }

// CreatedAt returns token creation timestamp
func (t *ServiceAgentTokenResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: t.token.CreatedAt}
}

// CreatedByID returns token creator ID
func (t *ServiceAgentTokenResolver) CreatedByID() model.ID { return model.ID(t.token.CreatedByID) }

// ServiceWorkspaceMembershipResolver resolves WorkspaceMembership in service context
type ServiceWorkspaceMembershipResolver struct {
	membership *platformdomain.WorkspaceMembership
}

type ServiceMembershipConstraintsResolver struct {
	constraints platformdomain.MembershipConstraints
}

// ID returns the membership ID
func (m *ServiceWorkspaceMembershipResolver) ID() model.ID { return model.ID(m.membership.ID) }

// WorkspaceID returns the workspace ID
func (m *ServiceWorkspaceMembershipResolver) WorkspaceID() model.ID {
	return model.ID(m.membership.WorkspaceID)
}

// PrincipalID returns the principal ID
func (m *ServiceWorkspaceMembershipResolver) PrincipalID() model.ID {
	return model.ID(m.membership.PrincipalID)
}

// PrincipalType returns principal type
func (m *ServiceWorkspaceMembershipResolver) PrincipalType() string {
	return string(m.membership.PrincipalType)
}

// Role returns the role
func (m *ServiceWorkspaceMembershipResolver) Role() string { return m.membership.Role }

// Permissions returns permission list
func (m *ServiceWorkspaceMembershipResolver) Permissions() []string {
	result := make([]string, len(m.membership.Permissions))
	copy(result, m.membership.Permissions)
	return result
}

func (m *ServiceWorkspaceMembershipResolver) Constraints() *ServiceMembershipConstraintsResolver {
	return &ServiceMembershipConstraintsResolver{constraints: m.membership.Constraints}
}

// GrantedAt returns grant timestamp
func (m *ServiceWorkspaceMembershipResolver) GrantedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: m.membership.GrantedAt}
}

// ExpiresAt returns expiration timestamp
func (m *ServiceWorkspaceMembershipResolver) ExpiresAt() *graphshared.DateTime {
	if m.membership.ExpiresAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *m.membership.ExpiresAt}
}

// RevokedAt returns revocation timestamp
func (m *ServiceWorkspaceMembershipResolver) RevokedAt() *graphshared.DateTime {
	if m.membership.RevokedAt == nil {
		return nil
	}
	return &graphshared.DateTime{Time: *m.membership.RevokedAt}
}

func (m *ServiceMembershipConstraintsResolver) RateLimitPerMinute() *int32 {
	return optionalInt32(m.constraints.RateLimitPerMinute)
}

func (m *ServiceMembershipConstraintsResolver) RateLimitPerHour() *int32 {
	return optionalInt32(m.constraints.RateLimitPerHour)
}

func (m *ServiceMembershipConstraintsResolver) AllowedIPs() []string {
	return append([]string(nil), m.constraints.AllowedIPs...)
}

func (m *ServiceMembershipConstraintsResolver) AllowedProjectIDs() []model.ID {
	result := make([]model.ID, len(m.constraints.AllowedProjectIDs))
	for i, projectID := range m.constraints.AllowedProjectIDs {
		result[i] = model.ID(projectID)
	}
	return result
}

func (m *ServiceMembershipConstraintsResolver) AllowedTeamIDs() []model.ID {
	result := make([]model.ID, len(m.constraints.AllowedTeamIDs))
	for i, teamID := range m.constraints.AllowedTeamIDs {
		result[i] = model.ID(teamID)
	}
	return result
}

func (m *ServiceMembershipConstraintsResolver) AllowDelegatedRouting() bool {
	return m.constraints.AllowDelegatedRouting
}

func (m *ServiceMembershipConstraintsResolver) DelegatedRoutingTeamIDs() []model.ID {
	result := make([]model.ID, len(m.constraints.DelegatedRoutingTeamIDs))
	for i, teamID := range m.constraints.DelegatedRoutingTeamIDs {
		result[i] = model.ID(teamID)
	}
	return result
}

func (m *ServiceMembershipConstraintsResolver) ActiveHoursStart() *string {
	return optionalTrimmedString(m.constraints.ActiveHoursStart)
}

func (m *ServiceMembershipConstraintsResolver) ActiveHoursEnd() *string {
	return optionalTrimmedString(m.constraints.ActiveHoursEnd)
}

func (m *ServiceMembershipConstraintsResolver) ActiveTimezone() *string {
	return optionalTrimmedString(m.constraints.ActiveTimezone)
}

func (m *ServiceMembershipConstraintsResolver) ActiveDays() []int32 {
	result := make([]int32, len(m.constraints.ActiveDays))
	for i, day := range m.constraints.ActiveDays {
		result[i] = int32(day)
	}
	return result
}

func normalizeKnowledgeSurface(value string) (knowledgedomain.KnowledgeSurface, error) {
	switch knowledgedomain.KnowledgeSurface(strings.ToLower(strings.TrimSpace(value))) {
	case knowledgedomain.KnowledgeSurfacePrivate:
		return knowledgedomain.KnowledgeSurfacePrivate, nil
	case knowledgedomain.KnowledgeSurfacePublished:
		return knowledgedomain.KnowledgeSurfacePublished, nil
	case knowledgedomain.KnowledgeSurfaceWorkspaceWide:
		return knowledgedomain.KnowledgeSurfaceWorkspaceWide, nil
	default:
		return "", fmt.Errorf("knowledge surface must be one of: %s, %s, %s", knowledgedomain.KnowledgeSurfacePrivate, knowledgedomain.KnowledgeSurfacePublished, knowledgedomain.KnowledgeSurfaceWorkspaceWide)
	}
}

func normalizeKnowledgeReviewStatus(value string) (knowledgedomain.KnowledgeReviewStatus, error) {
	switch knowledgedomain.KnowledgeReviewStatus(strings.ToLower(strings.TrimSpace(value))) {
	case knowledgedomain.KnowledgeReviewStatusDraft:
		return knowledgedomain.KnowledgeReviewStatusDraft, nil
	case knowledgedomain.KnowledgeReviewStatusReviewed:
		return knowledgedomain.KnowledgeReviewStatusReviewed, nil
	case knowledgedomain.KnowledgeReviewStatusApproved:
		return knowledgedomain.KnowledgeReviewStatusApproved, nil
	default:
		return "", fmt.Errorf("knowledge review status must be one of: %s, %s, %s", knowledgedomain.KnowledgeReviewStatusDraft, knowledgedomain.KnowledgeReviewStatusReviewed, knowledgedomain.KnowledgeReviewStatusApproved)
	}
}

func normalizeKnowledgeKindValue(value string) (knowledgedomain.KnowledgeResourceKind, error) {
	normalized, ok := knowledgedomain.ParseKnowledgeResourceKind(value)
	if !ok {
		validKinds := knowledgedomain.KnowledgeResourceKinds()
		valid := make([]string, 0, len(validKinds))
		for _, item := range validKinds {
			valid = append(valid, string(item))
		}
		return "", fmt.Errorf("knowledge kind must be one of: %s", strings.Join(valid, ", "))
	}
	return normalized, nil
}

func canAccessKnowledgeResource(authCtx *platformdomain.AuthContext, resource *knowledgedomain.KnowledgeResource) bool {
	if authCtx == nil || resource == nil {
		return false
	}
	if (resource.Surface == knowledgedomain.KnowledgeSurfaceWorkspaceWide || resource.Surface == knowledgedomain.KnowledgeSurfacePublished) &&
		authCtx.HasWorkspaceAccess(resource.WorkspaceID) {
		return true
	}
	if authCtx.CanAccessTeam(resource.OwnerTeamID) {
		return true
	}
	for _, teamID := range resource.SharedWithTeamIDs {
		if authCtx.CanAccessTeam(teamID) {
			return true
		}
	}
	return false
}
