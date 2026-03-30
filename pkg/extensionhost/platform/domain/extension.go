package platformdomain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

type ExtensionStatus string

const (
	ExtensionStatusInstalled ExtensionStatus = "installed"
	ExtensionStatusActive    ExtensionStatus = "active"
	ExtensionStatusInactive  ExtensionStatus = "inactive"
	ExtensionStatusFailed    ExtensionStatus = "failed"
)

type ExtensionValidationStatus string

const (
	ExtensionValidationUnknown ExtensionValidationStatus = "unknown"
	ExtensionValidationValid   ExtensionValidationStatus = "valid"
	ExtensionValidationInvalid ExtensionValidationStatus = "invalid"
)

type ExtensionHealthStatus string

const (
	ExtensionHealthUnknown  ExtensionHealthStatus = "unknown"
	ExtensionHealthHealthy  ExtensionHealthStatus = "healthy"
	ExtensionHealthDegraded ExtensionHealthStatus = "degraded"
	ExtensionHealthFailed   ExtensionHealthStatus = "failed"
	ExtensionHealthInactive ExtensionHealthStatus = "inactive"
)

type ExtensionAssetKind string

const (
	ExtensionAssetKindTemplate ExtensionAssetKind = "template"
	ExtensionAssetKindStatic   ExtensionAssetKind = "static"
	ExtensionAssetKindOther    ExtensionAssetKind = "other"
)

type ExtensionKind string

const (
	ExtensionKindProduct     ExtensionKind = "product"
	ExtensionKindIdentity    ExtensionKind = "identity"
	ExtensionKindConnector   ExtensionKind = "connector"
	ExtensionKindOperational ExtensionKind = "operational"
)

type ExtensionScope string

const (
	ExtensionScopeWorkspace ExtensionScope = "workspace"
	ExtensionScopeInstance  ExtensionScope = "instance"
)

type ExtensionRisk string

const (
	ExtensionRiskStandard   ExtensionRisk = "standard"
	ExtensionRiskPrivileged ExtensionRisk = "privileged"
)

type ExtensionRuntimeClass string

const (
	ExtensionRuntimeClassBundle        ExtensionRuntimeClass = "bundle"
	ExtensionRuntimeClassServiceBacked ExtensionRuntimeClass = "service_backed"
)

const (
	ExtensionRuntimeProtocolInProcessHTTP  = "in_process_http"
	ExtensionRuntimeProtocolUnixSocketHTTP = "unix_socket_http"
)

type ExtensionStorageClass string

const (
	ExtensionStorageClassSharedPrimitivesOnly ExtensionStorageClass = "shared_primitives_only"
	ExtensionStorageClassOwnedSchema          ExtensionStorageClass = "owned_schema"
)

type ExtensionEndpointClass string

const (
	ExtensionEndpointClassPublicPage   ExtensionEndpointClass = "public_page"
	ExtensionEndpointClassPublicAsset  ExtensionEndpointClass = "public_asset"
	ExtensionEndpointClassPublicIngest ExtensionEndpointClass = "public_ingest"
	ExtensionEndpointClassWebhook      ExtensionEndpointClass = "webhook"
	ExtensionEndpointClassAdminPage    ExtensionEndpointClass = "admin_page"
	ExtensionEndpointClassAdminAction  ExtensionEndpointClass = "admin_action"
	ExtensionEndpointClassExtensionAPI ExtensionEndpointClass = "extension_api"
	ExtensionEndpointClassHealth       ExtensionEndpointClass = "health"
)

type ExtensionEndpointAuth string

const (
	ExtensionEndpointAuthPublic         ExtensionEndpointAuth = "public"
	ExtensionEndpointAuthSignedWebhook  ExtensionEndpointAuth = "signed_webhook"
	ExtensionEndpointAuthSession        ExtensionEndpointAuth = "session"
	ExtensionEndpointAuthAgentToken     ExtensionEndpointAuth = "agent_token"
	ExtensionEndpointAuthExtensionToken ExtensionEndpointAuth = "extension_token"
	ExtensionEndpointAuthInternalOnly   ExtensionEndpointAuth = "internal_only"
)

type ExtensionWorkspaceBinding string

const (
	ExtensionWorkspaceBindingNone           ExtensionWorkspaceBinding = "none"
	ExtensionWorkspaceBindingFromSession    ExtensionWorkspaceBinding = "workspace_from_session"
	ExtensionWorkspaceBindingFromAgentToken ExtensionWorkspaceBinding = "workspace_from_agent_token"
	ExtensionWorkspaceBindingFromRoute      ExtensionWorkspaceBinding = "workspace_from_route"
	ExtensionWorkspaceBindingInstanceScoped ExtensionWorkspaceBinding = "instance_scoped"
)

type ExtensionWorkspaceInstallMode string

const (
	ExtensionWorkspaceInstallIntoExisting ExtensionWorkspaceInstallMode = "install_into_existing_workspace"
	ExtensionWorkspaceProvisionDedicated  ExtensionWorkspaceInstallMode = "provision_dedicated_workspace"
)

type ExtensionManifest struct {
	SchemaVersion      int
	Slug               string
	Name               string
	Version            string
	Publisher          string
	Kind               ExtensionKind
	Scope              ExtensionScope
	Risk               ExtensionRisk
	RuntimeClass       ExtensionRuntimeClass
	StorageClass       ExtensionStorageClass
	Schema             ExtensionSchemaManifest
	Runtime            ExtensionRuntimeSpec
	Description        string
	WorkspacePlan      ExtensionWorkspacePlan
	Permissions        []string
	Queues             []ExtensionQueueSeed
	Forms              []ExtensionFormSeed
	AutomationRules    []ExtensionAutomationSeed
	ArtifactSurfaces   []ExtensionArtifactSurface
	PublicRoutes       []ExtensionRoute
	AdminRoutes        []ExtensionRoute
	Endpoints          []ExtensionEndpoint
	AdminNavigation    []ExtensionAdminNavigationItem
	DashboardWidgets   []ExtensionDashboardWidget
	Events             ExtensionEventCatalog
	EventConsumers     []ExtensionEventConsumer
	ScheduledJobs      []ExtensionScheduledJob
	Commands           []ExtensionCommand
	AgentSkills        []ExtensionAgentSkill
	CustomizableAssets []string
	DefaultConfig      shareddomain.TypedCustomFields
}

type ExtensionSchemaManifest struct {
	Name            string
	PackageKey      string
	TargetVersion   string
	MigrationEngine string
}

type ExtensionRuntimeSpec struct {
	Protocol     string
	OCIReference string
	Digest       string
}

type ExtensionWorkspacePlan struct {
	Mode        ExtensionWorkspaceInstallMode
	Name        string
	Slug        string
	Description string
}

type ExtensionQueueSeed struct {
	Slug        string
	Name        string
	Description string
}

