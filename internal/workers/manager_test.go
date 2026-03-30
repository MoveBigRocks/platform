package workers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/pkg/eventbus"
	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	"github.com/movebigrocks/platform/pkg/logger"
)

type noopOutboxPublisher struct{}

func (noopOutboxPublisher) PublishEvent(context.Context, eventbus.Stream, eventbus.Event) error {
	return nil
}

func (noopOutboxPublisher) Publish(context.Context, eventbus.Stream, interface{}) error {
	return nil
}

type noopTransactionRunner struct{}

func (noopTransactionRunner) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

var (
	_ contracts.OutboxPublisher   = noopOutboxPublisher{}
	_ contracts.TransactionRunner = noopTransactionRunner{}
)

func TestManager_StartRegistersMilestoneWorkflowStreams(t *testing.T) {
	t.Parallel()

	bus := eventbus.NewInMemoryBus()
	manager := NewManager(ManagerDeps{
		EventBus:            bus,
		Logger:              logger.NewNop(),
		FormService:         &automationservices.FormService{},
		CaseService:         &serviceapp.CaseService{},
		EmailService:        &serviceapp.EmailService{},
		NotificationService: &serviceapp.NotificationService{},
		Outbox:              noopOutboxPublisher{},
		TxRunner:            noopTransactionRunner{},
	})

	require.NoError(t, manager.Start(context.Background()))
	t.Cleanup(func() {
		require.NoError(t, manager.Stop(2*time.Second))
	})

	_, formGroups, err := bus.GetStreamInfo(eventbus.StreamFormEvents)
	require.NoError(t, err)
	assert.Equal(t, int64(1), formGroups)

	_, inboundEmailGroups, err := bus.GetStreamInfo(eventbus.StreamEmailEvents)
	require.NoError(t, err)
	assert.Equal(t, int64(1), inboundEmailGroups)

	_, emailCommandGroups, err := bus.GetStreamInfo(eventbus.StreamEmailCommands)
	require.NoError(t, err)
	assert.Equal(t, int64(1), emailCommandGroups)

	_, notificationCommandGroups, err := bus.GetStreamInfo(eventbus.StreamNotificationCommands)
	require.NoError(t, err)
	assert.Equal(t, int64(1), notificationCommandGroups)

	_, caseCommandGroups, err := bus.GetStreamInfo(eventbus.StreamCaseCommands)
	require.NoError(t, err)
	assert.Equal(t, int64(1), caseCommandGroups)
}
