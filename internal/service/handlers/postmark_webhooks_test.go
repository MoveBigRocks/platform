package servicehandlers

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Tests for helper functions that don't require complex mocks

func TestExtractWorkspaceID(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected string
	}{
		{
			name:     "valid workspace email",
			email:    "workspace-123@support.movebigrocks.test",
			expected: "workspace-123",
		},
		{
			name:     "valid subdomain email",
			email:    "support@acme.movebigrocks.com",
			expected: "acme",
		},
		{
			name:     "valid nested support subdomain email",
			email:    "support@acme.support.movebigrocks.test",
			expected: "acme",
		},
		{
			name:     "empty email",
			email:    "",
			expected: "",
		},
		{
			name:     "email without @",
			email:    "invalid-email",
			expected: "",
		},
		{
			name:     "simple local part",
			email:    "test@domain.com",
			expected: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractWorkspaceID(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}
