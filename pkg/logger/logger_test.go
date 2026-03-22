package logger

import (
	"testing"
)

func TestNew(t *testing.T) {
	logger := New()
	if logger == nil {
		t.Fatal("Expected logger to be created")
	}

	// Test logging doesn't panic
	logger.Info("Test message")
	logger.Debug("Debug message")
	logger.Error("Error message")
	logger.Warn("Warning message")
}

func TestNewWithEnvironment_Development(t *testing.T) {
	logger := NewWithEnvironment("development")
	if logger == nil {
		t.Fatal("Expected logger to be created")
	}

	// Test logging
	logger.Info("Development log")
}

func TestNewWithEnvironment_Production(t *testing.T) {
	logger := NewWithEnvironment("production")
	if logger == nil {
		t.Fatal("Expected logger to be created")
	}

	// Test logging
	logger.Info("Production log")
}

func TestNewWithEnvironment_Invalid(t *testing.T) {
	logger := NewWithEnvironment("invalid")
	if logger == nil {
		t.Fatal("Expected logger to be created with default config")
	}

	// Should default to production
	logger.Info("Default log")
}

func TestWithField(t *testing.T) {
	logger := New()

	// Test WithField
	fieldLogger := logger.WithField("key", "value")
	if fieldLogger == nil {
		t.Fatal("Expected logger with field to be created")
	}

	// Test logging doesn't panic
	fieldLogger.Info("Test message with field")
}

func TestWithFields(t *testing.T) {
	logger := New()

	// Test WithFields
	fieldsLogger := logger.WithFields(map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	})

	if fieldsLogger == nil {
		t.Fatal("Expected logger with fields to be created")
	}

	// Test logging doesn't panic
	fieldsLogger.Info("Test message with fields")
}

func TestWithError(t *testing.T) {
	logger := New()

	// Test WithError
	err := &testError{msg: "test error"}
	errorLogger := logger.WithError(err)

	if errorLogger == nil {
		t.Fatal("Expected logger with error to be created")
	}

	// Test logging doesn't panic
	errorLogger.Error("Error occurred")
}

func TestWithWorkspace(t *testing.T) {
	logger := New()

	// Test WithWorkspace
	wsLogger := logger.WithWorkspace("workspace-123")
	if wsLogger == nil {
		t.Fatal("Expected logger with workspace to be created")
	}

	// Test logging doesn't panic
	wsLogger.Info("Workspace log")
}

func TestWithUser(t *testing.T) {
	logger := New()

	// Test WithUser
	userLogger := logger.WithUser("user-456")
	if userLogger == nil {
		t.Fatal("Expected logger with user to be created")
	}

	// Test logging doesn't panic
	userLogger.Info("User log")
}

func TestWithProject(t *testing.T) {
	logger := New()

	// Test WithProject
	projectLogger := logger.WithProject("project-789")
	if projectLogger == nil {
		t.Fatal("Expected logger with project to be created")
	}

	// Test logging doesn't panic
	projectLogger.Info("Project log")
}

func TestWithRequestID(t *testing.T) {
	logger := New()

	// Test WithRequestID
	reqLogger := logger.WithRequestID("req-abc123")
	if reqLogger == nil {
		t.Fatal("Expected logger with request ID to be created")
	}

	// Test logging doesn't panic
	reqLogger.Info("Request log")
}

func TestSync(t *testing.T) {
	logger := New()

	// Test sync doesn't panic
	if err := logger.Sync(); err != nil {
		// Sync error is acceptable in tests
		t.Logf("Sync returned error (acceptable in tests): %v", err)
	}
}

// Helper type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
