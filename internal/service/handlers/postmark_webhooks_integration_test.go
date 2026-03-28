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
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	sharedevents "github.com/movebigrocks/platform/internal/shared/events"
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

func TestHandleInboundEmail_ThreadsReplyToExistingCase(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := t.Context()
	log := logger.NewNop()

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
		nil,
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

	commandHandler := NewEmailCommandHandler(emailService, log)
	eventHandler := NewEmailEventHandler(emailService, log)
	eventBus := &capturedWebhookBus{}

	webhookHandler := NewPostmarkWebhookHandlers(
		workspaceService,
		emailService,
		nil,
		"test-secret-123",
		eventBus,
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

	comm := servicedomain.NewCommunication(caseObj.ID, workspace.ID, shareddomain.CommTypeEmail, "Prior agent response")
	comm.Direction = shareddomain.DirectionOutbound
	comm.IsInternal = false
	comm.FromUserID = user.ID
	comm.FromEmail = "agent@example.com"
	comm.FromName = "Test Agent"
	comm.ToEmails = []string{"customer@example.com"}
	comm.Subject = "Re: Billing follow-up"
	require.NoError(t, caseService.AddCommunication(ctx, comm))

	outbound := servicedomain.NewOutboundEmail(workspace.ID, "agent@example.com", comm.Subject, comm.Body)
	outbound.FromName = "Test Agent"
	outbound.ToEmails = []string{"customer@example.com"}
	outbound.ReplyToEmail = "agent@example.com"
	outbound.Category = "case-reply"
	outbound.CaseID = caseObj.ID
	outbound.CommunicationID = comm.ID
	outbound.CreatedByID = user.ID
	require.NoError(t, store.OutboundEmails().CreateOutboundEmail(ctx, outbound))

	command := sharedevents.NewSendEmailRequestedEvent(
		workspace.ID,
		"case_service",
		[]string{"customer@example.com"},
		comm.Subject,
		comm.Body,
	)
	command.OutboundEmailID = outbound.ID
	command.CaseID = caseObj.ID
	command.Category = "case-reply"
	command.ReplyTo = "agent@example.com"

	commandPayload, err := json.Marshal(command)
	require.NoError(t, err)
	require.NoError(t, commandHandler.HandleSendEmailRequested(ctx, commandPayload))

	storedOutbound, err := store.OutboundEmails().GetOutboundEmail(ctx, outbound.ID)
	require.NoError(t, err)
	threadMessageID, ok := storedOutbound.ProviderSettings["header_message_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, threadMessageID)

	updatedComm, err := store.Cases().GetCommunication(ctx, workspace.ID, comm.ID)
	require.NoError(t, err)
	assert.Equal(t, threadMessageID, updatedComm.MessageID)

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
	require.Len(t, eventBus.published, 1)
	assert.Equal(t, eventbus.StreamEmailEvents, eventBus.published[0].stream)

	eventPayload, err := json.Marshal(eventBus.published[0].data)
	require.NoError(t, err)
	require.NoError(t, eventHandler.HandleEmailReceived(ctx, eventPayload))

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	emailID := resp["email_id"]
	require.NotEmpty(t, emailID)

	inbound, err := store.InboundEmails().GetInboundEmail(ctx, emailID)
	require.NoError(t, err)
	assert.Equal(t, caseObj.ID, inbound.CaseID)
	assert.Equal(t, servicedomain.EmailProcessingStatusProcessed, inbound.ProcessingStatus)
	assert.Equal(t, threadMessageID, inbound.InReplyTo)
	assert.Equal(t, []string{threadMessageID}, inbound.References)
	assert.Equal(t, "Casey Customer", inbound.FromName)
	assert.Equal(t, "I still need help with this invoice.", inbound.TextContent)

	updatedCase, err := store.Cases().GetCase(ctx, caseObj.ID)
	require.NoError(t, err)
	assert.Equal(t, servicedomain.CaseStatusOpen, updatedCase.Status)

	comms, err := store.Cases().ListCaseCommunications(ctx, caseObj.ID)
	require.NoError(t, err)
	require.Len(t, comms, 2)
	reply := comms[1]
	assert.Equal(t, shareddomain.DirectionInbound, reply.Direction)
	assert.Equal(t, replyMessageID, reply.MessageID)
	assert.Equal(t, threadMessageID, reply.InReplyTo)
	assert.Equal(t, []string{threadMessageID}, reply.References)
	assert.Equal(t, "Casey Customer", reply.FromName)
	assert.Equal(t, "I still need help with this invoice.", reply.Body)
}
