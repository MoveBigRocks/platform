package sql

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
)

type KnowledgeResourceStore struct {
	db *SqlxDB
}

func NewKnowledgeResourceStore(db *SqlxDB) *KnowledgeResourceStore {
	return &KnowledgeResourceStore{db: db}
}

func (s *KnowledgeResourceStore) CreateKnowledgeResource(ctx context.Context, resource *knowledgedomain.KnowledgeResource) error {
	normalizePersistedUUID(&resource.ID)

	frontmatter, err := marshalJSONString(resource.Frontmatter, "frontmatter")
	if err != nil {
		return err
	}

	query := `
		INSERT INTO core_knowledge.knowledge_resources (
			id, workspace_id, owner_team_id, slug, title, kind, source_kind, source_ref, path_ref,
			artifact_path, summary, body_markdown, frontmatter_json, concept_spec_key, concept_spec_version, supported_channels,
			shared_with_team_ids, surface, trust_level, search_keywords, status, review_status,
			content_hash, revision_ref, published_revision_ref, reviewed_at, published_at, published_by,
			created_by, created_at, updated_at, deleted_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		resource.ID,
		resource.WorkspaceID,
		resource.OwnerTeamID,
		resource.Slug,
		resource.Title,
		string(resource.Kind),
		string(resource.SourceKind),
		nullableString(resource.SourceRef),
		nullableString(resource.PathRef),
		resource.ArtifactPath,
		resource.Summary,
		resource.BodyMarkdown,
		frontmatter,
		resource.ConceptSpecKey,
		resource.ConceptSpecVersion,
		pq.Array(resource.SupportedChannels),
		pq.Array(resource.SharedWithTeamIDs),
		string(resource.Surface),
		string(resource.TrustLevel),
		pq.Array(resource.SearchKeywords),
		string(resource.Status),
		string(resource.ReviewStatus),
		nullableString(resource.ContentHash),
		resource.RevisionRef,
		nullableString(resource.PublishedRevision),
		resource.ReviewedAt,
		resource.PublishedAt,
		nullableLegacyUUIDValue(resource.PublishedBy),
		nullableLegacyUUIDValue(resource.CreatedBy),
		resource.CreatedAt,
		resource.UpdatedAt,
		resource.DeletedAt,
	).Scan(&resource.ID)
	return TranslateSqlxError(err, "knowledge_resources")
}

func (s *KnowledgeResourceStore) GetKnowledgeResource(ctx context.Context, resourceID string) (*knowledgedomain.KnowledgeResource, error) {
	var model models.KnowledgeResource
	query := `SELECT * FROM core_knowledge.knowledge_resources WHERE id = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, resourceID); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "knowledge_resources")
	}
	return mapKnowledgeResourceToDomain(&model), nil
}

func (s *KnowledgeResourceStore) GetKnowledgeResourceBySlug(ctx context.Context, workspaceID, teamID string, surface knowledgedomain.KnowledgeSurface, slug string) (*knowledgedomain.KnowledgeResource, error) {
	var model models.KnowledgeResource
	query := `SELECT * FROM core_knowledge.knowledge_resources WHERE workspace_id = ? AND owner_team_id = ? AND surface = ? AND slug = ? AND deleted_at IS NULL`
	if err := s.db.Get(ctx).GetContext(ctx, &model, query, workspaceID, teamID, string(surface), slug); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "knowledge_resources")
	}
	return mapKnowledgeResourceToDomain(&model), nil
}

