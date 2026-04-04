package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	extensionruntime "github.com/movebigrocks/platform/internal/extensionhost/runtime"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/middleware"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
)

const (
	extensionWebhookSignatureHeader = "X-MBR-Webhook-Signature"
	extensionWebhookTimestampHeader = "X-MBR-Webhook-Timestamp"
	extensionWebhookMaxSkew         = 5 * time.Minute
)

type extensionEndpointRateCounter struct {
	count     int
	resetTime time.Time
}

type extensionEndpointRateLimiter struct {
	mu       sync.Mutex
	counters map[string]extensionEndpointRateCounter
}

type extensionWebhookReplayProtector struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

var globalExtensionEndpointRateLimiter = &extensionEndpointRateLimiter{
	counters: map[string]extensionEndpointRateCounter{},
}

var globalExtensionWebhookReplayProtector = &extensionWebhookReplayProtector{
	entries: map[string]time.Time{},
}

func serveResolvedExtensionServiceRoute(
	ctx *gin.Context,
	extensionService *platformservices.ExtensionService,
	registry *extensionruntime.Registry,
	cfg *config.Config,
	principalAuth *middleware.PrincipalAuthMiddleware,
) {
	if extensionService == nil || registry == nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	resolved, err := extensionService.ResolvePublicServiceRoute(ctx.Request.Context(), ctx.Request.Method, ctx.Request.URL.Path)
	if err != nil {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if resolved == nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	applyResolvedExtensionWorkspaceContext(ctx, resolved.Extension)
	applyExtensionRouteParams(ctx, resolved.RouteParams)
	if !enforceResolvedExtensionServiceRoutePolicy(ctx, resolved, cfg, principalAuth) {
		return
	}
	if err := registry.DispatchEndpoint(resolved.Extension, resolved.Endpoint, ctx); err != nil {
		ctx.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
}

func serveResolvedAdminExtensionServiceRoute(
	ctx *gin.Context,
	extensionService *platformservices.ExtensionService,
	registry *extensionruntime.Registry,
	workspaceID string,
	cfg *config.Config,
) bool {
	if extensionService == nil || registry == nil || ctx == nil {
		return false
	}
	workspaceID = strings.TrimSpace(workspaceID)

	resolved, err := extensionService.ResolveAdminServiceRoute(ctx.Request.Context(), workspaceID, ctx.Request.Method, ctx.Request.URL.Path)
	if err != nil {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return true
	}
	if resolved == nil {
		return false
	}
	applyResolvedExtensionWorkspaceContext(ctx, resolved.Extension)
	applyExtensionRouteParams(ctx, resolved.RouteParams)
	if !enforceResolvedExtensionServiceRoutePolicy(ctx, resolved, cfg, nil) {
		return true
	}
	if err := registry.DispatchEndpoint(resolved.Extension, resolved.Endpoint, ctx); err != nil {
		ctx.AbortWithStatus(http.StatusServiceUnavailable)
		return true
	}
	return true
}

func applyExtensionRouteParams(ctx *gin.Context, params map[string]string) {
	extensionruntime.ApplyRouteParams(ctx, params)
}

func enforceResolvedExtensionServiceRoutePolicy(
	ctx *gin.Context,
	resolved *platformservices.ResolvedExtensionServiceRoute,
	cfg *config.Config,
	principalAuth *middleware.PrincipalAuthMiddleware,
) bool {
	if ctx == nil || resolved == nil {
		return false
	}
	if !enforceExtensionEndpointBodyLimit(ctx, resolved.Endpoint) {
		return false
	}
	if !enforceExtensionEndpointContentTypes(ctx, resolved.Endpoint) {
		return false
	}
	if !enforceExtensionEndpointAuth(ctx, resolved.Endpoint, cfg, principalAuth) {
		return false
	}
	if !enforceExtensionEndpointWorkspaceBinding(ctx, resolved.Endpoint) {
		return false
	}
	if !enforceExtensionEndpointRateLimit(ctx, resolved) {
		return false
	}
	return true
}

func enforceExtensionEndpointBodyLimit(ctx *gin.Context, endpoint platformdomain.ExtensionEndpoint) bool {
	if endpoint.MaxBodyBytes <= 0 || ctx == nil || ctx.Request == nil {
		return true
	}
	if ctx.Request.ContentLength > endpoint.MaxBodyBytes {
		ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
			"error":   "Payload Too Large",
			"message": "Request body exceeds the endpoint limit",
		})
		return false
	}
	if ctx.Request.Body != nil {
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, endpoint.MaxBodyBytes)
	}
	return true
}

