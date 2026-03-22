package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

// EmailStore implements shared.EmailStore using SQLite/sqlx.
// Methods are organized across multiple files:
//   - email_store.go: struct, constructor, outbound operations
//   - email_inbound.go: inbound email operations
//   - email_template.go: template operations
//   - email_thread.go: thread operations
//   - email_security.go: blacklist operations
type EmailStore struct {
	db *SqlxDB
}

func NewEmailStore(db *SqlxDB) *EmailStore {
	return &EmailStore{db: db}
}

// =============================================================================
// Outbound Email Operations
// =============================================================================

func (s *EmailStore) CreateOutboundEmail(ctx context.Context, email *servicedomain.OutboundEmail) error {
	normalizePersistedUUID(&email.ID)
	toEmails, err := marshalJSONString(email.ToEmails, "to_emails")
	if err != nil {
		return fmt.Errorf("create outbound email: %w", err)
	}
	ccEmails, err := marshalJSONString(email.CCEmails, "cc_emails")
	if err != nil {
		return fmt.Errorf("create outbound email: %w", err)
	}
	bccEmails, err := marshalJSONString(email.BCCEmails, "bcc_emails")
	if err != nil {
		return fmt.Errorf("create outbound email: %w", err)
	}
	templateData, err := marshalJSONString(email.TemplateData, "template_data")
	if err != nil {
		return fmt.Errorf("create outbound email: %w", err)
	}
	providerSettings, err := marshalJSONString(email.ProviderSettings, "provider_settings")
	if err != nil {
		return fmt.Errorf("create outbound email: %w", err)
	}
	tags, err := marshalJSONString(email.Tags, "tags")
	if err != nil {
		return fmt.Errorf("create outbound email: %w", err)
	}
	attachments, err := marshalJSONString(email.AttachmentIDs, "attachment_ids")
	if err != nil {
		return fmt.Errorf("create outbound email: %w", err)
	}

	query := `INSERT INTO core_service.outbound_emails (
		id, workspace_id, from_email, from_name, to_emails, cc_emails, bcc_emails,
		reply_to_email, subject, html_content, text_content, template_id, template_data,
		provider, provider_settings, status, scheduled_for, sent_at, delivered_at,
		provider_message_id, provider_response, error_message, retry_count, max_retries,
		next_retry_at, opened_at, open_count, click_count, last_click_at, case_id,
		contact_id, communication_id, user_id, category, tags, attachment_ids,
		created_by_id, created_at, updated_at
	) VALUES (
		COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?
	)
	RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		email.ID, email.WorkspaceID, email.FromEmail, email.FromName, toEmails,
		ccEmails, bccEmails, email.ReplyToEmail, email.Subject, email.HTMLContent,
		email.TextContent, nullableUUIDValue(email.TemplateID), templateData, string(email.Provider),
		providerSettings, string(email.Status), email.ScheduledFor, email.SentAt,
		email.DeliveredAt, email.ProviderMessageID, email.ProviderResponse,
		email.ErrorMessage, email.RetryCount, email.MaxRetries, email.NextRetryAt,
		email.OpenedAt, email.OpenCount, email.ClickCount, email.LastClickAt,
		nullableUUIDValue(email.CaseID), nullableUUIDValue(email.ContactID),
		nullableUUIDValue(email.CommunicationID), nullableUUIDValue(email.UserID),
		email.Category, tags, attachments, nullableUUIDValue(email.CreatedByID), email.CreatedAt,
		email.UpdatedAt,
	).Scan(&email.ID)
	return TranslateSqlxError(err, "outbound_emails")
}

func (s *EmailStore) GetOutboundEmail(ctx context.Context, emailID string) (*servicedomain.OutboundEmail, error) {
	var dbEmail models.OutboundEmail
	query := `SELECT * FROM core_service.outbound_emails WHERE id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &dbEmail, query, emailID)
	if err != nil {
		return nil, TranslateSqlxError(err, "outbound_emails")
	}
	return s.mapOutboundToDomain(&dbEmail), nil
}

