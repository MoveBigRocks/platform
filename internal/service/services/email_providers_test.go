package serviceapp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
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

func TestPostmarkProvider_SendEmailIncludesCustomMessageIDHeader(t *testing.T) {
	provider := &PostmarkProvider{
		ServerToken: "test-token",
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)

				var payload map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &payload))

				headers, ok := payload["Headers"].([]interface{})
				require.True(t, ok)
				require.Len(t, headers, 1)

				header, ok := headers[0].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "Message-ID", header["Name"])
				assert.Equal(t, "<thread-123@example.com>", header["Value"])
				assert.Equal(t, "test-token", req.Header.Get("X-Postmark-Server-Token"))

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(`{"MessageID":"postmark-api-id"}`)),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}

	email := emaildom.NewOutboundEmail("ws_123", "sender@example.com", "Test Subject", "Test body")
	email.ToEmails = []string{"recipient@example.com"}
	email.ProviderSettings["header_message_id"] = "<thread-123@example.com>"

	err := provider.SendEmail(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, "postmark-api-id", email.ProviderMessageID)
}

func TestPostmarkProvider_ParseInboundEmailExtractsThreadHeadersAndAddresses(t *testing.T) {
	provider := &PostmarkProvider{}

	rawEmail := []byte(`{
		"From": "\"Casey Customer\" <customer@example.com>",
		"FromName": "Casey Customer",
		"FromFull": {
			"Email": "customer@example.com",
			"Name": "Casey Customer",
			"MailboxHash": ""
		},
		"To": "workspace-123@support.movebigrocks.test",
		"ToFull": [
			{
				"Email": "workspace-123@support.movebigrocks.test",
				"Name": "Support",
				"MailboxHash": ""
			}
		],
		"Cc": "\"Second Person\" <other@example.com>",
		"CcFull": [
			{
				"Email": "other@example.com",
				"Name": "Second Person",
				"MailboxHash": ""
			}
		],
		"Bcc": "audit@example.com",
		"BccFull": [
			{
				"Email": "audit@example.com",
				"Name": "",
				"MailboxHash": ""
			}
		],
		"Subject": "Re: Billing follow-up",
		"TextBody": "Quoted full email body",
		"StrippedTextReply": "Customer reply only",
		"HtmlBody": "<p>Quoted full email body</p>",
		"MessageID": "<reply@example.com>",
		"Headers": [
			{"Name": "Message-ID", "Value": "<reply@example.com>"},
			{"Name": "In-Reply-To", "Value": "<thread-123@example.com>"},
			{"Name": "References", "Value": "<thread-123@example.com> <older@example.com>"}
		]
	}`)

	inbound, err := provider.ParseInboundEmail(context.Background(), rawEmail)
	require.NoError(t, err)
	require.NotNil(t, inbound)
	assert.Equal(t, "customer@example.com", inbound.FromEmail)
	assert.Equal(t, "Casey Customer", inbound.FromName)
	assert.Equal(t, []string{"workspace-123@support.movebigrocks.test"}, inbound.ToEmails)
	assert.Equal(t, []string{"other@example.com"}, inbound.CCEmails)
	assert.Equal(t, []string{"audit@example.com"}, inbound.BCCEmails)
	assert.Equal(t, "<thread-123@example.com>", inbound.InReplyTo)
	assert.Equal(t, []string{"<thread-123@example.com>", "<older@example.com>"}, inbound.References)
	assert.Equal(t, "Customer reply only", inbound.TextContent)
	assert.Equal(t, "Quoted full email body", inbound.RawContent)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
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