func enforceExtensionEndpointContentTypes(ctx *gin.Context, endpoint platformdomain.ExtensionEndpoint) bool {
	if ctx == nil || len(endpoint.ContentTypes) == 0 {
		return true
	}
	contentType := strings.ToLower(strings.TrimSpace(ctx.GetHeader("Content-Type")))
	if contentType == "" {
		if ctx.Request != nil && ctx.Request.ContentLength > 0 {
			ctx.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
				"error":   "Unsupported Media Type",
				"message": "Content-Type header is required for this endpoint",
			})
			return false
		}
		return true
	}
	for _, allowed := range endpoint.ContentTypes {
		if strings.HasPrefix(contentType, strings.ToLower(strings.TrimSpace(allowed))) {
			return true
		}
	}
	ctx.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
		"error":   "Unsupported Media Type",
		"message": "Content-Type is not allowed for this endpoint",
	})
	return false
}

func enforceExtensionEndpointAuth(
	ctx *gin.Context,
	endpoint platformdomain.ExtensionEndpoint,
	cfg *config.Config,
	principalAuth *middleware.PrincipalAuthMiddleware,
) bool {
	switch endpoint.Auth {
	case platformdomain.ExtensionEndpointAuthPublic:
		return true
	case platformdomain.ExtensionEndpointAuthSession:
		authCtx := middleware.GetAuthContext(ctx)
		if authCtx == nil || authCtx.AuthMethod != platformdomain.AuthMethodSession || !authCtx.IsHuman() {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Session authentication is required for this endpoint",
			})
			return false
		}
		return true
	case platformdomain.ExtensionEndpointAuthAgentToken:
		authCtx := middleware.GetAuthContext(ctx)
		if authCtx == nil || authCtx.AuthMethod != platformdomain.AuthMethodAgentToken {
			if principalAuth == nil {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   "Unauthorized",
					"message": "Agent token authentication is required for this endpoint",
				})
				return false
			}
			principalAuth.AuthenticateAgent()(ctx)
			if ctx.IsAborted() {
				return false
			}
			authCtx = middleware.GetAuthContext(ctx)
		}
		return authCtx != nil && authCtx.AuthMethod == platformdomain.AuthMethodAgentToken
	case platformdomain.ExtensionEndpointAuthSignedWebhook:
		return validateSignedExtensionWebhookRequest(ctx, cfg)
	case platformdomain.ExtensionEndpointAuthExtensionToken:
		ctx.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{
			"error":   "Not Implemented",
			"message": "extension_token auth is not implemented yet",
		})
		return false
	case platformdomain.ExtensionEndpointAuthInternalOnly:
		if internal, ok := ctx.Get("internal_extension_request"); ok {
			if allowed, ok := internal.(bool); ok && allowed {
				return true
			}
		}
		ctx.AbortWithStatus(http.StatusNotFound)
		return false
	default:
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "Unsupported endpoint auth mode",
		})
		return false
	}
}

