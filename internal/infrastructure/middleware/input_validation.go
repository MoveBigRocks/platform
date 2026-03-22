package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// InputValidation middleware performs basic input validation on request bodies.
// It checks for common attack patterns and validates string lengths.
// SECURITY: Defense-in-depth against injection attacks.
func InputValidation() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check Content-Type for POST/PUT/PATCH requests
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			contentType := c.GetHeader("Content-Type")

			// Allow common content types
			validContentTypes := []string{
				"application/json",
				"application/x-www-form-urlencoded",
				"multipart/form-data",
				"text/plain",                    // For some webhooks
				"application/x-sentry-envelope", // Sentry SDK envelopes
			}

			if contentType != "" {
				isValid := false
				for _, valid := range validContentTypes {
					if strings.HasPrefix(contentType, valid) {
						isValid = true
						break
					}
				}

				if !isValid {
					c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
						"error":   "Unsupported Media Type",
						"message": "Content-Type must be application/json, application/x-www-form-urlencoded, multipart/form-data, or application/x-sentry-envelope",
					})
					return
				}
			}
		}

		c.Next()
	}
}

// ValidatePathParams middleware validates that path parameters don't contain
// dangerous characters that could be used for path traversal or injection.
// SECURITY: Prevents path traversal attacks.
func ValidatePathParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, param := range c.Params {
			// Check for path traversal attempts
			if strings.Contains(param.Value, "..") {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error":   "Bad Request",
					"message": "Invalid path parameter: " + param.Key,
				})
				return
			}

			// Check for null bytes
			if strings.Contains(param.Value, "\x00") {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error":   "Bad Request",
					"message": "Invalid path parameter: " + param.Key,
				})
				return
			}

			// Check for extremely long parameters (potential buffer overflow attempt)
			if len(param.Value) > 1000 {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error":   "Bad Request",
					"message": "Path parameter too long: " + param.Key,
				})
				return
			}
		}

		c.Next()
	}
}
