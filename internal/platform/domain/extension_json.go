package platformdomain

import (
	"encoding/json"

	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

type extensionManifestJSON struct {
	SchemaVersion      int                            `json:"schemaVersion"`
	Slug               string                         `json:"slug"`
	Name               string                         `json:"name"`
	Version            string                         `json:"version"`
	Publisher          string                         `json:"publisher"`
	Kind               ExtensionKind                  `json:"kind"`
	Scope              ExtensionScope                 `json:"scope"`
	Risk               ExtensionRisk                  `json:"risk"`
	RuntimeClass       ExtensionRuntimeClass          `json:"runtimeClass,omitempty"`
	StorageClass       ExtensionStorageClass          `json:"storageClass,omitempty"`
	Schema             extensionSchemaManifestJSON    `json:"schema,omitempty"`
	Runtime            extensionRuntimeSpecJSON       `json:"runtime,omitempty"`
	Description        string                         `json:"description,omitempty"`
	WorkspacePlan      extensionWorkspacePlanJSON     `json:"workspacePlan,omitempty"`
	Permissions        []string                       `json:"permissions,omitempty"`
	Queues             []extensionQueueSeedJSON       `json:"queues,omitempty"`
	Forms              []extensionFormSeedJSON        `json:"forms,omitempty"`
	AutomationRules    []extensionAutomationSeedJSON  `json:"automationRules,omitempty"`
	ArtifactSurfaces   []extensionArtifactSurfaceJSON `json:"artifactSurfaces,omitempty"`
	PublicRoutes       []extensionRouteJSON           `json:"publicRoutes,omitempty"`
	AdminRoutes        []extensionRouteJSON           `json:"adminRoutes,omitempty"`
	Endpoints          []extensionEndpointJSON        `json:"endpoints,omitempty"`
	AdminNavigation    []extensionAdminNavItemJSON    `json:"adminNavigation,omitempty"`
	DashboardWidgets   []extensionDashboardWidgetJSON `json:"dashboardWidgets,omitempty"`
	Events             extensionEventCatalogJSON      `json:"events,omitempty"`
	EventConsumers     []extensionEventConsumerJSON   `json:"eventConsumers,omitempty"`
	ScheduledJobs      []extensionScheduledJobJSON    `json:"scheduledJobs,omitempty"`
	Commands           []extensionCommandJSON         `json:"commands,omitempty"`
	AgentSkills        []extensionAgentSkillJSON      `json:"agentSkills,omitempty"`
	CustomizableAssets []string                       `json:"customizableAssets,omitempty"`
	DefaultConfig      shareddomain.TypedCustomFields `json:"defaultConfig,omitempty"`
}

type extensionSchemaManifestJSON struct {
	Name            string `json:"name,omitempty"`
	PackageKey      string `json:"packageKey,omitempty"`
	TargetVersion   string `json:"targetVersion,omitempty"`
	MigrationEngine string `json:"migrationEngine,omitempty"`
}

type extensionRuntimeSpecJSON struct {
	Protocol     string `json:"protocol,omitempty"`
	OCIReference string `json:"ociReference,omitempty"`
	Digest       string `json:"digest,omitempty"`
}

type extensionWorkspacePlanJSON struct {
	Mode        ExtensionWorkspaceInstallMode `json:"mode,omitempty"`
	Name        string                        `json:"name,omitempty"`
	Slug        string                        `json:"slug,omitempty"`
	Description string                        `json:"description,omitempty"`
}

type extensionQueueSeedJSON struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type extensionFormSeedJSON struct {
	Slug              string                   `json:"slug"`
	Name              string                   `json:"name"`
	Description       string                   `json:"description,omitempty"`
	Status            string                   `json:"status,omitempty"`
	IsPublic          bool                     `json:"isPublic,omitempty"`
	RequiresAuth      bool                     `json:"requiresAuth,omitempty"`
	AllowMultiple     bool                     `json:"allowMultiple,omitempty"`
	CollectEmail      bool                     `json:"collectEmail,omitempty"`
	AutoCreateCase    bool                     `json:"autoCreateCase,omitempty"`
	AutoCasePriority  string                   `json:"autoCasePriority,omitempty"`
	AutoCaseType      string                   `json:"autoCaseType,omitempty"`
	AutoTags          []string                 `json:"autoTags,omitempty"`
	SubmissionMessage string                   `json:"submissionMessage,omitempty"`
	RedirectURL       string                   `json:"redirectURL,omitempty"`
	Theme             string                   `json:"theme,omitempty"`
	Schema            shareddomain.TypedSchema `json:"schema,omitempty"`
	UISchema          shareddomain.TypedSchema `json:"uiSchema,omitempty"`
	ValidationRules   shareddomain.TypedSchema `json:"validationRules,omitempty"`
}

type extensionAutomationSeedJSON struct {
	Key                  string                   `json:"key"`
	Title                string                   `json:"title"`
	Description          string                   `json:"description,omitempty"`
	IsActive             bool                     `json:"isActive,omitempty"`
	Priority             int                      `json:"priority,omitempty"`
	MaxExecutionsPerHour int                      `json:"maxExecutionsPerHour,omitempty"`
	MaxExecutionsPerDay  int                      `json:"maxExecutionsPerDay,omitempty"`
	Conditions           shareddomain.TypedSchema `json:"conditions,omitempty"`
	Actions              shareddomain.TypedSchema `json:"actions,omitempty"`
}

type extensionArtifactSurfaceJSON struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	SeedAssetPath string `json:"seedAssetPath,omitempty"`
}