func (s *EmailStore) GetOutboundEmailByProviderMessageID(ctx context.Context, providerMessageID string) (*servicedomain.OutboundEmail, error) {
	var dbEmail models.OutboundEmail
	query := `SELECT * FROM core_service.outbound_emails WHERE provider_message_id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &dbEmail, query, providerMessageID)
	if err != nil {
		return nil, TranslateSqlxError(err, "outbound_emails")
	}
	return s.mapOutboundToDomain(&dbEmail), nil
}

func (s *EmailStore) UpdateOutboundEmail(ctx context.Context, email *servicedomain.OutboundEmail) error {
	toEmails, err := marshalJSONString(email.ToEmails, "to_emails")
	if err != nil {
		return fmt.Errorf("update outbound email: %w", err)
	}
	ccEmails, err := marshalJSONString(email.CCEmails, "cc_emails")
	if err != nil {
		return fmt.Errorf("update outbound email: %w", err)
	}
	bccEmails, err := marshalJSONString(email.BCCEmails, "bcc_emails")
	if err != nil {
		return fmt.Errorf("update outbound email: %w", err)
	}
	templateData, err := marshalJSONString(email.TemplateData, "template_data")
	if err != nil {
		return fmt.Errorf("update outbound email: %w", err)
	}
	providerSettings, err := marshalJSONString(email.ProviderSettings, "provider_settings")
	if err != nil {
		return fmt.Errorf("update outbound email: %w", err)
	}
	tags, err := marshalJSONString(email.Tags, "tags")
	if err != nil {
		return fmt.Errorf("update outbound email: %w", err)
	}
	attachments, err := marshalJSONString(email.AttachmentIDs, "attachment_ids")
	if err != nil {
		return fmt.Errorf("update outbound email: %w", err)
	}

	query := `UPDATE core_service.outbound_emails SET
		workspace_id = ?, from_email = ?, from_name = ?, to_emails = ?,
		cc_emails = ?, bcc_emails = ?, reply_to_email = ?, subject = ?,
		html_content = ?, text_content = ?, template_id = ?, template_data = ?,
		provider = ?, provider_settings = ?, status = ?, scheduled_for = ?,
		sent_at = ?, delivered_at = ?, provider_message_id = ?, provider_response = ?,
		error_message = ?, retry_count = ?, max_retries = ?, next_retry_at = ?,
		opened_at = ?, open_count = ?, click_count = ?, last_click_at = ?,
		case_id = ?, contact_id = ?, communication_id = ?, user_id = ?,
		category = ?, tags = ?, attachment_ids = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`

	_, err = s.db.Get(ctx).ExecContext(ctx, query,
		email.WorkspaceID, email.FromEmail, email.FromName, toEmails,
		ccEmails, bccEmails, email.ReplyToEmail, email.Subject, email.HTMLContent,
		email.TextContent, nullableUUIDValue(email.TemplateID), templateData, string(email.Provider),
		providerSettings, string(email.Status), email.ScheduledFor, email.SentAt,
		email.DeliveredAt, email.ProviderMessageID, email.ProviderResponse,
		email.ErrorMessage, email.RetryCount, email.MaxRetries, email.NextRetryAt,
		email.OpenedAt, email.OpenCount, email.ClickCount, email.LastClickAt,
		nullableUUIDValue(email.CaseID), nullableUUIDValue(email.ContactID),
		nullableUUIDValue(email.CommunicationID), nullableUUIDValue(email.UserID),
		email.Category, tags, attachments, time.Now(),
		email.ID, email.WorkspaceID,
	)
	return TranslateSqlxError(err, "outbound_emails")
}

func (s *EmailStore) mapOutboundToDomain(e *models.OutboundEmail) *servicedomain.OutboundEmail {
	var toEmails, ccEmails, bccEmails, tags, attachmentIDs []string
	var templateData, providerSettings map[string]interface{}

	unmarshalJSONField(e.ToEmails, &toEmails, "outbound_emails", "to_emails")
	unmarshalJSONField(e.CCEmails, &ccEmails, "outbound_emails", "cc_emails")
	unmarshalJSONField(e.BCCEmails, &bccEmails, "outbound_emails", "bcc_emails")
	unmarshalJSONField(e.Tags, &tags, "outbound_emails", "tags")
	unmarshalJSONField(e.AttachmentIDs, &attachmentIDs, "outbound_emails", "attachment_ids")
	unmarshalJSONField(e.TemplateData, &templateData, "outbound_emails", "template_data")
	unmarshalJSONField(e.ProviderSettings, &providerSettings, "outbound_emails", "provider_settings")

	return &servicedomain.OutboundEmail{
		ID:                e.ID,
		WorkspaceID:       e.WorkspaceID,
		FromEmail:         e.FromEmail,
		FromName:          e.FromName,
		ToEmails:          toEmails,
		CCEmails:          ccEmails,
		BCCEmails:         bccEmails,
		ReplyToEmail:      e.ReplyToEmail,
		Subject:           e.Subject,
		HTMLContent:       e.HTMLContent,
		TextContent:       e.TextContent,
		TemplateID:        derefStringPtr(e.TemplateID),
		TemplateData:      templateData,
		Provider:          servicedomain.EmailProvider(e.Provider),
		ProviderSettings:  providerSettings,
		Status:            servicedomain.EmailStatus(e.Status),
		ScheduledFor:      e.ScheduledFor,
		SentAt:            e.SentAt,
		DeliveredAt:       e.DeliveredAt,
		ProviderMessageID: e.ProviderMessageID,
		ProviderResponse:  e.ProviderResponse,
		ErrorMessage:      e.ErrorMessage,
		RetryCount:        e.RetryCount,
		MaxRetries:        e.MaxRetries,
		NextRetryAt:       e.NextRetryAt,
		OpenedAt:          e.OpenedAt,
		OpenCount:         e.OpenCount,
		ClickCount:        e.ClickCount,
		LastClickAt:       e.LastClickAt,
		CaseID:            derefStringPtr(e.CaseID),
		ContactID:         derefStringPtr(e.ContactID),
		CommunicationID:   derefStringPtr(e.CommunicationID),
		UserID:            derefStringPtr(e.UserID),
		Category:          e.Category,
		Tags:              tags,
		AttachmentIDs:     attachmentIDs,
		CreatedByID:       derefStringPtr(e.CreatedByID),
		CreatedAt:         e.CreatedAt,
		UpdatedAt:         e.UpdatedAt,
	}
}
