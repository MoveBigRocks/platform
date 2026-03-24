package knowledgeservices

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/id"
)

type artifactService interface {
	Write(ctx context.Context, params artifactservices.WriteParams) (*artifactservices.WriteResult, error)
	Delete(ctx context.Context, params artifactservices.DeleteParams) (*artifactservices.DeleteResult, error)
	History(ctx context.Context, repository artifactservices.RepositoryRef, relativePath string, limit int) ([]artifactservices.Revision, error)
	Diff(ctx context.Context, repository artifactservices.RepositoryRef, relativePath, fromRef, toRef string) (string, string, string, error)
}

type KnowledgeService struct {
	knowledgeStore shared.KnowledgeResourceStore
	workspaceStore shared.WorkspaceStore
	conceptStore   shared.ConceptSpecStore
	artifacts      artifactService
	outbox         contracts.OutboxPublisher
	tx             contracts.TransactionRunner
}

type CreateKnowledgeResourceParams struct {
	WorkspaceID        string
	TeamID             string
	Slug               string
	Title              string
	Kind               knowledgedomain.KnowledgeResourceKind
	SourceKind         knowledgedomain.KnowledgeResourceSourceKind
	SourceRef          string
	PathRef            string
	Summary            string
	BodyMarkdown       string
	ConceptSpecKey     string
	ConceptSpecVersion string
	Frontmatter        shareddomain.TypedSchema
	SupportedChannels  []string
	SharedWithTeamIDs  []string
	SearchKeywords     []string
	Surface            knowledgedomain.KnowledgeSurface
	Status             knowledgedomain.KnowledgeResourceStatus
	CreatedBy          string
}

type UpdateKnowledgeResourceParams struct {
	Slug               *string
	Title              *string
	Kind               *knowledgedomain.KnowledgeResourceKind
	SourceKind         *knowledgedomain.KnowledgeResourceSourceKind
	SourceRef          *string
	PathRef            *string
	Summary            *string
	BodyMarkdown       *string
	ConceptSpecKey     *string
	ConceptSpecVersion *string
	Frontmatter        *shareddomain.TypedSchema
	SupportedChannels  *[]string
	SearchKeywords     *[]string
	Status             *knowledgedomain.KnowledgeResourceStatus
}

type KnowledgeDiff struct {
	Path         string
	FromRevision string
	ToRevision   string
	Patch        string
}

func NewKnowledgeService(
	knowledgeStore shared.KnowledgeResourceStore,
	workspaceStore shared.WorkspaceStore,
	conceptStore shared.ConceptSpecStore,
	artifacts artifactService,
	outbox contracts.OutboxPublisher,
	tx contracts.TransactionRunner,
) *KnowledgeService {
	return &KnowledgeService{
		knowledgeStore: knowledgeStore,
		workspaceStore: workspaceStore,
		conceptStore:   conceptStore,
		artifacts:      artifacts,
		outbox:         outbox,
		tx:             tx,
	}
}

