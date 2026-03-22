package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/pkg/id"
)

// Default limits for request body size
const (
	// DefaultMaxBodySize is 1MB - suitable for most JSON API requests
	DefaultMaxBodySize = 1 * 1024 * 1024
	// LargeMaxBodySize is 25MB - suitable for authenticated attachment uploads.
	LargeMaxBodySize = 25 * 1024 * 1024
)

// MaxBodySize middleware limits the size of request bodies to prevent DoS attacks.
// This should be one of the first middleware in the chain.
// SECURITY: Prevents memory exhaustion from oversized requests.
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// RequestID middleware adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request already has an ID
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = id.New()
		}
		c.Set("request_id", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

// SecurityHeaders middleware adds security headers to responses
// SECURITY: Implements defense-in-depth with strict CSP and other protective headers
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this is a Grafana proxy request - these need to be embeddable in iframes
		isGrafanaProxy := strings.HasPrefix(c.Request.URL.Path, "/grafana/")

		// Prevent MIME type sniffing
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		// Prevent clickjacking (skip for Grafana proxy - needs to be embeddable)
		if !isGrafanaProxy {
			c.Writer.Header().Set("X-Frame-Options", "DENY")
		}
		// Legacy XSS protection (modern browsers use CSP)
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		// Enforce HTTPS for 1 year, including subdomains
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		// Control referrer information
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Prevent browser features that could leak data
		c.Writer.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=()")
		// Cross-Origin policies for additional isolation
		c.Writer.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		c.Writer.Header().Set("Cross-Origin-Resource-Policy", "same-origin")

		// Content Security Policy
		// Allows trusted CDNs used by admin panel (DaisyUI, Tailwind, HTMX, Iconify)
		// Note: 'unsafe-inline' for scripts is required for inline <script> tags in templates.
		// Trade-off: Using nonces would require template modifications and middleware changes.
		// Current approach is acceptable given trusted CDN sources and other security headers.
		if !isGrafanaProxy {
			// Standard CSP with frame-ancestors 'none' to prevent clickjacking
			csp := strings.Join([]string{
				"default-src 'self'",
				"script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://unpkg.com https://code.iconify.design",
				"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net",
				"img-src 'self' data: https:",
				"font-src 'self' https://fonts.gstatic.com",
				"connect-src 'self'",
				"frame-src 'self'",
				"frame-ancestors 'none'",
				"form-action 'self'",
				"base-uri 'self'",
				"object-src 'none'",
			}, "; ")
			c.Writer.Header().Set("Content-Security-Policy", csp)
		}
		// For Grafana proxy: skip CSP entirely to allow iframe embedding

		c.Next()
	}
}

// Recovery middleware recovers from panics and returns 500
func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}
