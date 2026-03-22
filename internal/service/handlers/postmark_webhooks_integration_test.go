//go:build integration

package servicehandlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/pkg/logger"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHandleInboundEmail_WorkspaceValidation(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := t.Context()
	log := logger.New()

	// Create a valid workspace
	validWorkspace := &platformdomain.Workspace{
		ID:       "valid-workspace-id",
		Name:     "Valid Workspace",
		Slug:     "valid-workspace",
		IsActive: true,
	}
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
			"To":      "valid-workspace-id@support.movebigrocks.test",
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
			"To":          "valid-workspace-id@support.movebigrocks.test",
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
