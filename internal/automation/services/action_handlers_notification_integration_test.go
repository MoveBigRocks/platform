//go:build integration

package automationservices_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/managedworkflowruntime"
	"github.com/movebigrocks/platform/internal/testutil/workflowproof"
)

func TestNotificationActionHandler_SendEmailDeliversOutboundSideEffect(t *testing.T) {
	store, cleanup := setupAutomationTestStore(t)
	defer cleanup()

	ctx := context.Background()
	runtime := managedworkflowruntime.NewHarness(t, store)
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	contact := testutil.NewIsolatedContact(t, workspace.ID)
	require.NoError(t, store.Contacts().CreateContact(ctx, contact))
	caseObj := testutil.NewIsolatedCase(t, workspace.ID)
	caseObj.Subject = "Refund request"
	caseObj.ContactEmail = contact.Email
	caseObj.ContactName = contact.Name
	require.NoError(t, store.Cases().CreateCase(ctx, caseObj))

	handler := automationservices.NewNotificationActionHandler(runtime.Outbox)

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
	runtime.UseManager(t, managedworkflowruntime.ManagerDeps{
		EmailService: emailService,
	})
	runtime.Start(t)

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

	var storedOutbound *servicedomain.OutboundEmail
	require.Eventually(t, func() bool {
		for _, sent := range mockProvider.GetSentEmails() {
			storedOutbound, err = store.OutboundEmails().GetOutboundEmailByProviderMessageID(ctx, sent.ProviderMessageID)
			if err == nil && storedOutbound.Status == servicedomain.EmailStatusSent {
				return true
			}
		}
		return false
	}, 2*time.Second, 25*time.Millisecond)

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
