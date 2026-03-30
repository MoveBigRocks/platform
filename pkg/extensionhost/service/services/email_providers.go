package serviceapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	netsmtp "net/smtp"
	"strconv"
	"strings"
	"time"

	emaildom "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/logger"
	"github.com/movebigrocks/platform/pkg/util"
)

// SendGridProvider implements EmailProvider for SendGrid
type SendGridProvider struct {
	APIKey  string
	client  *http.Client
	baseURL string // For testing
}

// SESProvider implements EmailProvider for Amazon SES
type SESProvider struct {
	Region    string
	AccessKey string
	SecretKey string
}

// SMTPProvider implements EmailProvider for SMTP
type SMTPProvider struct {
	Host     string
	Port     int
	Username string
	Password string
	sendMail func(addr string, a netsmtp.Auth, from string, to []string, msg []byte) error
}

// MockProvider implements EmailProvider for testing
type MockProvider struct {
	SentEmails []emaildom.OutboundEmail
}

// NewSendGridProvider creates a new SendGrid email provider
func NewSendGridProvider(config EmailConfig) (*SendGridProvider, error) {
	if config.SendGridAPIKey == "" {
		return nil, fmt.Errorf("SendGrid API key is required")
	}

	return &SendGridProvider{
		APIKey:  config.SendGridAPIKey,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.sendgrid.com",
	}, nil
}

// SendEmail sends an email via SendGrid
func (sg *SendGridProvider) SendEmail(ctx context.Context, email *emaildom.OutboundEmail) error {
	// Build personalization
	personalization := map[string]interface{}{
		"to":      sg.buildRecipients(email.ToEmails),
		"subject": email.Subject,
	}

	// Add CC and BCC if present
	if len(email.CCEmails) > 0 {
		personalization["cc"] = sg.buildRecipients(email.CCEmails)
	}
	if len(email.BCCEmails) > 0 {
		personalization["bcc"] = sg.buildRecipients(email.BCCEmails)
	}

	// Build content - text first, then HTML (SendGrid prefers this order)
	var content []map[string]string
	if email.TextContent != "" {
		content = append(content, map[string]string{
			"type":  "text/plain",
			"value": email.TextContent,
		})
	}
	if email.HTMLContent != "" {
		content = append(content, map[string]string{
			"type":  "text/html",
			"value": email.HTMLContent,
		})
	}

	// Construct SendGrid API payload
	payload := map[string]interface{}{
		"personalizations": []map[string]interface{}{personalization},
		"from": map[string]string{
			"email": email.FromEmail,
			"name":  email.FromName,
		},
		"content": content,
	}

	// Add reply-to if present
	if email.ReplyToEmail != "" {
		payload["reply_to"] = map[string]string{
			"email": email.ReplyToEmail,
		}
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal SendGrid payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", sg.baseURL+"/v3/mail/send", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create SendGrid request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+sg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := sg.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send via SendGrid: %w", err)
	}
	defer resp.Body.Close()

	// SendGrid returns 202 Accepted for successful sends
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("SendGrid error: status %d (failed to read body: %v)", resp.StatusCode, err)
		}
		return fmt.Errorf("SendGrid error: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Extract message ID from X-Message-Id header
	messageID := resp.Header.Get("X-Message-Id")
	if messageID != "" {
		email.ProviderMessageID = messageID
	} else {
		email.ProviderMessageID = fmt.Sprintf("sg_%d", time.Now().Unix())
	}

	email.ProviderResponse = fmt.Sprintf("Status: %d, MessageID: %s", resp.StatusCode, email.ProviderMessageID)

	return nil
}

// ParseInboundEmail parses an inbound email from SendGrid webhook
func (sg *SendGridProvider) ParseInboundEmail(ctx context.Context, rawEmail []byte) (*emaildom.InboundEmail, error) {
	// Parse SendGrid inbound email webhook format
	var webhook map[string]interface{}
	if err := json.Unmarshal(rawEmail, &webhook); err != nil {
		return nil, fmt.Errorf("failed to parse SendGrid webhook: %w", err)
	}

	inboundEmail := emaildom.NewInboundEmail("",
		util.GetString(webhook, "headers.Message-ID"),
		util.GetString(webhook, "from"),
		util.GetString(webhook, "subject"),
		util.GetString(webhook, "text"))

	inboundEmail.HTMLContent = util.GetString(webhook, "html")
	inboundEmail.ToEmails = strings.Split(util.GetString(webhook, "to"), ",")

	return inboundEmail, nil
}

