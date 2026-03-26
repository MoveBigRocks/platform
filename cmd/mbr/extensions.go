package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/cliapi"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

func runExtensions(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printExtensionsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr extensions list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		instanceScope := fs.Bool("instance", false, "List instance-scoped extensions")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)
		if *instanceScope && strings.TrimSpace(*workspaceID) != "" {
			fmt.Fprintln(stderr, "--workspace and --instance cannot be used together")
			return 2
		}
		if !*instanceScope && workspaceValue == "" {
			fmt.Fprintln(stderr, "--workspace is required unless --instance is set")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		var payload struct {
			Extensions         []extensionOutput `json:"extensions"`
			InstanceExtensions []extensionOutput `json:"instanceExtensions"`
		}
		query := `
			query CLIExtensions($workspaceID: ID!) {
			  extensions(workspaceID: $workspaceID) {
			    id
			    workspaceID
			    slug
			    name
			    publisher
			    version
			    kind
			    scope
			    risk
			    status
			    validationStatus
			    validationMessage
			    healthStatus
			    healthMessage
			  }
			}
		`
		variables := map[string]any{"workspaceID": workspaceValue}
		if *instanceScope {
			query = `
				query CLIInstanceExtensions {
				  instanceExtensions {
				    id
				    workspaceID
				    slug
				    name
				    publisher
				    version
				    kind
				    scope
				    risk
				    status
				    validationStatus
				    validationMessage
				    healthStatus
				    healthMessage
				  }
				}
			`
			variables = nil
		}
		err = client.Query(ctx, query, variables, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		extensions := payload.Extensions
		if *instanceScope {
			extensions = payload.InstanceExtensions
		}

		if *jsonOutput {
			return writeJSON(stdout, extensions, stderr)
		}
		if len(extensions) == 0 {
			fmt.Fprintln(stdout, "no extensions installed")
			return 0
		}
		for _, extension := range extensions {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\n", extension.ID, extension.Slug, extension.Version, extension.Status, extension.HealthStatus)
		}
		return 0
	case "show":
		return runExtensionInspect(ctx, args[1:], stdout, stderr)
	case "monitor":
		return runExtensionMonitor(ctx, args[1:], stdout, stderr)
	case "events":
		return runExtensionEvents(ctx, args[1:], stdout, stderr)
	case "skill", "skills":
		return runExtensionSkill(ctx, args[1:], stdout, stderr)
	case "deploy":
		fs := flag.NewFlagSet("mbr extensions deploy", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		licenseToken := fs.String("license-token", "", "Optional extension install credential")
		configPath := fs.String("config-file", "", "Path to JSON config file")
		configJSON := fs.String("config-json", "", "Inline JSON config object")
		skipActivate := fs.Bool("no-activate", false, "Install and validate without activating")
		skipMonitor := fs.Bool("no-monitor", false, "Skip the health-check pass after deployment")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":           true,
			"--api-url":       true,
			"--token":         true,
			"--workspace":     true,
			"--license-token": true,
			"--config-file":   true,
			"--config-json":   true,
			"--no-activate":   false,
			"--no-monitor":    false,
			"--json":          false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "bundle source is required")
			return 2
		}

		bundleSource, err := readBundleSourcePayload(ctx, positionals[0], *licenseToken)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		manifest, err := decodeBundleManifest(bundleSource.Bundle.Manifest)
		if err != nil {
			fmt.Fprintf(stderr, "decode extension manifest: %v\n", err)
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		resolvedWorkspaceID, provisionedWorkspace, err := resolveInstallWorkspace(ctx, cfg, resolveStoredWorkspaceID(*workspaceID, stored), bundleSource)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		existing, err := findInstalledExtensionBySlug(ctx, client, resolvedWorkspaceID, manifest.Scope == platformdomain.ExtensionScopeInstance, manifest.Slug)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		operation := "install"
		var result extensionOutput
		if existing != nil {
			operation = "upgrade"
			result, err = runExtensionUpgrade(ctx, client, existing.ID, *licenseToken, bundleSource)
		} else {
			result, err = runExtensionInstall(ctx, client, resolvedWorkspaceID, *licenseToken, bundleSource)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		configured := false
		if strings.TrimSpace(*configPath) != "" || strings.TrimSpace(*configJSON) != "" {
			config, err := readConfigInput(*configPath, *configJSON)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			result, err = runExtensionConfigure(ctx, client, result.ID, config)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			configured = true
		}

		result, err = runExtensionAction(ctx, client, "validate", result.ID, "")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		activated := false
		if !*skipActivate {
			result, err = runExtensionAction(ctx, client, "activate", result.ID, "")
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			activated = true
		}

		monitored := false
		if !*skipMonitor {
			result, err = runExtensionAction(ctx, client, "checkHealth", result.ID, "")
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			monitored = true
		}

		payload := map[string]any{
			"operation":  operation,
			"configured": configured,
			"activated":  activated,
			"monitored":  monitored,
			"extension":  result,
		}
		if provisionedWorkspace != nil {
			payload["provisionedWorkspace"] = provisionedWorkspace
		}
		if *jsonOutput {
			return writeJSON(stdout, payload, stderr)
		}
		if provisionedWorkspace != nil {
			fmt.Fprintf(stdout, "workspace\t%s\t%s\t%s\n", provisionedWorkspace.ID, provisionedWorkspace.Name, provisionedWorkspace.ShortCode)
		}
		fmt.Fprintf(stdout, "operation:\t%s\n", operation)
		fmt.Fprintf(stdout, "configured:\t%t\n", configured)
		fmt.Fprintf(stdout, "activated:\t%t\n", activated)
		fmt.Fprintf(stdout, "monitored:\t%t\n", monitored)
		fmt.Fprintf(stdout, "extension:\t%s\t%s\t%s\t%s\t%s\n", result.ID, result.Slug, result.Version, result.Status, result.HealthStatus)
		return 0
	case "install":
		fs := flag.NewFlagSet("mbr extensions install", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		licenseToken := fs.String("license-token", "", "Optional extension install credential")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":           true,
			"--api-url":       true,
			"--token":         true,
			"--workspace":     true,
			"--license-token": true,
			"--json":          false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "bundle source is required")
			return 2
		}

		bundleSource, err := readBundleSourcePayload(ctx, positionals[0], *licenseToken)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		resolvedWorkspaceID, provisionedWorkspace, err := resolveInstallWorkspace(ctx, cfg, resolveStoredWorkspaceID(*workspaceID, stored), bundleSource)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		result, err := runExtensionInstall(ctx, client, resolvedWorkspaceID, *licenseToken, bundleSource)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			payload := map[string]any{"extension": result}
			if provisionedWorkspace != nil {
				payload["provisionedWorkspace"] = provisionedWorkspace
			}
			return writeJSON(stdout, payload, stderr)
		}
		if provisionedWorkspace != nil {
			fmt.Fprintf(stdout, "workspace\t%s\t%s\t%s\n", provisionedWorkspace.ID, provisionedWorkspace.Name, provisionedWorkspace.ShortCode)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\n", result.ID, result.Slug, result.Version, result.Status, result.HealthStatus)
		return 0
	case "upgrade":
		fs := flag.NewFlagSet("mbr extensions upgrade", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		id := fs.String("id", "", "Extension ID")
		licenseToken := fs.String("license-token", "", "Optional replacement extension install credential")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":           true,
			"--api-url":       true,
			"--token":         true,
			"--id":            true,
			"--license-token": true,
			"--json":          false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if strings.TrimSpace(*id) == "" {
			fmt.Fprintln(stderr, "--id is required")
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "bundle source is required")
			return 2
		}

		bundleSource, err := readBundleSourcePayload(ctx, positionals[0], *licenseToken)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		result, err := runExtensionUpgrade(ctx, client, *id, *licenseToken, bundleSource)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\n", result.ID, result.Slug, result.Version, result.Status, result.HealthStatus)
		return 0
	case "configure":
		fs := flag.NewFlagSet("mbr extensions configure", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		id := fs.String("id", "", "Extension ID")
		configPath := fs.String("config-file", "", "Path to JSON config file")
		configJSON := fs.String("config-json", "", "Inline JSON config object")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*id) == "" {
			fmt.Fprintln(stderr, "--id is required")
			return 2
		}

		config, err := readConfigInput(*configPath, *configJSON)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		result, err := runExtensionConfigure(ctx, client, *id, config)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\n", result.ID, result.Slug, result.Version, result.Status, result.HealthStatus)
		return 0
	case "validate":
		return runExtensionMutation(ctx, args[1:], stdout, stderr, "validate")
	case "activate":
		return runExtensionMutation(ctx, args[1:], stdout, stderr, "activate")
	case "deactivate":
		return runExtensionMutation(ctx, args[1:], stdout, stderr, "deactivate")
	case "uninstall":
		return runExtensionUninstall(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown extensions command %q\n\n", args[0])
		printExtensionsUsage(stderr)
		return 2
	}
}

func runExtensionSkill(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printExtensionSkillsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr extensions skills list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		id := fs.String("id", "", "Extension ID")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*id) == "" {
			fmt.Fprintln(stderr, "--id is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		skills, err := fetchExtensionSkills(ctx, client, *id)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, skills, stderr)
		}
		if len(skills) == 0 {
			fmt.Fprintln(stdout, "no extension skills declared")
			return 0
		}
		for _, skill := range skills {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", skill.Name, coalesce(skill.Description, ""), skill.AssetPath)
		}
		return 0
	case "show":
		fs := flag.NewFlagSet("mbr extensions skills show", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		id := fs.String("id", "", "Extension ID")
		name := fs.String("name", "", "Skill name")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*id) == "" {
			fmt.Fprintln(stderr, "--id is required")
			return 2
		}
		if strings.TrimSpace(*name) == "" {
			fmt.Fprintln(stderr, "--name is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		skill, err := fetchExtensionSkill(ctx, client, *id, *name)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, skill, stderr)
		}
		if _, err := io.WriteString(stdout, skill.Content); err != nil {
			fmt.Fprintf(stderr, "write output: %v\n", err)
			return 1
		}
		if !strings.HasSuffix(skill.Content, "\n") {
			if _, err := io.WriteString(stdout, "\n"); err != nil {
				fmt.Fprintf(stderr, "write output: %v\n", err)
				return 1
			}
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown extensions skills command %q\n\n", args[0])
		printExtensionSkillsUsage(stderr)
		return 2
	}
}

func runExtensionEvents(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printExtensionEventsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr extensions events list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue, err := requireWorkspaceID(*workspaceID, stored)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		events, err := fetchExtensionEventCatalog(ctx, client, workspaceValue)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, events, stderr)
		}
		if len(events) == 0 {
			fmt.Fprintln(stdout, "no extension runtime events registered")
			return 0
		}
		for _, event := range events {
			fmt.Fprintf(stdout, "%s\tv%d\t%t\t%s\t%s\n",
				event.Type,
				event.SchemaVersion,
				event.Core,
				strings.Join(event.Publishers, ","),
				strings.Join(event.Subscribers, ","),
			)
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown extension events command %q\n\n", args[0])
		printExtensionEventsUsage(stderr)
		return 2
	}
}

