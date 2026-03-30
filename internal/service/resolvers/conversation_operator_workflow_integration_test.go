//go:build integration

package resolvers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	graphmodel "github.com/movebigrocks/platform/internal/graph/model"
	graphshared "github.com/movebigrocks/platform/pkg/extensionhost/graph/shared"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil/workflowproof"
)

func TestConversationOperatorWorkflow_ReplyHandoffAndEscalate(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	workspace.Name = "Conversation Ops"
	workspace.Slug = "conversation-ops-" + workspace.Slug
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	operator := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, operator))
	targetOperator := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, targetOperator))

	sourceQueue := servicedomain.NewQueue(workspace.ID, "Web Chat Inbox", "web-chat-inbox", "Incoming web chat")
	require.NoError(t, store.Queues().CreateQueue(ctx, sourceQueue))
	targetQueue := servicedomain.NewQueue(workspace.ID, "Billing Specialists", "billing-specialists", "Billing handoff")
	require.NoError(t, store.Queues().CreateQueue(ctx, targetQueue))

	targetTeam := &platformdomain.Team{
		ID:          testutil.UniqueUserID(t),
		WorkspaceID: workspace.ID,
		Name:        "Billing",
		Description: "Billing operators",
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.Workspaces().CreateTeam(ctx, targetTeam))

	agent := platformdomain.NewAgent(workspace.ID, "conversation-bot", "Guides conversations", operator.ID, operator.ID)
	agent.ID = testutil.UniqueUserID(t)
	require.NoError(t, store.Agents().CreateAgent(ctx, agent))

	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		nil,
		serviceapp.WithTransactionRunner(store),
		serviceapp.WithQueueItemStore(store.QueueItems()),
	)
	conversationService := serviceapp.NewConversationService(
		store.Conversations(),
		store.Queues(),
		store.QueueItems(),
		store.Workspaces(),
		caseService,
		store,
	)
	resolver := NewResolver(Config{
		ConversationService: conversationService,
		CaseService:         caseService,
	})

	started, err := conversationService.StartPublicConversation(ctx, serviceapp.StartPublicConversationParams{
		WorkspaceSlug:  workspace.Slug,
		QueueSlug:      sourceQueue.Slug,
		DisplayName:    "Casey Customer",
		ContactEmail:   "casey@example.com",
		InitialMessage: "I need help with a billing issue.",
		SourceRef:      "widget:website",
	})
	require.NoError(t, err)

	sessionID := started.Session.ID
	initialQueueItem, err := store.QueueItems().GetQueueItemByConversationSessionID(ctx, sessionID)
	require.NoError(t, err)
	require.Equal(t, sourceQueue.ID, initialQueueItem.QueueID)

	agentCtx := graphshared.SetAuthContext(ctx, &platformdomain.AuthContext{
		Principal:     agent,
		PrincipalType: platformdomain.PrincipalTypeAgent,
		WorkspaceID:   workspace.ID,
		WorkspaceIDs:  []string{workspace.ID},
		Membership: &platformdomain.WorkspaceMembership{
			WorkspaceID:   workspace.ID,
			PrincipalID:   agent.ID,
			PrincipalType: platformdomain.PrincipalTypeAgent,
			Constraints: platformdomain.MembershipConstraints{
				AllowDelegatedRouting:   true,
				DelegatedRoutingTeamIDs: []string{targetTeam.ID},
				AllowedTeamIDs:          []string{targetTeam.ID},
			},
		},
		Permissions: []string{
			platformdomain.PermissionConversationRead,
			platformdomain.PermissionConversationWrite,
			platformdomain.PermissionCaseRead,
			platformdomain.PermissionCaseWrite,
		},
	})
	humanCtx := graphshared.SetAuthContext(ctx, &platformdomain.AuthContext{
		Principal:     operator,
		PrincipalType: platformdomain.PrincipalTypeUser,
		WorkspaceID:   workspace.ID,
		WorkspaceIDs:  []string{workspace.ID},
		Permissions: []string{
			platformdomain.PermissionConversationRead,
			platformdomain.PermissionConversationWrite,
			platformdomain.PermissionCaseRead,
			platformdomain.PermissionCaseWrite,
		},
	})

	replyRole := "assistant"
	replyVisibility := "customer"
	replyText := "I can help gather the billing details before escalation."
	reply, err := resolver.AddConversationMessage(agentCtx, sessionID, graphmodel.AddConversationMessageInput{
		Role:        &replyRole,
		Visibility:  &replyVisibility,
		ContentText: &replyText,
	})
	require.NoError(t, err)
	require.NotNil(t, reply.ParticipantID())

	participants, err := store.Conversations().ListConversationParticipants(ctx, sessionID)
	require.NoError(t, err)
	var agentParticipant *servicedomain.ConversationParticipant
	for _, participant := range participants {
		if participant.ParticipantKind == servicedomain.ConversationParticipantKindAgent {
			agentParticipant = participant
			break
		}
	}
	require.NotNil(t, agentParticipant)
	assert.Equal(t, agent.ID, agentParticipant.ParticipantRef)
	assert.Equal(t, agent.Name, agentParticipant.DisplayName)

	workflowproof.WriteJSON(t, "conversation-operator-reply", map[string]any{
		"workspace_id":         workspace.ID,
		"session_id":           sessionID,
		"reply_message_id":     string(reply.ID()),
		"reply_participant_id": agentParticipant.ID,
		"participant_kind":     agentParticipant.ParticipantKind,
		"participant_ref":      agentParticipant.ParticipantRef,
		"visibility":           reply.Visibility(),
		"role":                 reply.Role(),
		"queue_id":             initialQueueItem.QueueID,
	})

	handoffReason := "billing specialist required"
	handoff, err := resolver.HandoffConversation(humanCtx, sessionID, graphmodel.ConversationHandoffInput{
		TeamID:         stringPtr(targetTeam.ID),
		QueueID:        targetQueue.ID,
		OperatorUserID: stringPtr(targetOperator.ID),
		Reason:         stringPtr(handoffReason),
	})
	require.NoError(t, err)
	require.Equal(t, targetTeam.ID, string(*handoff.HandlingTeamID()))
	require.Equal(t, targetOperator.ID, string(*handoff.AssignedOperatorUserID()))

	handoffQueueItem, err := store.QueueItems().GetQueueItemByConversationSessionID(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, targetQueue.ID, handoffQueueItem.QueueID)

	outcomes, err := store.Conversations().ListConversationOutcomes(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	assert.Equal(t, servicedomain.ConversationOutcomeKindHandedToOperator, outcomes[0].Kind)
	assert.Equal(t, operator.ID, outcomes[0].ResultRef.GetString("performed_by_id"))
	assert.Equal(t, "user", outcomes[0].ResultRef.GetString("performed_by_type"))
	assert.Equal(t, handoffReason, outcomes[0].ResultRef.GetString("reason"))

	workflowproof.WriteJSON(t, "conversation-operator-handoff", map[string]any{
		"workspace_id":      workspace.ID,
		"session_id":        sessionID,
		"target_queue_id":   handoffQueueItem.QueueID,
		"target_team_id":    outcomes[0].ResultRef.GetString("team_id"),
		"target_user_id":    outcomes[0].ResultRef.GetString("operator_user_id"),
		"performed_by_id":   outcomes[0].ResultRef.GetString("performed_by_id"),
		"performed_by_type": outcomes[0].ResultRef.GetString("performed_by_type"),
		"reason":            outcomes[0].ResultRef.GetString("reason"),
	})

	escalationReason := "customer needs manual refund review"
	escalatedCase, err := resolver.EscalateConversation(humanCtx, sessionID, graphmodel.EscalateConversationInput{
		TeamID:         stringPtr(targetTeam.ID),
		QueueID:        targetQueue.ID,
		OperatorUserID: stringPtr(targetOperator.ID),
		Subject:        stringPtr("Refund review needed"),
		Description:    stringPtr("Escalated from a supervised public conversation"),
		Priority:       stringPtr("high"),
		Category:       stringPtr("billing"),
		Reason:         stringPtr(escalationReason),
	})
	require.NoError(t, err)

	caseID := string(escalatedCase.ID())
	storedCase, err := store.Cases().GetCase(ctx, caseID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, storedCase.OriginatingConversationID)
	assert.Equal(t, targetQueue.ID, storedCase.QueueID)
	assert.Equal(t, targetTeam.ID, storedCase.TeamID)
	assert.Equal(t, targetOperator.ID, storedCase.AssignedToID)

	session, err := store.Conversations().GetConversationSession(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.ConversationStatusEscalated, session.Status)
	assert.Equal(t, caseID, session.LinkedCaseID)

	_, err = store.QueueItems().GetQueueItemByConversationSessionID(ctx, sessionID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, shared.ErrNotFound))

	caseQueueItem, err := store.QueueItems().GetQueueItemByCaseID(ctx, caseID)
	require.NoError(t, err)
	assert.Equal(t, targetQueue.ID, caseQueueItem.QueueID)

	finalOutcomes, err := store.Conversations().ListConversationOutcomes(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, finalOutcomes, 2)
	caseOutcome := finalOutcomes[1]
	assert.Equal(t, servicedomain.ConversationOutcomeKindCaseCreated, caseOutcome.Kind)
	assert.Equal(t, caseID, caseOutcome.ResultRef.GetString("case_id"))

	workflowproof.WriteJSON(t, "conversation-operator-escalation", map[string]any{
		"workspace_id":             workspace.ID,
		"session_id":               sessionID,
		"conversation_status":      session.Status,
		"linked_case_id":           session.LinkedCaseID,
		"case_queue_id":            caseQueueItem.QueueID,
		"case_id":                  caseID,
		"case_origin_conversation": storedCase.OriginatingConversationID,
		"performed_by_id":          caseOutcome.ResultRef.GetString("performed_by_id"),
		"performed_by_type":        caseOutcome.ResultRef.GetString("performed_by_type"),
		"reason":                   caseOutcome.ResultRef.GetString("reason"),
	})
}
