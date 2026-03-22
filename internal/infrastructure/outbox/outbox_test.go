package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/id"
	"github.com/movebigrocks/platform/pkg/logger"
)

type failingCaseEventBus struct {
	mu     sync.Mutex
	err    error
	calls  int
	stream eventbus.Stream
}

func (b *failingCaseEventBus) Publish(_ eventbus.Stream, _ interface{}) error {
	return b.PublishWithType(eventbus.Stream{}, eventbus.TypeUnknown, "", nil)
}

func (b *failingCaseEventBus) PublishEvent(_ eventbus.Stream, _ eventbus.Event) error {
	return b.PublishWithType(eventbus.Stream{}, eventbus.TypeUnknown, "", nil)
}

func (b *failingCaseEventBus) PublishEventWithRetry(_ eventbus.Stream, _ eventbus.Event, _ int) error {
	return b.PublishWithType(eventbus.Stream{}, eventbus.TypeUnknown, "", nil)
}

func (b *failingCaseEventBus) PublishWithType(stream eventbus.Stream, _ eventbus.EventType, _ string, _ interface{}) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	b.stream = stream
	return b.err
}

func (b *failingCaseEventBus) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func (b *failingCaseEventBus) calledStream() eventbus.Stream {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stream
}

func (b *failingCaseEventBus) Subscribe(_ eventbus.Stream, _, _ string, _ func(context.Context, []byte) error) error {
	return nil
}

func (b *failingCaseEventBus) GetStreamInfo(_ eventbus.Stream) (int64, int64, error) {
	return 0, 0, nil
}

func (b *failingCaseEventBus) GetPendingMessages(_ eventbus.Stream, _ string) (int64, error) {
	return 0, nil
}

func (b *failingCaseEventBus) HealthCheck() error {
	return nil
}

func (b *failingCaseEventBus) Shutdown(_ time.Duration) error {
	return nil
}

func (b *failingCaseEventBus) Close() error {
	return nil
}

func (b *failingCaseEventBus) PublishValidated(_ eventbus.Stream, _ eventbus.Event) error {
	return b.PublishWithType(eventbus.Stream{}, eventbus.TypeUnknown, "", nil)
}

func TestOutbox_ProcessPendingEvent_HandlesCaseEventDispatchFailuresAndTransitionsToFailedAfterMaxRetries(t *testing.T) {
	t.Parallel()

	store, cleanup := testutil.SetupTestSQLStore(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	failErr := errors.New("case event handler unavailable")
	bus := &failingCaseEventBus{err: failErr}
	service := NewServiceWithConfig(
		store,
		bus,
		logger.New(),
		config.OutboxConfig{
			MaxRetries:       2,
			MaxBackoff:       15 * time.Second,
			HealthMaxPending: 100,
			HealthMaxAge:     5 * time.Minute,
		},
	)

	eventPayload, err := json.Marshal(map[string]interface{}{
		"issue_id":    "issue-abc",
		"resolved_at": time.Now().Format(time.RFC3339),
	})
	require.NoError(t, err)

	outboxEvent := &shared.OutboxEvent{
		ID:        id.New(),
		Stream:    eventbus.StreamCaseEvents.String(),
		EventType: eventbus.TypeCasesBulkResolved.String(),
		EventData: eventPayload,
		Status:    "pending",
		Attempts:  0,
		CreatedAt: time.Now(),
	}
	require.NoError(t, store.Outbox().SaveOutboxEvent(ctx, outboxEvent))

	event, err := store.Outbox().GetOutboxEvent(ctx, outboxEvent.ID)
	require.NoError(t, err)

	service.ProcessPendingEvent(ctx, event)
	event, err = store.Outbox().GetOutboxEvent(ctx, outboxEvent.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, event.Attempts)
	assert.Equal(t, "pending", event.Status)
	assert.NotNil(t, event.NextRetry)
	assert.Equal(t, 1, bus.callCount())
	assert.Equal(t, eventbus.StreamCaseEvents, bus.calledStream())
	assert.Contains(t, event.LastError, failErr.Error())

	event.NextRetry = nil
	require.NoError(t, store.Outbox().UpdateOutboxEvent(ctx, event))
	service.ProcessPendingEvent(ctx, event)
	event, err = store.Outbox().GetOutboxEvent(ctx, outboxEvent.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, event.Attempts)
	assert.Equal(t, "pending", event.Status)
	assert.NotNil(t, event.NextRetry)
	assert.Equal(t, 2, bus.callCount())

	event.NextRetry = nil
	require.NoError(t, store.Outbox().UpdateOutboxEvent(ctx, event))
	service.ProcessPendingEvent(ctx, event)
	event, err = store.Outbox().GetOutboxEvent(ctx, outboxEvent.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", event.Status)
	assert.Equal(t, 2, event.Attempts)
	assert.Equal(t, "max retries (2) exceeded", event.LastError)
	assert.Equal(t, 2, bus.callCount())
}
