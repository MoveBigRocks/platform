package analyticshandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/web"
)

// AnalyticsScriptHandler serves the tracking script at GET /js/analytics.js.
type AnalyticsScriptHandler struct {
	scriptContent []byte
}

// NewAnalyticsScriptHandler creates a new script handler.
func NewAnalyticsScriptHandler() *AnalyticsScriptHandler {
	content, _ := web.Static.ReadFile("static/js/analytics.js")
	return &AnalyticsScriptHandler{scriptContent: content}
}

// ServeScript handles GET /js/analytics.js with aggressive caching.
func (h *AnalyticsScriptHandler) ServeScript(c *gin.Context) {
	if len(h.scriptContent) == 0 {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", "application/javascript; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", h.scriptContent)
}
