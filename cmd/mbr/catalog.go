package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runCatalog(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printCatalogUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr catalog list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		parentNodeID := fs.String("parent", "", "Parent catalog node ID")
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

		var payload struct {
			ServiceCatalogNodes []serviceCatalogNodeOutput `json:"serviceCatalogNodes"`
		}
		variables := map[string]any{
			"workspaceID": workspaceValue,
		}
		if value := strings.TrimSpace(*parentNodeID); value != "" {
			variables["parentNodeID"] = value
		}
		err = client.Query(ctx, `
			query CLIServiceCatalogNodes($workspaceID: ID!, $parentNodeID: ID) {
			  serviceCatalogNodes(workspaceID: $workspaceID, parentNodeID: $parentNodeID) {
			    `+serviceCatalogNodeListSelection+`
			  }
			}
		`, variables, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload.ServiceCatalogNodes, stderr)
		}
		if len(payload.ServiceCatalogNodes) == 0 {
			fmt.Fprintln(stdout, "no catalog nodes found")
			return 0
		}
		for _, node := range payload.ServiceCatalogNodes {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", node.ID, node.PathSlug, node.NodeKind, node.Title)
		}
		return 0
	case "show":
		fs := flag.NewFlagSet("mbr catalog show", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for path lookup")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":       true,
			"--api-url":   true,
			"--token":     true,
			"--workspace": true,
			"--json":      false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "catalog node identifier is required")
			return 2
		}
		identifier := strings.TrimSpace(positionals[0])
		if identifier == "" {
			fmt.Fprintln(stderr, "catalog node identifier is required")
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

		node, err := runServiceCatalogShow(ctx, client, identifier, workspaceValue)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, node, stderr)
		}

		fmt.Fprintf(stdout, "id:\t%s\n", node.ID)
		fmt.Fprintf(stdout, "workspace:\t%s\n", node.WorkspaceID)
		if node.ParentNodeID != nil {
			fmt.Fprintf(stdout, "parent:\t%s\n", *node.ParentNodeID)
		}
		fmt.Fprintf(stdout, "slug:\t%s\n", node.Slug)
		fmt.Fprintf(stdout, "path:\t%s\n", node.PathSlug)
		fmt.Fprintf(stdout, "title:\t%s\n", node.Title)
		fmt.Fprintf(stdout, "kind:\t%s\n", node.NodeKind)
		fmt.Fprintf(stdout, "status:\t%s\n", node.Status)
		fmt.Fprintf(stdout, "visibility:\t%s\n", node.Visibility)
		if node.DescriptionMarkdown != nil {
			fmt.Fprintf(stdout, "description:\t%s\n", *node.DescriptionMarkdown)
		}
		if node.DefaultCaseCategory != nil {
			fmt.Fprintf(stdout, "defaultCaseCategory:\t%s\n", *node.DefaultCaseCategory)
		}
		if node.DefaultQueueID != nil {
			fmt.Fprintf(stdout, "defaultQueueID:\t%s\n", *node.DefaultQueueID)
		}
		if node.DefaultPriority != nil {
			fmt.Fprintf(stdout, "defaultPriority:\t%s\n", *node.DefaultPriority)
		}
		fmt.Fprintf(stdout, "displayOrder:\t%d\n", node.DisplayOrder)
		fmt.Fprintf(stdout, "createdAt:\t%s\n", node.CreatedAt)
		fmt.Fprintf(stdout, "updatedAt:\t%s\n", node.UpdatedAt)
		if len(node.SearchKeywords) > 0 {
			fmt.Fprintf(stdout, "keywords:\t%s\n", strings.Join(node.SearchKeywords, ","))
		}
		if len(node.SupportedChannels) > 0 {
			fmt.Fprintf(stdout, "channels:\t%s\n", strings.Join(node.SupportedChannels, ","))
		}
		if len(node.Bindings) > 0 {
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "bindings:")
			for _, binding := range node.Bindings {
				fmt.Fprintf(stdout, "  %s\t%s\t%s\t%s\n", binding.ID, binding.BindingKind, binding.TargetKind, binding.TargetID)
			}
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown catalog command %q\n\n", args[0])
		printCatalogUsage(stderr)
		return 2
	}
}