func (s *KnowledgeResourceStore) UpdateKnowledgeResource(ctx context.Context, resource *knowledgedomain.KnowledgeResource) error {
	frontmatter, err := marshalJSONString(resource.Frontmatter, "frontmatter")
	if err != nil {
		return err
	}

	query := `
		UPDATE core_knowledge.knowledge_resources SET
			owner_team_id = ?, slug = ?, title = ?, kind = ?, source_kind = ?, source_ref = ?, path_ref = ?,
			artifact_path = ?, summary = ?, body_markdown = ?, frontmatter_json = ?, concept_spec_key = ?, concept_spec_version = ?, supported_channels = ?,
			shared_with_team_ids = ?, surface = ?, trust_level = ?, search_keywords = ?, status = ?,
			review_status = ?, content_hash = ?, revision_ref = ?, published_revision_ref = ?, reviewed_at = ?,
			published_at = ?, published_by = ?, updated_at = ?, deleted_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`

	result, err := s.db.Get(ctx).ExecContext(ctx, query,
		resource.OwnerTeamID,
		resource.Slug,
		resource.Title,
		string(resource.Kind),
		string(resource.SourceKind),
		nullableString(resource.SourceRef),
		nullableString(resource.PathRef),
		resource.ArtifactPath,
		resource.Summary,
		resource.BodyMarkdown,
		frontmatter,
		resource.ConceptSpecKey,
		resource.ConceptSpecVersion,
		pq.Array(resource.SupportedChannels),
		pq.Array(resource.SharedWithTeamIDs),
		string(resource.Surface),
		string(resource.TrustLevel),
		pq.Array(resource.SearchKeywords),
		string(resource.Status),
		string(resource.ReviewStatus),
		nullableString(resource.ContentHash),
		resource.RevisionRef,
		nullableString(resource.PublishedRevision),
		resource.ReviewedAt,
		resource.PublishedAt,
		nullableLegacyUUIDValue(resource.PublishedBy),
		resource.UpdatedAt,
		resource.DeletedAt,
		resource.ID,
		resource.WorkspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "knowledge_resources")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *KnowledgeResourceStore) ListWorkspaceKnowledgeResources(ctx context.Context, workspaceID string, filter *knowledgedomain.KnowledgeResourceFilter) ([]*knowledgedomain.KnowledgeResource, int, error) {
	conditions := []string{"workspace_id = ?", "deleted_at IS NULL"}
	args := []interface{}{workspaceID}

	if filter != nil {
		if strings.TrimSpace(filter.TeamID) != "" {
			conditions = append(conditions, "owner_team_id = ?")
			args = append(args, strings.TrimSpace(filter.TeamID))
		}
		if filter.Kind != "" {
			conditions = append(conditions, "kind = ?")
			args = append(args, string(filter.Kind))
		}
		if filter.Status != "" {
			conditions = append(conditions, "status = ?")
			args = append(args, string(filter.Status))
		}
		if filter.Surface != "" {
			conditions = append(conditions, "surface = ?")
			args = append(args, string(filter.Surface))
		}
		if filter.ReviewStatus != "" {
			conditions = append(conditions, "review_status = ?")
			args = append(args, string(filter.ReviewStatus))
		}
		if strings.TrimSpace(filter.Search) != "" {
			conditions = append(conditions, "search_vector @@ plainto_tsquery('simple', ?)")
			args = append(args, filter.Search)
		}
	}

	baseQuery := `FROM core_knowledge.knowledge_resources WHERE ` + strings.Join(conditions, " AND ")

	var total int
	if err := s.db.Get(ctx).GetContext(ctx, &total, `SELECT COUNT(*) `+baseQuery, args...); err != nil {
		return nil, 0, TranslateSqlxError(err, "knowledge_resources")
	}

	query := `SELECT * ` + baseQuery + ` ORDER BY updated_at DESC, id DESC`
	if filter != nil && filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += ` OFFSET ?`
			args = append(args, filter.Offset)
		}
	}

	var modelsList []models.KnowledgeResource
	if err := s.db.Get(ctx).SelectContext(ctx, &modelsList, query, args...); err != nil {
		return nil, 0, TranslateSqlxError(err, "knowledge_resources")
	}

	resources := make([]*knowledgedomain.KnowledgeResource, len(modelsList))
	for i := range modelsList {
		resources[i] = mapKnowledgeResourceToDomain(&modelsList[i])
	}
	return resources, total, nil
}

func (s *KnowledgeResourceStore) DeleteKnowledgeResource(ctx context.Context, workspaceID, resourceID string) error {
	now := time.Now().UTC()
	result, err := s.db.Get(ctx).ExecContext(ctx,
		`UPDATE core_knowledge.knowledge_resources SET deleted_at = ?, updated_at = ? WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`,
		now, now, resourceID, workspaceID,
	)
	if err != nil {
		return TranslateSqlxError(err, "knowledge_resources")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func mapKnowledgeResourceToDomain(model *models.KnowledgeResource) *knowledgedomain.KnowledgeResource {
	return &knowledgedomain.KnowledgeResource{
		ID:                 model.ID,
		WorkspaceID:        model.WorkspaceID,
		OwnerTeamID:        model.OwnerTeamID,
		Slug:               model.Slug,
		Title:              model.Title,
		Kind:               knowledgedomain.KnowledgeResourceKind(model.Kind),
		ConceptSpecKey:     model.ConceptSpecKey,
		ConceptSpecVersion: model.ConceptSpecVersion,
		SourceKind:         knowledgedomain.KnowledgeResourceSourceKind(model.SourceKind),
		SourceRef:          valueOrEmpty(model.SourceRef),
		PathRef:            valueOrEmpty(model.PathRef),
		ArtifactPath:       model.ArtifactPath,
		Summary:            model.Summary,
		BodyMarkdown:       model.BodyMarkdown,
		Frontmatter:        unmarshalTypedSchemaOrEmpty(model.FrontmatterJSON, "knowledge_resources", "frontmatter_json"),
		SupportedChannels:  unmarshalStringArrayField(model.SupportedChannels, "knowledge_resources", "supported_channels"),
		SharedWithTeamIDs:  unmarshalStringArrayField(model.SharedWithTeamIDs, "knowledge_resources", "shared_with_team_ids"),
		Surface:            knowledgedomain.KnowledgeSurface(model.Surface),
		TrustLevel:         knowledgedomain.KnowledgeResourceTrustLevel(model.TrustLevel),
		SearchKeywords:     unmarshalStringArrayField(model.SearchKeywords, "knowledge_resources", "search_keywords"),
		Status:             knowledgedomain.KnowledgeResourceStatus(model.Status),
		ReviewStatus:       knowledgedomain.KnowledgeReviewStatus(model.ReviewStatus),
		ContentHash:        valueOrEmpty(model.ContentHash),
		RevisionRef:        valueOrEmpty(model.RevisionRef),
		PublishedRevision:  valueOrEmpty(model.PublishedRevision),
		ReviewedAt:         model.ReviewedAt,
		PublishedAt:        model.PublishedAt,
		PublishedBy:        valueOrEmpty(model.PublishedBy),
		CreatedBy:          valueOrEmpty(model.CreatedBy),
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		DeletedAt:          model.DeletedAt,
	}
}

var _ shared.KnowledgeResourceStore = (*KnowledgeResourceStore)(nil)
