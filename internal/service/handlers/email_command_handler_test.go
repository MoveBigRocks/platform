//go:build integration

package servicehandlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/workflowproof"
	"github.com/movebigrocks/platform/internal/testutil/workflowruntime"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

func TestEmailCommandHandler_HandleSendEmailRequestedCreatesAndSendsOutboundEmail(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	mockProvider := serviceapp.NewMockProvider()
	registry := serviceapp.NewEmailProviderRegistry()
	registry.Register("mock", func(config serviceapp.EmailConfig) (serviceapp.EmailProvider, error) {
		return mockProvider, nil
	})

	emailService, err := serviceapp.NewEmailServiceWithRegistry(store, serviceapp.EmailConfig{
		Provider:         "mock",
		DefaultFromEmail: "support@movebigrocks.test",
		DefaultFromName:  "Support",
	}, nil, registry)
	require.NoError(t, err)

	handler := NewEmailCommandHandler(emailService, logger.NewNop())

	event := sharedevents.NewSendEmailRequestedEvent(
		workspace.ID,
		"form_event_handler",
		[]string{"customer@example.com"},
		"New form submission",
		"Please review the new submission.",
	)
	event.Category = "form"

	payload, err := json.Marshal(event)
	require.NoError(t, err)

	require.NoError(t, handler.HandleSendEmailRequested(ctx, payload))

	sent := mockProvider.GetSentEmails()
	require.Len(t, sent, 1)
	assert.Equal(t, "New form submission", sent[0].Subject)
	assert.Equal(t, "support@movebigrocks.test", sent[0].FromEmail)

	stored, err := store.OutboundEmails().GetOutboundEmailByProviderMessageID(ctx, sent[0].ProviderMessageID)
	require.NoError(t, err)
	assert.Equal(t, workspace.ID, stored.WorkspaceID)
	assert.Equal(t, "form", stored.Category)
	assert.Equal(t, servicedomain.EmailStatusSent, stored.Status)
	assert.NotNil(t, stored.SentAt)
}

func TestEmailCommandHandler_HandleSendEmailRequestedUsesExistingOutboundEmailAndSyncsCommunicationMessageID(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		nil,
		serviceapp.WithTransactionRunner(store),
		serviceapp.WithOutboundEmailStore(store.OutboundEmails()),
	)
	caseObj, err := caseService.CreateCase(ctx, serviceapp.CreateCaseParams{
		WorkspaceID:  workspace.ID,
		Subject:      "Billing follow-up",
		ContactEmail: "customer@example.com",
	})
	require.NoError(t, err)

	comm := servicedomain.NewCommunication(caseObj.ID, workspace.ID, shareddomain.CommTypeEmail, "Prior agent response")
	comm.Direction = shareddomain.DirectionOutbound
	comm.IsInternal = false
	user := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, user))
	comm.FromUserID = user.ID
	comm.FromEmail = "agent@example.com"
	comm.FromName = "Test Agent"
	comm.ToEmails = []string{"customer@example.com"}
	comm.Subject = "Re: Billing follow-up"
	require.NoError(t, caseService.AddCommunication(ctx, comm))

	outbound := servicedomain.NewOutboundEmail(workspace.ID, "agent@example.com", comm.Subject, comm.Body)
	outbound.FromName = "Test Agent"
	outbound.ToEmails = []string{"customer@example.com"}
	outbound.ReplyToEmail = "agent@example.com"
	outbound.Category = "case-reply"
	outbound.CaseID = caseObj.ID
	outbound.CommunicationID = comm.ID
	outbound.CreatedByID = comm.FromUserID
	require.NoError(t, store.OutboundEmails().CreateOutboundEmail(ctx, outbound))

	mockProvider := serviceapp.NewMockProvider()
	registry := serviceapp.NewEmailProviderRegistry()
	registry.Register("mock", func(config serviceapp.EmailConfig) (serviceapp.EmailProvider, error) {
		return mockProvider, nil
	})

	emailService, err := serviceapp.NewEmailServiceWithRegistry(store, serviceapp.EmailConfig{
		Provider:         "mock",
		DefaultFromEmail: "support@movebigrocks.test",
		DefaultFromName:  "Support",
	}, caseService, registry)
	require.NoError(t, err)
	handler := NewEmailCommandHandler(emailService, logger.NewNop())

	event := sharedevents.NewSendEmailRequestedEvent(
		workspace.ID,
		"case_service",
		[]string{"customer@example.com"},
		comm.Subject,
		comm.Body,
	)
	event.OutboundEmailID = outbound.ID
	event.CaseID = caseObj.ID
	event.Category = "case-reply"
	event.ReplyTo = "agent@example.com"

	payload, err := json.Marshal(event)
	require.NoError(t, err)
	require.NoError(t, handler.HandleSendEmailRequested(ctx, payload))

	stored, err := store.OutboundEmails().GetOutboundEmail(ctx, outbound.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.EmailStatusSent, stored.Status)
	assert.NotEmpty(t, stored.ProviderMessageID)
	threadMessageID := stored.ProviderSettings["header_message_id"]
	threadMessageIDValue, ok := threadMessageID.(string)
	require.True(t, ok)
	require.NotEmpty(t, threadMessageIDValue)

	updatedComm, err := store.Cases().GetCommunication(ctx, workspace.ID, comm.ID)
	require.NoError(t, err)
	assert.Equal(t, threadMessageIDValue, updatedComm.MessageID)
}

