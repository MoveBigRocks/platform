package serviceapp

import (
	"context"
	netsmtp "net/smtp"
	"testing"

	emaildom "github.com/movebigrocks/platform/internal/service/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSendGridProvider(t *testing.T) {
	t.Run("returns error if API key is empty", func(t *testing.T) {
		config := EmailConfig{SendGridAPIKey: ""}
		_, err := NewSendGridProvider(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key is required")
	})

	t.Run("creates provider with valid API key", func(t *testing.T) {
		config := EmailConfig{SendGridAPIKey: "SG.test-key"}
		provider, err := NewSendGridProvider(config)

		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.Equal(t, "SG.test-key", provider.APIKey)
		assert.NotNil(t, provider.client)
	})
}

func TestSendGridProvider_ValidateConfig(t *testing.T) {
	t.Run("returns error if API key is empty", func(t *testing.T) {
		provider := &SendGridProvider{}
		err := provider.ValidateConfig()
		assert.Error(t, err)
	})

	t.Run("returns nil if API key is set", func(t *testing.T) {
		provider := &SendGridProvider{APIKey: "test-key"}
		err := provider.ValidateConfig()
		assert.NoError(t, err)
	})
}

func TestSendGridProvider_buildRecipients(t *testing.T) {
	provider := &SendGridProvider{}

	t.Run("builds recipients list", func(t *testing.T) {
		emails := []string{"user1@example.com", "user2@example.com"}
		recipients := provider.buildRecipients(emails)

		assert.Len(t, recipients, 2)
		assert.Equal(t, "user1@example.com", recipients[0]["email"])
		assert.Equal(t, "user2@example.com", recipients[1]["email"])
	})

	t.Run("trims whitespace from emails", func(t *testing.T) {
		emails := []string{"  user@example.com  "}
		recipients := provider.buildRecipients(emails)

		assert.Equal(t, "user@example.com", recipients[0]["email"])
	})

	t.Run("handles empty list", func(t *testing.T) {
		recipients := provider.buildRecipients([]string{})
		assert.Empty(t, recipients)
	})
}

func TestNewPostmarkProvider(t *testing.T) {
	t.Run("creates provider without token (for webhook parsing)", func(t *testing.T) {
		// Token is now optional at construction time - only required for sending
		config := EmailConfig{PostmarkServerToken: ""}
		provider, err := NewPostmarkProvider(config)

		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.Equal(t, "", provider.ServerToken)
	})

	t.Run("creates provider with valid token", func(t *testing.T) {
		config := EmailConfig{PostmarkServerToken: "test-token"}
		provider, err := NewPostmarkProvider(config)

		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.Equal(t, "test-token", provider.ServerToken)
	})

	t.Run("send requires token", func(t *testing.T) {
		config := EmailConfig{PostmarkServerToken: ""}
		provider, err := NewPostmarkProvider(config)
		require.NoError(t, err)

		email := &emaildom.OutboundEmail{
			ToEmails:  []string{"test@example.com"},
			FromEmail: "sender@example.com",
			Subject:   "Test",
		}
		err = provider.SendEmail(context.Background(), email)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server token is required")
	})
}

func TestPostmarkProvider_ValidateConfig(t *testing.T) {
	t.Run("returns error if token is empty", func(t *testing.T) {
		provider := &PostmarkProvider{}
		err := provider.ValidateConfig()
		assert.Error(t, err)
	})

	t.Run("returns nil if token is set", func(t *testing.T) {
		provider := &PostmarkProvider{ServerToken: "test-token"}
		err := provider.ValidateConfig()
		assert.NoError(t, err)
	})
}

func TestNewSESProvider(t *testing.T) {
	t.Run("returns error if region is empty", func(t *testing.T) {
		config := EmailConfig{SESRegion: ""}
		_, err := NewSESProvider(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "region is required")
	})

	t.Run("creates provider with valid config", func(t *testing.T) {
		config := EmailConfig{
			SESRegion:    "us-east-1",
			SESAccessKey: "access-key",
			SESSecretKey: "secret-key",
		}
		provider, err := NewSESProvider(config)

		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.Equal(t, "us-east-1", provider.Region)
		assert.Equal(t, "access-key", provider.AccessKey)
		assert.Equal(t, "secret-key", provider.SecretKey)
	})
}

func TestSESProvider_ValidateConfig(t *testing.T) {
	t.Run("returns error if region is empty", func(t *testing.T) {
		provider := &SESProvider{}
		err := provider.ValidateConfig()
		assert.Error(t, err)
	})

	t.Run("returns nil if region is set", func(t *testing.T) {
		provider := &SESProvider{Region: "us-west-2"}
		err := provider.ValidateConfig()
		assert.NoError(t, err)
	})
}

func TestNewSMTPProvider(t *testing.T) {
	t.Run("returns error if host is empty", func(t *testing.T) {
		config := EmailConfig{SMTPHost: ""}
		_, err := NewSMTPProvider(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "host is required")
	})

	t.Run("creates provider with valid config", func(t *testing.T) {
		config := EmailConfig{
			SMTPHost:     "smtp.example.com",
			SMTPPort:     587,
			SMTPUsername: "user",
			SMTPPassword: "pass",
		}
		provider, err := NewSMTPProvider(config)

		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.Equal(t, "smtp.example.com", provider.Host)
		assert.Equal(t, 587, provider.Port)
	})
}

func TestSMTPProvider_ValidateConfig(t *testing.T) {
	t.Run("returns error if host is empty", func(t *testing.T) {
		provider := &SMTPProvider{}
		err := provider.ValidateConfig()
		assert.Error(t, err)
	})

	t.Run("returns error if port is zero", func(t *testing.T) {
		provider := &SMTPProvider{Host: "smtp.example.com", Port: 0}
		err := provider.ValidateConfig()
		assert.Error(t, err)
	})

	t.Run("returns error if port is negative", func(t *testing.T) {
		provider := &SMTPProvider{Host: "smtp.example.com", Port: -1}
		err := provider.ValidateConfig()
		assert.Error(t, err)
	})

	t.Run("returns nil if host and port are valid", func(t *testing.T) {
		provider := &SMTPProvider{Host: "smtp.example.com", Port: 587}
		err := provider.ValidateConfig()
		assert.NoError(t, err)
	})
}

func TestSMTPProvider_ParseInboundEmail(t *testing.T) {
	provider := &SMTPProvider{}

	t.Run("returns not supported error", func(t *testing.T) {
		_, err := provider.ParseInboundEmail(context.Background(), []byte("raw email"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not supported")
	})
}

func TestNewMockProvider(t *testing.T) {
	provider := NewMockProvider()
	require.NotNil(t, provider)
	assert.Empty(t, provider.SentEmails)
}

func TestMockProvider_SendEmail(t *testing.T) {
	provider := NewMockProvider()
	ctx := context.Background()

	email := &emaildom.OutboundEmail{
		ToEmails:    []string{"recipient@example.com"},
		FromEmail:   "sender@example.com",
		Subject:     "Test Subject",
		HTMLContent: "<p>Hello</p>",
	}

	err := provider.SendEmail(ctx, email)
	require.NoError(t, err)

	assert.NotEmpty(t, email.ProviderMessageID)
	assert.Contains(t, email.ProviderMessageID, "mock_")
	assert.Equal(t, "Mock email sent successfully", email.ProviderResponse)
	assert.Len(t, provider.SentEmails, 1)
}

func TestMockProvider_ParseInboundEmail(t *testing.T) {
	provider := NewMockProvider()
	ctx := context.Background()

	rawEmail := []byte("This is the email body")
	inbound, err := provider.ParseInboundEmail(ctx, rawEmail)

	require.NoError(t, err)
	require.NotNil(t, inbound)
	assert.Equal(t, "Test Subject", inbound.Subject)
	assert.Equal(t, "test@example.com", inbound.FromEmail)
}

func TestMockProvider_ValidateConfig(t *testing.T) {
	provider := NewMockProvider()
	err := provider.ValidateConfig()
	assert.NoError(t, err) // Always passes for mock
}

func TestMockProvider_GetSentEmails(t *testing.T) {
	provider := NewMockProvider()
	ctx := context.Background()

	// Send two emails
	provider.SendEmail(ctx, &emaildom.OutboundEmail{Subject: "Email 1"})
	provider.SendEmail(ctx, &emaildom.OutboundEmail{Subject: "Email 2"})

	sent := provider.GetSentEmails()
	assert.Len(t, sent, 2)
}

func TestMockProvider_ClearSentEmails(t *testing.T) {
	provider := NewMockProvider()
	ctx := context.Background()

	provider.SendEmail(ctx, &emaildom.OutboundEmail{Subject: "Email 1"})
	assert.Len(t, provider.SentEmails, 1)

	provider.ClearSentEmails()
	assert.Empty(t, provider.SentEmails)
}

func TestSESProvider_SendEmail(t *testing.T) {
	provider := &SESProvider{Region: "us-east-1"}
	ctx := context.Background()

	email := &emaildom.OutboundEmail{
		ToEmails: []string{"recipient@example.com"},
		Subject:  "Test",
	}

	err := provider.SendEmail(ctx, email)
	require.NoError(t, err)

	assert.NotEmpty(t, email.ProviderMessageID)
	assert.Contains(t, email.ProviderMessageID, "ses_")
}

func TestSMTPProvider_SendEmail(t *testing.T) {
	var sentAddr, sentFrom string
	var sentRecipients []string
	var sentMessage []byte

	provider := &SMTPProvider{
		Host: "smtp.example.com",
		Port: 587,
		sendMail: func(addr string, _ netsmtp.Auth, from string, to []string, msg []byte) error {
			sentAddr = addr
			sentFrom = from
			sentRecipients = append([]string{}, to...)
			sentMessage = append([]byte(nil), msg...)
			return nil
		},
	}
	ctx := context.Background()

	email := &emaildom.OutboundEmail{
		ToEmails:    []string{"recipient@example.com"},
		FromEmail:   "sender@example.com",
		FromName:    "Sender",
		Subject:     "Test",
		TextContent: "hello",
	}

	err := provider.SendEmail(ctx, email)
	require.NoError(t, err)

	assert.NotEmpty(t, email.ProviderMessageID)
	assert.Equal(t, "SMTP accepted for delivery", email.ProviderResponse)
	assert.Equal(t, "smtp.example.com:587", sentAddr)
	assert.Equal(t, "sender@example.com", sentFrom)
	assert.Equal(t, []string{"recipient@example.com"}, sentRecipients)
	assert.Contains(t, string(sentMessage), "Subject: Test")
	assert.Contains(t, string(sentMessage), "From: \"Sender\" <sender@example.com>")
}
