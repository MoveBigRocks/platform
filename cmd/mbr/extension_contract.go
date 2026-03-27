package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
)

const (
	defaultExtensionContractFile = "extension.contract.json"
	extensionContractSchemaV1    = 1
)

type extensionContractFile struct {
	SchemaVersion            int                                `json:"schemaVersion"`
	ExtensionSlug            string                             `json:"extensionSlug,omitempty"`
	ResolvedAdminNavigation  []extensionContractNavigationItem  `json:"resolvedAdminNavigation,omitempty"`
	ResolvedDashboardWidgets []extensionContractDashboardWidget `json:"resolvedDashboardWidgets,omitempty"`
	SeededQueues             []string                           `json:"seededQueues,omitempty"`
	SeededForms              []string                           `json:"seededForms,omitempty"`
	SeededAutomationRules    []string                           `json:"seededAutomationRules,omitempty"`
	PublicPaths              []string                           `json:"publicPaths,omitempty"`
	AdminPaths               []string                           `json:"adminPaths,omitempty"`
	HealthPaths              []string                           `json:"healthPaths,omitempty"`
	Commands                 []string                           `json:"commands,omitempty"`
	AgentSkills              []string                           `json:"agentSkills,omitempty"`
}

type extensionContractNavigationItem struct {
	Section    *string `json:"section,omitempty"`
	Title      string  `json:"title"`
	Icon       *string `json:"icon,omitempty"`
	Href       string  `json:"href"`
	ActivePage *string `json:"activePage,omitempty"`
}

type extensionContractDashboardWidget struct {
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	Href        string  `json:"href"`
}

type extensionLintOutput struct {
	Source          string                `json:"source"`
	ContractPath    string                `json:"contractPath"`
	ExtensionSlug   string                `json:"extensionSlug"`
	ManifestValid   bool                  `json:"manifestValid"`
	ManifestMessage string                `json:"manifestMessage"`
	ContractValid   bool                  `json:"contractValid"`
	Problems        []string              `json:"problems,omitempty"`
	Derived         extensionContractFile `json:"derived"`
}

type extensionVerifyOutput struct {
	Source                            string                                       `json:"source"`
	ContractPath                      string                                       `json:"contractPath"`
	WorkspaceID                       *string                                      `json:"workspaceID,omitempty"`
	Operation                         string                                       `json:"operation"`
	ProvisionedWorkspace              *workspaceOutput                             `json:"provisionedWorkspace,omitempty"`
	Lint                              extensionLintOutput                          `json:"lint"`
	Installed                         extensionOutput                              `json:"installed"`
	Validated                         extensionOutput                              `json:"validated"`
	Activated                         extensionOutput                              `json:"activated"`
	Monitored                         extensionOutput                              `json:"monitored"`
	Detail                            extensionDetailOutput                        `json:"detail"`
	WorkspaceResolvedAdminNavigation  []resolvedExtensionAdminNavigationItemOutput `json:"workspaceResolvedAdminNavigation,omitempty"`
	WorkspaceResolvedDashboardWidgets []resolvedExtensionDashboardWidgetOutput     `json:"workspaceResolvedDashboardWidgets,omitempty"`
	InstanceResolvedAdminNavigation   []resolvedExtensionAdminNavigationItemOutput `json:"instanceResolvedAdminNavigation,omitempty"`
	InstanceResolvedDashboardWidgets  []resolvedExtensionDashboardWidgetOutput     `json:"instanceResolvedDashboardWidgets,omitempty"`
}

type preparedExtensionSource struct {
	root         string
	bundle       bundleSourcePayload
	manifest     platformdomain.ExtensionManifest
	contract     extensionContractFile
	contractPath string
	lint         extensionLintOutput
}

func prepareExtensionSource(ctx context.Context, sourceDir, contractPath string) (preparedExtensionSource, error) {
	root, bundle, err := readLocalExtensionSourcePayload(sourceDir)
	if err != nil {
		return preparedExtensionSource{}, err
	}
	manifest, err := decodeBundleManifest(bundle.Bundle.Manifest)
	if err != nil {
		return preparedExtensionSource{}, fmt.Errorf("decode extension manifest: %w", err)
	}
	contract, resolvedContractPath, contractProblems := loadExtensionContract(root, contractPath)

	derived := deriveExtensionContractFromManifest(manifest)
	service := platformservices.NewExtensionService(nil, nil, nil, nil, nil, nil)
	assets := bundleAssetInputs(bundle)
	manifestValid, manifestMessage, err := service.ValidateManifestSource(ctx, "", manifest, assets)
	if err != nil {
		return preparedExtensionSource{}, err
	}

	problems := []string{}
	if !manifestValid {
		problems = append(problems, manifestMessage)
	}
	problems = append(problems, contractProblems...)
	contractValid := len(contractProblems) == 0
	if contractValid {
		contractProblems = compareExtensionContract(contract, derived)
		problems = append(problems, contractProblems...)
		contractValid = len(contractProblems) == 0
	}

	lint := extensionLintOutput{
		Source:          root,
		ContractPath:    resolvedContractPath,
		ExtensionSlug:   manifest.Slug,
		ManifestValid:   manifestValid,
		ManifestMessage: manifestMessage,
		ContractValid:   contractValid,
		Problems:        append([]string{}, problems...),
		Derived:         derived,
	}

	return preparedExtensionSource{
		root:         root,
		bundle:       bundle,
		manifest:     manifest,
		contract:     contract,
		contractPath: resolvedContractPath,
		lint:         lint,
	}, nil
}

