//go:build integration

package container

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/pkg/eventbus"
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

	require.NoError(t, c.Outbox.PublishEvent(ctx, eventbus.StreamEmailCommands, emailEvent))
	require.NoError(t, c.Outbox.PublishEvent(ctx, eventbus.StreamNotificationCommands, notificationEvent))

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
}
