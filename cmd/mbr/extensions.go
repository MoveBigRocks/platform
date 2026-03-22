package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

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
		licenseToken := fs.String("license-token", "", "Extension license token")
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
		if strings.TrimSpace(*licenseToken) == "" {
			fmt.Fprintln(stderr, "--license-token is required")
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
		licenseToken := fs.String("license-token", "", "Extension license token")
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
		if strings.TrimSpace(*licenseToken) == "" {
			fmt.Fprintln(stderr, "--license-token is required")
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
		licenseToken := fs.String("license-token", "", "Optional replacement extension license token")
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
