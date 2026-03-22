// Package sentry provides SDK compatibility tests for the Move Big Rocks error monitoring system.
// These tests verify that Move Big Rocks can correctly parse and process Sentry-formatted events.
//
// Run with: go test -v ./tests/sentry/...
package sentry

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/movebigrocks/platform/internal/infrastructure/middleware"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	errordom "github.com/movebigrocks/platform/internal/observability/domain"
	observabilityhandlers "github.com/movebigrocks/platform/internal/observability/handlers"
	shared "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/id"
	"github.com/movebigrocks/platform/pkg/logger"
)

// ============================================================================
// SENTRY PROTOCOL PARSING TESTS
// ============================================================================

// TestSentryEventJSONParsing verifies we can parse real Sentry event JSON
func TestSentryEventJSONParsing(t *testing.T) {
	sentryEventJSON := `{
		"event_id": "12c2d058d58442709aa2eca08bf20986",
		"timestamp": "2024-01-15T14:30:45.123Z",
		"level": "error",
		"logger": "javascript",
		"platform": "javascript",
		"message": "Cannot read property 'foo' of undefined",
		"environment": "production",
		"release": "1.2.3",
		"exception": [{
			"type": "TypeError",
			"value": "Cannot read property 'foo' of undefined",
			"stacktrace": {
				"frames": [
					{
						"filename": "app.js",
						"function": "doSomething",
						"lineno": 42,
						"colno": 12,
						"in_app": true
					},
					{
						"filename": "utils.js",
						"function": "helper",
						"lineno": 123,
						"colno": 5,
						"in_app": true
					}
				]
			}
		}],
		"user": {
			"id": "user123",
			"email": "user@example.com"
		},
		"tags": {
			"component": "frontend",
			"browser": "chrome"
		},
		"extra": {
			"session_id": "abc123",
			"build_number": 456
		}
	}`

	var eventData map[string]interface{}
	err := json.Unmarshal([]byte(sentryEventJSON), &eventData)
	require.NoError(t, err, "Should parse Sentry JSON")

	// Verify basic fields
	assert.Equal(t, "12c2d058d58442709aa2eca08bf20986", eventData["event_id"])
	assert.Equal(t, "error", eventData["level"])
	assert.Equal(t, "javascript", eventData["platform"])
	assert.Equal(t, "production", eventData["environment"])
}

