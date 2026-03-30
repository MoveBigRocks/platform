//go:build integration

package platformservices

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

// setupTestStore creates an isolated test store.
// Returns the store and a cleanup function.
func setupTestStore(t *testing.T) (shared.Store, func()) {
	t.Helper()
	return testutil.SetupTestStore(t)
}

// MockStore wraps a real store and exposes sub-stores as fields for test convenience
type MockStore struct {
	shared.Store
	users      shared.UserStore
	workspaces shared.WorkspaceStore
	tempDir    string
}

// NewMockStore creates a mock store backed by a real filesystem store in a temp directory
// Panics if store creation fails - this is acceptable for test setup code
func NewMockStore() *MockStore {
	tempDir, err := os.MkdirTemp("", "mock-store-test-*")
	if err != nil {
		panic("failed to create temp dir for mock store: " + err.Error())
	}
	store, err := stores.NewStore(filepath.Join(tempDir, "mbr.db"))
	if err != nil {
		os.RemoveAll(tempDir)
		panic("failed to create mock store: " + err.Error())
	}
	return &MockStore{
		Store:      store,
		users:      store.Users(),
		workspaces: store.Workspaces(),
		tempDir:    tempDir,
	}
}

// Cleanup removes the temp directory - should be called in test cleanup
func (m *MockStore) Cleanup() {
	os.RemoveAll(m.tempDir)
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
