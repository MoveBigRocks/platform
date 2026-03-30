//go:build integration

package servicehandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/pkg/eventbus"
	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil/workflowproof"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil/workflowruntime"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupFormTestStore(t *testing.T) (stores.Store, func()) {
	return testutil.SetupTestStore(t)
}

// mockOutbox implements contracts.OutboxPublisher for testing
type formMockOutbox struct {
	publishedEvents []interface{}
	publishErr      error
	publishCalls    int
	failOnCall      int
}

func (m *formMockOutbox) Publish(ctx context.Context, stream eventbus.Stream, event interface{}) error {
	m.publishCalls++
	if m.publishErr != nil && (m.failOnCall == 0 || m.publishCalls == m.failOnCall) {
		return m.publishErr
	}
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}

func (m *formMockOutbox) PublishEvent(ctx context.Context, stream eventbus.Stream, event eventbus.Event) error {
	m.publishCalls++
	if m.publishErr != nil && (m.failOnCall == 0 || m.publishCalls == m.failOnCall) {
		return m.publishErr
	}
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}

func setupFormHandler(t *testing.T) (*FormPublicHandler, stores.Store, *formMockOutbox, func()) {
	store, cleanup := setupFormTestStore(t)
	outbox := &formMockOutbox{publishedEvents: []interface{}{}}
	// Create FormService with all dependencies for testing
	formService := automationservices.NewFormServiceWithDeps(
		store.Forms(),
		store.Users(), // RateLimitStore
		store,         // TransactionRunner
		store,         // TenantContextSetter
		outbox,
	)
	handler := NewFormPublicHandler(formService)
	return handler, store, outbox, cleanup
}

func setupPublicFormRouter(handler *FormPublicHandler) *gin.Engine {
	router := gin.New()
	v1 := router.Group("/v1/forms")
	v1.GET("/:crypto_id", handler.RenderPublicForm)
	v1.POST("/:crypto_id/submit", handler.SubmitPublicForm)
	v1.GET("/:crypto_id/embed.js", handler.GetEmbedScript)
	apiSubmit := v1.Group("")
	apiSubmit.Use(handler.FormAPITokenMiddleware())
	apiSubmit.POST("/:crypto_id/api/submit", handler.SubmitPublicForm)
	return router
}

func performFormRequest(router *gin.Engine, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func createPublicForm(t *testing.T, store stores.Store, workspaceID, name, slug string) *servicedomain.FormSchema {
	workspaceID = ensureWorkspaceFixture(t, store, workspaceID)

	form := servicedomain.NewFormSchema(workspaceID, name, slug, "user-1")
	form.IsPublic = true
	form.Status = servicedomain.FormStatusActive
	form.SchemaData = shareddomain.TypedSchemaFromMap(map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":  "string",
				"title": "Name",
			},
			"email": map[string]interface{}{
				"type":   "string",
				"title":  "Email",
				"format": "email",
			},
		},
		"required": []string{"name", "email"},
	})
	err := store.Forms().CreateFormSchema(context.Background(), form)
	require.NoError(t, err)
	return form
}

func ensureWorkspaceFixture(t *testing.T, store stores.Store, workspaceID string) string {
	t.Helper()
	ctx := context.Background()
	if _, err := uuid.Parse(workspaceID); err == nil {
		if _, err := store.Workspaces().GetWorkspace(ctx, workspaceID); err == nil {
			return workspaceID
		} else if !errors.Is(err, shared.ErrNotFound) {
			require.NoError(t, err)
		}
	}
	if workspace, err := store.Workspaces().GetWorkspaceBySlug(ctx, workspaceID); err == nil {
		return workspace.ID
	} else if !errors.Is(err, shared.ErrNotFound) {
		require.NoError(t, err)
	}

	workspace := testutil.NewIsolatedWorkspace(t)
	workspace.Name = "Workspace " + workspaceID
	workspace.Slug = workspaceID

	err := store.Workspaces().CreateWorkspace(ctx, workspace)
	require.NoError(t, err)
	return workspace.ID
}

func createEmbedForm(t *testing.T, store stores.Store, workspaceID, name, slug string, domains []string) *servicedomain.FormSchema {
	form := createPublicForm(t, store, workspaceID, name, slug)
	form.AllowEmbed = true
	form.EmbedDomains = domains
	err := store.Forms().UpdateFormSchema(context.Background(), form)
	require.NoError(t, err)
	return form
}

