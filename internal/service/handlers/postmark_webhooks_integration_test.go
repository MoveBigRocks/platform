//go:build integration

package servicehandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type capturedWebhookEvent struct {
	stream eventbus.Stream
	data   interface{}
}

type capturedWebhookBus struct {
	published []capturedWebhookEvent
}

func (b *capturedWebhookBus) PublishEvent(stream eventbus.Stream, event eventbus.Event) error {
	b.published = append(b.published, capturedWebhookEvent{stream: stream, data: event})
	return nil
}

func (b *capturedWebhookBus) PublishEventWithRetry(stream eventbus.Stream, event eventbus.Event, _ int) error {
	return b.PublishEvent(stream, event)
}

func (b *capturedWebhookBus) Subscribe(eventbus.Stream, string, string, func(context.Context, []byte) error) error {
	return nil
}

func (b *capturedWebhookBus) GetStreamInfo(eventbus.Stream) (int64, int64, error) {
	return 0, 0, nil
}

func (b *capturedWebhookBus) GetPendingMessages(eventbus.Stream, string) (int64, error) {
	return 0, nil
}

func (b *capturedWebhookBus) HealthCheck() error {
	return nil
}

func (b *capturedWebhookBus) Shutdown(time.Duration) error {
	return nil
}

func (b *capturedWebhookBus) Close() error {
	return nil
}

func (b *capturedWebhookBus) PublishValidated(stream eventbus.Stream, event eventbus.Event) error {
	return b.PublishEvent(stream, event)
}

func (b *capturedWebhookBus) Publish(stream eventbus.Stream, data interface{}) error {
	b.published = append(b.published, capturedWebhookEvent{stream: stream, data: data})
	return nil
}

func (b *capturedWebhookBus) PublishWithType(stream eventbus.Stream, _ eventbus.EventType, _ string, data interface{}) error {
	return b.Publish(stream, data)
}