func enforceExtensionEndpointWorkspaceBinding(ctx *gin.Context, endpoint platformdomain.ExtensionEndpoint) bool {
	switch endpoint.WorkspaceBinding {
	case platformdomain.ExtensionWorkspaceBindingNone:
		return true
	case platformdomain.ExtensionWorkspaceBindingInstanceScoped:
		if !isAdminScopedEndpoint(endpoint.Class) {
			return true
		}
		authCtx := middleware.GetAuthContext(ctx)
		if authCtx == nil || !authCtx.IsInstanceAdmin() {
			ctx.AbortWithStatus(http.StatusNotFound)
			return false
		}
		return true
	case platformdomain.ExtensionWorkspaceBindingFromSession:
		authCtx := middleware.GetAuthContext(ctx)
		if authCtx == nil || authCtx.AuthMethod != platformdomain.AuthMethodSession || !authCtx.IsHuman() {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Workspace session context is required for this endpoint",
			})
			return false
		}
		workspaceID := strings.TrimSpace(ctx.GetString("workspace_id"))
		if workspaceID == "" {
			workspaceID = strings.TrimSpace(authCtx.WorkspaceID)
		}
		if workspaceID == "" || !authCtx.HasWorkspaceAccess(workspaceID) {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "An active workspace context is required for this endpoint",
			})
			return false
		}
		ctx.Set("workspace_id", workspaceID)
		return true
	case platformdomain.ExtensionWorkspaceBindingFromAgentToken:
		authCtx := middleware.GetAuthContext(ctx)
		if authCtx == nil || authCtx.AuthMethod != platformdomain.AuthMethodAgentToken {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Agent workspace context is required for this endpoint",
			})
			return false
		}
		workspaceID := strings.TrimSpace(authCtx.WorkspaceID)
		if workspaceID == "" {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "The authenticated agent is missing a workspace binding",
			})
			return false
		}
		ctx.Set("workspace_id", workspaceID)
		return true
	case platformdomain.ExtensionWorkspaceBindingFromRoute:
		workspaceID := firstNonEmpty(
			strings.TrimSpace(ctx.Param("workspace_id")),
			strings.TrimSpace(ctx.Param("workspaceID")),
			strings.TrimSpace(ctx.Param("workspaceId")),
		)
		if workspaceID == "" {
			workspaceID = strings.TrimSpace(ctx.GetString("workspace_id"))
		}
		if workspaceID == "" {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "A workspace route parameter is required for this endpoint",
			})
			return false
		}
		if authCtx := middleware.GetAuthContext(ctx); authCtx != nil && !authCtx.IsInstanceAdmin() && !authCtx.HasWorkspaceAccess(workspaceID) {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "Workspace access denied",
			})
			return false
		}
		ctx.Set("workspace_id", workspaceID)
		return true
	default:
		return true
	}
}

func enforceExtensionEndpointRateLimit(ctx *gin.Context, resolved *platformservices.ResolvedExtensionServiceRoute) bool {
	if ctx == nil || resolved == nil || resolved.Endpoint.RateLimitPerMin <= 0 {
		return true
	}
	key := extensionEndpointRateLimitKey(ctx, resolved)
	allowed, retryAfter := globalExtensionEndpointRateLimiter.allow(key, resolved.Endpoint.RateLimitPerMin, time.Now().UTC())
	if allowed {
		return true
	}
	ctx.Header("Retry-After", strconv.Itoa(maxInt(1, int(retryAfter.Seconds()))))
	ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error":   "Too Many Requests",
		"message": "Endpoint rate limit exceeded",
	})
	return false
}

func extensionEndpointRateLimitKey(ctx *gin.Context, resolved *platformservices.ResolvedExtensionServiceRoute) string {
	identity := "ip:" + strings.TrimSpace(ctx.ClientIP())
	if authCtx := middleware.GetAuthContext(ctx); authCtx != nil && authCtx.Principal != nil {
		identity = string(authCtx.PrincipalType) + ":" + strings.TrimSpace(authCtx.Principal.GetID())
	}
	return resolved.Extension.ID + ":" + resolved.Endpoint.Name + ":" + identity
}

