package servicehandlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	apierrors "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/errors"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/middleware"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

const (
	publicConversationRateLimitWindow = time.Minute
	publicConversationBlockDuration   = 5 * time.Minute
	publicConversationMaxAttempts     = 20
)

type publicConversationRateLimitStore interface {
	CheckRateLimit(ctx context.Context, key string, maxAttempts int, window, blockDuration time.Duration) (bool, time.Duration, error)
}

type PublicConversationHandler struct {
	conversationService *serviceapp.ConversationService
	rateLimitStore      publicConversationRateLimitStore
	logger              *logger.Logger
}

type startPublicConversationRequest struct {
	WorkspaceSlug      string                 `json:"workspace_slug"`
	QueueSlug          string                 `json:"queue_slug"`
	ExternalSessionKey string                 `json:"external_session_key"`
	Title              string                 `json:"title"`
	DisplayName        string                 `json:"display_name"`
	ContactEmail       string                 `json:"contact_email"`
	InitialMessage     string                 `json:"initial_message"`
	SourceRef          string                 `json:"source_ref"`
	Metadata           map[string]interface{} `json:"metadata"`
}

type appendPublicConversationMessageRequest struct {
	ExternalSessionKey string `json:"external_session_key"`
	DisplayName        string `json:"display_name"`
	ContactEmail       string `json:"contact_email"`
	Content            string `json:"content"`
}

func NewPublicConversationHandler(conversationService *serviceapp.ConversationService, rateLimitStore publicConversationRateLimitStore) *PublicConversationHandler {
	return &PublicConversationHandler{
		conversationService: conversationService,
		rateLimitStore:      rateLimitStore,
		logger:              logger.New().WithField("handler", "public-conversation"),
	}
}

func (h *PublicConversationHandler) StartConversation(c *gin.Context) {
	if h.conversationService == nil {
		middleware.RespondWithError(c, http.StatusServiceUnavailable, "Public conversation intake is not configured")
		return
	}
	if !h.allowRequest(c, "start") {
		return
	}

	var req startPublicConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid conversation payload")
		return
	}

	result, err := h.conversationService.StartPublicConversation(c.Request.Context(), serviceapp.StartPublicConversationParams{
		WorkspaceSlug:      req.WorkspaceSlug,
		QueueSlug:          req.QueueSlug,
		ExternalSessionKey: req.ExternalSessionKey,
		Title:              req.Title,
		DisplayName:        req.DisplayName,
		ContactEmail:       req.ContactEmail,
		InitialMessage:     req.InitialMessage,
		SourceRef:          req.SourceRef,
		Metadata:           shareddomain.TypedSchemaFromMap(req.Metadata),
	})
	if err != nil {
		h.respondConversationError(c, err)
		return
	}

	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	payload := gin.H{
		"created":              result.Created,
		"session_id":           result.Session.ID,
		"status":               result.Session.Status,
		"channel":              result.Session.Channel,
		"external_session_key": result.Session.ExternalSessionKey,
		"queue_id":             result.QueueID,
		"queue_slug":           result.QueueSlug,
		"source_ref":           result.Session.SourceRef,
	}
	if result.Participant != nil {
		payload["participant_id"] = result.Participant.ID
	}
	if result.Message != nil {
		payload["message_id"] = result.Message.ID
	}

	c.JSON(status, payload)
}

func (h *PublicConversationHandler) AddMessage(c *gin.Context) {
	if h.conversationService == nil {
		middleware.RespondWithError(c, http.StatusServiceUnavailable, "Public conversation intake is not configured")
		return
	}
	if !h.allowRequest(c, "message") {
		return
	}

	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		middleware.RespondWithError(c, http.StatusBadRequest, "Conversation identifier is required")
		return
	}

	var req appendPublicConversationMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid conversation payload")
		return
	}

	result, err := h.conversationService.AppendPublicConversationMessage(c.Request.Context(), sessionID, serviceapp.AppendPublicConversationMessageParams{
		ExternalSessionKey: req.ExternalSessionKey,
		DisplayName:        req.DisplayName,
		ContactEmail:       req.ContactEmail,
		ContentText:        req.Content,
	})
	if err != nil {
		h.respondConversationError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"session_id":     result.Session.ID,
		"status":         result.Session.Status,
		"queue_id":       result.QueueID,
		"participant_id": result.Participant.ID,
		"message_id":     result.Message.ID,
	})
}

func (h *PublicConversationHandler) allowRequest(c *gin.Context, operation string) bool {
	if h.rateLimitStore == nil {
		return true
	}
	key := fmt.Sprintf("public-conversation:%s:%s", operation, c.ClientIP())
	allowed, retryAfter, err := h.rateLimitStore.CheckRateLimit(c.Request.Context(), key, publicConversationMaxAttempts, publicConversationRateLimitWindow, publicConversationBlockDuration)
	if err != nil {
		h.logger.WithError(err).Error("Public conversation rate limit check failed")
		middleware.RespondWithError(c, http.StatusServiceUnavailable, "Service temporarily unavailable. Please try again.")
		return false
	}
	if !allowed {
		c.Header("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
		middleware.RespondWithError(c, http.StatusTooManyRequests, "Too many conversation requests. Please try again later.")
		return false
	}
	return true
}

func (h *PublicConversationHandler) respondConversationError(c *gin.Context, err error) {
	var apiErr *apierrors.APIError
	if errors.As(err, &apiErr) {
		middleware.RespondWithError(c, apiErr.StatusCode, apiErr.Message)
		return
	}
	h.logger.WithError(err).Error("Public conversation request failed")
	middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to process conversation request")
}
