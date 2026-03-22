package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
)

func runArtifacts(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printArtifactsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr artifacts list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		extensionID := fs.String("extension", "", "Extension ID")
		surface := fs.String("surface", "", "Artifact surface name")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*extensionID) == "" || strings.TrimSpace(*surface) == "" {
			fmt.Fprintln(stderr, "--extension and --surface are required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		files, err := runExtensionArtifactList(ctx, client, *extensionID, *surface)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, files, stderr)
		}
		if len(files) == 0 {
			fmt.Fprintln(stdout, "no managed artifacts found")
			return 0
		}
		for _, file := range files {
			fmt.Fprintf(stdout, "%s\t%s\n", file.Surface, file.Path)
		}
		return 0
	case "show":
		fs := flag.NewFlagSet("mbr artifacts show", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		extensionID := fs.String("extension", "", "Extension ID")
		surface := fs.String("surface", "", "Artifact surface name")
		artifactPath := fs.String("path", "", "Artifact path")
		ref := fs.String("ref", "", "Optional revision ref")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*extensionID) == "" || strings.TrimSpace(*surface) == "" || strings.TrimSpace(*artifactPath) == "" {
			fmt.Fprintln(stderr, "--extension, --surface, and --path are required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		content, err := runExtensionArtifactShow(ctx, client, *extensionID, *surface, *artifactPath, *ref)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			payload := map[string]any{
				"extensionID": *extensionID,
				"surface":     *surface,
				"path":        *artifactPath,
				"content":     content,
			}
			if strings.TrimSpace(*ref) != "" {
				payload["ref"] = strings.TrimSpace(*ref)
			}
			return writeJSON(stdout, payload, stderr)
		}
		if _, err := io.WriteString(stdout, content); err != nil {
			fmt.Fprintf(stderr, "write output: %v\n", err)
			return 1
		}
		if !strings.HasSuffix(content, "\n") {
			if _, err := io.WriteString(stdout, "\n"); err != nil {
				fmt.Fprintf(stderr, "write output: %v\n", err)
				return 1
			}
		}
		return 0
	case "history":
		fs := flag.NewFlagSet("mbr artifacts history", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		extensionID := fs.String("extension", "", "Extension ID")
		surface := fs.String("surface", "", "Artifact surface name")
		artifactPath := fs.String("path", "", "Artifact path")
		limit := fs.Int("limit", 20, "Maximum revisions to return")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*extensionID) == "" || strings.TrimSpace(*surface) == "" || strings.TrimSpace(*artifactPath) == "" {
			fmt.Fprintln(stderr, "--extension, --surface, and --path are required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		revisions, err := runExtensionArtifactHistory(ctx, client, *extensionID, *surface, *artifactPath, *limit)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, revisions, stderr)
		}
		if len(revisions) == 0 {
			fmt.Fprintln(stdout, "no artifact revisions found")
			return 0
		}
		for _, revision := range revisions {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", revision.Ref, revision.CommittedAt, revision.Subject)
		}
		return 0
	case "diff":
		fs := flag.NewFlagSet("mbr artifacts diff", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		extensionID := fs.String("extension", "", "Extension ID")
		surface := fs.String("surface", "", "Artifact surface name")
		artifactPath := fs.String("path", "", "Artifact path")
		fromRevision := fs.String("from", "", "Base revision")
		toRevision := fs.String("to", "", "Target revision")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*extensionID) == "" || strings.TrimSpace(*surface) == "" || strings.TrimSpace(*artifactPath) == "" {
			fmt.Fprintln(stderr, "--extension, --surface, and --path are required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		diff, err := runExtensionArtifactDiff(ctx, client, *extensionID, *surface, *artifactPath, *fromRevision, *toRevision)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, diff, stderr)
		}
		if _, err := io.WriteString(stdout, diff.Patch); err != nil {
			fmt.Fprintf(stderr, "write output: %v\n", err)
			return 1
		}
		if !strings.HasSuffix(diff.Patch, "\n") {
			if _, err := io.WriteString(stdout, "\n"); err != nil {
				fmt.Fprintf(stderr, "write output: %v\n", err)
				return 1
			}
		}
		return 0
	case "publish":
		fs := flag.NewFlagSet("mbr artifacts publish", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		extensionID := fs.String("extension", "", "Extension ID")
		surface := fs.String("surface", "", "Artifact surface name")
		artifactPath := fs.String("path", "", "Artifact path")
		filePath := fs.String("file", "", "Path to local file")
		inlineContent := fs.String("content", "", "Inline content")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*extensionID) == "" || strings.TrimSpace(*surface) == "" || strings.TrimSpace(*artifactPath) == "" {
			fmt.Fprintln(stderr, "--extension, --surface, and --path are required")
			return 2
		}
		content, err := readArtifactPublishContent(*filePath, *inlineContent)
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
		publication, err := runExtensionArtifactPublish(ctx, client, *extensionID, *surface, *artifactPath, content)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, publication, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", publication.Surface, publication.Path, publication.RevisionRef)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown artifacts command %q\n\n", args[0])
		printArtifactsUsage(stderr)
		return 2
	}
}
