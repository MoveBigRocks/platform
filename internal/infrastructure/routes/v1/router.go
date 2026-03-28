package v1

import (
	"github.com/gin-gonic/gin"

	servicehandlers "github.com/movebigrocks/platform/internal/service/handlers"
)

// Router handles v1 API route registration
type Router struct {
	postmarkWebhookHandlers *servicehandlers.PostmarkWebhookHandlers
	publicFormHandler       *servicehandlers.FormPublicHandler
	publicConversation      *servicehandlers.PublicConversationHandler
}

// NewRouter creates a new v1 router
func NewRouter() *Router {
	return &Router{}
}

// SetPostmarkHandlers sets the Postmark webhook handlers
func (r *Router) SetPostmarkHandlers(handlers *servicehandlers.PostmarkWebhookHandlers) {
	r.postmarkWebhookHandlers = handlers
}

// SetPublicFormHandler sets the public form handler
func (r *Router) SetPublicFormHandler(handler *servicehandlers.FormPublicHandler) {
	r.publicFormHandler = handler
}

// SetPublicConversationHandler sets the public conversation handler.
func (r *Router) SetPublicConversationHandler(handler *servicehandlers.PublicConversationHandler) {
	r.publicConversation = handler
}

// RegisterRoutes registers all v1 API routes
func (r *Router) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Postmark webhook endpoints (no auth - validated via secret in URL)
	if r.postmarkWebhookHandlers != nil {
		webhooks := api.Group("/webhooks/postmark/:secret")
		{
			webhooks.POST("/inbound", r.postmarkWebhookHandlers.HandleInboundEmail)
			webhooks.POST("/bounce", r.postmarkWebhookHandlers.HandleBounce)
			webhooks.POST("/delivery", r.postmarkWebhookHandlers.HandleDelivery)
		}
	}

	// Public form endpoints (no auth required - forms accessed via crypto ID or API token)
	if r.publicFormHandler != nil {
		forms := api.Group("/forms")
		{
			// Public form rendering and submission (by crypto ID)
			forms.GET("/:crypto_id", r.publicFormHandler.RenderPublicForm)
			forms.POST("/:crypto_id/submit", r.publicFormHandler.SubmitPublicForm)
			forms.GET("/:crypto_id/embed.js", r.publicFormHandler.GetEmbedScript)

			// API token authenticated submission (for programmatic access)
			apiSubmit := forms.Group("")
			apiSubmit.Use(r.publicFormHandler.FormAPITokenMiddleware())
			{
				apiSubmit.POST("/:crypto_id/api/submit", r.publicFormHandler.SubmitPublicForm)
			}
		}
	}

	if r.publicConversation != nil {
		conversations := api.Group("/conversations")
		{
			conversations.POST("", r.publicConversation.StartConversation)
			conversations.POST("/:session_id/messages", r.publicConversation.AddMessage)
		}
	}
}
