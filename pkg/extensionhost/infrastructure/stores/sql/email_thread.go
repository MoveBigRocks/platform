package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

// =============================================================================
// Email Thread Operations
// =============================================================================

func (s *EmailStore) CreateEmailThread(ctx context.Context, thread *servicedomain.EmailThread) error {
	normalizePersistedUUID(&thread.ID)
	participants, err := marshalJSONString(thread.Participants, "participants")
	if err != nil {
		return fmt.Errorf("create email thread: %w", err)
	}
	contactIDs, err := marshalJSONString(thread.ContactIDs, "contact_ids")
	if err != nil {
		return fmt.Errorf("create email thread: %w", err)
	}
	messageIDs, err := marshalJSONString(thread.MessageIDs, "message_ids")
	if err != nil {
		return fmt.Errorf("create email thread: %w", err)
	}
	childThreadIDs, err := marshalJSONString(thread.ChildThreadIDs, "child_thread_ids")
	if err != nil {
		return fmt.Errorf("create email thread: %w", err)
	}
	mergedFromIDs, err := marshalJSONString(thread.MergedFromIDs, "merged_from_ids")
	if err != nil {
		return fmt.Errorf("create email thread: %w", err)
	}
	tags, err := marshalJSONString(thread.Tags, "tags")
	if err != nil {
		return fmt.Errorf("create email thread: %w", err)
	}
	labels, err := marshalJSONString(thread.Labels, "labels")
	if err != nil {
		return fmt.Errorf("create email thread: %w", err)
	}
	customFields, err := marshalJSONString(thread.CustomFields, "custom_fields")
	if err != nil {
		return fmt.Errorf("create email thread: %w", err)
	}

	query := `INSERT INTO core_service.email_threads (
		id, workspace_id, thread_key, subject, type, status, priority, participants,
		case_id, contact_ids, email_count, unread_count, last_email_id, first_email_id,
		message_ids, first_email_at, last_email_at, last_activity, sentiment_score,
		is_important, has_attachments, attachment_count, parent_thread_id,
		child_thread_ids, merged_from_ids, merged_into_id, detected_by,
		detection_method, detection_score, tags, labels, is_spam, spam_score,
		is_quarantined, is_watched, is_muted, is_archived, notes, custom_fields,
		created_at, updated_at
	) VALUES (
		COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	)
	RETURNING id`

	firstEmailAt := &thread.FirstEmailAt
	lastEmailAt := &thread.LastEmailAt
	lastActivity := &thread.LastActivity

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		thread.ID, thread.WorkspaceID, thread.ThreadKey, thread.Subject,
		string(thread.Type), string(thread.Status), string(thread.Priority),
		participants, nullableUUIDValue(thread.CaseID), contactIDs, thread.EmailCount,
		thread.UnreadCount, nullableUUIDValue(thread.LastEmailID), nullableUUIDValue(thread.FirstEmailID), messageIDs,
		firstEmailAt, lastEmailAt, lastActivity, thread.SentimentScore,
		thread.IsImportant, thread.HasAttachments, thread.AttachmentCount,
		nullableUUIDValue(thread.ParentThreadID), childThreadIDs, mergedFromIDs, nullableUUIDValue(thread.MergedIntoID),
		thread.DetectedBy, thread.DetectionMethod, thread.DetectionScore, tags,
		labels, thread.IsSpam, thread.SpamScore, thread.IsQuarantined,
		thread.IsWatched, thread.IsMuted, thread.IsArchived, thread.Notes,
		customFields, thread.CreatedAt, thread.UpdatedAt,
	).Scan(&thread.ID)
	return TranslateSqlxError(err, "email_threads")
}