func readLocalExtensionSourcePayload(sourceDir string) (string, bundleSourcePayload, error) {
	root := filepath.Clean(strings.TrimSpace(sourceDir))
	if root == "" {
		return "", bundleSourcePayload{}, fmt.Errorf("source directory is required")
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", bundleSourcePayload{}, fmt.Errorf("stat source directory: %w", err)
	}
	if !info.IsDir() {
		return "", bundleSourcePayload{}, fmt.Errorf("source must be a directory containing manifest.json")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", bundleSourcePayload{}, fmt.Errorf("resolve source directory: %w", err)
	}
	payload, err := readBundleDirectoryPayload(absoluteRoot)
	if err != nil {
		return "", bundleSourcePayload{}, err
	}
	return absoluteRoot, payload, nil
}

func loadExtensionContract(root, overridePath string) (extensionContractFile, string, []string) {
	absolutePath, err := resolveExtensionContractPath(root, overridePath)
	if err != nil {
		return extensionContractFile{}, "", []string{fmt.Sprintf("resolve extension contract path: %v", err)}
	}

	data, err := os.ReadFile(absolutePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return extensionContractFile{}, absolutePath, []string{fmt.Sprintf("extension contract not found at %s", absolutePath)}
		}
		return extensionContractFile{}, absolutePath, []string{fmt.Sprintf("read extension contract: %v", err)}
	}

	var contract extensionContractFile
	if err := json.Unmarshal(data, &contract); err != nil {
		return extensionContractFile{}, absolutePath, []string{fmt.Sprintf("decode extension contract: %v", err)}
	}
	normalizeExtensionContract(&contract)

	problems := []string{}
	if contract.SchemaVersion != extensionContractSchemaV1 {
		problems = append(problems, fmt.Sprintf("extension contract schemaVersion must be %d", extensionContractSchemaV1))
	}
	problems = append(problems, validateExtensionContractShape(contract)...)
	if len(problems) > 0 {
		return extensionContractFile{}, absolutePath, problems
	}
	return contract, absolutePath, nil
}

func resolveExtensionContractPath(root, overridePath string) (string, error) {
	path := strings.TrimSpace(overridePath)
	if path == "" {
		path = filepath.Join(root, defaultExtensionContractFile)
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve extension contract path: %w", err)
	}
	return absolutePath, nil
}

func writeExtensionContract(path string, contract extensionContractFile) error {
	normalizeExtensionContract(&contract)
	data, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		return fmt.Errorf("encode extension contract: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write extension contract: %w", err)
	}
	return nil
}

func validateExtensionContractShape(contract extensionContractFile) []string {
	problems := []string{}
	for _, item := range contract.ResolvedAdminNavigation {
		if strings.TrimSpace(item.Title) == "" {
			problems = append(problems, "contract resolvedAdminNavigation title is required")
		}
		if strings.TrimSpace(item.Href) == "" {
			problems = append(problems, "contract resolvedAdminNavigation href is required")
		}
	}
	for _, widget := range contract.ResolvedDashboardWidgets {
		if strings.TrimSpace(widget.Title) == "" {
			problems = append(problems, "contract resolvedDashboardWidgets title is required")
		}
		if strings.TrimSpace(widget.Href) == "" {
			problems = append(problems, "contract resolvedDashboardWidgets href is required")
		}
	}
	return problems
}

func deriveExtensionContractFromManifest(manifest platformdomain.ExtensionManifest) extensionContractFile {
	contract := extensionContractFile{
		SchemaVersion:            extensionContractSchemaV1,
		ExtensionSlug:            strings.TrimSpace(manifest.Slug),
		ResolvedAdminNavigation:  deriveContractNavigationFromManifest(manifest),
		ResolvedDashboardWidgets: deriveContractWidgetsFromManifest(manifest),
		SeededQueues:             sortedUniqueStrings(contractSeededQueues(manifest)),
		SeededForms:              sortedUniqueStrings(contractSeededForms(manifest)),
		SeededAutomationRules:    sortedUniqueStrings(contractSeededAutomationRules(manifest)),
		PublicPaths:              sortedUniqueStrings(contractPublicPaths(manifest)),
		AdminPaths:               sortedUniqueStrings(contractAdminPaths(manifest)),
		HealthPaths:              sortedUniqueStrings(contractHealthPaths(manifest)),
		Commands:                 sortedUniqueStrings(contractCommands(manifest)),
		AgentSkills:              sortedUniqueStrings(contractAgentSkills(manifest)),
	}
	normalizeExtensionContract(&contract)
	return contract
}