// TestSentryEventConversion tests conversion from Sentry format to internal ErrorEvent
func TestSentryEventConversion(t *testing.T) {
	projectID := "test-project-123"

	testCases := []struct {
		name      string
		eventJSON string
		verify    func(t *testing.T, event *errordom.ErrorEvent)
	}{
		{
			name: "basic error event",
			eventJSON: `{
				"event_id": "abc123",
				"message": "Test error",
				"level": "error",
				"platform": "python"
			}`,
			verify: func(t *testing.T, event *errordom.ErrorEvent) {
				assert.Equal(t, "Test error", event.Message)
				assert.Equal(t, "error", event.Level)
				assert.Equal(t, "python", event.Platform)
			},
		},
		{
			name: "event with exception",
			eventJSON: `{
				"event_id": "exc123",
				"level": "error",
				"platform": "node",
				"exception": [{
					"type": "TypeError",
					"value": "Cannot read property 'x' of null",
					"stacktrace": {
						"frames": [
							{"filename": "index.js", "function": "main", "lineno": 10}
						]
					}
				}]
			}`,
			verify: func(t *testing.T, event *errordom.ErrorEvent) {
				require.Len(t, event.Exception, 1)
				assert.Equal(t, "TypeError", event.Exception[0].Type)
				assert.Equal(t, "Cannot read property 'x' of null", event.Exception[0].Value)
				require.NotNil(t, event.Exception[0].Stacktrace)
				require.Len(t, event.Exception[0].Stacktrace.Frames, 1)
				assert.Equal(t, "index.js", event.Exception[0].Stacktrace.Frames[0].Filename)
			},
		},
		{
			name: "event with user context",
			eventJSON: `{
				"event_id": "user123",
				"message": "User error",
				"level": "warning",
				"user": {
					"id": "u-123",
					"email": "test@example.com",
					"username": "testuser"
				}
			}`,
			verify: func(t *testing.T, event *errordom.ErrorEvent) {
				require.NotNil(t, event.User)
				assert.Equal(t, "u-123", event.User.ID)
				assert.Equal(t, "test@example.com", event.User.Email)
			},
		},
		{
			name: "event with tags and extra",
			eventJSON: `{
				"event_id": "tags123",
				"message": "Tagged error",
				"level": "error",
				"tags": {
					"environment": "production",
					"version": "1.2.3"
				},
				"extra": {
					"request_id": "req-456",
					"retry_count": 3
				}
			}`,
			verify: func(t *testing.T, event *errordom.ErrorEvent) {
				assert.Equal(t, "production", event.Tags["environment"])
				assert.Equal(t, "1.2.3", event.Tags["version"])
				assert.Equal(t, "req-456", event.Extra.GetString("request_id"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var data map[string]interface{}
			err := json.Unmarshal([]byte(tc.eventJSON), &data)
			require.NoError(t, err)

			event, err := convertSentryEvent(data, projectID)
			require.NoError(t, err)
			tc.verify(t, event)
		})
	}
}

// convertSentryEvent converts Sentry event data to internal ErrorEvent format
func convertSentryEvent(data map[string]interface{}, projectID string) (*errordom.ErrorEvent, error) {
	eventID, _ := data["event_id"].(string)
	if eventID == "" {
		eventID = strings.ReplaceAll(id.New(), "-", "")
	}

	event := errordom.NewErrorEvent(projectID, eventID)

	if msg, ok := data["message"].(string); ok {
		event.Message = msg
	}

	if level, ok := data["level"].(string); ok {
		event.Level = level
	}

	if platform, ok := data["platform"].(string); ok {
		event.Platform = platform
	}

	if logger, ok := data["logger"].(string); ok {
		event.Logger = logger
	}

	if env, ok := data["environment"].(string); ok {
		event.Environment = env
	}

	if release, ok := data["release"].(string); ok {
		event.Release = release
	}

	// Parse timestamp
	if ts, ok := data["timestamp"].(string); ok {
		formats := []string{
			"2006-01-02T15:04:05.000Z",
			time.RFC3339,
			time.RFC3339Nano,
		}
		for _, format := range formats {
			if parsed, err := time.Parse(format, ts); err == nil {
				event.Timestamp = parsed
				break
			}
		}
	}

	// Parse exceptions
	if exceptions, ok := data["exception"].([]interface{}); ok {
		for _, excInterface := range exceptions {
			if excData, ok := excInterface.(map[string]interface{}); ok {
				exc := errordom.ExceptionData{}
				exc.Type, _ = excData["type"].(string)
				exc.Value, _ = excData["value"].(string)

				if stackData, ok := excData["stacktrace"].(map[string]interface{}); ok {
					if frames, ok := stackData["frames"].([]interface{}); ok {
						var parsedFrames []errordom.FrameData
						for _, frameInterface := range frames {
							if frameData, ok := frameInterface.(map[string]interface{}); ok {
								frame := errordom.FrameData{}
								frame.Filename, _ = frameData["filename"].(string)
								frame.Function, _ = frameData["function"].(string)
								if lineno, ok := frameData["lineno"].(float64); ok {
									frame.LineNumber = int(lineno)
								}
								frame.InApp, _ = frameData["in_app"].(bool)
								parsedFrames = append(parsedFrames, frame)
							}
						}
						exc.Stacktrace = &errordom.StacktraceData{Frames: parsedFrames}
					}
				}
				event.Exception = append(event.Exception, exc)
			}
		}
	}

	// Parse user context
	if userData, ok := data["user"].(map[string]interface{}); ok {
		user := &errordom.UserContext{}
		user.ID, _ = userData["id"].(string)
		user.Email, _ = userData["email"].(string)
		user.Username, _ = userData["username"].(string)
		event.User = user
	}

	// Parse tags
	if tags, ok := data["tags"].(map[string]interface{}); ok {
		event.Tags = make(map[string]string)
		for k, v := range tags {
			if strVal, ok := v.(string); ok {
				event.Tags[k] = strVal
			}
		}
	}

	// Parse extra
	if extra, ok := data["extra"].(map[string]interface{}); ok {
		event.Extra = shared.MetadataFromMap(extra)
	}

	return event, nil
}

// ============================================================================
// SENTRY ENVELOPE FORMAT TESTS
// ============================================================================

// TestSentryEnvelopeFormat verifies we can parse Sentry envelope format
func TestSentryEnvelopeFormat(t *testing.T) {
	t.Run("parses valid envelope header", func(t *testing.T) {
		envelope := `{"event_id":"abc123","sent_at":"2024-01-01T00:00:00Z"}
{"type":"event","length":50}
{"message":"Test error","level":"error","platform":"go"}`

		lines := strings.Split(envelope, "\n")
		require.Len(t, lines, 3)

		var header map[string]interface{}
		err := json.Unmarshal([]byte(lines[0]), &header)
		require.NoError(t, err)
		assert.Equal(t, "abc123", header["event_id"])
	})

	t.Run("handles SDK-specific envelope formats", func(t *testing.T) {
		// Node.js SDK format
		nodeEnvelope := `{"event_id":"a1b2c3","sdk":{"name":"sentry.javascript.node","version":"7.91.0"}}
{"type":"event"}
{"level":"error","platform":"node"}`

		lines := strings.Split(nodeEnvelope, "\n")
		assert.Len(t, lines, 3)

		var header map[string]interface{}
		err := json.Unmarshal([]byte(lines[0]), &header)
		require.NoError(t, err)

		sdk := header["sdk"].(map[string]interface{})
		assert.Equal(t, "sentry.javascript.node", sdk["name"])
	})
}

// TestSentryAuthHeader verifies X-Sentry-Auth header parsing
func TestSentryAuthHeader(t *testing.T) {
	testCases := []struct {
		name      string
		header    string
		wantKey   string
		wantValid bool
	}{
		{
			name:      "valid auth header",
			header:    "Sentry sentry_key=abc123,sentry_version=7",
			wantKey:   "abc123",
			wantValid: true,
		},
		{
			name:      "auth with secret",
			header:    "Sentry sentry_key=pub123,sentry_secret=sec456,sentry_version=7",
			wantKey:   "pub123",
			wantValid: true,
		},
		{
			name:      "empty header",
			header:    "",
			wantValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key, valid := parseSentryAuth(tc.header)
			assert.Equal(t, tc.wantKey, key)
			assert.Equal(t, tc.wantValid, valid)
		})
	}
}

func parseSentryAuth(header string) (string, bool) {
	if !strings.HasPrefix(header, "Sentry ") {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(header, "Sentry "), ",")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && kv[0] == "sentry_key" {
			return kv[1], true
		}
	}
	return "", false
}

