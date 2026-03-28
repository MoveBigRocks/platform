//go:build integration

package automationservices_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	servicehandlers "github.com/movebigrocks/platform/internal/service/handlers"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/workflowproof"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

type automationRecordingOutbox struct {
	events []interface{}
}

func (o *automationRecordingOutbox) Publish(_ context.Context, _ eventbus.Stream, event interface{}) error {
	o.events = append(o.events, event)
	return nil
}

func (o *automationRecordingOutbox) PublishEvent(_ context.Context, _ eventbus.Stream, event eventbus.Event) error {
	o.events = append(o.events, event)
	return nil
}

func TestNotificationActionHandler_SendEmailDeliversOutboundSideEffect(t *testing.T) {
	store, cleanup := setupAutomationTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	contact := testutil.NewIsolatedContact(t, workspace.ID)
	require.NoError(t, store.Contacts().CreateContact(ctx, contact))
	caseObj := testutil.NewIsolatedCase(t, workspace.ID)
	caseObj.Subject = "Refund request"
	caseObj.ContactEmail = contact.Email
	caseObj.ContactName = contact.Name
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	outbox := &automationRecordingOutbox{}
	handler := automationservices.NewNotificationActionHandler(outbox)

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
	emailHandler := servicehandlers.NewEmailCommandHandler(emailService, logger.NewNop())

	action := automationservices.RuleAction{
		Type:    "send_email",
		Value:   shareddomain.StringValue("We received your refund request."),
		Options: shareddomain.MetadataFromMap(map[string]interface{}{"subject": "Refund update"}),
	}
	ruleContext := &automationservices.RuleContext{
		Case:    caseObj,
		Contact: contact,
		RuleID:  "rule_refund_update",
	}
	result := &automationservices.ActionResult{Changes: automationservices.NewActionChanges()}

	require.NoError(t, handler.Handle(ctx, action, ruleContext, result))

	var emailEvent sharedevents.SendEmailRequestedEvent
	for _, event := range outbox.events {
		candidate, ok := event.(sharedevents.SendEmailRequestedEvent)
		if !ok {
			continue
		}
		emailEvent = candidate
		break
	}
	require.NotEmpty(t, emailEvent.EventID)
	assert.Equal(t, contact.Email, emailEvent.ToEmails[0])

	payload, err := json.Marshal(emailEvent)
	require.NoError(t, err)
	require.NoError(t, emailHandler.HandleSendEmailRequested(ctx, payload))

	sent := mockProvider.GetSentEmails()
	require.Len(t, sent, 1)

	storedOutbound, err := store.OutboundEmails().GetOutboundEmailByProviderMessageID(ctx, sent[0].ProviderMessageID)
	require.NoError(t, err)
	assert.Equal(t, "rule", storedOutbound.Category)
	assert.Equal(t, caseObj.ID, storedOutbound.CaseID)
	assert.Equal(t, "Refund update", storedOutbound.Subject)

	workflowproof.WriteJSON(t, "rule-send-email", map[string]interface{}{
		"workspace_id":        workspace.ID,
		"rule_id":             ruleContext.RuleID,
		"case_id":             caseObj.ID,
		"outbound_email_id":   storedOutbound.ID,
		"provider_message_id": storedOutbound.ProviderMessageID,
		"status":              storedOutbound.Status,
	})
}

func setupAutomationTestStore(t *testing.T) (stores.Store, func()) {
	t.Helper()
	return testutil.SetupTestStore(t)
}