func (s *KnowledgeService) CreateKnowledgeResource(ctx context.Context, params CreateKnowledgeResourceParams) (*knowledgedomain.KnowledgeResource, error) {
	if strings.TrimSpace(params.WorkspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if strings.TrimSpace(params.TeamID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("team_id", "required"))
	}
	if strings.TrimSpace(params.Title) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("title", "required"))
	}
	if err := s.validateWorkspaceTeam(ctx, params.WorkspaceID, params.TeamID); err != nil {
		return nil, err
	}

	resource := knowledgedomain.NewKnowledgeResource(params.WorkspaceID, params.TeamID, params.Slug, params.Title)
	resource.ID = id.New()
	if params.Kind != "" {
		normalizedKind, err := normalizeKnowledgeKind(params.Kind)
		if err != nil {
			return nil, err
		}
		resource.Kind = normalizedKind
	}
	if params.SourceKind != "" {
		resource.SourceKind = params.SourceKind
	}
	if params.Surface != "" {
		resource.Surface = params.Surface
	}
	if params.Status != "" {
		resource.Status = params.Status
	}
	explicitConcept := strings.TrimSpace(params.ConceptSpecKey) != "" || strings.TrimSpace(params.ConceptSpecVersion) != ""
	resolvedKind, conceptKey, conceptVersion, err := s.resolveKnowledgeConcept(ctx, resource.WorkspaceID, resource.Kind, params.ConceptSpecKey, params.ConceptSpecVersion, explicitConcept && params.Kind == "")
	if err != nil {
		return nil, err
	}
	resource.Kind = resolvedKind
	resource.ConceptSpecKey = conceptKey
	resource.ConceptSpecVersion = conceptVersion
	resource.SourceRef = strings.TrimSpace(params.SourceRef)
	resource.PathRef = strings.TrimSpace(params.PathRef)
	resource.Summary = strings.TrimSpace(params.Summary)
	resource.BodyMarkdown = strings.TrimSpace(params.BodyMarkdown)
	resource.Frontmatter = params.Frontmatter.Clone()
	resource.SupportedChannels = normalizeStringList(params.SupportedChannels)
	resource.SharedWithTeamIDs = normalizeStringList(params.SharedWithTeamIDs)
	resource.SearchKeywords = normalizeStringList(params.SearchKeywords)
	resource.CreatedBy = strings.TrimSpace(params.CreatedBy)
	resource.ContentHash = knowledgeContentHash(resource.BodyMarkdown)
	resource.ArtifactPath = knowledgeArtifactPath(resource)

	if err := resource.Validate(); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "knowledge validation failed")
	}
	if err := s.validateKnowledgeResourceConceptRules(ctx, resource); err != nil {
		return nil, err
	}
	if _, err := s.knowledgeStore.GetKnowledgeResourceBySlug(ctx, resource.WorkspaceID, resource.OwnerTeamID, resource.Surface, resource.Slug); err == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeConflict, "knowledge resource %s already exists for team and surface", resource.Slug)
	} else if !errors.Is(err, shared.ErrNotFound) {
		return nil, apierrors.DatabaseError("check knowledge slug", err)
	}

	if err := s.persistKnowledgeChange(ctx, resource, "", "create"); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *KnowledgeService) GetKnowledgeResource(ctx context.Context, resourceID string) (*knowledgedomain.KnowledgeResource, error) {
	if strings.TrimSpace(resourceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("resource_id", "required"))
	}
	resource, err := s.knowledgeStore.GetKnowledgeResource(ctx, resourceID)
	if err != nil {
		return nil, apierrors.NotFoundError("knowledge resource", resourceID)
	}
	return resource, nil
}

func (s *KnowledgeService) GetKnowledgeResourceBySlug(ctx context.Context, workspaceID, teamID string, surface knowledgedomain.KnowledgeSurface, slug string) (*knowledgedomain.KnowledgeResource, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if strings.TrimSpace(teamID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("team_id", "required"))
	}
	normalizedSlug := knowledgedomain.NormalizeKnowledgeSlug(slug, "")
	if normalizedSlug == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("slug", "required"))
	}
	if surface == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("surface", "required"))
	}
	resource, err := s.knowledgeStore.GetKnowledgeResourceBySlug(ctx, workspaceID, teamID, surface, normalizedSlug)
	if err != nil {
		return nil, apierrors.NotFoundError("knowledge resource", normalizedSlug)
	}
	return resource, nil
}

func (s *KnowledgeService) ListWorkspaceKnowledgeResources(ctx context.Context, workspaceID string, filter knowledgedomain.KnowledgeResourceFilter) ([]*knowledgedomain.KnowledgeResource, int, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, 0, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	resources, total, err := s.knowledgeStore.ListWorkspaceKnowledgeResources(ctx, workspaceID, &filter)
	if err != nil {
		return nil, 0, apierrors.DatabaseError("list knowledge resources", err)
	}
	return resources, total, nil
}

