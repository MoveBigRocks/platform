package servicehandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/pkg/eventbus"
	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// FormEventHandler processes form submission events after they have been durably
// written to the outbox. It applies built-in form automation like auto case
// creation and submission notifications.
type FormEventHandler struct {
	formService *automationservices.FormService
	caseService *serviceapp.CaseService
	outbox      contracts.OutboxPublisher
	txRunner    contracts.TransactionRunner
	logger      *logger.Logger
}

func NewFormEventHandler(
	formService *automationservices.FormService,
	caseService *serviceapp.CaseService,
	outbox contracts.OutboxPublisher,
	txRunner contracts.TransactionRunner,
	log *logger.Logger,
) *FormEventHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &FormEventHandler{
		formService: formService,
		caseService: caseService,
		outbox:      outbox,
		txRunner:    txRunner,
		logger:      log.WithField("handler", "form-event"),
	}
}

func (h *FormEventHandler) HandleFormSubmitted(ctx context.Context, eventData []byte) error {
	startedAt := time.Now()

	var event contracts.FormSubmittedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal FormSubmitted event: %w", err)
	}
	if event.FormID == "" || event.SubmissionID == "" || event.WorkspaceID == "" {
		h.logger.Debug("Skipping FormSubmitted event with missing identifiers")
		return nil
	}
	if h.txRunner == nil {
		return fmt.Errorf("transaction runner is required for form event handling")
	}

	return h.txRunner.WithTransaction(ctx, func(txCtx context.Context) error {
		form, err := h.formService.GetForm(txCtx, event.FormID)
		if err != nil {
			return fmt.Errorf("load form: %w", err)
		}
		submission, err := h.formService.GetSubmission(txCtx, event.SubmissionID)
		if err != nil {
			return fmt.Errorf("load submission: %w", err)
		}
		if submission.Status == servicedomain.SubmissionStatusCompleted {
			return nil
		}

		submission.Status = servicedomain.SubmissionStatusProcessing
		submission.ProcessingError = ""
		submission.UpdatedAt = time.Now()

		if form.AutoCreateCase {
			caseObj, err := h.caseService.CreateCaseFromForm(
				txCtx,
				form.WorkspaceID,
				buildFormSubmissionSubject(form, &event),
				buildFormSubmissionDescription(form, &event),
				serviceapp.CreateCaseParams{
					Priority:     parseFormCasePriority(form.AutoCasePriority),
					Category:     form.AutoCaseType,
					ContactEmail: event.SubmitterEmail,
					ContactName:  event.SubmitterName,
					TeamID:       form.AutoAssignTeamID,
					AssignedToID: form.AutoAssignUserID,
					Tags:         append([]string{"form-submission", form.Slug}, form.AutoTags...),
					CustomFields: buildFormCaseCustomFields(&event),
				},
			)
			if err != nil {
				return fmt.Errorf("create case from form submission: %w", err)
			}
			submission.CaseID = caseObj.ID
		}

		if form.NotifyOnSubmission && len(form.NotificationEmails) > 0 {
			if h.outbox == nil {
				return fmt.Errorf("outbox publisher is required for form notifications")
			}
			emailEvent := sharedevents.NewSendEmailRequestedEvent(
				form.WorkspaceID,
				"form_event_handler",
				form.NotificationEmails,
				fmt.Sprintf("New form submission: %s", form.Name),
				buildFormSubmissionDescription(form, &event),
			)
			emailEvent.Category = "form"
			emailEvent.SourceFormID = form.ID
			if err := h.outbox.PublishEvent(txCtx, eventbus.StreamEmailCommands, emailEvent); err != nil {
				return fmt.Errorf("queue form notification email: %w", err)
			}
		}

		submission.Status = servicedomain.SubmissionStatusCompleted
		now := time.Now()
		submission.ProcessedAt = &now
		submission.ProcessedByID = "form_event_handler"
		submission.ProcessingTime = now.Sub(startedAt).Milliseconds()
		submission.UpdatedAt = now
		if err := h.formService.UpdateSubmission(txCtx, submission); err != nil {
			return fmt.Errorf("update submission status: %w", err)
		}

		return nil
	})
}

func (h *FormEventHandler) RegisterHandlers(subscribe func(stream eventbus.Stream, group, consumer string, handler func(context.Context, []byte) error) error) error {
	formHandler := EventHandlerMiddleware(h.logger, h.HandleFormSubmitted)
	if err := subscribe(eventbus.StreamFormEvents, "form-events-handler", "consumer", formHandler); err != nil {
		return fmt.Errorf("failed to register form event handler: %w", err)
	}
	h.logger.Info("Form event handlers registered successfully")
	return nil
}

func buildFormSubmissionSubject(form *servicedomain.FormSchema, event *contracts.FormSubmittedEvent) string {
	if subject, ok := event.Data["subject"].(string); ok && subject != "" {
		return subject
	}
	if title, ok := event.Data["title"].(string); ok && title != "" {
		return title
	}
	if form.Name != "" {
		return fmt.Sprintf("Form submission: %s", form.Name)
	}
	return fmt.Sprintf("Form submission: %s", event.FormSlug)
}

func buildFormSubmissionDescription(form *servicedomain.FormSchema, event *contracts.FormSubmittedEvent) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("**Form:** %s", form.Name))
	parts = append(parts, fmt.Sprintf("**Form Slug:** %s", event.FormSlug))
	parts = append(parts, fmt.Sprintf("**Submission ID:** %s", event.SubmissionID))

	if description, ok := event.Data["description"].(string); ok && description != "" {
		parts = append(parts, "", description)
	} else if message, ok := event.Data["message"].(string); ok && message != "" {
		parts = append(parts, "", message)
	} else if body, ok := event.Data["body"].(string); ok && body != "" {
		parts = append(parts, "", body)
	}

	parts = append(parts, "", "**Form Data:**")
	for key, value := range event.Data {
		switch key {
		case "subject", "description", "message", "body", "title":
			continue
		default:
			parts = append(parts, fmt.Sprintf("- **%s:** %v", key, value))
		}
	}

	return strings.Join(parts, "\n")
}

func buildFormCaseCustomFields(event *contracts.FormSubmittedEvent) shareddomain.TypedCustomFields {
	customFields := shareddomain.NewTypedCustomFields()
	customFields.SetString("form_id", event.FormID)
	customFields.SetString("form_slug", event.FormSlug)
	customFields.SetString("submission_id", event.SubmissionID)
	customFields.SetString("source", "form_automation")
	customFields.SetBool("auto_created", true)
	for key, value := range event.Data {
		customFields.SetAny("form_"+key, value)
	}
	return customFields
}

func parseFormCasePriority(priority string) servicedomain.CasePriority {
	switch servicedomain.CasePriority(priority) {
	case servicedomain.CasePriorityLow:
		return servicedomain.CasePriorityLow
	case servicedomain.CasePriorityMedium:
		return servicedomain.CasePriorityMedium
	case servicedomain.CasePriorityHigh:
		return servicedomain.CasePriorityHigh
	case servicedomain.CasePriorityUrgent:
		return servicedomain.CasePriorityUrgent
	default:
		return servicedomain.CasePriorityMedium
	}
}