func runExtensionMutation(ctx context.Context, args []string, stdout, stderr io.Writer, action string) int {
	fs := flag.NewFlagSet("mbr extensions "+action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	instanceURL := registerInstanceURLFlag(fs)
	token := fs.String("token", "", "Bearer token")
	id := fs.String("id", "", "Extension ID")
	reason := fs.String("reason", "", "Deactivation reason")
	jsonOutput := fs.Bool("json", false, "Emit JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}

	cfg, err := loadCLIConfig(*instanceURL, *token)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	client := newCLIClient(cfg)

	query, variables := mutationForAction(action, *id, *reason)
	var payload map[string]extensionOutput
	if err := client.Query(ctx, query, variables, &payload); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	result := payload[action+"Extension"]
	if *jsonOutput {
		return writeJSON(stdout, result, stderr)
	}

	fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\n", result.ID, result.Slug, result.Version, result.Status, result.HealthStatus)
	return 0
}

func runExtensionUninstall(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mbr extensions uninstall", flag.ContinueOnError)
	fs.SetOutput(stderr)
	instanceURL := registerInstanceURLFlag(fs)
	token := fs.String("token", "", "Bearer token")
	id := fs.String("id", "", "Extension ID")
	deactivate := fs.Bool("deactivate", false, "Deactivate the extension first when it is still active")
	reason := fs.String("reason", "", "Reason to record when deactivating before uninstall")
	exportOut := fs.String("export-out", "", "Write a removal export bundle before uninstalling")
	confirmNoExport := fs.Bool("confirm-no-export", false, "Confirm uninstall without writing an export bundle")
	dryRun := fs.Bool("dry-run", false, "Show the removal plan without uninstalling")
	jsonOutput := fs.Bool("json", false, "Emit JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}

	cfg, err := loadCLIConfig(*instanceURL, *token)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	client := newCLIClient(cfg)

	detail, err := fetchExtensionDetail(ctx, client, *id)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	plan, err := buildExtensionRemovalPlan(ctx, client, detail)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if detail.Status == string(platformdomain.ExtensionStatusActive) && !*deactivate && !*dryRun {
		fmt.Fprintln(stderr, "extension is active; rerun with --deactivate or deactivate it first")
		return 2
	}
	if plan.RequiresExportConfirmation && strings.TrimSpace(*exportOut) == "" && !*confirmNoExport && !*dryRun {
		fmt.Fprintln(stderr, "extension has exportable state; rerun with --export-out PATH or --confirm-no-export")
		return 2
	}

	result := map[string]any{
		"id":          *id,
		"slug":        detail.Slug,
		"planned":     plan,
		"deactivated": false,
		"uninstalled": false,
	}
	if strings.TrimSpace(*exportOut) != "" {
		exportPath, err := writeExtensionRemovalBundle(*exportOut, plan)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		result["exportOut"] = exportPath
	}
	if *dryRun {
		result["dryRun"] = true
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		printExtensionRemovalResult(stdout, result)
		return 0
	}

	if detail.Status == string(platformdomain.ExtensionStatusActive) && *deactivate {
		deactivatedResult, err := runExtensionAction(ctx, client, "deactivate", *id, *reason)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		result["deactivated"] = true
		result["deactivation"] = deactivatedResult
	}

	var payload struct {
		UninstallExtension bool `json:"uninstallExtension"`
	}
	err = client.Query(ctx, `
		mutation CLIUninstallExtension($id: ID!) {
		  uninstallExtension(id: $id)
		}
	`, map[string]any{"id": *id}, &payload)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !payload.UninstallExtension {
		fmt.Fprintln(stderr, "extension uninstall did not complete")
		return 1
	}
	result["uninstalled"] = true

	if *jsonOutput {
		return writeJSON(stdout, result, stderr)
	}
	printExtensionRemovalResult(stdout, result)
	return 0
}

type extensionRemovalPlan struct {
	GeneratedAt                string                                   `json:"generatedAt"`
	Extension                  extensionDetailOutput                    `json:"extension"`
	ArtifactFilesBySurface     map[string][]extensionArtifactFileOutput `json:"artifactFilesBySurface,omitempty"`
	RequiresExportConfirmation bool                                     `json:"requiresExportConfirmation"`
	Warnings                   []string                                 `json:"warnings,omitempty"`
	NextSteps                  []string                                 `json:"nextSteps"`
	SchemaCleanup              *extensionSchemaCleanupPlan              `json:"schemaCleanup,omitempty"`
}

type extensionSchemaCleanupPlan struct {
	SchemaName    string `json:"schemaName"`
	PackageKey    string `json:"packageKey"`
	TargetVersion string `json:"targetVersion"`
	SuggestedSQL  string `json:"suggestedSQL"`
	Warning       string `json:"warning"`
}

func buildExtensionRemovalPlan(ctx context.Context, client *cliapi.Client, detail extensionDetailOutput) (extensionRemovalPlan, error) {
	artifactFilesBySurface := map[string][]extensionArtifactFileOutput{}
	for _, surface := range detail.ArtifactSurfaces {
		files, err := runExtensionArtifactList(ctx, client, detail.ID, surface.Name)
		if err != nil {
			return extensionRemovalPlan{}, fmt.Errorf("list artifact files for surface %s: %w", surface.Name, err)
		}
		artifactFilesBySurface[surface.Name] = files
	}

	plan := extensionRemovalPlan{
		GeneratedAt:                time.Now().UTC().Format(time.RFC3339),
		Extension:                  detail,
		ArtifactFilesBySurface:     artifactFilesBySurface,
		RequiresExportConfirmation: len(detail.Assets) > 0 || len(detail.ArtifactSurfaces) > 0 || detail.StorageClass == string(platformdomain.ExtensionStorageClassOwnedSchema),
		Warnings:                   []string{},
		NextSteps:                  []string{},
	}
	if detail.Status == string(platformdomain.ExtensionStatusActive) {
		plan.Warnings = append(plan.Warnings, "The extension is currently active and must be deactivated before uninstall.")
	}
	if len(detail.Assets) > 0 {
		plan.NextSteps = append(plan.NextSteps, "Review the exported asset inventory before removing the installation.")
	}
	if len(detail.ArtifactSurfaces) > 0 {
		plan.NextSteps = append(plan.NextSteps, "Review exported artifact surface file lists before removing published extension content.")
	}
	if detail.StorageClass == string(platformdomain.ExtensionStorageClassOwnedSchema) && detail.Schema != nil && strings.TrimSpace(detail.Schema.Name) != "" {
		plan.SchemaCleanup = &extensionSchemaCleanupPlan{
			SchemaName:    detail.Schema.Name,
			PackageKey:    detail.Schema.PackageKey,
			TargetVersion: detail.Schema.TargetVersion,
			SuggestedSQL:  fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE;", detail.Schema.Name),
			Warning:       "Run schema cleanup only after uninstall and only after you have exported or archived any extension-owned data you still need.",
		}
		plan.NextSteps = append(plan.NextSteps, "If you want to reclaim PostgreSQL state after uninstall, review the suggested schema cleanup SQL carefully before executing it.")
	}
	if len(plan.NextSteps) == 0 {
		plan.NextSteps = append(plan.NextSteps, "The extension has no exported assets or artifact surfaces in the current API view; uninstall can proceed once you are comfortable with the current state.")
	}
	return plan, nil
}

func writeExtensionRemovalBundle(path string, plan extensionRemovalPlan) (string, error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return "", fmt.Errorf("export path is required")
	}
	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve export path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absoluteTarget), 0o755); err != nil {
		return "", fmt.Errorf("create export directory: %w", err)
	}
	payload, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode removal export bundle: %w", err)
	}
	if err := os.WriteFile(absoluteTarget, payload, 0o600); err != nil {
		return "", fmt.Errorf("write removal export bundle: %w", err)
	}
	return absoluteTarget, nil
}

