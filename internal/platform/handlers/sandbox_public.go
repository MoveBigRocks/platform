package platformhandlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
)

type SandboxPublicHandler struct {
	service *platformservices.SandboxService
}

func NewSandboxPublicHandler(service *platformservices.SandboxService) *SandboxPublicHandler {
	return &SandboxPublicHandler{service: service}
}

type sandboxCreateRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type sandboxDestroyRequest struct {
	Reason string `json:"reason"`
}

func (h *SandboxPublicHandler) CreateSandbox(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sandbox control plane is not configured"})
		return
	}
	var req sandboxCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sandbox request"})
		return
	}
	result, err := h.service.CreateSandbox(c.Request.Context(), platformservices.SandboxCreateParams{
		Email: req.Email,
		Name:  req.Name,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sandboxResponse(result.Sandbox, gin.H{
		"manage_token": result.ManageToken,
		"next_steps":   sandboxCreateNextSteps(result.Sandbox),
	}))
}

func (h *SandboxPublicHandler) GetSandbox(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sandbox control plane is not configured"})
		return
	}
	sandbox, err := h.service.GetSandbox(c.Request.Context(), c.Param("id"), sandboxManageToken(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sandboxResponse(sandbox, nil))
}

func (h *SandboxPublicHandler) ExtendSandbox(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sandbox control plane is not configured"})
		return
	}
	sandbox, err := h.service.ExtendSandbox(c.Request.Context(), c.Param("id"), sandboxManageToken(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sandboxResponse(sandbox, gin.H{
		"next_steps": []string{
			"Continue using the sandbox with the same manage token.",
		},
	}))
}

func (h *SandboxPublicHandler) ExportSandbox(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sandbox control plane is not configured"})
		return
	}
	exported, err := h.service.ExportSandbox(c.Request.Context(), c.Param("id"), sandboxManageToken(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, exported)
}

func (h *SandboxPublicHandler) DestroySandbox(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sandbox control plane is not configured"})
		return
	}
	var req sandboxDestroyRequest
	_ = c.ShouldBindJSON(&req)
	sandbox, err := h.service.DestroySandbox(c.Request.Context(), c.Param("id"), sandboxManageToken(c), req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sandboxResponse(sandbox, nil))
}

func (h *SandboxPublicHandler) VerifySandbox(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "sandbox control plane is not configured"})
		return
	}
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "verification token is required"})
		return
	}
	sandbox, err := h.service.VerifySandbox(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if wantsJSON(c) || strings.TrimSpace(sandbox.LoginURL) == "" {
		c.JSON(http.StatusOK, sandboxResponse(sandbox, nil))
		return
	}
	c.Redirect(http.StatusFound, sandbox.LoginURL)
}

func sandboxManageToken(c *gin.Context) string {
	if header := strings.TrimSpace(c.GetHeader("X-MBR-Sandbox-Token")); header != "" {
		return header
	}
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return strings.TrimSpace(c.Query("manage_token"))
}

func sandboxResponse(sandbox *platformdomain.Sandbox, extra gin.H) gin.H {
	payload := gin.H{
		"id":                     sandbox.ID,
		"slug":                   sandbox.Slug,
		"name":                   sandbox.Name,
		"requested_email":        sandbox.RequestedEmail,
		"status":                 sandbox.Status,
		"runtime_url":            sandbox.RuntimeURL,
		"login_url":              sandbox.LoginURL,
		"bootstrap_url":          sandbox.BootstrapURL,
		"activation_deadline_at": sandbox.ActivationDeadlineAt,
		"verified_at":            sandbox.VerifiedAt,
		"expires_at":             sandbox.ExpiresAt,
		"extended_at":            sandbox.ExtendedAt,
		"destroyed_at":           sandbox.DestroyedAt,
		"last_error":             sandbox.LastError,
		"created_at":             sandbox.CreatedAt,
		"updated_at":             sandbox.UpdatedAt,
	}
	for k, v := range extra {
		payload[k] = v
	}
	return payload
}

func wantsJSON(c *gin.Context) bool {
	return strings.Contains(strings.ToLower(c.GetHeader("Accept")), "application/json")
}

func sandboxCreateNextSteps(sandbox *platformdomain.Sandbox) []string {
	if sandbox == nil {
		return nil
	}
	if strings.TrimSpace(sandbox.RuntimeURL) == "" {
		return []string{
			"Review `last_error` and retry once the sandbox control plane is healthy.",
		}
	}
	return []string{
		"mbr auth login --url " + sandbox.RuntimeURL,
		"mbr context set --workspace sandbox --team operations",
	}
}
