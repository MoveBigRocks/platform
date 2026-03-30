package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runAuth(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printAuthUsage(stderr)
		return 2
	}

	switch args[0] {
	case "whoami":
		fs := flag.NewFlagSet("mbr auth whoami", flag.ContinueOnError)
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
			Me struct {
				TypeName  string  `json:"__typename"`
				ID        string  `json:"id"`
				Email     string  `json:"email"`
				Name      string  `json:"name"`
				Workspace string  `json:"workspaceID"`
				Status    *string `json:"status"`
			} `json:"me"`
		}
		err = client.Query(ctx, `
			query CLIWhoAmI {
			  me {
			    __typename
			    ... on User {
			      id
			      email
			      name
			    }
			    ... on Agent {
			      id
			      workspaceID
			      name
			      status
			    }
			  }
			}
		`, nil, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		if *jsonOutput {
			return writeJSON(stdout, map[string]any{
				"type":       payload.Me.TypeName,
				"id":         payload.Me.ID,
				"email":      payload.Me.Email,
				"name":       payload.Me.Name,
				"workspace":  payload.Me.Workspace,
				"status":     payload.Me.Status,
				"graphqlURL": cfg.GraphQLURL,
			}, stderr)
		}

		switch payload.Me.TypeName {
		case "User":
			fmt.Fprintf(stdout, "user %s <%s>\n", payload.Me.Name, payload.Me.Email)
		case "Agent":
			status := ""
			if payload.Me.Status != nil {
				status = *payload.Me.Status
			}
			fmt.Fprintf(stdout, "agent %s (%s) workspace=%s\n", payload.Me.Name, status, payload.Me.Workspace)
		default:
			fmt.Fprintf(stdout, "%s %s\n", payload.Me.TypeName, payload.Me.ID)
		}
		return 0
	case "login":
		fs := flag.NewFlagSet("mbr auth login", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		tokenStdin := fs.Bool("token-stdin", false, "Read the bearer token from stdin")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}

		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		instanceURLValue := resolveStoredInstanceURL(*instanceURL, stored)
		if instanceURLValue == "" {
			fmt.Fprintf(stderr, "missing Move Big Rocks URL: pass --url or set %s\n", cliapi.EnvInstanceURL)
			return 2
		}

		tokenValue := strings.TrimSpace(*token)
		if *tokenStdin {
			stdinBytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(stderr, "read token from stdin: %v\n", err)
				return 1
			}
			tokenValue = strings.TrimSpace(string(stdinBytes))
		}
		if tokenValue == "" {
			tokenValue = strings.TrimSpace(os.Getenv(cliapi.EnvToken))
		}

		if tokenValue == "" {
			return runBrowserLogin(ctx, instanceURLValue, *jsonOutput, stdout, stderr)
		}

		cfg, err := loadCLIConfig(instanceURLValue, tokenValue)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		authResult, err := runHealthAuthCheck(ctx, client)
		if err != nil {
			fmt.Fprintf(stderr, "validate credentials: %v\n", err)
			return 1
		}
		configPath, err := cliapi.SaveStoredConfig(instanceURLValue, tokenValue)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		if *jsonOutput {
			return writeJSON(stdout, map[string]any{
				"instanceURL":   cfg.InstanceURL,
				"apiBaseURL":    cfg.APIBaseURL,
				"adminBaseURL":  cfg.AdminBaseURL,
				"graphqlURL":    cfg.GraphQLURL,
				"configPath":    configPath,
				"principalType": authResult.Type,
				"principalID":   authResult.ID,
			}, stderr)
		}

		fmt.Fprintf(stdout, "logged in\t%s %s\n", authResult.Type, authResult.ID)
		fmt.Fprintf(stdout, "configPath:\t%s\n", configPath)
		fmt.Fprintf(stdout, "instanceURL:\t%s\n", cfg.InstanceURL)
		fmt.Fprintf(stdout, "graphqlURL:\t%s\n", cfg.GraphQLURL)
		return 0
	case "logout":
		if err := cliapi.ClearStoredConfig(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "logged out")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown auth command %q\n\n", args[0])
		printAuthUsage(stderr)
		return 2
	}
}