func printExtensionRemovalResult(stdout io.Writer, payload map[string]any) {
	fmt.Fprintf(stdout, "extension:\t%s\t%s\n", payload["id"], payload["slug"])
	if deactivated, ok := payload["deactivated"].(bool); ok {
		fmt.Fprintf(stdout, "deactivated:\t%t\n", deactivated)
	}
	if exportOut, ok := payload["exportOut"].(string); ok && strings.TrimSpace(exportOut) != "" {
		fmt.Fprintf(stdout, "exportOut:\t%s\n", exportOut)
	}
	if dryRun, ok := payload["dryRun"].(bool); ok && dryRun {
		fmt.Fprintln(stdout, "dryRun:\ttrue")
	}
	if uninstalled, ok := payload["uninstalled"].(bool); ok {
		fmt.Fprintf(stdout, "uninstalled:\t%t\n", uninstalled)
	}
	plan, ok := payload["planned"].(extensionRemovalPlan)
	if !ok {
		return
	}
	if len(plan.Warnings) > 0 {
		fmt.Fprintln(stdout, "warnings:")
		for _, warning := range plan.Warnings {
			fmt.Fprintf(stdout, "  %s\n", warning)
		}
	}
	if plan.SchemaCleanup != nil {
		fmt.Fprintf(stdout, "schemaCleanup:\t%s\n", plan.SchemaCleanup.SuggestedSQL)
	}
	if len(plan.NextSteps) > 0 {
		fmt.Fprintln(stdout, "nextSteps:")
		for _, step := range plan.NextSteps {
			fmt.Fprintf(stdout, "  %s\n", step)
		}
	}
}

