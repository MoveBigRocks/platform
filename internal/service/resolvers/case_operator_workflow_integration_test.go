//go:build integration

package resolvers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	graphmodel "github.com/movebigrocks/platform/internal/graph/model"
	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	servicehandlers "github.com/movebigrocks/platform/internal/service/handlers"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/workflowproof"
	"github.com/movebigrocks/platform/internal/testutil/workflowruntime"
	"github.com/movebigrocks/platform/pkg/logger"
)

func TestCaseOperatorWorkflow_CreateManageAndReply(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	owner := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, owner))
	assignee := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, assignee))

	queue := servicedomain.NewQueue(workspace.ID, "Triage", "triage", "Primary triage queue")
	require.NoError(t, store.Queues().CreateQueue(ctx, queue))

	agent := platformdomain.NewAgent(workspace.ID, "triage-bot", "Assists human operators", owner.ID, owner.ID)
	agent.ID = testutil.UniqueUserID(t)
	require.NoError(t, store.Agents().CreateAgent(ctx, agent))

	runtime := workflowruntime.NewHarness(t, store)
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		runtime.Outbox,
		serviceapp.WithTransactionRunner(store),
		serviceapp.WithQueueItemStore(store.QueueItems()),
		serviceapp.WithOutboundEmailStore(store.OutboundEmails()),
		serviceapp.WithUserStore(store.Users()),
	)
	queueService := serviceapp.NewQueueService(store.Queues(), store.QueueItems(), store.Workspaces())
	userService := platformservices.NewUserManagementService(store.Users(), store.Workspaces())

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

	emailHandler := servicehandlers.NewEmailCommandHandler(emailService, logger.NewNop())
	require.NoError(t, emailHandler.RegisterHandlers(runtime.EventBus.Subscribe))
	runtime.Start(t)

	resolver := NewResolver(Config{
		CaseService:  caseService,
		QueueService: queueService,
		UserService:  userService,
	})

	humanCtx := graphshared.SetAuthContext(ctx, &platformdomain.AuthContext{
		Principal:     owner,
		PrincipalType: platformdomain.PrincipalTypeUser,
		WorkspaceID:   workspace.ID,
		WorkspaceIDs:  []string{workspace.ID},
		Permissions: []string{
			platformdomain.PermissionCaseRead,
			platformdomain.PermissionCaseWrite,
		},
	})

	created, err := resolver.CreateCase(humanCtx, graphmodel.CreateCaseInput{
		WorkspaceID:  workspace.ID,
		Subject:      "Manual follow-up",
		Description:  stringPtr("Customer requested manual review"),
		Priority:     stringPtr("high"),
		QueueID:      stringPtr(queue.ID),
		ContactEmail: stringPtr("customer@example.com"),
		ContactName:  stringPtr("Casey Customer"),
		Category:     stringPtr("billing"),
	})
	require.NoError(t, err)

	caseID := string(created.ID())
	queueItem, err := store.QueueItems().GetQueueItemByCaseID(ctx, caseID)
	require.NoError(t, err)
	require.Equal(t, queue.ID, queueItem.QueueID)

	storedCase, err := store.Cases().GetCase(ctx, caseID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CasePriorityHigh, storedCase.Priority)
	assert.Equal(t, servicedomain.CaseChannelAPI, storedCase.Channel)
	assert.Equal(t, "customer@example.com", storedCase.ContactEmail)

	workflowproof.WriteJSON(t, "case-operator-manual-create", map[string]interface{}{
		"workspace_id":  workspace.ID,
		"case_id":       storedCase.ID,
		"case_human_id": storedCase.HumanID,
		"queue_id":      queueItem.QueueID,
		"priority":      storedCase.Priority,
		"channel":       storedCase.Channel,
		"contact_email": storedCase.ContactEmail,
	})

	assigned, err := resolver.AssignCase(humanCtx, caseID, &assignee.ID)
	require.NoError(t, err)
	require.Equal(t, assignee.ID, string(*assigned.AssigneeID()))

	unassigned, err := resolver.AssignCase(humanCtx, caseID, nil)
	require.NoError(t, err)
	require.Nil(t, unassigned.AssigneeID())

	reassigned, err := resolver.AssignCase(humanCtx, caseID, &assignee.ID)
	require.NoError(t, err)
	require.Equal(t, assignee.ID, string(*reassigned.AssigneeID()))

	reprioritized, err := resolver.SetCasePriority(humanCtx, caseID, "urgent")
	require.NoError(t, err)
	require.Equal(t, "urgent", reprioritized.Priority())

	agentCtx := graphshared.SetAuthContext(ctx, &platformdomain.AuthContext{
		Principal:     agent,
		PrincipalType: platformdomain.PrincipalTypeAgent,
		WorkspaceID:   workspace.ID,
		WorkspaceIDs:  []string{workspace.ID},
		Permissions: []string{
			platformdomain.PermissionCaseRead,
			platformdomain.PermissionCaseWrite,
		},
	})

	note, err := resolver.AddCaseNote(agentCtx, caseID, "Escalating billing context for manual review.")
	require.NoError(t, err)
	require.NotNil(t, note.FromAgentID())
	require.Nil(t, note.FromUserID())
	require.True(t, note.IsInternal())

	reply, err := resolver.ReplyToCase(agentCtx, caseID, graphmodel.ReplyToCaseInput{
		Body:     "We reviewed the request and applied the requested billing change.",
		ToEmails: &[]string{"customer@example.com"},
	})
	require.NoError(t, err)
	require.NotNil(t, reply.FromAgentID())
	require.False(t, reply.IsInternal())

	var outbound *servicedomain.OutboundEmail
	require.Eventually(t, func() bool {
		emails := mockProvider.GetSentEmails()
		if len(emails) != 1 {
			return false
		}
		outbound, err = store.OutboundEmails().GetOutboundEmailByProviderMessageID(ctx, emails[0].ProviderMessageID)
		require.NoError(t, err)
		return outbound.Status == servicedomain.EmailStatusSent
	}, 2*time.Second, 25*time.Millisecond)

	communications, err := store.Cases().ListCaseCommunications(ctx, caseID)
	require.NoError(t, err)
	require.Len(t, communications, 2)
	communicationsByID := map[string]*servicedomain.Communication{}
	for _, comm := range communications {
		communicationsByID[comm.ID] = comm
	}
	require.Contains(t, communicationsByID, string(note.ID()))
	require.Contains(t, communicationsByID, string(reply.ID()))
	assert.Equal(t, agent.ID, communicationsByID[string(note.ID())].FromAgentID)
	assert.Equal(t, agent.ID, communicationsByID[string(reply.ID())].FromAgentID)

	workflowproof.WriteJSON(t, "case-operator-work-management", map[string]interface{}{
		"workspace_id":       workspace.ID,
		"case_id":            caseID,
		"assignee_id":        assignee.ID,
		"unassigned":         true,
		"priority":           "urgent",
		"note_id":            string(note.ID()),
		"note_from_agent_id": string(*note.FromAgentID()),
		"queue_id":           queueItem.QueueID,
	})

	workflowproof.WriteJSON(t, "case-operator-reply", map[string]interface{}{
		"workspace_id":        workspace.ID,
		"case_id":             caseID,
		"communication_id":    string(reply.ID()),
		"from_agent_id":       string(*reply.FromAgentID()),
		"sender_user_id":      owner.ID,
		"sender_email":        owner.Email,
		"outbound_email_id":   outbound.ID,
		"provider_message_id": outbound.ProviderMessageID,
		"status":              outbound.Status,
	})
}

