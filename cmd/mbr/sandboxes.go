package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

type sandboxCLIState struct {
	ID                   string   `json:"id"`
	Slug                 string   `json:"slug"`
	Name                 string   `json:"name"`
	RequestedEmail       string   `json:"requested_email"`
	Status               string   `json:"status"`
	RuntimeURL           string   `json:"runtime_url"`
	LoginURL             string   `json:"login_url"`
	BootstrapURL         string   `json:"bootstrap_url"`
	VerificationURL      string   `json:"verification_url,omitempty"`
	ManageToken          string   `json:"manage_token,omitempty"`
	ActivationDeadlineAt string   `json:"activation_deadline_at"`
	VerifiedAt           *string  `json:"verified_at,omitempty"`
	ExpiresAt            *string  `json:"expires_at,omitempty"`
	ExpiredAt            *string  `json:"expired_at,omitempty"`
	ExtendedAt           *string  `json:"extended_at,omitempty"`
	DestroyedAt          *string  `json:"destroyed_at,omitempty"`
	LastError            string   `json:"last_error,omitempty"`
	CreatedAt            string   `json:"created_at"`
	UpdatedAt            string   `json:"updated_at"`
	NextSteps            []string `json:"next_steps,omitempty"`
}

type sandboxExportCLIResult struct {
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
		email := fs.String("email", "", "Email address used for the sandbox request")
		name := fs.String("name", "", "Optional display name for the sandbox")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if fs.NArg() != 0 {
			fmt.Fprintln(stderr, "unexpected arguments")
			return 2
		}
		if strings.TrimSpace(*email) == "" {
			fmt.Fprintln(stderr, "--email is required")
			return 2
		}

		baseURL, err := resolveSandboxBaseURL(*instanceURL)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		var result sandboxCLIState
		if err := sandboxAPIDecode(ctx, http.MethodPost, baseURL, "/api/public/sandboxes", "", map[string]any{
			"email": strings.TrimSpace(*email),
			"name":  strings.TrimSpace(*name),
		}, &result); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		writeSandboxState(stdout, result)
		return 0
	case "show":
		fs := flag.NewFlagSet("mbr sandboxes show", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		manageToken := fs.String("manage-token", "", "Sandbox manage token")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		sandboxID, parseArgs := splitLeadingSandboxID(args[1:])
		if err := fs.Parse(parseArgs); err != nil {
			return 2
		}
		if sandboxID == "" {
			if fs.NArg() != 1 {
				fmt.Fprintln(stderr, "usage: mbr sandboxes show SANDBOX_ID --manage-token TOKEN [--url URL] [--json]")
				return 2
			}
			sandboxID = strings.TrimSpace(fs.Arg(0))
		} else if fs.NArg() != 0 {
			fmt.Fprintln(stderr, "usage: mbr sandboxes show SANDBOX_ID --manage-token TOKEN [--url URL] [--json]")
			return 2
		}
		if strings.TrimSpace(*manageToken) == "" {
			fmt.Fprintln(stderr, "--manage-token is required")
			return 2
		}

		baseURL, err := resolveSandboxBaseURL(*instanceURL)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		var result sandboxCLIState
		if err := sandboxAPIDecode(ctx, http.MethodGet, baseURL, "/api/public/sandboxes/"+sandboxID, strings.TrimSpace(*manageToken), nil, &result); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		writeSandboxState(stdout, result)
		return 0
	case "extend":
		fs := flag.NewFlagSet("mbr sandboxes extend", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		manageToken := fs.String("manage-token", "", "Sandbox manage token")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		sandboxID, parseArgs := splitLeadingSandboxID(args[1:])
		if err := fs.Parse(parseArgs); err != nil {
			return 2
		}
		if sandboxID == "" {
			if fs.NArg() != 1 {
				fmt.Fprintln(stderr, "usage: mbr sandboxes extend SANDBOX_ID --manage-token TOKEN [--url URL] [--json]")
				return 2
			}
			sandboxID = strings.TrimSpace(fs.Arg(0))
		} else if fs.NArg() != 0 {
			fmt.Fprintln(stderr, "usage: mbr sandboxes extend SANDBOX_ID --manage-token TOKEN [--url URL] [--json]")
			return 2
		}
		if strings.TrimSpace(*manageToken) == "" {
			fmt.Fprintln(stderr, "--manage-token is required")
			return 2
		}

		baseURL, err := resolveSandboxBaseURL(*instanceURL)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		var result sandboxCLIState
		if err := sandboxAPIDecode(ctx, http.MethodPost, baseURL, "/api/public/sandboxes/"+sandboxID+"/extend", strings.TrimSpace(*manageToken), nil, &result); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		writeSandboxState(stdout, result)
		return 0
	case "export":
		fs := flag.NewFlagSet("mbr sandboxes export", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		manageToken := fs.String("manage-token", "", "Sandbox manage token")
		outPath := fs.String("out", "", "Write the export bundle to PATH")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		sandboxID, parseArgs := splitLeadingSandboxID(args[1:])
		if err := fs.Parse(parseArgs); err != nil {
			return 2
		}
		if sandboxID == "" {
			if fs.NArg() != 1 {
				fmt.Fprintln(stderr, "usage: mbr sandboxes export SANDBOX_ID --manage-token TOKEN [--out PATH] [--url URL] [--json]")
				return 2
			}
			sandboxID = strings.TrimSpace(fs.Arg(0))
		} else if fs.NArg() != 0 {
			fmt.Fprintln(stderr, "usage: mbr sandboxes export SANDBOX_ID --manage-token TOKEN [--out PATH] [--url URL] [--json]")
			return 2
		}
		if strings.TrimSpace(*manageToken) == "" {
			fmt.Fprintln(stderr, "--manage-token is required")
			return 2
		}

		baseURL, err := resolveSandboxBaseURL(*instanceURL)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		body, err := sandboxAPIRawRequest(ctx, http.MethodGet, baseURL, "/api/public/sandboxes/"+sandboxID+"/export", strings.TrimSpace(*manageToken), nil)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		writtenPath := ""
		if strings.TrimSpace(*outPath) != "" {
			writtenPath, err = writeSandboxExportFile(*outPath, body)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}

		if *jsonOutput {
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				fmt.Fprintf(stderr, "decode sandbox export: %v\n", err)
				return 1
			}
			if writtenPath != "" {
				payload["out"] = writtenPath
			}
			return writeJSON(stdout, payload, stderr)
		}

		var result sandboxExportCLIResult
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Fprintf(stderr, "decode sandbox export: %v\n", err)
			return 1
		}
		writeSandboxExportSummary(stdout, result, writtenPath)
		return 0
	case "destroy":
		fs := flag.NewFlagSet("mbr sandboxes destroy", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		manageToken := fs.String("manage-token", "", "Sandbox manage token")
		reason := fs.String("reason", "", "Optional destroy reason")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		sandboxID, parseArgs := splitLeadingSandboxID(args[1:])
		if err := fs.Parse(parseArgs); err != nil {
			return 2
		}
		if sandboxID == "" {
			if fs.NArg() != 1 {
				fmt.Fprintln(stderr, "usage: mbr sandboxes destroy SANDBOX_ID --manage-token TOKEN [--reason TEXT] [--url URL] [--json]")
				return 2
			}
			sandboxID = strings.TrimSpace(fs.Arg(0))
		} else if fs.NArg() != 0 {
			fmt.Fprintln(stderr, "usage: mbr sandboxes destroy SANDBOX_ID --manage-token TOKEN [--reason TEXT] [--url URL] [--json]")
			return 2
		}
		if strings.TrimSpace(*manageToken) == "" {
			fmt.Fprintln(stderr, "--manage-token is required")
			return 2
		}

		baseURL, err := resolveSandboxBaseURL(*instanceURL)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		var result sandboxCLIState
		if err := sandboxAPIDecode(ctx, http.MethodDelete, baseURL, "/api/public/sandboxes/"+sandboxID, strings.TrimSpace(*manageToken), map[string]any{
			"reason": strings.TrimSpace(*reason),
		}, &result); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		writeSandboxState(stdout, result)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown sandboxes command %q\n\n", args[0])
		printSandboxesUsage(stderr)
		return 2
	}
}

func resolveSandboxBaseURL(flagValue string) (string, error) {
	stored, err := cliapi.LoadStoredConfig()
	if err != nil {
		return "", err
	}
	rawURL := resolveStoredInstanceURL(flagValue, stored)
	if rawURL == "" {
		return "", fmt.Errorf("missing Move Big Rocks URL: pass --url or set %s", cliapi.EnvInstanceURL)
	}
	return normalizeSandboxBaseURL(rawURL)
}

func normalizeSandboxBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("Move Big Rocks URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid Move Big Rocks URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("Move Big Rocks URL must include scheme and host")
	}

	host := u.Hostname()
	switch {
	case host == "localhost" || host == "127.0.0.1" || host == "::1":
	case strings.HasPrefix(host, "api."):
		u.Host = strings.TrimPrefix(u.Host, "api.")
	case strings.HasPrefix(host, "admin."):
		u.Host = strings.TrimPrefix(u.Host, "admin.")
	}

	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func sandboxAPIDecode(ctx context.Context, method, baseURL, path, manageToken string, requestBody any, out any) error {
	body, err := sandboxAPIRawRequest(ctx, method, baseURL, path, manageToken, requestBody)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode sandbox response: %w", err)
	}
	return nil
}