type fakeSentryProjectStore struct {
	project         *errordom.Project
	erroredProject  error
	gotProjectByKey string

	incrementCalled    bool
	incrementCalls     int
	incrementProjectID string
}

func (s *fakeSentryProjectStore) GetProjectByKey(_ context.Context, projectKey string) (*errordom.Project, error) {
	s.gotProjectByKey = projectKey
	if s.erroredProject != nil {
		return nil, s.erroredProject
	}
	return s.project, nil
}

func (s *fakeSentryProjectStore) IncrementEventCount(_ context.Context, _, projectID string, _ *time.Time) (int64, error) {
	s.incrementCalled = true
	s.incrementCalls++
	s.incrementProjectID = projectID
	return 42, nil
}

type fakeSentryEventStore struct {
	createCalled int
	lastEvent    *errordom.ErrorEvent
	createErr    error
}

func (s *fakeSentryEventStore) CreateErrorEvent(_ context.Context, event *errordom.ErrorEvent) error {
	s.createCalled++
	s.lastEvent = event
	return s.createErr
}

type fakeSentryEventProcessor struct {
	processCalled int
	processErr    error
}

func (s *fakeSentryEventProcessor) ProcessEvent(_ context.Context, _ *errordom.ErrorEvent) error {
	s.processCalled++
	return s.processErr
}

func newSentryIngestRouter(projectStore *fakeSentryProjectStore, eventStore *fakeSentryEventStore, processor *fakeSentryEventProcessor) *gin.Engine {
	gin.SetMode(gin.TestMode)

	handler := observabilityhandlers.NewSentryIngestHandler(projectStore, eventStore, processor, logger.NewNop())

	router := gin.New()
	router.Use(middleware.InputValidation())
	router.POST("/1/envelope", handler.HandleEnvelope)
	router.POST("/1/envelope/", handler.HandleEnvelope)
	router.POST("/api/envelope", handler.HandleEnvelope)
	router.POST("/api/envelope/", handler.HandleEnvelope)
	router.POST("/api/:projectNumber/envelope", handler.HandleEnvelopeWithProject)
	router.POST("/api/:projectNumber/envelope/", handler.HandleEnvelopeWithProject)
	return router
}

func newGzipEnvelope(body string) []byte {
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	_, _ = gz.Write([]byte(body))
	_ = gz.Close()
	return buffer.Bytes()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newInMemoryHTTPClient(handler http.Handler) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			return recorder.Result(), nil
		}),
	}
}

