package servicehandlers

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/middleware"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/logger"
	"github.com/movebigrocks/platform/pkg/util"
)

const (
	// MaxWebhookBodySize limits incoming webhook payloads to prevent DoS attacks.
	// 10MB is generous for email webhooks (Postmark max attachment is ~10MB).
	MaxWebhookBodySize = 10 * 1024 * 1024
)

// PostmarkWebhookHandlers handles Postmark webhook endpoints
type PostmarkWebhookHandlers struct {
	workspaceService  *platformservices.WorkspaceManagementService
	emailService      *serviceapp.EmailService
	attachmentService *serviceapp.AttachmentService
	attachmentStore   attachmentCaseStore
	webhookSecret     string
	eventBus          eventbus.Bus
	logger            *logger.Logger
}

// NewPostmarkWebhookHandlers creates a new Postmark webhook handler
func NewPostmarkWebhookHandlers(
	workspaceService *platformservices.WorkspaceManagementService,
	emailService *serviceapp.EmailService,
	attachmentService *serviceapp.AttachmentService,
	attachmentStore attachmentCaseStore,
	webhookSecret string,
	eventBus eventbus.Bus,
	logger *logger.Logger,
) *PostmarkWebhookHandlers {
	return &PostmarkWebhookHandlers{
		workspaceService:  workspaceService,
		emailService:      emailService,
		attachmentService: attachmentService,
		attachmentStore:   attachmentStore,
		webhookSecret:     webhookSecret,
		eventBus:          eventBus,
		logger:            logger,
	}
}