type extensionRouteJSON struct {
	PathPrefix      string `json:"pathPrefix"`
	AssetPath       string `json:"assetPath,omitempty"`
	ArtifactSurface string `json:"artifactSurface,omitempty"`
	ArtifactPath    string `json:"artifactPath,omitempty"`
}

type extensionEndpointJSON struct {
	Name             string                    `json:"name"`
	Class            ExtensionEndpointClass    `json:"class"`
	MountPath        string                    `json:"mountPath"`
	Methods          []string                  `json:"methods,omitempty"`
	Auth             ExtensionEndpointAuth     `json:"auth,omitempty"`
	ContentTypes     []string                  `json:"contentTypes,omitempty"`
	MaxBodyBytes     int64                     `json:"maxBodyBytes,omitempty"`
	RateLimitPerMin  int                       `json:"rateLimitPerMinute,omitempty"`
	WorkspaceBinding ExtensionWorkspaceBinding `json:"workspaceBinding,omitempty"`
	AssetPath        string                    `json:"assetPath,omitempty"`
	ArtifactSurface  string                    `json:"artifactSurface,omitempty"`
	ArtifactPath     string                    `json:"artifactPath,omitempty"`
	ServiceTarget    string                    `json:"serviceTarget,omitempty"`
}

type extensionAdminNavItemJSON struct {
	Name       string `json:"name"`
	Section    string `json:"section,omitempty"`
	Title      string `json:"title"`
	Icon       string `json:"icon,omitempty"`
	Endpoint   string `json:"endpoint"`
	ActivePage string `json:"activePage,omitempty"`
}

type extensionDashboardWidgetJSON struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Endpoint    string `json:"endpoint"`
}

type extensionEventConsumerJSON struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Stream        string   `json:"stream"`
	EventTypes    []string `json:"eventTypes,omitempty"`
	ConsumerGroup string   `json:"consumerGroup,omitempty"`
	ServiceTarget string   `json:"serviceTarget"`
}

type extensionScheduledJobJSON struct {
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	IntervalSeconds int    `json:"intervalSeconds"`
	ServiceTarget   string `json:"serviceTarget"`
}

type extensionCommandJSON struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type extensionAgentSkillJSON struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	AssetPath   string `json:"assetPath"`
}