// Test NewFormPublicHandler
func TestNewFormPublicHandler(t *testing.T) {
	handler, _, outbox, cleanup := setupFormHandler(t)
	defer cleanup()

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.formService)
	assert.NotNil(t, outbox)
}

// NOTE: RenderPublicForm returns HTML which requires a template engine.
// The HTML rendering is tested via integration tests. Here we test the
// validation logic through the SubmitPublicForm endpoint which returns JSON.

// Test SubmitPublicForm
func TestSubmitPublicForm(t *testing.T) {
	handler, store, outbox, cleanup := setupFormHandler(t)
	defer cleanup()
	router := setupPublicFormRouter(handler)

	t.Run("returns 404 for non-existent form", func(t *testing.T) {
		body := `{"name": "Test", "email": "test@example.com"}`
		w := performFormRequest(router, http.MethodPost, "/v1/forms/nonexistent/submit", bytes.NewBufferString(body), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 403 for non-public form", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_submit", "Private Submit", "private-submit")
		form.IsPublic = false
		form.AllowEmbed = false
		store.Forms().UpdateFormSchema(context.Background(), form)

		body := `{"name": "Test", "email": "test@example.com"}`
		w := performFormRequest(router, http.MethodPost, "/v1/forms/"+form.CryptoID+"/submit", bytes.NewBufferString(body), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("returns 410 for inactive form", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_submit", "Inactive Submit", "inactive-submit")
		form.Status = servicedomain.FormStatusArchived
		store.Forms().UpdateFormSchema(context.Background(), form)

		body := `{"name": "Test", "email": "test@example.com"}`
		w := performFormRequest(router, http.MethodPost, "/v1/forms/"+form.CryptoID+"/submit", bytes.NewBufferString(body), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, http.StatusGone, w.Code)
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_submit", "Valid Form", "valid-form-json")
		w := performFormRequest(router, http.MethodPost, "/v1/forms/"+form.CryptoID+"/submit", bytes.NewBufferString("invalid json"), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("accepts valid submission", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_submit", "Submit Form", "submit-form")
		body := `{"name": "John Doe", "email": "john@example.com"}`
		w := performFormRequest(router, http.MethodPost, "/v1/forms/"+form.CryptoID+"/submit", bytes.NewBufferString(body), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp["success"].(bool))
		assert.NotEmpty(t, resp["submission_id"])
		assert.NotEmpty(t, resp["message"])
	})

	t.Run("publishes event on submission", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_event", "Event Form", "event-form")
		outbox.publishedEvents = []interface{}{} // Reset
		body := `{"name": "Jane", "email": "jane@example.com"}`
		w := performFormRequest(router, http.MethodPost, "/v1/forms/"+form.CryptoID+"/submit", bytes.NewBufferString(body), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Len(t, outbox.publishedEvents, 1)
	})

	t.Run("rolls back submission when event persistence fails", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_outbox_fail", "Outbox Fail Form", "outbox-fail-form")
		outbox.publishErr = errors.New("outbox unavailable")
		defer func() {
			outbox.publishErr = nil
		}()

		body := `{"name": "Jane", "email": "jane@example.com"}`
		w := performFormRequest(router, http.MethodPost, "/v1/forms/"+form.CryptoID+"/submit", bytes.NewBufferString(body), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		submissions, err := store.Forms().ListFormSubmissions(context.Background(), form.ID)
		require.NoError(t, err)
		assert.Len(t, submissions, 0)
	})

	t.Run("extracts submitter info from data", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_extract", "Extract Form", "extract-form")
		body := `{"name": "Alice Smith", "email": "alice@example.com", "message": "Hello"}`
		w := performFormRequest(router, http.MethodPost, "/v1/forms/"+form.CryptoID+"/submit", bytes.NewBufferString(body), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify submission was stored with submitter info
		submissions, err := store.Forms().ListFormSubmissions(context.Background(), form.ID)
		require.NoError(t, err)
		require.Len(t, submissions, 1)
		assert.Equal(t, "alice@example.com", submissions[0].SubmitterEmail)
		assert.Equal(t, "Alice Smith", submissions[0].SubmitterName)
	})

	t.Run("includes custom success message", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_message", "Message Form", "message-form")
		form.SubmissionMessage = "Thanks for contacting us!"
		store.Forms().UpdateFormSchema(context.Background(), form)
		body := `{"name": "Test", "email": "test@example.com"}`
		w := performFormRequest(router, http.MethodPost, "/v1/forms/"+form.CryptoID+"/submit", bytes.NewBufferString(body), map[string]string{
			"Content-Type": "application/json",
		})

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "Thanks for contacting us!", resp["message"])
	})

	t.Run("includes redirect URL when set", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_redirect", "Redirect Form", "redirect-form")
		form.RedirectURL = "https://example.com/thanks"
		store.Forms().UpdateFormSchema(context.Background(), form)
		body := `{"name": "Test", "email": "test@example.com"}`
		w := performFormRequest(router, http.MethodPost, "/v1/forms/"+form.CryptoID+"/submit", bytes.NewBufferString(body), map[string]string{
			"Content-Type": "application/json",
		})

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "https://example.com/thanks", resp["redirect_url"])
	})
}

