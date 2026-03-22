package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ValidateUUIDParam validates that a path parameter is a valid UUID.
// Returns empty string if validation fails (also sets error response and aborts).
func ValidateUUIDParam(c *gin.Context, paramName string) string {
	value := c.Param(paramName)
	if value == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": paramName + " is required",
		})
		c.Abort()
		return ""
	}

	if _, err := uuid.Parse(value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": paramName + " must be a valid UUID",
		})
		c.Abort()
		return ""
	}

	return value
}
