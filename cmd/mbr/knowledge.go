package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runKnowledge(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printKnowledgeUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr knowledge list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		teamID := fs.String("team", "", "Owner team ID")
		surface := fs.String("surface", "", "Knowledge surface filter")
		reviewStatus := fs.String("review-status", "", "Knowledge review status filter")
		kind := fs.String("kind", "", "Knowledge kind filter")
		status := fs.String("status", "", "Knowledge status filter")
		search := fs.String("search", "", "Search query")
		limit := fs.Int("limit", 20, "Maximum number of resources to return")
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
		teamValue := resolveStoredTeamID(*teamID, stored)
		if *limit <= 0 {
			fmt.Fprintln(stderr, "--limit must be greater than 0")
			return 2
		}
		normalizedKind, err := normalizeKnowledgeKindValue(*kind)
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

		filter := map[string]any{
			"limit": *limit,
		}
		if value := teamValue; value != "" {
			filter["teamID"] = value
		}
		if value := strings.TrimSpace(*surface); value != "" {
			filter["surface"] = strings.ToLower(value)
		}
		if value := strings.TrimSpace(*reviewStatus); value != "" {
			filter["reviewStatus"] = strings.ToLower(value)
		}
		if normalizedKind != "" {
			filter["kind"] = normalizedKind
		}
		if value := strings.TrimSpace(*status); value != "" {
			filter["status"] = strings.ToLower(value)
		}
		if value := strings.TrimSpace(*search); value != "" {
			filter["search"] = value
		}

		var payload struct {
			KnowledgeResources []knowledgeResourceOutput `json:"knowledgeResources"`
		}
		err = client.Query(ctx, `
			query CLIKnowledgeResources($workspaceID: ID!, $filter: KnowledgeResourceFilter) {
			  knowledgeResources(workspaceID: $workspaceID, filter: $filter) {
			    `+knowledgeSelection+`
			  }
			}
		`, map[string]any{
			"workspaceID": workspaceValue,
			"filter":      filter,
		}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload.KnowledgeResources, stderr)
		}
		if len(payload.KnowledgeResources) == 0 {
			fmt.Fprintln(stdout, "no knowledge resources found")
			return 0
		}
		for _, resource := range payload.KnowledgeResources {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", resource.ID, resource.OwnerTeamID, resource.Surface, resource.ReviewStatus, resource.Slug, resource.Title)
		}
		return 0
	case "show":
		fs := flag.NewFlagSet("mbr knowledge show", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for slug lookup")
		teamID := fs.String("team", "", "Owner team ID for slug lookup")
		surface := fs.String("surface", "private", "Knowledge surface for slug lookup")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":       true,
			"--api-url":   true,
			"--token":     true,
			"--workspace": true,
			"--team":      true,
			"--surface":   true,
			"--json":      false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "knowledge resource identifier is required")
			return 2
		}
		identifier := strings.TrimSpace(positionals[0])
		if identifier == "" {
			fmt.Fprintln(stderr, "knowledge resource identifier is required")
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)
		teamValue := resolveStoredTeamID(*teamID, stored)

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		resource, err := runKnowledgeShow(ctx, client, identifier, workspaceValue, teamValue, strings.TrimSpace(*surface))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, resource, stderr)
		}

		fmt.Fprintf(stdout, "id:\t%s\n", resource.ID)
		fmt.Fprintf(stdout, "workspace:\t%s\n", resource.WorkspaceID)
		fmt.Fprintf(stdout, "team:\t%s\n", resource.OwnerTeamID)
		fmt.Fprintf(stdout, "surface:\t%s\n", resource.Surface)
		fmt.Fprintf(stdout, "slug:\t%s\n", resource.Slug)
		fmt.Fprintf(stdout, "title:\t%s\n", resource.Title)
		fmt.Fprintf(stdout, "kind:\t%s\n", resource.Kind)
		fmt.Fprintf(stdout, "conceptSpec:\t%s@%s\n", resource.ConceptSpecKey, resource.ConceptSpecVersion)
		fmt.Fprintf(stdout, "status:\t%s\n", resource.Status)
		fmt.Fprintf(stdout, "reviewStatus:\t%s\n", resource.ReviewStatus)
		fmt.Fprintf(stdout, "sourceKind:\t%s\n", resource.SourceKind)
		if resource.SourceRef != nil {
			fmt.Fprintf(stdout, "sourceRef:\t%s\n", *resource.SourceRef)
		}
		if resource.PathRef != nil {
			fmt.Fprintf(stdout, "pathRef:\t%s\n", *resource.PathRef)
		}
		fmt.Fprintf(stdout, "artifactPath:\t%s\n", resource.ArtifactPath)
		if resource.Summary != nil {
			fmt.Fprintf(stdout, "summary:\t%s\n", *resource.Summary)
		}
		fmt.Fprintf(stdout, "contentHash:\t%s\n", resource.ContentHash)
		fmt.Fprintf(stdout, "revision:\t%s\n", resource.RevisionRef)
		fmt.Fprintf(stdout, "createdAt:\t%s\n", resource.CreatedAt)
		fmt.Fprintf(stdout, "updatedAt:\t%s\n", resource.UpdatedAt)
		if len(resource.SearchKeywords) > 0 {
			fmt.Fprintf(stdout, "keywords:\t%s\n", strings.Join(resource.SearchKeywords, ","))
		}
		if len(resource.SupportedChannels) > 0 {
			fmt.Fprintf(stdout, "channels:\t%s\n", strings.Join(resource.SupportedChannels, ","))
		}
		if len(resource.SharedWithTeamIDs) > 0 {
			fmt.Fprintf(stdout, "sharedWith:\t%s\n", strings.Join(resource.SharedWithTeamIDs, ","))
		}
		if resource.ReviewedAt != nil {
			fmt.Fprintf(stdout, "reviewedAt:\t%s\n", *resource.ReviewedAt)
		}
		if resource.PublishedRevision != nil {
			fmt.Fprintf(stdout, "publishedRevision:\t%s\n", *resource.PublishedRevision)
		}
		if resource.PublishedAt != nil {
			fmt.Fprintf(stdout, "publishedAt:\t%s\n", *resource.PublishedAt)
		}
		if resource.PublishedByID != nil {
			fmt.Fprintf(stdout, "publishedBy:\t%s\n", *resource.PublishedByID)
		}
		if body := strings.TrimSpace(resource.BodyMarkdown); body != "" {
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, body)
		}
		return 0
	case "search":
		fs := flag.NewFlagSet("mbr knowledge search", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		teamID := fs.String("team", "", "Owner team ID")
		surface := fs.String("surface", "", "Knowledge surface filter")
		reviewStatus := fs.String("review-status", "", "Knowledge review status filter")
		kind := fs.String("kind", "", "Knowledge kind filter")
		status := fs.String("status", "", "Knowledge status filter")
		limit := fs.Int("limit", 20, "Maximum number of resources to return")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":           true,
			"--api-url":       true,
			"--token":         true,
			"--workspace":     true,
			"--team":          true,
			"--surface":       true,
			"--review-status": true,
			"--kind":          true,
			"--status":        true,
			"--limit":         true,
			"--json":          false,
		})
		if err := fs.Parse(flagArgs); err != nil {
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
		teamValue := resolveStoredTeamID(*teamID, stored)
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "search query is required")
			return 2
		}
		query := strings.TrimSpace(positionals[0])
		if query == "" {
			fmt.Fprintln(stderr, "search query is required")
			return 2
		}
		if *limit <= 0 {
			fmt.Fprintln(stderr, "--limit must be greater than 0")
			return 2
		}
		normalizedKind, err := normalizeKnowledgeKindValue(*kind)
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

		filter := map[string]any{
			"search": query,
			"limit":  *limit,
		}
		if value := teamValue; value != "" {
			filter["teamID"] = value
		}
		if value := strings.TrimSpace(*surface); value != "" {
			filter["surface"] = strings.ToLower(value)
		}
		if value := strings.TrimSpace(*reviewStatus); value != "" {
			filter["reviewStatus"] = strings.ToLower(value)
		}
		if normalizedKind != "" {
			filter["kind"] = normalizedKind
		}
		if value := strings.TrimSpace(*status); value != "" {
			filter["status"] = strings.ToLower(value)
		}

		var payload struct {
			KnowledgeResources []knowledgeResourceOutput `json:"knowledgeResources"`
		}
		err = client.Query(ctx, `
			query CLIKnowledgeSearch($workspaceID: ID!, $filter: KnowledgeResourceFilter) {
			  knowledgeResources(workspaceID: $workspaceID, filter: $filter) {
			    `+knowledgeSelection+`
			  }
			}
		`, map[string]any{
			"workspaceID": workspaceValue,
			"filter":      filter,
		}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload.KnowledgeResources, stderr)
		}
		if len(payload.KnowledgeResources) == 0 {
			fmt.Fprintln(stdout, "no knowledge resources found")
			return 0
		}
		for _, resource := range payload.KnowledgeResources {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", resource.ID, resource.OwnerTeamID, resource.Surface, resource.ReviewStatus, resource.Slug, resource.Title)
		}
		return 0
	case "review-queue":
		fs := flag.NewFlagSet("mbr knowledge review-queue", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		teamID := fs.String("team", "", "Owner team ID")
		kind := fs.String("kind", "decision", "Knowledge kind filter")
		reviewStatus := fs.String("review-status", "draft", "Knowledge review status filter")
		limit := fs.Int("limit", 20, "Maximum number of resources to return")
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
		teamValue := resolveStoredTeamID(*teamID, stored)
		if strings.TrimSpace(teamValue) == "" {
			fmt.Fprintln(stderr, "--team is required")
			return 2
		}
		if *limit <= 0 {
			fmt.Fprintln(stderr, "--limit must be greater than 0")
			return 2
		}
		normalizedKind, err := normalizeKnowledgeKindValue(*kind)
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

		filter := map[string]any{
			"teamID":       teamValue,
			"kind":         normalizedKind,
			"reviewStatus": strings.ToLower(strings.TrimSpace(*reviewStatus)),
			"limit":        *limit,
		}
		var payload struct {
			KnowledgeResources []knowledgeResourceOutput `json:"knowledgeResources"`
		}
		err = client.Query(ctx, `
			query CLIKnowledgeReviewQueue($workspaceID: ID!, $filter: KnowledgeResourceFilter) {
			  knowledgeResources(workspaceID: $workspaceID, filter: $filter) {
			    `+knowledgeSelection+`
			  }
			}
		`, map[string]any{
			"workspaceID": workspaceValue,
			"filter":      filter,
		}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload.KnowledgeResources, stderr)
		}
		if len(payload.KnowledgeResources) == 0 {
			fmt.Fprintln(stdout, "no knowledge items need review")
			return 0
		}
		for _, resource := range payload.KnowledgeResources {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", resource.ID, resource.OwnerTeamID, resource.Surface, resource.ReviewStatus, resource.Slug, resource.Title)
		}
		return 0
	case "upsert":
		fs := flag.NewFlagSet("mbr knowledge upsert", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		teamID := fs.String("team", "", "Owner team ID")
		surface := fs.String("surface", "private", "Knowledge surface")
		slug := fs.String("slug", "", "Knowledge slug")
		title := fs.String("title", "", "Knowledge title")
		kind := fs.String("kind", "", "Knowledge kind")
		conceptSpec := fs.String("concept-spec", "", "Concept spec key")
		conceptVersion := fs.String("concept-version", "", "Concept spec version")
		status := fs.String("status", "", "Knowledge status")
		summary := fs.String("summary", "", "Short summary")
		body := fs.String("body", "", "Inline markdown body")
		bodyFile := fs.String("file", "", "Path to a markdown file")
		sourceKind := fs.String("source-kind", "", "Knowledge source kind")
		sourceRef := fs.String("source-ref", "", "Source reference")
		pathRef := fs.String("path-ref", "", "Stable path reference")
		channels := fs.String("channels", "", "Comma-separated supported channels")
		sharedWith := fs.String("share-with", "", "Comma-separated team IDs that can access the resource")
		keywords := fs.String("keywords", "", "Comma-separated search keywords")
		frontmatterFile := fs.String("frontmatter-file", "", "Path to frontmatter JSON")
		frontmatterJSON := fs.String("frontmatter-json", "", "Inline frontmatter JSON")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":              true,
			"--api-url":          true,
			"--token":            true,
			"--workspace":        true,
			"--team":             true,
			"--surface":          true,
			"--slug":             true,
			"--title":            true,
			"--kind":             true,
			"--concept-spec":     true,
			"--concept-version":  true,
			"--status":           true,
			"--summary":          true,
			"--body":             true,
			"--file":             true,
			"--source-kind":      true,
			"--source-ref":       true,
			"--path-ref":         true,
			"--channels":         true,
			"--share-with":       true,
			"--keywords":         true,
			"--frontmatter-file": true,
			"--frontmatter-json": true,
			"--json":             false,
		})
		if err := fs.Parse(flagArgs); err != nil {
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
		teamValue := resolveStoredTeamID(*teamID, stored)
		if strings.TrimSpace(*slug) == "" {
			fmt.Fprintln(stderr, "--slug is required")
			return 2
		}

		bodyValue, err := readOptionalTextInput(*bodyFile, *body, "body")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		normalizedKind, err := normalizeKnowledgeKindValue(*kind)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		frontmatterValue, err := readOptionalJSONObjectInput(*frontmatterFile, *frontmatterJSON, "frontmatter")
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

		existing := (*knowledgeResourceOutput)(nil)
		if len(positionals) == 1 {
			existingResource, err := runKnowledgeShow(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, teamValue, strings.TrimSpace(*surface))
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			existing = &existingResource
		} else {
			if strings.TrimSpace(teamValue) == "" {
				fmt.Fprintln(stderr, "--team is required when creating or updating by slug")
				return 2
			}
			existing, err = runKnowledgeShowBySlug(ctx, client, workspaceValue, teamValue, strings.TrimSpace(*surface), strings.TrimSpace(*slug))
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		var resource knowledgeResourceOutput
		if existing == nil {
			if strings.TrimSpace(*title) == "" {
				fmt.Fprintln(stderr, "--title is required when creating a knowledge resource")
				return 2
			}
			resource, err = runKnowledgeCreate(ctx, client, knowledgeMutationInput{
				WorkspaceID:        workspaceValue,
				TeamID:             teamValue,
				Slug:               strings.TrimSpace(*slug),
				Title:              strings.TrimSpace(*title),
				Kind:               normalizedKind,
				ConceptSpecKey:     strings.TrimSpace(*conceptSpec),
				ConceptSpecVersion: strings.TrimSpace(*conceptVersion),
				Status:             strings.TrimSpace(*status),
				Summary:            strings.TrimSpace(*summary),
				BodyMarkdown:       bodyValue,
				SourceKind:         strings.TrimSpace(*sourceKind),
				SourceRef:          strings.TrimSpace(*sourceRef),
				PathRef:            strings.TrimSpace(*pathRef),
				SupportedChannels:  commaSeparatedValues(*channels),
				SharedWithTeamIDs:  commaSeparatedValues(*sharedWith),
				SearchKeywords:     commaSeparatedValues(*keywords),
				Surface:            strings.TrimSpace(*surface),
				Frontmatter:        frontmatterValue,
			})
		} else {
			input := knowledgeMutationInput{
				Title:              strings.TrimSpace(*title),
				Kind:               normalizedKind,
				ConceptSpecKey:     strings.TrimSpace(*conceptSpec),
				ConceptSpecVersion: strings.TrimSpace(*conceptVersion),
				Status:             strings.TrimSpace(*status),
				Summary:            strings.TrimSpace(*summary),
				BodyMarkdown:       bodyValue,
				SourceKind:         strings.TrimSpace(*sourceKind),
				SourceRef:          strings.TrimSpace(*sourceRef),
				PathRef:            strings.TrimSpace(*pathRef),
				SupportedChannels:  commaSeparatedValues(*channels),
				SearchKeywords:     commaSeparatedValues(*keywords),
				Frontmatter:        frontmatterValue,
				Slug:               strings.TrimSpace(*slug),
			}
			resource, err = runKnowledgeUpdate(ctx, client, existing.ID, input)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		if *jsonOutput {
			return writeJSON(stdout, resource, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", resource.ID, resource.OwnerTeamID, resource.Surface, resource.ReviewStatus, resource.Slug, resource.Title)
		return 0
	case "sync":
		fs := flag.NewFlagSet("mbr knowledge sync", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		teamID := fs.String("team", "", "Default owner team ID")
		surface := fs.String("surface", "", "Default knowledge surface")
		kind := fs.String("kind", "", "Default knowledge kind")
		conceptSpec := fs.String("concept-spec", "", "Default concept spec key")
		conceptVersion := fs.String("concept-version", "", "Default concept spec version")
		status := fs.String("status", "", "Default knowledge status")
		reviewStatus := fs.String("review-status", "", "Default review status")
		shareWith := fs.String("share-with", "", "Default comma-separated team IDs to share with")
		sourceKind := fs.String("source-kind", "", "Default source kind")
		sourceRef := fs.String("source-ref", "", "Default source reference")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":             true,
			"--api-url":         true,
			"--token":           true,
			"--workspace":       true,
			"--team":            true,
			"--surface":         true,
			"--kind":            true,
			"--concept-spec":    true,
			"--concept-version": true,
			"--status":          true,
			"--review-status":   true,
			"--share-with":      true,
			"--source-kind":     true,
			"--source-ref":      true,
			"--json":            false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "markdown path is required")
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
		defaultTeamID := resolveStoredTeamID(*teamID, stored)

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		documents, err := readKnowledgeSyncDocuments(strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		results := make([]knowledgeSyncResult, 0, len(documents))
		defaults := knowledgeSyncDefaults{
			WorkspaceID:        workspaceValue,
			TeamID:             defaultTeamID,
			Surface:            strings.TrimSpace(*surface),
			Kind:               "",
			ConceptSpecKey:     strings.TrimSpace(*conceptSpec),
			ConceptSpecVersion: strings.TrimSpace(*conceptVersion),
			Status:             strings.TrimSpace(*status),
			ReviewStatus:       strings.TrimSpace(*reviewStatus),
			SharedWithTeamIDs:  commaSeparatedValues(*shareWith),
			SourceKind:         strings.TrimSpace(*sourceKind),
			SourceRef:          strings.TrimSpace(*sourceRef),
		}
		defaults.Kind, err = normalizeKnowledgeKindValue(*kind)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		for _, document := range documents {
			result, err := syncKnowledgeDocument(ctx, client, defaults, document)
			if err != nil {
				fmt.Fprintf(stderr, "sync %s: %v\n", document.RelativePath, err)
				return 1
			}
			results = append(results, result)
		}

		if *jsonOutput {
			return writeJSON(stdout, results, stderr)
		}
		for _, result := range results {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\n", result.RelativePath, result.Action, result.TeamID, result.Surface, result.Slug)
		}
		return 0
	case "checkout":
		fs := flag.NewFlagSet("mbr knowledge checkout", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		teamID := fs.String("team", "", "Team ID filter")
		surface := fs.String("surface", "", "Knowledge surface filter")
		kind := fs.String("kind", "", "Knowledge kind filter")
		reviewStatus := fs.String("review-status", "", "Knowledge review status filter")
		status := fs.String("status", "", "Knowledge status filter")
		search := fs.String("search", "", "Knowledge search filter")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":           true,
			"--api-url":       true,
			"--token":         true,
			"--workspace":     true,
			"--team":          true,
			"--surface":       true,
			"--kind":          true,
			"--review-status": true,
			"--status":        true,
			"--search":        true,
			"--json":          false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "checkout path is required")
			return 2
		}
		result, err := executeKnowledgeCheckout(ctx, *instanceURL, *token, *workspaceID, *teamID, *surface, *kind, *reviewStatus, *status, *search, strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		fmt.Fprintf(stdout, "checked out %d knowledge resources to %s\n", result.ResourceCount, result.RootPath)
		fmt.Fprintf(stdout, "manifest:\t%s\n", result.ManifestPath)
		return 0
	case "status":
		fs := flag.NewFlagSet("mbr knowledge status", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":     true,
			"--api-url": true,
			"--token":   true,
			"--json":    false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		rootPath := "."
		if len(positionals) > 1 {
			fmt.Fprintln(stderr, "at most one checkout path may be provided")
			return 2
		}
		if len(positionals) == 1 && strings.TrimSpace(positionals[0]) != "" {
			rootPath = strings.TrimSpace(positionals[0])
		}
		result, err := executeKnowledgeStatus(ctx, rootPath, *instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		fmt.Fprintf(stdout, "status:\t%s\n", result.Status)
		fmt.Fprintf(stdout, "root:\t%s\n", result.RootPath)
		fmt.Fprintf(stdout, "summary:\tclean=%d ahead=%d behind=%d diverged=%d local_only=%d new_on_server=%d deleted_local=%d deleted_on_server=%d\n",
			result.Summary.Clean,
			result.Summary.Ahead,
			result.Summary.Behind,
			result.Summary.Diverged,
			result.Summary.LocalOnly,
			result.Summary.NewOnServer,
			result.Summary.DeletedLocal,
			result.Summary.DeletedOnServer,
		)
		for _, entry := range result.Entries {
			fmt.Fprintf(stdout, "%s\t%s\n", entry.State, entry.Path)
		}
		return 0
	case "pull":
		fs := flag.NewFlagSet("mbr knowledge pull", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":     true,
			"--api-url": true,
			"--token":   true,
			"--json":    false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		rootPath := "."
		if len(positionals) > 1 {
			fmt.Fprintln(stderr, "at most one checkout path may be provided")
			return 2
		}
		if len(positionals) == 1 && strings.TrimSpace(positionals[0]) != "" {
			rootPath = strings.TrimSpace(positionals[0])
		}
		result, err := executeKnowledgePull(ctx, rootPath, *instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		fmt.Fprintf(stdout, "status:\t%s\n", result.Status)
		fmt.Fprintf(stdout, "root:\t%s\n", result.RootPath)
		fmt.Fprintf(stdout, "manifest:\t%s\n", result.ManifestPath)
		fmt.Fprintf(stdout, "summary:\tadded=%d updated=%d deleted=%d preserved_local_only=%d\n",
			result.Summary.Added,
			result.Summary.Updated,
			result.Summary.Deleted,
			result.Summary.PreservedLocalOnly,
		)
		for _, entry := range result.Entries {
			fmt.Fprintf(stdout, "%s\t%s\n", entry.Action, entry.Path)
		}
		return 0
	case "push":
		fs := flag.NewFlagSet("mbr knowledge push", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":     true,
			"--api-url": true,
			"--token":   true,
			"--json":    false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		rootPath := "."
		if len(positionals) > 1 {
			fmt.Fprintln(stderr, "at most one checkout path may be provided")
			return 2
		}
		if len(positionals) == 1 && strings.TrimSpace(positionals[0]) != "" {
			rootPath = strings.TrimSpace(positionals[0])
		}
		result, err := executeKnowledgePush(ctx, rootPath, *instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		fmt.Fprintf(stdout, "status:\t%s\n", result.Status)
		fmt.Fprintf(stdout, "root:\t%s\n", result.RootPath)
		fmt.Fprintf(stdout, "manifest:\t%s\n", result.ManifestPath)
		fmt.Fprintf(stdout, "summary:\tadded=%d updated=%d deleted=%d\n",
			result.Summary.Added,
			result.Summary.Updated,
			result.Summary.Deleted,
		)
		for _, entry := range result.Entries {
			fmt.Fprintf(stdout, "%s\t%s\n", entry.Action, entry.Path)
		}
		return 0
	case "import":
		fs := flag.NewFlagSet("mbr knowledge import", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		teamID := fs.String("team", "", "Default owner team ID")
		surface := fs.String("surface", "", "Default knowledge surface")
		kind := fs.String("kind", "", "Default knowledge kind")
		conceptSpec := fs.String("concept-spec", "", "Default concept spec key")
		conceptVersion := fs.String("concept-version", "", "Default concept spec version")
		status := fs.String("status", "", "Default knowledge status")
		reviewStatus := fs.String("review-status", "", "Default review status")
		shareWith := fs.String("share-with", "", "Default comma-separated team IDs to share with")
		sourceKind := fs.String("source-kind", "", "Default source kind")
		sourceRef := fs.String("source-ref", "", "Default source reference")
		mode := fs.String("mode", "preview", "Import mode: preview or apply")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":             true,
			"--api-url":         true,
			"--token":           true,
			"--workspace":       true,
			"--team":            true,
			"--surface":         true,
			"--kind":            true,
			"--concept-spec":    true,
			"--concept-version": true,
			"--status":          true,
			"--review-status":   true,
			"--share-with":      true,
			"--source-kind":     true,
			"--source-ref":      true,
			"--mode":            true,
			"--json":            false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "markdown path is required")
			return 2
		}
		importMode := strings.ToLower(strings.TrimSpace(*mode))
		if importMode != "preview" && importMode != "apply" {
			fmt.Fprintln(stderr, "--mode must be preview or apply")
			return 2
		}

		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)
		if importMode == "apply" {
			workspaceValue, err = requireWorkspaceID(*workspaceID, stored)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
		}
		defaultTeamID := resolveStoredTeamID(*teamID, stored)
		defaultKind, err := normalizeKnowledgeKindValue(*kind)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		documents, err := readKnowledgeSyncDocuments(strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defaults := knowledgeSyncDefaults{
			WorkspaceID:        workspaceValue,
			TeamID:             defaultTeamID,
			Surface:            strings.TrimSpace(*surface),
			Kind:               defaultKind,
			ConceptSpecKey:     strings.TrimSpace(*conceptSpec),
			ConceptSpecVersion: strings.TrimSpace(*conceptVersion),
			Status:             strings.TrimSpace(*status),
			ReviewStatus:       strings.TrimSpace(*reviewStatus),
			SharedWithTeamIDs:  commaSeparatedValues(*shareWith),
			SourceKind:         strings.TrimSpace(*sourceKind),
			SourceRef:          strings.TrimSpace(*sourceRef),
		}

		if importMode == "preview" {
			plans := make([]knowledgeImportPlan, 0, len(documents))
			for _, document := range documents {
				plan, err := planKnowledgeImport(defaults, document)
				if err != nil {
					fmt.Fprintf(stderr, "import %s: %v\n", document.RelativePath, err)
					return 1
				}
				plans = append(plans, plan)
			}
			if *jsonOutput {
				return writeJSON(stdout, plans, stderr)
			}
			for _, plan := range plans {
				fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", plan.RelativePath, plan.TeamID, plan.Surface, plan.Kind, plan.Slug, plan.Title)
			}
			return 0
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		results := make([]knowledgeSyncResult, 0, len(documents))
		for _, document := range documents {
			result, err := syncKnowledgeDocument(ctx, client, defaults, document)
			if err != nil {
				fmt.Fprintf(stderr, "import %s: %v\n", document.RelativePath, err)
				return 1
			}
			results = append(results, result)
		}
		if *jsonOutput {
			return writeJSON(stdout, results, stderr)
		}
		for _, result := range results {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\n", result.RelativePath, result.Action, result.TeamID, result.Surface, result.Slug)
		}
		return 0
	case "review":
		fs := flag.NewFlagSet("mbr knowledge review", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for slug lookup")
		teamID := fs.String("team", "", "Owner team ID for slug lookup")
		surface := fs.String("surface", "private", "Knowledge surface for slug lookup")
		reviewStatus := fs.String("status", "reviewed", "Review status")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url": true, "--api-url": true, "--token": true, "--workspace": true, "--team": true, "--surface": true, "--status": true, "--json": false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "knowledge resource identifier is required")
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)
		teamValue := resolveStoredTeamID(*teamID, stored)
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		resource, err := runKnowledgeShow(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, teamValue, strings.TrimSpace(*surface))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		resource, err = runKnowledgeReview(ctx, client, resource.ID, strings.TrimSpace(*reviewStatus))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, resource, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", resource.ID, resource.OwnerTeamID, resource.Surface, resource.ReviewStatus, resource.Slug, resource.Title)
		return 0
	case "publish":
		fs := flag.NewFlagSet("mbr knowledge publish", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for slug lookup")
		teamID := fs.String("team", "", "Owner team ID for slug lookup")
		surface := fs.String("surface", "private", "Current knowledge surface for slug lookup")
		targetSurface := fs.String("to-surface", "published", "Target surface")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url": true, "--api-url": true, "--token": true, "--workspace": true, "--team": true, "--surface": true, "--to-surface": true, "--json": false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "knowledge resource identifier is required")
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)
		teamValue := resolveStoredTeamID(*teamID, stored)
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		resource, err := runKnowledgeShow(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, teamValue, strings.TrimSpace(*surface))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		resource, err = runKnowledgePublish(ctx, client, resource.ID, strings.TrimSpace(*targetSurface))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, resource, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", resource.ID, resource.OwnerTeamID, resource.Surface, resource.ReviewStatus, resource.Slug, resource.Title)
		return 0
	case "share":
		fs := flag.NewFlagSet("mbr knowledge share", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for slug lookup")
		teamID := fs.String("team", "", "Owner team ID for slug lookup")
		surface := fs.String("surface", "private", "Knowledge surface for slug lookup")
		shareWith := fs.String("share-with", "", "Comma-separated team IDs")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url": true, "--api-url": true, "--token": true, "--workspace": true, "--team": true, "--surface": true, "--share-with": true, "--json": false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "knowledge resource identifier is required")
			return 2
		}
		teamIDs := commaSeparatedValues(*shareWith)
		if len(teamIDs) == 0 {
			fmt.Fprintln(stderr, "--share-with is required")
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)
		teamValue := resolveStoredTeamID(*teamID, stored)
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		resource, err := runKnowledgeShow(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, teamValue, strings.TrimSpace(*surface))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		resource, err = runKnowledgeShare(ctx, client, resource.ID, teamIDs)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, resource, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\n", resource.ID, strings.Join(resource.SharedWithTeamIDs, ","))
		return 0
	case "delete":
		fs := flag.NewFlagSet("mbr knowledge delete", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for slug lookup")
		teamID := fs.String("team", "", "Owner team ID for slug lookup")
		surface := fs.String("surface", "private", "Knowledge surface for slug lookup")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url": true, "--api-url": true, "--token": true, "--workspace": true, "--team": true, "--surface": true, "--json": false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "knowledge resource identifier is required")
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)
		teamValue := resolveStoredTeamID(*teamID, stored)
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		resource, err := runKnowledgeShow(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, teamValue, strings.TrimSpace(*surface))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		resource, err = runKnowledgeDelete(ctx, client, resource.ID)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, resource, stderr)
		}
		fmt.Fprintf(stdout, "deleted\t%s\t%s\t%s\n", resource.ID, resource.OwnerTeamID, resource.Slug)
		return 0
	case "history":
		fs := flag.NewFlagSet("mbr knowledge history", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for slug lookup")
		teamID := fs.String("team", "", "Owner team ID for slug lookup")
		surface := fs.String("surface", "private", "Knowledge surface for slug lookup")
		limit := fs.Int("limit", 20, "Maximum number of revisions")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url": true, "--api-url": true, "--token": true, "--workspace": true, "--team": true, "--surface": true, "--limit": true, "--json": false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "knowledge resource identifier is required")
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)
		teamValue := resolveStoredTeamID(*teamID, stored)
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		resource, err := runKnowledgeShow(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, teamValue, strings.TrimSpace(*surface))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		revisions, err := runKnowledgeHistory(ctx, client, resource.ID, *limit)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, revisions, stderr)
		}
		for _, revision := range revisions {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", revision.Ref, revision.CommittedAt, revision.Subject)
		}
		return 0
	case "diff":
		fs := flag.NewFlagSet("mbr knowledge diff", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for slug lookup")
		teamID := fs.String("team", "", "Owner team ID for slug lookup")
		surface := fs.String("surface", "private", "Knowledge surface for slug lookup")
		fromRevision := fs.String("from", "", "Base revision")
		toRevision := fs.String("to", "", "Target revision")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url": true, "--api-url": true, "--token": true, "--workspace": true, "--team": true, "--surface": true, "--from": true, "--to": true, "--json": false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "knowledge resource identifier is required")
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)
		teamValue := resolveStoredTeamID(*teamID, stored)
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		resource, err := runKnowledgeShow(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, teamValue, strings.TrimSpace(*surface))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		diff, err := runKnowledgeDiff(ctx, client, resource.ID, strings.TrimSpace(*fromRevision), strings.TrimSpace(*toRevision))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, diff, stderr)
		}
		fmt.Fprint(stdout, diff.Patch)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown knowledge command %q\n\n", args[0])
		printKnowledgeUsage(stderr)
		return 2
	}
}
