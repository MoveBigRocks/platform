package serviceapp

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	sharedevents "github.com/movebigrocks/platform/internal/shared/events"

	emaildom "github.com/movebigrocks/platform/internal/service/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// EmailService handles email operations orchestration
type EmailService struct {
	store       stores.Store
	provider    EmailProvider
	config      EmailConfig
	caseService *CaseService
	logger      *logger.Logger
}

// EmailProvider defines the interface for email providers
type EmailProvider interface {
	SendEmail(ctx context.Context, email *emaildom.OutboundEmail) error
	ParseInboundEmail(ctx context.Context, rawEmail []byte) (*emaildom.InboundEmail, error)
	ValidateConfig() error
}

type EmailProviderFactory func(config EmailConfig) (EmailProvider, error)

type EmailProviderRegistry struct {
	factories map[string]EmailProviderFactory
}

const outboundHeaderMessageIDKey = "header_message_id"

func NewEmailProviderRegistry() *EmailProviderRegistry {
	registry := &EmailProviderRegistry{factories: make(map[string]EmailProviderFactory)}
	registry.Register("sendgrid", func(config EmailConfig) (EmailProvider, error) {
		return NewSendGridProvider(config)
	})
	registry.Register("postmark", func(config EmailConfig) (EmailProvider, error) {
		return NewPostmarkProvider(config)
	})
	registry.Register("ses", func(config EmailConfig) (EmailProvider, error) {
		return NewSESProvider(config)
	})
	registry.Register("smtp", func(config EmailConfig) (EmailProvider, error) {
		return NewSMTPProvider(config)
	})
	registry.Register("mock", func(config EmailConfig) (EmailProvider, error) {
		return NewMockProvider(), nil
	})
	registry.Register("none", func(config EmailConfig) (EmailProvider, error) {
		return NewMockProvider(), nil
	})
	return registry
}

func (r *EmailProviderRegistry) Register(name string, factory EmailProviderFactory) {
	if r == nil || factory == nil {
		return
	}
	r.factories[normalizeEmailBackendName(name)] = factory
}

func (r *EmailProviderRegistry) NewProvider(config EmailConfig) (EmailProvider, error) {
	if r == nil {
		return nil, fmt.Errorf("email provider registry is required")
	}

	backend := normalizeEmailBackendName(config.Provider)
	factory, ok := r.factories[backend]
	if !ok {
		return nil, fmt.Errorf("unsupported email backend %q", config.Provider)
	}
	return factory(config)
}

// EmailConfig contains email service configuration
type EmailConfig struct {
	Provider             string
	DefaultFromName      string
	DefaultFromEmail     string
	SendGridAPIKey       string
	PostmarkServerToken  string
	PostmarkAccountToken string // For management API (optional)
	SESRegion            string
	SESAccessKey         string
	SESSecretKey         string
	SMTPHost             string
	SMTPPort             int
	SMTPUsername         string
	SMTPPassword         string
	WebhookSecret        string
	MaxRetries           int
	RetryDelay           time.Duration
}

// NewEmailService creates a new email service
func NewEmailService(store stores.Store, config EmailConfig, caseService *CaseService) (*EmailService, error) {
	return NewEmailServiceWithRegistry(store, config, caseService, NewEmailProviderRegistry())
}

func NewEmailServiceWithRegistry(store stores.Store, config EmailConfig, caseService *CaseService, registry *EmailProviderRegistry) (*EmailService, error) {
	provider, err := registry.NewProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize email provider: %w", err)
	}

	if err := provider.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid email provider configuration: %w", err)
	}

	service := &EmailService{
		store:       store,
		provider:    provider,
		config:      config,
		caseService: caseService,
		logger:      logger.New(),
	}

	return service, nil
}

func normalizeEmailBackendName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "mock"
	}
	return value
}