type extensionEventCatalogJSON struct {
	Publishes  []extensionEventDefinitionJSON `json:"publishes,omitempty"`
	Subscribes []string                       `json:"subscribes,omitempty"`
}

type extensionEventDefinitionJSON struct {
	Type          string `json:"type"`
	Description   string `json:"description,omitempty"`
	SchemaVersion int    `json:"schemaVersion,omitempty"`
}

func (m ExtensionManifest) MarshalJSON() ([]byte, error) {
	return json.Marshal(extensionManifestJSONFromDomain(m))
}

func (m *ExtensionManifest) UnmarshalJSON(data []byte) error {
	var payload extensionManifestJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	*m = payload.toDomain()
	return nil
}

func extensionManifestJSONFromDomain(m ExtensionManifest) extensionManifestJSON {
	return extensionManifestJSON{
		SchemaVersion:      m.SchemaVersion,
		Slug:               m.Slug,
		Name:               m.Name,
		Version:            m.Version,
		Publisher:          m.Publisher,
		Kind:               m.Kind,
		Scope:              m.Scope,
		Risk:               m.Risk,
		RuntimeClass:       m.RuntimeClass,
		StorageClass:       m.StorageClass,
		Schema:             extensionSchemaManifestJSON(m.Schema),
		Runtime:            extensionRuntimeSpecJSON(m.Runtime),
		Description:        m.Description,
		WorkspacePlan:      extensionWorkspacePlanJSON(m.WorkspacePlan),
		Permissions:        m.Permissions,
		Queues:             queueSeedsJSONFromDomain(m.Queues),
		Forms:              formSeedsJSONFromDomain(m.Forms),
		AutomationRules:    automationSeedsJSONFromDomain(m.AutomationRules),
		ArtifactSurfaces:   artifactSurfacesJSONFromDomain(m.ArtifactSurfaces),
		PublicRoutes:       routesJSONFromDomain(m.PublicRoutes),
		AdminRoutes:        routesJSONFromDomain(m.AdminRoutes),
		Endpoints:          endpointsJSONFromDomain(m.Endpoints),
		AdminNavigation:    adminNavJSONFromDomain(m.AdminNavigation),
		DashboardWidgets:   widgetsJSONFromDomain(m.DashboardWidgets),
		Events:             eventCatalogJSONFromDomain(m.Events),
		EventConsumers:     eventConsumersJSONFromDomain(m.EventConsumers),
		ScheduledJobs:      scheduledJobsJSONFromDomain(m.ScheduledJobs),
		Commands:           commandsJSONFromDomain(m.Commands),
		AgentSkills:        agentSkillsJSONFromDomain(m.AgentSkills),
		CustomizableAssets: m.CustomizableAssets,
		DefaultConfig:      m.DefaultConfig,
	}
}

func (m extensionManifestJSON) toDomain() ExtensionManifest {
	return ExtensionManifest{
		SchemaVersion:      m.SchemaVersion,
		Slug:               m.Slug,
		Name:               m.Name,
		Version:            m.Version,
		Publisher:          m.Publisher,
		Kind:               m.Kind,
		Scope:              m.Scope,
		Risk:               m.Risk,
		RuntimeClass:       m.RuntimeClass,
		StorageClass:       m.StorageClass,
		Schema:             ExtensionSchemaManifest(m.Schema),
		Runtime:            ExtensionRuntimeSpec(m.Runtime),
		Description:        m.Description,
		WorkspacePlan:      ExtensionWorkspacePlan(m.WorkspacePlan),
		Permissions:        m.Permissions,
		Queues:             queueSeedsFromJSON(m.Queues),
		Forms:              formSeedsFromJSON(m.Forms),
		AutomationRules:    automationSeedsFromJSON(m.AutomationRules),
		ArtifactSurfaces:   artifactSurfacesFromJSON(m.ArtifactSurfaces),
		PublicRoutes:       routesFromJSON(m.PublicRoutes),
		AdminRoutes:        routesFromJSON(m.AdminRoutes),
		Endpoints:          endpointsFromJSON(m.Endpoints),
		AdminNavigation:    adminNavFromJSON(m.AdminNavigation),
		DashboardWidgets:   widgetsFromJSON(m.DashboardWidgets),
		Events:             m.Events.toDomain(),
		EventConsumers:     eventConsumersFromJSON(m.EventConsumers),
		ScheduledJobs:      scheduledJobsFromJSON(m.ScheduledJobs),
		Commands:           commandsFromJSON(m.Commands),
		AgentSkills:        agentSkillsFromJSON(m.AgentSkills),
		CustomizableAssets: m.CustomizableAssets,
		DefaultConfig:      m.DefaultConfig,
	}
}