func (rl *extensionEndpointRateLimiter) allow(key string, limit int, now time.Time) (bool, time.Duration) {
	if rl == nil || limit <= 0 {
		return true, 0
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	for existingKey, counter := range rl.counters {
		if now.After(counter.resetTime) {
			delete(rl.counters, existingKey)
		}
	}

	counter, ok := rl.counters[key]
	if !ok || now.After(counter.resetTime) {
		rl.counters[key] = extensionEndpointRateCounter{
			count:     1,
			resetTime: now.Add(time.Minute),
		}
		return true, 0
	}
	if counter.count >= limit {
		return false, counter.resetTime.Sub(now)
	}
	counter.count++
	rl.counters[key] = counter
	return true, 0
}

func validateSignedExtensionWebhookRequest(ctx *gin.Context, cfg *config.Config) bool {
	if ctx == nil || ctx.Request == nil || cfg == nil || strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Service Unavailable",
			"message": "Signed webhook verification is not configured",
		})
		return false
	}

	timestampRaw := strings.TrimSpace(ctx.GetHeader(extensionWebhookTimestampHeader))
	signatureRaw := strings.TrimSpace(ctx.GetHeader(extensionWebhookSignatureHeader))
	if timestampRaw == "" || signatureRaw == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "Signed webhook headers are required",
		})
		return false
	}

	timestampUnix, err := strconv.ParseInt(timestampRaw, 10, 64)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "Webhook timestamp is invalid",
		})
		return false
	}
	now := time.Now().UTC()
	timestamp := time.Unix(timestampUnix, 0)
	if timestamp.Before(now.Add(-extensionWebhookMaxSkew)) || timestamp.After(now.Add(extensionWebhookMaxSkew)) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "Webhook timestamp is outside the allowed window",
		})
		return false
	}

	body, err := readRequestBody(ctx)
	if err != nil {
		statusCode := http.StatusBadRequest
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			statusCode = http.StatusRequestEntityTooLarge
		}
		ctx.AbortWithStatusJSON(statusCode, gin.H{
			"error":   http.StatusText(statusCode),
			"message": "Failed to read webhook body",
		})
		return false
	}

	signature := normalizeExtensionWebhookSignature(signatureRaw)
	expected := computeSignedWebhookSignature(cfg.Auth.JWTSecret, timestampRaw, body)
	if signature == "" || !hmac.Equal([]byte(signature), []byte(expected)) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "Webhook signature verification failed",
		})
		return false
	}
	if globalExtensionWebhookReplayProtector.seen(signature+":"+timestampRaw, now.Add(extensionWebhookMaxSkew)) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "Webhook replay detected",
		})
		return false
	}
	return true
}

func readRequestBody(ctx *gin.Context) ([]byte, error) {
	if ctx == nil || ctx.Request == nil || ctx.Request.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return nil, err
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func normalizeExtensionWebhookSignature(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "sha256=")
	return value
}

func computeSignedWebhookSignature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	mac.Write([]byte(strings.TrimSpace(timestamp)))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func (rp *extensionWebhookReplayProtector) seen(key string, expiresAt time.Time) bool {
	if rp == nil || key == "" {
		return false
	}

	now := time.Now().UTC()
	rp.mu.Lock()
	defer rp.mu.Unlock()

	for existingKey, expiry := range rp.entries {
		if now.After(expiry) {
			delete(rp.entries, existingKey)
		}
	}
	if expiry, ok := rp.entries[key]; ok && now.Before(expiry) {
		return true
	}
	rp.entries[key] = expiresAt
	return false
}

func isAdminScopedEndpoint(class platformdomain.ExtensionEndpointClass) bool {
	switch class {
	case platformdomain.ExtensionEndpointClassAdminPage,
		platformdomain.ExtensionEndpointClassAdminAction,
		platformdomain.ExtensionEndpointClassExtensionAPI,
		platformdomain.ExtensionEndpointClassHealth:
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
