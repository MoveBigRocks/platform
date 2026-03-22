package sql

import (
	"context"
	"database/sql"
	"strings"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
)

type ConceptSpecStore struct {
	db *SqlxDB
}

func NewConceptSpecStore(db *SqlxDB) *ConceptSpecStore {
	return &ConceptSpecStore{db: db}
}

func (s *ConceptSpecStore) CreateConceptSpec(ctx context.Context, spec *knowledgedomain.ConceptSpec) error {
	normalizePersistedUUID(&spec.ID)

	metadataSchema, err := marshalJSONString(spec.MetadataSchema, "metadata_schema")
	if err != nil {
		return err
	}
	sectionsSchema, err := marshalJSONString(spec.SectionsSchema, "sections_schema")
	if err != nil {
		return err
	}
	workflowSchema, err := marshalJSONString(spec.WorkflowSchema, "workflow_schema")
	if err != nil {
		return err
	}

	query := `
		INSERT INTO core_knowledge.concept_specs (
			id, workspace_id, owner_team_id, spec_key, spec_version, name, description,
			extends_key, extends_version, instance_kind, metadata_schema_json, sections_schema_json,
			workflow_schema_json, agent_guidance_markdown, artifact_path, revision_ref,
			source_kind, source_ref, status, created_by, created_at, updated_at, deleted_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		spec.ID,
		spec.WorkspaceID,
		nullableLegacyUUIDValue(spec.OwnerTeamID),
		spec.Key,
		spec.Version,
		spec.Name,
		spec.Description,
		nullableString(spec.ExtendsKey),
		nullableString(spec.ExtendsVersion),
		string(spec.InstanceKind),
		metadataSchema,
		sectionsSchema,
		workflowSchema,
		spec.AgentGuidanceMD,
		spec.ArtifactPath,
		nullableString(spec.RevisionRef),
		string(spec.SourceKind),
		nullableString(spec.SourceRef),
		string(spec.Status),
		nullableLegacyUUIDValue(spec.CreatedBy),
		spec.CreatedAt,
		spec.UpdatedAt,
		spec.DeletedAt,
	).Scan(&spec.ID)
	return TranslateSqlxError(err, "concept_specs")
}

func (s *ConceptSpecStore) GetConceptSpec(ctx context.Context, workspaceID, key, version string) (*knowledgedomain.ConceptSpec, error) {
	var model models.ConceptSpec
	query := `SELECT * FROM core_knowledge.concept_specs WHERE workspace_id = ? AND spec_key = ? AND spec_version = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, workspaceID, knowledgedomain.NormalizeConceptSpecKey(key), knowledgedomain.NormalizeConceptSpecVersion(version)); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "concept_specs")
	}
	return mapConceptSpecToDomain(&model), nil
}

func (s *ConceptSpecStore) ListWorkspaceConceptSpecs(ctx context.Context, workspaceID string) ([]*knowledgedomain.ConceptSpec, error) {
	var modelsList []models.ConceptSpec
	query := `SELECT * FROM core_knowledge.concept_specs WHERE workspace_id = ? AND deleted_at IS NULL ORDER BY spec_key ASC, spec_version DESC`
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, workspaceID); err != nil {
		return nil, TranslateSqlxError(err, "concept_specs")
	}
	specs := make([]*knowledgedomain.ConceptSpec, len(modelsList))
	for i := range modelsList {
		specs[i] = mapConceptSpecToDomain(&modelsList[i])
	}
	return specs, nil
}

func mapConceptSpecToDomain(model *models.ConceptSpec) *knowledgedomain.ConceptSpec {
	return &knowledgedomain.ConceptSpec{
		ID:              model.ID,
		WorkspaceID:     model.WorkspaceID,
		OwnerTeamID:     valueOrEmpty(model.OwnerTeamID),
		Key:             model.Key,
		Version:         model.Version,
		Name:            model.Name,
		Description:     model.Description,
		ExtendsKey:      valueOrEmpty(model.ExtendsKey),
		ExtendsVersion:  valueOrEmpty(model.ExtendsVersion),
		InstanceKind:    knowledgedomain.KnowledgeResourceKind(model.InstanceKind),
		MetadataSchema:  unmarshalTypedSchemaOrEmpty(model.MetadataSchema, "concept_specs", "metadata_schema_json"),
		SectionsSchema:  unmarshalTypedSchemaOrEmpty(model.SectionsSchema, "concept_specs", "sections_schema_json"),
		WorkflowSchema:  unmarshalTypedSchemaOrEmpty(model.WorkflowSchema, "concept_specs", "workflow_schema_json"),
		AgentGuidanceMD: model.AgentGuidanceMD,
		ArtifactPath:    model.ArtifactPath,
		RevisionRef:     valueOrEmpty(model.RevisionRef),
		SourceKind:      knowledgedomain.ConceptSpecSourceKind(model.SourceKind),
		SourceRef:       valueOrEmpty(model.SourceRef),
		Status:          knowledgedomain.ConceptSpecStatus(model.Status),
		CreatedBy:       valueOrEmpty(model.CreatedBy),
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
		DeletedAt:       model.DeletedAt,
	}
}

func normalizeConceptSpecLookupVersion(value string) string {
	normalized := knowledgedomain.NormalizeConceptSpecVersion(value)
	if strings.TrimSpace(normalized) == "" {
		return "1"
	}
	return normalized
}

var _ shared.ConceptSpecStore = (*ConceptSpecStore)(nil)