func deriveExtensionContractFromDetail(detail extensionDetailOutput) extensionContractFile {
	contract := extensionContractFile{
		SchemaVersion:            extensionContractSchemaV1,
		ExtensionSlug:            strings.TrimSpace(detail.Slug),
		ResolvedAdminNavigation:  deriveContractNavigationFromDetail(detail),
		ResolvedDashboardWidgets: deriveContractWidgetsFromDetail(detail),
		SeededQueues:             sortedUniqueStrings(detailSeededQueueSlugs(detail)),
		SeededForms:              sortedUniqueStrings(detailSeededFormSlugs(detail)),
		SeededAutomationRules:    sortedUniqueStrings(detailSeededAutomationRuleKeys(detail)),
		PublicPaths:              sortedUniqueStrings(detailPublicPaths(detail)),
		AdminPaths:               sortedUniqueStrings(detailAdminPaths(detail)),
		HealthPaths:              sortedUniqueStrings(detailHealthPaths(detail)),
		Commands:                 sortedUniqueStrings(detailCommandNames(detail)),
		AgentSkills:              sortedUniqueStrings(detailAgentSkillNames(detail)),
	}
	normalizeExtensionContract(&contract)
	return contract
}

func compareExtensionContract(contract, actual extensionContractFile) []string {
	problems := []string{}
	if strings.TrimSpace(contract.ExtensionSlug) != "" && contract.ExtensionSlug != actual.ExtensionSlug {
		problems = append(problems, fmt.Sprintf("contract extensionSlug mismatch: expected %q, got %q", contract.ExtensionSlug, actual.ExtensionSlug))
	}

	if len(actual.ResolvedAdminNavigation) > 0 && contract.ResolvedAdminNavigation == nil {
		problems = append(problems, "contract must declare resolvedAdminNavigation")
	} else if contract.ResolvedAdminNavigation != nil && !reflect.DeepEqual(contract.ResolvedAdminNavigation, actual.ResolvedAdminNavigation) {
		problems = append(problems, fmt.Sprintf("contract resolvedAdminNavigation mismatch: expected %s, got %s", mustJSONString(contract.ResolvedAdminNavigation), mustJSONString(actual.ResolvedAdminNavigation)))
	}

	if len(actual.ResolvedDashboardWidgets) > 0 && contract.ResolvedDashboardWidgets == nil {
		problems = append(problems, "contract must declare resolvedDashboardWidgets")
	} else if contract.ResolvedDashboardWidgets != nil && !reflect.DeepEqual(contract.ResolvedDashboardWidgets, actual.ResolvedDashboardWidgets) {
		problems = append(problems, fmt.Sprintf("contract resolvedDashboardWidgets mismatch: expected %s, got %s", mustJSONString(contract.ResolvedDashboardWidgets), mustJSONString(actual.ResolvedDashboardWidgets)))
	}

	problems = append(problems, compareContractStringSlice("seededQueues", contract.SeededQueues, actual.SeededQueues)...)
	problems = append(problems, compareContractStringSlice("seededForms", contract.SeededForms, actual.SeededForms)...)
	problems = append(problems, compareContractStringSlice("seededAutomationRules", contract.SeededAutomationRules, actual.SeededAutomationRules)...)
	problems = append(problems, compareContractStringSlice("publicPaths", contract.PublicPaths, actual.PublicPaths)...)
	problems = append(problems, compareContractStringSlice("adminPaths", contract.AdminPaths, actual.AdminPaths)...)
	problems = append(problems, compareContractStringSlice("healthPaths", contract.HealthPaths, actual.HealthPaths)...)
	problems = append(problems, compareContractStringSlice("commands", contract.Commands, actual.Commands)...)
	problems = append(problems, compareContractStringSlice("agentSkills", contract.AgentSkills, actual.AgentSkills)...)
	return problems
}

func compareContractStringSlice(name string, expected, actual []string) []string {
	switch {
	case len(actual) > 0 && expected == nil:
		return []string{fmt.Sprintf("contract must declare %s", name)}
	case expected != nil && !slices.Equal(expected, actual):
		return []string{fmt.Sprintf("contract %s mismatch: expected %s, got %s", name, mustJSONString(expected), mustJSONString(actual))}
	default:
		return nil
	}
}

