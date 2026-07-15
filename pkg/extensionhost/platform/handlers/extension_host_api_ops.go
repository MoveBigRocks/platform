package platformhandlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/platform/internal/extensionhost/hostapi"
)

// coreClaims resolves the verified host claims and confirms the core host
// service is configured. It writes the appropriate error response and returns
// false when the caller should stop. Every core-data ops handler funnels
// through it so the guard is written once.
func (h *ExtensionHostAPIHandler) coreClaims(c *gin.Context) (*hostapi.TokenClaims, bool) {
	if h == nil || h.core == nil {
		c.JSON(http.StatusServiceUnavailable, runtimehost.ErrorResponse{Status: "failed", Message: "core host services are not configured"})
		return nil, false
	}
	claims, ok := hostClaims(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, runtimehost.ErrorResponse{Status: "failed", Message: "host token is required"})
		return nil, false
	}
	return claims, true
}

// GetQueueOrBySlug serves GET /queues/:queueID and GET /queues?slug=.
func (h *ExtensionHostAPIHandler) GetQueue(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	out, err := h.core.GetQueue(c.Request.Context(), claims.ExtensionID, c.Param("queueID"))
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// GetQueueBySlug serves GET /queues?slug=.
func (h *ExtensionHostAPIHandler) GetQueueBySlug(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	slug := strings.TrimSpace(c.Query("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "slug query parameter is required"})
		return
	}
	out, err := h.core.GetQueueBySlug(c.Request.Context(), claims.ExtensionID, slug)
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// CreateQueue serves POST /queues.
func (h *ExtensionHostAPIHandler) CreateQueue(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input runtimehost.CreateQueueInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid queue payload"})
		return
	}
	out, err := h.core.CreateQueue(c.Request.Context(), claims.ExtensionID, input)
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// CreateContact serves POST /contacts.
func (h *ExtensionHostAPIHandler) CreateContact(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input runtimehost.CreateContactInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid contact payload"})
		return
	}
	out, err := h.core.CreateContact(c.Request.Context(), claims.ExtensionID, input)
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// UpdateCase serves PATCH /cases/:caseID.
func (h *ExtensionHostAPIHandler) UpdateCase(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var patch runtimehost.CaseUpdateInput
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid case patch"})
		return
	}
	out, err := h.core.UpdateCase(c.Request.Context(), claims.ExtensionID, c.Param("caseID"), patch)
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandoffCase serves POST /cases/:caseID/handoff.
func (h *ExtensionHostAPIHandler) HandoffCase(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input runtimehost.HandoffCaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid handoff payload"})
		return
	}
	if err := h.core.HandoffCase(c.Request.Context(), claims.ExtensionID, c.Param("caseID"), input); err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// MarkCaseResolved serves POST /cases/:caseID/resolve.
func (h *ExtensionHostAPIHandler) MarkCaseResolved(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var body struct {
		WorkspaceID string    `json:"workspaceId"`
		ResolvedAt  time.Time `json:"resolvedAt"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid resolve payload"})
		return
	}
	if body.ResolvedAt.IsZero() {
		body.ResolvedAt = time.Now().UTC()
	}
	if err := h.core.MarkCaseResolved(c.Request.Context(), claims.ExtensionID, body.WorkspaceID, c.Param("caseID"), body.ResolvedAt); err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *ExtensionHostAPIHandler) LinkIssueToCase(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input runtimehost.LinkIssueToCaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid issue link payload"})
		return
	}
	if err := h.core.LinkIssueToCase(c.Request.Context(), claims.ExtensionID, c.Param("caseID"), input); err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *ExtensionHostAPIHandler) UnlinkIssueFromCase(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input runtimehost.UnlinkIssueFromCaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid issue unlink payload"})
		return
	}
	if err := h.core.UnlinkIssueFromCase(c.Request.Context(), claims.ExtensionID, c.Param("caseID"), input); err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *ExtensionHostAPIHandler) GetCaseByIssueAndContact(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	out, err := h.core.GetCaseByIssueAndContact(c.Request.Context(), claims.ExtensionID, c.Query("workspaceId"), c.Query("issueId"), c.Query("contactId"))
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *ExtensionHostAPIHandler) ListWorkspaces(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	workspaces, err := h.core.ListWorkspaces(c.Request.Context(), claims.ExtensionID)
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

func (h *ExtensionHostAPIHandler) GetWorkspacesByIDs(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid workspace ids payload"})
		return
	}
	workspaces, err := h.core.GetWorkspacesByIDs(c.Request.Context(), claims.ExtensionID, input.IDs)
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

func (h *ExtensionHostAPIHandler) PublishEvent(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input runtimehost.PublishEventInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid event payload"})
		return
	}
	if err := h.core.PublishEvent(c.Request.Context(), claims.ExtensionID, input); err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