// ValidateConfig validates SendGrid configuration
func (sg *SendGridProvider) ValidateConfig() error {
	if sg.APIKey == "" {
		return fmt.Errorf("SendGrid API key is required")
	}
	return nil
}

// buildRecipients converts email addresses to SendGrid format
func (sg *SendGridProvider) buildRecipients(emails []string) []map[string]string {
	recipients := make([]map[string]string, len(emails))
	for i, email := range emails {
		recipients[i] = map[string]string{"email": strings.TrimSpace(email)}
	}
	return recipients
}

// PostmarkProvider implements EmailProvider for Postmark
type PostmarkProvider struct {
	ServerToken string
	client      *http.Client
}

// NewPostmarkProvider creates a new Postmark email provider
// Token is optional for parsing-only use cases (inbound webhooks)
// but required for sending emails
func NewPostmarkProvider(config EmailConfig) (*PostmarkProvider, error) {
	return &PostmarkProvider{
		ServerToken: config.PostmarkServerToken,
		client:      &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// SendEmail sends an email via Postmark
func (pm *PostmarkProvider) SendEmail(ctx context.Context, email *emaildom.OutboundEmail) error {
	// Validate token is present for sending
	if pm.ServerToken == "" {
		return fmt.Errorf("Postmark server token is required for sending emails")
	}

	// Construct Postmark API payload
	payload := map[string]interface{}{
		"From":     fmt.Sprintf("%s <%s>", email.FromName, email.FromEmail),
		"To":       strings.Join(email.ToEmails, ","),
		"Subject":  email.Subject,
		"HtmlBody": email.HTMLContent,
	}

	// Add text content if available
	if email.TextContent != "" {
		payload["TextBody"] = email.TextContent
	}

	// Add CC and BCC if present
	if len(email.CCEmails) > 0 {
		payload["Cc"] = strings.Join(email.CCEmails, ",")
	}
	if len(email.BCCEmails) > 0 {
		payload["Bcc"] = strings.Join(email.BCCEmails, ",")
	}

	// Add reply-to if present
	if email.ReplyToEmail != "" {
		payload["ReplyTo"] = email.ReplyToEmail
	}
	if messageID := outboundHeaderMessageID(email); messageID != "" {
		payload["Headers"] = []map[string]string{
			{
				"Name":  "Message-ID",
				"Value": messageID,
			},
		}
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Postmark payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.postmarkapp.com/email", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create Postmark request: %w", err)
	}

	req.Header.Set("X-Postmark-Server-Token", pm.ServerToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := pm.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send via Postmark: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var postmarkResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&postmarkResp); err != nil {
		return fmt.Errorf("failed to decode Postmark response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		errorMsg := util.GetString(postmarkResp, "Message")
		if errorMsg == "" {
			errorMsg = fmt.Sprintf("Postmark returned status %d", resp.StatusCode)
		}
		return fmt.Errorf("postmark error: %s", errorMsg)
	}

	// Extract message ID from response
	if messageID := util.GetString(postmarkResp, "MessageID"); messageID != "" {
		email.ProviderMessageID = messageID
	}

	email.ProviderResponse = fmt.Sprintf("Status: %d, MessageID: %s", resp.StatusCode, email.ProviderMessageID)

	return nil
}

// ParseInboundEmail parses an inbound email from Postmark webhook
func (pm *PostmarkProvider) ParseInboundEmail(ctx context.Context, rawEmail []byte) (*emaildom.InboundEmail, error) {
	// Parse Postmark inbound webhook format
	var webhook map[string]interface{}
	if err := json.Unmarshal(rawEmail, &webhook); err != nil {
		return nil, fmt.Errorf("failed to parse Postmark webhook: %w", err)
	}

	// Extract basic fields
	messageID := util.GetString(webhook, "MessageID")
	fromEmail, fromName := postmarkPrimaryAddress(webhook, "FromFull", "From")
	subject := util.GetString(webhook, "Subject")
	rawTextBody := util.GetString(webhook, "TextBody")
	textBody := strings.TrimSpace(util.GetString(webhook, "StrippedTextReply"))
	if textBody == "" {
		textBody = rawTextBody
	}
	htmlBody := util.GetString(webhook, "HtmlBody")

	// Create inbound email
	inboundEmail := emaildom.NewInboundEmail("",
		messageID,
		fromEmail,
		subject,
		textBody)

	inboundEmail.FromName = fromName
	inboundEmail.HTMLContent = htmlBody
	inboundEmail.RawContent = rawTextBody
	inboundEmail.ToEmails = postmarkAddressEmails(webhook, "ToFull", "To")
	inboundEmail.CCEmails = postmarkAddressEmails(webhook, "CcFull", "Cc")
	inboundEmail.BCCEmails = postmarkAddressEmails(webhook, "BccFull", "Bcc")

	// Extract headers
	inboundEmail.Headers = postmarkHeaders(webhook["Headers"])
	inboundEmail.InReplyTo = postmarkHeaderValue(inboundEmail.Headers, "In-Reply-To")
	inboundEmail.References = parseMessageIDHeaderList(postmarkHeaderValue(inboundEmail.Headers, "References"))

	// Extract attachments if present
	// Note: Actual attachment processing (virus scanning, S3 upload) happens
	// in the webhook handler via AttachmentService
	if attachments, ok := webhook["Attachments"].([]interface{}); ok {
		inboundEmail.AttachmentCount = len(attachments)
		var totalSize int64
		for _, att := range attachments {
			if a, ok := att.(map[string]interface{}); ok {
				if size, ok := a["ContentLength"].(float64); ok {
					totalSize += int64(size)
				}
			}
		}
		inboundEmail.TotalAttachmentSize = totalSize
	}

	return inboundEmail, nil
}

func postmarkPrimaryAddress(webhook map[string]interface{}, fullKey, rawKey string) (string, string) {
	addresses := postmarkAddressList(webhook, fullKey, rawKey)
	if len(addresses) == 0 {
		return "", ""
	}
	return addresses[0].Address, addresses[0].Name
}

func postmarkAddressEmails(webhook map[string]interface{}, fullKey, rawKey string) []string {
	addresses := postmarkAddressList(webhook, fullKey, rawKey)
	emails := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if strings.TrimSpace(address.Address) == "" {
			continue
		}
		emails = append(emails, address.Address)
	}
	return emails
}

func postmarkAddressList(webhook map[string]interface{}, fullKey, rawKey string) []*mail.Address {
	if webhook == nil {
		return nil
	}
	if fullAddresses := parsePostmarkAddressObjects(webhook[fullKey]); len(fullAddresses) > 0 {
		return fullAddresses
	}
	return parseAddressListString(util.GetString(webhook, rawKey))
}

func parsePostmarkAddressObjects(raw interface{}) []*mail.Address {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	addresses := make([]*mail.Address, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		email := strings.TrimSpace(util.GetString(entry, "Email"))
		if email == "" {
			continue
		}
		addresses = append(addresses, &mail.Address{
			Name:    strings.TrimSpace(util.GetString(entry, "Name")),
			Address: email,
		})
	}
	return addresses
}

func parseAddressListString(value string) []*mail.Address {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if parsed, err := mail.ParseAddressList(value); err == nil {
		return parsed
	}

	parts := strings.Split(value, ",")
	addresses := make([]*mail.Address, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if parsed, err := mail.ParseAddress(part); err == nil {
			addresses = append(addresses, parsed)
			continue
		}
		addresses = append(addresses, &mail.Address{Address: part})
	}
	return addresses
}

func postmarkHeaders(raw interface{}) map[string]string {
	headers := make(map[string]string)
	items, ok := raw.([]interface{})
	if !ok {
		return headers
	}
	for _, item := range items {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name := strings.TrimSpace(util.GetString(entry, "Name"))
		value := util.GetString(entry, "Value")
		if name == "" {
			continue
		}
		headers[name] = value
	}
	return headers
}

func postmarkHeaderValue(headers map[string]string, target string) string {
	target = strings.TrimSpace(strings.ToLower(target))
	for name, value := range headers {
		if strings.ToLower(strings.TrimSpace(name)) == target {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseMessageIDHeaderList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}
	}
	replacer := strings.NewReplacer(",", " ", "\n", " ", "\r", " ", "\t", " ")
	parts := strings.Fields(replacer.Replace(value))
	seen := make(map[string]struct{}, len(parts))
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		values = append(values, part)
	}
	return values
}