func normalizeExtensionContract(contract *extensionContractFile) {
	contract.ExtensionSlug = strings.TrimSpace(contract.ExtensionSlug)
	if contract.ResolvedAdminNavigation != nil {
		for i := range contract.ResolvedAdminNavigation {
			contract.ResolvedAdminNavigation[i].Title = strings.TrimSpace(contract.ResolvedAdminNavigation[i].Title)
			contract.ResolvedAdminNavigation[i].Href = strings.TrimSpace(contract.ResolvedAdminNavigation[i].Href)
			contract.ResolvedAdminNavigation[i].Section = normalizeOptionalString(contract.ResolvedAdminNavigation[i].Section)
			contract.ResolvedAdminNavigation[i].Icon = normalizeOptionalString(contract.ResolvedAdminNavigation[i].Icon)
			contract.ResolvedAdminNavigation[i].ActivePage = normalizeOptionalString(contract.ResolvedAdminNavigation[i].ActivePage)
		}
		slices.SortFunc(contract.ResolvedAdminNavigation, func(left, right extensionContractNavigationItem) int {
			return strings.Compare(contractNavigationSortKey(left), contractNavigationSortKey(right))
		})
	}
	if contract.ResolvedDashboardWidgets != nil {
		for i := range contract.ResolvedDashboardWidgets {
			contract.ResolvedDashboardWidgets[i].Title = strings.TrimSpace(contract.ResolvedDashboardWidgets[i].Title)
			contract.ResolvedDashboardWidgets[i].Href = strings.TrimSpace(contract.ResolvedDashboardWidgets[i].Href)
			contract.ResolvedDashboardWidgets[i].Description = normalizeOptionalString(contract.ResolvedDashboardWidgets[i].Description)
			contract.ResolvedDashboardWidgets[i].Icon = normalizeOptionalString(contract.ResolvedDashboardWidgets[i].Icon)
		}
		slices.SortFunc(contract.ResolvedDashboardWidgets, func(left, right extensionContractDashboardWidget) int {
			return strings.Compare(contractWidgetSortKey(left), contractWidgetSortKey(right))
		})
	}
	contract.SeededQueues = normalizeContractStringSlice(contract.SeededQueues)
	contract.SeededForms = normalizeContractStringSlice(contract.SeededForms)
	contract.SeededAutomationRules = normalizeContractStringSlice(contract.SeededAutomationRules)
	contract.PublicPaths = normalizeContractStringSlice(contract.PublicPaths)
	contract.AdminPaths = normalizeContractStringSlice(contract.AdminPaths)
	contract.HealthPaths = normalizeContractStringSlice(contract.HealthPaths)
	contract.Commands = normalizeContractStringSlice(contract.Commands)
	contract.AgentSkills = normalizeContractStringSlice(contract.AgentSkills)
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeContractStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	return sortedUniqueStrings(values)
}

func contractNavigationSortKey(item extensionContractNavigationItem) string {
	return strings.Join([]string{
		coalesce(item.Section, ""),
		item.Title,
		coalesce(item.Icon, ""),
		item.Href,
		coalesce(item.ActivePage, ""),
	}, "\x00")
}

func contractWidgetSortKey(item extensionContractDashboardWidget) string {
	return strings.Join([]string{
		item.Title,
		coalesce(item.Description, ""),
		coalesce(item.Icon, ""),
		item.Href,
	}, "\x00")
}

func deriveContractNavigationFromManifest(manifest platformdomain.ExtensionManifest) []extensionContractNavigationItem {
	endpoints := manifestEndpointMountPaths(manifest)
	items := make([]extensionContractNavigationItem, 0, len(manifest.AdminNavigation))
	for _, item := range manifest.AdminNavigation {
		href, ok := endpoints[item.Endpoint]
		if !ok {
			continue
		}
		items = append(items, extensionContractNavigationItem{
			Section:    nullableString(item.Section),
			Title:      item.Title,
			Icon:       nullableString(item.Icon),
			Href:       href,
			ActivePage: nullableString(item.ActivePage),
		})
	}
	return items
}

func deriveContractWidgetsFromManifest(manifest platformdomain.ExtensionManifest) []extensionContractDashboardWidget {
	endpoints := manifestEndpointMountPaths(manifest)
	items := make([]extensionContractDashboardWidget, 0, len(manifest.DashboardWidgets))
	for _, widget := range manifest.DashboardWidgets {
		href, ok := endpoints[widget.Endpoint]
		if !ok {
			continue
		}
		items = append(items, extensionContractDashboardWidget{
			Title:       widget.Title,
			Description: nullableString(widget.Description),
			Icon:        nullableString(widget.Icon),
			Href:        href,
		})
	}
	return items
}

func manifestEndpointMountPaths(manifest platformdomain.ExtensionManifest) map[string]string {
	endpoints := make(map[string]string, len(manifest.Endpoints))
	for _, endpoint := range manifest.Endpoints {
		if endpoint.Class != platformdomain.ExtensionEndpointClassAdminPage {
			continue
		}
		endpoints[endpoint.Name] = endpoint.MountPath
	}
	return endpoints
}

func contractSeededQueues(manifest platformdomain.ExtensionManifest) []string {
	items := make([]string, 0, len(manifest.Queues))
	for _, queue := range manifest.Queues {
		items = append(items, queue.Slug)
	}
	return items
}

func contractSeededForms(manifest platformdomain.ExtensionManifest) []string {
	items := make([]string, 0, len(manifest.Forms))
	for _, form := range manifest.Forms {
		items = append(items, form.Slug)
	}
	return items
}

