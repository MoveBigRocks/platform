package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runAttachments(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printAttachmentsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "upload":
		fs := flag.NewFlagSet("mbr attachments upload", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		caseID := fs.String("case", "", "Optional case ID")
		description := fs.String("description", "", "Optional attachment description")
		contentType := fs.String("content-type", "", "Optional content type override")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":          true,
			"--api-url":      true,
			"--token":        true,
			"--workspace":    true,
			"--case":         true,
			"--description":  true,
			"--content-type": true,
			"--json":         false,
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
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "file path is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		result, err := runAttachmentUpload(ctx, client, positionals[0], cliapi.AttachmentUploadParams{
			WorkspaceID: workspaceValue,
			CaseID:      *caseID,
			Description: *description,
			ContentType: *contentType,
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%d\t%s\n", result.ID, result.WorkspaceID, result.Filename, result.Size, result.Status)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown attachments command %q\n\n", args[0])
		printAttachmentsUsage(stderr)
		return 2
	}
}
