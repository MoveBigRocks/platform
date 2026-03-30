package container

import (
	"fmt"

	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	knowledgeservices "github.com/movebigrocks/platform/internal/knowledge/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/outbox"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	"github.com/movebigrocks/platform/pkg/logger"
)

// ServiceContainer holds the service-domain services.
// Services in this domain depend on infrastructure only.
type ServiceContainer struct {
	Artifacts    *artifactservices.GitService
	Concepts     *knowledgeservices.ConceptSpecService
	Queue        *serviceapp.QueueService
	Conversation *serviceapp.ConversationService
	Catalog      *serviceapp.ServiceCatalogService
	FormSpecs    *serviceapp.FormSpecService
	Knowledge    *knowledgeservices.KnowledgeService
	Case         *serviceapp.CaseService
	Email        *serviceapp.EmailService
	Notification *serviceapp.NotificationService
	Attachment   *serviceapp.AttachmentService
}

// ServiceContainerDeps contains the dependencies for creating a ServiceContainer.
type ServiceContainerDeps struct {
	Store  stores.Store
	Outbox *outbox.Service
	Config *config.Config
	Logger *logger.Logger
}

// NewServiceContainer creates a new service container with all service-domain services.
// Returns an error if required services fail to initialize.
func NewServiceContainer(deps ServiceContainerDeps) (*ServiceContainer, error) {
	c := &ServiceContainer{}
	c.Artifacts = artifactservices.NewGitService(deps.Config.Storage.Artifacts.Path)

	c.Queue = serviceapp.NewQueueService(
		deps.Store.Queues(),
		deps.Store.QueueItems(),
		deps.Store.Workspaces(),
	)
	c.Catalog = serviceapp.NewServiceCatalogService(
		deps.Store.ServiceCatalog(),
		deps.Store.Workspaces(),
	)
	c.FormSpecs = serviceapp.NewFormSpecService(
		deps.Store.FormSpecs(),
		deps.Store.Workspaces(),
	)
	c.Knowledge = knowledgeservices.NewKnowledgeService(
		deps.Store.KnowledgeResources(),
		deps.Store.Workspaces(),
		deps.Store.ConceptSpecs(),
		c.Artifacts,
		deps.Outbox,
		transactionRunner(deps.Store),
	)
	c.Concepts = knowledgeservices.NewConceptSpecService(
		deps.Store.ConceptSpecs(),
		deps.Store.Workspaces(),
		c.Artifacts,
	)

	// CaseService (dependency for EmailService)
	// Wire TransactionRunner for atomic case + event creation
	c.Case = serviceapp.NewCaseService(
		deps.Store.Queues(),
		deps.Store.Cases(),
		deps.Store.Workspaces(),
		deps.Outbox,
		serviceapp.WithQueueItemStore(deps.Store.QueueItems()),
		serviceapp.WithOutboundEmailStore(deps.Store.OutboundEmails()),
		serviceapp.WithTransactionRunner(deps.Store),
		serviceapp.WithUserStore(deps.Store.Users()),
	)

	c.Conversation = serviceapp.NewConversationService(
		deps.Store.Conversations(),
		deps.Store.Queues(),
		deps.Store.QueueItems(),
		deps.Store.Workspaces(),
		c.Case,
		transactionRunner(deps.Store),
	)

	// EmailService
	emailService, err := serviceapp.NewEmailService(
		deps.Store,
		serviceapp.EmailConfig{
			Provider:             deps.Config.Email.Backend,
			SendGridAPIKey:       deps.Config.Email.SendGridAPIKey,
			PostmarkServerToken:  deps.Config.Email.PostmarkServerToken,
			PostmarkAccountToken: deps.Config.Email.PostmarkAccountToken,
			SESRegion:            deps.Config.Email.SESRegion,
			SESAccessKey:         deps.Config.Email.SESAccessKey,
			SESSecretKey:         deps.Config.Email.SESSecretKey,
			SMTPHost:             deps.Config.Email.SMTPHost,
			SMTPPort:             deps.Config.Email.SMTPPort,
			SMTPUsername:         deps.Config.Email.SMTPUsername,
			SMTPPassword:         deps.Config.Email.SMTPPassword,
			DefaultFromEmail:     deps.Config.Email.FromEmail,
			DefaultFromName:      deps.Config.Email.FromName,
			MaxRetries:           deps.Config.Email.MaxRetries,
			RetryDelay:           deps.Config.Email.RetryDelay,
		},
		c.Case,
	)
	if err != nil {
		return nil, fmt.Errorf("email service: %w", err)
	}
	c.Email = emailService
	c.Notification = serviceapp.NewNotificationService(deps.Store, c.Email, deps.Logger)

	// AttachmentService (optional, requires S3 config)
	if deps.Config.Storage.Attachments.Bucket != "" {
		attachmentService, err := serviceapp.NewAttachmentService(serviceapp.AttachmentServiceConfig{
			S3Endpoint:    deps.Config.Storage.Operational.Endpoint,
			S3Region:      deps.Config.Storage.Operational.Region,
			S3Bucket:      deps.Config.Storage.Attachments.Bucket,
			S3AccessKey:   deps.Config.Storage.Operational.AccessKey,
			S3SecretKey:   deps.Config.Storage.Operational.SecretKey,
			ClamAVAddr:    deps.Config.Integrations.ClamAVAddr,
			ClamAVTimeout: deps.Config.Integrations.ClamAVTimeout,
			Logger:        deps.Logger,
		})
		if err != nil {
			deps.Logger.WithError(err).Warn("Failed to initialize attachment service")
		} else {
			c.Attachment = attachmentService
			deps.Logger.Info("Attachment service initialized",
				"bucket", deps.Config.Storage.Attachments.Bucket)
		}
	}

	return c, nil
}

func transactionRunner(store stores.Store) contracts.TransactionRunner {
	if store == nil {
		return nil
	}
	return store
}