func contractSeededAutomationRules(manifest platformdomain.ExtensionManifest) []string {
	items := make([]string, 0, len(manifest.AutomationRules))
	for _, rule := range manifest.AutomationRules {
		items = append(items, rule.Key)
	}
	return items
}

func contractPublicPaths(manifest platformdomain.ExtensionManifest) []string {
	items := []string{}
	for _, route := range manifest.PublicRoutes {
		items = append(items, route.PathPrefix)
	}
	for _, endpoint := range manifest.Endpoints {
		switch endpoint.Class {
		case platformdomain.ExtensionEndpointClassPublicPage,
			platformdomain.ExtensionEndpointClassPublicAsset,
			platformdomain.ExtensionEndpointClassPublicIngest,
			platformdomain.ExtensionEndpointClassWebhook:
			items = append(items, endpoint.MountPath)
		}
	}
	return items
}

func contractAdminPaths(manifest platformdomain.ExtensionManifest) []string {
	items := []string{}
	for _, route := range manifest.AdminRoutes {
		items = append(items, route.PathPrefix)
	}
	for _, endpoint := range manifest.Endpoints {
		switch endpoint.Class {
		case platformdomain.ExtensionEndpointClassAdminPage,
			platformdomain.ExtensionEndpointClassAdminAction,
			platformdomain.ExtensionEndpointClassExtensionAPI:
			items = append(items, endpoint.MountPath)
		}
	}
	return items
}

func contractHealthPaths(manifest platformdomain.ExtensionManifest) []string {
	items := []string{}
	for _, endpoint := range manifest.Endpoints {
		if endpoint.Class == platformdomain.ExtensionEndpointClassHealth {
			items = append(items, endpoint.MountPath)
		}
	}
	return items
}

func contractCommands(manifest platformdomain.ExtensionManifest) []string {
	items := make([]string, 0, len(manifest.Commands))
	for _, command := range manifest.Commands {
		items = append(items, command.Name)
	}
	return items
}

func contractAgentSkills(manifest platformdomain.ExtensionManifest) []string {
	items := make([]string, 0, len(manifest.AgentSkills))
	for _, skill := range manifest.AgentSkills {
		items = append(items, skill.Name)
	}
	return items
}

func deriveContractNavigationFromDetail(detail extensionDetailOutput) []extensionContractNavigationItem {
	items := make([]extensionContractNavigationItem, 0, len(detail.ResolvedAdminNavigation))
	for _, item := range detail.ResolvedAdminNavigation {
		items = append(items, extensionContractNavigationItem{
			Section:    item.Section,
			Title:      item.Title,
			Icon:       item.Icon,
			Href:       item.Href,
			ActivePage: item.ActivePage,
		})
	}
	return items
}

func deriveContractWidgetsFromDetail(detail extensionDetailOutput) []extensionContractDashboardWidget {
	items := make([]extensionContractDashboardWidget, 0, len(detail.ResolvedDashboardWidgets))
	for _, widget := range detail.ResolvedDashboardWidgets {
		items = append(items, extensionContractDashboardWidget{
			Title:       widget.Title,
			Description: widget.Description,
			Icon:        widget.Icon,
			Href:        widget.Href,
		})
	}
	return items
}

func detailSeededQueueSlugs(detail extensionDetailOutput) []string {
	items := make([]string, 0, len(detail.SeededResources.Queues))
	for _, queue := range detail.SeededResources.Queues {
		items = append(items, queue.Slug)
	}
	return items
}

func detailSeededFormSlugs(detail extensionDetailOutput) []string {
	items := make([]string, 0, len(detail.SeededResources.Forms))
	for _, form := range detail.SeededResources.Forms {
		items = append(items, form.Slug)
	}
	return items
}

func detailSeededAutomationRuleKeys(detail extensionDetailOutput) []string {
	items := make([]string, 0, len(detail.SeededResources.AutomationRules))
	for _, rule := range detail.SeededResources.AutomationRules {
		items = append(items, rule.Key)
	}
	return items
}

func detailPublicPaths(detail extensionDetailOutput) []string {
	items := []string{}
	for _, route := range detail.PublicRoutes {
		items = append(items, route.PathPrefix)
	}
	for _, endpoint := range detail.Endpoints {
		switch endpoint.Class {
		case string(platformdomain.ExtensionEndpointClassPublicPage),
			string(platformdomain.ExtensionEndpointClassPublicAsset),
			string(platformdomain.ExtensionEndpointClassPublicIngest),
			string(platformdomain.ExtensionEndpointClassWebhook):
			items = append(items, endpoint.MountPath)
		}
	}
	return items
}

func detailAdminPaths(detail extensionDetailOutput) []string {
	items := []string{}
	for _, route := range detail.AdminRoutes {
		items = append(items, route.PathPrefix)
	}
	for _, endpoint := range detail.Endpoints {
		switch endpoint.Class {
		case string(platformdomain.ExtensionEndpointClassAdminPage),
			string(platformdomain.ExtensionEndpointClassAdminAction),
			string(platformdomain.ExtensionEndpointClassExtensionAPI):
			items = append(items, endpoint.MountPath)
		}
	}
	return items
}

