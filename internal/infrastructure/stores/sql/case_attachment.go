package sql

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

func (s *CaseStore) SaveAttachment(ctx context.Context, att *servicedomain.Attachment, data io.Reader) error {
	storageType := "s3"
	storageBucket := att.S3Bucket

	if att.S3Key == "" {
		return fmt.Errorf("attachment must have S3 key (upload required)")
	}

	metadataJSON, err := marshalJSONString(att.Metadata, "metadata")
	if err != nil {
		return fmt.Errorf("SaveAttachment: %w", err)
	}
	normalizePersistedUUID(&att.ID)

	query := `INSERT INTO core_service.attachments (
		id, workspace_id, filename, original_name, content_type, size, checksum,
		storage_key, storage_type, storage_bucket, case_id, email_id,
		is_scanned, scan_result, scanned_at, description, metadata,
		uploaded_by_id, uploaded_by_type, created_at, updated_at
	) VALUES (
		COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	)
	RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		att.ID, att.WorkspaceID, att.Filename, att.Filename, att.ContentType, att.Size, att.SHA256Hash,
		att.StorageKey, storageType, storageBucket, nullableUUIDValue(att.CaseID), nullableUUIDValue(att.EmailID),
		att.Status != servicedomain.AttachmentStatusPending, att.ScanResult, att.ScannedAt,
		att.Description, metadataJSON, nullableLegacyUUIDValue(att.UploadedBy), string(att.Source), att.CreatedAt, att.UpdatedAt,
	).Scan(&att.ID)
	return TranslateSqlxError(err, "attachments")
}

func (s *CaseStore) GetAttachment(ctx context.Context, workspaceID, attID string) (*servicedomain.Attachment, error) {
	var dbAtt models.Attachment
	query := `SELECT * FROM core_service.attachments WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &dbAtt, query, attID, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "attachments")
	}
	return s.mapAttachmentToDomain(&dbAtt), nil
}

func (s *CaseStore) DeleteAttachment(ctx context.Context, workspaceID, attID string) error {
	// Atomic soft delete: UPDATE + check RowsAffected eliminates the TOCTOU
	// race between a separate SELECT COUNT and UPDATE.
	query := `UPDATE core_service.attachments SET deleted_at = ? WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now(), attID, workspaceID)
	if err != nil {
		return TranslateSqlxError(err, "attachments")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *CaseStore) mapAttachmentToDomain(a *models.Attachment) *servicedomain.Attachment {
	var metadata map[string]string
	if a.Metadata != "" {
		unmarshalJSONField(a.Metadata, &metadata, "attachments", "metadata")
	}

	att := &servicedomain.Attachment{
		ID:          a.ID,
		WorkspaceID: a.WorkspaceID,
		Filename:    a.Filename,
		ContentType: a.ContentType,
		Size:        a.Size,
		StorageKey:  a.StorageKey,
		Status:      servicedomain.AttachmentStatusFromScan(a.IsScanned, a.ScanResult),
		ScanResult:  a.ScanResult,
		ScannedAt:   a.ScannedAt,
		SHA256Hash:  a.Checksum,
		Source:      servicedomain.AttachmentSource(a.UploadedByType),
		CaseID:      derefStringPtr(a.CaseID),
		EmailID:     derefStringPtr(a.EmailID),
		UploadedBy:  derefStringPtr(a.UploadedByID),
		Description: a.Description,
		Metadata:    metadata,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}

	if a.StorageType == "s3" && a.StorageBucket != "" {
		att.SetS3Location(a.StorageBucket, a.StorageKey)
	}

	return att
}

func (s *CaseStore) CreateCaseKnowledgeResourceLink(ctx context.Context, link *knowledgedomain.CaseKnowledgeResourceLink) error {
	normalizePersistedUUID(&link.ID)
	query := `INSERT INTO core_knowledge.case_knowledge_resource_links (
		id, workspace_id, case_id, knowledge_resource_id, linked_by_id, linked_at, is_auto_suggested,
		relevance_score, was_helpful, feedback_by, feedback_at, feedback_comment
	) SELECT
		COALESCE(NULLIF(?, '')::uuid, uuidv7()), cases.workspace_id, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	  FROM core_service.cases
	  WHERE cases.id = ?
	  RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		link.ID, link.CaseID, link.KnowledgeResourceID, nullableLegacyUUIDValue(link.LinkedByID), link.LinkedAt,
		link.IsAutoSuggested, link.RelevanceScore, link.WasHelpful,
		nullableLegacyUUIDValue(link.FeedbackBy), link.FeedbackAt, link.FeedbackComment, link.CaseID,
	).Scan(&link.ID)
	return TranslateSqlxError(err, "case_knowledge_resource_links")
}

