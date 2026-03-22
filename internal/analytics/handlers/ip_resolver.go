package analyticshandlers

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// ResolveClientIP resolves the real client IP from proxy headers.
// Priority: CF-Connecting-IP > X-Forwarded-For (first) > X-Real-IP > RemoteAddr
func ResolveClientIP(c *gin.Context) string {
	// Cloudflare
	if ip := c.GetHeader("CF-Connecting-IP"); ip != "" {
		return cleanIP(ip)
	}

	// X-Forwarded-For: first/leftmost IP only
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		if ip := cleanIP(parts[0]); ip != "" {
			return ip
		}
	}

	// Nginx
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return cleanIP(ip)
	}

	// Direct connection fallback
	return cleanIP(c.ClientIP())
}

// cleanIP strips port numbers and IPv6 bracket notation.
func cleanIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Try SplitHostPort first (handles both "1.2.3.4:8080" and "[::1]:8080")
	host, _, err := net.SplitHostPort(raw)
	if err == nil {
		return host
	}

	// Strip IPv6 brackets from bare addresses like "[::1]"
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")

	return raw
}
