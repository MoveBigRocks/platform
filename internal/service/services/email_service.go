package serviceapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores"

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
