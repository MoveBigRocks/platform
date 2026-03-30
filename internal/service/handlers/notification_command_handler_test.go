//go:build integration

package servicehandlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	storeshared "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil/workflowproof"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil/workflowruntime"
	"github.com/movebigrocks/platform/pkg/id"
	"github.com/movebigrocks/platform/pkg/logger"
)

func TestNotificationCommandHandler_HandleSendNotificationRequestedCreatesInAppNotifications(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	userOne := createWorkspaceMember(t, ctx, store, workspace.ID)
	userTwo := createWorkspaceMember(t, ctx, store, workspace.ID)

	notificationService := serviceapp.NewNotificationService(store, nil, logger.NewNop())
	handler := NewNotificationCommandHandler(notificationService, logger.NewNop())

	event := sharedevents.NewSendNotificationRequestedEvent(
		workspace.ID,
		"knowledge_service",
		"in_app",
		[]string{userOne.ID, userTwo.ID, userOne.ID},
	)
	event.Subject = "New RFC: Search revamp"
	event.Body = "A new RFC is ready for team review."
	event.SourceType = "knowledge_review"
	event.SourceID = "kr_123"
	event.Data = map[string]interface{}{
		"action_url":   "/knowledge/kr_123",
		"knowledge_id": "kr_123",
	}

	payload, err := json.Marshal(event)
	require.NoError(t, err)
	require.NoError(t, handler.HandleSendNotificationRequested(ctx, payload))

	userOneNotifications, err := store.Notifications().ListUserNotifications(ctx, workspace.ID, userOne.ID)
	require.NoError(t, err)
	require.Len(t, userOneNotifications, 1)
	assert.Equal(t, event.Subject, userOneNotifications[0].Subject)
	assert.Equal(t, shareddomain.NotificationTypeInApp, userOneNotifications[0].Type)
	assert.Equal(t, "/knowledge/kr_123", userOneNotifications[0].ActionURL)
	assert.JSONEq(t, `{"action_url":"/knowledge/kr_123","knowledge_id":"kr_123"}`, string(userOneNotifications[0].TrackingData))

	userTwoNotifications, err := store.Notifications().ListUserNotifications(ctx, workspace.ID, userTwo.ID)
	require.NoError(t, err)
	require.Len(t, userTwoNotifications, 1)
	assert.Equal(t, event.SourceID, userTwoNotifications[0].EntityID)
}

func TestNotificationCommandHandler_HandleSendNotificationRequestedBridgesEmailNotifications(t *testing.T) {
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

	notificationService := serviceapp.NewNotificationService(store, emailService, logger.NewNop())
	handler := NewNotificationCommandHandler(notificationService, logger.NewNop())

	event := sharedevents.NewSendNotificationRequestedEvent(
		workspace.ID,
		"workflow_service",
		"email",
		[]string{"ops@example.com"},
	)
	event.Subject = "Workflow alert"
	event.Body = "A workflow requires attention."
	event.Template = "workflow-alert"
	event.SourceType = "rule"
	event.SourceID = "rule_123"
	event.Data = map[string]interface{}{
		"severity": "high",
	}

	payload, err := json.Marshal(event)
	require.NoError(t, err)
	require.NoError(t, handler.HandleSendNotificationRequested(ctx, payload))

	sent := mockProvider.GetSentEmails()
	require.Len(t, sent, 1)
	assert.Equal(t, event.Subject, sent[0].Subject)
	assert.Equal(t, []string{"ops@example.com"}, sent[0].ToEmails)

	stored, err := store.OutboundEmails().GetOutboundEmailByProviderMessageID(ctx, sent[0].ProviderMessageID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.EmailStatusSent, stored.Status)
	assert.Equal(t, "notification", stored.Category)
	assert.Equal(t, "workflow-alert", stored.TemplateData["notification_template"])
}

func TestNotificationCommandWorkflow_FailureLeavesOutboxStateVisible(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	validUser := createWorkspaceMember(t, ctx, store, workspace.ID)
	runtime := workflowruntime.NewHarness(t, store)

	notificationService := serviceapp.NewNotificationService(store, nil, logger.NewNop())
	handler := NewNotificationCommandHandler(notificationService, logger.NewNop())
	require.NoError(t, handler.RegisterHandlers(runtime.EventBus.Subscribe))

	event := sharedevents.NewSendNotificationRequestedEvent(
		workspace.ID,
		"knowledge_service",
		"in_app",
		[]string{"missing-workspace-member"},
	)
	event.Subject = "Review required"
	event.Body = "A review is ready."
	event.SourceType = "knowledge_review"
	event.SourceID = "kr_failure"
	require.NoError(t, runtime.Outbox.PublishEvent(ctx, eventbus.StreamNotificationCommands, event))

	pendingEvents, err := store.Outbox().GetPendingOutboxEvents(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pendingEvents, 1)
	eventID := pendingEvents[0].ID

	require.False(t, runtime.Outbox.ProcessPendingEvent(ctx, pendingEvents[0]))

	var outboxEvent *storeshared.OutboxEvent
	outboxEvent, err = store.Outbox().GetOutboxEvent(ctx, eventID)
	require.NoError(t, err)
	require.Equal(t, "pending", outboxEvent.Status)
	require.Equal(t, 1, outboxEvent.Attempts)
	require.NotNil(t, outboxEvent.NextRetry)

	notifications, err := store.Notifications().ListUserNotifications(ctx, workspace.ID, validUser.ID)
	require.NoError(t, err)
	assert.Len(t, notifications, 0)
	assert.Contains(t, outboxEvent.LastError, "is not a member of workspace")

	workflowproof.WriteJSON(t, "notification-command-failure-visible", map[string]interface{}{
		"workspace_id":      workspace.ID,
		"outbox_event_id":   outboxEvent.ID,
		"outbox_status":     outboxEvent.Status,
		"outbox_attempts":   outboxEvent.Attempts,
		"outbox_next_retry": outboxEvent.NextRetry,
		"outbox_last_error": outboxEvent.LastError,
		"recipient_id":      "missing-workspace-member",
	})
}

func createWorkspaceMember(t *testing.T, ctx context.Context, store stores.Store, workspaceID string) *platformdomain.User {
	t.Helper()

	user := testutil.NewIsolatedUser(t, workspaceID)
	require.NoError(t, store.Users().CreateUser(ctx, user))
	require.NoError(t, store.Workspaces().CreateUserWorkspaceRole(ctx, &platformdomain.UserWorkspaceRole{
		ID:          id.New(),
		UserID:      user.ID,
		WorkspaceID: workspaceID,
		Role:        platformdomain.WorkspaceRoleMember,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}))
	return user
}
