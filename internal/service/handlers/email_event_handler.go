package servicehandlers

import (
	"context"
	"encoding/json"
	"fmt"

	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

type inboundEmailReceivedEvent struct {
	EventType   string `json:"event_type"`
	EmailID     string `json:"email_id"`
	WorkspaceID string `json:"workspace_id"`
}

// EmailEventHandler consumes inbound email events and runs the case matching /
// case creation workflow after the webhook write has completed.
type EmailEventHandler struct {
	emailService *serviceapp.EmailService
	logger       *logger.Logger
}

func NewEmailEventHandler(emailService *serviceapp.EmailService, log *logger.Logger) *EmailEventHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &EmailEventHandler{
		emailService: emailService,
		logger:       log.WithField("handler", "email-event"),
	}
}

func (h *EmailEventHandler) HandleEmailReceived(ctx context.Context, eventData []byte) error {
	if h.emailService == nil {
		return fmt.Errorf("email service is required")
	}

	var event inboundEmailReceivedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal inbound email event: %w", err)
	}
	if event.EmailID == "" {
		h.logger.Debug("Skipping inbound email event with missing email_id")
		return nil
	}

	if _, err := h.emailService.ProcessInboundEmail(ctx, event.EmailID); err != nil {
		return fmt.Errorf("process inbound email %s: %w", event.EmailID, err)
	}
	return nil
}

func (h *EmailEventHandler) RegisterHandlers(subscribe func(stream eventbus.Stream, group, consumer string, handler func(context.Context, []byte) error) error) error {
	emailHandler := EventHandlerMiddleware(h.logger, h.HandleEmailReceived)
	if err := subscribe(eventbus.StreamEmailEvents, "email-events-handler", "consumer", emailHandler); err != nil {
		return fmt.Errorf("failed to register email event handler: %w", err)
	}
	h.logger.Info("Email event handlers registered successfully")
	return nil
}
