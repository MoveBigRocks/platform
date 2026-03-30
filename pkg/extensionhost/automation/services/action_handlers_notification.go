package automationservices

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	"github.com/movebigrocks/platform/pkg/logger"
)

// NotificationActionHandler handles email and event publishing actions
type NotificationActionHandler struct {
	outbox contracts.OutboxPublisher
	logger *logger.Logger
}

// NewNotificationActionHandler creates a new notification handler
func NewNotificationActionHandler(outbox contracts.OutboxPublisher) *NotificationActionHandler {
	return &NotificationActionHandler{
		outbox: outbox,
		logger: logger.New().WithField("handler", "notification_action"),
	}
}

// ActionTypes returns the action types this handler supports
func (h *NotificationActionHandler) ActionTypes() []string {
	return []string{"send_email", "email", "publish_event"}
}

// Handle executes the notification action
func (h *NotificationActionHandler) Handle(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	switch action.Type {
	case "send_email", "email":
		return h.handleSendEmail(ctx, action, ruleContext, result)
	case "publish_event":
		return h.handlePublishEvent(ctx, action, ruleContext, result)
	default:
		return fmt.Errorf("unsupported action type: %s", action.Type)
	}
}

func (h *NotificationActionHandler) handleSendEmail(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	if h.outbox == nil {
		h.logger.Warn("outbox publisher not configured, skipping send_email action",
			"action", action.Type,
			"rule_id", safeRuleID(ruleContext),
			"target_type", safeTargetType(ruleContext),
			"target_id", safeTargetID(ruleContext))
		result.Changes.SetString("email_send_skipped", "outbox_not_configured")
		return nil
	}

	// Extract email parameters from action options or use defaults
	toEmail := action.Options.GetString("to")
	subject := action.Options.GetString("subject")
	body := action.Options.GetString("body")

	// Use contact email if no specific recipient
	if toEmail == "" && ruleContext.Contact != nil {
		toEmail = ruleContext.Contact.Email
	}
	if toEmail == "" && ruleContext != nil && ruleContext.FormSubmission != nil {
		toEmail = ruleContext.FormSubmission.SubmitterEmail
	}

	// Use case subject if no specific subject
	if subject == "" {
		subject = defaultNotificationSubject(ruleContext)
	}

	// Use action value as body if no body specified
	if body == "" {
		body = action.Value.AsString()
	}

	if toEmail == "" {
		return fmt.Errorf("no recipient email specified")
	}

	// Create and publish email command event
	workspaceID, err := workspaceIDFromRuleContext(ruleContext)
	if err != nil {
		return err
	}

	emailEvent := events.NewSendEmailRequestedEvent(
		workspaceID,
		"rule_action_executor",
		[]string{toEmail},
		subject,
		body,
	)
	emailEvent.Category = "rule"
	if ruleContext != nil {
		emailEvent.SourceRuleID = ruleContext.RuleID
		if ruleContext.Case != nil {
			emailEvent.CaseID = ruleContext.Case.ID
		}
		if ruleContext.FormSubmission != nil {
			emailEvent.SourceFormID = ruleContext.FormSubmission.FormID
		}
	}

	// Publish the email command event
	if err := h.outbox.PublishEvent(ctx, eventbus.StreamEmailCommands, emailEvent); err != nil {
		return fmt.Errorf("failed to publish email command: %w", err)
	}

	h.logger.Info("Published email command",
		"target_type", safeTargetType(ruleContext),
		"target_id", safeTargetID(ruleContext),
		"to", toEmail,
		"rule_id", safeRuleID(ruleContext))

	result.Changes.SetString("email_requested", toEmail)
	result.Changes.SetString("email_event_id", emailEvent.EventID)
	return nil
}

func (h *NotificationActionHandler) handlePublishEvent(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error {
	if h.outbox == nil {
		h.logger.Warn("outbox publisher not configured, skipping publish_event action",
			"action", action.Type,
			"rule_id", safeRuleID(ruleContext),
			"target_type", safeTargetType(ruleContext),
			"target_id", safeTargetID(ruleContext))
		result.Changes.SetString("event_publish_skipped", "outbox_not_configured")
		return nil
	}

	// Get event stream and payload from action
	stream := "custom.events"
	var payload map[string]interface{}

	if s := action.Options.GetString("stream"); s != "" {
		stream = s
	}
	// Payload can be extracted from metadata as a nested structure
	// For now, we don't support complex payload from options

	// Build event with context
	event := map[string]interface{}{
		"type":         action.Target,
		"workspace_id": "",
		"timestamp":    time.Now(),
		"rule_id":      ruleContext.RuleID,
		"context":      ruleContext.Metadata.ToMap(),
	}

	// Add source-specific context
	if ruleContext.Case != nil {
		event["workspace_id"] = ruleContext.Case.WorkspaceID
		event["case_id"] = ruleContext.Case.ID
	} else if ruleContext.Issue != nil {
		event["workspace_id"] = ruleContext.Issue.WorkspaceID
		event["issue_id"] = ruleContext.Issue.ID
	} else if ruleContext.FormSubmission != nil {
		event["workspace_id"] = ruleContext.FormSubmission.WorkspaceID
		event["form_id"] = ruleContext.FormSubmission.FormID
		event["submission_id"] = ruleContext.FormSubmission.SubmissionID
	}

	// Merge payload
	for k, v := range payload {
		event[k] = v
	}

	// Publish the event
	if err := h.outbox.Publish(ctx, eventbus.StreamFromString(stream), event); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	h.logger.Info("Published custom event", "stream", stream)
	result.Changes.SetString("event_published", stream)
	result.Changes.SetString("event_type", action.Target)
	return nil
}