type ExtensionFormSeed struct {
	Slug              string
	Name              string
	Description       string
	Status            string
	IsPublic          bool
	RequiresAuth      bool
	AllowMultiple     bool
	CollectEmail      bool
	AutoCreateCase    bool
	AutoCasePriority  string
	AutoCaseType      string
	AutoTags          []string
	SubmissionMessage string
	RedirectURL       string
	Theme             string
	Schema            shareddomain.TypedSchema
	UISchema          shareddomain.TypedSchema
	ValidationRules   shareddomain.TypedSchema
}

type ExtensionAutomationSeed struct {
	Key                  string
	Title                string
	Description          string
	IsActive             bool
	Priority             int
	MaxExecutionsPerHour int
	MaxExecutionsPerDay  int
	Conditions           shareddomain.TypedSchema
	Actions              shareddomain.TypedSchema
}

type ExtensionArtifactSurface struct {
	Name          string
	Description   string
	SeedAssetPath string
}

type ExtensionRoute struct {
	PathPrefix      string
	AssetPath       string
	ArtifactSurface string
	ArtifactPath    string
}

type ExtensionEndpoint struct {
	Name             string
	Class            ExtensionEndpointClass
	MountPath        string
	Methods          []string
	Auth             ExtensionEndpointAuth
	ContentTypes     []string
	MaxBodyBytes     int64
	RateLimitPerMin  int
	WorkspaceBinding ExtensionWorkspaceBinding
	AssetPath        string
	ArtifactSurface  string
	ArtifactPath     string
	ServiceTarget    string
}

type ExtensionAdminNavigationItem struct {
	Name       string
	Section    string
	Title      string
	Icon       string
	Endpoint   string
	ActivePage string
}

type ExtensionDashboardWidget struct {
	Name        string
	Title       string
	Description string
	Icon        string
	Endpoint    string
}

type ExtensionEventConsumer struct {
	Name          string
	Description   string
	Stream        string
	EventTypes    []string
	ConsumerGroup string
	ServiceTarget string
}

type ExtensionScheduledJob struct {
	Name            string
	Description     string
	IntervalSeconds int
	ServiceTarget   string
}

type ExtensionCommand struct {
	Name        string
	Description string
}

type ExtensionAgentSkill struct {
	Name        string
	Description string
	AssetPath   string
}

type ExtensionEventCatalog struct {
	Publishes  []ExtensionEventDefinition
	Subscribes []string
}

type ExtensionEventDefinition struct {
	Type          string
	Description   string
	SchemaVersion int
}

type ExtensionRuntimeEvent struct {
	Type          string
	Description   string
	SchemaVersion int
	Core          bool
	Publishers    []string
	Subscribers   []string
}

