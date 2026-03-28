package sql

import (
	"context"
	"fmt"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

func (s *CaseStore) CreateCommunication(ctx context.Context, comm *servicedomain.Communication) error {
	toEmails, err := marshalJSONString(comm.ToEmails, "to_emails")
	if err != nil {
		return fmt.Errorf("CreateCommunication: %w", err)
	}
	ccEmails, err := marshalJSONString(comm.CCEmails, "cc_emails")
	if err != nil {
		return fmt.Errorf("CreateCommunication: %w", err)
	}
	bccEmails, err := marshalJSONString(comm.BCCEmails, "bcc_emails")
	if err != nil {
		return fmt.Errorf("CreateCommunication: %w", err)
	}
	emailReferences, err := marshalJSONString(comm.References, "email_references")
	if err != nil {
		return fmt.Errorf("CreateCommunication: %w", err)
	}
	attachments, err := marshalJSONString(comm.AttachmentIDs, "attachment_ids")
	if err != nil {
		return fmt.Errorf("CreateCommunication: %w", err)
	}

	normalizePersistedUUID(&comm.ID)
	query := `INSERT INTO core_service.communications (
		id, case_id, workspace_id, type, direction, subject, body, body_html,
		from_email, from_name, from_user_id, to_emails, cc_emails, bcc_emails,
		message_id, in_reply_to, email_references, attachment_ids,
		is_internal, is_read, is_spam, created_at, updated_at
	) VALUES (
		COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?
	) RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		comm.ID, comm.CaseID, comm.WorkspaceID, string(comm.Type), string(comm.Direction),
		comm.Subject, comm.Body, comm.BodyHTML, comm.FromEmail, comm.FromName,
		nullableUUIDValue(comm.FromUserID), toEmails, ccEmails, bccEmails, comm.MessageID,
		comm.InReplyTo, emailReferences, attachments, comm.IsInternal, comm.IsRead,
		comm.IsSpam, comm.CreatedAt, comm.UpdatedAt,
	).Scan(&comm.ID)
	return TranslateSqlxError(err, "communications")
}

func (s *CaseStore) GetCommunication(ctx context.Context, workspaceID, commID string) (*servicedomain.Communication, error) {
	var dbComm models.Communication
	query := `SELECT * FROM core_service.communications WHERE id = ? AND workspace_id = ?`
	err := s.db.Get(ctx).GetContext(ctx, &dbComm, query, commID, workspaceID)
	if err != nil {
		return nil, TranslateSqlxError(err, "communications")
	}
	return s.mapCommunicationToDomain(&dbComm), nil
}

func (s *CaseStore) UpdateCommunication(ctx context.Context, comm *servicedomain.Communication) error {
	toEmails, err := marshalJSONString(comm.ToEmails, "to_emails")
	if err != nil {
		return fmt.Errorf("UpdateCommunication: %w", err)
	}
	ccEmails, err := marshalJSONString(comm.CCEmails, "cc_emails")
	if err != nil {
		return fmt.Errorf("UpdateCommunication: %w", err)
	}
	bccEmails, err := marshalJSONString(comm.BCCEmails, "bcc_emails")
	if err != nil {
		return fmt.Errorf("UpdateCommunication: %w", err)
	}
	emailReferences, err := marshalJSONString(comm.References, "email_references")
	if err != nil {
		return fmt.Errorf("UpdateCommunication: %w", err)
	}
	attachments, err := marshalJSONString(comm.AttachmentIDs, "attachment_ids")
	if err != nil {
		return fmt.Errorf("UpdateCommunication: %w", err)
	}

	query := `UPDATE core_service.communications SET
		type = ?, direction = ?, subject = ?, body = ?, body_html = ?,
		from_email = ?, from_name = ?, from_user_id = ?, to_emails = ?,
		cc_emails = ?, bcc_emails = ?, message_id = ?, in_reply_to = ?,
		email_references = ?, attachment_ids = ?, is_internal = ?, is_read = ?,
		is_spam = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ?`

	_, err = s.db.Get(ctx).ExecContext(ctx, query,
		string(comm.Type), string(comm.Direction), comm.Subject, comm.Body, comm.BodyHTML,
		comm.FromEmail, comm.FromName, nullableUUIDValue(comm.FromUserID), toEmails,
		ccEmails, bccEmails, comm.MessageID, comm.InReplyTo, emailReferences,
		attachments, comm.IsInternal, comm.IsRead, comm.IsSpam, comm.UpdatedAt,
		comm.ID, comm.WorkspaceID,
	)
	return TranslateSqlxError(err, "communications")
}

func (s *CaseStore) mapCommunicationToDomain(c *models.Communication) *servicedomain.Communication {
	var toEmails, ccEmails, bccEmails, references, attachmentIDs []string
	unmarshalJSONField(c.ToEmails, &toEmails, "communications", "to_emails")
	unmarshalJSONField(c.CCEmails, &ccEmails, "communications", "cc_emails")
	unmarshalJSONField(c.BCCEmails, &bccEmails, "communications", "bcc_emails")
	unmarshalJSONField(c.EmailReferences, &references, "communications", "email_references")
	unmarshalJSONField(c.AttachmentIDs, &attachmentIDs, "communications", "attachment_ids")

	var fromAgentID string
	if c.FromAgentID != nil {
		fromAgentID = *c.FromAgentID
	}

	return &servicedomain.Communication{
		ID:            c.ID,
		CaseID:        c.CaseID,
		WorkspaceID:   c.WorkspaceID,
		Type:          shareddomain.CommunicationType(c.Type),
		Direction:     shareddomain.Direction(c.Direction),
		Subject:       c.Subject,
		Body:          c.Body,
		BodyHTML:      c.BodyHTML,
		FromEmail:     c.FromEmail,
		FromName:      c.FromName,
		FromUserID:    derefStringPtr(c.FromUserID),
		FromAgentID:   fromAgentID,
		ToEmails:      toEmails,
		CCEmails:      ccEmails,
		BCCEmails:     bccEmails,
		MessageID:     c.MessageID,
		InReplyTo:     c.InReplyTo,
		References:    references,
		AttachmentIDs: attachmentIDs,
		IsInternal:    c.IsInternal,
		IsRead:        c.IsRead,
		IsSpam:        c.IsSpam,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

func (s *CaseStore) ListCaseCommunications(ctx context.Context, caseID string) ([]*servicedomain.Communication, error) {
	var dbComms []models.Communication
	query := `SELECT * FROM core_service.communications WHERE case_id = ? ORDER BY created_at ASC`
	err := s.db.Get(ctx).SelectContext(ctx, &dbComms, query, caseID)
	if err != nil {
		return nil, fmt.Errorf("list case communications: %w", err)
	}

	comms := make([]*servicedomain.Communication, len(dbComms))
	for i, c := range dbComms {
		comms[i] = s.mapCommunicationToDomain(&c)
	}
	return comms, nil
}

func (s *CaseStore) ListCommunications(ctx context.Context, workspaceID, caseID string) ([]*servicedomain.Communication, error) {
	var dbComms []models.Communication
	query := `SELECT * FROM core_service.communications WHERE workspace_id = ? AND case_id = ? ORDER BY created_at ASC`
	err := s.db.Get(ctx).SelectContext(ctx, &dbComms, query, workspaceID, caseID)
	if err != nil {
		return nil, fmt.Errorf("list communications: %w", err)
	}

	comms := make([]*servicedomain.Communication, len(dbComms))
	for i, c := range dbComms {
		comms[i] = s.mapCommunicationToDomain(&c)
	}
	return comms, nil
}
