package observabilityhandlers

import (
	"context"
	"encoding/json"
	"fmt"

	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	observabilityservices "github.com/movebigrocks/platform/internal/observability/services"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// ErrorEventHandler handles incoming error events from the error-events stream
// It processes raw error events and groups them into issues
type ErrorEventHandler struct {
	processor *observabilityservices.ErrorProcessor
	logger    *logger.Logger
}

// NewErrorEventHandler creates a new error event handler
func NewErrorEventHandler(processor *observabilityservices.ErrorProcessor, log *logger.Logger) *ErrorEventHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &ErrorEventHandler{
		processor: processor,
		logger:    log,
	}
}

// HandleErrorEvent processes an incoming error event
// This is the bridge between the event bus and the ErrorProcessor
func (h *ErrorEventHandler) HandleErrorEvent(ctx context.Context, eventData []byte) error {
	// Parse the error event
	var event observabilitydomain.ErrorEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal ErrorEvent: %w", err)
	}

	h.logger.WithFields(map[string]interface{}{
		"event_id":   event.EventID,
		"project_id": event.ProjectID,
		"level":      event.Level,
	}).Info("Processing error event")

	// Process the event through ErrorProcessor
	// This will group the event into an issue and publish IssueCreated/IssueUpdated events
	if err := h.processor.ProcessEvent(ctx, &event); err != nil {
		h.logger.WithError(err).WithField("event_id", event.EventID).Error("Failed to process error event")
		return fmt.Errorf("failed to process error event: %w", err)
	}

	h.logger.WithField("event_id", event.EventID).Info("Successfully processed error event")
	return nil
}

// RegisterHandlers registers the error event handler with the event bus
func (h *ErrorEventHandler) RegisterHandlers(subscribe func(stream eventbus.Stream, group, consumer string, handler func(context.Context, []byte) error) error) error {
	// Wrap handler with middleware
	errorEventHandler := EventHandlerMiddleware(h.logger, h.HandleErrorEvent)

	// Subscribe to error events stream
	if err := subscribe(eventbus.StreamErrorEvents, "error-processors", "consumer", errorEventHandler); err != nil {
		return fmt.Errorf("failed to register error event handler: %w", err)
	}

	h.logger.Info("Error event handlers registered successfully")
	return nil
}
