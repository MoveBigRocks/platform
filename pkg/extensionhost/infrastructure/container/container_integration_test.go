//go:build integration

package container

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/pkg/eventbus"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
	"github.com/movebigrocks/platform/pkg/id"
)

func TestContainer_StartRegistersScopedWorkflowConsumersAndProcessesPendingCommands(t *testing.T) {
	testutil.SetupTestEnv(t)

	cfg := testutil.NewTestConfig(t)
	cfg.Outbox.PollInterval = 10 * time.Millisecond

	c, err := New(cfg, Options{})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, c.Stop(2*time.Second))
	})

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, c.Store.Workspaces().CreateWorkspace(ctx, workspace))

	user := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, c.Store.Users().CreateUser(ctx, user))
	require.NoError(t, c.Store.Workspaces().CreateUserWorkspaceRole(ctx, &platformdomain.UserWorkspaceRole{
		ID:          id.New(),
		UserID:      user.ID,
		WorkspaceID: workspace.ID,
		Role:        platformdomain.WorkspaceRoleMember,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}))
	queue := servicedomain.NewQueue(workspace.ID, "Command Intake", "command-intake", "Command-created cases land here")
	require.NoError(t, c.Store.Queues().CreateQueue(ctx, queue))

	outbound := servicedomain.NewOutboundEmail(workspace.ID, "support@movebigrocks.test", "Container startup proof", "Pending commands should be processed after startup.")
	outbound.FromName = "Support"
	outbound.ToEmails = []string{"customer@example.com"}
	outbound.Category = "container-proof"
	require.NoError(t, c.Store.OutboundEmails().CreateOutboundEmail(ctx, outbound))

	emailEvent := sharedevents.NewSendEmailRequestedEvent(
		workspace.ID,
		"container_test",
		[]string{"customer@example.com"},
		outbound.Subject,
		outbound.TextContent,
	)
	emailEvent.OutboundEmailID = outbound.ID
	emailEvent.Category = outbound.Category

	notificationEvent := sharedevents.NewSendNotificationRequestedEvent(
		workspace.ID,
		"container_test",
		"in_app",
		[]string{user.ID},
	)
	notificationEvent.Subject = "Container startup notification"
	notificationEvent.Body = "Scoped workflow commands should be processed after startup."
	notificationEvent.SourceType = "knowledge_review"
	notificationEvent.SourceID = "container-start"

	caseEvent := sharedevents.NewCreateCaseRequestedEvent(
		workspace.ID,
		"container_test",
		"Container startup case command",
		"customer@example.com",
	)
	caseEvent.Description = "Pending case commands should be processed after startup."
	caseEvent.Priority = string(servicedomain.CasePriorityHigh)
	caseEvent.Channel = string(servicedomain.CaseChannelAPI)
	caseEvent.QueueID = queue.ID
	caseEvent.ContactName = "Container Customer"
	caseEvent.SourceType = "agent"
	caseEvent.SourceID = "container-start"
	caseEvent.Metadata = map[string]interface{}{
		"scenario": "startup-proof",
	}

	caseCreatedEvents := make(chan sharedevents.CaseCreatedFromCommandEvent, 1)
	require.NoError(t, c.EventBus.Subscribe(eventbus.StreamCaseEvents, "container-proof", "case-created-listener", func(_ context.Context, data []byte) error {
		var event sharedevents.CaseCreatedFromCommandEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil
		}
		if event.RequestID != caseEvent.EventID {
			return nil
		}
		select {
		case caseCreatedEvents <- event:
		default:
		}
		return nil
	}))

	require.NoError(t, c.Outbox.PublishEvent(ctx, eventbus.StreamEmailCommands, emailEvent))
	require.NoError(t, c.Outbox.PublishEvent(ctx, eventbus.StreamNotificationCommands, notificationEvent))
	require.NoError(t, c.Outbox.PublishEvent(ctx, eventbus.StreamCaseCommands, caseEvent))

	require.NoError(t, c.Start(ctx))

	_, emailCommandGroups, err := c.EventBus.GetStreamInfo(eventbus.StreamEmailCommands)
	require.NoError(t, err)
	assert.Equal(t, int64(1), emailCommandGroups)

	_, emailEventGroups, err := c.EventBus.GetStreamInfo(eventbus.StreamEmailEvents)
	require.NoError(t, err)
	assert.Equal(t, int64(1), emailEventGroups)

	_, formEventGroups, err := c.EventBus.GetStreamInfo(eventbus.StreamFormEvents)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, formEventGroups, int64(1))

	_, notificationCommandGroups, err := c.EventBus.GetStreamInfo(eventbus.StreamNotificationCommands)
	require.NoError(t, err)
	assert.Equal(t, int64(1), notificationCommandGroups)

	_, caseCommandGroups, err := c.EventBus.GetStreamInfo(eventbus.StreamCaseCommands)
	require.NoError(t, err)
	assert.Equal(t, int64(1), caseCommandGroups)

	require.Eventually(t, func() bool {
		storedOutbound, lookupErr := c.Store.OutboundEmails().GetOutboundEmail(ctx, outbound.ID)
		require.NoError(t, lookupErr)
		return storedOutbound.Status == servicedomain.EmailStatusSent && storedOutbound.ProviderMessageID != ""
	}, 2*time.Second, 25*time.Millisecond)

	require.Eventually(t, func() bool {
		notifications, lookupErr := c.Store.Notifications().ListUserNotifications(ctx, workspace.ID, user.ID)
		require.NoError(t, lookupErr)
		return len(notifications) == 1
	}, 2*time.Second, 25*time.Millisecond)

	var responseEvent sharedevents.CaseCreatedFromCommandEvent
	require.Eventually(t, func() bool {
		select {
		case responseEvent = <-caseCreatedEvents:
			return true
		default:
			return false
		}
	}, 2*time.Second, 25*time.Millisecond)

	caseObj, err := c.Store.Cases().GetCase(ctx, responseEvent.CaseID)
	require.NoError(t, err)
	assert.Equal(t, workspace.ID, caseObj.WorkspaceID)
	assert.Equal(t, queue.ID, caseObj.QueueID)
	assert.Equal(t, servicedomain.CasePriorityHigh, caseObj.Priority)
	assert.Equal(t, servicedomain.CaseChannelAPI, caseObj.Channel)
	assert.Equal(t, "customer@example.com", caseObj.ContactEmail)

	queueItem, err := c.Store.QueueItems().GetQueueItemByCaseID(ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, queue.ID, queueItem.QueueID)
}