func (m *ExtensionManifest) Normalize() {
	m.SchemaVersion = max(m.SchemaVersion, 1)
	m.Slug = normalizeExtensionSlug(m.Slug)
	m.Name = strings.TrimSpace(m.Name)
	m.Version = strings.TrimSpace(m.Version)
	m.Publisher = strings.TrimSpace(m.Publisher)
	m.Kind = normalizeExtensionKind(m.Kind)
	m.Scope = normalizeExtensionScope(m.Scope)
	m.Risk = normalizeExtensionRisk(m.Risk)
	m.RuntimeClass = normalizeRuntimeClass(m.RuntimeClass)
	m.StorageClass = normalizeStorageClass(m.StorageClass, m.RuntimeClass)
	m.Schema.Name = normalizeExtensionSchemaName(m.Schema.Name)
	m.Schema.PackageKey = normalizeExtensionPackageKey(m.Schema.PackageKey)
	m.Schema.TargetVersion = strings.TrimSpace(m.Schema.TargetVersion)
	m.Schema.MigrationEngine = strings.TrimSpace(strings.ToLower(m.Schema.MigrationEngine))
	m.Runtime.Protocol = normalizeRuntimeProtocol(m.Runtime.Protocol)
	m.Runtime.OCIReference = strings.TrimSpace(m.Runtime.OCIReference)
	m.Runtime.Digest = strings.TrimSpace(m.Runtime.Digest)
	m.Description = strings.TrimSpace(m.Description)
	m.WorkspacePlan.Mode = normalizeWorkspaceInstallMode(m.WorkspacePlan.Mode)
	m.WorkspacePlan.Name = strings.TrimSpace(m.WorkspacePlan.Name)
	m.WorkspacePlan.Slug = normalizeExtensionSlug(m.WorkspacePlan.Slug)
	m.WorkspacePlan.Description = strings.TrimSpace(m.WorkspacePlan.Description)

	m.Permissions = normalizeStringList(m.Permissions)
	m.CustomizableAssets = normalizePathList(m.CustomizableAssets)

	for i := range m.Queues {
		m.Queues[i].Slug = normalizeExtensionSlug(m.Queues[i].Slug)
		m.Queues[i].Name = strings.TrimSpace(m.Queues[i].Name)
		m.Queues[i].Description = strings.TrimSpace(m.Queues[i].Description)
	}
	for i := range m.Forms {
		m.Forms[i].Slug = normalizeExtensionSlug(m.Forms[i].Slug)
		m.Forms[i].Name = strings.TrimSpace(m.Forms[i].Name)
		m.Forms[i].Description = strings.TrimSpace(m.Forms[i].Description)
		m.Forms[i].Status = strings.TrimSpace(strings.ToLower(m.Forms[i].Status))
		m.Forms[i].AutoCasePriority = strings.TrimSpace(strings.ToLower(m.Forms[i].AutoCasePriority))
		m.Forms[i].AutoCaseType = strings.TrimSpace(strings.ToLower(m.Forms[i].AutoCaseType))
		m.Forms[i].AutoTags = normalizeStringList(m.Forms[i].AutoTags)
		m.Forms[i].SubmissionMessage = strings.TrimSpace(m.Forms[i].SubmissionMessage)
		m.Forms[i].RedirectURL = strings.TrimSpace(m.Forms[i].RedirectURL)
		m.Forms[i].Theme = strings.TrimSpace(m.Forms[i].Theme)
	}
	for i := range m.AutomationRules {
		m.AutomationRules[i].Key = normalizeExtensionRuleKey(m.AutomationRules[i].Key)
		m.AutomationRules[i].Title = strings.TrimSpace(m.AutomationRules[i].Title)
		m.AutomationRules[i].Description = strings.TrimSpace(m.AutomationRules[i].Description)
	}
	for i := range m.ArtifactSurfaces {
		m.ArtifactSurfaces[i].Name = normalizeExtensionArtifactSurfaceName(m.ArtifactSurfaces[i].Name)
		m.ArtifactSurfaces[i].Description = strings.TrimSpace(m.ArtifactSurfaces[i].Description)
		m.ArtifactSurfaces[i].SeedAssetPath = normalizeAssetPath(m.ArtifactSurfaces[i].SeedAssetPath)
	}
	for i := range m.PublicRoutes {
		m.PublicRoutes[i].PathPrefix = normalizeRoutePrefix(m.PublicRoutes[i].PathPrefix)
		m.PublicRoutes[i].AssetPath = normalizeAssetPath(m.PublicRoutes[i].AssetPath)
		m.PublicRoutes[i].ArtifactSurface = normalizeExtensionArtifactSurfaceName(m.PublicRoutes[i].ArtifactSurface)
		m.PublicRoutes[i].ArtifactPath = normalizeAssetPath(m.PublicRoutes[i].ArtifactPath)
	}
	for i := range m.AdminRoutes {
		m.AdminRoutes[i].PathPrefix = normalizeRoutePrefix(m.AdminRoutes[i].PathPrefix)
		m.AdminRoutes[i].AssetPath = normalizeAssetPath(m.AdminRoutes[i].AssetPath)
		m.AdminRoutes[i].ArtifactSurface = normalizeExtensionArtifactSurfaceName(m.AdminRoutes[i].ArtifactSurface)
		m.AdminRoutes[i].ArtifactPath = normalizeAssetPath(m.AdminRoutes[i].ArtifactPath)
	}
	for i := range m.Endpoints {
		m.Endpoints[i].Name = strings.TrimSpace(m.Endpoints[i].Name)
		m.Endpoints[i].Class = normalizeEndpointClass(m.Endpoints[i].Class)
		m.Endpoints[i].MountPath = normalizeRoutePrefix(m.Endpoints[i].MountPath)
		m.Endpoints[i].Methods = normalizeMethodList(m.Endpoints[i].Methods)
		m.Endpoints[i].Auth = normalizeEndpointAuth(m.Endpoints[i].Auth, m.Endpoints[i].Class)
		m.Endpoints[i].ContentTypes = normalizeStringList(m.Endpoints[i].ContentTypes)
		m.Endpoints[i].WorkspaceBinding = normalizeWorkspaceBinding(m.Endpoints[i].WorkspaceBinding, m.Endpoints[i].Class)
		m.Endpoints[i].AssetPath = normalizeAssetPath(m.Endpoints[i].AssetPath)
		m.Endpoints[i].ArtifactSurface = normalizeExtensionArtifactSurfaceName(m.Endpoints[i].ArtifactSurface)
		m.Endpoints[i].ArtifactPath = normalizeAssetPath(m.Endpoints[i].ArtifactPath)
		m.Endpoints[i].ServiceTarget = strings.TrimSpace(m.Endpoints[i].ServiceTarget)
	}
	for i := range m.AdminNavigation {
		m.AdminNavigation[i].Name = normalizeExtensionNavigationName(m.AdminNavigation[i].Name)
		m.AdminNavigation[i].Section = strings.TrimSpace(m.AdminNavigation[i].Section)
		m.AdminNavigation[i].Title = strings.TrimSpace(m.AdminNavigation[i].Title)
		m.AdminNavigation[i].Icon = strings.TrimSpace(m.AdminNavigation[i].Icon)
		m.AdminNavigation[i].Endpoint = strings.TrimSpace(m.AdminNavigation[i].Endpoint)
		m.AdminNavigation[i].ActivePage = normalizeExtensionNavigationName(m.AdminNavigation[i].ActivePage)
		if m.AdminNavigation[i].Name == "" {
			m.AdminNavigation[i].Name = normalizeExtensionNavigationName(m.AdminNavigation[i].Endpoint)
		}
		if m.AdminNavigation[i].Section == "" {
			m.AdminNavigation[i].Section = m.Name
		}
		if m.AdminNavigation[i].ActivePage == "" {
			m.AdminNavigation[i].ActivePage = m.AdminNavigation[i].Name
		}
	}
	for i := range m.DashboardWidgets {
		m.DashboardWidgets[i].Name = normalizeExtensionNavigationName(m.DashboardWidgets[i].Name)
		m.DashboardWidgets[i].Title = strings.TrimSpace(m.DashboardWidgets[i].Title)
		m.DashboardWidgets[i].Description = strings.TrimSpace(m.DashboardWidgets[i].Description)
		m.DashboardWidgets[i].Icon = strings.TrimSpace(m.DashboardWidgets[i].Icon)
		m.DashboardWidgets[i].Endpoint = strings.TrimSpace(m.DashboardWidgets[i].Endpoint)
		if m.DashboardWidgets[i].Name == "" {
			m.DashboardWidgets[i].Name = normalizeExtensionNavigationName(m.DashboardWidgets[i].Endpoint)
		}
	}
	for i := range m.Commands {
		m.Commands[i].Name = strings.TrimSpace(m.Commands[i].Name)
		m.Commands[i].Description = strings.TrimSpace(m.Commands[i].Description)
	}
	for i := range m.AgentSkills {
		m.AgentSkills[i].Name = strings.TrimSpace(m.AgentSkills[i].Name)
		m.AgentSkills[i].Description = strings.TrimSpace(m.AgentSkills[i].Description)
		m.AgentSkills[i].AssetPath = normalizeAssetPath(m.AgentSkills[i].AssetPath)
	}
	for i := range m.Events.Publishes {
		m.Events.Publishes[i].Type = normalizeExtensionEventType(m.Events.Publishes[i].Type)
		m.Events.Publishes[i].Description = strings.TrimSpace(m.Events.Publishes[i].Description)
		m.Events.Publishes[i].SchemaVersion = max(m.Events.Publishes[i].SchemaVersion, 1)
	}
	m.Events.Subscribes = normalizeEventTypeList(m.Events.Subscribes)
	for i := range m.EventConsumers {
		m.EventConsumers[i].Name = normalizeExtensionNavigationName(m.EventConsumers[i].Name)
		m.EventConsumers[i].Description = strings.TrimSpace(m.EventConsumers[i].Description)
		m.EventConsumers[i].Stream = normalizeExtensionEventStream(m.EventConsumers[i].Stream)
		m.EventConsumers[i].EventTypes = normalizeEventTypeList(m.EventConsumers[i].EventTypes)
		m.EventConsumers[i].ConsumerGroup = normalizeExtensionConsumerGroup(m.EventConsumers[i].ConsumerGroup)
		m.EventConsumers[i].ServiceTarget = strings.TrimSpace(m.EventConsumers[i].ServiceTarget)
		if m.EventConsumers[i].Name == "" {
			m.EventConsumers[i].Name = normalizeExtensionNavigationName(m.EventConsumers[i].ServiceTarget)
		}
		if m.EventConsumers[i].ConsumerGroup == "" {
			m.EventConsumers[i].ConsumerGroup = normalizeExtensionConsumerGroup(m.EventConsumers[i].Name)
		}
	}
	for i := range m.ScheduledJobs {
		m.ScheduledJobs[i].Name = normalizeExtensionNavigationName(m.ScheduledJobs[i].Name)
		m.ScheduledJobs[i].Description = strings.TrimSpace(m.ScheduledJobs[i].Description)
		m.ScheduledJobs[i].ServiceTarget = strings.TrimSpace(m.ScheduledJobs[i].ServiceTarget)
		if m.ScheduledJobs[i].Name == "" {
			m.ScheduledJobs[i].Name = normalizeExtensionNavigationName(m.ScheduledJobs[i].ServiceTarget)
		}
	}
}