func (s *EmailStore) GetEmailThread(ctx context.Context, threadID string) (*servicedomain.EmailThread, error) {
	var dbThread models.EmailThread
	query := `SELECT * FROM core_service.email_threads WHERE id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &dbThread, query, threadID)
	if err != nil {
		return nil, TranslateSqlxError(err, "email_threads")
	}
	return s.mapThreadToDomain(&dbThread), nil
}

func (s *EmailStore) UpdateEmailThread(ctx context.Context, thread *servicedomain.EmailThread) error {
	participants, err := marshalJSONString(thread.Participants, "participants")
	if err != nil {
		return fmt.Errorf("update email thread: %w", err)
	}
	contactIDs, err := marshalJSONString(thread.ContactIDs, "contact_ids")
	if err != nil {
		return fmt.Errorf("update email thread: %w", err)
	}
	messageIDs, err := marshalJSONString(thread.MessageIDs, "message_ids")
	if err != nil {
		return fmt.Errorf("update email thread: %w", err)
	}
	childThreadIDs, err := marshalJSONString(thread.ChildThreadIDs, "child_thread_ids")
	if err != nil {
		return fmt.Errorf("update email thread: %w", err)
	}
	mergedFromIDs, err := marshalJSONString(thread.MergedFromIDs, "merged_from_ids")
	if err != nil {
		return fmt.Errorf("update email thread: %w", err)
	}
	tags, err := marshalJSONString(thread.Tags, "tags")
	if err != nil {
		return fmt.Errorf("update email thread: %w", err)
	}
	labels, err := marshalJSONString(thread.Labels, "labels")
	if err != nil {
		return fmt.Errorf("update email thread: %w", err)
	}
	customFields, err := marshalJSONString(thread.CustomFields, "custom_fields")
	if err != nil {
		return fmt.Errorf("update email thread: %w", err)
	}

	query := `UPDATE core_service.email_threads SET
		workspace_id = ?, thread_key = ?, subject = ?, type = ?, status = ?,
		priority = ?, participants = ?, case_id = ?, contact_ids = ?,
		email_count = ?, unread_count = ?, last_email_id = ?,
		first_email_id = ?, message_ids = ?, first_email_at = ?,
		last_email_at = ?, last_activity = ?, sentiment_score = ?,
		is_important = ?, has_attachments = ?, attachment_count = ?,
		parent_thread_id = ?, child_thread_ids = ?, merged_from_ids = ?,
		merged_into_id = ?, detected_by = ?, detection_method = ?,
		detection_score = ?, tags = ?, labels = ?, is_spam = ?,
		spam_score = ?, is_quarantined = ?, is_watched = ?, is_muted = ?,
		is_archived = ?, notes = ?, custom_fields = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`

	firstEmailAt := &thread.FirstEmailAt
	lastEmailAt := &thread.LastEmailAt
	lastActivity := &thread.LastActivity

	_, err = s.db.Get(ctx).ExecContext(ctx, query,
		thread.WorkspaceID, thread.ThreadKey, thread.Subject,
		string(thread.Type), string(thread.Status), string(thread.Priority),
		participants, nullableUUIDValue(thread.CaseID), contactIDs, thread.EmailCount,
		thread.UnreadCount, nullableUUIDValue(thread.LastEmailID), nullableUUIDValue(thread.FirstEmailID), messageIDs,
		firstEmailAt, lastEmailAt, lastActivity, thread.SentimentScore,
		thread.IsImportant, thread.HasAttachments, thread.AttachmentCount,
		nullableUUIDValue(thread.ParentThreadID), childThreadIDs, mergedFromIDs, nullableUUIDValue(thread.MergedIntoID),
		thread.DetectedBy, thread.DetectionMethod, thread.DetectionScore, tags,
		labels, thread.IsSpam, thread.SpamScore, thread.IsQuarantined,
		thread.IsWatched, thread.IsMuted, thread.IsArchived, thread.Notes,
		customFields, time.Now(),
		thread.ID, thread.WorkspaceID,
	)
	return TranslateSqlxError(err, "email_threads")
}

func (s *EmailStore) mapThreadToDomain(t *models.EmailThread) *servicedomain.EmailThread {
	var participants []servicedomain.ThreadParticipant
	var contactIDs, messageIDs, childThreadIDs, mergedFromIDs, tags, labels []string
	var customFields map[string]interface{}

	unmarshalJSONField(t.Participants, &participants, "email_threads", "participants")
	unmarshalJSONField(t.ContactIDs, &contactIDs, "email_threads", "contact_ids")
	unmarshalJSONField(t.MessageIDs, &messageIDs, "email_threads", "message_ids")
	unmarshalJSONField(t.ChildThreadIDs, &childThreadIDs, "email_threads", "child_thread_ids")
	unmarshalJSONField(t.MergedFromIDs, &mergedFromIDs, "email_threads", "merged_from_ids")
	unmarshalJSONField(t.Tags, &tags, "email_threads", "tags")
	unmarshalJSONField(t.Labels, &labels, "email_threads", "labels")
	unmarshalJSONField(t.CustomFields, &customFields, "email_threads", "custom_fields")

	thread := &servicedomain.EmailThread{
		ID:              t.ID,
		WorkspaceID:     t.WorkspaceID,
		ThreadKey:       t.ThreadKey,
		Subject:         t.Subject,
		Type:            servicedomain.ThreadType(t.Type),
		Status:          servicedomain.ThreadStatus(t.Status),
		Priority:        servicedomain.ThreadPriority(t.Priority),
		Participants:    participants,
		CaseID:          derefStringPtr(t.CaseID),
		ContactIDs:      contactIDs,
		EmailCount:      t.EmailCount,
		UnreadCount:     t.UnreadCount,
		LastEmailID:     derefStringPtr(t.LastEmailID),
		FirstEmailID:    derefStringPtr(t.FirstEmailID),
		MessageIDs:      messageIDs,
		SentimentScore:  t.SentimentScore,
		IsImportant:     t.IsImportant,
		HasAttachments:  t.HasAttachments,
		AttachmentCount: t.AttachmentCount,
		ParentThreadID:  derefStringPtr(t.ParentThreadID),
		ChildThreadIDs:  childThreadIDs,
		MergedFromIDs:   mergedFromIDs,
		MergedIntoID:    derefStringPtr(t.MergedIntoID),
		DetectedBy:      t.DetectedBy,
		DetectionMethod: t.DetectionMethod,
		DetectionScore:  t.DetectionScore,
		Tags:            tags,
		Labels:          labels,
		IsSpam:          t.IsSpam,
		SpamScore:       t.SpamScore,
		IsQuarantined:   t.IsQuarantined,
		IsWatched:       t.IsWatched,
		IsMuted:         t.IsMuted,
		IsArchived:      t.IsArchived,
		Notes:           t.Notes,
		CustomFields:    customFields,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
	if t.FirstEmailAt != nil {
		thread.FirstEmailAt = *t.FirstEmailAt
	}
	if t.LastEmailAt != nil {
		thread.LastEmailAt = *t.LastEmailAt
	}
	if t.LastActivity != nil {
		thread.LastActivity = *t.LastActivity
	}
	return thread
}