// ProcessSendEmailRequested persists and sends an outbound email from a command
// event. If OutboundEmailID is present, the existing outbound email record is
// used and updated; otherwise a new record is created from the event payload.
func (es *EmailService) ProcessSendEmailRequested(ctx context.Context, event sharedevents.SendEmailRequestedEvent) (*emaildom.OutboundEmail, error) {
	if es.store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if strings.TrimSpace(event.WorkspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	var outboundEmail *emaildom.OutboundEmail
	if strings.TrimSpace(event.OutboundEmailID) != "" {
		existing, err := es.store.OutboundEmails().GetOutboundEmail(ctx, event.OutboundEmailID)
		if err != nil {
			return nil, fmt.Errorf("load outbound email: %w", err)
		}
		outboundEmail = existing
	} else {
		outboundEmail = es.buildOutboundEmailFromCommand(event)
		if err := outboundEmail.Validate(); err != nil {
			return nil, fmt.Errorf("build outbound email: %w", err)
		}
		if err := es.store.OutboundEmails().CreateOutboundEmail(ctx, outboundEmail); err != nil {
			return nil, fmt.Errorf("create outbound email: %w", err)
		}
	}

	if err := es.sendOutboundEmail(ctx, outboundEmail); err != nil {
		return nil, err
	}
	return outboundEmail, nil
}

func (es *EmailService) buildOutboundEmailFromCommand(event sharedevents.SendEmailRequestedEvent) *emaildom.OutboundEmail {
	fromEmail := strings.TrimSpace(es.config.DefaultFromEmail)
	if fromEmail == "" {
		fromEmail = strings.TrimSpace(event.ReplyTo)
	}
	if fromEmail == "" {
		fromEmail = "noreply@movebigrocks.test"
	}

	fromName := strings.TrimSpace(es.config.DefaultFromName)
	if fromName == "" {
		fromName = strings.TrimSpace(event.RequestedBy)
	}
	if fromName == "" {
		fromName = "Move Big Rocks"
	}

	email := emaildom.NewOutboundEmail(event.WorkspaceID, fromEmail, event.Subject, event.TextContent)
	email.FromName = fromName
	email.ToEmails = append([]string(nil), event.ToEmails...)
	email.CCEmails = append([]string(nil), event.CcEmails...)
	email.HTMLContent = event.HTMLContent
	email.ReplyToEmail = strings.TrimSpace(event.ReplyTo)
	email.TemplateID = event.TemplateID
	email.TemplateData = cloneTemplateData(event.TemplateData)
	email.Category = event.Category
	email.CaseID = event.CaseID
	email.CreatedAt = time.Now().UTC()
	email.UpdatedAt = email.CreatedAt
	email.Provider = emaildom.EmailProvider(normalizeEmailBackendName(es.config.Provider))
	return email
}

func (es *EmailService) sendOutboundEmail(ctx context.Context, outboundEmail *emaildom.OutboundEmail) error {
	if outboundEmail == nil {
		return fmt.Errorf("outbound email is required")
	}
	if es.provider == nil {
		return fmt.Errorf("email provider is required")
	}
	if outboundEmail.Status == emaildom.EmailStatusSent || outboundEmail.Status == emaildom.EmailStatusDelivered {
		return nil
	}

	ensureOutboundHeaderMessageID(outboundEmail)

	now := time.Now().UTC()
	outboundEmail.Status = emaildom.EmailStatusSending
	outboundEmail.UpdatedAt = now
	if err := es.store.OutboundEmails().UpdateOutboundEmail(ctx, outboundEmail); err != nil {
		return fmt.Errorf("mark outbound email sending: %w", err)
	}

	if err := es.provider.SendEmail(ctx, outboundEmail); err != nil {
		outboundEmail.Status = emaildom.EmailStatusFailed
		outboundEmail.ErrorMessage = err.Error()
		outboundEmail.RetryCount++
		outboundEmail.UpdatedAt = time.Now().UTC()
		if updateErr := es.store.OutboundEmails().UpdateOutboundEmail(ctx, outboundEmail); updateErr != nil {
			return fmt.Errorf("send outbound email: %w (also failed to persist failure state: %v)", err, updateErr)
		}
		return fmt.Errorf("send outbound email: %w", err)
	}

	sentAt := time.Now().UTC()
	outboundEmail.Status = emaildom.EmailStatusSent
	outboundEmail.SentAt = &sentAt
	outboundEmail.UpdatedAt = sentAt
	if err := es.store.OutboundEmails().UpdateOutboundEmail(ctx, outboundEmail); err != nil {
		return fmt.Errorf("mark outbound email sent: %w", err)
	}

	if err := es.syncCommunicationMessageID(ctx, outboundEmail); err != nil {
		return fmt.Errorf("sync communication message id: %w", err)
	}
	return nil
}

func (es *EmailService) syncCommunicationMessageID(ctx context.Context, outboundEmail *emaildom.OutboundEmail) error {
	if outboundEmail == nil {
		return nil
	}
	if strings.TrimSpace(outboundEmail.CommunicationID) == "" {
		return nil
	}
	messageID := outboundHeaderMessageID(outboundEmail)
	if messageID == "" {
		messageID = strings.TrimSpace(outboundEmail.ProviderMessageID)
	}
	if messageID == "" {
		return nil
	}

	comm, err := es.store.Cases().GetCommunication(ctx, outboundEmail.WorkspaceID, outboundEmail.CommunicationID)
	if err != nil {
		return fmt.Errorf("get communication: %w", err)
	}
	if comm.MessageID == messageID {
		return nil
	}
	comm.MessageID = messageID
	comm.UpdatedAt = time.Now().UTC()
	if err := es.store.Cases().UpdateCommunication(ctx, comm); err != nil {
		return fmt.Errorf("update communication: %w", err)
	}
	return nil
}

func cloneTemplateData(data map[string]interface{}) map[string]interface{} {
	if len(data) == 0 {
		return make(map[string]interface{})
	}
	cloned := make(map[string]interface{}, len(data))
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func ensureOutboundHeaderMessageID(outboundEmail *emaildom.OutboundEmail) string {
	if outboundEmail == nil {
		return ""
	}
	if messageID := outboundHeaderMessageID(outboundEmail); messageID != "" {
		return messageID
	}
	if outboundEmail.ProviderSettings == nil {
		outboundEmail.ProviderSettings = make(map[string]interface{})
	}

	domain := "movebigrocks.local"
	if parsed, err := mail.ParseAddress(strings.TrimSpace(outboundEmail.FromEmail)); err == nil {
		if at := strings.LastIndex(parsed.Address, "@"); at >= 0 && at+1 < len(parsed.Address) {
			domain = strings.ToLower(strings.TrimSpace(parsed.Address[at+1:]))
		}
	}

	identifier := strings.TrimSpace(outboundEmail.ID)
	if identifier == "" {
		identifier = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	messageID := fmt.Sprintf("<outbound-%s@%s>", identifier, domain)
	outboundEmail.ProviderSettings[outboundHeaderMessageIDKey] = messageID
	return messageID
}

func outboundHeaderMessageID(outboundEmail *emaildom.OutboundEmail) string {
	if outboundEmail == nil || len(outboundEmail.ProviderSettings) == 0 {
		return ""
	}
	raw, ok := outboundEmail.ProviderSettings[outboundHeaderMessageIDKey]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

// =============================================================================
// Webhook Handler Methods
// =============================================================================

// CreateInboundEmailWithTenantContext stores an inbound email with explicit tenant context.
// This is used for webhook endpoints that bypass normal RLS middleware.
func (es *EmailService) CreateInboundEmailWithTenantContext(ctx context.Context, workspaceID string, email *emaildom.InboundEmail) error {
	if email == nil {
		return fmt.Errorf("email is required")
	}
	email.WorkspaceID = workspaceID
	return es.store.WithTransaction(ctx, func(txCtx context.Context) error {
		// Set tenant context for RLS policies
		if err := es.store.SetTenantContext(txCtx, workspaceID); err != nil {
			return fmt.Errorf("set tenant context: %w", err)
		}
		// Store email within tenant-scoped transaction
		if err := es.store.InboundEmails().CreateInboundEmail(txCtx, email); err != nil {
			return fmt.Errorf("create inbound email: %w", err)
		}
		return nil
	})
}

// MarkOutboundEmailBounced updates an outbound email status to bounced by provider message ID.
// Uses admin context to perform cross-tenant lookup.
func (es *EmailService) MarkOutboundEmailBounced(ctx context.Context, providerMessageID, bounceType, description string) (*emaildom.OutboundEmail, error) {
	var outboundEmail *emaildom.OutboundEmail
	err := es.store.WithAdminContext(ctx, func(adminCtx context.Context) error {
		// Find outbound email by provider message ID
		var fetchErr error
		outboundEmail, fetchErr = es.store.OutboundEmails().GetOutboundEmailByProviderMessageID(adminCtx, providerMessageID)
		if fetchErr != nil {
			return fetchErr
		}

		// Update email status to bounced
		outboundEmail.Status = emaildom.EmailStatusBounced
		outboundEmail.ErrorMessage = fmt.Sprintf("%s bounce: %s", bounceType, description)

		return es.store.OutboundEmails().UpdateOutboundEmail(adminCtx, outboundEmail)
	})
	if err != nil {
		return nil, err
	}
	return outboundEmail, nil
}

// MarkOutboundEmailDelivered updates an outbound email status to delivered by provider message ID.
// Uses admin context to perform cross-tenant lookup.
func (es *EmailService) MarkOutboundEmailDelivered(ctx context.Context, providerMessageID string) (*emaildom.OutboundEmail, error) {
	var outboundEmail *emaildom.OutboundEmail
	err := es.store.WithAdminContext(ctx, func(adminCtx context.Context) error {
		// Find outbound email by provider message ID
		var fetchErr error
		outboundEmail, fetchErr = es.store.OutboundEmails().GetOutboundEmailByProviderMessageID(adminCtx, providerMessageID)
		if fetchErr != nil {
			return fetchErr
		}

		// Update email status to delivered
		outboundEmail.Status = emaildom.EmailStatusDelivered
		now := time.Now()
		outboundEmail.DeliveredAt = &now

		return es.store.OutboundEmails().UpdateOutboundEmail(adminCtx, outboundEmail)
	})
	if err != nil {
		return nil, err
	}
	return outboundEmail, nil
}

// ProcessInboundEmail resolves an inbound email into either a new case or an
// existing case thread, then marks the email as processed.
func (es *EmailService) ProcessInboundEmail(ctx context.Context, emailID string) (*emaildom.InboundEmail, error) {
	if strings.TrimSpace(emailID) == "" {
		return nil, fmt.Errorf("email_id is required")
	}
	if es.store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if es.caseService == nil {
		return nil, fmt.Errorf("case service is required")
	}

	var processed *emaildom.InboundEmail
	var processingTarget *emaildom.InboundEmail
	err := es.store.WithAdminContext(ctx, func(adminCtx context.Context) error {
		email, err := es.store.InboundEmails().GetInboundEmail(adminCtx, emailID)
		if err != nil {
			return fmt.Errorf("load inbound email: %w", err)
		}
		processingTarget = email
		if err := es.store.SetTenantContext(adminCtx, email.WorkspaceID); err != nil {
			return fmt.Errorf("set tenant context: %w", err)
		}

		if email.ProcessingStatus == emaildom.EmailProcessingStatusProcessed {
			processed = email
			return nil
		}

		if err := es.processInboundEmailInTransaction(adminCtx, email); err != nil {
			return err
		}

		processed = email
		return nil
	})
	if err != nil {
		if processingTarget != nil {
			es.persistInboundEmailFailure(ctx, processingTarget, err)
		}
		return nil, err
	}
	return processed, nil
}

func (es *EmailService) processInboundEmailInTransaction(ctx context.Context, email *emaildom.InboundEmail) error {
	if email == nil {
		return fmt.Errorf("email is required")
	}

	matchedCase, err := es.matchInboundEmailToCase(ctx, email)
	if err != nil {
		return err
	}

	var comm *emaildom.Communication
	var caseObj *emaildom.Case
	if matchedCase != nil {
		comm, caseObj, err = es.caseService.AddInboundEmailToCase(ctx, matchedCase.ID, email)
		email.IsThreadStart = false
	} else {
		caseObj, comm, err = es.caseService.CreateCaseFromInboundEmail(ctx, email)
		email.IsThreadStart = true
	}
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	email.CaseID = caseObj.ID
	email.CommunicationID = comm.ID
	email.ProcessedAt = &now
	email.ProcessingStatus = emaildom.EmailProcessingStatusProcessed
	email.ProcessingError = ""
	email.MarkUpdated()

	if err := es.store.InboundEmails().UpdateInboundEmail(ctx, email); err != nil {
		return fmt.Errorf("update inbound email: %w", err)
	}

	return nil
}

func (es *EmailService) matchInboundEmailToCase(ctx context.Context, email *emaildom.InboundEmail) (*emaildom.Case, error) {
	if email == nil {
		return nil, fmt.Errorf("email is required")
	}

	for _, candidateMessageID := range inboundEmailReferenceCandidates(email) {
		cases, err := es.store.Cases().ListCasesByMessageID(ctx, email.WorkspaceID, candidateMessageID)
		if err != nil {
			return nil, fmt.Errorf("match case by message id: %w", err)
		}
		if matched := pickMostRecentlyUpdatedCase(cases); matched != nil {
			return matched, nil
		}
	}

	if !subjectLooksLikeReply(email.Subject) {
		return nil, nil
	}

	normalizedSubject := normalizeInboundSubject(email.Subject)
	if normalizedSubject == "" {
		return nil, nil
	}

	cases, err := es.store.Cases().ListCasesBySubject(ctx, email.WorkspaceID, normalizedSubject)
	if err != nil {
		return nil, fmt.Errorf("match case by subject: %w", err)
	}
	return pickMatchingCaseByContact(cases, email.FromEmail), nil
}

func (es *EmailService) persistInboundEmailFailure(ctx context.Context, email *emaildom.InboundEmail, cause error) {
	if email == nil {
		return
	}
	if err := es.store.WithAdminContext(ctx, func(adminCtx context.Context) error {
		if err := es.store.SetTenantContext(adminCtx, email.WorkspaceID); err != nil {
			return fmt.Errorf("set tenant context: %w", err)
		}
		email.ProcessingStatus = emaildom.EmailProcessingStatusFailed
		if cause != nil {
			email.ProcessingError = cause.Error()
		}
		email.MarkUpdated()
		return es.store.InboundEmails().UpdateInboundEmail(adminCtx, email)
	}); err != nil {
		es.logger.WithError(err).Warn("Failed to persist inbound email failure state", "email_id", email.ID)
	}
}

func inboundEmailReferenceCandidates(email *emaildom.InboundEmail) []string {
	if email == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var candidates []string
	push := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	push(email.InReplyTo)
	for _, ref := range email.References {
		push(ref)
	}
	return candidates
}

func normalizeInboundSubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return ""
	}

	for {
		parts := strings.SplitN(subject, ":", 2)
		if len(parts) != 2 {
			return strings.TrimSpace(subject)
		}
		prefix := strings.ToLower(strings.TrimSpace(parts[0]))
		switch prefix {
		case "re", "fw", "fwd":
			subject = strings.TrimSpace(parts[1])
		default:
			return strings.TrimSpace(subject)
		}
	}
}

func subjectLooksLikeReply(subject string) bool {
	subject = strings.TrimSpace(strings.ToLower(subject))
	return strings.HasPrefix(subject, "re:") || strings.HasPrefix(subject, "fw:") || strings.HasPrefix(subject, "fwd:")
}

func pickMatchingCaseByContact(cases []*emaildom.Case, fromEmail string) *emaildom.Case {
	fromEmail = strings.TrimSpace(strings.ToLower(fromEmail))
	if fromEmail == "" {
		return pickMostRecentlyUpdatedCase(cases)
	}

	var filtered []*emaildom.Case
	for _, caseObj := range cases {
		if strings.EqualFold(strings.TrimSpace(caseObj.ContactEmail), fromEmail) {
			filtered = append(filtered, caseObj)
		}
	}
	if len(filtered) > 0 {
		return pickMostRecentlyUpdatedCase(filtered)
	}
	return nil
}

func pickMostRecentlyUpdatedCase(cases []*emaildom.Case) *emaildom.Case {
	var best *emaildom.Case
	for _, caseObj := range cases {
		if caseObj == nil {
			continue
		}
		if best == nil || caseObj.UpdatedAt.After(best.UpdatedAt) {
			best = caseObj
		}
	}
	return best
}