func (s *KnowledgeService) UpdateKnowledgeResource(ctx context.Context, resourceID string, params UpdateKnowledgeResourceParams) (*knowledgedomain.KnowledgeResource, error) {
	resource, err := s.GetKnowledgeResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	previousPath := resource.ArtifactPath

	if params.Slug != nil {
		if err := resource.SetSlug(*params.Slug); err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "knowledge validation failed")
		}
	}
	if params.Title != nil {
		resource.Title = strings.TrimSpace(*params.Title)
	}
	if params.Kind != nil {
		normalizedKind, err := normalizeKnowledgeKind(*params.Kind)
		if err != nil {
			return nil, err
		}
		resource.Kind = normalizedKind
	}
	if params.SourceKind != nil {
		resource.SourceKind = *params.SourceKind
	}
	if params.SourceRef != nil {
		resource.SourceRef = strings.TrimSpace(*params.SourceRef)
	}
	if params.PathRef != nil {
		resource.PathRef = strings.TrimSpace(*params.PathRef)
	}
	if params.Summary != nil {
		resource.Summary = strings.TrimSpace(*params.Summary)
	}
	if params.BodyMarkdown != nil {
		resource.BodyMarkdown = strings.TrimSpace(*params.BodyMarkdown)
	}
	if params.Frontmatter != nil {
		resource.Frontmatter = params.Frontmatter.Clone()
	}
	if params.SupportedChannels != nil {
		resource.SupportedChannels = normalizeStringList(*params.SupportedChannels)
	}
	if params.SearchKeywords != nil {
		resource.SearchKeywords = normalizeStringList(*params.SearchKeywords)
	}
	if params.Status != nil {
		resource.Status = *params.Status
	}
	if params.Kind != nil || params.ConceptSpecKey != nil || params.ConceptSpecVersion != nil {
		conceptKey := resource.ConceptSpecKey
		conceptVersion := resource.ConceptSpecVersion
		if params.Kind != nil && params.ConceptSpecKey == nil && params.ConceptSpecVersion == nil {
			conceptKey, conceptVersion = knowledgedomain.DefaultConceptSpecForKind(resource.Kind)
		}
		if params.ConceptSpecKey != nil {
			conceptKey = strings.TrimSpace(*params.ConceptSpecKey)
		}
		if params.ConceptSpecVersion != nil {
			conceptVersion = strings.TrimSpace(*params.ConceptSpecVersion)
		}
		explicitConcept := params.ConceptSpecKey != nil || params.ConceptSpecVersion != nil
		resolvedKind, resolvedKey, resolvedVersion, err := s.resolveKnowledgeConcept(ctx, resource.WorkspaceID, resource.Kind, conceptKey, conceptVersion, explicitConcept && params.Kind == nil)
		if err != nil {
			return nil, err
		}
		resource.Kind = resolvedKind
		resource.ConceptSpecKey = resolvedKey
		resource.ConceptSpecVersion = resolvedVersion
	}

	resource.ContentHash = knowledgeContentHash(resource.BodyMarkdown)
	resource.ArtifactPath = knowledgeArtifactPath(resource)
	resource.UpdatedAt = time.Now().UTC()

	if err := resource.Validate(); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "knowledge validation failed")
	}
	if err := s.validateKnowledgeResourceConceptRules(ctx, resource); err != nil {
		return nil, err
	}

	existing, err := s.knowledgeStore.GetKnowledgeResourceBySlug(ctx, resource.WorkspaceID, resource.OwnerTeamID, resource.Surface, resource.Slug)
	if err == nil && existing != nil && existing.ID != resource.ID {
		return nil, apierrors.Newf(apierrors.ErrorTypeConflict, "knowledge resource %s already exists for team and surface", resource.Slug)
	} else if err != nil && !errors.Is(err, shared.ErrNotFound) {
		return nil, apierrors.DatabaseError("check knowledge slug", err)
	}

	if err := s.persistKnowledgeChange(ctx, resource, previousPath, "update"); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *KnowledgeService) ReviewKnowledgeResource(ctx context.Context, resourceID, actorID string, status knowledgedomain.KnowledgeReviewStatus) (*knowledgedomain.KnowledgeResource, error) {
	resource, err := s.GetKnowledgeResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if status == "" {
		status = knowledgedomain.KnowledgeReviewStatusReviewed
	}
	now := time.Now().UTC()
	resource.ReviewStatus = status
	resource.ReviewedAt = &now
	resource.TrustLevel = knowledgedomain.KnowledgeResourceTrustLevelReviewed
	resource.UpdatedAt = now
	if err := s.validateKnowledgeResourceConceptRules(ctx, resource); err != nil {
		return nil, err
	}
	if err := s.persistKnowledgeChange(ctx, resource, resource.ArtifactPath, "review"); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *KnowledgeService) PublishKnowledgeResource(ctx context.Context, resourceID, actorID string, surface knowledgedomain.KnowledgeSurface) (*knowledgedomain.KnowledgeResource, error) {
	resource, err := s.GetKnowledgeResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if surface == "" {
		surface = knowledgedomain.KnowledgeSurfacePublished
	}
	if surface == knowledgedomain.KnowledgeSurfacePrivate {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("surface", "publish requires a non-private surface"))
	}
	previousPath := resource.ArtifactPath
	now := time.Now().UTC()
	resource.Surface = surface
	resource.ReviewStatus = knowledgedomain.KnowledgeReviewStatusApproved
	resource.TrustLevel = knowledgedomain.KnowledgeResourceTrustLevelReviewed
	resource.ReviewedAt = &now
	resource.PublishedAt = &now
	resource.PublishedBy = strings.TrimSpace(actorID)
	resource.Status = knowledgedomain.KnowledgeResourceStatusActive
	resource.UpdatedAt = now
	resource.ArtifactPath = knowledgeArtifactPath(resource)
	if err := s.validateKnowledgeResourceConceptRules(ctx, resource); err != nil {
		return nil, err
	}
	if err := s.persistKnowledgeChange(ctx, resource, previousPath, "publish"); err != nil {
		return nil, err
	}
	resource.PublishedRevision = resource.RevisionRef
	if err := s.updateKnowledgeResource(ctx, resource); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *KnowledgeService) ShareKnowledgeResource(ctx context.Context, resourceID string, teamIDs []string) (*knowledgedomain.KnowledgeResource, error) {
	resource, err := s.GetKnowledgeResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	resource.SharedWithTeamIDs = normalizeStringList(teamIDs)
	resource.UpdatedAt = time.Now().UTC()
	if err := s.validateKnowledgeResourceConceptRules(ctx, resource); err != nil {
		return nil, err
	}
	if err := s.persistKnowledgeChange(ctx, resource, resource.ArtifactPath, "share"); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *KnowledgeService) DeleteKnowledgeResource(ctx context.Context, resourceID, actorID string) (*knowledgedomain.KnowledgeResource, error) {
	resource, err := s.GetKnowledgeResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if s.artifacts == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}

	run := func(txCtx context.Context) error {
		deleteResult, err := s.artifacts.Delete(txCtx, artifactservices.DeleteParams{
			Repository:    artifactservices.WorkspaceRepository(resource.WorkspaceID),
			RelativePath:  resource.ArtifactPath,
			CommitMessage: knowledgeCommitMessage("delete", resource),
			ActorID:       firstNonEmpty(strings.TrimSpace(actorID), resource.PublishedBy, resource.CreatedBy),
		})
		if err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeInternal, "delete knowledge artifact")
		}
		resource.RevisionRef = deleteResult.Ref
		resource.UpdatedAt = time.Now().UTC()
		if err := s.knowledgeStore.DeleteKnowledgeResource(txCtx, resource.WorkspaceID, resource.ID); err != nil {
			if errors.Is(err, shared.ErrNotFound) {
				return apierrors.NotFoundError("knowledge resource", resource.ID)
			}
			return apierrors.DatabaseError("delete knowledge resource", err)
		}
		return nil
	}

	if s.tx != nil {
		if err := s.tx.WithTransaction(ctx, run); err != nil {
			return nil, err
		}
		return resource, nil
	}
	if err := run(ctx); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *KnowledgeService) KnowledgeHistory(ctx context.Context, resourceID string, limit int) ([]artifactservices.Revision, error) {
	resource, err := s.GetKnowledgeResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if s.artifacts == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}
	revisions, err := s.artifacts.History(ctx, artifactservices.WorkspaceRepository(resource.WorkspaceID), resource.ArtifactPath, limit)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "load knowledge history")
	}
	return revisions, nil
}

