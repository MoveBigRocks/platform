package servicehandlers

import (
	"context"
	"encoding/json"
	"fmt"

	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/pkg/eventbus"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/logger"
)

// NotificationCommandHandler consumes notification command events and turns
// them into durable in-app notifications or bridged email sends.
type NotificationCommandHandler struct {
	notificationService *serviceapp.NotificationService
	logger              *logger.Logger
}

func NewNotificationCommandHandler(notificationService *serviceapp.NotificationService, log *logger.Logger) *NotificationCommandHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &NotificationCommandHandler{
		notificationService: notificationService,
		logger:              log.WithField("handler", "notification-command"),
	}
}

func (h *NotificationCommandHandler) HandleSendNotificationRequested(ctx context.Context, eventData []byte) error {
	if h.notificationService == nil {
		return fmt.Errorf("notification service is required")
	}

	var event sharedevents.SendNotificationRequestedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal notification command event: %w", err)
	}

	if _, err := h.notificationService.ProcessSendNotificationRequested(ctx, event); err != nil {
		return fmt.Errorf("process send notification requested event %s: %w", event.EventID, err)
	}
	return nil
}

func (h *NotificationCommandHandler) RegisterHandlers(subscribe func(stream eventbus.Stream, group, consumer string, handler func(context.Context, []byte) error) error) error {
	handler := EventHandlerMiddleware(h.logger, h.HandleSendNotificationRequested)
	if err := subscribe(eventbus.StreamNotificationCommands, "notification-commands-handler", "consumer", handler); err != nil {
		return fmt.Errorf("failed to register notification command handler: %w", err)
	}
	h.logger.Info("Notification command handlers registered successfully")
	return nil
}