func (m *ExtensionManifest) Validate() error {
	m.Normalize()

	var problems []string
	if m.Slug == "" {
		problems = append(problems, "slug is required")
	}
	if m.Name == "" {
		problems = append(problems, "name is required")
	}
	if m.Version == "" {
		problems = append(problems, "version is required")
	}
	if m.Publisher == "" {
		problems = append(problems, "publisher is required")
	}
	if m.Kind == "" {
		problems = append(problems, "kind is required")
	}
	if m.Scope == "" {
		problems = append(problems, "scope is required")
	}
	if m.Risk == "" {
		problems = append(problems, "risk is required")
	}
	if m.RuntimeClass == "" {
		problems = append(problems, "runtimeClass is required")
	}
	if m.StorageClass == "" {
		problems = append(problems, "storageClass is required")
	}
	if m.RuntimeClass == ExtensionRuntimeClassBundle && m.StorageClass == ExtensionStorageClassOwnedSchema {
		problems = append(problems, "bundle extensions cannot use owned_schema storage")
	}
	if m.StorageClass == ExtensionStorageClassOwnedSchema && m.RuntimeClass != ExtensionRuntimeClassServiceBacked {
		problems = append(problems, "owned_schema storage requires service_backed runtime")
	}
	if m.RuntimeClass == ExtensionRuntimeClassServiceBacked {
		if m.Schema.Name == "" {
			problems = append(problems, "schema.name is required for service-backed extensions")
		}
		if m.Schema.PackageKey == "" {
			problems = append(problems, "schema.packageKey is required for service-backed extensions")
		}
		if expected := m.PackageKey(); expected != "" && m.Schema.PackageKey != expected {
			problems = append(problems, fmt.Sprintf("schema.packageKey must match manifest publisher/slug (%s)", expected))
		}
		if m.Schema.TargetVersion == "" {
			problems = append(problems, "schema.targetVersion is required for service-backed extensions")
		}
		if m.Schema.MigrationEngine != "postgres_sql" {
			problems = append(problems, "schema.migrationEngine must be postgres_sql for service-backed extensions")
		}
		if m.Runtime.Protocol == "" {
			problems = append(problems, "runtime.protocol is required for service-backed extensions")
		}
		if m.Runtime.Protocol != "" &&
			m.Runtime.Protocol != ExtensionRuntimeProtocolInProcessHTTP &&
			m.Runtime.Protocol != ExtensionRuntimeProtocolUnixSocketHTTP {
			problems = append(problems, fmt.Sprintf("runtime.protocol %q is not supported", m.Runtime.Protocol))
		}
		if m.Runtime.Protocol == ExtensionRuntimeProtocolUnixSocketHTTP {
			if m.Runtime.OCIReference == "" {
				problems = append(problems, "runtime.ociReference is required for unix_socket_http service-backed extensions")
			}
			if m.Runtime.Digest == "" {
				problems = append(problems, "runtime.digest is required for unix_socket_http service-backed extensions")
			}
		}
		if !manifestHasHealthEndpoint(m.Endpoints) {
			problems = append(problems, "service-backed extensions must declare at least one internal health endpoint")
		}
	}
	if m.WorkspacePlan.Mode == ExtensionWorkspaceProvisionDedicated {
		if m.WorkspacePlan.Name == "" {
			problems = append(problems, "workspacePlan.name is required when provisioning a dedicated workspace")
		}
		if m.WorkspacePlan.Slug == "" {
			problems = append(problems, "workspacePlan.slug is required when provisioning a dedicated workspace")
		}
	}
	artifactSurfaces := map[string]struct{}{}
	for _, surface := range m.ArtifactSurfaces {
		if surface.Name == "" {
			problems = append(problems, "artifact surface name is required")
			continue
		}
		if _, exists := artifactSurfaces[surface.Name]; exists {
			problems = append(problems, fmt.Sprintf("duplicate artifact surface %s", surface.Name))
		} else {
			artifactSurfaces[surface.Name] = struct{}{}
		}
	}
	for _, route := range append([]ExtensionRoute{}, m.PublicRoutes...) {
		if route.PathPrefix == "" {
			problems = append(problems, "route pathPrefix is required")
		}
		if route.AssetPath != "" && route.ArtifactSurface != "" {
			problems = append(problems, fmt.Sprintf("route %s cannot declare both assetPath and artifactSurface", route.PathPrefix))
		}
		if route.AssetPath == "" && route.ArtifactSurface == "" {
			problems = append(problems, "route assetPath or artifactSurface is required")
		}
		if route.ArtifactSurface != "" {
			if _, exists := artifactSurfaces[route.ArtifactSurface]; !exists {
				problems = append(problems, fmt.Sprintf("route %s references unknown artifact surface %s", route.PathPrefix, route.ArtifactSurface))
			}
			if route.ArtifactPath == "" {
				problems = append(problems, fmt.Sprintf("route %s artifactPath is required when artifactSurface is declared", route.PathPrefix))
			}
		}
	}
	for _, route := range m.AdminRoutes {
		if route.PathPrefix == "" {
			problems = append(problems, "route pathPrefix is required")
		}
		if route.AssetPath != "" && route.ArtifactSurface != "" {
			problems = append(problems, fmt.Sprintf("route %s cannot declare both assetPath and artifactSurface", route.PathPrefix))
		}
		if route.AssetPath == "" && route.ArtifactSurface == "" {
			problems = append(problems, "route assetPath or artifactSurface is required")
		}
		if route.ArtifactSurface != "" {
			if _, exists := artifactSurfaces[route.ArtifactSurface]; !exists {
				problems = append(problems, fmt.Sprintf("route %s references unknown artifact surface %s", route.PathPrefix, route.ArtifactSurface))
			}
			if route.ArtifactPath == "" {
				problems = append(problems, fmt.Sprintf("route %s artifactPath is required when artifactSurface is declared", route.PathPrefix))
			}
		}
	}
	seenEndpoints := map[string]struct{}{}
	adminPageEndpoints := map[string]struct{}{}
	for _, endpoint := range m.Endpoints {
		if endpoint.Name == "" {
			problems = append(problems, "endpoint name is required")
		} else if _, exists := seenEndpoints[endpoint.Name]; exists {
			problems = append(problems, fmt.Sprintf("duplicate endpoint name %s", endpoint.Name))
		} else {
			seenEndpoints[endpoint.Name] = struct{}{}
		}
		if endpoint.Class == "" {
			problems = append(problems, "endpoint class is required")
		}
		if endpoint.MountPath == "" {
			problems = append(problems, fmt.Sprintf("endpoint %s mountPath is required", endpoint.Name))
		}
		if endpoint.Auth == "" {
			problems = append(problems, fmt.Sprintf("endpoint %s auth is required", endpoint.Name))
		}
		if endpoint.WorkspaceBinding == "" {
			problems = append(problems, fmt.Sprintf("endpoint %s workspaceBinding is required", endpoint.Name))
		}
		if endpoint.AssetPath != "" && endpoint.ArtifactSurface != "" {
			problems = append(problems, fmt.Sprintf("endpoint %s cannot declare both assetPath and artifactSurface", endpoint.Name))
		}
		if endpoint.ServiceTarget != "" && endpoint.ArtifactSurface != "" {
			problems = append(problems, fmt.Sprintf("endpoint %s cannot declare both artifactSurface and serviceTarget", endpoint.Name))
		}
		if endpoint.AssetPath != "" && endpoint.ServiceTarget != "" {
			problems = append(problems, fmt.Sprintf("endpoint %s cannot declare both assetPath and serviceTarget", endpoint.Name))
		}
		switch endpoint.Class {
		case ExtensionEndpointClassPublicPage, ExtensionEndpointClassPublicAsset, ExtensionEndpointClassAdminPage:
			if endpoint.AssetPath == "" && endpoint.ServiceTarget == "" && endpoint.ArtifactSurface == "" {
				problems = append(problems, fmt.Sprintf("endpoint %s requires assetPath, artifactSurface, or serviceTarget", endpoint.Name))
			}
		case ExtensionEndpointClassPublicIngest, ExtensionEndpointClassWebhook, ExtensionEndpointClassAdminAction, ExtensionEndpointClassExtensionAPI, ExtensionEndpointClassHealth:
			if endpoint.ServiceTarget == "" {
				problems = append(problems, fmt.Sprintf("endpoint %s requires serviceTarget", endpoint.Name))
			}
		}
		if endpoint.Class == ExtensionEndpointClassHealth && endpoint.Auth != ExtensionEndpointAuthInternalOnly {
			problems = append(problems, fmt.Sprintf("endpoint %s health endpoints must be internal_only", endpoint.Name))
		}
		if endpoint.ArtifactSurface != "" {
			if _, exists := artifactSurfaces[endpoint.ArtifactSurface]; !exists {
				problems = append(problems, fmt.Sprintf("endpoint %s references unknown artifact surface %s", endpoint.Name, endpoint.ArtifactSurface))
			}
			if endpoint.ArtifactPath == "" {
				problems = append(problems, fmt.Sprintf("endpoint %s artifactPath is required when artifactSurface is declared", endpoint.Name))
			}
		}
		if endpoint.Class == ExtensionEndpointClassAdminPage && endpoint.Name != "" {
			adminPageEndpoints[endpoint.Name] = struct{}{}
		}
	}
	seenNavigationItems := map[string]struct{}{}
	for _, item := range m.AdminNavigation {
		if item.Name == "" {
			problems = append(problems, "admin navigation item name is required")
			continue
		}
		if _, exists := seenNavigationItems[item.Name]; exists {
			problems = append(problems, fmt.Sprintf("duplicate admin navigation item %s", item.Name))
		} else {
			seenNavigationItems[item.Name] = struct{}{}
		}
		if item.Title == "" {
			problems = append(problems, fmt.Sprintf("admin navigation item %s title is required", item.Name))
		}
		if item.Endpoint == "" {
			problems = append(problems, fmt.Sprintf("admin navigation item %s endpoint is required", item.Name))
			continue
		}
		if _, exists := adminPageEndpoints[item.Endpoint]; !exists {
			problems = append(problems, fmt.Sprintf("admin navigation item %s must reference an admin_page endpoint", item.Name))
		}
	}
	seenWidgets := map[string]struct{}{}
	for _, widget := range m.DashboardWidgets {
		if widget.Name == "" {
			problems = append(problems, "dashboard widget name is required")
			continue
		}
		if _, exists := seenWidgets[widget.Name]; exists {
			problems = append(problems, fmt.Sprintf("duplicate dashboard widget %s", widget.Name))
		} else {
			seenWidgets[widget.Name] = struct{}{}
		}
		if widget.Title == "" {
			problems = append(problems, fmt.Sprintf("dashboard widget %s title is required", widget.Name))
		}
		if widget.Endpoint == "" {
			problems = append(problems, fmt.Sprintf("dashboard widget %s endpoint is required", widget.Name))
			continue
		}
		if _, exists := adminPageEndpoints[widget.Endpoint]; !exists {
			problems = append(problems, fmt.Sprintf("dashboard widget %s must reference an admin_page endpoint", widget.Name))
		}
	}
	for _, seed := range m.Queues {
		if seed.Slug == "" {
			problems = append(problems, "queue slug is required")
		}
		if seed.Name == "" {
			problems = append(problems, "queue name is required")
		}
	}
	seenForms := map[string]struct{}{}
	for _, seed := range m.Forms {
		if seed.Slug == "" {
			problems = append(problems, "form slug is required")
		} else if _, exists := seenForms[seed.Slug]; exists {
			problems = append(problems, fmt.Sprintf("duplicate form slug %s", seed.Slug))
		} else {
			seenForms[seed.Slug] = struct{}{}
		}
		if seed.Name == "" {
			problems = append(problems, fmt.Sprintf("form %s name is required", seed.Slug))
		}
		if seed.IsPublic && seed.RequiresAuth {
			problems = append(problems, fmt.Sprintf("form %s cannot be both public and auth-required", seed.Slug))
		}
	}
	seenRuleKeys := map[string]struct{}{}
	for _, seed := range m.AutomationRules {
		if seed.Key == "" {
			problems = append(problems, "automation rule key is required")
		} else if _, exists := seenRuleKeys[seed.Key]; exists {
			problems = append(problems, fmt.Sprintf("duplicate automation rule key %s", seed.Key))
		} else {
			seenRuleKeys[seed.Key] = struct{}{}
		}
		if seed.Title == "" {
			problems = append(problems, fmt.Sprintf("automation rule %s title is required", seed.Key))
		}
	}
	seenPublishedEvents := map[string]struct{}{}
	for _, event := range m.Events.Publishes {
		if event.Type == "" {
			problems = append(problems, "published event type is required")
			continue
		}
		if !isValidPublishedExtensionEventType(event.Type) {
			problems = append(problems, fmt.Sprintf("published event %s must use ext.<publisher>.<extension>.<event> naming", event.Type))
		}
		if _, exists := seenPublishedEvents[event.Type]; exists {
			problems = append(problems, fmt.Sprintf("duplicate published event %s", event.Type))
		} else {
			seenPublishedEvents[event.Type] = struct{}{}
		}
	}
	seenSubscribedEvents := map[string]struct{}{}
	for _, eventType := range m.Events.Subscribes {
		if eventType == "" {
			problems = append(problems, "subscribed event type is required")
			continue
		}
		if _, exists := seenSubscribedEvents[eventType]; exists {
			problems = append(problems, fmt.Sprintf("duplicate subscribed event %s", eventType))
		} else {
			seenSubscribedEvents[eventType] = struct{}{}
		}
	}
	seenConsumers := map[string]struct{}{}
	for _, consumer := range m.EventConsumers {
		if m.RuntimeClass != ExtensionRuntimeClassServiceBacked {
			problems = append(problems, "event consumers require service_backed runtime")
			break
		}
		if consumer.Name == "" {
			problems = append(problems, "event consumer name is required")
			continue
		}
		if _, exists := seenConsumers[consumer.Name]; exists {
			problems = append(problems, fmt.Sprintf("duplicate event consumer %s", consumer.Name))
		} else {
			seenConsumers[consumer.Name] = struct{}{}
		}
		if consumer.Stream == "" {
			problems = append(problems, fmt.Sprintf("event consumer %s stream is required", consumer.Name))
		}
		if consumer.ServiceTarget == "" {
			problems = append(problems, fmt.Sprintf("event consumer %s serviceTarget is required", consumer.Name))
		}
		for _, eventType := range consumer.EventTypes {
			if _, exists := seenSubscribedEvents[eventType]; !exists {
				problems = append(problems, fmt.Sprintf("event consumer %s must reference an event declared in events.subscribes", consumer.Name))
				break
			}
		}
	}
	seenJobs := map[string]struct{}{}
	for _, job := range m.ScheduledJobs {
		if m.RuntimeClass != ExtensionRuntimeClassServiceBacked {
			problems = append(problems, "scheduled jobs require service_backed runtime")
			break
		}
		if job.Name == "" {
			problems = append(problems, "scheduled job name is required")
			continue
		}
		if _, exists := seenJobs[job.Name]; exists {
			problems = append(problems, fmt.Sprintf("duplicate scheduled job %s", job.Name))
		} else {
			seenJobs[job.Name] = struct{}{}
		}
		if job.IntervalSeconds <= 0 {
			problems = append(problems, fmt.Sprintf("scheduled job %s intervalSeconds must be greater than zero", job.Name))
		}
		if job.ServiceTarget == "" {
			problems = append(problems, fmt.Sprintf("scheduled job %s serviceTarget is required", job.Name))
		}
	}
	seenCommands := map[string]struct{}{}
	for _, command := range m.Commands {
		if command.Name == "" {
			problems = append(problems, "command name is required")
			continue
		}
		if !isExtensionCommandName(command.Name, m.Slug) {
			problems = append(problems, fmt.Sprintf("command %s must be namespaced under %s.", command.Name, m.Slug))
		}
		if _, exists := seenCommands[command.Name]; exists {
			problems = append(problems, fmt.Sprintf("duplicate command %s", command.Name))
		} else {
			seenCommands[command.Name] = struct{}{}
		}
	}
	seenSkills := map[string]struct{}{}
	for _, skill := range m.AgentSkills {
		if skill.Name == "" {
			problems = append(problems, "agent skill name is required")
		}
		if skill.AssetPath == "" {
			problems = append(problems, fmt.Sprintf("agent skill %s assetPath is required", skill.Name))
		}
		if skill.Name != "" {
			if _, exists := seenSkills[skill.Name]; exists {
				problems = append(problems, fmt.Sprintf("duplicate agent skill %s", skill.Name))
			} else {
				seenSkills[skill.Name] = struct{}{}
			}
		}
	}
	for _, assetPath := range m.CustomizableAssets {
		if assetPath == "" {
			problems = append(problems, "customizable asset path is required")
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, ", "))
	}
	return nil
}