func (s *KnowledgeService) KnowledgeDiff(ctx context.Context, resourceID, fromRevision, toRevision string) (*KnowledgeDiff, error) {
	resource, err := s.GetKnowledgeResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if s.artifacts == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}
	fromRef, toRef, patch, err := s.artifacts.Diff(ctx, artifactservices.WorkspaceRepository(resource.WorkspaceID), resource.ArtifactPath, fromRevision, toRevision)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "load knowledge diff")
	}
	return &KnowledgeDiff{
		Path:         resource.ArtifactPath,
		FromRevision: fromRef,
		ToRevision:   toRef,
		Patch:        patch,
	}, nil
}

func (s *KnowledgeService) persistKnowledgeChange(ctx context.Context, resource *knowledgedomain.KnowledgeResource, previousPath, action string) error {
	if resource == nil {
		return apierrors.Newf(apierrors.ErrorTypeValidation, "resource is required")
	}
	if s.artifacts == nil {
		return apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}
	run := func(txCtx context.Context) error {
		existing, err := s.knowledgeStore.GetKnowledgeResource(txCtx, resource.ID)
		create := false
		if err != nil {
			if !errors.Is(err, shared.ErrNotFound) {
				return apierrors.DatabaseError("load knowledge resource", err)
			}
			create = true
		}
		if create {
			if err := s.knowledgeStore.CreateKnowledgeResource(txCtx, resource); err != nil {
				if errors.Is(err, shared.ErrUniqueViolation) {
					return apierrors.Newf(apierrors.ErrorTypeConflict, "knowledge resource %s already exists for team and surface", resource.Slug)
				}
				return apierrors.DatabaseError("create knowledge resource", err)
			}
		} else {
			_ = existing
		}

		rendered, err := renderKnowledgeMarkdown(resource)
		if err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeValidation, "render knowledge markdown")
		}
		writeResult, err := s.artifacts.Write(txCtx, artifactservices.WriteParams{
			Repository:    artifactservices.WorkspaceRepository(resource.WorkspaceID),
			RelativePath:  resource.ArtifactPath,
			PreviousPath:  previousPath,
			Content:       []byte(rendered),
			CommitMessage: knowledgeCommitMessage(action, resource),
			ActorID:       firstNonEmpty(resource.PublishedBy, resource.CreatedBy),
		})
		if err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeInternal, "write knowledge artifact")
		}
		resource.RevisionRef = writeResult.Ref
		resource.UpdatedAt = time.Now().UTC()
		if err := s.updateKnowledgeResource(txCtx, resource); err != nil {
			return err
		}
		if err := s.publishKnowledgeSignals(txCtx, action, resource); err != nil {
			return err
		}
		return nil
	}

	if s.tx != nil {
		return s.tx.WithTransaction(ctx, run)
	}
	return run(ctx)
}