func (s *CaseStore) GetCaseKnowledgeResourceLinks(ctx context.Context, caseID string) ([]*knowledgedomain.CaseKnowledgeResourceLink, error) {
	var dbLinks []models.CaseKnowledgeResourceLink
	query := `SELECT
		id,
		workspace_id,
		case_id,
		knowledge_resource_id,
		linked_by_id,
		linked_at,
		is_auto_suggested,
		relevance_score,
		was_helpful,
		feedback_by,
		feedback_at,
		COALESCE(feedback_comment, '') AS feedback_comment
	FROM core_knowledge.case_knowledge_resource_links
	WHERE case_id = ?`
	err := s.db.Get(ctx).SelectContext(ctx, &dbLinks, query, caseID)
	if err != nil {
		return nil, TranslateSqlxError(err, "case_knowledge_resource_links")
	}

	links := make([]*knowledgedomain.CaseKnowledgeResourceLink, len(dbLinks))
	for i, l := range dbLinks {
		links[i] = &knowledgedomain.CaseKnowledgeResourceLink{
			ID:                  l.ID,
			CaseID:              l.CaseID,
			KnowledgeResourceID: l.KnowledgeResourceID,
			LinkedByID:          derefStringPtr(l.LinkedByID),
			LinkedAt:            l.LinkedAt,
			IsAutoSuggested:     l.IsAutoSuggested,
			RelevanceScore:      l.RelevanceScore,
			WasHelpful:          l.WasHelpful,
			FeedbackBy:          derefStringPtr(l.FeedbackBy),
			FeedbackAt:          l.FeedbackAt,
			FeedbackComment:     l.FeedbackComment,
		}
	}
	return links, nil
}

func (s *CaseStore) DeleteCaseKnowledgeResourceLink(ctx context.Context, linkID string) error {
	query := `DELETE FROM core_knowledge.case_knowledge_resource_links WHERE id = ?`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, linkID)
	if err != nil {
		return TranslateSqlxError(err, "case_knowledge_resource_links")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *CaseStore) LinkCaseToKnowledgeResource(ctx context.Context, caseID, knowledgeResourceID string) error {
	// Atomic idempotent insert: INSERT ... WHERE NOT EXISTS eliminates the
	// TOCTOU race between a separate SELECT COUNT and INSERT.
	query := `INSERT INTO core_knowledge.case_knowledge_resource_links (
			id, workspace_id, case_id, knowledge_resource_id, linked_at, is_auto_suggested, relevance_score, was_helpful
		)
		SELECT COALESCE(NULLIF(?, '')::uuid, uuidv7()), cases.workspace_id, ?, ?, ?, FALSE, 0, FALSE
		FROM core_service.cases
		WHERE cases.id = ?
		  AND NOT EXISTS (
			SELECT 1
			FROM core_knowledge.case_knowledge_resource_links
			WHERE case_id = ? AND knowledge_resource_id = ?
		  )`
	now := time.Now()
	_, err := s.db.Get(ctx).ExecContext(ctx, query, "", caseID, knowledgeResourceID, now, caseID, caseID, knowledgeResourceID)
	return TranslateSqlxError(err, "case_knowledge_resource_links")
}
