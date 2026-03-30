//go:build integration

package servicehandlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

func TestFormEventHandler_HandleFormSubmitted(t *testing.T) {
	store, cleanup := setupFormTestStore(t)
	defer cleanup()

	outbox := &formMockOutbox{publishedEvents: []interface{}{}}
	formService := automationservices.NewFormServiceWithDeps(
		store.Forms(),
		store.Users(),
		store,
		store,
		outbox,
	)
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		outbox,
		serviceapp.WithTransactionRunner(store),
	)
	handler := NewFormEventHandler(formService, caseService, outbox, store, logger.NewNop())

	t.Run("creates case, queues notification, and completes submission", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_form_event", "Contact Support", "contact-support")
		form.AutoCreateCase = true
		form.AutoCasePriority = string(servicedomain.CasePriorityHigh)
		form.AutoCaseType = "support"
		form.AutoTags = []string{"web"}
		form.NotifyOnSubmission = true
		form.NotificationEmails = []string{"support@example.com"}
		require.NoError(t, store.Forms().UpdateFormSchema(context.Background(), form))

		submission := servicedomain.NewPublicFormSubmission(form.WorkspaceID, form.ID, shareddomain.MetadataFromMap(map[string]interface{}{
			"name":    "Jane Doe",
			"email":   "jane@example.com",
			"message": "Need help with my account",
		}))
		submission.SubmitterName = "Jane Doe"
		submission.SubmitterEmail = "jane@example.com"
		require.NoError(t, store.Forms().CreateFormSubmission(context.Background(), submission))

		event := contracts.NewFormSubmittedEvent(
			form.ID,
			form.Slug,
			submission.ID,
			form.WorkspaceID,
			submission.SubmitterEmail,
			submission.SubmitterName,
			submission.Data.ToInterfaceMap(),
		)
		eventData, err := json.Marshal(event)
		require.NoError(t, err)

		require.NoError(t, handler.HandleFormSubmitted(context.Background(), eventData))

		updatedSubmission, err := store.Forms().GetFormSubmission(context.Background(), submission.ID)
		require.NoError(t, err)
		assert.Equal(t, servicedomain.SubmissionStatusCompleted, updatedSubmission.Status)
		assert.NotEmpty(t, updatedSubmission.CaseID)
		assert.NotNil(t, updatedSubmission.ProcessedAt)
		assert.Equal(t, "form_event_handler", updatedSubmission.ProcessedByID)

		cases, total, err := store.Cases().ListCases(context.Background(), contracts.CaseFilters{
			WorkspaceID: form.WorkspaceID,
			Limit:       10,
		})
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Len(t, cases, 1)
		assert.Equal(t, cases[0].ID, updatedSubmission.CaseID)
		assert.Equal(t, servicedomain.CasePriorityHigh, cases[0].Priority)
		assert.Contains(t, cases[0].Tags, "form-submission")
		assert.Contains(t, cases[0].Tags, form.Slug)
		assert.Contains(t, cases[0].Tags, "web")

		var emailQueued bool
		for _, published := range outbox.publishedEvents {
			if emailEvent, ok := published.(sharedevents.SendEmailRequestedEvent); ok {
				emailQueued = true
				assert.Equal(t, form.NotificationEmails, emailEvent.ToEmails)
				assert.Equal(t, form.ID, emailEvent.SourceFormID)
			}
		}
		assert.True(t, emailQueued, "expected notification email event to be queued")
	})

	t.Run("rolls back case creation when outbox publish fails", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_form_rollback", "Rollback Form", "rollback-form")
		form.AutoCreateCase = true
		form.NotifyOnSubmission = true
		form.NotificationEmails = []string{"ops@example.com"}
		require.NoError(t, store.Forms().UpdateFormSchema(context.Background(), form))

		submission := servicedomain.NewPublicFormSubmission(form.WorkspaceID, form.ID, shareddomain.MetadataFromMap(map[string]interface{}{
			"email": "rollback@example.com",
		}))
		submission.SubmitterEmail = "rollback@example.com"
		require.NoError(t, store.Forms().CreateFormSubmission(context.Background(), submission))

		event := contracts.NewFormSubmittedEvent(
			form.ID,
			form.Slug,
			submission.ID,
			form.WorkspaceID,
			submission.SubmitterEmail,
			submission.SubmitterName,
			submission.Data.ToInterfaceMap(),
		)
		eventData, err := json.Marshal(event)
		require.NoError(t, err)

		outbox.publishCalls = 0
		outbox.failOnCall = 2
		outbox.publishErr = errors.New("outbox unavailable")
		defer func() {
			outbox.publishErr = nil
			outbox.failOnCall = 0
		}()

		err = handler.HandleFormSubmitted(context.Background(), eventData)
		require.Error(t, err)

		updatedSubmission, getErr := store.Forms().GetFormSubmission(context.Background(), submission.ID)
		require.NoError(t, getErr)
		assert.Equal(t, servicedomain.SubmissionStatusPending, updatedSubmission.Status)

		cases, total, listErr := store.Cases().ListCases(context.Background(), contracts.CaseFilters{
			WorkspaceID: form.WorkspaceID,
			Limit:       10,
		})
		require.NoError(t, listErr)
		assert.Equal(t, 0, total)
		assert.Len(t, cases, 0)
	})
}