func (s *KnowledgeService) updateKnowledgeResource(ctx context.Context, resource *knowledgedomain.KnowledgeResource) error {
	if err := s.knowledgeStore.UpdateKnowledgeResource(ctx, resource); err != nil {
		if errors.Is(err, shared.ErrUniqueViolation) {
			return apierrors.Newf(apierrors.ErrorTypeConflict, "knowledge resource %s already exists for team and surface", resource.Slug)
		}
		return apierrors.DatabaseError("update knowledge resource", err)
	}
	return nil
}

func (s *KnowledgeService) validateWorkspaceTeam(ctx context.Context, workspaceID, teamID string) error {
	if s.workspaceStore == nil {
		return nil
	}
	workspace, err := s.workspaceStore.GetWorkspace(ctx, workspaceID)
	if err != nil || workspace == nil {
		return apierrors.NotFoundError("workspace", workspaceID)
	}
	team, err := s.workspaceStore.GetTeam(ctx, teamID)
	if err != nil || team == nil || team.WorkspaceID != workspaceID {
		return apierrors.NotFoundError("team", teamID)
	}
	return nil
}

func (s *KnowledgeService) publishKnowledgeSignals(ctx context.Context, action string, resource *knowledgedomain.KnowledgeResource) error {
	if s.outbox == nil || resource == nil {
		return nil
	}

	if strings.EqualFold(action, "create") {
		createdEvent := shareddomain.NewKnowledgeCreatedEvent(
			resource.ID,
			resource.WorkspaceID,
			resource.OwnerTeamID,
			resource.Slug,
			resource.Title,
			string(resource.Kind),
			string(resource.Surface),
			string(resource.ReviewStatus),
			resource.CreatedBy,
		)
		if err := s.outbox.PublishEvent(ctx, eventbus.StreamKnowledgeEvents, createdEvent); err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeInternal, "publish knowledge created event")
		}
	}

	if shouldNotifyRFCReview(action, resource) {
		reviewEvent := shareddomain.NewKnowledgeReviewRequestedEvent(
			resource.ID,
			resource.WorkspaceID,
			resource.OwnerTeamID,
			resource.Slug,
			resource.Title,
			string(resource.Kind),
			string(resource.Surface),
			string(resource.ReviewStatus),
			firstNonEmpty(resource.CreatedBy, resource.PublishedBy, "knowledge_service"),
		)
		if err := s.outbox.PublishEvent(ctx, eventbus.StreamKnowledgeEvents, reviewEvent); err != nil {
			return apierrors.Wrap(err, apierrors.ErrorTypeInternal, "publish knowledge review event")
		}

		recipients, err := s.reviewRecipientUserIDs(ctx, resource)
		if err != nil {
			return err
		}
		if len(recipients) > 0 {
			notification := sharedevents.NewSendNotificationRequestedEvent(
				resource.WorkspaceID,
				"knowledge_service",
				"in_app",
				recipients,
			)
			notification.Subject = fmt.Sprintf("New RFC: %s", resource.Title)
			notification.Body = "A new RFC is ready for team review."
			notification.SourceType = "knowledge_review"
			notification.SourceID = resource.ID
			notification.Data = map[string]interface{}{
				"knowledge_id":  resource.ID,
				"team_id":       resource.OwnerTeamID,
				"kind":          string(resource.Kind),
				"slug":          resource.Slug,
				"title":         resource.Title,
				"surface":       string(resource.Surface),
				"review_status": string(resource.ReviewStatus),
			}
			if err := s.outbox.PublishEvent(ctx, eventbus.StreamNotificationCommands, notification); err != nil {
				return apierrors.Wrap(err, apierrors.ErrorTypeInternal, "publish knowledge review notification")
			}
		}
	}

	return nil
}

