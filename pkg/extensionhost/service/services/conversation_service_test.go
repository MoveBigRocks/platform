package serviceapp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

func TestConversationService_HandoffConversationRecordsDelegatedActorContext(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	workspaceID := setupTestWorkspace(t, store, "conversation-handoff")
	ctx := context.Background()

	queue := servicedomain.NewQueue(workspaceID, "Support Inbox", "support-inbox", "")
	require.NoError(t, store.Queues().CreateQueue(ctx, queue))

	session := servicedomain.NewConversationSession(workspaceID, servicedomain.ConversationChannelWebChat)
	require.NoError(t, store.Conversations().CreateConversationSession(ctx, session))

	caseSvc := NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil, WithQueueItemStore(store.QueueItems()))
	conversationSvc := NewConversationService(store.Conversations(), store.Queues(), store.QueueItems(), store.Workspaces(), caseSvc, nil)

	operatorUserID := id.New()
	agentID := id.New()
	ownerUserID := id.New()

	updated, err := conversationSvc.HandoffConversation(ctx, session.ID, HandoffConversationParams{
		QueueID:          queue.ID,
		OperatorUserID:   operatorUserID,
		Reason:           "needs operator follow-up",
		PerformedByID:    agentID,
		PerformedByName:  "Claude Agent",
		PerformedByType:  "agent",
		OnBehalfOfUserID: ownerUserID,
	})
	require.NoError(t, err)
	assert.Equal(t, servicedomain.ConversationStatusWaiting, updated.Status)
	assert.Equal(t, "", updated.HandlingTeamID)
	assert.Equal(t, operatorUserID, updated.AssignedOperatorUserID)

	queueItem, err := store.QueueItems().GetQueueItemByConversationSessionID(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, queue.ID, queueItem.QueueID)

	outcomes, err := store.Conversations().ListConversationOutcomes(ctx, session.ID)
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	assert.Equal(t, servicedomain.ConversationOutcomeKindHandedToOperator, outcomes[0].Kind)
	assert.Equal(t, "", outcomes[0].ResultRef.GetString("team_id"))
	assert.Equal(t, queue.ID, outcomes[0].ResultRef.GetString("queue_id"))
	assert.Equal(t, agentID, outcomes[0].ResultRef.GetString("performed_by_id"))
	assert.Equal(t, "Claude Agent", outcomes[0].ResultRef.GetString("performed_by_name"))
	assert.Equal(t, "agent", outcomes[0].ResultRef.GetString("performed_by_type"))
	assert.Equal(t, "delegated_agent", outcomes[0].ResultRef.GetString("routing_mode"))
	assert.Equal(t, ownerUserID, outcomes[0].ResultRef.GetString("on_behalf_of_user_id"))
	assert.Equal(t, "needs operator follow-up", outcomes[0].ResultRef.GetString("reason"))
}
