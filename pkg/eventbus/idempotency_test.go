package eventbus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type idempotencyStoreStub struct {
	claimed  bool
	claimErr error
	released bool
}

func (s *idempotencyStoreStub) ClaimProcessing(context.Context, string, string, time.Duration) (bool, error) {
	return s.claimed, s.claimErr
}

func (s *idempotencyStoreStub) MarkProcessed(context.Context, string, string) error {
	return nil
}

func (s *idempotencyStoreStub) ReleaseProcessingClaim(context.Context, string, string) error {
	s.released = true
	return nil
}

func (s *idempotencyStoreStub) IsProcessed(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestDBIdempotencyFailsClosedWhenLeaseCannotBeClaimed(t *testing.T) {
	store := &idempotencyStoreStub{claimErr: errors.New("database unavailable")}
	called := false
	handler := WithDBIdempotency(store, "test-handler", func(context.Context, []byte) error {
		called = true
		return nil
	})

	err := handler(context.Background(), []byte(`{"event_id":"evt-1","event_type":"test.event"}`))
	require.ErrorContains(t, err, "claim idempotency lease")
	require.False(t, called)
}

func TestDBIdempotencyReleasesLeaseWhenHandlerFails(t *testing.T) {
	store := &idempotencyStoreStub{claimed: true}
	handler := WithDBIdempotency(store, "test-handler", func(context.Context, []byte) error {
		return errors.New("handler failed")
	})

	err := handler(context.Background(), []byte(`{"event_id":"evt-1","event_type":"test.event"}`))
	require.ErrorContains(t, err, "handler failed")
	require.True(t, store.released)
}