func detailHealthPaths(detail extensionDetailOutput) []string {
	items := []string{}
	for _, endpoint := range detail.Endpoints {
		if endpoint.Class == string(platformdomain.ExtensionEndpointClassHealth) {
			items = append(items, endpoint.MountPath)
		}
	}
	return items
}

func detailCommandNames(detail extensionDetailOutput) []string {
	items := make([]string, 0, len(detail.Commands))
	for _, command := range detail.Commands {
		items = append(items, command.Name)
	}
	return items
}

func detailAgentSkillNames(detail extensionDetailOutput) []string {
	items := make([]string, 0, len(detail.AgentSkills))
	for _, skill := range detail.AgentSkills {
		items = append(items, skill.Name)
	}
	return items
}

func sortedUniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	slices.Sort(items)
	if len(items) == 0 {
		return []string{}
	}
	return items
}

func mustJSONString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

func nullableString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func bundleAssetInputs(source bundleSourcePayload) []platformservices.ExtensionAssetInput {
	assets := make([]platformservices.ExtensionAssetInput, 0, len(source.Bundle.Assets))
	for _, asset := range source.Bundle.Assets {
		assets = append(assets, platformservices.ExtensionAssetInput{
			Path:           asset.Path,
			ContentType:    asset.ContentType,
			Content:        []byte(asset.Content),
			IsCustomizable: asset.IsCustomizable,
		})
	}
	return assets
}

func verifyPreparedExtensionSource(ctx context.Context, prepared preparedExtensionSource, client *cliapi.Client, cfg cliapi.Config, requestedWorkspaceID, licenseToken string) (extensionVerifyOutput, error) {
	if !prepared.lint.ManifestValid || !prepared.lint.ContractValid {
		return extensionVerifyOutput{}, fmt.Errorf("extension source failed lint: %s", strings.Join(prepared.lint.Problems, "; "))
	}

	verifyOutput := extensionVerifyOutput{
		Source:       prepared.root,
		ContractPath: prepared.contractPath,
		Lint:         prepared.lint,
	}

	resolvedWorkspaceID := ""
	var provisionedWorkspace *workspaceOutput
	if prepared.manifest.Scope == platformdomain.ExtensionScopeWorkspace {
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			return extensionVerifyOutput{}, err
		}
		var errResolve error
		resolvedWorkspaceID, provisionedWorkspace, errResolve = resolveInstallWorkspace(ctx, cfg, resolveStoredWorkspaceID(requestedWorkspaceID, stored), prepared.bundle)
		if errResolve != nil {
			return extensionVerifyOutput{}, errResolve
		}
		verifyOutput.WorkspaceID = &resolvedWorkspaceID
		verifyOutput.ProvisionedWorkspace = provisionedWorkspace
	} else if strings.TrimSpace(requestedWorkspaceID) != "" {
		return extensionVerifyOutput{}, fmt.Errorf("--workspace cannot be used with instance-scoped extensions")
	}

	existing, err := findInstalledExtensionBySlug(ctx, client, resolvedWorkspaceID, prepared.manifest.Scope == platformdomain.ExtensionScopeInstance, prepared.manifest.Slug)
	if err != nil {
		return extensionVerifyOutput{}, err
	}

	verifyOutput.Operation = "install"
	if existing != nil {
		verifyOutput.Operation = "upgrade"
		verifyOutput.Installed, err = runExtensionUpgrade(ctx, client, existing.ID, licenseToken, prepared.bundle)
	} else {
		verifyOutput.Installed, err = runExtensionInstall(ctx, client, resolvedWorkspaceID, licenseToken, prepared.bundle)
	}
	if err != nil {
		return extensionVerifyOutput{}, err
	}

	verifyOutput.Validated, err = runExtensionAction(ctx, client, "validate", verifyOutput.Installed.ID, "")
	if err != nil {
		return extensionVerifyOutput{}, err
	}
	verifyOutput.Activated, err = runExtensionAction(ctx, client, "activate", verifyOutput.Installed.ID, "")
	if err != nil {
		return extensionVerifyOutput{}, err
	}
	verifyOutput.Monitored, err = runExtensionAction(ctx, client, "checkHealth", verifyOutput.Installed.ID, "")
	if err != nil {
		return extensionVerifyOutput{}, err
	}

	verifyOutput.Detail, err = fetchExtensionDetail(ctx, client, verifyOutput.Installed.ID)
	if err != nil {
		return extensionVerifyOutput{}, err
	}
	if verifyOutput.Monitored.HealthStatus != "" {
		verifyOutput.Detail.HealthStatus = verifyOutput.Monitored.HealthStatus
		verifyOutput.Detail.HealthMessage = verifyOutput.Monitored.HealthMessage
	}

	if prepared.manifest.Scope == platformdomain.ExtensionScopeWorkspace {
		verifyOutput.WorkspaceResolvedAdminNavigation, err = fetchResolvedExtensionAdminNavigation(ctx, client, resolvedWorkspaceID, false)
		if err != nil {
			return extensionVerifyOutput{}, err
		}
		verifyOutput.WorkspaceResolvedDashboardWidgets, err = fetchResolvedExtensionDashboardWidgets(ctx, client, resolvedWorkspaceID, false)
		if err != nil {
			return extensionVerifyOutput{}, err
		}
	} else {
		verifyOutput.WorkspaceResolvedAdminNavigation, err = fetchResolvedExtensionAdminNavigation(ctx, client, "", true)
		if err != nil {
			return extensionVerifyOutput{}, err
		}
		verifyOutput.WorkspaceResolvedDashboardWidgets, err = fetchResolvedExtensionDashboardWidgets(ctx, client, "", true)
		if err != nil {
			return extensionVerifyOutput{}, err
		}
	}
	verifyOutput.InstanceResolvedAdminNavigation, err = fetchResolvedExtensionAdminNavigation(ctx, client, "", true)
	if err != nil {
		return extensionVerifyOutput{}, err
	}
	verifyOutput.InstanceResolvedDashboardWidgets, err = fetchResolvedExtensionDashboardWidgets(ctx, client, "", true)
	if err != nil {
		return extensionVerifyOutput{}, err
	}

	problems := compareExtensionContract(prepared.contract, deriveExtensionContractFromDetail(verifyOutput.Detail))
	problems = append(problems, verifyWorkspaceNavigation(prepared.contract, verifyOutput.Detail.Slug, verifyOutput.WorkspaceResolvedAdminNavigation)...)
	problems = append(problems, verifyWorkspaceWidgets(prepared.contract, verifyOutput.Detail.Slug, verifyOutput.WorkspaceResolvedDashboardWidgets)...)
	problems = append(problems, verifyInstanceNavigation(prepared.contract, verifyOutput.Detail.Slug, resolvedWorkspaceID, verifyOutput.InstanceResolvedAdminNavigation)...)
	problems = append(problems, verifyInstanceWidgets(prepared.contract, verifyOutput.Detail.Slug, resolvedWorkspaceID, verifyOutput.InstanceResolvedDashboardWidgets)...)
	problems = append(problems, verifySeededResourcesHealthy(verifyOutput.Detail)...)
	if verifyOutput.Validated.ValidationStatus != string(platformdomain.ExtensionValidationValid) {
		problems = append(problems, fmt.Sprintf("validation status is %s", verifyOutput.Validated.ValidationStatus))
	}
	if verifyOutput.Detail.HealthStatus != string(platformdomain.ExtensionHealthHealthy) {
		problems = append(problems, fmt.Sprintf("health status is %s", verifyOutput.Detail.HealthStatus))
	}
	if len(problems) > 0 {
		return extensionVerifyOutput{}, fmt.Errorf("extension verification failed: %s", strings.Join(problems, "; "))
	}

	return verifyOutput, nil
}

