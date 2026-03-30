package knowledgeservices

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	apierrors "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/errors"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"

	"gopkg.in/yaml.v3"
)

type conceptArtifactService interface {
	Write(ctx context.Context, params artifactservices.WriteParams) (*artifactservices.WriteResult, error)
	History(ctx context.Context, repository artifactservices.RepositoryRef, relativePath string, limit int) ([]artifactservices.Revision, error)
	Diff(ctx context.Context, repository artifactservices.RepositoryRef, relativePath, fromRef, toRef string) (string, string, string, error)
}

type ConceptSpecService struct {
	conceptStore   shared.ConceptSpecStore
	workspaceStore shared.WorkspaceStore
	artifacts      conceptArtifactService
}

type ConceptSpecDiff struct {
	Path         string
	FromRevision string
	ToRevision   string
	Patch        string
}

type RegisterConceptSpecParams struct {
	WorkspaceID     string
	OwnerTeamID     string
	Key             string
	Version         string
	Name            string
	Description     string
	ExtendsKey      string
	ExtendsVersion  string
	InstanceKind    knowledgedomain.KnowledgeResourceKind
	MetadataSchema  shareddomain.TypedSchema
	SectionsSchema  shareddomain.TypedSchema
	WorkflowSchema  shareddomain.TypedSchema
	AgentGuidanceMD string
	ArtifactPath    string
	SourceKind      knowledgedomain.ConceptSpecSourceKind
	SourceRef       string
	Status          knowledgedomain.ConceptSpecStatus
	CreatedBy       string
}

func NewConceptSpecService(
	conceptStore shared.ConceptSpecStore,
	workspaceStore shared.WorkspaceStore,
	artifacts conceptArtifactService,
) *ConceptSpecService {
	return &ConceptSpecService{
		conceptStore:   conceptStore,
		workspaceStore: workspaceStore,
		artifacts:      artifacts,
	}
}

func (s *ConceptSpecService) ListConceptSpecs(ctx context.Context, workspaceID string) ([]*knowledgedomain.ConceptSpec, error) {
	specs := knowledgedomain.BuiltInConceptSpecs()
	if strings.TrimSpace(workspaceID) == "" {
		return sortConceptSpecs(specs), nil
	}
	if s.workspaceStore != nil {
		workspace, err := s.workspaceStore.GetWorkspace(ctx, workspaceID)
		if err != nil || workspace == nil {
			return nil, apierrors.NotFoundError("workspace", workspaceID)
		}
	}
	if s.conceptStore != nil {
		custom, err := s.conceptStore.ListWorkspaceConceptSpecs(ctx, workspaceID)
		if err != nil {
			return nil, apierrors.DatabaseError("list concept specs", err)
		}
		specs = append(specs, custom...)
	}
	return sortConceptSpecs(specs), nil
}

func (s *ConceptSpecService) GetConceptSpec(ctx context.Context, workspaceID, key, version string) (*knowledgedomain.ConceptSpec, error) {
	normalizedKey := knowledgedomain.NormalizeConceptSpecKey(key)
	if normalizedKey == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("key", "required"))
	}
	normalizedVersion := knowledgedomain.NormalizeConceptSpecVersion(version)
	if builtIn, ok := knowledgedomain.LookupBuiltInConceptSpec(normalizedKey, normalizedVersion); ok {
		return builtIn, nil
	}
	if strings.TrimSpace(workspaceID) == "" {
		return nil, apierrors.NotFoundError("concept spec", normalizedKey+"@"+normalizedVersion)
	}
	if s.conceptStore == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "concept spec store not configured")
	}
	spec, err := s.conceptStore.GetConceptSpec(ctx, workspaceID, normalizedKey, normalizedVersion)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, apierrors.NotFoundError("concept spec", normalizedKey+"@"+normalizedVersion)
		}
		return nil, apierrors.DatabaseError("get concept spec", err)
	}
	return spec, nil
}

