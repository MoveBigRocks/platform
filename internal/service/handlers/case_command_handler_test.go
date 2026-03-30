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
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil/workflowproof"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil/workflowruntime"
	"github.com/movebigrocks/platform/pkg/logger"
)

func TestCaseCommandWorkflow_CreateCaseThroughWorkerPath(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	queue := servicedomain.NewQueue(workspace.ID, "Command Queue", "command-queue", "Queue for command-created cases")
	require.NoError(t, store.Queues().CreateQueue(ctx, queue))

	runtime := workflowruntime.NewHarness(t, store)
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		runtime.Outbox,
		serviceapp.WithTransactionRunner(store),
		serviceapp.WithQueueItemStore(store.QueueItems()),
	)
	handler := NewCaseCommandHandler(caseService, logger.NewNop())
	require.NoError(t, handler.RegisterHandlers(runtime.EventBus.Subscribe))

	responseEvents := make(chan sharedevents.CaseCreatedFromCommandEvent, 1)
	require.NoError(t, runtime.EventBus.Subscribe(eventbus.StreamCaseEvents, "workflow-proof", "case-command-observer", func(_ context.Context, data []byte) error {
		var event sharedevents.CaseCreatedFromCommandEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil
		}
		if event.EventType != eventbus.TypeCaseCreatedFromCommand {
			return nil
		}
		select {
		case responseEvents <- event:
		default:
		}
		return nil
	}))

	event := sharedevents.NewCreateCaseRequestedEvent(
		workspace.ID,
		"sales-pipeline-extension",
		"Enterprise renewal flagged",
		"buyer@example.com",
	)
	event.Description = "Extension escalation requests a human follow-up on an at-risk renewal."
	event.Priority = string(servicedomain.CasePriorityUrgent)
	event.Channel = string(servicedomain.CaseChannelChat)
	event.QueueID = queue.ID
	event.ContactName = "Avery Buyer"
	event.ContactPhone = "+31201234567"
	event.Category = "renewal-risk"
	event.Tags = []string{"expansion", "renewal"}
	event.SourceType = "extension"
	event.SourceID = "sales-pipeline"
	event.Metadata = map[string]interface{}{
		"journey_stage": "renewal",
		"signal_score":  94,
	}
	event.CustomFields = map[string]interface{}{
		"account_id": "acct_123",
		"playbook":   "renewal-watch",
	}
	require.NoError(t, runtime.Outbox.PublishEvent(ctx, eventbus.StreamCaseCommands, event))

	runtime.Start(t)

	var responseEvent sharedevents.CaseCreatedFromCommandEvent
	require.Eventually(t, func() bool {
		select {
		case responseEvent = <-responseEvents:
			return responseEvent.RequestID == event.EventID
		default:
			return false
		}
	}, 2*time.Second, 25*time.Millisecond)

	caseObj, err := store.Cases().GetCase(ctx, responseEvent.CaseID)
	require.NoError(t, err)
	assert.Equal(t, workspace.ID, caseObj.WorkspaceID)
	assert.Equal(t, queue.ID, caseObj.QueueID)
	assert.Equal(t, servicedomain.CasePriorityUrgent, caseObj.Priority)
	assert.Equal(t, servicedomain.CaseChannelChat, caseObj.Channel)
	assert.Equal(t, "buyer@example.com", caseObj.ContactEmail)
	assert.Equal(t, "Avery Buyer", caseObj.ContactName)
	assert.Equal(t, "renewal-risk", caseObj.Category)
	assert.ElementsMatch(t, []string{"expansion", "renewal"}, caseObj.Tags)

	queueItem, err := store.QueueItems().GetQueueItemByCaseID(ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, queue.ID, queueItem.QueueID)

	accountID, ok := caseObj.CustomFields.GetString("account_id")
	require.True(t, ok)
	assert.Equal(t, "acct_123", accountID)
	playbook, ok := caseObj.CustomFields.GetString("playbook")
	require.True(t, ok)
	assert.Equal(t, "renewal-watch", playbook)
	source, ok := caseObj.CustomFields.GetString("source")
	require.True(t, ok)
	assert.Equal(t, "extension_command", source)
	sourceType, ok := caseObj.CustomFields.GetString("source_type")
	require.True(t, ok)
	assert.Equal(t, "extension", sourceType)
	commandRequestID, ok := caseObj.CustomFields.GetString("command_request_id")
	require.True(t, ok)
	assert.Equal(t, event.EventID, commandRequestID)
	commandRequestedBy, ok := caseObj.CustomFields.GetString("command_requested_by")
	require.True(t, ok)
	assert.Equal(t, "sales-pipeline-extension", commandRequestedBy)
	commandQueueID, ok := caseObj.CustomFields.GetString("command_queue_id")
	require.True(t, ok)
	assert.Equal(t, queue.ID, commandQueueID)
	sourceID, ok := caseObj.CustomFields.GetString("source_id")
	require.True(t, ok)
	assert.Equal(t, "sales-pipeline", sourceID)
	phone, ok := caseObj.CustomFields.GetString("contact_phone")
	require.True(t, ok)
	assert.Equal(t, "+31201234567", phone)
	journeyStage, ok := caseObj.CustomFields.GetString("metadata_journey_stage")
	require.True(t, ok)
	assert.Equal(t, "renewal", journeyStage)
	signalScore, ok := caseObj.CustomFields.GetInt("metadata_signal_score")
	require.True(t, ok)
	assert.EqualValues(t, 94, signalScore)

	workflowproof.WriteJSON(t, "case-command-create", map[string]interface{}{
		"request_event_id":       event.EventID,
		"response_event_id":      responseEvent.EventID,
		"workspace_id":           workspace.ID,
		"case_id":                caseObj.ID,
		"case_human_id":          caseObj.HumanID,
		"queue_id":               queue.ID,
		"queue_item_id":          queueItem.ID,
		"priority":               caseObj.Priority,
		"channel":                caseObj.Channel,
		"source":                 source,
		"source_type":            sourceType,
		"source_id":              sourceID,
		"command_requested_by":   commandRequestedBy,
		"metadata_journey_stage": journeyStage,
		"metadata_signal_score":  signalScore,
	})
}