func TestSubmitPublicForm_WorkflowCreatesCaseAndSendsNotification(t *testing.T) {
	store, cleanup := setupFormTestStore(t)
	defer cleanup()
	runtime := workflowruntime.NewHarness(t, store)

	formService := automationservices.NewFormServiceWithDeps(
		store.Forms(),
		store.Users(),
		store,
		store,
		runtime.Outbox,
	)
	handler := NewFormPublicHandler(formService)

	form := createPublicForm(t, store, "ws_public_workflow", "Support Request", "support-request")
	form.AutoCreateCase = true
	form.AutoCasePriority = string(servicedomain.CasePriorityHigh)
	form.NotifyOnSubmission = true
	form.NotificationEmails = []string{"support@example.com"}
	require.NoError(t, store.Forms().UpdateFormSchema(context.Background(), form))

	router := setupPublicFormRouter(handler)
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		runtime.Outbox,
		serviceapp.WithTransactionRunner(store),
		serviceapp.WithOutboundEmailStore(store.OutboundEmails()),
	)
	mockProvider := serviceapp.NewMockProvider()
	registry := serviceapp.NewEmailProviderRegistry()
	registry.Register("mock", func(config serviceapp.EmailConfig) (serviceapp.EmailProvider, error) {
		return mockProvider, nil
	})
	emailService, err := serviceapp.NewEmailServiceWithRegistry(store, serviceapp.EmailConfig{
		Provider:         "mock",
		DefaultFromEmail: "support@movebigrocks.test",
		DefaultFromName:  "Support",
	}, caseService, registry)
	require.NoError(t, err)
	formHandler := NewFormEventHandler(formService, caseService, runtime.Outbox, store, nil)
	emailHandler := NewEmailCommandHandler(emailService, nil)
	require.NoError(t, formHandler.RegisterHandlers(runtime.EventBus.Subscribe))
	require.NoError(t, emailHandler.RegisterHandlers(runtime.EventBus.Subscribe))
	runtime.Start(t)

	submissionBody, err := json.Marshal(map[string]interface{}{
		"name":    "Jane Doe",
		"email":   "jane@example.com",
		"message": "Need help with my account",
	})
	require.NoError(t, err)

	response := performFormRequest(
		router,
		http.MethodPost,
		"/v1/forms/"+form.CryptoID+"/submit",
		bytes.NewReader(submissionBody),
		map[string]string{"Content-Type": "application/json"},
	)
	require.Equal(t, http.StatusOK, response.Code)

	var submissionResp map[string]interface{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &submissionResp))
	submissionID, _ := submissionResp["submission_id"].(string)
	require.NotEmpty(t, submissionID)

	var submission *servicedomain.PublicFormSubmission
	var caseObj *servicedomain.Case
	var storedOutbound *servicedomain.OutboundEmail
	require.Eventually(t, func() bool {
		submission, err = store.Forms().GetFormSubmission(context.Background(), submissionID)
		require.NoError(t, err)
		if submission.Status != servicedomain.SubmissionStatusCompleted || submission.CaseID == "" {
			return false
		}
		caseObj, err = store.Cases().GetCase(context.Background(), submission.CaseID)
		require.NoError(t, err)
		for _, sent := range mockProvider.GetSentEmails() {
			storedOutbound, err = store.OutboundEmails().GetOutboundEmailByProviderMessageID(context.Background(), sent.ProviderMessageID)
			if err == nil && storedOutbound.Status == servicedomain.EmailStatusSent {
				return true
			}
		}
		return false
	}, 2*time.Second, 25*time.Millisecond)

	assert.Equal(t, servicedomain.CasePriorityHigh, caseObj.Priority)
	assert.Equal(t, servicedomain.EmailStatusSent, storedOutbound.Status)
	assert.Equal(t, "form", storedOutbound.Category)

	workflowproof.WriteJSON(t, "public-form-case-notification", map[string]interface{}{
		"workspace_id":      form.WorkspaceID,
		"form_id":           form.ID,
		"submission_id":     submission.ID,
		"case_id":           caseObj.ID,
		"outbound_email_id": storedOutbound.ID,
		"outbound_status":   storedOutbound.Status,
	})
}