// HandleInboundEmail handles Postmark inbound email webhooks
// @Summary Receive inbound email from Postmark
// @Description Webhook endpoint for Postmark inbound emails
// @Tags webhooks
// @Accept json
// @Produce json
// @Param secret path string true "Webhook Secret Token"
// @Param body body object true "Postmark Inbound Email Payload"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /webhooks/postmark/{secret}/inbound [post]
func (h *PostmarkWebhookHandlers) HandleInboundEmail(c *gin.Context) {
	// Validate webhook secret using constant-time comparison to prevent timing attacks
	secret := c.Param("secret")
	if subtle.ConstantTimeCompare([]byte(secret), []byte(h.webhookSecret)) != 1 {
		h.logger.Warn("Invalid webhook secret attempt", "ip", c.ClientIP())
		middleware.RespondWithError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Read raw body with size limit to prevent DoS attacks
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, MaxWebhookBodySize))
	if err != nil {
		h.logger.WithError(err).Error("Failed to read webhook body")
		middleware.RespondWithError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if int64(len(body)) >= MaxWebhookBodySize {
		h.logger.Warn("Webhook body size limit exceeded", "ip", c.ClientIP())
		middleware.RespondWithError(c, http.StatusRequestEntityTooLarge, "request too large")
		return
	}

	// Parse Postmark webhook payload
	var webhook map[string]interface{}
	if err := json.Unmarshal(body, &webhook); err != nil {
		h.logger.WithError(err).Error("Failed to parse webhook JSON")
		middleware.RespondWithError(c, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Extract workspace ID from "To" email address
	// Format: <workspace-id>@support.movebigrocks.com or support@<workspace-slug>.movebigrocks.com
	toEmail := util.GetString(webhook, "To")
	workspaceID := extractWorkspaceID(toEmail)

	if workspaceID == "" {
		h.logger.Error("Could not determine workspace from email", "to", toEmail)
		middleware.RespondWithError(c, http.StatusBadRequest, "invalid recipient")
		return
	}

	// Validate that workspace exists before processing email
	// This prevents orphaned emails for non-existent workspaces
	workspace, err := h.workspaceService.GetWorkspace(c.Request.Context(), workspaceID)
	if err != nil || workspace == nil {
		// Try lookup by slug if ID lookup fails (email might use slug format)
		workspace, err = h.workspaceService.GetWorkspaceBySlug(c.Request.Context(), workspaceID)
		if err != nil || workspace == nil {
			h.logger.Warn("Email received for unknown workspace",
				"to", toEmail,
				"workspace_id", workspaceID)
			middleware.RespondWithError(c, http.StatusBadRequest, "unknown workspace")
			return
		}
		// Use the actual workspace ID from the found workspace
		workspaceID = workspace.ID
	}

	h.logger.Info("Received inbound email webhook",
		"from", util.GetString(webhook, "From"),
		"subject", util.GetString(webhook, "Subject"),
		"workspace_id", workspaceID)

	// Parse inbound email using Postmark provider
	// We'll create a temporary provider to parse the webhook
	provider, err := serviceapp.NewPostmarkProvider(serviceapp.EmailConfig{
		PostmarkServerToken: "", // Not needed for parsing
	})
	if err != nil {
		h.logger.WithError(err).Error("Failed to create Postmark provider")
		middleware.RespondWithError(c, http.StatusInternalServerError, "processing failed")
		return
	}

	inboundEmail, err := provider.ParseInboundEmail(c.Request.Context(), body)
	if err != nil {
		h.logger.WithError(err).Error("Failed to parse inbound email")
		middleware.RespondWithError(c, http.StatusBadRequest, "email parsing failed")
		return
	}

	inboundEmail.WorkspaceID = workspaceID
	inboundEmail.ProcessingStatus = "pending"

	// Process attachments if present and AttachmentService is configured
	if h.attachmentService != nil && h.attachmentStore != nil {
		attachmentIDs, err := h.processInboundAttachments(c.Request.Context(), webhook, workspaceID, inboundEmail.ID)
		if err != nil {
			h.logger.WithError(err).Warn("Some attachments failed to process", "email_id", inboundEmail.ID)
			// Continue processing email even if some attachments fail
		}
		inboundEmail.AttachmentIDs = attachmentIDs
	}

	// Store the inbound email within tenant context for RLS
	// Public webhooks don't go through tenant context middleware, so the service handles it
	if h.emailService == nil {
		h.logger.Error("Email service not configured for inbound webhook")
		middleware.RespondWithError(c, http.StatusInternalServerError, "processing unavailable")
		return
	}
	if err := h.emailService.CreateInboundEmailWithTenantContext(c.Request.Context(), workspaceID, inboundEmail); err != nil {
		h.logger.WithError(err).Error("Failed to store inbound email", "email_id", inboundEmail.ID)
		middleware.RespondWithError(c, http.StatusInternalServerError, "storage failed")
		return
	}

	// Publish event for async processing
	// The email processing worker will handle threading, spam detection, and case creation
	emailEvent := map[string]interface{}{
		"event_type":   "email_received",
		"email_id":     inboundEmail.ID,
		"workspace_id": workspaceID,
		"timestamp":    time.Now().UTC(),
	}

	if h.eventBus != nil {
		if err := h.eventBus.Publish(eventbus.StreamEmailEvents, emailEvent); err != nil {
			h.logger.WithError(err).Error("Failed to publish email event", "email_id", inboundEmail.ID)
			// Continue anyway - email is stored, can be processed manually
		}
	}

	h.logger.Info("Inbound email processed successfully", "email_id", inboundEmail.ID)

	// Return 200 OK to Postmark
	c.JSON(http.StatusOK, gin.H{"status": "accepted", "email_id": inboundEmail.ID})
}

// HandleBounce handles Postmark bounce webhooks
// @Summary Receive bounce notification from Postmark
// @Description Webhook endpoint for Postmark bounce events
// @Tags webhooks
// @Accept json
// @Produce json
// @Param secret path string true "Webhook Secret Token"
// @Param body body object true "Postmark Bounce Payload"
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /webhooks/postmark/{secret}/bounce [post]
func (h *PostmarkWebhookHandlers) HandleBounce(c *gin.Context) {
	// Validate webhook secret using constant-time comparison to prevent timing attacks
	secret := c.Param("secret")
	if subtle.ConstantTimeCompare([]byte(secret), []byte(h.webhookSecret)) != 1 {
		h.logger.Warn("Invalid webhook secret attempt", "ip", c.ClientIP())
		middleware.RespondWithError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Read raw body with size limit to prevent DoS attacks
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, MaxWebhookBodySize))
	if err != nil {
		h.logger.WithError(err).Error("Failed to read bounce webhook body")
		middleware.RespondWithError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if int64(len(body)) >= MaxWebhookBodySize {
		h.logger.Warn("Webhook body size limit exceeded", "ip", c.ClientIP())
		middleware.RespondWithError(c, http.StatusRequestEntityTooLarge, "request too large")
		return
	}

	var webhook map[string]interface{}
	if err := json.Unmarshal(body, &webhook); err != nil {
		h.logger.WithError(err).Error("Failed to parse bounce webhook JSON")
		middleware.RespondWithError(c, http.StatusBadRequest, "invalid JSON")
		return
	}

	messageID := util.GetString(webhook, "MessageID")
	bounceType := util.GetString(webhook, "Type") // "HardBounce" or "SoftBounce"
	emailAddr := util.GetString(webhook, "Email")
	description := util.GetString(webhook, "Description")

	h.logger.Info("Received bounce webhook",
		"message_id", messageID,
		"type", bounceType,
		"email", emailAddr,
		"description", description)

	// Use email service to update status (handles admin context internally)
	outboundEmail, err := h.emailService.MarkOutboundEmailBounced(c.Request.Context(), messageID, bounceType, description)
	if err != nil {
		h.logger.WithError(err).Warn("Could not process bounce webhook", "message_id", messageID)
		// Still return 200 OK to Postmark to prevent retries
		c.JSON(http.StatusOK, gin.H{"status": "accepted", "warning": "email not found or update failed"})
		return
	}

	h.logger.Info("Bounce processed successfully",
		"message_id", messageID,
		"email_id", outboundEmail.ID,
		"type", bounceType)

	c.JSON(http.StatusOK, gin.H{"status": "accepted", "email_id": outboundEmail.ID})
}

// HandleDelivery handles Postmark delivery webhooks
// @Summary Receive delivery notification from Postmark
// @Description Webhook endpoint for Postmark delivery events
// @Tags webhooks
// @Accept json
// @Produce json
// @Param secret path string true "Webhook Secret Token"
// @Param body body object true "Postmark Delivery Payload"
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /webhooks/postmark/{secret}/delivery [post]
func (h *PostmarkWebhookHandlers) HandleDelivery(c *gin.Context) {
	// Validate webhook secret using constant-time comparison to prevent timing attacks
	secret := c.Param("secret")
	if subtle.ConstantTimeCompare([]byte(secret), []byte(h.webhookSecret)) != 1 {
		h.logger.Warn("Invalid webhook secret attempt", "ip", c.ClientIP())
		middleware.RespondWithError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Read raw body with size limit to prevent DoS attacks
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, MaxWebhookBodySize))
	if err != nil {
		h.logger.WithError(err).Error("Failed to read delivery webhook body")
		middleware.RespondWithError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if int64(len(body)) >= MaxWebhookBodySize {
		h.logger.Warn("Webhook body size limit exceeded", "ip", c.ClientIP())
		middleware.RespondWithError(c, http.StatusRequestEntityTooLarge, "request too large")
		return
	}

	var webhook map[string]interface{}
	if err := json.Unmarshal(body, &webhook); err != nil {
		h.logger.WithError(err).Error("Failed to parse delivery webhook JSON")
		middleware.RespondWithError(c, http.StatusBadRequest, "invalid JSON")
		return
	}

	messageID := util.GetString(webhook, "MessageID")
	recipient := util.GetString(webhook, "Recipient")
	deliveredAt := util.GetString(webhook, "DeliveredAt")

	h.logger.Info("Received delivery webhook",
		"message_id", messageID,
		"recipient", recipient,
		"delivered_at", deliveredAt)

	// Use email service to update status (handles admin context internally)
	outboundEmail, err := h.emailService.MarkOutboundEmailDelivered(c.Request.Context(), messageID)
	if err != nil {
		h.logger.WithError(err).Warn("Could not process delivery webhook", "message_id", messageID)
		// Still return 200 OK to Postmark to prevent retries
		c.JSON(http.StatusOK, gin.H{"status": "accepted", "warning": "email not found or update failed"})
		return
	}

	h.logger.Info("Delivery processed successfully",
		"message_id", messageID,
		"email_id", outboundEmail.ID)

	c.JSON(http.StatusOK, gin.H{"status": "accepted", "email_id": outboundEmail.ID})
}

func extractWorkspaceID(email string) string {
	// Extract workspace ID from email address
	// Examples:
	// - abc123@support.movebigrocks.com -> workspace ID: abc123
	// - support@mycompany.movebigrocks.com -> workspace slug: mycompany
	// - support@acme.support.movebigrocks.com -> workspace slug: acme

	if email == "" {
		return ""
	}

	// Parse email
	at := strings.IndexByte(email, '@')
	if at == -1 {
		return ""
	}

	localPart := email[:at]
	domainPart := email[at+1:]

	if strings.EqualFold(localPart, "support") {
		if slug := extractWorkspaceSlugFromDomain(domainPart); slug != "" {
			return slug
		}
	}

	return localPart
}

func extractWorkspaceSlugFromDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return ""
	}

	labels := strings.Split(domain, ".")
	if len(labels) < 3 {
		return ""
	}

	switch {
	case labels[0] == "support" && len(labels) >= 4:
		return strings.TrimSpace(labels[1])
	case labels[0] != "support":
		return strings.TrimSpace(labels[0])
	default:
		return ""
	}
}