func shouldNotifyRFCReview(action string, resource *knowledgedomain.KnowledgeResource) bool {
	if resource == nil {
		return false
	}
	if resource.ConceptSpecKey != "core/rfc" || resource.ConceptSpecVersion != "1" {
		return false
	}
	if strings.TrimSpace(action) != "create" {
		return false
	}
	return true
}

func (s *KnowledgeService) reviewRecipientUserIDs(ctx context.Context, resource *knowledgedomain.KnowledgeResource) ([]string, error) {
	if s.workspaceStore == nil || resource == nil {
		return nil, nil
	}
	members, err := s.workspaceStore.GetTeamMembers(ctx, resource.WorkspaceID, resource.OwnerTeamID)
	if err != nil {
		return nil, apierrors.DatabaseError("list team members", err)
	}
	recipients := make([]string, 0, len(members))
	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		if member == nil || !member.IsActive {
			continue
		}
		userID := strings.TrimSpace(member.UserID)
		if userID == "" || userID == strings.TrimSpace(resource.CreatedBy) {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		recipients = append(recipients, userID)
	}
	sort.Strings(recipients)
	return recipients, nil
}

func knowledgeCommitMessage(action string, resource *knowledgedomain.KnowledgeResource) string {
	switch strings.TrimSpace(action) {
	case "create":
		return fmt.Sprintf("knowledge create %s", resource.ArtifactPath)
	case "review":
		return fmt.Sprintf("knowledge review %s", resource.ArtifactPath)
	case "publish":
		return fmt.Sprintf("knowledge publish %s", resource.ArtifactPath)
	case "share":
		return fmt.Sprintf("knowledge share %s", resource.ArtifactPath)
	case "delete":
		return fmt.Sprintf("knowledge delete %s", resource.ArtifactPath)
	default:
		return fmt.Sprintf("knowledge update %s", resource.ArtifactPath)
	}
}

func knowledgeArtifactPath(resource *knowledgedomain.KnowledgeResource) string {
	return strings.Join([]string{
		"knowledge",
		"teams",
		resource.OwnerTeamID,
		string(resource.Surface),
		resource.Slug + ".md",
	}, "/")
}

func renderKnowledgeMarkdown(resource *knowledgedomain.KnowledgeResource) (string, error) {
	return knowledgedomain.RenderKnowledgeMarkdown(resource)
}

func knowledgeContentHash(bodyMarkdown string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(bodyMarkdown)))
	return hex.EncodeToString(sum[:])
}