func queueSeedsJSONFromDomain(in []ExtensionQueueSeed) []extensionQueueSeedJSON {
	out := make([]extensionQueueSeedJSON, len(in))
	for i := range in {
		out[i] = extensionQueueSeedJSON(in[i])
	}
	return out
}

func queueSeedsFromJSON(in []extensionQueueSeedJSON) []ExtensionQueueSeed {
	out := make([]ExtensionQueueSeed, len(in))
	for i := range in {
		out[i] = ExtensionQueueSeed(in[i])
	}
	return out
}

func formSeedsJSONFromDomain(in []ExtensionFormSeed) []extensionFormSeedJSON {
	out := make([]extensionFormSeedJSON, len(in))
	for i := range in {
		out[i] = extensionFormSeedJSON(in[i])
	}
	return out
}

func formSeedsFromJSON(in []extensionFormSeedJSON) []ExtensionFormSeed {
	out := make([]ExtensionFormSeed, len(in))
	for i := range in {
		out[i] = ExtensionFormSeed(in[i])
	}
	return out
}

func automationSeedsJSONFromDomain(in []ExtensionAutomationSeed) []extensionAutomationSeedJSON {
	out := make([]extensionAutomationSeedJSON, len(in))
	for i := range in {
		out[i] = extensionAutomationSeedJSON(in[i])
	}
	return out
}

func automationSeedsFromJSON(in []extensionAutomationSeedJSON) []ExtensionAutomationSeed {
	out := make([]ExtensionAutomationSeed, len(in))
	for i := range in {
		out[i] = ExtensionAutomationSeed(in[i])
	}
	return out
}

func artifactSurfacesJSONFromDomain(in []ExtensionArtifactSurface) []extensionArtifactSurfaceJSON {
	out := make([]extensionArtifactSurfaceJSON, len(in))
	for i := range in {
		out[i] = extensionArtifactSurfaceJSON(in[i])
	}
	return out
}

func artifactSurfacesFromJSON(in []extensionArtifactSurfaceJSON) []ExtensionArtifactSurface {
	out := make([]ExtensionArtifactSurface, len(in))
	for i := range in {
		out[i] = ExtensionArtifactSurface(in[i])
	}
	return out
}

func routesJSONFromDomain(in []ExtensionRoute) []extensionRouteJSON {
	out := make([]extensionRouteJSON, len(in))
	for i := range in {
		out[i] = extensionRouteJSON(in[i])
	}
	return out
}

func routesFromJSON(in []extensionRouteJSON) []ExtensionRoute {
	out := make([]ExtensionRoute, len(in))
	for i := range in {
		out[i] = ExtensionRoute(in[i])
	}
	return out
}

func endpointsJSONFromDomain(in []ExtensionEndpoint) []extensionEndpointJSON {
	out := make([]extensionEndpointJSON, len(in))
	for i := range in {
		out[i] = extensionEndpointJSON(in[i])
	}
	return out
}

func endpointsFromJSON(in []extensionEndpointJSON) []ExtensionEndpoint {
	out := make([]ExtensionEndpoint, len(in))
	for i := range in {
		out[i] = ExtensionEndpoint(in[i])
	}
	return out
}