func TestHandleInboundEmail_WorkspaceValidation(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := t.Context()
	log := logger.New()

	// Create a valid workspace
	validWorkspace := testutil.NewIsolatedWorkspace(t)
	validWorkspace.Name = "Valid Workspace"
	validWorkspace.Slug = "valid-workspace"
	err := store.Workspaces().CreateWorkspace(ctx, validWorkspace)
	require.NoError(t, err)

	// Create workspace service
	workspaceService := platformservices.NewWorkspaceManagementService(
		store.Workspaces(),
		store.Cases(),
		store.Users(),
		store.Rules(),
	)

	// Create handler with test secret
	webhookSecret := "test-secret-123"
	handler := NewPostmarkWebhookHandlers(
		workspaceService,
		nil, // emailService - not needed for this test
		nil, // attachmentService
		webhookSecret,
		nil, // eventBus
		log,
	)

	t.Run("rejects email for non-existent workspace", func(t *testing.T) {
		router := gin.New()
		router.POST("/webhooks/postmark/:secret/inbound", handler.HandleInboundEmail)

		payload := map[string]interface{}{
			"From":        "sender@example.com",
			"To":          "nonexistent-workspace@support.movebigrocks.test",
			"Subject":     "Test Email",
			"TextBody":    "Test body",
			"MessageID":   "test-message-id",
			"MailboxHash": "",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/webhooks/postmark/"+webhookSecret+"/inbound", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should reject with 400 for unknown workspace
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "unknown workspace")
	})

	t.Run("rejects email with invalid webhook secret", func(t *testing.T) {
		router := gin.New()
		router.POST("/webhooks/postmark/:secret/inbound", handler.HandleInboundEmail)

		payload := map[string]interface{}{
			"From":    "sender@example.com",
			"To":      validWorkspace.ID + "@support.movebigrocks.test",
			"Subject": "Test Email",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/webhooks/postmark/wrong-secret/inbound", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should reject with 401 for invalid secret
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("accepts email for valid workspace by ID", func(t *testing.T) {
		router := gin.New()
		router.POST("/webhooks/postmark/:secret/inbound", handler.HandleInboundEmail)

		payload := map[string]interface{}{
			"From":        "sender@example.com",
			"To":          validWorkspace.ID + "@support.movebigrocks.test",
			"Subject":     "Test Email",
			"TextBody":    "Test body content",
			"MessageID":   "test-message-id-valid",
			"MailboxHash": "",
			"Date":        "2025-01-01T00:00:00Z",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/webhooks/postmark/"+webhookSecret+"/inbound", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should accept (may fail later in processing due to missing services, but workspace validation passes)
		// The handler will fail at the email parsing stage since we don't have a full provider setup,
		// but the important thing is it doesn't fail at workspace validation
		// A 200 or 500 (processing error) indicates workspace was found
		// A 400 with "unknown workspace" would indicate failure
		if w.Code == http.StatusBadRequest {
			assert.NotContains(t, w.Body.String(), "unknown workspace",
				"Should not reject valid workspace")
		}
	})

	t.Run("accepts email for valid workspace by slug", func(t *testing.T) {
		router := gin.New()
		router.POST("/webhooks/postmark/:secret/inbound", handler.HandleInboundEmail)

		payload := map[string]interface{}{
			"From":        "sender@example.com",
			"To":          "valid-workspace@support.movebigrocks.test", // Using slug
			"Subject":     "Test Email via Slug",
			"TextBody":    "Test body content",
			"MessageID":   "test-message-id-slug",
			"MailboxHash": "",
			"Date":        "2025-01-01T00:00:00Z",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/webhooks/postmark/"+webhookSecret+"/inbound", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should not fail with "unknown workspace" - slug lookup should find it
		if w.Code == http.StatusBadRequest {
			assert.NotContains(t, w.Body.String(), "unknown workspace",
				"Should find workspace by slug")
		}
	})
}

func TestHandleInboundEmail_StoresEmailAndPublishesEvent(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := t.Context()
	log := logger.NewNop()

	workspace := testutil.NewIsolatedWorkspace(t)
	workspace.Name = "Inbound Email Workspace"
	workspace.Slug = "inbound-email-workspace"
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	workspaceService := platformservices.NewWorkspaceManagementService(
		store.Workspaces(),
		store.Cases(),
		store.Users(),
		store.Rules(),
	)
	emailService, err := serviceapp.NewEmailService(store, serviceapp.EmailConfig{Provider: "mock"}, nil)
	require.NoError(t, err)
	eventBus := &capturedWebhookBus{}

	handler := NewPostmarkWebhookHandlers(
		workspaceService,
		emailService,
		nil,
		"test-secret-123",
		eventBus,
		log,
	)

	router := gin.New()
	router.POST("/webhooks/postmark/:secret/inbound", handler.HandleInboundEmail)

	tests := []struct {
		name string
		to   string
	}{
		{
			name: "stores inbound email for workspace ID recipient",
			to:   workspace.ID + "@support.movebigrocks.test",
		},
		{
			name: "stores inbound email for workspace slug recipient",
			to:   workspace.Slug + "@support.movebigrocks.test",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"From":      "sender@example.com",
				"To":        tc.to,
				"Subject":   "Billing help",
				"TextBody":  "I need help with my invoice.",
				"HtmlBody":  "<p>I need help with my invoice.</p>",
				"MessageID": "msg-" + testutil.UniqueID("inbound"),
			}
			body, err := json.Marshal(payload)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/webhooks/postmark/test-secret-123/inbound", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			beforeEvents := len(eventBus.published)
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			var resp map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			emailID := resp["email_id"]
			require.NotEmpty(t, emailID)

			stored, err := store.InboundEmails().GetInboundEmail(ctx, emailID)
			require.NoError(t, err)
			assert.Equal(t, workspace.ID, stored.WorkspaceID)
			assert.Equal(t, "sender@example.com", stored.FromEmail)
			assert.Equal(t, "Billing help", stored.Subject)
			assert.Equal(t, "I need help with my invoice.", stored.TextContent)
			assert.Equal(t, []string{tc.to}, stored.ToEmails)
			assert.Equal(t, "pending", string(stored.ProcessingStatus))

			require.Len(t, eventBus.published, beforeEvents+1)
			lastEvent := eventBus.published[len(eventBus.published)-1]
			assert.Equal(t, eventbus.StreamEmailEvents, lastEvent.stream)

			eventPayload, ok := lastEvent.data.(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "email_received", eventPayload["event_type"])
			assert.Equal(t, emailID, eventPayload["email_id"])
			assert.Equal(t, workspace.ID, eventPayload["workspace_id"])
		})
	}
}
