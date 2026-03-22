package middleware

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/metrics"
)

// PrometheusMetrics returns a middleware that automatically tracks HTTP requests
// with Prometheus metrics including duration, status code, method, and endpoint
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip metrics for the metrics endpoint itself to avoid recursive tracking
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()

		// Process request
		c.Next()

		// Record metrics after request is processed
		duration := time.Since(start).Seconds()
		status := c.Writer.Status()
		method := c.Request.Method
		endpoint := c.FullPath()

		// Use route pattern if available, otherwise use raw path
		if endpoint == "" {
			endpoint = c.Request.URL.Path
		}

		// Record HTTP request metrics
		metrics.RecordHTTPRequest(method, endpoint, fmt.Sprintf("%d", status), duration)
	}
}

// MetricsAuth returns a middleware that protects the /metrics endpoint.
// If metricsToken is set, it requires Bearer token authentication.
// If metricsToken is empty, it allows requests from localhost or private/internal networks.
func MetricsAuth(metricsToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if metricsToken != "" {
			// Token authentication mode
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
				return
			}

			// Expect "Bearer <token>" format
			if !strings.HasPrefix(authHeader, "Bearer ") {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(token), []byte(metricsToken)) != 1 {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid token"})
				return
			}
		} else {
			// Internal network mode - allow localhost and private/internal networks
			clientIP := c.ClientIP()
			if !isInternalIP(clientIP) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "metrics only available from internal network"})
				return
			}
		}

		c.Next()
	}
}

// isInternalIP checks if the IP address is localhost or a private/internal network address.
// This allows Prometheus and other monitoring tools on trusted internal networks to scrape metrics.
func isInternalIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Allow loopback addresses (127.0.0.0/8 for IPv4, ::1 for IPv6)
	if parsedIP.IsLoopback() {
		return true
	}

	// Allow private network addresses (RFC 1918)
	// 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
	if parsedIP.IsPrivate() {
		return true
	}

	// Also allow link-local addresses used in some internal network configurations
	if parsedIP.IsLinkLocalUnicast() {
		return true
	}

	return false
}
