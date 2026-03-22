package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runHealth(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printHealthUsage(stderr)
		return 2
	}

	switch args[0] {
	case "check":
		fs := flag.NewFlagSet("mbr health check", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
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

		healthURL, err := normalizeHealthURL(instanceURLValue)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		result, err := runHealthCheck(ctx, healthURL)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		cfg, err := loadCLIConfig(instanceURLValue, strings.TrimSpace(*token))
		if err == nil {
			client := newCLIClient(cfg)
			authResult, err := runHealthAuthCheck(ctx, client)
			if err != nil {
				result.AuthOK = false
				message := err.Error()
				result.AuthMessage = &message
			} else {
				result.AuthOK = true
				result.PrincipalType = authResult.Type
				result.PrincipalID = authResult.ID
			}
		}

		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}

		authStatus := "skipped"
		if result.AuthOK {
			authStatus = "ok"
		} else if result.AuthMessage != nil {
			authStatus = "failed"
		}
		fmt.Fprintf(stdout, "status:\t%s\n", result.Status)
		fmt.Fprintf(stdout, "service:\t%s\n", result.Service)
		fmt.Fprintf(stdout, "healthURL:\t%s\n", result.HealthURL)
		fmt.Fprintf(stdout, "httpStatus:\t%d\n", result.HTTPStatus)
		fmt.Fprintf(stdout, "auth:\t%s\n", authStatus)
		if result.PrincipalType != "" {
			fmt.Fprintf(stdout, "principal:\t%s %s\n", result.PrincipalType, result.PrincipalID)
		}
		if result.AuthMessage != nil {
			fmt.Fprintf(stdout, "authError:\t%s\n", *result.AuthMessage)
		}
		if result.InstanceID != nil && strings.TrimSpace(*result.InstanceID) != "" {
			fmt.Fprintf(stdout, "instanceID:\t%s\n", *result.InstanceID)
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown health command %q\n\n", args[0])
		printHealthUsage(stderr)
		return 2
	}
}

func runHealthCheck(ctx context.Context, healthURL string) (healthCheckResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return healthCheckResult{}, fmt.Errorf("build health request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return healthCheckResult{}, fmt.Errorf("perform health request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return healthCheckResult{}, fmt.Errorf("read health response: %w", err)
	}

	var payload struct {
		Status    string  `json:"status"`
		Service   string  `json:"service"`
		Version   *string `json:"version"`
		GitCommit *string `json:"git_commit"`
		BuildDate *string `json:"build_date"`
		Build     *struct {
			Version    *string `json:"version"`
			GitCommit  *string `json:"git_commit"`
			BuildDate  *string `json:"build_date"`
			InstanceID *string `json:"instance_id"`
		} `json:"build"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return healthCheckResult{}, fmt.Errorf("decode health response: %w", err)
	}
	version := payload.Version
	gitCommit := payload.GitCommit
	buildDate := payload.BuildDate
	var instanceID *string
	if payload.Build != nil {
		if version == nil {
			version = payload.Build.Version
		}
		if gitCommit == nil {
			gitCommit = payload.Build.GitCommit
		}
		if buildDate == nil {
			buildDate = payload.Build.BuildDate
		}
		instanceID = payload.Build.InstanceID
	}

	return healthCheckResult{
		HealthURL:  healthURL,
		HTTPStatus: resp.StatusCode,
		Status:     payload.Status,
		Service:    payload.Service,
		Version:    version,
		GitCommit:  gitCommit,
		BuildDate:  buildDate,
		InstanceID: instanceID,
	}, nil
}

func runHealthAuthCheck(ctx context.Context, client *cliapi.Client) (healthAuthResult, error) {
	var payload struct {
		Me struct {
			TypeName string `json:"__typename"`
			ID       string `json:"id"`
		} `json:"me"`
	}
	err := client.Query(ctx, `
		query CLIHealthAuth {
		  me {
		    __typename
		    ... on User {
		      id
		    }
		    ... on Agent {
		      id
		    }
		  }
		}
	`, nil, &payload)
	if err != nil {
		return healthAuthResult{}, err
	}
	return healthAuthResult{
		Type: payload.Me.TypeName,
		ID:   payload.Me.ID,
	}, nil
}

func normalizeHealthURL(raw string) (string, error) {
	apiBaseURL, err := cliapi.NormalizeAPIBaseURL(raw)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(apiBaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid API base URL: %w", err)
	}
	u.Path = "/health"
	return u.String(), nil
}
