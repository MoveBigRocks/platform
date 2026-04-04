package platformhandlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/platform/internal/extensionhost/hostapi"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
)

type ExtensionHostAPIHandler struct {
	cfg      *config.Config
	identity *platformservices.ExtensionIdentityHostService
}

func NewExtensionHostAPIHandler(
	cfg *config.Config,
	identity *platformservices.ExtensionIdentityHostService,
) *ExtensionHostAPIHandler {
	return &ExtensionHostAPIHandler{
		cfg:      cfg,
		identity: identity,
	}
}

func (h *ExtensionHostAPIHandler) RequireHostToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c.GetHeader("Authorization"))
		if token == "" {
			c.JSON(http.StatusUnauthorized, runtimehost.ErrorResponse{
				Status:  "failed",
				Message: "host bearer token is required",
			})
			c.Abort()
			return
		}
		claims, err := hostapi.VerifyToken(h.cfg.Auth.JWTSecret, token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, runtimehost.ErrorResponse{
				Status:  "failed",
				Message: err.Error(),
			})
			c.Abort()
			return
		}
		c.Set("extension_host_claims", claims)
		c.Next()
	}
}

func (h *ExtensionHostAPIHandler) IssueIdentitySession(c *gin.Context) {
	if h == nil || h.identity == nil {
		c.JSON(http.StatusServiceUnavailable, runtimehost.ErrorResponse{
			Status:  "failed",
			Message: "identity host services are not configured",
		})
		return
	}
	rawClaims, ok := c.Get("extension_host_claims")
	if !ok {
		c.JSON(http.StatusUnauthorized, runtimehost.ErrorResponse{
			Status:  "failed",
			Message: "host token is required",
		})
		return
	}
	claims, ok := rawClaims.(*hostapi.TokenClaims)
	if !ok || claims == nil {
		c.JSON(http.StatusUnauthorized, runtimehost.ErrorResponse{
			Status:  "failed",
			Message: "host token is invalid",
		})
		return
	}

	var input runtimehost.IdentitySessionRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, runtimehost.ErrorResponse{
			Status:  "failed",
			Message: "invalid identity session payload",
		})
		return
	}
	response, err := h.identity.IssueIdentitySession(c.Request.Context(), claims.ExtensionID, input)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, platformservices.ErrExtensionHostForbidden):
			status = http.StatusForbidden
		case errors.Is(err, platformservices.ErrIdentityUserNotFound):
			status = http.StatusUnauthorized
		case errors.Is(err, platformservices.ErrIdentityRoleRequired):
			status = http.StatusBadRequest
		case strings.Contains(err.Error(), "required"):
			status = http.StatusBadRequest
		}
		c.JSON(status, runtimehost.ErrorResponse{
			Status:  "failed",
			Message: err.Error(),
		})
		return
	}
	response.CookieName = "mbr_session"
	response.CookieDomain = strings.TrimSpace(h.cfg.Auth.CookieDomain)
	response.CookiePath = "/"
	response.CookieSecure = h.cfg.Server.Environment != "development"
	c.JSON(http.StatusOK, response)
}

func extractBearerToken(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return ""
	}
	return strings.TrimSpace(value[len("Bearer "):])
}
