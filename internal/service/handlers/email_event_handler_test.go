//go:build integration

package servicehandlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
	"github.com/movebigrocks/platform/pkg/logger"
)

func TestEmailEventHandler_HandleEmailReceivedProcessesInboundEmail(t *testing.T) {
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
	)
	emailService, err := serviceapp.NewEmailService(store, serviceapp.EmailConfig{Provider: "mock"}, caseService)
	require.NoError(t, err)

	handler := NewEmailEventHandler(emailService, logger.NewNop())

	inbound := servicedomain.NewInboundEmail(workspace.ID, "<worker-event@example.com>", "customer@example.com", "Worker created case", "The worker should create a case for this message.")
	inbound.ToEmails = []string{"support@example.com"}
	require.NoError(t, store.InboundEmails().CreateInboundEmail(ctx, inbound))

	payload, err := json.Marshal(map[string]interface{}{
		"event_type":   "email_received",
		"email_id":     inbound.ID,
		"workspace_id": workspace.ID,
	})
	require.NoError(t, err)

	require.NoError(t, handler.HandleEmailReceived(ctx, payload))

	stored, err := store.InboundEmails().GetInboundEmail(ctx, inbound.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.EmailProcessingStatusProcessed, stored.ProcessingStatus)
	assert.NotEmpty(t, stored.CaseID)
	assert.NotEmpty(t, stored.CommunicationID)

	caseObj, err := store.Cases().GetCase(ctx, stored.CaseID)
	require.NoError(t, err)
	assert.Equal(t, workspace.ID, caseObj.WorkspaceID)
	assert.Equal(t, "customer@example.com", caseObj.ContactEmail)
}