func runExtensionInspect(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mbr extensions inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	instanceURL := registerInstanceURLFlag(fs)
	token := fs.String("token", "", "Bearer token")
	id := fs.String("id", "", "Extension ID")
	jsonOutput := fs.Bool("json", false, "Emit JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}

	cfg, err := loadCLIConfig(*instanceURL, *token)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	client := newCLIClient(cfg)

	extension, err := fetchExtensionDetail(ctx, client, *id)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *jsonOutput {
		return writeJSON(stdout, extension, stderr)
	}

	printExtensionDetail(stdout, extension)
	return 0
}

func runExtensionMonitor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mbr extensions monitor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	instanceURL := registerInstanceURLFlag(fs)
	token := fs.String("token", "", "Bearer token")
	id := fs.String("id", "", "Extension ID")
	jsonOutput := fs.Bool("json", false, "Emit JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}

	cfg, err := loadCLIConfig(*instanceURL, *token)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	client := newCLIClient(cfg)

	query, variables := mutationForAction("checkHealth", *id, "")
	var payload map[string]extensionOutput
	if err := client.Query(ctx, query, variables, &payload); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	extension, err := fetchExtensionDetail(ctx, client, *id)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if payload["checkExtensionHealth"].HealthStatus != "" {
		extension.HealthStatus = payload["checkExtensionHealth"].HealthStatus
		extension.HealthMessage = payload["checkExtensionHealth"].HealthMessage
	}

	if *jsonOutput {
		return writeJSON(stdout, extension, stderr)
	}
	printExtensionDetail(stdout, extension)
	return 0
}