// Test GetEmbedScript
func TestGetEmbedScript(t *testing.T) {
	handler, store, _, cleanup := setupFormHandler(t)
	defer cleanup()
	router := setupPublicFormRouter(handler)

	t.Run("returns 404 for non-existent form", func(t *testing.T) {
		w := performFormRequest(router, http.MethodGet, "/v1/forms/nonexistent/embed.js", nil, nil)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "Form not found")
	})

	t.Run("returns 403 for non-embeddable form", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_embed", "No Embed", "no-embed-test")
		form.AllowEmbed = false
		form.IsPublic = false
		store.Forms().UpdateFormSchema(context.Background(), form)
		w := performFormRequest(router, http.MethodGet, "/v1/forms/"+form.CryptoID+"/embed.js", nil, nil)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("returns JavaScript for embeddable form", func(t *testing.T) {
		form := createPublicForm(t, store, "ws_embed_js", "Embed OK", "embed-ok-test")
		form.AllowEmbed = true
		store.Forms().UpdateFormSchema(context.Background(), form)
		req := httptest.NewRequest(http.MethodGet, "/v1/forms/"+form.CryptoID+"/embed.js", nil)
		req.Host = "forms.example.com"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/javascript", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), "mbr-form")
		assert.Contains(t, w.Body.String(), "iframe")
	})
}