func verifyWorkspaceNavigation(contract extensionContractFile, slug string, items []resolvedExtensionAdminNavigationItemOutput) []string {
	if contract.ResolvedAdminNavigation == nil {
		return nil
	}
	actual := make([]extensionContractNavigationItem, 0)
	for _, item := range items {
		if item.ExtensionSlug != slug {
			continue
		}
		actual = append(actual, extensionContractNavigationItem{
			Section:    item.Section,
			Title:      item.Title,
			Icon:       item.Icon,
			Href:       item.Href,
			ActivePage: item.ActivePage,
		})
	}
	normalizeExtensionContract(&extensionContractFile{ResolvedAdminNavigation: actual})
	slices.SortFunc(actual, func(left, right extensionContractNavigationItem) int {
		return strings.Compare(contractNavigationSortKey(left), contractNavigationSortKey(right))
	})
	if !reflect.DeepEqual(contract.ResolvedAdminNavigation, actual) {
		return []string{fmt.Sprintf("workspace resolvedAdminNavigation mismatch: expected %s, got %s", mustJSONString(contract.ResolvedAdminNavigation), mustJSONString(actual))}
	}
	return nil
}

func verifyWorkspaceWidgets(contract extensionContractFile, slug string, items []resolvedExtensionDashboardWidgetOutput) []string {
	if contract.ResolvedDashboardWidgets == nil {
		return nil
	}
	actual := make([]extensionContractDashboardWidget, 0)
	for _, item := range items {
		if item.ExtensionSlug != slug {
			continue
		}
		actual = append(actual, extensionContractDashboardWidget{
			Title:       item.Title,
			Description: item.Description,
			Icon:        item.Icon,
			Href:        item.Href,
		})
	}
	slices.SortFunc(actual, func(left, right extensionContractDashboardWidget) int {
		return strings.Compare(contractWidgetSortKey(left), contractWidgetSortKey(right))
	})
	if !reflect.DeepEqual(contract.ResolvedDashboardWidgets, actual) {
		return []string{fmt.Sprintf("workspace resolvedDashboardWidgets mismatch: expected %s, got %s", mustJSONString(contract.ResolvedDashboardWidgets), mustJSONString(actual))}
	}
	return nil
}