func TestCaseOperatorWorkflow_HandoffAndStatusTransitions(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	owner := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, owner))
	initialAssignee := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, initialAssignee))
	targetAssignee := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, targetAssignee))

	sourceQueue := servicedomain.NewQueue(workspace.ID, "Support Inbox", "support-inbox", "Primary support queue")
	require.NoError(t, store.Queues().CreateQueue(ctx, sourceQueue))
	targetQueue := servicedomain.NewQueue(workspace.ID, "Billing Escalations", "billing-escalations", "Billing specialists")
	require.NoError(t, store.Queues().CreateQueue(ctx, targetQueue))
	targetTeam := &platformdomain.Team{
		ID:          testutil.UniqueUserID(t),
		WorkspaceID: workspace.ID,
		Name:        "Billing",
		Description: "Billing specialists",
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.Workspaces().CreateTeam(ctx, targetTeam))

	runtime := workflowruntime.NewHarness(t, store)
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		runtime.Outbox,
		serviceapp.WithTransactionRunner(store),
		serviceapp.WithQueueItemStore(store.QueueItems()),
		serviceapp.WithUserStore(store.Users()),
	)
	queueService := serviceapp.NewQueueService(store.Queues(), store.QueueItems(), store.Workspaces())
	userService := platformservices.NewUserManagementService(store.Users(), store.Workspaces())
	runtime.Start(t)

	resolver := NewResolver(Config{
		CaseService:  caseService,
		QueueService: queueService,
		UserService:  userService,
	})

	humanCtx := graphshared.SetAuthContext(ctx, &platformdomain.AuthContext{
		Principal:     owner,
		PrincipalType: platformdomain.PrincipalTypeUser,
		WorkspaceID:   workspace.ID,
		WorkspaceIDs:  []string{workspace.ID},
		Permissions: []string{
			platformdomain.PermissionCaseRead,
			platformdomain.PermissionCaseWrite,
		},
	})

	created, err := resolver.CreateCase(humanCtx, graphmodel.CreateCaseInput{
		WorkspaceID:  workspace.ID,
		Subject:      "Refund specialist handoff",
		Description:  stringPtr("Customer requested a refund review"),
		Priority:     stringPtr("medium"),
		QueueID:      stringPtr(sourceQueue.ID),
		ContactEmail: stringPtr("refunds@example.com"),
		ContactName:  stringPtr("Riley Refund"),
	})
	require.NoError(t, err)

	caseID := string(created.ID())
	assigned, err := resolver.AssignCase(humanCtx, caseID, &initialAssignee.ID)
	require.NoError(t, err)
	require.NotNil(t, assigned.AssigneeID())
	require.Equal(t, initialAssignee.ID, string(*assigned.AssigneeID()))

	targetTeamID := targetTeam.ID
	handoffReason := "refund specialist required"
	handoff, err := resolver.HandoffCase(humanCtx, caseID, graphmodel.CaseHandoffInput{
		QueueID:    targetQueue.ID,
		TeamID:     &targetTeamID,
		AssigneeID: &targetAssignee.ID,
		Reason:     &handoffReason,
	})
	require.NoError(t, err)
	require.NotNil(t, handoff.QueueID())
	require.Equal(t, targetQueue.ID, string(*handoff.QueueID()))
	require.NotNil(t, handoff.TeamID())
	require.Equal(t, targetTeamID, string(*handoff.TeamID()))
	require.NotNil(t, handoff.AssigneeID())
	require.Equal(t, targetAssignee.ID, string(*handoff.AssigneeID()))

	queueItem, err := store.QueueItems().GetQueueItemByCaseID(ctx, caseID)
	require.NoError(t, err)
	require.Equal(t, targetQueue.ID, queueItem.QueueID)

	storedCase, err := store.Cases().GetCase(ctx, caseID)
	require.NoError(t, err)
	assert.Equal(t, targetQueue.ID, storedCase.QueueID)
	assert.Equal(t, targetTeamID, storedCase.TeamID)
	assert.Equal(t, targetAssignee.ID, storedCase.AssignedToID)

	communications, err := store.Cases().ListCaseCommunications(ctx, caseID)
	require.NoError(t, err)
	require.Len(t, communications, 1)
	assert.Equal(t, owner.ID, communications[0].FromUserID)
	assert.Contains(t, communications[0].Body, "Case handed off.")
	assert.Contains(t, communications[0].Body, "Queue: "+sourceQueue.ID+" -> "+targetQueue.ID)
	assert.Contains(t, communications[0].Body, "Team: none -> "+targetTeamID)
	assert.Contains(t, communications[0].Body, "Assignee: "+initialAssignee.ID+" -> "+targetAssignee.ID)
	assert.Contains(t, communications[0].Body, "Reason: "+handoffReason)

	workflowproof.WriteJSON(t, "case-operator-handoff", map[string]interface{}{
		"workspace_id":             workspace.ID,
		"case_id":                  caseID,
		"source_queue_id":          sourceQueue.ID,
		"target_queue_id":          targetQueue.ID,
		"target_team_id":           targetTeamID,
		"target_assignee_id":       targetAssignee.ID,
		"handoff_communication_id": communications[0].ID,
		"handoff_communication_by": communications[0].FromUserID,
		"handoff_queue_item_queue": queueItem.QueueID,
		"handoff_message_count":    storedCase.MessageCount,
	})

	pending, err := resolver.UpdateCaseStatus(humanCtx, caseID, "pending")
	require.NoError(t, err)
	require.Equal(t, "pending", pending.Status())

	resolved, err := resolver.UpdateCaseStatus(humanCtx, caseID, "resolved")
	require.NoError(t, err)
	require.Equal(t, "resolved", resolved.Status())
	require.NotNil(t, resolved.ResolvedAt())

	closed, err := resolver.UpdateCaseStatus(humanCtx, caseID, "closed")
	require.NoError(t, err)
	require.Equal(t, "closed", closed.Status())

	closedState, err := store.Cases().GetCase(ctx, caseID)
	require.NoError(t, err)
	require.NotNil(t, closedState.ResolvedAt)
	require.NotNil(t, closedState.ClosedAt)

	reopened, err := resolver.UpdateCaseStatus(humanCtx, caseID, "open")
	require.NoError(t, err)
	require.Equal(t, "open", reopened.Status())
	require.Nil(t, reopened.ResolvedAt())

	reopenedState, err := store.Cases().GetCase(ctx, caseID)
	require.NoError(t, err)
	assert.Nil(t, reopenedState.ResolvedAt)
	assert.Nil(t, reopenedState.ClosedAt)
	assert.Equal(t, 1, reopenedState.ReopenCount)

	workflowproof.WriteJSON(t, "case-operator-status-transition", map[string]interface{}{
		"workspace_id":                 workspace.ID,
		"case_id":                      caseID,
		"status_sequence":              []string{"pending", "resolved", "closed", "open"},
		"resolved_timestamp_visible":   resolved.ResolvedAt() != nil,
		"closed_timestamp_persisted":   closedState.ClosedAt != nil,
		"reopened_cleared_resolved_at": reopenedState.ResolvedAt == nil,
		"reopened_cleared_closed_at":   reopenedState.ClosedAt == nil,
		"reopen_count":                 reopenedState.ReopenCount,
	})
}

func stringPtr(value string) *string {
	return &value
}