func adminNavJSONFromDomain(in []ExtensionAdminNavigationItem) []extensionAdminNavItemJSON {
	out := make([]extensionAdminNavItemJSON, len(in))
	for i := range in {
		out[i] = extensionAdminNavItemJSON(in[i])
	}
	return out
}

func adminNavFromJSON(in []extensionAdminNavItemJSON) []ExtensionAdminNavigationItem {
	out := make([]ExtensionAdminNavigationItem, len(in))
	for i := range in {
		out[i] = ExtensionAdminNavigationItem(in[i])
	}
	return out
}

func widgetsJSONFromDomain(in []ExtensionDashboardWidget) []extensionDashboardWidgetJSON {
	out := make([]extensionDashboardWidgetJSON, len(in))
	for i := range in {
		out[i] = extensionDashboardWidgetJSON(in[i])
	}
	return out
}

func widgetsFromJSON(in []extensionDashboardWidgetJSON) []ExtensionDashboardWidget {
	out := make([]ExtensionDashboardWidget, len(in))
	for i := range in {
		out[i] = ExtensionDashboardWidget(in[i])
	}
	return out
}

func eventConsumersJSONFromDomain(in []ExtensionEventConsumer) []extensionEventConsumerJSON {
	out := make([]extensionEventConsumerJSON, len(in))
	for i := range in {
		out[i] = extensionEventConsumerJSON(in[i])
	}
	return out
}

func eventConsumersFromJSON(in []extensionEventConsumerJSON) []ExtensionEventConsumer {
	out := make([]ExtensionEventConsumer, len(in))
	for i := range in {
		out[i] = ExtensionEventConsumer(in[i])
	}
	return out
}

func scheduledJobsJSONFromDomain(in []ExtensionScheduledJob) []extensionScheduledJobJSON {
	out := make([]extensionScheduledJobJSON, len(in))
	for i := range in {
		out[i] = extensionScheduledJobJSON(in[i])
	}
	return out
}

func scheduledJobsFromJSON(in []extensionScheduledJobJSON) []ExtensionScheduledJob {
	out := make([]ExtensionScheduledJob, len(in))
	for i := range in {
		out[i] = ExtensionScheduledJob(in[i])
	}
	return out
}

func commandsJSONFromDomain(in []ExtensionCommand) []extensionCommandJSON {
	out := make([]extensionCommandJSON, len(in))
	for i := range in {
		out[i] = extensionCommandJSON(in[i])
	}
	return out
}

func commandsFromJSON(in []extensionCommandJSON) []ExtensionCommand {
	out := make([]ExtensionCommand, len(in))
	for i := range in {
		out[i] = ExtensionCommand(in[i])
	}
	return out
}

func agentSkillsJSONFromDomain(in []ExtensionAgentSkill) []extensionAgentSkillJSON {
	out := make([]extensionAgentSkillJSON, len(in))
	for i := range in {
		out[i] = extensionAgentSkillJSON(in[i])
	}
	return out
}

func agentSkillsFromJSON(in []extensionAgentSkillJSON) []ExtensionAgentSkill {
	out := make([]ExtensionAgentSkill, len(in))
	for i := range in {
		out[i] = ExtensionAgentSkill(in[i])
	}
	return out
}

func eventCatalogJSONFromDomain(in ExtensionEventCatalog) extensionEventCatalogJSON {
	out := extensionEventCatalogJSON{
		Publishes:  make([]extensionEventDefinitionJSON, len(in.Publishes)),
		Subscribes: in.Subscribes,
	}
	for i := range in.Publishes {
		out.Publishes[i] = extensionEventDefinitionJSON(in.Publishes[i])
	}
	return out
}

func (in extensionEventCatalogJSON) toDomain() ExtensionEventCatalog {
	out := ExtensionEventCatalog{
		Publishes:  make([]ExtensionEventDefinition, len(in.Publishes)),
		Subscribes: in.Subscribes,
	}
	for i := range in.Publishes {
		out.Publishes[i] = ExtensionEventDefinition(in.Publishes[i])
	}
	return out
}
