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

// EmailCommandHandler consumes send-email command events and turns them into
// durable outbound email records plus provider send attempts.
type EmailCommandHandler struct {
	emailService *serviceapp.EmailService
	logger       *logger.Logger
}

func NewEmailCommandHandler(emailService *serviceapp.EmailService, log *logger.Logger) *EmailCommandHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &EmailCommandHandler{
		emailService: emailService,
		logger:       log.WithField("handler", "email-command"),
	}
}

func (h *EmailCommandHandler) HandleSendEmailRequested(ctx context.Context, eventData []byte) error {
	if h.emailService == nil {
		return fmt.Errorf("email service is required")
	}

	var event sharedevents.SendEmailRequestedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal email command event: %w", err)
	}

	if _, err := h.emailService.ProcessSendEmailRequested(ctx, event); err != nil {
		return fmt.Errorf("process send email requested event %s: %w", event.EventID, err)
	}
	return nil
}

func (h *EmailCommandHandler) RegisterHandlers(subscribe func(stream eventbus.Stream, group, consumer string, handler func(context.Context, []byte) error) error) error {
	handler := EventHandlerMiddleware(h.logger, h.HandleSendEmailRequested)
	if err := subscribe(eventbus.StreamEmailCommands, "email-commands-handler", "consumer", handler); err != nil {
		return fmt.Errorf("failed to register email command handler: %w", err)
	}
	h.logger.Info("Email command handlers registered successfully")
	return nil
}
