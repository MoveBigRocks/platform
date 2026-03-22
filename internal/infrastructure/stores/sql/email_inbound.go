package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

// =============================================================================
// Inbound Email Operations
// =============================================================================

func (s *EmailStore) CreateInboundEmail(ctx context.Context, email *servicedomain.InboundEmail) error {
	normalizePersistedUUID(&email.ID)
	toEmails, err := marshalJSONString(email.ToEmails, "to_emails")
	if err != nil {
		return fmt.Errorf("create inbound email: %w", err)
	}
	ccEmails, err := marshalJSONString(email.CCEmails, "cc_emails")
	if err != nil {
		return fmt.Errorf("create inbound email: %w", err)
	}
	bccEmails, err := marshalJSONString(email.BCCEmails, "bcc_emails")
	if err != nil {
		return fmt.Errorf("create inbound email: %w", err)
	}
	emailReferences, err := marshalJSONString(email.References, "email_references")
	if err != nil {
		return fmt.Errorf("create inbound email: %w", err)
	}
	spamReasons, err := marshalJSONString(email.SpamReasons, "spam_reasons")
	if err != nil {
		return fmt.Errorf("create inbound email: %w", err)
	}
	previousEmailIDs, err := marshalJSONString(email.PreviousEmailIDs, "previous_email_ids")
	if err != nil {
		return fmt.Errorf("create inbound email: %w", err)
	}
	tags, err := marshalJSONString(email.Tags, "tags")
	if err != nil {
		return fmt.Errorf("create inbound email: %w", err)
	}
	attachmentIDs, err := marshalJSONString(email.AttachmentIDs, "attachment_ids")
	if err != nil {
		return fmt.Errorf("create inbound email: %w", err)
	}
	headers, err := marshalJSONString(email.Headers, "headers")
	if err != nil {
		return fmt.Errorf("create inbound email: %w", err)
	}

	query := `INSERT INTO core_service.inbound_emails (
		id, workspace_id, message_id, in_reply_to, email_references, from_email,
		from_name, to_emails, cc_emails, bcc_emails, subject, html_content,
		text_content, processing_status, processed_at, processing_error, spam_score,
		spam_reasons, is_spam, case_id, contact_id, communication_id, thread_id,
		is_thread_start, previous_email_ids, is_loop, loop_score, is_bounce,
		bounce_type, original_message_id, is_auto_response, auto_response_type,
		is_read, tags, attachment_ids, attachment_count, total_attachment_size,
		raw_content, headers, received_at, created_at, updated_at
	) VALUES (
		COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	)
	RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		email.ID, email.WorkspaceID, email.MessageID, email.InReplyTo, emailReferences,
		email.FromEmail, email.FromName, toEmails, ccEmails, bccEmails, email.Subject,
		email.HTMLContent, email.TextContent, string(email.ProcessingStatus),
		email.ProcessedAt, email.ProcessingError, email.SpamScore, spamReasons,
		email.IsSpam, nullableUUIDValue(email.CaseID), nullableUUIDValue(email.ContactID),
		nullableUUIDValue(email.CommunicationID), nullableUUIDValue(email.ThreadID),
		email.IsThreadStart, previousEmailIDs, email.IsLoop,
		email.LoopScore, email.IsBounce, email.BounceType, email.OriginalMessageID,
		email.IsAutoResponse, email.AutoResponseType, email.IsRead, tags,
		attachmentIDs, email.AttachmentCount, email.TotalAttachmentSize,
		email.RawContent, headers, email.ReceivedAt, email.CreatedAt, email.UpdatedAt,
	).Scan(&email.ID)
	return TranslateSqlxError(err, "inbound_emails")
}

func (s *EmailStore) GetInboundEmail(ctx context.Context, emailID string) (*servicedomain.InboundEmail, error) {
	var dbEmail models.InboundEmail
	query := `SELECT * FROM core_service.inbound_emails WHERE id = ?`
	err := s.db.Get(ctx).GetContext(ctx, &dbEmail, query, emailID)
	if err != nil {
		return nil, TranslateSqlxError(err, "inbound_emails")
	}
	return s.mapInboundToDomain(&dbEmail), nil
}

func (s *EmailStore) UpdateInboundEmail(ctx context.Context, email *servicedomain.InboundEmail) error {
	toEmails, err := marshalJSONString(email.ToEmails, "to_emails")
	if err != nil {
		return fmt.Errorf("update inbound email: %w", err)
	}
	ccEmails, err := marshalJSONString(email.CCEmails, "cc_emails")
	if err != nil {
		return fmt.Errorf("update inbound email: %w", err)
	}
	bccEmails, err := marshalJSONString(email.BCCEmails, "bcc_emails")
	if err != nil {
		return fmt.Errorf("update inbound email: %w", err)
	}
	emailReferences, err := marshalJSONString(email.References, "email_references")
	if err != nil {
		return fmt.Errorf("update inbound email: %w", err)
	}
	spamReasons, err := marshalJSONString(email.SpamReasons, "spam_reasons")
	if err != nil {
		return fmt.Errorf("update inbound email: %w", err)
	}
	previousEmailIDs, err := marshalJSONString(email.PreviousEmailIDs, "previous_email_ids")
	if err != nil {
		return fmt.Errorf("update inbound email: %w", err)
	}
	tags, err := marshalJSONString(email.Tags, "tags")
	if err != nil {
		return fmt.Errorf("update inbound email: %w", err)
	}
	attachmentIDs, err := marshalJSONString(email.AttachmentIDs, "attachment_ids")
	if err != nil {
		return fmt.Errorf("update inbound email: %w", err)
	}
	headers, err := marshalJSONString(email.Headers, "headers")
	if err != nil {
		return fmt.Errorf("update inbound email: %w", err)
	}

	query := `UPDATE core_service.inbound_emails SET
		workspace_id = ?, message_id = ?, in_reply_to = ?, email_references = ?,
		from_email = ?, from_name = ?, to_emails = ?, cc_emails = ?, bcc_emails = ?,
		subject = ?, html_content = ?, text_content = ?, processing_status = ?,
		processed_at = ?, processing_error = ?, spam_score = ?, spam_reasons = ?,
		is_spam = ?, case_id = ?, contact_id = ?, communication_id = ?,
		thread_id = ?, is_thread_start = ?, previous_email_ids = ?, is_loop = ?,
		loop_score = ?, is_bounce = ?, bounce_type = ?, original_message_id = ?,
		is_auto_response = ?, auto_response_type = ?, is_read = ?, tags = ?,
		attachment_ids = ?, attachment_count = ?, total_attachment_size = ?,
		raw_content = ?, headers = ?, received_at = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ?`

	_, err = s.db.Get(ctx).ExecContext(ctx, query,
		email.WorkspaceID, email.MessageID, email.InReplyTo, emailReferences,
		email.FromEmail, email.FromName, toEmails, ccEmails, bccEmails, email.Subject,
		email.HTMLContent, email.TextContent, string(email.ProcessingStatus),
		email.ProcessedAt, email.ProcessingError, email.SpamScore, spamReasons,
		email.IsSpam, nullableUUIDValue(email.CaseID), nullableUUIDValue(email.ContactID),
		nullableUUIDValue(email.CommunicationID), nullableUUIDValue(email.ThreadID),
		email.IsThreadStart, previousEmailIDs, email.IsLoop,
		email.LoopScore, email.IsBounce, email.BounceType, email.OriginalMessageID,
		email.IsAutoResponse, email.AutoResponseType, email.IsRead, tags,
		attachmentIDs, email.AttachmentCount, email.TotalAttachmentSize,
		email.RawContent, headers, email.ReceivedAt, time.Now(),
		email.ID, email.WorkspaceID,
	)
	return TranslateSqlxError(err, "inbound_emails")
}

func (s *EmailStore) GetEmailsByThread(ctx context.Context, threadID string) ([]*servicedomain.InboundEmail, error) {
	var dbModels []models.InboundEmail
	query := `SELECT * FROM core_service.inbound_emails WHERE thread_id = ? ORDER BY received_at ASC`
	err := s.db.Get(ctx).SelectContext(ctx, &dbModels, query, threadID)
	if err != nil {
		return nil, TranslateSqlxError(err, "inbound_emails")
	}

	result := make([]*servicedomain.InboundEmail, len(dbModels))
	for i, m := range dbModels {
		result[i] = s.mapInboundToDomain(&m)
	}
	return result, nil
}

func (s *EmailStore) mapInboundToDomain(e *models.InboundEmail) *servicedomain.InboundEmail {
	var toEmails, ccEmails, bccEmails, references, spamReasons, previousEmailIDs, tags, attachmentIDs []string
	var headers map[string]string

	unmarshalJSONField(e.ToEmails, &toEmails, "inbound_emails", "to_emails")
	unmarshalJSONField(e.CCEmails, &ccEmails, "inbound_emails", "cc_emails")
	unmarshalJSONField(e.BCCEmails, &bccEmails, "inbound_emails", "bcc_emails")
	unmarshalJSONField(e.EmailReferences, &references, "inbound_emails", "email_references")
	unmarshalJSONField(e.SpamReasons, &spamReasons, "inbound_emails", "spam_reasons")
	unmarshalJSONField(e.PreviousEmailIDs, &previousEmailIDs, "inbound_emails", "previous_email_ids")
	unmarshalJSONField(e.Tags, &tags, "inbound_emails", "tags")
	unmarshalJSONField(e.AttachmentIDs, &attachmentIDs, "inbound_emails", "attachment_ids")
	unmarshalJSONField(e.Headers, &headers, "inbound_emails", "headers")

	return &servicedomain.InboundEmail{
		ID:                  e.ID,
		WorkspaceID:         e.WorkspaceID,
		MessageID:           e.MessageID,
		InReplyTo:           e.InReplyTo,
		References:          references,
		FromEmail:           e.FromEmail,
		FromName:            e.FromName,
		ToEmails:            toEmails,
		CCEmails:            ccEmails,
		BCCEmails:           bccEmails,
		Subject:             e.Subject,
		HTMLContent:         e.HTMLContent,
		TextContent:         e.TextContent,
		ProcessingStatus:    servicedomain.EmailProcessingStatus(e.ProcessingStatus),
		ProcessedAt:         e.ProcessedAt,
		ProcessingError:     e.ProcessingError,
		SpamScore:           e.SpamScore,
		SpamReasons:         spamReasons,
		IsSpam:              e.IsSpam,
		CaseID:              derefStringPtr(e.CaseID),
		ContactID:           derefStringPtr(e.ContactID),
		CommunicationID:     derefStringPtr(e.CommunicationID),
		ThreadID:            derefStringPtr(e.ThreadID),
		IsThreadStart:       e.IsThreadStart,
		PreviousEmailIDs:    previousEmailIDs,
		IsLoop:              e.IsLoop,
		LoopScore:           e.LoopScore,
		IsBounce:            e.IsBounce,
		BounceType:          e.BounceType,
		OriginalMessageID:   e.OriginalMessageID,
		IsAutoResponse:      e.IsAutoResponse,
		AutoResponseType:    e.AutoResponseType,
		IsRead:              e.IsRead,
		Tags:                tags,
		AttachmentIDs:       attachmentIDs,
		AttachmentCount:     e.AttachmentCount,
		TotalAttachmentSize: e.TotalAttachmentSize,
		RawContent:          e.RawContent,
		Headers:             headers,
		ReceivedAt:          e.ReceivedAt,
		CreatedAt:           e.CreatedAt,
		UpdatedAt:           e.UpdatedAt,
	}
}