func TestCaseReplyWorkflow_QueuesAndSendsOutboundEmail(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	user := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, user))

	runtime := workflowruntime.NewHarness(t, store)
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		runtime.Outbox,
		serviceapp.WithTransactionRunner(store),
		serviceapp.WithOutboundEmailStore(store.OutboundEmails()),
	)

	caseObj, err := caseService.CreateCase(ctx, serviceapp.CreateCaseParams{
		WorkspaceID:  workspace.ID,
		Subject:      "Billing follow-up",
		ContactEmail: "customer@example.com",
		ContactName:  "Casey Customer",
		Channel:      servicedomain.CaseChannelEmail,
	})
	require.NoError(t, err)

	mockProvider := serviceapp.NewMockProvider()
	registry := serviceapp.NewEmailProviderRegistry()
	registry.Register("mock", func(config serviceapp.EmailConfig) (serviceapp.EmailProvider, error) {
		return mockProvider, nil
	})

	emailService, err := serviceapp.NewEmailServiceWithRegistry(store, serviceapp.EmailConfig{
		Provider:         "mock",
		DefaultFromEmail: "support@movebigrocks.test",
		DefaultFromName:  "Support",
	}, caseService, registry)
	require.NoError(t, err)
	handler := NewEmailCommandHandler(emailService, logger.NewNop())
	require.NoError(t, handler.RegisterHandlers(runtime.EventBus.Subscribe))

	reply, err := caseService.ReplyToCase(ctx, serviceapp.ReplyToCaseParams{
		WorkspaceID: workspace.ID,
		CaseID:      caseObj.ID,
		UserID:      user.ID,
		UserName:    user.Name,
		UserEmail:   user.Email,
		ToEmails:    []string{"customer@example.com"},
		Subject:     "Re: Billing follow-up",
		Body:        "Here is the latest billing update.",
	})
	require.NoError(t, err)
	require.NotNil(t, reply)

	pendingEvents, err := store.Outbox().GetPendingOutboxEvents(ctx, 10)
	require.NoError(t, err)
	require.NotEmpty(t, pendingEvents)

	var emailEvent sharedevents.SendEmailRequestedEvent
	for _, pending := range pendingEvents {
		if pending.Stream != eventbus.StreamEmailCommands.String() {
			continue
		}
		require.NoError(t, json.Unmarshal(pending.EventData, &emailEvent))
		if emailEvent.CaseID == caseObj.ID {
			break
		}
	}
	require.NotEmpty(t, emailEvent.OutboundEmailID)

	pendingOutbound, err := store.OutboundEmails().GetOutboundEmail(ctx, emailEvent.OutboundEmailID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.EmailStatusPending, pendingOutbound.Status)
	assert.Equal(t, reply.ID, pendingOutbound.CommunicationID)

	runtime.Start(t)

	var storedOutbound *servicedomain.OutboundEmail
	require.Eventually(t, func() bool {
		storedOutbound, err = store.OutboundEmails().GetOutboundEmail(ctx, pendingOutbound.ID)
		require.NoError(t, err)
		return storedOutbound.Status == servicedomain.EmailStatusSent && storedOutbound.ProviderMessageID != ""
	}, 2*time.Second, 25*time.Millisecond)

	threadMessageID, ok := storedOutbound.ProviderSettings["header_message_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, threadMessageID)

	storedReply, err := store.Cases().GetCommunication(ctx, workspace.ID, reply.ID)
	require.NoError(t, err)
	assert.Equal(t, threadMessageID, storedReply.MessageID)

	workflowproof.WriteJSON(t, "case-reply-send", map[string]interface{}{
		"workspace_id":        workspace.ID,
		"case_id":             caseObj.ID,
		"communication_id":    reply.ID,
		"outbound_email_id":   storedOutbound.ID,
		"provider_message_id": storedOutbound.ProviderMessageID,
		"thread_message_id":   threadMessageID,
		"status":              storedOutbound.Status,
	})
}
