package platformservices

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdminEmailService(t *testing.T) {
	tests := []struct {
		name     string
		provider string
	}{
		{"Default (mock) provider", ""},
		{"Mock provider", "mock"},
		{"Postmark provider", "postmark"},
		{"SendGrid provider", "sendgrid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AdminEmailConfig{
				Provider: tt.provider,
				APIKey:   "test-api-key",
				From:     "test@example.com",
				FromName: "Test Admin",
			}

			service := NewAdminEmailService(cfg)
			require.NotNil(t, service)
			assert.Equal(t, "test@example.com", service.from)
			assert.Equal(t, "Test Admin", service.fromName)
		})
	}
}

func TestAdminEmailService_SendMagicLinkEmail_Mock(t *testing.T) {
	ctx := context.Background()

	cfg := AdminEmailConfig{
		Provider: "mock",
		From:     "admin@example.com",
		FromName: "Test Admin",
	}

	service := NewAdminEmailService(cfg)

	// Send a magic link email using mock provider
	err := service.SendMagicLinkEmail(ctx, "user@example.com", "https://example.com/auth/verify?token=abc123")
	require.NoError(t, err)
}

func TestAdminMockProvider_SendEmail(t *testing.T) {
	ctx := context.Background()
	provider := &AdminMockProvider{from: "admin@example.com"}

	// Test sending email
	err := provider.SendEmail(ctx, []string{"user1@example.com", "user2@example.com"}, "Test Subject", "<p>Test Body</p>")
	require.NoError(t, err)
}

func TestAdminPostmarkProvider_SendEmail_BadConfig(t *testing.T) {
	// This test verifies the provider can be created but will fail on actual send
	// since we don't have a valid API key
	ctx := context.Background()
	provider := &AdminPostmarkProvider{
		apiKey: "invalid-key",
		from:   "admin@example.com",
	}

	// This will fail because the API key is invalid
	err := provider.SendEmail(ctx, []string{"user@example.com"}, "Test", "Body")
	// We expect an error because the API call will fail
	require.Error(t, err)
}

func TestAdminSendGridProvider_SendEmail_BadConfig(t *testing.T) {
	// This test verifies the provider can be created but will fail on actual send
	ctx := context.Background()
	provider := &AdminSendGridProvider{
		apiKey: "invalid-key",
		from:   "admin@example.com",
	}

	// This will fail because the API key is invalid
	err := provider.SendEmail(ctx, []string{"user@example.com"}, "Test", "Body")
	// We expect an error because the API call will fail
	require.Error(t, err)
}

// NOTE: Interface compliance is verified implicitly by the behavioral tests above.
// Compile-time checks like "var _ AdminEmailProvider = &AdminMockProvider{}"
// are redundant when the same types are used in actual test functions.
