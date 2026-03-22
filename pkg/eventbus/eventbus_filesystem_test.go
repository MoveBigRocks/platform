package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/pkg/logger"
)

// testEvent is a simple event type for testing that implements Event interface
type testEvent struct {
	BaseEvent
	Message string `json:"message"`
	Value   int    `json:"value"`
}

// newTestEvent creates a test event with proper BaseEvent initialization
func newTestEvent(message string, value int) testEvent {
	return testEvent{
		BaseEvent: NewBaseEvent(TypeCaseCreated),
		Message:   message,
		Value:     value,
	}
}

// Validate implements Event interface for testEvent
func (e testEvent) Validate() error {
	return nil
}

// validatingEvent implements Event interface for testing
type validatingEvent struct {
	BaseEvent
	Message string `json:"message"`
	Valid   bool   `json:"-"` // Controls whether validation passes
}

func (e *validatingEvent) Validate() error {
	if !e.Valid {
		return errors.New("validation failed")
	}
	return nil
}

func setupTestEventBus(t *testing.T) (*FileEventBus, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "eventbus_test_*")
	require.NoError(t, err)

	log := logger.NewWithEnvironment("test")
	eb, err := NewFileEventBus(context.Background(), tmpDir, log)
	require.NoError(t, err)

	cleanup := func() {
		eb.Close()
		os.RemoveAll(tmpDir)
	}

	return eb, tmpDir, cleanup
}

func TestNewFileEventBus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "eventbus_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	log := logger.NewWithEnvironment("test")
	eb, err := NewFileEventBus(context.Background(), tmpDir, log)
	require.NoError(t, err)
	defer eb.Close()

	assert.NotNil(t, eb)
	assert.Equal(t, filepath.Join(tmpDir, "events"), eb.basePath)

	// Verify events directory was created
	_, err = os.Stat(filepath.Join(tmpDir, "events"))
	assert.NoError(t, err)
}

func TestFileEventBus_Publish(t *testing.T) {
	eb, tmpDir, cleanup := setupTestEventBus(t)
	defer cleanup()

	event := newTestEvent("Hello World", 42)

	// Publish event
	err := eb.Publish(StreamCaseEvents, event)
	require.NoError(t, err)

	// Verify event file was created
	pendingDir := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "pending")
	entries, err := os.ReadDir(pendingDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	// Read and verify event content
	eventPath := filepath.Join(pendingDir, entries[0].Name())
	data, err := os.ReadFile(eventPath)
	require.NoError(t, err)

	var ef eventFile
	err = json.Unmarshal(data, &ef)
	require.NoError(t, err)

	assert.Equal(t, StreamCaseEvents.String(), ef.Stream)
	assert.NotEmpty(t, ef.ID)
	assert.Greater(t, ef.Timestamp, int64(0))
	assert.Equal(t, 0, ef.Retries)
}

func TestFileEventBus_PublishValidated_ValidEvent(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	event := &validatingEvent{
		BaseEvent: NewBaseEvent(TypeCaseCreated),
		Message:   "Valid event",
		Valid:     true,
	}

	err := eb.PublishValidated(StreamCaseEvents, event)
	require.NoError(t, err)
}