// ValidateConfig validates Postmark configuration
func (pm *PostmarkProvider) ValidateConfig() error {
	if pm.ServerToken == "" {
		return fmt.Errorf("Postmark server token is required")
	}
	return nil
}

// NewSESProvider creates a new SES email provider
func NewSESProvider(config EmailConfig) (*SESProvider, error) {
	if config.SESRegion == "" {
		return nil, fmt.Errorf("SES region is required")
	}

	return &SESProvider{
		Region:    config.SESRegion,
		AccessKey: config.SESAccessKey,
		SecretKey: config.SESSecretKey,
	}, nil
}

// SendEmail sends an email via Amazon SES
func (ses *SESProvider) SendEmail(ctx context.Context, email *emaildom.OutboundEmail) error {
	// In a real implementation, we would use AWS SDK to send via SES
	log := logger.New()
	log.Debugw("SES mock sending email", "to", email.ToEmails, "subject", email.Subject)

	email.ProviderMessageID = fmt.Sprintf("ses_%d", time.Now().Unix())
	email.ProviderResponse = "MessageId: " + email.ProviderMessageID

	return nil
}

// ParseInboundEmail parses an inbound email from SES
func (ses *SESProvider) ParseInboundEmail(ctx context.Context, rawEmail []byte) (*emaildom.InboundEmail, error) {
	// Parse SES SNS notification format
	var notification map[string]interface{}
	if err := json.Unmarshal(rawEmail, &notification); err != nil {
		return nil, fmt.Errorf("failed to parse SES notification: %w", err)
	}

	// Extract email from SNS message
	message := util.GetString(notification, "Message")
	var sesMessage map[string]interface{}
	if err := json.Unmarshal([]byte(message), &sesMessage); err != nil {
		return nil, fmt.Errorf("failed to parse SES message: %w", err)
	}

	content := util.GetString(sesMessage, "content")
	msg, err := mail.ReadMessage(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	inboundEmail := emaildom.NewInboundEmail("",
		msg.Header.Get("Message-ID"),
		msg.Header.Get("From"),
		msg.Header.Get("Subject"),
		"") // Would extract text content from body

	return inboundEmail, nil
}

// ValidateConfig validates SES configuration
func (ses *SESProvider) ValidateConfig() error {
	if ses.Region == "" {
		return fmt.Errorf("SES region is required")
	}
	return nil
}

// NewSMTPProvider creates a new SMTP email provider
func NewSMTPProvider(config EmailConfig) (*SMTPProvider, error) {
	if config.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP host is required")
	}

	return &SMTPProvider{
		Host:     config.SMTPHost,
		Port:     config.SMTPPort,
		Username: config.SMTPUsername,
		Password: config.SMTPPassword,
		sendMail: netsmtp.SendMail,
	}, nil
}

