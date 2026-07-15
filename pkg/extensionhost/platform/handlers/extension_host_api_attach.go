package platformhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/movebigrocks/extension-sdk/runtimehost"
)

// UploadAttachment serves POST /attachments.
func (h *ExtensionHostAPIHandler) UploadAttachment(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input runtimehost.UploadAttachmentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid attachment payload"})
		return
	}
	out, err := h.core.UploadAttachment(c.Request.Context(), claims.ExtensionID, input)
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// GetAttachment serves GET /attachments/:attachmentID.
func (h *ExtensionHostAPIHandler) GetAttachment(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	out, err := h.core.GetAttachment(c.Request.Context(), claims.ExtensionID, c.Param("attachmentID"))
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// LinkAttachmentsToCase serves POST /cases/:caseID/attachments.
func (h *ExtensionHostAPIHandler) LinkAttachmentsToCase(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var body struct {
		AttachmentIDs []string `json:"attachmentIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid link payload"})
		return
	}
	if err := h.core.LinkAttachmentsToCase(c.Request.Context(), claims.ExtensionID, c.Param("caseID"), body.AttachmentIDs); err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// EvaluateRulesForCase serves POST /rules/evaluate.
func (h *ExtensionHostAPIHandler) EvaluateRulesForCase(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input runtimehost.EvaluateRulesInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid rule evaluation payload"})
		return
	}
	if err := h.core.EvaluateRulesForCase(c.Request.Context(), claims.ExtensionID, input); err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// PublishArtifact serves POST /artifacts.
func (h *ExtensionHostAPIHandler) PublishArtifact(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input runtimehost.PublishArtifactInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid artifact payload"})
		return
	}
	if err := h.core.PublishArtifact(c.Request.Context(), claims.ExtensionID, input); err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