// Test FormAPITokenMiddleware
func TestFormAPITokenMiddleware(t *testing.T) {
	handler, store, _, cleanup := setupFormHandler(t)
	defer cleanup()

	form := createPublicForm(t, store, "ws_token", "Token Form", "token-form")

	t.Run("passes through without auth header", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/forms/api/submit", nil)

		middleware := handler.FormAPITokenMiddleware()
		middleware(c)

		assert.False(t, c.IsAborted())
	})

	t.Run("returns 401 for invalid token", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/forms/api/submit", nil)
		c.Request.Header.Set("Authorization", "Bearer invalid_token_123")

		middleware := handler.FormAPITokenMiddleware()
		middleware(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("returns 401 for inactive token", func(t *testing.T) {
		inactiveToken := testutil.UniqueID("inactive_token")
		token := &servicedomain.FormAPIToken{
			ID:          testutil.UniqueID("token_inactive"),
			WorkspaceID: form.WorkspaceID,
			FormID:      form.ID,
			Token:       inactiveToken,
			IsActive:    false,
		}
		require.NoError(t, store.Forms().CreateFormAPIToken(context.Background(), token))

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/forms/api/submit", nil)
		c.Request.Header.Set("Authorization", "Bearer "+inactiveToken)

		middleware := handler.FormAPITokenMiddleware()
		middleware(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("returns 401 for expired token", func(t *testing.T) {
		expired := time.Now().Add(-24 * time.Hour)
		expiredToken := testutil.UniqueID("expired_token")
		token := &servicedomain.FormAPIToken{
			ID:          testutil.UniqueID("token_expired"),
			WorkspaceID: form.WorkspaceID,
			FormID:      form.ID,
			Token:       expiredToken,
			IsActive:    true,
			ExpiresAt:   &expired,
		}
		require.NoError(t, store.Forms().CreateFormAPIToken(context.Background(), token))

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/forms/api/submit", nil)
		c.Request.Header.Set("Authorization", "Bearer "+expiredToken)

		middleware := handler.FormAPITokenMiddleware()
		middleware(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("returns 403 for disallowed host", func(t *testing.T) {
		hostToken := testutil.UniqueID("host_token")
		token := &servicedomain.FormAPIToken{
			ID:           testutil.UniqueID("token_host"),
			WorkspaceID:  form.WorkspaceID,
			FormID:       form.ID,
			Token:        hostToken,
			IsActive:     true,
			AllowedHosts: []string{"192.168.1.1"},
		}
		require.NoError(t, store.Forms().CreateFormAPIToken(context.Background(), token))

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/forms/api/submit", nil)
		c.Request.Header.Set("Authorization", "Bearer "+hostToken)
		c.Request.RemoteAddr = "10.0.0.1:12345"

		middleware := handler.FormAPITokenMiddleware()
		middleware(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("accepts valid token", func(t *testing.T) {
		validToken := testutil.UniqueID("valid_token")
		token := &servicedomain.FormAPIToken{
			ID:          testutil.UniqueID("token_valid"),
			WorkspaceID: form.WorkspaceID,
			FormID:      form.ID,
			Token:       validToken,
			IsActive:    true,
		}
		require.NoError(t, store.Forms().CreateFormAPIToken(context.Background(), token))

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/forms/api/submit", nil)
		c.Request.Header.Set("Authorization", "Bearer "+validToken)

		middleware := handler.FormAPITokenMiddleware()
		middleware(c)

		assert.False(t, c.IsAborted())
		assert.Equal(t, form.ID, c.GetString("form_id"))
	})

	t.Run("accepts wildcard host", func(t *testing.T) {
		wildcardToken := testutil.UniqueID("wildcard_token")
		token := &servicedomain.FormAPIToken{
			ID:           testutil.UniqueID("token_wildcard"),
			WorkspaceID:  form.WorkspaceID,
			FormID:       form.ID,
			Token:        wildcardToken,
			IsActive:     true,
			AllowedHosts: []string{"*"},
		}
		require.NoError(t, store.Forms().CreateFormAPIToken(context.Background(), token))

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/forms/api/submit", nil)
		c.Request.Header.Set("Authorization", "Bearer "+wildcardToken)
		c.Request.RemoteAddr = "10.0.0.99:54321"

		middleware := handler.FormAPITokenMiddleware()
		middleware(c)

		assert.False(t, c.IsAborted())
	})
}

// Test helper functions
func TestMatchDomain(t *testing.T) {
	tests := []struct {
		name     string
		referer  string
		pattern  string
		expected bool
	}{
		// Exact domain matching
		{
			name:     "exact match",
			referer:  "https://example.com/page",
			pattern:  "example.com",
			expected: true,
		},
		{
			name:     "exact match with path",
			referer:  "https://example.com/some/deep/path",
			pattern:  "example.com",
			expected: true,
		},
		{
			name:     "exact match with port",
			referer:  "https://example.com:8080/page",
			pattern:  "example.com",
			expected: true,
		},
		{
			name:     "no match - different domain",
			referer:  "https://other.com/page",
			pattern:  "example.com",
			expected: false,
		},

		// SECURITY: Path injection should NOT match
		{
			name:     "SECURITY: domain in path should NOT match",
			referer:  "https://other.com/example.com/page",
			pattern:  "example.com",
			expected: false, // Fixed: was true (security vulnerability)
		},
		{
			name:     "SECURITY: domain in query string should NOT match",
			referer:  "https://other.com/page?redirect=example.com",
			pattern:  "example.com",
			expected: false,
		},
		{
			name:     "SECURITY: similar domain should NOT match",
			referer:  "https://malicious-example.com/page",
			pattern:  "example.com",
			expected: false,
		},
		{
			name:     "SECURITY: domain as subdomain of attacker should NOT match",
			referer:  "https://example.com.attacker.com/page",
			pattern:  "example.com",
			expected: false,
		},

		// Wildcard subdomain matching
		{
			name:     "wildcard subdomain match",
			referer:  "https://app.example.com/page",
			pattern:  "*.example.com",
			expected: true,
		},
		{
			name:     "wildcard any subdomain",
			referer:  "https://deep.sub.example.com/page",
			pattern:  "*.example.com",
			expected: true,
		},
		{
			name:     "wildcard does NOT match base domain",
			referer:  "https://example.com/page",
			pattern:  "*.example.com",
			expected: false, // *.example.com should only match subdomains
		},
		{
			name:     "wildcard does NOT match different domain",
			referer:  "https://other.com/page",
			pattern:  "*.example.com",
			expected: false,
		},

		// SECURITY: Wildcard bypass attempts
		{
			name:     "SECURITY: wildcard in path should NOT match",
			referer:  "https://other.com/sub.example.com/page",
			pattern:  "*.example.com",
			expected: false,
		},
		{
			name:     "SECURITY: wildcard similar domain should NOT match",
			referer:  "https://malicious-example.com/page",
			pattern:  "*.example.com",
			expected: false,
		},
		{
			name:     "SECURITY: wildcard domain suffix attack should NOT match",
			referer:  "https://sub.example.com.attacker.com/page",
			pattern:  "*.example.com",
			expected: false,
		},

		// Case insensitivity
		{
			name:     "case insensitive match",
			referer:  "https://EXAMPLE.COM/page",
			pattern:  "example.com",
			expected: true,
		},
		{
			name:     "case insensitive pattern",
			referer:  "https://example.com/page",
			pattern:  "EXAMPLE.COM",
			expected: true,
		},

		// Edge cases
		{
			name:     "invalid URL returns false",
			referer:  "not-a-valid-url",
			pattern:  "example.com",
			expected: false,
		},
		{
			name:     "empty referer returns false",
			referer:  "",
			pattern:  "example.com",
			expected: false,
		},
		{
			name:     "http scheme works",
			referer:  "http://example.com/page",
			pattern:  "example.com",
			expected: true,
		},
		{
			name:     "localhost matching",
			referer:  "http://localhost:3000/page",
			pattern:  "localhost",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchDomain(tt.referer, tt.pattern)
			assert.Equal(t, tt.expected, result, "matchDomain(%q, %q)", tt.referer, tt.pattern)
		})
	}
}

func TestExtractFormFields(t *testing.T) {
	t.Run("extracts fields from schema", func(t *testing.T) {
		form := &servicedomain.FormSchema{
			SchemaData: shareddomain.TypedSchemaFromMap(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":  "string",
						"title": "Full Name",
					},
					"email": map[string]interface{}{
						"type":        "string",
						"title":       "Email Address",
						"description": "Your email",
					},
				},
				"required": []string{"name"},
			}),
		}

		fields := extractFormFields(form)

		assert.Len(t, fields, 2)

		// Find name field
		var nameField, emailField map[string]interface{}
		for _, f := range fields {
			if f["name"] == "name" {
				nameField = f
			}
			if f["name"] == "email" {
				emailField = f
			}
		}

		assert.NotNil(t, nameField)
		assert.Equal(t, "string", nameField["type"])
		assert.Equal(t, "Full Name", nameField["label"])
		assert.True(t, nameField["required"].(bool))

		assert.NotNil(t, emailField)
		assert.Equal(t, "Your email", emailField["description"])
	})

	t.Run("handles enum options", func(t *testing.T) {
		form := &servicedomain.FormSchema{
			SchemaData: shareddomain.TypedSchemaFromMap(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"priority": map[string]interface{}{
						"type":  "string",
						"title": "Priority",
						"enum":  []interface{}{"low", "medium", "high"},
					},
				},
			}),
		}

		fields := extractFormFields(form)

		require.Len(t, fields, 1)
		assert.NotNil(t, fields[0]["options"])
	})

	t.Run("handles validation constraints", func(t *testing.T) {
		form := &servicedomain.FormSchema{
			SchemaData: shareddomain.TypedSchemaFromMap(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":      "string",
						"title":     "Message",
						"minLength": 10,
						"maxLength": 1000,
					},
				},
			}),
		}

		fields := extractFormFields(form)

		require.Len(t, fields, 1)
		assert.Equal(t, 10, fields[0]["minLength"])
		assert.Equal(t, 1000, fields[0]["maxLength"])
	})

	t.Run("returns empty for invalid schema", func(t *testing.T) {
		form := &servicedomain.FormSchema{
			SchemaData: shareddomain.TypedSchemaFromMap(map[string]interface{}{
				"invalid": "schema",
			}),
		}

		fields := extractFormFields(form)

		assert.Empty(t, fields)
	})
}