func (s *ConceptSpecService) RegisterConceptSpec(ctx context.Context, params RegisterConceptSpecParams) (*knowledgedomain.ConceptSpec, error) {
	if strings.TrimSpace(params.WorkspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if err := s.validateWorkspaceTeam(ctx, params.WorkspaceID, params.OwnerTeamID); err != nil {
		return nil, err
	}

	spec := knowledgedomain.NewConceptSpec(params.WorkspaceID, params.OwnerTeamID, params.Key, params.Version, params.Name)
	spec.ID = id.New()
	spec.Description = strings.TrimSpace(params.Description)
	spec.ExtendsKey = knowledgedomain.NormalizeConceptSpecKey(params.ExtendsKey)
	spec.ExtendsVersion = strings.TrimSpace(params.ExtendsVersion)
	if params.InstanceKind != "" {
		spec.InstanceKind = params.InstanceKind
	}
	spec.MetadataSchema = params.MetadataSchema.Clone()
	spec.SectionsSchema = params.SectionsSchema.Clone()
	spec.WorkflowSchema = params.WorkflowSchema.Clone()
	spec.AgentGuidanceMD = strings.TrimSpace(params.AgentGuidanceMD)
	if strings.TrimSpace(params.ArtifactPath) != "" {
		spec.ArtifactPath = strings.TrimSpace(params.ArtifactPath)
	}
	if params.SourceKind != "" {
		spec.SourceKind = params.SourceKind
	}
	spec.SourceRef = strings.TrimSpace(params.SourceRef)
	if params.Status != "" {
		spec.Status = params.Status
	} else {
		spec.Status = knowledgedomain.ConceptSpecStatusActive
	}
	spec.CreatedBy = strings.TrimSpace(params.CreatedBy)

	if spec.SourceKind == knowledgedomain.ConceptSpecSourceKindCore {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("source_kind", "core concept specs are built in and cannot be registered"))
	}
	if strings.HasPrefix(spec.Key, "core/") {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("key", "core/ namespace is reserved"))
	}
	if spec.SourceKind == knowledgedomain.ConceptSpecSourceKindExtension && strings.TrimSpace(spec.SourceRef) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("source_ref", "required for extension concept specs"))
	}
	if err := spec.Validate(); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "concept spec validation failed")
	}
	if _, ok := knowledgedomain.LookupBuiltInConceptSpec(spec.Key, spec.Version); ok {
		return nil, apierrors.Newf(apierrors.ErrorTypeConflict, "concept spec %s@%s already exists", spec.Key, spec.Version)
	}
	if s.conceptStore == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "concept spec store not configured")
	}
	if _, err := s.conceptStore.GetConceptSpec(ctx, spec.WorkspaceID, spec.Key, spec.Version); err == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeConflict, "concept spec %s@%s already exists", spec.Key, spec.Version)
	} else if !errors.Is(err, shared.ErrNotFound) {
		return nil, apierrors.DatabaseError("check concept spec", err)
	}

	if s.artifacts == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}
	content, err := renderConceptSpecYAML(spec)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "render concept spec")
	}
	writeResult, err := s.artifacts.Write(ctx, artifactservices.WriteParams{
		Repository:    artifactservices.WorkspaceRepository(spec.WorkspaceID),
		RelativePath:  spec.ArtifactPath,
		Content:       content,
		CommitMessage: conceptSpecCommitMessage(spec),
		ActorID:       spec.CreatedBy,
	})
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "write concept spec artifact")
	}
	spec.RevisionRef = writeResult.Ref

	if err := s.conceptStore.CreateConceptSpec(ctx, spec); err != nil {
		if errors.Is(err, shared.ErrUniqueViolation) || errors.Is(err, shared.ErrDuplicate) {
			return nil, apierrors.Newf(apierrors.ErrorTypeConflict, "concept spec %s@%s already exists", spec.Key, spec.Version)
		}
		return nil, apierrors.DatabaseError("create concept spec", err)
	}
	return spec, nil
}

func (s *ConceptSpecService) ConceptSpecHistory(ctx context.Context, workspaceID, key, version string, limit int) ([]artifactservices.Revision, error) {
	spec, err := s.GetConceptSpec(ctx, workspaceID, key, version)
	if err != nil {
		return nil, err
	}
	if spec.SourceKind == knowledgedomain.ConceptSpecSourceKindCore || strings.TrimSpace(spec.WorkspaceID) == "" {
		return []artifactservices.Revision{}, nil
	}
	if s.artifacts == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}
	revisions, err := s.artifacts.History(ctx, artifactservices.WorkspaceRepository(spec.WorkspaceID), spec.ArtifactPath, limit)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "load concept spec history")
	}
	return revisions, nil
}

