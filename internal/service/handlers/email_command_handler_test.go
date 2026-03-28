//go:build integration

package servicehandlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/internal/testutil"
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

	updatedComm, err := store.Cases().GetCommunication(ctx, workspace.ID, comm.ID)
	require.NoError(t, err)
	assert.Equal(t, stored.ProviderMessageID, updatedComm.MessageID)
}