// SendEmail sends an email via SMTP
func (smtp *SMTPProvider) SendEmail(ctx context.Context, email *emaildom.OutboundEmail) error {
	if err := smtp.ValidateConfig(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if email.FromEmail == "" {
		return fmt.Errorf("from email is required")
	}
	if len(email.ToEmails) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	messageID := outboundHeaderMessageID(email)
	if messageID == "" {
		messageID = fmt.Sprintf("<%d@%s>", time.Now().UnixNano(), smtp.Host)
	}
	message, err := smtp.buildMessage(email, messageID, shouldKeepSMTPMessageID(smtp.Host, messageID))
	if err != nil {
		return fmt.Errorf("build smtp message: %w", err)
	}

	sendMail := smtp.sendMail
	if sendMail == nil {
		sendMail = netsmtp.SendMail
	}

	addr := net.JoinHostPort(smtp.Host, strconv.Itoa(smtp.Port))
	if err := sendMail(addr, smtp.auth(), email.FromEmail, smtp.allRecipients(email), message); err != nil {
		return fmt.Errorf("failed to send via SMTP: %w", err)
	}

	email.ProviderMessageID = messageID
	email.ProviderResponse = "SMTP accepted for delivery"
	return nil
}

// ParseInboundEmail parses an inbound email (SMTP providers don't typically support this)
func (smtp *SMTPProvider) ParseInboundEmail(ctx context.Context, rawEmail []byte) (*emaildom.InboundEmail, error) {
	return nil, fmt.Errorf("inbound email parsing not supported for SMTP provider")
}

// ValidateConfig validates SMTP configuration
func (smtp *SMTPProvider) ValidateConfig() error {
	if smtp.Host == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if smtp.Port <= 0 {
		return fmt.Errorf("valid SMTP port is required")
	}
	return nil
}

func (smtp *SMTPProvider) auth() netsmtp.Auth {
	if strings.TrimSpace(smtp.Username) == "" {
		return nil
	}
	return netsmtp.PlainAuth("", smtp.Username, smtp.Password, smtp.Host)
}

func (smtp *SMTPProvider) allRecipients(email *emaildom.OutboundEmail) []string {
	recipients := make([]string, 0, len(email.ToEmails)+len(email.CCEmails)+len(email.BCCEmails))
	recipients = append(recipients, email.ToEmails...)
	recipients = append(recipients, email.CCEmails...)
	recipients = append(recipients, email.BCCEmails...)
	return recipients
}

func (smtp *SMTPProvider) buildMessage(email *emaildom.OutboundEmail, messageID string, keepMessageID bool) ([]byte, error) {
	var body bytes.Buffer

	writeHeader := func(name, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		body.WriteString(name)
		body.WriteString(": ")
		body.WriteString(value)
		body.WriteString("\r\n")
	}

	writeHeader("From", formatEmailAddress(email.FromName, email.FromEmail))
	writeHeader("To", strings.Join(email.ToEmails, ", "))
	if len(email.CCEmails) > 0 {
		writeHeader("Cc", strings.Join(email.CCEmails, ", "))
	}
	writeHeader("Reply-To", email.ReplyToEmail)
	writeHeader("Subject", email.Subject)
	writeHeader("Date", time.Now().UTC().Format(time.RFC1123Z))
	writeHeader("Message-ID", messageID)
	if keepMessageID {
		writeHeader("X-PM-KeepID", "true")
	}
	writeHeader("MIME-Version", "1.0")

	switch {
	case email.TextContent != "" && email.HTMLContent != "":
		boundary := fmt.Sprintf("mbr-alt-%d", time.Now().UnixNano())
		writeHeader("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", boundary))
		body.WriteString("\r\n")
		body.WriteString("--" + boundary + "\r\n")
		body.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		body.WriteString(email.TextContent)
		body.WriteString("\r\n--" + boundary + "\r\n")
		body.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		body.WriteString(email.HTMLContent)
		body.WriteString("\r\n--" + boundary + "--\r\n")
	case email.HTMLContent != "":
		writeHeader("Content-Type", "text/html; charset=UTF-8")
		body.WriteString("\r\n")
		body.WriteString(email.HTMLContent)
	default:
		writeHeader("Content-Type", "text/plain; charset=UTF-8")
		body.WriteString("\r\n")
		body.WriteString(email.TextContent)
	}

	return body.Bytes(), nil
}