func normalizeKnowledgeKind(kind knowledgedomain.KnowledgeResourceKind) (knowledgedomain.KnowledgeResourceKind, error) {
	if strings.TrimSpace(string(kind)) == "" {
		return "", nil
	}
	normalized, ok := knowledgedomain.ParseKnowledgeResourceKind(string(kind))
	if !ok {
		validKinds := knowledgedomain.KnowledgeResourceKinds()
		valid := make([]string, 0, len(validKinds))
		for _, item := range validKinds {
			valid = append(valid, string(item))
		}
		return "", apierrors.NewValidationErrors(apierrors.NewValidationError("kind", fmt.Sprintf("must be one of %s", strings.Join(valid, ", "))))
	}
	return normalized, nil
}

func (s *KnowledgeService) resolveKnowledgeConcept(ctx context.Context, workspaceID string, kind knowledgedomain.KnowledgeResourceKind, conceptKey, conceptVersion string, useSpecInstanceKind bool) (knowledgedomain.KnowledgeResourceKind, string, string, error) {
	candidateKind := kind
	if candidateKind == "" {
		candidateKind = knowledgedomain.KnowledgeResourceKindGuide
	}
	normalizedKey := knowledgedomain.NormalizeConceptSpecKey(conceptKey)
	normalizedVersion := knowledgedomain.NormalizeConceptSpecVersion(conceptVersion)
	if normalizedKey == "" {
		normalizedKey, normalizedVersion = knowledgedomain.DefaultConceptSpecForKind(candidateKind)
	}

	spec, err := s.lookupConceptSpec(ctx, workspaceID, normalizedKey, normalizedVersion)
	if err != nil {
		return "", "", "", err
	}
	if useSpecInstanceKind {
		candidateKind = spec.InstanceKind
	}
	if candidateKind != spec.InstanceKind {
		return "", "", "", apierrors.NewValidationErrors(apierrors.NewValidationError("concept_spec_key", fmt.Sprintf("concept spec %s@%s requires knowledge kind %s", spec.Key, spec.Version, spec.InstanceKind)))
	}
	return candidateKind, spec.Key, spec.Version, nil
}

func (s *KnowledgeService) lookupConceptSpec(ctx context.Context, workspaceID, key, version string) (*knowledgedomain.ConceptSpec, error) {
	if spec, ok := knowledgedomain.LookupBuiltInConceptSpec(key, version); ok {
		return spec, nil
	}
	if s.conceptStore == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeValidation, "unknown concept spec %s@%s", key, version)
	}
	spec, err := s.conceptStore.GetConceptSpec(ctx, workspaceID, key, version)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("concept_spec_key", fmt.Sprintf("unknown concept spec %s@%s", key, version)))
		}
		return nil, apierrors.DatabaseError("get concept spec", err)
	}
	return spec, nil
}

func (s *KnowledgeService) validateKnowledgeResourceConceptRules(ctx context.Context, resource *knowledgedomain.KnowledgeResource) error {
	if resource == nil {
		return nil
	}

	spec, err := s.lookupConceptSpec(ctx, resource.WorkspaceID, resource.ConceptSpecKey, resource.ConceptSpecVersion)
	if err != nil {
		return err
	}
	if !shouldEnforceConceptSpecRules(spec) {
		return nil
	}

	validationErrors := make([]apierrors.ValidationError, 0, 3)
	if missing := missingConceptMetadataFields(spec.MetadataSchema, resource.Frontmatter); len(missing) > 0 {
		validationErrors = append(validationErrors, apierrors.NewValidationError("frontmatter", fmt.Sprintf("concept spec %s@%s requires frontmatter fields: %s", spec.Key, spec.Version, strings.Join(missing, ", "))))
	}
	if missing := missingConceptSections(spec.SectionsSchema, resource); len(missing) > 0 {
		validationErrors = append(validationErrors, apierrors.NewValidationError("body_markdown", fmt.Sprintf("concept spec %s@%s requires sections: %s", spec.Key, spec.Version, strings.Join(missing, ", "))))
	}
	if allowedStates := schemaStringList(spec.WorkflowSchema, "states"); len(allowedStates) > 0 && !matchesAnyLifecycleState(knowledgeLifecycleStateCandidates(resource), allowedStates) {
		validationErrors = append(validationErrors, apierrors.NewValidationError("review_status", fmt.Sprintf("concept spec %s@%s allows workflow states: %s", spec.Key, spec.Version, strings.Join(allowedStates, ", "))))
	}
	if len(validationErrors) > 0 {
		return apierrors.NewValidationErrors(validationErrors...)
	}
	return nil
}