func (m ExtensionManifest) PackageKey() string {
	publisher := normalizeExtensionPackageKey(m.Publisher)
	if publisher == "" || m.Slug == "" {
		return ""
	}
	return publisher + "/" + m.Slug
}

func isExtensionCommandName(name, slug string) bool {
	name = strings.TrimSpace(name)
	slug = normalizeExtensionSlug(slug)
	if name == "" || slug == "" {
		return false
	}
	return strings.HasPrefix(name, slug+".") && len(name) > len(slug)+1
}

type InstalledExtension struct {
	ID          string
	WorkspaceID string
	Slug        string
	Name        string
	Publisher   string
	Version     string
	Description string

	LicenseToken  string
	BundleSHA256  string
	BundleSize    int64
	BundlePayload []byte

	Manifest ExtensionManifest
	Config   shareddomain.TypedCustomFields

	Status            ExtensionStatus
	ValidationStatus  ExtensionValidationStatus
	ValidationMessage string
	HealthStatus      ExtensionHealthStatus
	HealthMessage     string
	InstalledByID     string
	InstalledAt       time.Time
	ActivatedAt       *time.Time
	DeactivatedAt     *time.Time
	ValidatedAt       *time.Time
	LastHealthCheckAt *time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

type ExtensionRuntimeDiagnostics struct {
	BootstrapStatus    string
	LastBootstrapAt    *time.Time
	LastBootstrapError string
	Endpoints          []ExtensionRuntimeEndpointState
	EventConsumers     []ExtensionRuntimeConsumerState
	ScheduledJobs      []ExtensionRuntimeJobState
}

type ExtensionRuntimeEndpointState struct {
	Name                string
	Class               string
	MountPath           string
	ServiceTarget       string
	Status              string
	ConsecutiveFailures int
	RegisteredAt        *time.Time
	LastCheckedAt       *time.Time
	LastSuccessAt       *time.Time
	LastFailureAt       *time.Time
	LastError           string
}

type ExtensionRuntimeConsumerState struct {
	Name                string
	Stream              string
	ConsumerGroup       string
	ServiceTarget       string
	Status              string
	ConsecutiveFailures int
	RegisteredAt        *time.Time
	LastDeliveredAt     *time.Time
	LastSuccessAt       *time.Time
	LastFailureAt       *time.Time
	LastError           string
}

type ExtensionRuntimeJobState struct {
	Name                string
	IntervalSeconds     int
	ServiceTarget       string
	Status              string
	ConsecutiveFailures int
	RegisteredAt        *time.Time
	LastStartedAt       *time.Time
	LastSuccessAt       *time.Time
	LastFailureAt       *time.Time
	BackoffUntil        *time.Time
	LastError           string
}

func NewInstalledExtension(workspaceID, installedByID, licenseToken string, manifest ExtensionManifest, bundle []byte) (*InstalledExtension, error) {
	manifest.Normalize()
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if manifest.Scope == ExtensionScopeWorkspace && workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if manifest.Scope == ExtensionScopeInstance && workspaceID != "" {
		return nil, fmt.Errorf("workspace_id is not allowed for instance-scoped extensions")
	}

	now := time.Now()
	config := manifest.DefaultConfig
	if config.IsEmpty() {
		config = shareddomain.NewTypedCustomFields()
	}

	installation := &InstalledExtension{
		WorkspaceID:       workspaceID,
		Slug:              manifest.Slug,
		Name:              manifest.Name,
		Publisher:         manifest.Publisher,
		Version:           manifest.Version,
		Description:       manifest.Description,
		LicenseToken:      strings.TrimSpace(licenseToken),
		BundleSHA256:      checksumBytes(bundle),
		BundleSize:        int64(len(bundle)),
		BundlePayload:     cloneBundlePayload(bundle),
		Manifest:          manifest,
		Config:            config,
		Status:            ExtensionStatusInstalled,
		ValidationStatus:  ExtensionValidationValid,
		ValidationMessage: "bundle parsed and manifest validated",
		HealthStatus:      ExtensionHealthInactive,
		HealthMessage:     "extension installed but not active",
		InstalledByID:     strings.TrimSpace(installedByID),
		InstalledAt:       now,
		ValidatedAt:       &now,
		UpdatedAt:         now,
	}
	return installation, nil
}

func cloneBundlePayload(bundle []byte) []byte {
	if bundle == nil {
		return []byte{}
	}
	cloned := make([]byte, len(bundle))
	copy(cloned, bundle)
	return cloned
}

func (e *InstalledExtension) IsInstanceScoped() bool {
	return e != nil && e.Manifest.Scope == ExtensionScopeInstance
}

func (e *InstalledExtension) Activate() {
	now := time.Now()
	e.Status = ExtensionStatusActive
	e.HealthStatus = ExtensionHealthHealthy
	e.HealthMessage = "extension active"
	e.ActivatedAt = &now
	e.DeactivatedAt = nil
	e.UpdatedAt = now
}

func (e *InstalledExtension) Deactivate(reason string) {
	now := time.Now()
	e.Status = ExtensionStatusInactive
	e.HealthStatus = ExtensionHealthInactive
	e.HealthMessage = strings.TrimSpace(reason)
	if e.HealthMessage == "" {
		e.HealthMessage = "extension inactive"
	}
	e.DeactivatedAt = &now
	e.UpdatedAt = now
}

func (e *InstalledExtension) MarkValidation(valid bool, message string) {
	now := time.Now()
	e.ValidatedAt = &now
	e.ValidationMessage = strings.TrimSpace(message)
	if valid {
		e.ValidationStatus = ExtensionValidationValid
		if e.ValidationMessage == "" {
			e.ValidationMessage = "manifest and installed assets validated"
		}
		if e.Status == ExtensionStatusActive {
			e.HealthStatus = ExtensionHealthHealthy
			e.HealthMessage = "extension active"
		}
	} else {
		e.ValidationStatus = ExtensionValidationInvalid
		if e.ValidationMessage == "" {
			e.ValidationMessage = "validation failed"
		}
		e.Status = ExtensionStatusFailed
		e.HealthStatus = ExtensionHealthFailed
		e.HealthMessage = e.ValidationMessage
	}
	e.UpdatedAt = now
}

func (e *InstalledExtension) UpdateConfig(config shareddomain.TypedCustomFields) {
	if config.IsEmpty() {
		config = shareddomain.NewTypedCustomFields()
	}
	e.Config = config
	e.UpdatedAt = time.Now()
}

func (e *InstalledExtension) EffectiveConfig() shareddomain.TypedCustomFields {
	if e == nil {
		return shareddomain.NewTypedCustomFields()
	}

	effective := shareddomain.NewTypedCustomFields()
	for key, value := range e.Manifest.DefaultConfig.ToMap() {
		effective.SetAny(key, value)
	}
	for key, value := range e.Config.ToMap() {
		effective.SetAny(key, value)
	}
	return effective
}

func (e *InstalledExtension) RecordHealth(status ExtensionHealthStatus, message string) {
	now := time.Now()
	e.HealthStatus = status
	e.HealthMessage = strings.TrimSpace(message)
	e.LastHealthCheckAt = &now
	e.UpdatedAt = now
}

type ExtensionAsset struct {
	ID             string
	ExtensionID    string
	Path           string
	Kind           ExtensionAssetKind
	ContentType    string
	Content        []byte
	IsCustomizable bool
	Checksum       string
	Size           int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type ExtensionArtifactFile struct {
	Surface string
	Path    string
}

type ExtensionArtifactPublication struct {
	Surface     string
	Path        string
	RevisionRef string
}

type ExtensionArtifactDiff struct {
	Surface      string
	Path         string
	FromRevision string
	ToRevision   string
	Patch        string
}

func NewExtensionAsset(extensionID, assetPath, contentType string, content []byte, customizable bool) (*ExtensionAsset, error) {
	path := normalizeAssetPath(assetPath)
	if extensionID == "" {
		return nil, fmt.Errorf("extension_id is required")
	}
	if path == "" {
		return nil, fmt.Errorf("asset path is required")
	}
	now := time.Now()
	asset := &ExtensionAsset{
		ExtensionID:    extensionID,
		Path:           path,
		Kind:           inferAssetKind(path),
		ContentType:    strings.TrimSpace(contentType),
		Content:        content,
		IsCustomizable: customizable,
		Checksum:       checksumBytes(content),
		Size:           int64(len(content)),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if asset.ContentType == "" {
		asset.ContentType = "application/octet-stream"
	}
	return asset, nil
}

func (a *ExtensionAsset) UpdateContent(content []byte, contentType string) {
	a.Content = content
	a.Size = int64(len(content))
	a.Checksum = checksumBytes(content)
	if strings.TrimSpace(contentType) != "" {
		a.ContentType = strings.TrimSpace(contentType)
	}
	a.UpdatedAt = time.Now()
}

func inferAssetKind(assetPath string) ExtensionAssetKind {
	switch {
	case strings.HasPrefix(assetPath, "templates/"):
		return ExtensionAssetKindTemplate
	case strings.HasPrefix(assetPath, "public/"):
		return ExtensionAssetKindStatic
	default:
		return ExtensionAssetKindOther
	}
}

func normalizeExtensionSlug(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.Join(strings.FieldsFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-')
	}), "-")
	return strings.Trim(value, "-")
}

func normalizeRoutePrefix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return strings.TrimRight(value, "/")
}