// processInboundAttachments processes attachments from a Postmark inbound email
func (h *PostmarkWebhookHandlers) processInboundAttachments(
	ctx context.Context,
	webhook map[string]interface{},
	workspaceID string,
	emailID string,
) ([]string, error) {
	attachments, ok := webhook["Attachments"].([]interface{})
	if !ok || len(attachments) == 0 {
		return nil, nil
	}

	var attachmentIDs []string
	var lastErr error

	for _, att := range attachments {
		attMap, ok := att.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract attachment info from Postmark format
		filename := util.GetString(attMap, "Name")
		contentType := util.GetString(attMap, "ContentType")
		content := util.GetString(attMap, "Content")     // Base64 encoded
		contentID := util.GetString(attMap, "ContentID") // For inline images
		contentLength := util.GetInt(attMap, "ContentLength")

		if filename == "" || content == "" {
			continue
		}

		// Decode base64 content
		data, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			h.logger.WithError(err).Warn("Failed to decode attachment",
				"filename", filename,
				"email_id", emailID)
			lastErr = err
			continue
		}

		// Create attachment record
		attachment := servicedomain.NewAttachment(
			workspaceID,
			filename,
			contentType,
			int64(len(data)),
			servicedomain.AttachmentSourceEmail,
		)
		attachment.EmailID = emailID

		// Add content ID for inline images
		if contentID != "" {
			attachment.Metadata["content_id"] = contentID
		}

		// Validate size (use reported size if available, otherwise decoded size)
		if contentLength > 0 {
			attachment.Size = int64(contentLength)
		}

		// Upload with virus scanning
		if err := h.attachmentService.UploadFromBase64(ctx, attachment, data); err != nil {
			h.logger.WithError(err).Warn("Failed to upload attachment",
				"filename", filename,
				"email_id", emailID,
				"attachment_id", attachment.ID)
			lastErr = err

			if persistErr := h.attachmentStore.SaveAttachment(ctx, attachment, nil); persistErr != nil {
				h.logger.WithError(persistErr).Warn("Failed to persist failed attachment metadata",
					"filename", filename,
					"email_id", emailID,
					"attachment_id", attachment.ID)
				continue
			}
			attachmentIDs = append(attachmentIDs, attachment.ID)
			continue
		}

		if err := h.attachmentStore.SaveAttachment(ctx, attachment, nil); err != nil {
			h.logger.WithError(err).Warn("Failed to persist attachment metadata",
				"filename", filename,
				"email_id", emailID,
				"attachment_id", attachment.ID)
			lastErr = err
			continue
		}

		attachmentIDs = append(attachmentIDs, attachment.ID)

		h.logger.Info("Processed inbound attachment",
			"attachment_id", attachment.ID,
			"filename", filename,
			"size", attachment.Size,
			"email_id", emailID)
	}

	return attachmentIDs, lastErr
}
