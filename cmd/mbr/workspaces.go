package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runWorkspaces(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printWorkspacesUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr workspaces list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		var payload struct {
			Workspaces []workspaceOutput `json:"workspaces"`
		}
		err = client.Query(ctx, `
			query CLIWorkspaces {
			  workspaces {
			    id
			    name
			    shortCode
			  }
			}
		`, nil, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		if *jsonOutput {
			return writeJSON(stdout, payload.Workspaces, stderr)
		}
		if len(payload.Workspaces) == 0 {
			fmt.Fprintln(stdout, "no workspaces found")
			return 0
		}
		for _, workspace := range payload.Workspaces {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", workspace.ID, workspace.Name, workspace.ShortCode)
		}
		return 0
	case "create":
		fs := flag.NewFlagSet("mbr workspaces create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		name := fs.String("name", "", "Workspace name")
		slug := fs.String("slug", "", "Workspace slug")
		description := fs.String("description", "", "Workspace description")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*name) == "" {
			fmt.Fprintln(stderr, "--name is required")
			return 2
		}
		if strings.TrimSpace(*slug) == "" {
			fmt.Fprintln(stderr, "--slug is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		workspace, err := createWorkspace(ctx, cfg, workspaceCreateInput{
			Name:        *name,
			Slug:        *slug,
			Description: *description,
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, workspace, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", workspace.ID, workspace.Name, workspace.ShortCode)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown workspaces command %q\n\n", args[0])
		printWorkspacesUsage(stderr)
		return 2
	}
}

func createWorkspace(ctx context.Context, cfg cliapi.Config, input workspaceCreateInput) (workspaceOutput, error) {
	if cfg.AuthMode != cliapi.AuthModeSession {
		return workspaceOutput{}, fmt.Errorf("workspaces create requires browser login or session-backed auth")
	}
	adminURL, err := adminActionURL(cfg.AdminBaseURL, "/actions/workspaces")
	if err != nil {
		return workspaceOutput{}, err
	}
	body, err := json.Marshal(input)
	if err != nil {
		return workspaceOutput{}, fmt.Errorf("encode workspace request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, adminURL, strings.NewReader(string(body)))
	if err != nil {
		return workspaceOutput{}, fmt.Errorf("build workspace request: %w", err)
	}
	cfg.ApplyAuth(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return workspaceOutput{}, fmt.Errorf("perform workspace request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return workspaceOutput{}, fmt.Errorf("read workspace response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = resp.Status
		}
		return workspaceOutput{}, fmt.Errorf("create workspace: %s", message)
	}

	var response struct {
		ID          string  `json:"ID"`
		Name        string  `json:"Name"`
		Slug        string  `json:"Slug"`
		Description *string `json:"Description,omitempty"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return workspaceOutput{}, fmt.Errorf("decode workspace response: %w", err)
	}
	return workspaceOutput{
		ID:          response.ID,
		Name:        response.Name,
		ShortCode:   response.Slug,
		Description: response.Description,
	}, nil
}