func normalizeExtensionArtifactSurfaceName(value string) string {
	return normalizeExtensionNavigationName(value)
}

func normalizeAssetPath(value string) string {
	value = filepath.ToSlash(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "./")
	value = strings.TrimPrefix(value, "/")
	return strings.TrimSpace(value)
}

func normalizeEndpointClass(value ExtensionEndpointClass) ExtensionEndpointClass {
	normalized := ExtensionEndpointClass(strings.TrimSpace(strings.ToLower(string(value))))
	switch normalized {
	case ExtensionEndpointClassPublicPage,
		ExtensionEndpointClassPublicAsset,
		ExtensionEndpointClassPublicIngest,
		ExtensionEndpointClassWebhook,
		ExtensionEndpointClassAdminPage,
		ExtensionEndpointClassAdminAction,
		ExtensionEndpointClassExtensionAPI,
		ExtensionEndpointClassHealth:
		return normalized
	default:
		return ""
	}
}

func normalizeEndpointAuth(value ExtensionEndpointAuth, class ExtensionEndpointClass) ExtensionEndpointAuth {
	normalized := ExtensionEndpointAuth(strings.TrimSpace(strings.ToLower(string(value))))
	switch normalized {
	case ExtensionEndpointAuthPublic,
		ExtensionEndpointAuthSignedWebhook,
		ExtensionEndpointAuthSession,
		ExtensionEndpointAuthAgentToken,
		ExtensionEndpointAuthExtensionToken,
		ExtensionEndpointAuthInternalOnly:
		return normalized
	}

	switch class {
	case ExtensionEndpointClassWebhook:
		return ExtensionEndpointAuthSignedWebhook
	case ExtensionEndpointClassAdminPage, ExtensionEndpointClassAdminAction:
		return ExtensionEndpointAuthSession
	case ExtensionEndpointClassHealth:
		return ExtensionEndpointAuthInternalOnly
	default:
		return ExtensionEndpointAuthPublic
	}
}

