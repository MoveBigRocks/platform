package serviceapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
	"github.com/movebigrocks/platform/pkg/logger"
)

// NotificationService processes notification command events into durable
// in-app notifications or bridged outbound email sends.
type NotificationService struct {
	store        stores.Store
	emailService *EmailService
	logger       *logger.Logger
}

func NewNotificationService(store stores.Store, emailService *EmailService, log *logger.Logger) *NotificationService {
	if log == nil {
		log = logger.NewNop()
	}
	return &NotificationService{
		store:        store,
		emailService: emailService,
		logger:       log.WithField("service", "notification"),
	}
}

func (s *NotificationService) ProcessSendNotificationRequested(ctx context.Context, event sharedevents.SendNotificationRequestedEvent) ([]*shareddomain.Notification, error) {
	switch strings.ToLower(strings.TrimSpace(event.Type)) {
	case string(shareddomain.NotificationTypeInApp):
		return s.createInAppNotifications(ctx, event)
	case string(shareddomain.NotificationTypeEmail):
		if s.emailService == nil {
			return nil, fmt.Errorf("email service is required for email notifications")
		}
		emailEvent := sharedevents.NewSendEmailRequestedEvent(
			event.WorkspaceID,
			event.RequestedBy,
			append([]string(nil), event.Recipients...),
			event.Subject,
			event.Body,
		)
		emailEvent.Category = "notification"
		emailEvent.TemplateData = cloneTemplateData(event.Data)
		if templateID := normalizeNotificationTemplateID(event.Template); templateID != "" {
			emailEvent.TemplateID = templateID
		} else if templateName := strings.TrimSpace(event.Template); templateName != "" {
			emailEvent.TemplateData["notification_template"] = templateName
		}
		switch strings.TrimSpace(event.SourceType) {
		case "form":
			emailEvent.SourceFormID = event.SourceID
		case "rule":
			emailEvent.SourceRuleID = event.SourceID
		}
		if _, err := s.emailService.ProcessSendEmailRequested(ctx, emailEvent); err != nil {
			return nil, err
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported notification type %q", event.Type)
	}
}

func (s *NotificationService) createInAppNotifications(ctx context.Context, event sharedevents.SendNotificationRequestedEvent) ([]*shareddomain.Notification, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is required")
	}

	var created []*shareddomain.Notification
	err := s.store.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.SetTenantContext(txCtx, event.WorkspaceID); err != nil {
			return fmt.Errorf("set tenant context: %w", err)
		}

		workspaceUsers, err := s.store.Workspaces().GetWorkspaceUsers(txCtx, event.WorkspaceID)
		if err != nil {
			return fmt.Errorf("list workspace users: %w", err)
		}
		allowedRecipients := make(map[string]struct{}, len(workspaceUsers))
		for _, membership := range workspaceUsers {
			if membership == nil {
				continue
			}
			allowedRecipients[strings.TrimSpace(membership.UserID)] = struct{}{}
		}

		trackingData, err := marshalNotificationData(event.Data)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		seen := make(map[string]struct{}, len(event.Recipients))
		for _, recipientID := range event.Recipients {
			recipientID = strings.TrimSpace(recipientID)
			if recipientID == "" {
				continue
			}
			if _, exists := seen[recipientID]; exists {
				continue
			}
			seen[recipientID] = struct{}{}
			if _, allowed := allowedRecipients[recipientID]; !allowed {
				return fmt.Errorf("notification recipient %s is not a member of workspace %s", recipientID, event.WorkspaceID)
			}

			notification := &shareddomain.Notification{
				ID:           id.New(),
				WorkspaceID:  event.WorkspaceID,
				RecipientID:  recipientID,
				Type:         shareddomain.NotificationTypeInApp,
				Category:     notificationCategoryForSourceType(event.SourceType),
				Priority:     shareddomain.NotificationPriorityMedium,
				Status:       shareddomain.NotificationStatusSent,
				Subject:      event.Subject,
				Body:         event.Body,
				TemplateID:   event.Template,
				EntityType:   event.SourceType,
				EntityID:     event.SourceID,
				ActionURL:    notificationActionURL(event.Data),
				ImageURL:     notificationImageURL(event.Data),
				TrackingData: trackingData,
				SentAt:       &now,
				CreatedAt:    now,
				UpdatedAt:    now,
			}

			if err := s.store.Notifications().CreateNotification(txCtx, notification); err != nil {
				return fmt.Errorf("create notification for %s: %w", recipientID, err)
			}
			created = append(created, notification)
		}

		if len(created) == 0 {
			return fmt.Errorf("no valid notification recipients")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func marshalNotificationData(data map[string]interface{}) (json.RawMessage, error) {
	if len(data) == 0 {
		return json.RawMessage(`{}`), nil
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal notification data: %w", err)
	}
	return json.RawMessage(payload), nil
}

func notificationActionURL(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	value, ok := data["action_url"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func notificationImageURL(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	value, ok := data["image_url"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func notificationCategoryForSourceType(sourceType string) shareddomain.NotificationCategory {
	switch strings.TrimSpace(sourceType) {
	case "case":
		return shareddomain.NotificationCategoryCaseUpdate
	case "knowledge_review":
		return shareddomain.NotificationCategoryWorkflow
	default:
		return shareddomain.NotificationCategoryWorkflow
	}
}

func normalizeNotificationTemplateID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if _, err := uuid.Parse(value); err != nil {
		return ""
	}
	return value
}
