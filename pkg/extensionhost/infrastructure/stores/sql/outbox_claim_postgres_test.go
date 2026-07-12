package sql_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
	"github.com/movebigrocks/platform/pkg/id"
)

func TestOutboxClaimPendingEventsIsAtomicAcrossWorkers(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	event := &shared.OutboxEvent{
		ID:        id.New(),
		Stream:    "cases",
		EventType: "case.created",
		EventData: []byte(`{"event_id":"claim-test"}`),
		Status:    "pending",
		CreatedAt: time.Now().UTC().Add(-time.Hour),
	}
	require.NoError(t, store.Outbox().SaveOutboxEvent(ctx, event))

	start := make(chan struct{})
	type claimResult struct {
		count int
		err   error
	}
	results := make(chan claimResult, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claimed, err := store.Outbox().ClaimPendingOutboxEvents(ctx, 1)
			results <- claimResult{count: len(claimed), err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	totalClaimed := 0
	for result := range results {
		require.NoError(t, result.err)
		totalClaimed += result.count
	}
	require.Equal(t, 1, totalClaimed)

	stored, err := store.Outbox().GetOutboxEvent(ctx, event.ID)
	require.NoError(t, err)
	require.Equal(t, "publishing", stored.Status)
	require.NotNil(t, stored.ClaimedAt)

	// Recovery is based on the active publishing claim, not the original event
	// age. An old event freshly claimed by a live worker must not be reclaimed.
	recovered, err := store.Outbox().RecoverStalePublishingEvents(ctx, 2*time.Minute)
	require.NoError(t, err)
	require.Zero(t, recovered)

	db, err := store.GetSQLDB()
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		UPDATE core_infra.outbox_events
		SET claimed_at = NOW() - INTERVAL '3 minutes'
		WHERE id = $1`, event.ID)
	require.NoError(t, err)
	recovered, err = store.Outbox().RecoverStalePublishingEvents(ctx, 2*time.Minute)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
}

func TestIdempotencyProcessingLeasePreventsConcurrentHandlers(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	ctx := context.Background()
	eventID := id.New()
	group := "case-sync"

	claimed, err := store.Idempotency().ClaimProcessing(ctx, eventID, group, time.Minute)
	require.NoError(t, err)
	require.True(t, claimed)

	claimed, err = store.Idempotency().ClaimProcessing(ctx, eventID, group, time.Minute)
	require.NoError(t, err)
	require.False(t, claimed)

	require.NoError(t, store.Idempotency().ReleaseProcessingClaim(ctx, eventID, group))
	claimed, err = store.Idempotency().ClaimProcessing(ctx, eventID, group, time.Minute)
	require.NoError(t, err)
	require.True(t, claimed)

	require.NoError(t, store.Idempotency().MarkProcessed(ctx, eventID, group))
	processed, err := store.Idempotency().IsProcessed(ctx, eventID, group)
	require.NoError(t, err)
	require.True(t, processed)

	claimed, err = store.Idempotency().ClaimProcessing(ctx, eventID, group, time.Minute)
	require.NoError(t, err)
	require.False(t, claimed)
}