func normalizeWorkspaceBinding(value ExtensionWorkspaceBinding, class ExtensionEndpointClass) ExtensionWorkspaceBinding {
	normalized := ExtensionWorkspaceBinding(strings.TrimSpace(strings.ToLower(string(value))))
	switch normalized {
	case ExtensionWorkspaceBindingNone,
		ExtensionWorkspaceBindingFromSession,
		ExtensionWorkspaceBindingFromAgentToken,
		ExtensionWorkspaceBindingFromRoute,
		ExtensionWorkspaceBindingInstanceScoped:
		return normalized
	}

	switch class {
	case ExtensionEndpointClassAdminPage, ExtensionEndpointClassAdminAction:
		return ExtensionWorkspaceBindingFromSession
	case ExtensionEndpointClassExtensionAPI:
		return ExtensionWorkspaceBindingFromAgentToken
	default:
		return ExtensionWorkspaceBindingNone
	}
}

func normalizeWorkspaceInstallMode(value ExtensionWorkspaceInstallMode) ExtensionWorkspaceInstallMode {
	switch ExtensionWorkspaceInstallMode(strings.TrimSpace(strings.ToLower(string(value)))) {
	case ExtensionWorkspaceInstallIntoExisting, ExtensionWorkspaceProvisionDedicated:
		return ExtensionWorkspaceInstallMode(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func normalizeMethodList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		method := strings.TrimSpace(strings.ToUpper(value))
		if method == "" || slices.Contains(out, method) {
			continue
		}
		out = append(out, method)
	}
	return out
}

func normalizeEventTypeList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeExtensionEventType(value)
		if value == "" || slices.Contains(out, value) {
			continue
		}
		out = append(out, value)
	}
	return out
}

