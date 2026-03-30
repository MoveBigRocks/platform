package container

import (
	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	sqlstore "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	"github.com/movebigrocks/platform/pkg/logger"
)

// PlatformContainer holds platform domain services.
// Platform services have no dependencies on other domain services.
type PlatformContainer struct {
	Session   *platformservices.SessionService
	CLILogin  *platformservices.CLILoginService
	Stats     *platformservices.AdminStatsService
	Workspace *platformservices.WorkspaceManagementService
	User      *platformservices.UserManagementService
	Agent     *platformservices.AgentService
	Sandbox   *platformservices.SandboxService
	Contact   *platformservices.ContactService
	Extension *platformservices.ExtensionService
}

// NewPlatformContainer creates a new platform container with all platform services.
// Platform services depend only on infrastructure (store, config, logger).
func NewPlatformContainer(
	store stores.Store,
	cfg *config.Config,
	log *logger.Logger,
) *PlatformContainer {
	extensionOptions := []platformservices.ExtensionServiceOption{}
	if sqlBackedStore, ok := store.(*sqlstore.Store); ok {
		extensionOptions = append(extensionOptions, platformservices.WithExtensionSchemaRuntime(sqlBackedStore.ExtensionSchemaMigrator()))
	}
	if verifier, err := platformservices.NewExtensionBundleTrustVerifier(cfg.InstanceID, cfg.ExtensionTrust.RequireVerification, cfg.ExtensionTrust.TrustedPublishers); err == nil {
		extensionOptions = append(extensionOptions, platformservices.WithExtensionBundleVerifier(verifier))
	} else {
		log.Warn("Failed to initialize extension bundle trust verifier", "error", err)
	}
	extensionOptions = append(extensionOptions, platformservices.WithExtensionArtifactService(artifactservices.NewGitService(cfg.Storage.Artifacts.Path)))

	statsService := platformservices.NewAdminStatsService(
		store,
		log,
		cfg.Integrations.PrometheusURL,
	)
	extensionService := platformservices.NewExtensionServiceWithOptions(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
		extensionOptions...,
	)
	statsService.SetExtensionGate(extensionService)

	return &PlatformContainer{
		Session: platformservices.NewSessionService(
			store.Users(),
			store.Workspaces(),
		),
		CLILogin: platformservices.NewCLILoginService(),
		Stats:    statsService,
		Workspace: platformservices.NewWorkspaceManagementService(
			store.Workspaces(),
			store.Cases(),
			store.Users(),
			store.Rules(),
		),
		User: platformservices.NewUserManagementService(
			store.Users(),
			store.Workspaces(),
		),
		Agent: platformservices.NewAgentService(
			store.Agents(),
		),
		Sandbox: platformservices.NewSandboxService(
			store.Sandboxes(),
			platformservices.SandboxServiceConfig{
				PublicBaseURL: cfg.Server.BaseURL,
				RuntimeDomain: "movebigrocks.io",
			},
		),
		Contact: platformservices.NewContactService(
			store.Contacts(),
		),
		Extension: extensionService,
	}
}

// Close stops any background goroutines in platform services.
func (p *PlatformContainer) Close() {
	if p.Session != nil {
		p.Session.Close()
	}
	if p.CLILogin != nil {
		p.CLILogin.Close()
	}
}