func verifyInstanceNavigation(contract extensionContractFile, slug, workspaceID string, items []resolvedExtensionAdminNavigationItemOutput) []string {
	if contract.ResolvedAdminNavigation == nil {
		return nil
	}
	actual := make([]extensionContractNavigationItem, 0)
	problems := []string{}
	for _, item := range items {
		if item.ExtensionSlug != slug {
			continue
		}
		if strings.TrimSpace(workspaceID) != "" {
			if strings.TrimSpace(coalesce(item.WorkspaceID, "")) != workspaceID {
				continue
			}
			if hint := extensionHrefWorkspaceHint(item.Href); hint != workspaceID {
				problems = append(problems, fmt.Sprintf("instance admin navigation href %q must include workspace=%s", item.Href, workspaceID))
			}
		}
		actual = append(actual, extensionContractNavigationItem{
			Section:    item.Section,
			Title:      item.Title,
			Icon:       item.Icon,
			Href:       stripExtensionHrefWorkspaceHint(item.Href),
			ActivePage: item.ActivePage,
		})
	}
	slices.SortFunc(actual, func(left, right extensionContractNavigationItem) int {
		return strings.Compare(contractNavigationSortKey(left), contractNavigationSortKey(right))
	})
	if !reflect.DeepEqual(contract.ResolvedAdminNavigation, actual) {
		problems = append(problems, fmt.Sprintf("instance admin resolvedAdminNavigation mismatch: expected %s, got %s", mustJSONString(contract.ResolvedAdminNavigation), mustJSONString(actual)))
	}
	return problems
}

func verifyInstanceWidgets(contract extensionContractFile, slug, workspaceID string, items []resolvedExtensionDashboardWidgetOutput) []string {
	if contract.ResolvedDashboardWidgets == nil {
		return nil
	}
	actual := make([]extensionContractDashboardWidget, 0)
	problems := []string{}
	for _, item := range items {
		if item.ExtensionSlug != slug {
			continue
		}
		if strings.TrimSpace(workspaceID) != "" {
			if strings.TrimSpace(coalesce(item.WorkspaceID, "")) != workspaceID {
				continue
			}
			if hint := extensionHrefWorkspaceHint(item.Href); hint != workspaceID {
				problems = append(problems, fmt.Sprintf("instance admin widget href %q must include workspace=%s", item.Href, workspaceID))
			}
		}
		actual = append(actual, extensionContractDashboardWidget{
			Title:       item.Title,
			Description: item.Description,
			Icon:        item.Icon,
			Href:        stripExtensionHrefWorkspaceHint(item.Href),
		})
	}
	slices.SortFunc(actual, func(left, right extensionContractDashboardWidget) int {
		return strings.Compare(contractWidgetSortKey(left), contractWidgetSortKey(right))
	})
	if !reflect.DeepEqual(contract.ResolvedDashboardWidgets, actual) {
		problems = append(problems, fmt.Sprintf("instance admin resolvedDashboardWidgets mismatch: expected %s, got %s", mustJSONString(contract.ResolvedDashboardWidgets), mustJSONString(actual)))
	}
	return problems
}

func extensionHrefWorkspaceHint(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	for _, key := range []string{"workspace", "workspace_id", "workspaceID"} {
		if value := strings.TrimSpace(parsed.Query().Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func stripExtensionHrefWorkspaceHint(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	query := parsed.Query()
	query.Del("workspace")
	query.Del("workspace_id")
	query.Del("workspaceID")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func verifySeededResourcesHealthy(detail extensionDetailOutput) []string {
	problems := []string{}
	for _, queue := range detail.SeededResources.Queues {
		if !queue.Exists {
			problems = append(problems, fmt.Sprintf("seeded queue %s is missing", queue.Slug))
			continue
		}
		if !queue.MatchesSeed {
			problems = append(problems, fmt.Sprintf("seeded queue %s drifted: %s", queue.Slug, strings.Join(queue.Problems, ", ")))
		}
	}
	for _, form := range detail.SeededResources.Forms {
		if !form.Exists {
			problems = append(problems, fmt.Sprintf("seeded form %s is missing", form.Slug))
			continue
		}
		if !form.MatchesSeed {
			problems = append(problems, fmt.Sprintf("seeded form %s drifted: %s", form.Slug, strings.Join(form.Problems, ", ")))
		}
	}
	for _, rule := range detail.SeededResources.AutomationRules {
		if !rule.Exists {
			problems = append(problems, fmt.Sprintf("seeded automation rule %s is missing", rule.Key))
			continue
		}
		if !rule.MatchesSeed {
			problems = append(problems, fmt.Sprintf("seeded automation rule %s drifted: %s", rule.Key, strings.Join(rule.Problems, ", ")))
		}
	}
	return problems
}