func shouldEnforceConceptSpecRules(spec *knowledgedomain.ConceptSpec) bool {
	if spec == nil {
		return false
	}
	if spec.SourceKind != knowledgedomain.ConceptSpecSourceKindCore {
		return true
	}
	return spec.MetadataSchema.GetBool("enforce") || spec.SectionsSchema.GetBool("enforce") || spec.WorkflowSchema.GetBool("enforce")
}

func missingConceptMetadataFields(schema shareddomain.TypedSchema, frontmatter shareddomain.TypedSchema) []string {
	required := schemaRequiredFields(schema)
	if len(required) == 0 {
		return nil
	}
	values := frontmatter.ToMap()
	missing := make([]string, 0, len(required))
	for _, field := range required {
		if !hasRequiredSchemaValue(values[field]) {
			missing = append(missing, field)
		}
	}
	sort.Strings(missing)
	return missing
}

func missingConceptSections(schema shareddomain.TypedSchema, resource *knowledgedomain.KnowledgeResource) []string {
	required := schemaRequiredFields(schema)
	if len(required) == 0 || resource == nil {
		return nil
	}

	headings := extractMarkdownSectionNames(resource.BodyMarkdown)
	if strings.TrimSpace(resource.Summary) != "" {
		headings["summary"] = struct{}{}
	}

	missing := make([]string, 0, len(required))
	for _, section := range required {
		if _, ok := headings[normalizeConceptSectionName(section)]; ok {
			continue
		}
		missing = append(missing, section)
	}
	sort.Strings(missing)
	return missing
}

func schemaRequiredFields(schema shareddomain.TypedSchema) []string {
	required := schemaListValue(schema, "required")
	if len(required) == 0 {
		return nil
	}
	return normalizeStringList(required)
}

func schemaStringList(schema shareddomain.TypedSchema, key string) []string {
	values := schemaListValue(schema, key)
	if len(values) == 0 {
		return nil
	}
	return normalizeStringList(values)
}

func schemaListValue(schema shareddomain.TypedSchema, key string) []string {
	raw, ok := schema.Get(key)
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if value, ok := item.(string); ok {
				values = append(values, value)
			}
		}
		return values
	default:
		return nil
	}
}

func hasRequiredSchemaValue(value interface{}) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []string:
		return len(typed) > 0
	case []interface{}:
		return len(typed) > 0
	case map[string]interface{}:
		return len(typed) > 0
	default:
		return true
	}
}

func extractMarkdownSectionNames(body string) map[string]struct{} {
	headings := make(map[string]struct{})
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if title == "" {
			continue
		}
		headings[normalizeConceptSectionName(title)] = struct{}{}
	}
	return headings
}

func normalizeConceptSectionName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastSeparator := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastSeparator = false
		case r == '_' || r == '-' || unicode.IsSpace(r):
			if !lastSeparator && b.Len() > 0 {
				b.WriteByte('_')
				lastSeparator = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func knowledgeLifecycleStateCandidates(resource *knowledgedomain.KnowledgeResource) []string {
	if resource == nil {
		return nil
	}
	switch resource.Status {
	case knowledgedomain.KnowledgeResourceStatusArchived:
		return []string{"archived"}
	}
	switch resource.ReviewStatus {
	case knowledgedomain.KnowledgeReviewStatusApproved:
		return []string{"approved"}
	case knowledgedomain.KnowledgeReviewStatusReviewed:
		return []string{"reviewed", "in_review"}
	default:
		return []string{"draft"}
	}
}

func matchesAnyLifecycleState(candidates, allowed []string) bool {
	if len(candidates) == 0 || len(allowed) == 0 {
		return false
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		allowedSet[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	for _, candidate := range candidates {
		if _, ok := allowedSet[strings.ToLower(strings.TrimSpace(candidate))]; ok {
			return true
		}
	}
	return false
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
