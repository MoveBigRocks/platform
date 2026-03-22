package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSandboxesCreateJSON(t *testing.T) {
	previous := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost {
					t.Fatalf("expected POST request, got %s", r.Method)
				}
				if r.URL.String() != "https://movebigrocks.com/api/public/sandboxes" {
					t.Fatalf("unexpected URL %s", r.URL.String())
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				assertJSONContains(t, body, "email", "ops@example.com")
				assertJSONContains(t, body, "name", "Agent Sandbox")
				response := `{
					"id":"sbx_123",
					"slug":"magic-dumpling-26",
					"status":"ready",
					"runtime_url":"https://magic-dumpling-26.movebigrocks.io",
					"login_url":"https://magic-dumpling-26.movebigrocks.io/login",
					"bootstrap_url":"https://magic-dumpling-26.movebigrocks.io/.well-known/mbr-instance.json",
					"expires_at":"2026-03-27T10:00:00Z",
					"manage_token":"sbm_token",
					"next_steps":[
						"mbr auth login --url https://magic-dumpling-26.movebigrocks.io",
						"mbr context set --workspace sandbox --team operations"
					]
				}`
				return &http.Response{
					StatusCode: http.StatusCreated,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(response)),
				}, nil
			}),
		}
	}
	defer func() {
		newHTTPClient = previous
	}()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"sandboxes", "create",
		"--email", "ops@example.com",
		"--name", "Agent Sandbox",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if got := payload["id"]; got != "sbx_123" {
		t.Fatalf("unexpected id %#v", got)
	}
	if got := payload["manage_token"]; got != "sbm_token" {
		t.Fatalf("unexpected manage token %#v", got)
	}
	if got := payload["runtime_url"]; got != "https://magic-dumpling-26.movebigrocks.io" {
		t.Fatalf("unexpected runtime URL %#v", got)
	}
}

func TestRunSandboxesShowRequiresManageToken(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{"sandboxes", "show", "sbx_123"}, stdout, stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "--manage-token is required") {
		t.Fatalf("expected manage-token error, got %q", stderr.String())
	}
}

func TestRunSandboxesExtendUsesBearerToken(t *testing.T) {
	previous := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost {
					t.Fatalf("expected POST request, got %s", r.Method)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer sbm_token" {
					t.Fatalf("unexpected authorization header %q", got)
				}
				response := `{"id":"sbx_123","slug":"magic-dumpling-26","status":"ready"}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(response)),
				}, nil
			}),
		}
	}
	defer func() {
		newHTTPClient = previous
	}()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"sandboxes", "extend", "sbx_123",
		"--manage-token", "sbm_token",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}
}

func TestRunSandboxesExportWritesFile(t *testing.T) {
	previous := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet {
					t.Fatalf("expected GET request, got %s", r.Method)
				}
				if !strings.HasSuffix(r.URL.Path, "/api/public/sandboxes/sbx_123/export") {
					t.Fatalf("unexpected URL path %s", r.URL.Path)
				}
				response := `{
					"export_version":"mbr-sandbox-export-v1",
					"generated_at":"2026-03-21T10:00:00Z",
					"file_name":"mbr-sandbox-magic-dumpling-26-export.json",
					"content_type":"application/json",
					"includes":["sandbox_metadata","runtime_urls"],
					"bundle":{"sandbox":{"id":"sbx_123"}}
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(response)),
				}, nil
			}),
		}
	}
	defer func() {
		newHTTPClient = previous
	}()

	outputPath := filepath.Join(t.TempDir(), "sandbox-export.json")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"sandboxes", "export", "sbx_123",
		"--manage-token", "sbm_token",
		"--out", outputPath,
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "exported:\t"+outputPath) {
		t.Fatalf("expected export path in output, got %q", stdout.String())
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}
	if !strings.Contains(string(content), `"export_version": "mbr-sandbox-export-v1"`) {
		t.Fatalf("expected export version in file, got %q", string(content))
	}
}

func assertJSONContains(t *testing.T, raw []byte, key, want string) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if got := payload[key]; got != want {
		t.Fatalf("unexpected %s %#v", key, got)
	}
}
