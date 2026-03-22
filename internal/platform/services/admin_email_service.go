package platformservices

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/movebigrocks/platform/pkg/logger"
)

// AdminEmailService handles sending transactional emails for admin operations
// This is a simplified email service for platform-level emails (magic links, etc.)
// without workspace context.
type AdminEmailService struct {
	provider AdminEmailProvider
	from     string
	fromName string
}

// AdminEmailConfig holds configuration for admin email sending
type AdminEmailConfig struct {
	Provider string // "postmark", "sendgrid", or "mock"
	APIKey   string
	From     string
	FromName string
}

// AdminEmailProvider defines the interface for sending transactional emails
type AdminEmailProvider interface {
	SendEmail(ctx context.Context, to []string, subject, htmlBody string) error
}

// NewAdminEmailService creates a new admin email service
func NewAdminEmailService(cfg AdminEmailConfig) *AdminEmailService {
	var provider AdminEmailProvider
	switch strings.ToLower(cfg.Provider) {
	case "sendgrid":
		provider = &AdminSendGridProvider{apiKey: cfg.APIKey, from: cfg.From}
	case "postmark":
		provider = &AdminPostmarkProvider{apiKey: cfg.APIKey, from: cfg.From}
	default:
		provider = &AdminMockProvider{from: cfg.From}
	}

	return &AdminEmailService{
		provider: provider,
		from:     cfg.From,
		fromName: cfg.FromName,
	}
}

// SendMagicLinkEmail sends a magic link authentication email
func (s *AdminEmailService) SendMagicLinkEmail(ctx context.Context, toEmail, magicLinkURL string) error {
	subject := "Your Move Big Rocks Admin Login Link"
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 40px auto; padding: 20px; }
        .header { text-align: center; margin-bottom: 30px; }
        .button { display: inline-block; background: #4F46E5; color: white; padding: 14px 28px; text-decoration: none; border-radius: 6px; font-weight: 600; margin: 20px 0; }
        .button:hover { background: #4338CA; }
        .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; font-size: 12px; color: #666; }
        .link { word-break: break-all; color: #4F46E5; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Admin Login</h1>
        </div>
        <p>Click the button below to sign in to the Move Big Rocks admin panel:</p>
        <p style="text-align: center;">
            <a href="%s" class="button">Sign In to Admin Panel</a>
        </p>
        <p>Or copy and paste this link:</p>
        <p class="link">%s</p>
        <p><strong>This link will expire in 15 minutes.</strong></p>
        <div class="footer">
            <p>If you didn't request this login link, you can safely ignore this email.</p>
        </div>
    </div>
</body>
</html>
`, magicLinkURL, magicLinkURL)

	return s.provider.SendEmail(ctx, []string{toEmail}, subject, htmlBody)
}

// --- Postmark Implementation ---

type AdminPostmarkProvider struct {
	apiKey string
	from   string
	client *http.Client
}

func (p *AdminPostmarkProvider) SendEmail(ctx context.Context, to []string, subject, htmlBody string) error {
	type postmarkMessage struct {
		From     string `json:"From"`
		To       string `json:"To"`
		Subject  string `json:"Subject"`
		HtmlBody string `json:"HtmlBody"`
	}

	msg := postmarkMessage{
		From:     p.from,
		To:       strings.Join(to, ","),
		Subject:  subject,
		HtmlBody: htmlBody,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal Postmark message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.postmarkapp.com/email", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create Postmark request: %w", err)
	}

	req.Header.Set("X-Postmark-Server-Token", p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Lazy-initialize HTTP client (reused across requests)
	if p.client == nil {
		p.client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send via Postmark: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read response body for debugging (ignore read errors)
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck // debug logging only
		return fmt.Errorf("Postmark returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// --- SendGrid Implementation ---

type AdminSendGridProvider struct {
	apiKey string
	from   string
	client *http.Client
}

func (p *AdminSendGridProvider) SendEmail(ctx context.Context, to []string, subject, htmlBody string) error {
	type sendGridMessage struct {
		Personalizations []struct {
			To []struct {
				Email string `json:"email"`
			} `json:"to"`
		} `json:"personalizations"`
		From struct {
			Email string `json:"email"`
		} `json:"from"`
		Subject string `json:"subject"`
		Content []struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"content"`
	}

	msg := sendGridMessage{}
	msg.From.Email = p.from
	msg.Subject = subject
	msg.Personalizations = make([]struct {
		To []struct {
			Email string `json:"email"`
		} `json:"to"`
	}, 1)
	msg.Personalizations[0].To = make([]struct {
		Email string `json:"email"`
	}, len(to))
	for i, recipient := range to {
		msg.Personalizations[0].To[i].Email = recipient
	}
	msg.Content = append(msg.Content, struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}{Type: "text/html", Value: htmlBody})

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal SendGrid message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create SendGrid request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Lazy-initialize HTTP client (reused across requests)
	if p.client == nil {
		p.client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send via SendGrid: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		// Read response body for debugging (ignore read errors)
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck // debug logging only
		return fmt.Errorf("SendGrid returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// --- Mock Implementation ---

type AdminMockProvider struct {
	from   string
	logger *logger.Logger
}

func (p *AdminMockProvider) SendEmail(ctx context.Context, to []string, subject, htmlBody string) error {
	if p.logger == nil {
		p.logger = logger.New()
	}
	p.logger.Debugw("Sending mock email",
		"from", p.from,
		"to", strings.Join(to, ", "),
		"subject", subject,
		"body_length", len(htmlBody))
	return nil
}
