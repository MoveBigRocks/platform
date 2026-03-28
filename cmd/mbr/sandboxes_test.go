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

	"github.com/movebigrocks/platform/internal/cliapi"
)

func TestRunSandboxesCreateJSON(t *testing.T) {
	previousHTTPClient := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost {
					t.Fatalf("unexpected method %q", r.Method)
				}
				if r.URL.String() != "https://app.mbr.test/api/public/sandboxes" {
					t.Fatalf("unexpected sandbox create url %q", r.URL.String())
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				if !strings.Contains(string(body), `"email":"ops@example.com"`) {
					t.Fatalf("unexpected request body %q", string(body))
				}
				if !strings.Contains(string(body), `"name":"Sandbox Trial"`) {
					t.Fatalf("unexpected request body %q", string(body))
				}

				responseBody := `{
				  "id":"sbx_123",
				  "slug":"steady-lantern-25",
				  "name":"Sandbox Trial",
				  "requested_email":"ops@example.com",
				  "status":"ready",
				  "runtime_url":"https://steady-lantern-25.movebigrocks.io",
				  "login_url":"https://steady-lantern-25.movebigrocks.io/login",
				  "bootstrap_url":"https://steady-lantern-25.movebigrocks.io/.well-known/mbr-instance.json",
				  "verification_url":"https://app.mbr.test/sandbox/verify?token=sbv_123",
				  "manage_token":"sbm_123",
				  "activation_deadline_at":"2026-03-29T08:00:00Z",
				  "expires_at":"2026-04-02T08:00:00Z",
				  "created_at":"2026-03-28T08:00:00Z",
				  "updated_at":"2026-03-28T08:00:00Z",
				  "next_steps":["mbr auth login --url https://steady-lantern-25.movebigrocks.io"]
				}`
				return &http.Response{
					StatusCode: http.StatusCreated,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(responseBody)),
				}, nil
			}),
		}
	}
	t.Cleanup(func() {
		newHTTPClient = previousHTTPClient
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"sandboxes", "create",
		"--url", "https://api.app.mbr.test",
		"--email", "ops@example.com",
		"--name", "Sandbox Trial",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload sandboxCLIState
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "sbx_123" || payload.ManageToken != "sbm_123" {
		t.Fatalf("unexpected sandbox payload: %#v", payload)
	}
	if payload.VerificationURL == "" {
		t.Fatalf("expected verification url in payload: %#v", payload)
	}
}

func TestRunSandboxesLifecycleAndExportFile(t *testing.T) {
	previousHTTPClient := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if got := r.Header.Get("Authorization"); got != "Bearer sbm_123" {
					t.Fatalf("unexpected authorization header %q", got)
				}
				switch {
				case r.Method == http.MethodGet && r.URL.String() == "https://app.mbr.test/api/public/sandboxes/sbx_123":
					body := `{"id":"sbx_123","slug":"steady-lantern-25","status":"ready","runtime_url":"https://steady-lantern-25.movebigrocks.io","login_url":"https://steady-lantern-25.movebigrocks.io/login","bootstrap_url":"https://steady-lantern-25.movebigrocks.io/.well-known/mbr-instance.json","activation_deadline_at":"2026-03-29T08:00:00Z","expires_at":"2026-04-02T08:00:00Z","created_at":"2026-03-28T08:00:00Z","updated_at":"2026-03-28T08:00:00Z"}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
					}, nil
				case r.Method == http.MethodPost && r.URL.String() == "https://app.mbr.test/api/public/sandboxes/sbx_123/extend":
					body := `{"id":"sbx_123","slug":"steady-lantern-25","status":"ready","runtime_url":"https://steady-lantern-25.movebigrocks.io","login_url":"https://steady-lantern-25.movebigrocks.io/login","bootstrap_url":"https://steady-lantern-25.movebigrocks.io/.well-known/mbr-instance.json","activation_deadline_at":"2026-03-29T08:00:00Z","expires_at":"2026-05-02T08:00:00Z","extended_at":"2026-03-28T09:00:00Z","created_at":"2026-03-28T08:00:00Z","updated_at":"2026-03-28T09:00:00Z"}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
					}, nil
				case r.Method == http.MethodGet && r.URL.String() == "https://app.mbr.test/api/public/sandboxes/sbx_123/export":
					body := `{"export_version":"mbr-sandbox-export-v1","generated_at":"2026-03-28T09:00:00Z","file_name":"mbr-sandbox-steady-lantern-25-export.json","content_type":"application/json","includes":["sandbox_metadata","runtime_configuration"],"bundle":{"sandbox":{"id":"sbx_123"}}}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
					}, nil
				case r.Method == http.MethodDelete && r.URL.String() == "https://app.mbr.test/api/public/sandboxes/sbx_123":
					requestBody, err := io.ReadAll(r.Body)
					if err != nil {
						t.Fatalf("read destroy request body: %v", err)
					}
					if !strings.Contains(string(requestBody), `"reason":"proof complete"`) {
						t.Fatalf("unexpected destroy body %q", string(requestBody))
					}
					body := `{"id":"sbx_123","slug":"steady-lantern-25","status":"destroyed","runtime_url":"https://steady-lantern-25.movebigrocks.io","login_url":"https://steady-lantern-25.movebigrocks.io/login","bootstrap_url":"https://steady-lantern-25.movebigrocks.io/.well-known/mbr-instance.json","activation_deadline_at":"2026-03-29T08:00:00Z","expires_at":"2026-05-02T08:00:00Z","destroyed_at":"2026-03-28T10:00:00Z","created_at":"2026-03-28T08:00:00Z","updated_at":"2026-03-28T10:00:00Z"}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
					}, nil
				default:
					t.Fatalf("unexpected sandbox lifecycle request %s %s", r.Method, r.URL.String())
					return nil, nil
				}
			}),
		}
	}
	t.Cleanup(func() {
		newHTTPClient = previousHTTPClient
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	if _, err := cliapi.SaveStoredConfig("https://app.mbr.test", "hat_unused"); err != nil {
		t.Fatalf("SaveStoredConfig returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"sandboxes", "show", "sbx_123",
		"--manage-token", "sbm_123",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected show exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "status:\tready") {
		t.Fatalf("expected show output to include ready status, got %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{
		"sandboxes", "extend", "sbx_123",
		"--manage-token", "sbm_123",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected extend exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "extendedAt:\t2026-03-28T09:00:00Z") {
		t.Fatalf("expected extend output to include extendedAt, got %q", stdout.String())
	}

	exportPath := filepath.Join(t.TempDir(), "sandbox-export.json")
	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{
		"sandboxes", "export", "sbx_123",
		"--manage-token", "sbm_123",
		"--out", exportPath,
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected export exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "out:\t") {
		t.Fatalf("expected export output to include out path, got %q", stdout.String())
	}
	exported, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}
	if !strings.Contains(string(exported), `"export_version":"mbr-sandbox-export-v1"`) {
		t.Fatalf("unexpected export file content %q", string(exported))
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{
		"sandboxes", "destroy", "sbx_123",
		"--manage-token", "sbm_123",
		"--reason", "proof complete",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected destroy exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "status:\tdestroyed") {
		t.Fatalf("expected destroy output to include destroyed status, got %q", stdout.String())
	}
}

func TestRunSandboxesRequireManageToken(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"sandboxes", "show", "sbx_123",
		"--url", "https://app.mbr.test",
	}, stdout, stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--manage-token is required") {
		t.Fatalf("expected manage token guidance, got %q", stderr.String())
	}
}