func runBrowserLogin(ctx context.Context, instanceURL string, jsonOutput bool, stdout, stderr io.Writer) int {
	start, err := startBrowserLogin(ctx, instanceURL)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if err := openBrowserURL(start.AuthorizeURL); err != nil {
		fmt.Fprintf(stderr, "open browser: %v\n", err)
		fmt.Fprintf(stderr, "complete login in your browser by visiting:\n%s\n", start.AuthorizeURL)
	} else if !jsonOutput {
		fmt.Fprintf(stdout, "opening browser:\t%s\n", start.AuthorizeURL)
	}

	pollInterval := 2 * time.Second
	if start.IntervalSeconds > 0 {
		pollInterval = time.Duration(start.IntervalSeconds) * time.Second
	}
	deadline := time.Now().Add(10 * time.Minute)
	for {
		if time.Now().After(deadline) {
			fmt.Fprintln(stderr, "browser login timed out")
			return 1
		}

		result, err := pollBrowserLogin(ctx, instanceURL, start.PollToken)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if result.Status == "ready" {
			configPath, err := cliapi.SaveStoredSessionConfig(instanceURL, start.AdminBaseURL, result.SessionToken)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			cfg, err := loadCLIConfig(instanceURL, "")
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			client := newCLIClient(cfg)
			authResult, err := runHealthAuthCheck(ctx, client)
			if err != nil {
				fmt.Fprintf(stderr, "validate browser login: %v\n", err)
				return 1
			}
			if jsonOutput {
				return writeJSON(stdout, map[string]any{
					"instanceURL":    cfg.InstanceURL,
					"apiBaseURL":     cfg.APIBaseURL,
					"adminBaseURL":   start.AdminBaseURL,
					"graphqlURL":     cfg.GraphQLURL,
					"configPath":     configPath,
					"principalType":  authResult.Type,
					"principalID":    authResult.ID,
					"loginTransport": "browser_session",
				}, stderr)
			}
			fmt.Fprintf(stdout, "logged in\t%s %s\n", authResult.Type, authResult.ID)
			fmt.Fprintf(stdout, "configPath:\t%s\n", configPath)
			fmt.Fprintf(stdout, "instanceURL:\t%s\n", cfg.InstanceURL)
			fmt.Fprintf(stdout, "graphqlURL:\t%s\n", cfg.GraphQLURL)
			return 0
		}

		select {
		case <-ctx.Done():
			fmt.Fprintln(stderr, ctx.Err())
			return 1
		case <-time.After(pollInterval):
		}
	}
}

func startBrowserLogin(ctx context.Context, instanceURL string) (browserLoginStartResponse, error) {
	loginURL, err := endpointURL(instanceURL, "/auth/cli/start")
	if err != nil {
		return browserLoginStartResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(`{}`))
	if err != nil {
		return browserLoginStartResponse{}, fmt.Errorf("build browser login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClientFromContext(ctx).Do(req)
	if err != nil {
		return browserLoginStartResponse{}, fmt.Errorf("start browser login: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return browserLoginStartResponse{}, fmt.Errorf("read browser login response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return browserLoginStartResponse{}, fmt.Errorf("start browser login: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload browserLoginStartResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return browserLoginStartResponse{}, fmt.Errorf("decode browser login response: %w", err)
	}
	if strings.TrimSpace(payload.PollToken) == "" || strings.TrimSpace(payload.AuthorizeURL) == "" {
		return browserLoginStartResponse{}, fmt.Errorf("browser login response missing required fields")
	}
	return payload, nil
}

func pollBrowserLogin(ctx context.Context, instanceURL, pollToken string) (browserLoginPollResponse, error) {
	pollURL, err := endpointURL(instanceURL, "/auth/cli/poll")
	if err != nil {
		return browserLoginPollResponse{}, err
	}
	body, err := json.Marshal(map[string]any{"pollToken": pollToken})
	if err != nil {
		return browserLoginPollResponse{}, fmt.Errorf("encode browser login poll: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pollURL, strings.NewReader(string(body)))
	if err != nil {
		return browserLoginPollResponse{}, fmt.Errorf("build browser login poll: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClientFromContext(ctx).Do(req)
	if err != nil {
		return browserLoginPollResponse{}, fmt.Errorf("poll browser login: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return browserLoginPollResponse{}, fmt.Errorf("read browser login poll response: %w", err)
	}
	if resp.StatusCode == http.StatusGone {
		return browserLoginPollResponse{}, fmt.Errorf("browser login expired")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return browserLoginPollResponse{}, fmt.Errorf("poll browser login: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var payload browserLoginPollResponse
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return browserLoginPollResponse{}, fmt.Errorf("decode browser login poll response: %w", err)
	}
	if strings.TrimSpace(payload.Status) == "" {
		return browserLoginPollResponse{}, fmt.Errorf("browser login poll response missing status")
	}
	return payload, nil
}

func endpointURL(rawURL, path string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("invalid Move Big Rocks URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("Move Big Rocks URL must include scheme and host")
	}
	u.Path = path
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}
