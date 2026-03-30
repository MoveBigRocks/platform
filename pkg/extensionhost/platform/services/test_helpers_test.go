//go:build integration

package platformservices

import (
	"context"
	"testing"

	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

// setupTestStore creates an isolated test store.
// Returns the store and a cleanup function.
func setupTestStore(t *testing.T) (shared.Store, func()) {
	t.Helper()
	return testutil.SetupTestStore(t)
}

// MockOutboxPublisher captures published events for test assertions
type MockOutboxPublisher struct {
	events []interface{}
}

func NewMockOutboxPublisher() *MockOutboxPublisher {
	return &MockOutboxPublisher{events: make([]interface{}, 0)}
}

func (m *MockOutboxPublisher) Publish(ctx context.Context, stream eventbus.Stream, event interface{}) error {
	m.events = append(m.events, event)
	return nil
}

func (m *MockOutboxPublisher) PublishEvent(ctx context.Context, stream eventbus.Stream, event eventbus.Event) error {
	m.events = append(m.events, event)
	return nil
}

func (m *MockOutboxPublisher) GetPublishedEvents() []interface{} {
	return m.events
}