// TestSentryIngestRoute verifies /1/envelope and /api/envelope route coverage
func TestSentryIngestRoute(t *testing.T) {
	project := &errordom.Project{
		ID:            "project-1",
		ProjectNumber: 12345,
		Status:        "active",
	}

	projectStore := &fakeSentryProjectStore{project: project}
	eventStore := &fakeSentryEventStore{}
	processor := &fakeSentryEventProcessor{}
	router := newSentryIngestRouter(projectStore, eventStore, processor)

	envelope := `{"event_id":"evt-abc","sent_at":"2024-01-01T00:00:00Z"}
{"type":"event"}
{"message":"test error","level":"error","platform":"go"}`

	t.Run("canonical /1/envelope accepts envelope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/1/envelope", bytes.NewReader([]byte(envelope)))
		req.Header.Set("Content-Type", "application/x-sentry-envelope")
		req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=test-key,sentry_version=7")
		resp := httptest.NewRecorder()

		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "test-key", projectStore.gotProjectByKey)
		assert.Equal(t, 1, eventStore.createCalled)
		assert.Equal(t, 1, processor.processCalled)
		assert.True(t, projectStore.incrementCalled)
	})

	t.Run("compatibility /api/envelope accepts envelope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/envelope", bytes.NewReader([]byte(envelope)))
		req.Header.Set("Content-Type", "application/x-sentry-envelope")
		req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=test-key,sentry_version=7")
		resp := httptest.NewRecorder()

		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("project-number route validates path project id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/12345/envelope", bytes.NewReader([]byte(envelope)))
		req.Header.Set("Content-Type", "application/x-sentry-envelope")
		req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=test-key,sentry_version=7")
		resp := httptest.NewRecorder()

		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("project-number route accepts trailing slash", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/12345/envelope/", bytes.NewReader([]byte(envelope)))
		req.Header.Set("Content-Type", "application/x-sentry-envelope")
		req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=test-key,sentry_version=7")
		resp := httptest.NewRecorder()

		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("invalid path project id is rejected", func(t *testing.T) {
		invalidReq := httptest.NewRequest(http.MethodPost, "/api/not-a-number/envelope", bytes.NewReader([]byte(envelope)))
		invalidReq.Header.Set("Content-Type", "application/x-sentry-envelope")
		invalidReq.Header.Set("X-Sentry-Auth", "Sentry sentry_key=test-key,sentry_version=7")
		invalidResp := httptest.NewRecorder()
		router.ServeHTTP(invalidResp, invalidReq)
		assert.Equal(t, http.StatusBadRequest, invalidResp.Code)
	})

	t.Run("project-number mismatch is rejected", func(t *testing.T) {
		mismatchReq := httptest.NewRequest(http.MethodPost, "/api/99999/envelope", bytes.NewReader([]byte(envelope)))
		mismatchReq.Header.Set("Content-Type", "application/x-sentry-envelope")
		mismatchReq.Header.Set("X-Sentry-Auth", "Sentry sentry_key=test-key,sentry_version=7")
		mismatchResp := httptest.NewRecorder()
		router.ServeHTTP(mismatchResp, mismatchReq)
		assert.Equal(t, http.StatusUnauthorized, mismatchResp.Code)
	})

	t.Run("missing auth is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/1/envelope", bytes.NewReader([]byte(envelope)))
		req.Header.Set("Content-Type", "application/x-sentry-envelope")
		resp := httptest.NewRecorder()

		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})

	t.Run("missing content-type is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/1/envelope", bytes.NewReader([]byte(envelope)))
		req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=test-key,sentry_version=7")
		resp := httptest.NewRecorder()

		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("gzip request body is supported", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/1/envelope", bytes.NewReader(newGzipEnvelope(envelope)))
		req.Header.Set("Content-Type", "application/x-sentry-envelope")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=test-key,sentry_version=7")
		resp := httptest.NewRecorder()

		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
	})
}

// ============================================================================
// MOCK SERVER TESTS FOR CI
// ============================================================================

// TestMockSentryIngestServer provides a socket-free mock HTTP server for testing.
func TestMockSentryIngestServer(t *testing.T) {
	receivedEnvelopes := make([][]byte, 0)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/envelope/") {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			receivedEnvelopes = append(receivedEnvelopes, body)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"id": "test-123"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	client := newInMemoryHTTPClient(handler)

	envelope := `{"event_id":"test-123"}
{"type":"event"}
{"message":"Test error","level":"error"}`

	req, _ := http.NewRequest("POST", "http://app.mbr.test/api/envelope/", bytes.NewReader([]byte(envelope)))
	req.Header.Set("Content-Type", "application/x-sentry-envelope")
	req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=test,sentry_version=7")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, receivedEnvelopes, 1)
}

// TestCIIntegration runs against real server when CI env vars are set
func TestCIIntegration(t *testing.T) {
	serverURL := os.Getenv("MBR_TEST_SERVER_URL")
	projectKey := os.Getenv("MBR_TEST_PROJECT_KEY")

	if serverURL == "" || projectKey == "" {
		t.Skip("CI environment variables not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	envelope := fmt.Sprintf(`{"event_id":"ci-%d"}
{"type":"event"}
{"message":"CI test","level":"info"}`, time.Now().UnixNano())

	req, _ := http.NewRequestWithContext(ctx, "POST", serverURL+"/api/envelope/", strings.NewReader(envelope))
	req.Header.Set("Content-Type", "application/x-sentry-envelope")
	req.Header.Set("X-Sentry-Auth", fmt.Sprintf("Sentry sentry_key=%s,sentry_version=7", projectKey))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
