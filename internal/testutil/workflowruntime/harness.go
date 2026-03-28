package workflowruntime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/outbox"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// Harness provides the real event bus, outbox, and worker-manager path used in
// production so workflow tests can prove end-to-end behavior instead of calling
// command handlers directly.
type Harness struct {
	started bool

	EventBus *eventbus.InMemoryBus
	Outbox   *outbox.Service
}

func NewHarness(t *testing.T, store stores.Store) *Harness {
	t.Helper()

	h := &Harness{
		EventBus: eventbus.NewInMemoryBus(),
	}
	h.Outbox = outbox.NewServiceWithConfig(store, h.EventBus, logger.NewNop(), config.OutboxConfig{
		PollInterval:     10 * time.Millisecond,
		MaxRetries:       3,
		RetentionDays:    1,
		BatchSize:        25,
		MaxBackoff:       250 * time.Millisecond,
		HealthMaxPending: 100,
		HealthMaxAge:     time.Minute,
	})

	t.Cleanup(func() {
		if h.started {
			require.NoError(t, h.Outbox.Stop(2*time.Second))
		}
	})

	return h
}

func (h *Harness) Start(t *testing.T) {
	t.Helper()
	h.Outbox.Start()
	h.started = true
}