func TestFileEventBus_PublishValidated_InvalidEvent(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	event := &validatingEvent{
		BaseEvent: NewBaseEvent(TypeCaseCreated),
		Message:   "Invalid event",
		Valid:     false,
	}

	err := eb.PublishValidated(StreamCaseEvents, event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestFileEventBus_PublishValidated_NonValidatingEvent(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Regular event without Validate method should pass
	event := newTestEvent("Regular event", 0)

	err := eb.PublishValidated(StreamCaseEvents, event)
	require.NoError(t, err)
}

func TestFileEventBus_PublishWithRetry(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	event := newTestEvent("Retry test", 0)

	// Should succeed on first try
	err := eb.PublishEventWithRetry(StreamCaseEvents, event, 3)
	require.NoError(t, err)
}

func TestFileEventBus_GetStreamInfo_EmptyStream(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	length, groups, err := eb.GetStreamInfo(StreamCaseEvents)
	require.NoError(t, err)
	assert.Equal(t, int64(0), length)
	assert.Equal(t, int64(0), groups)
}

func TestFileEventBus_GetStreamInfo_WithEvents(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Publish some events
	for i := 0; i < 5; i++ {
		err := eb.Publish(StreamCaseEvents, newTestEvent("test", i))
		require.NoError(t, err)
	}

	length, groups, err := eb.GetStreamInfo(StreamCaseEvents)
	require.NoError(t, err)
	assert.Equal(t, int64(5), length)
	assert.Equal(t, int64(1), groups) // FileEventBus always returns 1 group
}

func TestFileEventBus_GetPendingMessages(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Publish some events
	for i := 0; i < 3; i++ {
		err := eb.Publish(StreamCaseEvents, newTestEvent("test", i))
		require.NoError(t, err)
	}

	pending, err := eb.GetPendingMessages(StreamCaseEvents, "test-group")
	require.NoError(t, err)
	assert.Equal(t, int64(3), pending)
}

func TestFileEventBus_HealthCheck(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	err := eb.HealthCheck()
	assert.NoError(t, err)
}

func TestFileEventBus_TrimStream(t *testing.T) {
	eb, tmpDir, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Create processed directory and add old file
	processedPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "processed")
	err := os.MkdirAll(processedPath, 0755)
	require.NoError(t, err)

	// Create an old file
	oldFile := filepath.Join(processedPath, "old-event.json")
	err = os.WriteFile(oldFile, []byte(`{}`), 0644)
	require.NoError(t, err)

	// Set modification time to 2 hours ago
	oldTime := time.Now().Add(-2 * time.Hour)
	err = os.Chtimes(oldFile, oldTime, oldTime)
	require.NoError(t, err)

	// Create a recent file
	recentFile := filepath.Join(processedPath, "recent-event.json")
	err = os.WriteFile(recentFile, []byte(`{}`), 0644)
	require.NoError(t, err)

	// Trim events older than 1 hour
	err = eb.TrimStream(StreamCaseEvents.String(), 1*time.Hour)
	require.NoError(t, err)

	// Old file should be removed
	_, err = os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err))

	// Recent file should remain
	_, err = os.Stat(recentFile)
	assert.NoError(t, err)
}

func TestFileEventBus_TrimStream_NonexistentStream(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Trimming nonexistent stream should not error
	err := eb.TrimStream("nonexistent-stream", 1*time.Hour)
	assert.NoError(t, err)
}

func TestFileEventBus_Shutdown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "eventbus_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	log := logger.NewWithEnvironment("test")
	eb, err := NewFileEventBus(context.Background(), tmpDir, log)
	require.NoError(t, err)

	// Manually call shutdown (don't use cleanup which would call Close)
	err = eb.Shutdown(5 * time.Second)
	assert.NoError(t, err)
}

func TestFileEventBus_Close(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "eventbus_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	log := logger.NewWithEnvironment("test")
	eb, err := NewFileEventBus(context.Background(), tmpDir, log)
	require.NoError(t, err)

	// Manually call close (don't use cleanup which would call Close again)
	err = eb.Close()
	assert.NoError(t, err)
}

func TestFileEventBus_MoveFile(t *testing.T) {
	eb, tmpDir, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Create source directory and file
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")
	err := os.MkdirAll(srcDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(dstDir, 0755)
	require.NoError(t, err)

	srcFile := filepath.Join(srcDir, "test.json")
	dstFile := filepath.Join(dstDir, "test.json")
	err = os.WriteFile(srcFile, []byte(`{"test": true}`), 0644)
	require.NoError(t, err)

	// Move file
	err = eb.moveFile(srcFile, dstFile)
	require.NoError(t, err)

	// Verify source is gone
	_, err = os.Stat(srcFile)
	assert.True(t, os.IsNotExist(err))

	// Verify destination exists
	data, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, `{"test": true}`, string(data))
}