func normalizeExtensionEventStream(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func normalizeExtensionConsumerGroup(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func normalizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(out, value) {
			continue
		}
		out = append(out, value)
	}
	return out
}

func normalizeExtensionRuleKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func normalizeExtensionEventType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func normalizeExtensionNavigationName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return strings.Trim(value, "-")
}

func isValidPublishedExtensionEventType(value string) bool {
	if !strings.HasPrefix(value, "ext.") {
		return false
	}
	parts := strings.Split(value, ".")
	if len(parts) < 4 {
		return false
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return false
		}
	}
	return true
}

func normalizeExtensionKind(value ExtensionKind) ExtensionKind {
	switch ExtensionKind(strings.TrimSpace(strings.ToLower(string(value)))) {
	case ExtensionKindProduct, ExtensionKindIdentity, ExtensionKindConnector, ExtensionKindOperational:
		return ExtensionKind(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func normalizeExtensionScope(value ExtensionScope) ExtensionScope {
	switch ExtensionScope(strings.TrimSpace(strings.ToLower(string(value)))) {
	case ExtensionScopeWorkspace, ExtensionScopeInstance:
		return ExtensionScope(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func normalizeExtensionRisk(value ExtensionRisk) ExtensionRisk {
	switch ExtensionRisk(strings.TrimSpace(strings.ToLower(string(value)))) {
	case ExtensionRiskStandard, ExtensionRiskPrivileged:
		return ExtensionRisk(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func normalizeRuntimeClass(value ExtensionRuntimeClass) ExtensionRuntimeClass {
	switch ExtensionRuntimeClass(strings.TrimSpace(strings.ToLower(string(value)))) {
	case "":
		return ExtensionRuntimeClassBundle
	case ExtensionRuntimeClassBundle, ExtensionRuntimeClassServiceBacked:
		return ExtensionRuntimeClass(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func normalizeRuntimeProtocol(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", ExtensionRuntimeProtocolInProcessHTTP, ExtensionRuntimeProtocolUnixSocketHTTP:
		return value
	default:
		return value
	}
}

func normalizeStorageClass(value ExtensionStorageClass, _ ExtensionRuntimeClass) ExtensionStorageClass {
	switch ExtensionStorageClass(strings.TrimSpace(strings.ToLower(string(value)))) {
	case "":
		return ExtensionStorageClassSharedPrimitivesOnly
	case ExtensionStorageClassSharedPrimitivesOnly, ExtensionStorageClassOwnedSchema:
		return ExtensionStorageClass(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func normalizeExtensionSchemaName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	value = strings.NewReplacer("-", "_", "/", "_", " ", "_").Replace(value)
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	})
	return strings.Trim(strings.Join(parts, "_"), "_")
}

func normalizeExtensionPackageKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = normalizeExtensionSlug(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "/")
}

func normalizePathList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeAssetPath(value)
		if value == "" || slices.Contains(out, value) {
			continue
		}
		out = append(out, value)
	}
	return out
}

func manifestHasHealthEndpoint(endpoints []ExtensionEndpoint) bool {
	for _, endpoint := range endpoints {
		if endpoint.Class == ExtensionEndpointClassHealth {
			return true
		}
	}
	return false
}

func checksumBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
