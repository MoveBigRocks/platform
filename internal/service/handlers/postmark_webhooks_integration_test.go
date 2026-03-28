//go:build integration

package servicehandlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceresolvers "github.com/movebigrocks/platform/internal/service/resolvers"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/internal/testutil/workflowproof"
	"github.com/movebigrocks/platform/internal/testutil/workflowruntime"
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
		nil, // attachment metadata store
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

func TestHandleInboundEmail_CreatesCaseThroughWorker(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := t.Context()
	log := logger.NewNop()
	runtime := workflowruntime.NewHarness(t, store)

	workspace := testutil.NewIsolatedWorkspace(t)
	workspace.Name = "Inbound Workflow Workspace"
	workspace.Slug = "inbound-workflow-workspace"
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	workspaceService := platformservices.NewWorkspaceManagementService(
		store.Workspaces(),
		store.Cases(),
		store.Users(),
		store.Rules(),
	)
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		runtime.Outbox,
		serviceapp.WithTransactionRunner(store),
	)
	emailService, err := serviceapp.NewEmailService(store, serviceapp.EmailConfig{Provider: "mock"}, caseService)
	require.NoError(t, err)
	eventHandler := NewEmailEventHandler(emailService, log)
	require.NoError(t, eventHandler.RegisterHandlers(runtime.EventBus.Subscribe))
	runtime.Start(t)

	webhookHandler := NewPostmarkWebhookHandlers(
		workspaceService,
		emailService,
		nil,
		nil,
		"test-secret-123",
		runtime.EventBus,
		log,
	)

	router := gin.New()
	router.POST("/webhooks/postmark/:secret/inbound", webhookHandler.HandleInboundEmail)

	messageID := "<new-thread@example.com>"
	payload := map[string]interface{}{
		"From":      "\"Casey Customer\" <customer@example.com>",
		"FromName":  "Casey Customer",
		"FromFull":  map[string]interface{}{"Email": "customer@example.com", "Name": "Casey Customer", "MailboxHash": ""},
		"To":        workspace.ID + "@support.movebigrocks.test",
		"ToFull":    []map[string]interface{}{{"Email": workspace.ID + "@support.movebigrocks.test", "Name": "Support", "MailboxHash": ""}},
		"Subject":   "Need help with my order",
		"TextBody":  "I need an update on my order.",
		"HtmlBody":  "<p>I need an update on my order.</p>",
		"MessageID": messageID,
		"Headers": []map[string]interface{}{
			{"Name": "Message-ID", "Value": messageID},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/postmark/test-secret-123/inbound", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	emailID := resp["email_id"]
	require.NotEmpty(t, emailID)

	var inbound *servicedomain.InboundEmail
	require.Eventually(t, func() bool {
		inbound, err = store.InboundEmails().GetInboundEmail(ctx, emailID)
		require.NoError(t, err)
		return inbound.ProcessingStatus == servicedomain.EmailProcessingStatusProcessed &&
			inbound.CaseID != "" &&
			inbound.CommunicationID != ""
	}, 2*time.Second, 25*time.Millisecond)

	caseObj, err := store.Cases().GetCase(ctx, inbound.CaseID)
	require.NoError(t, err)
	assert.Equal(t, workspace.ID, caseObj.WorkspaceID)
	assert.Equal(t, "customer@example.com", caseObj.ContactEmail)
	assert.Equal(t, "Casey Customer", caseObj.ContactName)
	assert.Equal(t, servicedomain.CaseChannelEmail, caseObj.Channel)

	comms, err := store.Cases().ListCaseCommunications(ctx, caseObj.ID)
	require.NoError(t, err)
	require.Len(t, comms, 1)
	assert.Equal(t, messageID, comms[0].MessageID)
	assert.Equal(t, shareddomain.DirectionInbound, comms[0].Direction)

	workflowproof.WriteJSON(t, "inbound-new-email-case-create", map[string]interface{}{
		"workspace_id":      workspace.ID,
		"inbound_email_id":  inbound.ID,
		"case_id":           caseObj.ID,
		"communication_id":  comms[0].ID,
		"case_status":       caseObj.Status,
		"message_count":     caseObj.MessageCount,
		"processing_status": inbound.ProcessingStatus,
	})
}

func TestHandleInboundEmail_ThreadsReplyToExistingCase(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := t.Context()
	log := logger.NewNop()
	runtime := workflowruntime.NewHarness(t, store)

	workspace := testutil.NewIsolatedWorkspace(t)
	workspace.Name = "Threading Workspace"
	workspace.Slug = "threading-workspace"
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	workspaceService := platformservices.NewWorkspaceManagementService(
		store.Workspaces(),
		store.Cases(),
		store.Users(),
		store.Rules(),
	)

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
	eventHandler := NewEmailEventHandler(emailService, log)
	commandHandler := NewEmailCommandHandler(emailService, log)
	require.NoError(t, eventHandler.RegisterHandlers(runtime.EventBus.Subscribe))
	require.NoError(t, commandHandler.RegisterHandlers(runtime.EventBus.Subscribe))
	runtime.Start(t)

	webhookHandler := NewPostmarkWebhookHandlers(
		workspaceService,
		emailService,
		nil,
		nil,
		"test-secret-123",
		runtime.EventBus,
		log,
	)

	caseObj, err := caseService.CreateCase(ctx, serviceapp.CreateCaseParams{
		WorkspaceID:  workspace.ID,
		Subject:      "Billing follow-up",
		ContactEmail: "customer@example.com",
		ContactName:  "Casey Customer",
		Channel:      servicedomain.CaseChannelEmail,
	})
	require.NoError(t, err)
	require.NoError(t, caseService.SetCaseStatus(ctx, caseObj.ID, servicedomain.CaseStatusPending))

	user := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, user))
	reply, err := caseService.ReplyToCase(ctx, serviceapp.ReplyToCaseParams{
		WorkspaceID: workspace.ID,
		CaseID:      caseObj.ID,
		UserID:      user.ID,
		UserName:    user.Name,
		UserEmail:   user.Email,
		ToEmails:    []string{"customer@example.com"},
		Subject:     "Re: Billing follow-up",
		Body:        "Prior agent response",
	})
	require.NoError(t, err)

	var storedOutbound *servicedomain.OutboundEmail
	require.Eventually(t, func() bool {
		comms, listErr := store.Cases().ListCaseCommunications(ctx, caseObj.ID)
		require.NoError(t, listErr)
		if len(comms) == 0 {
			return false
		}
		for _, comm := range comms {
			if comm.ID != reply.ID {
				continue
			}
			if comm.MessageID == "" {
				return false
			}
			for _, sent := range mockProvider.GetSentEmails() {
				storedOutbound, err = store.OutboundEmails().GetOutboundEmailByProviderMessageID(ctx, sent.ProviderMessageID)
				if err == nil && storedOutbound.CommunicationID == reply.ID {
					return true
				}
			}
		}
		return false
	}, 2*time.Second, 25*time.Millisecond)

	threadMessageID, ok := storedOutbound.ProviderSettings["header_message_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, threadMessageID)

	router := gin.New()
	router.POST("/webhooks/postmark/:secret/inbound", webhookHandler.HandleInboundEmail)

	replyMessageID := "<customer-reply@example.com>"
	webhookPayload := map[string]interface{}{
		"From":              "\"Casey Customer\" <customer@example.com>",
		"FromName":          "Casey Customer",
		"FromFull":          map[string]interface{}{"Email": "customer@example.com", "Name": "Casey Customer", "MailboxHash": ""},
		"To":                workspace.ID + "@support.movebigrocks.test",
		"ToFull":            []map[string]interface{}{{"Email": workspace.ID + "@support.movebigrocks.test", "Name": "Support", "MailboxHash": ""}},
		"Subject":           "Re: Billing follow-up",
		"TextBody":          "Casey Customer wrote:\n> prior reply",
		"StrippedTextReply": "I still need help with this invoice.",
		"HtmlBody":          "<p>I still need help with this invoice.</p>",
		"MessageID":         replyMessageID,
		"Headers": []map[string]interface{}{
			{"Name": "Message-ID", "Value": replyMessageID},
			{"Name": "In-Reply-To", "Value": threadMessageID},
			{"Name": "References", "Value": threadMessageID},
		},
	}
	body, err := json.Marshal(webhookPayload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/postmark/test-secret-123/inbound", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	emailID := resp["email_id"]
	require.NotEmpty(t, emailID)

	var inbound *servicedomain.InboundEmail
	var updatedCase *servicedomain.Case
	var comms []*servicedomain.Communication
	require.Eventually(t, func() bool {
		inbound, err = store.InboundEmails().GetInboundEmail(ctx, emailID)
		require.NoError(t, err)
		updatedCase, err = store.Cases().GetCase(ctx, caseObj.ID)
		require.NoError(t, err)
		comms, err = store.Cases().ListCaseCommunications(ctx, caseObj.ID)
		require.NoError(t, err)
		return inbound.ProcessingStatus == servicedomain.EmailProcessingStatusProcessed &&
			inbound.CaseID == caseObj.ID &&
			updatedCase.Status == servicedomain.CaseStatusOpen &&
			len(comms) == 2
	}, 2*time.Second, 25*time.Millisecond)

	assert.Equal(t, threadMessageID, inbound.InReplyTo)
	assert.Equal(t, []string{threadMessageID}, inbound.References)
	assert.Equal(t, "Casey Customer", inbound.FromName)
	assert.Equal(t, "I still need help with this invoice.", inbound.TextContent)

	inboundReply := comms[1]
	assert.Equal(t, shareddomain.DirectionInbound, inboundReply.Direction)
	assert.Equal(t, replyMessageID, inboundReply.MessageID)
	assert.Equal(t, threadMessageID, inboundReply.InReplyTo)
	assert.Equal(t, []string{threadMessageID}, inboundReply.References)
	assert.Equal(t, "Casey Customer", inboundReply.FromName)
	assert.Equal(t, "I still need help with this invoice.", inboundReply.Body)

	workflowproof.WriteJSON(t, "inbound-reply-threading", map[string]interface{}{
		"workspace_id":             workspace.ID,
		"case_id":                  caseObj.ID,
		"inbound_email_id":         inbound.ID,
		"matched_communication_id": inboundReply.ID,
		"thread_message_id":        threadMessageID,
		"reply_message_id":         replyMessageID,
		"case_status":              updatedCase.Status,
		"message_count":            len(comms),
	})
}

func TestHandleInboundEmail_PersistsAndLinksAttachments(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := t.Context()
	log := logger.NewNop()
	runtime := workflowruntime.NewHarness(t, store)

	workspace := testutil.NewIsolatedWorkspace(t)
	workspace.Name = "Attachment Workflow Workspace"
	workspace.Slug = "attachment-workflow-workspace"
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	operator := testutil.NewIsolatedUser(t, workspace.ID)
	require.NoError(t, store.Users().CreateUser(ctx, operator))

	workspaceService := platformservices.NewWorkspaceManagementService(
		store.Workspaces(),
		store.Cases(),
		store.Users(),
		store.Rules(),
	)
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		runtime.Outbox,
		serviceapp.WithTransactionRunner(store),
	)
	emailService, err := serviceapp.NewEmailService(store, serviceapp.EmailConfig{Provider: "mock"}, caseService)
	require.NoError(t, err)
	eventHandler := NewEmailEventHandler(emailService, log)
	require.NoError(t, eventHandler.RegisterHandlers(runtime.EventBus.Subscribe))
	runtime.Start(t)

	attachmentService, s3Server := newTestAttachmentService(t)
	webhookHandler := NewPostmarkWebhookHandlers(
		workspaceService,
		emailService,
		attachmentService,
		store.Cases(),
		"test-secret-123",
		runtime.EventBus,
		log,
	)

	router := gin.New()
	router.POST("/webhooks/postmark/:secret/inbound", webhookHandler.HandleInboundEmail)

	payload := map[string]interface{}{
		"From":      "\"Casey Customer\" <customer@example.com>",
		"FromName":  "Casey Customer",
		"FromFull":  map[string]interface{}{"Email": "customer@example.com", "Name": "Casey Customer", "MailboxHash": ""},
		"To":        workspace.ID + "@support.movebigrocks.test",
		"ToFull":    []map[string]interface{}{{"Email": workspace.ID + "@support.movebigrocks.test", "Name": "Support", "MailboxHash": ""}},
		"Subject":   "Need help with attachment review",
		"TextBody":  "Please review the attached invoice and note the executable should be rejected.",
		"HtmlBody":  "<p>Please review the attached invoice and note the executable should be rejected.</p>",
		"MessageID": "<attachment-thread@example.com>",
		"Headers": []map[string]interface{}{
			{"Name": "Message-ID", "Value": "<attachment-thread@example.com>"},
		},
		"Attachments": []map[string]interface{}{
			{
				"Name":          "invoice.pdf",
				"ContentType":   "application/pdf",
				"ContentLength": len([]byte("%PDF-1.4 attachment body")),
				"Content":       base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 attachment body")),
			},
			{
				"Name":          "payload.exe",
				"ContentType":   "application/octet-stream",
				"ContentLength": len([]byte("malware-ish bytes")),
				"Content":       base64.StdEncoding.EncodeToString([]byte("malware-ish bytes")),
			},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/postmark/test-secret-123/inbound", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var webhookResp map[string]string
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &webhookResp))
	emailID := webhookResp["email_id"]
	require.NotEmpty(t, emailID)

	var inbound *servicedomain.InboundEmail
	var caseObj *servicedomain.Case
	var comms []*servicedomain.Communication
	var attachments []*servicedomain.Attachment
	require.Eventually(t, func() bool {
		inbound, err = store.InboundEmails().GetInboundEmail(ctx, emailID)
		require.NoError(t, err)
		if inbound.ProcessingStatus != servicedomain.EmailProcessingStatusProcessed || inbound.CaseID == "" || inbound.CommunicationID == "" {
			return false
		}

		caseObj, err = store.Cases().GetCase(ctx, inbound.CaseID)
		require.NoError(t, err)
		comms, err = store.Cases().ListCaseCommunications(ctx, caseObj.ID)
		require.NoError(t, err)
		attachments, err = store.Cases().ListCaseAttachments(ctx, workspace.ID, caseObj.ID)
		require.NoError(t, err)
		return len(inbound.AttachmentIDs) == 2 && len(comms) == 1 && len(attachments) == 2
	}, 3*time.Second, 25*time.Millisecond)

	require.Len(t, s3Server.PutRequests(), 1)
	assert.ElementsMatch(t, []string{attachments[0].ID, attachments[1].ID}, inbound.AttachmentIDs)
	assert.ElementsMatch(t, inbound.AttachmentIDs, comms[0].AttachmentIDs)

	attachmentsByFilename := map[string]*servicedomain.Attachment{}
	for _, attachment := range attachments {
		attachmentsByFilename[attachment.Filename] = attachment
	}
	require.Contains(t, attachmentsByFilename, "invoice.pdf")
	require.Contains(t, attachmentsByFilename, "payload.exe")

	cleanAttachment := attachmentsByFilename["invoice.pdf"]
	assert.Equal(t, servicedomain.AttachmentStatusClean, cleanAttachment.Status)
	assert.Equal(t, inbound.ID, cleanAttachment.EmailID)
	assert.Equal(t, caseObj.ID, cleanAttachment.CaseID)
	assert.NotEmpty(t, cleanAttachment.S3Key)

	rejectedAttachment := attachmentsByFilename["payload.exe"]
	assert.Equal(t, servicedomain.AttachmentStatusError, rejectedAttachment.Status)
	assert.Equal(t, inbound.ID, rejectedAttachment.EmailID)
	assert.Equal(t, caseObj.ID, rejectedAttachment.CaseID)
	assert.Empty(t, rejectedAttachment.S3Key)

	resolver := serviceresolvers.NewResolver(serviceresolvers.Config{CaseService: caseService})
	authCtx := graphshared.SetAuthContext(ctx, &platformdomain.AuthContext{
		Principal:     operator,
		PrincipalType: platformdomain.PrincipalTypeUser,
		WorkspaceID:   workspace.ID,
		WorkspaceIDs:  []string{workspace.ID},
		Permissions:   []string{platformdomain.PermissionCaseRead},
	})
	caseResolver, err := resolver.Case(authCtx, caseObj.ID)
	require.NoError(t, err)
	visibleAttachments, err := caseResolver.Attachments(authCtx)
	require.NoError(t, err)
	require.Len(t, visibleAttachments, 2)

	workflowproof.WriteJSON(t, "inbound-email-attachments", map[string]interface{}{
		"workspace_id":               workspace.ID,
		"case_id":                    caseObj.ID,
		"inbound_email_id":           inbound.ID,
		"communication_id":           comms[0].ID,
		"attachment_ids":             inbound.AttachmentIDs,
		"clean_attachment_id":        cleanAttachment.ID,
		"clean_attachment_status":    cleanAttachment.Status,
		"rejected_attachment_id":     rejectedAttachment.ID,
		"rejected_attachment_status": rejectedAttachment.Status,
		"visible_attachment_count":   len(visibleAttachments),
		"stored_upload_count":        len(s3Server.PutRequests()),
	})
}