func formatEmailAddress(name, address string) string {
	if strings.TrimSpace(name) == "" {
		return address
	}
	return (&mail.Address{Name: name, Address: address}).String()
}

func shouldKeepSMTPMessageID(host, messageID string) bool {
	if strings.TrimSpace(messageID) == "" {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(host)), "postmark")
}

// NewMockProvider creates a mock email provider for testing
func NewMockProvider() *MockProvider {
	return &MockProvider{
		SentEmails: make([]emaildom.OutboundEmail, 0),
	}
}

// SendEmail mock sends an email (stores it in memory)
func (mock *MockProvider) SendEmail(ctx context.Context, email *emaildom.OutboundEmail) error {
	log := logger.New()
	log.Debugw("Mock sending email", "to", email.ToEmails, "subject", email.Subject)

	email.ProviderMessageID = fmt.Sprintf("mock_%d", time.Now().Unix())
	email.ProviderResponse = "Mock email sent successfully"

	mock.SentEmails = append(mock.SentEmails, *email)
	return nil
}

// ParseInboundEmail mock parses inbound email
func (mock *MockProvider) ParseInboundEmail(ctx context.Context, rawEmail []byte) (*emaildom.InboundEmail, error) {
	// Simple mock parsing
	// Use UnixNano to avoid unique constraint collisions in tests running quickly
	inboundEmail := emaildom.NewInboundEmail("",
		fmt.Sprintf("mock_%d", time.Now().UnixNano()),
		"test@example.com",
		"Test Subject",
		string(rawEmail))

	return inboundEmail, nil
}

// ValidateConfig mock validation (always passes)
func (mock *MockProvider) ValidateConfig() error {
	return nil
}

// GetSentEmails returns all sent emails (for testing)
func (mock *MockProvider) GetSentEmails() []emaildom.OutboundEmail {
	return mock.SentEmails
}

// ClearSentEmails clears the sent emails list
func (mock *MockProvider) ClearSentEmails() {
	mock.SentEmails = make([]emaildom.OutboundEmail, 0)
}