func sandboxAPIRawRequest(ctx context.Context, method, baseURL, path, manageToken string, requestBody any) ([]byte, error) {
	requestURL, err := sandboxRequestURL(baseURL, path)
	if err != nil {
		return nil, err
	}

	var requestReader io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("encode sandbox request: %w", err)
		}
		requestReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, requestReader)
	if err != nil {
		return nil, fmt.Errorf("build sandbox request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(manageToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(manageToken))
	}

	resp, err := httpClientFromContext(ctx).Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform sandbox request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read sandbox response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		var payload struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Error) != "" {
			return nil, fmt.Errorf("%s", payload.Error)
		}
		return nil, fmt.Errorf("sandbox request failed: %s", strings.TrimSpace(string(body)))
	}
	return body, nil
}

func sandboxRequestURL(baseURL, path string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("Move Big Rocks URL is required")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid Move Big Rocks URL: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func writeSandboxExportFile(path string, body []byte) (string, error) {
	resolvedPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", fmt.Errorf("resolve export path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return "", fmt.Errorf("create export directory: %w", err)
	}
	if err := os.WriteFile(resolvedPath, body, 0o600); err != nil {
		return "", fmt.Errorf("write sandbox export: %w", err)
	}
	return resolvedPath, nil
}

func writeSandboxState(w io.Writer, state sandboxCLIState) {
	fmt.Fprintf(w, "id:\t%s\n", state.ID)
	fmt.Fprintf(w, "slug:\t%s\n", state.Slug)
	fmt.Fprintf(w, "status:\t%s\n", state.Status)
	if state.Name != "" {
		fmt.Fprintf(w, "name:\t%s\n", state.Name)
	}
	if state.RequestedEmail != "" {
		fmt.Fprintf(w, "requestedEmail:\t%s\n", state.RequestedEmail)
	}
	if state.ManageToken != "" {
		fmt.Fprintf(w, "manageToken:\t%s\n", state.ManageToken)
	}
	if state.VerificationURL != "" {
		fmt.Fprintf(w, "verificationURL:\t%s\n", state.VerificationURL)
	}
	if state.RuntimeURL != "" {
		fmt.Fprintf(w, "runtimeURL:\t%s\n", state.RuntimeURL)
	}
	if state.LoginURL != "" {
		fmt.Fprintf(w, "loginURL:\t%s\n", state.LoginURL)
	}
	if state.BootstrapURL != "" {
		fmt.Fprintf(w, "bootstrapURL:\t%s\n", state.BootstrapURL)
	}
	if state.ActivationDeadlineAt != "" {
		fmt.Fprintf(w, "activationDeadlineAt:\t%s\n", state.ActivationDeadlineAt)
	}
	if state.VerifiedAt != nil {
		fmt.Fprintf(w, "verifiedAt:\t%s\n", *state.VerifiedAt)
	}
	if state.ExpiresAt != nil {
		fmt.Fprintf(w, "expiresAt:\t%s\n", *state.ExpiresAt)
	}
	if state.ExpiredAt != nil {
		fmt.Fprintf(w, "expiredAt:\t%s\n", *state.ExpiredAt)
	}
	if state.ExtendedAt != nil {
		fmt.Fprintf(w, "extendedAt:\t%s\n", *state.ExtendedAt)
	}
	if state.DestroyedAt != nil {
		fmt.Fprintf(w, "destroyedAt:\t%s\n", *state.DestroyedAt)
	}
	if state.LastError != "" {
		fmt.Fprintf(w, "lastError:\t%s\n", state.LastError)
	}
	if len(state.NextSteps) > 0 {
		fmt.Fprintf(w, "nextSteps:\t%s\n", strings.Join(state.NextSteps, " | "))
	}
}

func writeSandboxExportSummary(w io.Writer, result sandboxExportCLIResult, outPath string) {
	fmt.Fprintf(w, "exportVersion:\t%s\n", result.ExportVersion)
	fmt.Fprintf(w, "fileName:\t%s\n", result.FileName)
	fmt.Fprintf(w, "contentType:\t%s\n", result.ContentType)
	if result.GeneratedAt != "" {
		fmt.Fprintf(w, "generatedAt:\t%s\n", result.GeneratedAt)
	}
	if len(result.Includes) > 0 {
		fmt.Fprintf(w, "includes:\t%s\n", strings.Join(result.Includes, ", "))
	}
	if len(result.Omissions) > 0 {
		fmt.Fprintf(w, "omissions:\t%s\n", strings.Join(result.Omissions, ", "))
	}
	if outPath != "" {
		fmt.Fprintf(w, "out:\t%s\n", outPath)
	}
}

func splitLeadingSandboxID(args []string) (string, []string) {
	if len(args) == 0 {
		return "", args
	}
	if strings.HasPrefix(strings.TrimSpace(args[0]), "-") {
		return "", args
	}
	return strings.TrimSpace(args[0]), args[1:]
}
