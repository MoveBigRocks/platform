package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const defaultSandboxControlURL = "https://movebigrocks.com"

type sandboxOutput struct {
	ID                   string   `json:"id"`
	Slug                 string   `json:"slug"`
	Name                 string   `json:"name"`
	RequestedEmail       string   `json:"requested_email"`
	Status               string   `json:"status"`
	RuntimeURL           string   `json:"runtime_url"`
	LoginURL             string   `json:"login_url"`
	BootstrapURL         string   `json:"bootstrap_url"`
	ActivationDeadlineAt string   `json:"activation_deadline_at"`
	VerifiedAt           *string  `json:"verified_at,omitempty"`
	ExpiresAt            *string  `json:"expires_at,omitempty"`
	ExtendedAt           *string  `json:"extended_at,omitempty"`
	DestroyedAt          *string  `json:"destroyed_at,omitempty"`
	LastError            string   `json:"last_error"`
	CreatedAt            string   `json:"created_at"`
	UpdatedAt            string   `json:"updated_at"`
	ManageToken          *string  `json:"manage_token,omitempty"`
	VerificationURL      *string  `json:"verification_url,omitempty"`
	NextSteps            []string `json:"next_steps,omitempty"`
}

type sandboxExportOutput struct {
	ExportVersion string         `json:"export_version"`
	GeneratedAt   string         `json:"generated_at"`
	FileName      string         `json:"file_name"`
	ContentType   string         `json:"content_type"`
	Includes      []string       `json:"includes"`
	Omissions     []string       `json:"omissions,omitempty"`
	Bundle        map[string]any `json:"bundle"`
}

func runSandboxes(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printSandboxesUsage(stderr)
		return 2
	}

	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("mbr sandboxes create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		email := fs.String("email", "", "Operator email")
		name := fs.String("name", "", "Optional sandbox name")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*email) == "" {
			fmt.Fprintln(stderr, "--email is required")
			return 2
		}
		payload, err := createSandboxRequest(ctx, sandboxControlURL(*instanceURL), *email, *name)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return printSandboxOutput(stdout, stderr, payload, *jsonOutput)
	case "show":
		sandboxID, flagArgs := splitLeadingID(args[1:])
		fs := flag.NewFlagSet("mbr sandboxes show", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		manageToken := fs.String("manage-token", "", "Sandbox manage token")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if strings.TrimSpace(sandboxID) == "" {
			fmt.Fprintln(stderr, "sandbox ID is required")
			return 2
		}
		if strings.TrimSpace(*manageToken) == "" {
			fmt.Fprintln(stderr, "--manage-token is required")
			return 2
		}
		payload, err := sandboxLifecycleRequest(ctx, http.MethodGet, sandboxControlURL(*instanceURL), sandboxID, *manageToken, "", nil)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return printSandboxOutput(stdout, stderr, payload, *jsonOutput)
	case "extend":
		sandboxID, flagArgs := splitLeadingID(args[1:])
		fs := flag.NewFlagSet("mbr sandboxes extend", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		manageToken := fs.String("manage-token", "", "Sandbox manage token")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if strings.TrimSpace(sandboxID) == "" {
			fmt.Fprintln(stderr, "sandbox ID is required")
			return 2
		}
		if strings.TrimSpace(*manageToken) == "" {
			fmt.Fprintln(stderr, "--manage-token is required")
			return 2
		}
		payload, err := sandboxLifecycleRequest(ctx, http.MethodPost, sandboxControlURL(*instanceURL), sandboxID, *manageToken, "/extend", nil)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return printSandboxOutput(stdout, stderr, payload, *jsonOutput)
	case "destroy":
		sandboxID, flagArgs := splitLeadingID(args[1:])
		fs := flag.NewFlagSet("mbr sandboxes destroy", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		manageToken := fs.String("manage-token", "", "Sandbox manage token")
		reason := fs.String("reason", "", "Optional destroy reason")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if strings.TrimSpace(sandboxID) == "" {
			fmt.Fprintln(stderr, "sandbox ID is required")
			return 2
		}
		if strings.TrimSpace(*manageToken) == "" {
			fmt.Fprintln(stderr, "--manage-token is required")
			return 2
		}
		var requestBody io.Reader
		if strings.TrimSpace(*reason) != "" {
			body, _ := json.Marshal(map[string]any{"reason": strings.TrimSpace(*reason)})
			requestBody = bytes.NewReader(body)
		}
		payload, err := sandboxLifecycleRequest(ctx, http.MethodDelete, sandboxControlURL(*instanceURL), sandboxID, *manageToken, "", requestBody)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return printSandboxOutput(stdout, stderr, payload, *jsonOutput)
	case "export":
		sandboxID, flagArgs := splitLeadingID(args[1:])
		fs := flag.NewFlagSet("mbr sandboxes export", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		manageToken := fs.String("manage-token", "", "Sandbox manage token")
		outputPath := fs.String("out", "", "Write export JSON to a local file")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if strings.TrimSpace(sandboxID) == "" {
			fmt.Fprintln(stderr, "sandbox ID is required")
			return 2
		}
		if strings.TrimSpace(*manageToken) == "" {
			fmt.Fprintln(stderr, "--manage-token is required")
			return 2
		}

		payload, err := sandboxExportRequest(ctx, sandboxControlURL(*instanceURL), sandboxID, *manageToken)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload, stderr)
		}
		if strings.TrimSpace(*outputPath) != "" {
			if err := writeSandboxExportFile(strings.TrimSpace(*outputPath), payload); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			fmt.Fprintf(stdout, "exported:\t%s\n", strings.TrimSpace(*outputPath))
			fmt.Fprintf(stdout, "fileName:\t%s\n", payload.FileName)
			return 0
		}
		formatted, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if _, err := stdout.Write(append(formatted, '\n')); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown sandboxes command %q\n\n", args[0])
		printSandboxesUsage(stderr)
		return 2
	}
}

