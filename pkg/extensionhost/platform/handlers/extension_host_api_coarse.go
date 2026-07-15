package platformhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/movebigrocks/extension-sdk/runtimehost"
)

// IngestApplication serves POST /ingest/application: the coarse operation that
// creates a core contact and case for an application and links its attachments
// in one transaction, idempotent on the caller's key.
func (h *ExtensionHostAPIHandler) IngestApplication(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input runtimehost.IngestApplicationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid application ingest payload"})
		return
	}
	out, err := h.core.IngestApplication(c.Request.Context(), claims.ExtensionID, input)
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ApplyCaseChange serves POST /cases/:caseID/apply-change: the coarse operation
// that patches a case and fires automation rules in one transaction, idempotent
// on the caller's key.
func (h *ExtensionHostAPIHandler) ApplyCaseChange(c *gin.Context) {
	claims, ok := h.coreClaims(c)
	if !ok {
		return
	}
	var input runtimehost.ApplyCaseChangeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{Status: "failed", Message: "invalid case change payload"})
		return
	}
	out, err := h.core.ApplyCaseChange(c.Request.Context(), claims.ExtensionID, c.Param("caseID"), input)
	if err != nil {
		respondHostError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
