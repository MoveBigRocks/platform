package sql

import (
	"context"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

// =============================================================================
// Email Template Operations
// =============================================================================

func (s *EmailStore) CreateEmailTemplate(ctx context.Context, template *servicedomain.EmailTemplate) error {
	normalizePersistedUUID(&template.ID)
	query := `INSERT INTO core_service.email_templates (
		id, workspace_id, key, name, subject, body_html, body_text, created_at, updated_at
	) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?)
	RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		template.ID, template.WorkspaceID, template.Name, template.Name,
		template.Subject, template.HTMLContent, template.TextContent,
		template.CreatedAt, template.UpdatedAt,
	).Scan(&template.ID)
	return TranslateSqlxError(err, "email_templates")
}

func (s *EmailStore) GetEmailTemplate(ctx context.Context, templateID string) (*servicedomain.EmailTemplate, error) {
	var dbTemplate models.EmailTemplate
	query := `SELECT * FROM core_service.email_templates WHERE id = ?`
	err := s.db.Get(ctx).GetContext(ctx, &dbTemplate, query, templateID)
	if err != nil {
		return nil, TranslateSqlxError(err, "email_templates")
	}
	return s.mapTemplateToDomain(&dbTemplate), nil
}

func (s *EmailStore) UpdateEmailTemplate(ctx context.Context, template *servicedomain.EmailTemplate) error {
	query := `UPDATE core_service.email_templates SET
		workspace_id = ?, key = ?, name = ?, subject = ?,
		body_html = ?, body_text = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ?`

	_, err := s.db.Get(ctx).ExecContext(ctx, query,
		template.WorkspaceID, template.Name, template.Name,
		template.Subject, template.HTMLContent, template.TextContent, time.Now(),
		template.ID, template.WorkspaceID,
	)
	return TranslateSqlxError(err, "email_templates")
}

func (s *EmailStore) ListWorkspaceEmailTemplates(ctx context.Context, workspaceID string) ([]*servicedomain.EmailTemplate, error) {
	var dbTemplates []models.EmailTemplate
	query := `SELECT * FROM core_service.email_templates WHERE workspace_id = ? ORDER BY name ASC`
	err := s.db.Get(ctx).SelectContext(ctx, &dbTemplates, query, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "email_templates")
	}

	result := make([]*servicedomain.EmailTemplate, len(dbTemplates))
	for i, t := range dbTemplates {
		result[i] = s.mapTemplateToDomain(&t)
	}
	return result, nil
}

func (s *EmailStore) mapTemplateToDomain(t *models.EmailTemplate) *servicedomain.EmailTemplate {
	return &servicedomain.EmailTemplate{
		ID:          t.ID,
		WorkspaceID: t.WorkspaceID,
		Name:        t.Name,
		Subject:     t.Subject,
		HTMLContent: t.BodyHTML,
		TextContent: t.BodyText,
		SampleData:  make(map[string]interface{}),
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
