package sql

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

type NotificationStore struct {
	db *SqlxDB
}

func NewNotificationStore(db *SqlxDB) *NotificationStore {
	return &NotificationStore{db: db}
}

func (s *NotificationStore) CreateNotification(ctx context.Context, notification *shareddomain.Notification) error {
	if notification == nil {
		return fmt.Errorf("notification is required")
	}

	deliveryMethods, err := marshalJSONString([]string{string(notification.Type)}, "delivery_methods")
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	metadata := string(notification.TrackingData)
	if len(notification.TrackingData) == 0 {
		metadata = "{}"
	} else if !json.Valid(notification.TrackingData) {
		return fmt.Errorf("create notification: tracking_data must be valid JSON")
	}

	query := `INSERT INTO core_identity.notifications (
		id, workspace_id, user_id, type, title, body, icon_url, target_type, target_id,
		action_url, action_label, is_read, read_at, is_archived, archived_at, priority,
		delivery_methods, email_sent_at, push_sent_at, sms_sent_at, expires_at, metadata,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.Get(ctx).ExecContext(ctx, query,
		notification.ID,
		notification.WorkspaceID,
		notification.RecipientID,
		string(notification.Type),
		notification.Subject,
		notification.Body,
		notification.ImageURL,
		notification.EntityType,
		notification.EntityID,
		notification.ActionURL,
		"",
		notification.Status == shareddomain.NotificationStatusRead,
		notification.ReadAt,
		false,
		nil,
		string(notification.Priority),
		deliveryMethods,
		notification.SentAt,
		nil,
		nil,
		notification.ExpiresAt,
		metadata,
		notification.CreatedAt,
		notification.UpdatedAt,
	)
	return TranslateSqlxError(err, "notifications")
}

func (s *NotificationStore) GetNotification(ctx context.Context, workspaceID, notificationID string) (*shareddomain.Notification, error) {
	query := `SELECT * FROM core_identity.notifications WHERE id = ? AND workspace_id = ?`

	var dbNotification models.Notification
	if err := s.db.Get(ctx).GetContext(ctx, &dbNotification, query, notificationID, workspaceID); err != nil {
		return nil, TranslateSqlxError(err, "notifications")
	}
	return s.mapNotificationToDomain(&dbNotification), nil
}

func (s *NotificationStore) ListUserNotifications(ctx context.Context, workspaceID, userID string) ([]*shareddomain.Notification, error) {
	query := `SELECT * FROM core_identity.notifications
		WHERE workspace_id = ? AND user_id = ? AND is_archived = FALSE
		ORDER BY created_at DESC`

	var dbNotifications []models.Notification
	if err := s.db.Get(ctx).SelectContext(ctx, &dbNotifications, query, workspaceID, userID); err != nil {
		return nil, TranslateSqlxError(err, "notifications")
	}

	notifications := make([]*shareddomain.Notification, len(dbNotifications))
	for i, notification := range dbNotifications {
		notifications[i] = s.mapNotificationToDomain(&notification)
	}
	return notifications, nil
}

func (s *NotificationStore) mapNotificationToDomain(notification *models.Notification) *shareddomain.Notification {
	if notification == nil {
		return nil
	}

	var deliveryMethods []string
	unmarshalJSONField(notification.DeliveryMethods, &deliveryMethods, "notifications", "delivery_methods")

	var trackingData json.RawMessage
	if len(notification.Metadata) > 0 && json.Valid([]byte(notification.Metadata)) {
		trackingData = json.RawMessage(notification.Metadata)
	}

	status := shareddomain.NotificationStatusPending
	switch {
	case notification.IsRead:
		status = shareddomain.NotificationStatusRead
	case notification.EmailSentAt != nil || notification.PushSentAt != nil || notification.SMSSentAt != nil:
		status = shareddomain.NotificationStatusSent
	}

	result := &shareddomain.Notification{
		ID:           notification.ID,
		WorkspaceID:  notification.WorkspaceID,
		RecipientID:  notification.UserID,
		Type:         shareddomain.NotificationType(notification.Type),
		Priority:     shareddomain.NotificationPriority(notification.Priority),
		Status:       status,
		Subject:      notification.Title,
		Body:         notification.Body,
		ImageURL:     notification.IconURL,
		EntityType:   notification.TargetType,
		EntityID:     notification.TargetID,
		ActionURL:    notification.ActionURL,
		ReadAt:       notification.ReadAt,
		ExpiresAt:    notification.ExpiresAt,
		TrackingData: trackingData,
		CreatedAt:    notification.CreatedAt,
		UpdatedAt:    notification.UpdatedAt,
	}

	if len(deliveryMethods) > 0 && result.Type == "" {
		result.Type = shareddomain.NotificationType(deliveryMethods[0])
	}
	if notification.EmailSentAt != nil {
		sentAt := *notification.EmailSentAt
		result.SentAt = &sentAt
	}
	return result
}