func printSandboxOutput(stdout, stderr io.Writer, payload sandboxOutput, jsonOutput bool) int {
	if jsonOutput {
		return writeJSON(stdout, payload, stderr)
	}
	fmt.Fprintf(stdout, "id:\t%s\n", payload.ID)
	fmt.Fprintf(stdout, "slug:\t%s\n", payload.Slug)
	fmt.Fprintf(stdout, "status:\t%s\n", payload.Status)
	if payload.RuntimeURL != "" {
		fmt.Fprintf(stdout, "runtimeURL:\t%s\n", payload.RuntimeURL)
	}
	if payload.LoginURL != "" {
		fmt.Fprintf(stdout, "loginURL:\t%s\n", payload.LoginURL)
	}
	if payload.BootstrapURL != "" {
		fmt.Fprintf(stdout, "bootstrapURL:\t%s\n", payload.BootstrapURL)
	}
	if payload.VerificationURL != nil {
		fmt.Fprintf(stdout, "verificationURL:\t%s\n", *payload.VerificationURL)
	}
	if payload.ManageToken != nil {
		fmt.Fprintf(stdout, "manageToken:\t%s\n", *payload.ManageToken)
	}
	if payload.ExpiresAt != nil {
		fmt.Fprintf(stdout, "expiresAt:\t%s\n", *payload.ExpiresAt)
	}
	for _, step := range payload.NextSteps {
		fmt.Fprintf(stdout, "nextStep:\t%s\n", step)
	}
	if payload.LastError != "" {
		fmt.Fprintf(stdout, "lastError:\t%s\n", payload.LastError)
	}
	return 0
}

func createSandboxRequest(ctx context.Context, baseURL, email, name string) (sandboxOutput, error) {
	endpoint, err := endpointURL(baseURL, "/api/public/sandboxes")
	if err != nil {
		return sandboxOutput{}, err
	}
	body, err := json.Marshal(map[string]any{
		"email": strings.TrimSpace(email),
		"name":  strings.TrimSpace(name),
	})
	if err != nil {
		return sandboxOutput{}, fmt.Errorf("encode sandbox request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return sandboxOutput{}, fmt.Errorf("build sandbox request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return doSandboxRequest(req)
}

func sandboxLifecycleRequest(ctx context.Context, method, baseURL, sandboxID, manageToken, suffix string, body io.Reader) (sandboxOutput, error) {
	if strings.TrimSpace(manageToken) == "" {
		return sandboxOutput{}, fmt.Errorf("--manage-token is required")
	}
	endpoint, err := endpointURL(baseURL, "/api/public/sandboxes/"+strings.TrimSpace(sandboxID)+suffix)
	if err != nil {
		return sandboxOutput{}, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return sandboxOutput{}, fmt.Errorf("build sandbox request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(manageToken))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return doSandboxRequest(req)
}

func sandboxExportRequest(ctx context.Context, baseURL, sandboxID, manageToken string) (sandboxExportOutput, error) {
	if strings.TrimSpace(manageToken) == "" {
		return sandboxExportOutput{}, fmt.Errorf("--manage-token is required")
	}
	endpoint, err := endpointURL(baseURL, "/api/public/sandboxes/"+strings.TrimSpace(sandboxID)+"/export")
	if err != nil {
		return sandboxExportOutput{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return sandboxExportOutput{}, fmt.Errorf("build sandbox export request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(manageToken))
	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return sandboxExportOutput{}, fmt.Errorf("sandbox export request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sandboxExportOutput{}, fmt.Errorf("read sandbox export response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sandboxExportOutput{}, fmt.Errorf("sandbox export failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload sandboxExportOutput
	if err := json.Unmarshal(body, &payload); err != nil {
		return sandboxExportOutput{}, fmt.Errorf("decode sandbox export response: %w", err)
	}
	return payload, nil
}

func doSandboxRequest(req *http.Request) (sandboxOutput, error) {
	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return sandboxOutput{}, fmt.Errorf("sandbox request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sandboxOutput{}, fmt.Errorf("read sandbox response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sandboxOutput{}, fmt.Errorf("sandbox request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload sandboxOutput
	if err := json.Unmarshal(body, &payload); err != nil {
		return sandboxOutput{}, fmt.Errorf("decode sandbox response: %w", err)
	}
	return payload, nil
}

func sandboxControlURL(raw string) string {
	if value := strings.TrimSpace(raw); value != "" {
		return value
	}
	return defaultSandboxControlURL
}

func writeSandboxExportFile(path string, payload sandboxExportOutput) error {
	formatted, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sandbox export: %w", err)
	}
	if err := os.WriteFile(path, append(formatted, '\n'), 0o600); err != nil {
		return fmt.Errorf("write sandbox export file: %w", err)
	}
	return nil
}

func splitLeadingID(args []string) (string, []string) {
	if len(args) == 0 {
		return "", args
	}
	if strings.HasPrefix(args[0], "-") {
		return "", args
	}
	return strings.TrimSpace(args[0]), args[1:]
}
