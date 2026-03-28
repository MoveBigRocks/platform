package servicehandlers

import (
	"context"
	"encoding/json"
	"fmt"

	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// CaseCommandHandler consumes sanctioned case-command events and executes them
// through the core case service.
type CaseCommandHandler struct {
	caseService *serviceapp.CaseService
	logger      *logger.Logger
}

func NewCaseCommandHandler(caseService *serviceapp.CaseService, log *logger.Logger) *CaseCommandHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &CaseCommandHandler{
		caseService: caseService,
		logger:      log.WithField("handler", "case-command"),
	}
}

func (h *CaseCommandHandler) HandleCreateCaseRequested(ctx context.Context, eventData []byte) error {
	if h.caseService == nil {
		return fmt.Errorf("case service is required")
	}

	var event sharedevents.CreateCaseRequestedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal case command event: %w", err)
	}

	if _, err := h.caseService.ProcessCreateCaseRequested(ctx, event); err != nil {
		return fmt.Errorf("process create case requested event %s: %w", event.EventID, err)
	}
	return nil
}

func (h *CaseCommandHandler) RegisterHandlers(subscribe func(stream eventbus.Stream, group, consumer string, handler func(context.Context, []byte) error) error) error {
	handler := EventHandlerMiddleware(h.logger, h.HandleCreateCaseRequested)
	if err := subscribe(eventbus.StreamCaseCommands, "case-commands-handler", "consumer", handler); err != nil {
		return fmt.Errorf("failed to register case command handler: %w", err)
	}
	h.logger.Info("Case command handlers registered successfully")
	return nil
}