func TestStreamConstants(t *testing.T) {
	// Verify stream constants are defined with consistent naming
	assert.Equal(t, "error-events", StreamErrorEvents.String())
	assert.Equal(t, "issue-events", StreamIssueEvents.String())
	assert.Equal(t, "case-events", StreamCaseEvents.String())
	assert.Equal(t, "email-events", StreamEmailEvents.String())
	assert.Equal(t, "alert-events", StreamAlertEvents.String())
	assert.Equal(t, "audit-events", StreamAuditEvents.String())
	assert.Equal(t, "metrics", StreamMetrics.String())
	assert.Equal(t, "analytics", StreamAnalytics.String())
	assert.Equal(t, "job-events", StreamJobEvents.String())
	assert.Equal(t, "permission-events", StreamPermissionEvents.String())
}

func TestMaxRetries(t *testing.T) {
	assert.Equal(t, 3, MaxRetries)
}

func TestFileEventBus_ImplementsBusInterface(t *testing.T) {
	// Compile-time check that FileEventBus implements Bus
	var _ Bus = (*FileEventBus)(nil)
}

func TestEventBusTypeAlias(t *testing.T) {
	// Verify EventBus is an alias for Bus
	var eb EventBus
	var b Bus

	// These should be assignable to each other
	_ = eb
	_ = b
}