func TestCaseCommandWorkflow_FailureLeavesOutboxStateVisible(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	runtime := workflowruntime.NewHarness(t, store)
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		runtime.Outbox,
		serviceapp.WithTransactionRunner(store),
		serviceapp.WithQueueItemStore(store.QueueItems()),
	)
	handler := NewCaseCommandHandler(caseService, logger.NewNop())
	require.NoError(t, handler.RegisterHandlers(runtime.EventBus.Subscribe))

	event := sharedevents.NewCreateCaseRequestedEvent(
		workspace.ID,
		"extension-proof",
		"Broken command input",
		"buyer@example.com",
	)
	event.Priority = "critical-plus"
	require.NoError(t, runtime.Outbox.PublishEvent(ctx, eventbus.StreamCaseCommands, event))

	pendingEvents, err := store.Outbox().GetPendingOutboxEvents(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pendingEvents, 1)
	eventID := pendingEvents[0].ID

	require.False(t, runtime.Outbox.ProcessPendingEvent(ctx, pendingEvents[0]))

	outboxEvent, err := store.Outbox().GetOutboxEvent(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, "pending", outboxEvent.Status)
	assert.Equal(t, 1, outboxEvent.Attempts)
	assert.NotNil(t, outboxEvent.NextRetry)
	assert.Contains(t, outboxEvent.LastError, "priority")

	cases, total, err := store.Cases().ListCases(ctx, contracts.CaseFilters{
		WorkspaceID: workspace.ID,
		Limit:       10,
	})
	require.NoError(t, err)
	assert.Len(t, cases, 0)
	assert.Equal(t, 0, total)

	workflowproof.WriteJSON(t, "case-command-failure-visible", map[string]interface{}{
		"workspace_id":      workspace.ID,
		"request_event_id":  event.EventID,
		"outbox_event_id":   outboxEvent.ID,
		"outbox_status":     outboxEvent.Status,
		"outbox_attempts":   outboxEvent.Attempts,
		"outbox_next_retry": outboxEvent.NextRetry,
		"outbox_last_error": outboxEvent.LastError,
	})
}
