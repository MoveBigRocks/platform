package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func TestRunAgentsCreateJSON(t *testing.T) {
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_SESSION_TOKEN", "session_cli")

	previousClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/graphql" {
					t.Fatalf("unexpected graphql path %q", r.URL.Path)
				}
				cookie, err := r.Cookie("mbr_session")
				if err != nil {
					t.Fatalf("expected session cookie: %v", err)
				}
				if cookie.Value != "session_cli" {
					t.Fatalf("unexpected session cookie %q", cookie.Value)
				}

				payload := decodeGraphQLPayload(t, r.Body)
				if !strings.Contains(payload.Query, "createAgent") {
					t.Fatalf("expected createAgent mutation, got %q", payload.Query)
				}
				input := payload.Variables["input"].(map[string]any)
				if got := input["workspaceID"]; got != "ws_123" {
					t.Fatalf("unexpected workspace ID %#v", got)
				}
				if got := input["name"]; got != "Support Router" {
					t.Fatalf("unexpected agent name %#v", got)
				}
				if got := input["description"]; got != "Routes support work" {
					t.Fatalf("unexpected description %#v", got)
				}

				return graphQLSuccess(`{
					"createAgent": {
						"id":"agent_123",
						"workspaceID":"ws_123",
						"name":"Support Router",
						"description":"Routes support work",
						"ownerID":"user_123",
						"status":"active",
						"statusReason":null,
						"createdAt":"2026-03-22T10:00:00Z",
						"updatedAt":"2026-03-22T10:00:00Z",
						"createdByID":"user_123"
					}
				}`)
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() { newCLIClient = previousClient })

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"agents", "create",
		"--workspace", "ws_123",
		"--name", "Support Router",
		"--description", "Routes support work",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if got := payload["id"]; got != "agent_123" {
		t.Fatalf("unexpected id %#v", got)
	}
}

func TestRunAgentTokensCreateJSON(t *testing.T) {
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_SESSION_TOKEN", "session_cli")

	previousClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				payload := decodeGraphQLPayload(t, r.Body)
				if !strings.Contains(payload.Query, "createAgentToken") {
					t.Fatalf("expected createAgentToken mutation, got %q", payload.Query)
				}
				input := payload.Variables["input"].(map[string]any)
				if got := input["agentID"]; got != "agent_123" {
					t.Fatalf("unexpected agent ID %#v", got)
				}
				if got := input["name"]; got != "sandbox-token" {
					t.Fatalf("unexpected token name %#v", got)
				}
				if got := input["expiresInDays"]; got != float64(7) && got != 7 {
					t.Fatalf("unexpected expiresInDays %#v", got)
				}

				return graphQLSuccess(`{
					"createAgentToken": {
						"plaintextToken":"hat_abc123",
						"token": {
							"id":"agtok_123",
							"agentID":"agent_123",
							"tokenPrefix":"hat_abc123",
							"name":"sandbox-token",
							"expiresAt":"2026-03-29T10:00:00Z",
							"revokedAt":null,
							"lastUsedAt":null,
							"lastUsedIP":null,
							"useCount":0,
							"createdAt":"2026-03-22T10:00:00Z",
							"createdByID":"user_123"
						}
					}
				}`)
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() { newCLIClient = previousClient })

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"agents", "tokens", "create", "agent_123",
		"--name", "sandbox-token",
		"--expires-in-days", "7",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if got := payload["plaintextToken"]; got != "hat_abc123" {
		t.Fatalf("unexpected plaintext token %#v", got)
	}
}

func TestRunAgentMembershipsGrantJSON(t *testing.T) {
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_SESSION_TOKEN", "session_cli")

	previousClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				payload := decodeGraphQLPayload(t, r.Body)
				if !strings.Contains(payload.Query, "grantAgentMembership") {
					t.Fatalf("expected grantAgentMembership mutation, got %q", payload.Query)
				}
				input := payload.Variables["input"].(map[string]any)
				if got := input["workspaceID"]; got != "ws_123" {
					t.Fatalf("unexpected workspace ID %#v", got)
				}
				if got := input["agentID"]; got != "agent_123" {
					t.Fatalf("unexpected agent ID %#v", got)
				}
				if got := input["role"]; got != "operator" {
					t.Fatalf("unexpected role %#v", got)
				}
				permissions := input["permissions"].([]any)
				if len(permissions) != 2 || permissions[0] != "conversation:write" || permissions[1] != "queue:read" {
					t.Fatalf("unexpected permissions %#v", permissions)
				}

				constraints := input["constraints"].(map[string]any)
				if got := constraints["allowDelegatedRouting"]; got != true {
					t.Fatalf("unexpected allowDelegatedRouting %#v", got)
				}
				allowedTeams := constraints["allowedTeamIDs"].([]any)
				if len(allowedTeams) != 1 || allowedTeams[0] != "team_support" {
					t.Fatalf("unexpected allowed teams %#v", allowedTeams)
				}
				delegatedTeams := constraints["delegatedRoutingTeamIDs"].([]any)
				if len(delegatedTeams) != 2 || delegatedTeams[0] != "team_support" || delegatedTeams[1] != "team_billing" {
					t.Fatalf("unexpected delegated teams %#v", delegatedTeams)
				}
				activeDays := constraints["activeDays"].([]any)
				if len(activeDays) != 3 || activeDays[0] != float64(1) || activeDays[1] != float64(2) || activeDays[2] != float64(5) {
					t.Fatalf("unexpected active days %#v", activeDays)
				}

				return graphQLSuccess(`{
					"grantAgentMembership": {
						"id":"mship_123",
						"workspaceID":"ws_123",
						"principalID":"agent_123",
						"principalType":"agent",
						"role":"operator",
						"permissions":["queue:read","conversation:write"],
						"constraints":{
							"rateLimitPerMinute":null,
							"rateLimitPerHour":null,
							"allowedIPs":[],
							"allowedProjectIDs":[],
							"allowedTeamIDs":["team_support"],
							"allowDelegatedRouting":true,
							"delegatedRoutingTeamIDs":["team_support","team_billing"],
							"activeHoursStart":null,
							"activeHoursEnd":null,
							"activeTimezone":null,
							"activeDays":[1,2,5]
						},
						"grantedAt":"2026-03-22T10:00:00Z",
						"expiresAt":null,
						"revokedAt":null
					}
				}`)
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() { newCLIClient = previousClient })

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"agents", "memberships", "grant", "agent_123",
		"--workspace", "ws_123",
		"--input-json", `{"role":"operator","permissions":["queue:read"],"constraints":{"allowedTeamIDs":["team_support"]}}`,
		"--permissions", "queue:read,conversation:write",
		"--allow-delegated-routing",
		"--delegated-routing-teams", "team_support,team_billing",
		"--active-days", "1,2,5",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if got := payload["role"]; got != "operator" {
		t.Fatalf("unexpected role %#v", got)
	}
}

type graphQLPayload struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

func decodeGraphQLPayload(t *testing.T, body io.Reader) graphQLPayload {
	t.Helper()

	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}

	var payload graphQLPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return payload
}

func graphQLSuccess(data string) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":` + data + `}`)),
	}, nil
}