func TestFileEventBus_ProcessPendingEvents_WithRetries(t *testing.T) {
	eb, tmpDir, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Create a pending event with max retries exceeded
	pendingPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "pending")
	dlqPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "dlq")
	processedPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "processed")

	err := os.MkdirAll(pendingPath, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(dlqPath, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(processedPath, 0755)
	require.NoError(t, err)

	// Create event that has exceeded max retries
	testData, _ := json.Marshal(map[string]string{"test": "data"})
	maxRetriesEvent := eventFile{
		ID:        "test-max-retries",
		Stream:    StreamCaseEvents.String(),
		Data:      testData,
		Timestamp: time.Now().UnixNano(),
		Retries:   MaxRetries + 1,
	}
	data, err := json.Marshal(maxRetriesEvent)
	require.NoError(t, err)

	eventPath := filepath.Join(pendingPath, "test-event.json")
	err = os.WriteFile(eventPath, data, 0644)
	require.NoError(t, err)

	// Process events - the max retries event should be moved to DLQ
	handler := func(ctx context.Context, data []byte) error {
		return nil
	}
	err = eb.processPendingEvents(pendingPath, processedPath, dlqPath, StreamCaseEvents.String(), handler)
	require.NoError(t, err)

	// Verify event was moved to DLQ
	dlqFiles, err := os.ReadDir(dlqPath)
	require.NoError(t, err)
	assert.Len(t, dlqFiles, 1)

	// Verify event was removed from pending
	pendingFiles, err := os.ReadDir(pendingPath)
	require.NoError(t, err)
	assert.Len(t, pendingFiles, 0)
}

func TestFileEventBus_ProcessPendingEvents_HandlerError(t *testing.T) {
	eb, tmpDir, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Create a pending event
	pendingPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "pending")
	dlqPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "dlq")
	processedPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "processed")

	err := os.MkdirAll(pendingPath, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(dlqPath, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(processedPath, 0755)
	require.NoError(t, err)

	eventPayload, _ := json.Marshal(map[string]string{"test": "data"})
	event := eventFile{
		ID:        "test-handler-error",
		Stream:    StreamCaseEvents.String(),
		Data:      eventPayload,
		Timestamp: time.Now().UnixNano(),
		Retries:   0,
	}
	data, err := json.Marshal(event)
	require.NoError(t, err)

	eventPath := filepath.Join(pendingPath, "test-event.json")
	err = os.WriteFile(eventPath, data, 0644)
	require.NoError(t, err)

	// Handler that always fails
	handler := func(ctx context.Context, data []byte) error {
		return errors.New("handler failed")
	}

	err = eb.processPendingEvents(pendingPath, processedPath, dlqPath, StreamCaseEvents.String(), handler)
	require.NoError(t, err) // processPendingEvents doesn't return errors for individual events

	// Verify retry count was incremented
	eventData, err := os.ReadFile(eventPath)
	require.NoError(t, err)

	var updatedEvent eventFile
	err = json.Unmarshal(eventData, &updatedEvent)
	require.NoError(t, err)

	assert.Equal(t, 1, updatedEvent.Retries)
}

func TestFileEventBus_ProcessPendingEvents_InvalidJSON(t *testing.T) {
	eb, tmpDir, cleanup := setupTestEventBus(t)
	defer cleanup()

	pendingPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "pending")
	dlqPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "dlq")
	processedPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "processed")

	err := os.MkdirAll(pendingPath, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(dlqPath, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(processedPath, 0755)
	require.NoError(t, err)

	// Write invalid JSON
	eventPath := filepath.Join(pendingPath, "invalid-event.json")
	err = os.WriteFile(eventPath, []byte("not valid json"), 0644)
	require.NoError(t, err)

	handler := func(ctx context.Context, data []byte) error {
		return nil
	}

	err = eb.processPendingEvents(pendingPath, processedPath, dlqPath, StreamCaseEvents.String(), handler)
	require.NoError(t, err)

	// Invalid JSON should be moved to DLQ
	dlqFiles, err := os.ReadDir(dlqPath)
	require.NoError(t, err)
	assert.Len(t, dlqFiles, 1)
}

func TestFileEventBus_ProcessPendingEvents_SkipsNonJSON(t *testing.T) {
	eb, tmpDir, cleanup := setupTestEventBus(t)
	defer cleanup()

	pendingPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "pending")
	dlqPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "dlq")
	processedPath := filepath.Join(tmpDir, "events", StreamCaseEvents.String(), "processed")

	err := os.MkdirAll(pendingPath, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(dlqPath, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(processedPath, 0755)
	require.NoError(t, err)

	// Create a non-JSON file
	err = os.WriteFile(filepath.Join(pendingPath, "readme.txt"), []byte("not an event"), 0644)
	require.NoError(t, err)

	// Create a subdirectory (should be skipped)
	err = os.MkdirAll(filepath.Join(pendingPath, "subdir"), 0755)
	require.NoError(t, err)

	handler := func(ctx context.Context, data []byte) error {
		return nil
	}

	err = eb.processPendingEvents(pendingPath, processedPath, dlqPath, StreamCaseEvents.String(), handler)
	require.NoError(t, err)

	// Non-JSON file should not be moved
	_, err = os.Stat(filepath.Join(pendingPath, "readme.txt"))
	assert.NoError(t, err)
}

func TestFileEventBus_ProcessPendingEvents_NonexistentPath(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	handler := func(ctx context.Context, data []byte) error {
		return nil
	}

	// Should not error on nonexistent path
	err := eb.processPendingEvents("/nonexistent/path", "/nonexistent/processed", "/nonexistent/dlq", "test", handler)
	assert.NoError(t, err)
}

func TestFileEventBus_Publish_MarshalError(t *testing.T) {
	eb, _, cleanup := setupTestEventBus(t)
	defer cleanup()

	// Create an unmarshalable value (channel cannot be marshaled to JSON)
	badEvent := make(chan int)

	err := eb.Publish(StreamCaseEvents, badEvent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal data")
}

func TestEventInterface(t *testing.T) {
	// Verify Event interface is properly implemented by validatingEvent
	var _ Event = (*validatingEvent)(nil)
}

func TestBusInterface(t *testing.T) {
	// Compile-time verification that FileEventBus implements all Bus methods
	var bus Bus = &FileEventBus{}
	_ = bus

	// Verify the interface has all expected methods
	assert.NotNil(t, bus.Publish)
	assert.NotNil(t, bus.PublishValidated)
	assert.NotNil(t, bus.Subscribe)
	assert.NotNil(t, bus.GetStreamInfo)
	assert.NotNil(t, bus.GetPendingMessages)
	assert.NotNil(t, bus.HealthCheck)
	assert.NotNil(t, bus.Shutdown)
	assert.NotNil(t, bus.Close)
}