func (s *ConceptSpecService) ConceptSpecDiff(ctx context.Context, workspaceID, key, version, fromRevision, toRevision string) (*ConceptSpecDiff, error) {
	spec, err := s.GetConceptSpec(ctx, workspaceID, key, version)
	if err != nil {
		return nil, err
	}
	if spec.SourceKind == knowledgedomain.ConceptSpecSourceKindCore || strings.TrimSpace(spec.WorkspaceID) == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("key", "built-in concept specs do not have git-backed diff history"))
	}
	if s.artifacts == nil {
		return nil, apierrors.Newf(apierrors.ErrorTypeInternal, "artifact service not configured")
	}
	fromRef, toRef, patch, err := s.artifacts.Diff(ctx, artifactservices.WorkspaceRepository(spec.WorkspaceID), spec.ArtifactPath, fromRevision, toRevision)
	if err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "load concept spec diff")
	}
	return &ConceptSpecDiff{
		Path:         spec.ArtifactPath,
		FromRevision: fromRef,
		ToRevision:   toRef,
		Patch:        patch,
	}, nil
}

func (s *ConceptSpecService) validateWorkspaceTeam(ctx context.Context, workspaceID, teamID string) error {
	if s.workspaceStore == nil {
		return nil
	}
	workspace, err := s.workspaceStore.GetWorkspace(ctx, workspaceID)
	if err != nil || workspace == nil {
		return apierrors.NotFoundError("workspace", workspaceID)
	}
	if strings.TrimSpace(teamID) == "" {
		return nil
	}
	team, err := s.workspaceStore.GetTeam(ctx, teamID)
	if err != nil || team == nil || team.WorkspaceID != workspaceID {
		return apierrors.NotFoundError("team", teamID)
	}
	return nil
}

func conceptSpecCommitMessage(spec *knowledgedomain.ConceptSpec) string {
	return fmt.Sprintf("concept spec register %s@%s", spec.Key, spec.Version)
}

func renderConceptSpecYAML(spec *knowledgedomain.ConceptSpec) ([]byte, error) {
	type conceptSpecYAML struct {
		Key             string                 `yaml:"key"`
		Version         string                 `yaml:"version"`
		Name            string                 `yaml:"name"`
		Description     string                 `yaml:"description,omitempty"`
		ExtendsKey      string                 `yaml:"extends_key,omitempty"`
		ExtendsVersion  string                 `yaml:"extends_version,omitempty"`
		InstanceKind    string                 `yaml:"instance_kind"`
		MetadataSchema  map[string]interface{} `yaml:"metadata_schema,omitempty"`
		SectionsSchema  map[string]interface{} `yaml:"sections_schema,omitempty"`
		WorkflowSchema  map[string]interface{} `yaml:"workflow_schema,omitempty"`
		AgentGuidanceMD string                 `yaml:"agent_guidance_markdown,omitempty"`
		SourceKind      string                 `yaml:"source_kind,omitempty"`
		SourceRef       string                 `yaml:"source_ref,omitempty"`
		Status          string                 `yaml:"status,omitempty"`
	}

	payload := conceptSpecYAML{
		Key:             spec.Key,
		Version:         spec.Version,
		Name:            spec.Name,
		Description:     spec.Description,
		ExtendsKey:      spec.ExtendsKey,
		ExtendsVersion:  spec.ExtendsVersion,
		InstanceKind:    string(spec.InstanceKind),
		MetadataSchema:  nilIfEmptyMap(spec.MetadataSchema.ToMap()),
		SectionsSchema:  nilIfEmptyMap(spec.SectionsSchema.ToMap()),
		WorkflowSchema:  nilIfEmptyMap(spec.WorkflowSchema.ToMap()),
		AgentGuidanceMD: spec.AgentGuidanceMD,
		SourceKind:      string(spec.SourceKind),
		SourceRef:       spec.SourceRef,
		Status:          string(spec.Status),
	}
	return yaml.Marshal(payload)
}

func nilIfEmptyMap(value map[string]interface{}) map[string]interface{} {
	if len(value) == 0 {
		return nil
	}
	return value
}

func sortConceptSpecs(specs []*knowledgedomain.ConceptSpec) []*knowledgedomain.ConceptSpec {
	filtered := make([]*knowledgedomain.ConceptSpec, 0, len(specs))
	for _, spec := range specs {
		if spec == nil {
			continue
		}
		filtered = append(filtered, spec)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Key == filtered[j].Key {
			return filtered[i].Version < filtered[j].Version
		}
		return filtered[i].Key < filtered[j].Key
	})
	return filtered
}
