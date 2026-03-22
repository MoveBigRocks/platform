package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runConcepts(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printConceptsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr concepts list", flag.ContinueOnError)
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
		workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		specs, err := runConceptSpecList(ctx, client, workspaceValue)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, specs, stderr)
		}
		if len(specs) == 0 {
			fmt.Fprintln(stdout, "no concept specs found")
			return 0
		}
		for _, spec := range specs {
			fmt.Fprintf(stdout, "%s@%s\t%s\t%s\t%s\t%s\n", spec.Key, spec.Version, spec.InstanceKind, spec.SourceKind, spec.Status, spec.Name)
		}
		return 0
	case "show":
		fs := flag.NewFlagSet("mbr concepts show", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		version := fs.String("version", "", "Concept spec version")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url": true, "--api-url": true, "--token": true, "--workspace": true, "--version": true, "--json": false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "concept spec key is required")
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		spec, err := runConceptSpecShow(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, strings.TrimSpace(*version))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, spec, stderr)
		}
		fmt.Fprintf(stdout, "key:\t%s\n", spec.Key)
		fmt.Fprintf(stdout, "version:\t%s\n", spec.Version)
		fmt.Fprintf(stdout, "name:\t%s\n", spec.Name)
		fmt.Fprintf(stdout, "instanceKind:\t%s\n", spec.InstanceKind)
		fmt.Fprintf(stdout, "sourceKind:\t%s\n", spec.SourceKind)
		fmt.Fprintf(stdout, "status:\t%s\n", spec.Status)
		if spec.WorkspaceID != nil {
			fmt.Fprintf(stdout, "workspace:\t%s\n", *spec.WorkspaceID)
		}
		if spec.OwnerTeamID != nil {
			fmt.Fprintf(stdout, "team:\t%s\n", *spec.OwnerTeamID)
		}
		if spec.ExtendsKey != nil {
			versionValue := ""
			if spec.ExtendsVersion != nil {
				versionValue = *spec.ExtendsVersion
			}
			fmt.Fprintf(stdout, "extends:\t%s@%s\n", *spec.ExtendsKey, versionValue)
		}
		fmt.Fprintf(stdout, "artifactPath:\t%s\n", spec.ArtifactPath)
		if spec.RevisionRef != nil {
			fmt.Fprintf(stdout, "revision:\t%s\n", *spec.RevisionRef)
		}
		if spec.SourceRef != nil {
			fmt.Fprintf(stdout, "sourceRef:\t%s\n", *spec.SourceRef)
		}
		if spec.CreatedByID != nil {
			fmt.Fprintf(stdout, "createdBy:\t%s\n", *spec.CreatedByID)
		}
		fmt.Fprintf(stdout, "createdAt:\t%s\n", spec.CreatedAt)
		fmt.Fprintf(stdout, "updatedAt:\t%s\n", spec.UpdatedAt)
		if strings.TrimSpace(spec.Description) != "" {
			fmt.Fprintf(stdout, "description:\t%s\n", spec.Description)
		}
		if strings.TrimSpace(spec.AgentGuidanceMarkdown) != "" {
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, spec.AgentGuidanceMarkdown)
		}
		return 0
	case "register":
		fs := flag.NewFlagSet("mbr concepts register", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		teamID := fs.String("team", "", "Owner team ID")
		sourceKind := fs.String("source-kind", "", "Source kind")
		sourceRef := fs.String("source-ref", "", "Source reference")
		status := fs.String("status", "", "Concept spec status")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url": true, "--api-url": true, "--token": true, "--workspace": true, "--team": true, "--source-kind": true, "--source-ref": true, "--status": true, "--json": false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "concept spec file is required")
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
		specInput, err := readConceptSpecInputFile(strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		specInput.WorkspaceID = workspaceValue
		if value := resolveStoredTeamID(*teamID, stored); value != "" {
			specInput.OwnerTeamID = value
		}
		if value := strings.TrimSpace(*sourceKind); value != "" {
			specInput.SourceKind = value
		}
		if value := strings.TrimSpace(*sourceRef); value != "" {
			specInput.SourceRef = value
		}
		if value := strings.TrimSpace(*status); value != "" {
			specInput.Status = value
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		spec, err := runConceptSpecRegister(ctx, client, specInput)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, spec, stderr)
		}
		fmt.Fprintf(stdout, "%s@%s\t%s\t%s\t%s\n", spec.Key, spec.Version, spec.InstanceKind, spec.SourceKind, spec.Status)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown concepts command %q\n\n", args[0])
		printConceptsUsage(stderr)
		return 2
	}
}
