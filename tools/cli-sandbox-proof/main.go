package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func main() {
	var (
		mbrBin    string
		outPath   string
		version   string
		gitSHA    string
		buildDate string
	)

	flag.StringVar(&mbrBin, "mbr-bin", "", "path to the built mbr binary")
	flag.StringVar(&outPath, "out", "", "path to write the CLI sandbox proof artifact")
	flag.StringVar(&version, "version", "", "runtime version string")
	flag.StringVar(&gitSHA, "git-sha", "", "runtime git commit")
	flag.StringVar(&buildDate, "build-date", "", "runtime build date")
	flag.Parse()

	if strings.TrimSpace(mbrBin) == "" {
		failf("--mbr-bin is required")
	}
	if strings.TrimSpace(outPath) == "" {
		failf("--out is required")
	}

	state := &sandboxProofState{
		Requests: make([]map[string]any, 0, 5),
	}
	server := httptest.NewServer(http.HandlerFunc(state.handle))
	defer server.Close()

	ctx := context.Background()
	tempRoot, err := os.MkdirTemp("", "mbr-cli-sandbox-proof-*")
	if err != nil {
		failf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempRoot)

	configPath := filepath.Join(tempRoot, "mbr-config.json")
	exportPath := filepath.Join(tempRoot, "sandbox-export.json")

	createOutput := runCLI(ctx, mbrBin, tempRoot, configPath, "sandboxes", "create", "--url", server.URL, "--email", "ops@example.com", "--name", "Milestone Proof Sandbox", "--json")
	var created map[string]any
	if err := json.Unmarshal(createOutput.Stdout, &created); err != nil {
		failf("decode create output: %v", err)
	}
	sandboxID, _ := created["id"].(string)
	manageToken, _ := created["manage_token"].(string)
	if sandboxID == "" || manageToken == "" {
		failf("create output missing sandbox id or manage token: %#v", created)
	}

	showOutput := runCLI(ctx, mbrBin, tempRoot, configPath, "sandboxes", "show", sandboxID, "--manage-token", manageToken, "--url", server.URL, "--json")
	var shown map[string]any
	if err := json.Unmarshal(showOutput.Stdout, &shown); err != nil {
		failf("decode show output: %v", err)
	}

	extendOutput := runCLI(ctx, mbrBin, tempRoot, configPath, "sandboxes", "extend", sandboxID, "--manage-token", manageToken, "--url", server.URL, "--json")
	var extended map[string]any
	if err := json.Unmarshal(extendOutput.Stdout, &extended); err != nil {
		failf("decode extend output: %v", err)
	}

	exportOutput := runCLI(ctx, mbrBin, tempRoot, configPath, "sandboxes", "export", sandboxID, "--manage-token", manageToken, "--url", server.URL, "--out", exportPath, "--json")
	var exported map[string]any
	if err := json.Unmarshal(exportOutput.Stdout, &exported); err != nil {
		failf("decode export output: %v", err)
	}
	exportFile, err := os.ReadFile(exportPath)
	if err != nil {
		failf("read exported file: %v", err)
	}
	var exportedFilePayload map[string]any
	if err := json.Unmarshal(exportFile, &exportedFilePayload); err != nil {
		failf("decode exported file: %v", err)
	}

	destroyOutput := runCLI(ctx, mbrBin, tempRoot, configPath, "sandboxes", "destroy", sandboxID, "--manage-token", manageToken, "--url", server.URL, "--reason", "proof complete", "--json")
	var destroyed map[string]any
	if err := json.Unmarshal(destroyOutput.Stdout, &destroyed); err != nil {
		failf("decode destroy output: %v", err)
	}

	artifact := map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"version":      version,
		"git_sha":      gitSHA,
		"build_date":   buildDate,
		"server_url":   server.URL,
		"requests":     state.snapshotRequests(),
		"create": map[string]any{
			"stdout": string(createOutput.Stdout),
			"result": created,
		},
		"show": map[string]any{
			"stdout": string(showOutput.Stdout),
			"result": shown,
		},
		"extend": map[string]any{
			"stdout": string(extendOutput.Stdout),
			"result": extended,
		},
		"export": map[string]any{
			"stdout":       string(exportOutput.Stdout),
			"result":       exported,
			"export_file":  exportPath,
			"file_payload": exportedFilePayload,
		},
		"destroy": map[string]any{
			"stdout": string(destroyOutput.Stdout),
			"result": destroyed,
		},
	}

	encoded, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		failf("marshal proof artifact: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		failf("create output directory: %v", err)
	}
	if err := os.WriteFile(outPath, append(encoded, '\n'), 0o644); err != nil {
		failf("write proof artifact: %v", err)
	}

	fmt.Printf("wrote %s\n", outPath)
}

type cliRunOutput struct {
	Stdout []byte
}

func runCLI(ctx context.Context, mbrBin, workdir, configPath string, args ...string) cliRunOutput {
	cmd := exec.CommandContext(ctx, mbrBin, args...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(),
		"MBR_CONFIG_PATH="+configPath,
		"MBR_DISABLE_KEYCHAIN=1",
	)
	stdout, err := cmd.Output()
	if err == nil {
		return cliRunOutput{Stdout: stdout}
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		failf("command %q failed: %s", strings.Join(append([]string{mbrBin}, args...), " "), strings.TrimSpace(string(exitErr.Stderr)))
	}
	failf("run command %q: %v", strings.Join(append([]string{mbrBin}, args...), " "), err)
	return cliRunOutput{}
}

type sandboxProofState struct {
	mu        sync.Mutex
	Requests  []map[string]any
	ExpiresAt string
}

func (s *sandboxProofState) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	s.mu.Lock()
	s.Requests = append(s.Requests, map[string]any{
		"method":        r.Method,
		"path":          r.URL.Path,
		"authorization": r.Header.Get("Authorization"),
		"body":          strings.TrimSpace(string(body)),
	})
	if s.ExpiresAt == "" {
		s.ExpiresAt = "2026-04-02T08:00:00Z"
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/public/sandboxes":
		_, _ = io.WriteString(w, `{
		  "id":"sbx_cli_proof",
		  "slug":"steady-lantern-25",
		  "name":"Milestone Proof Sandbox",
		  "requested_email":"ops@example.com",
		  "status":"ready",
		  "runtime_url":"https://steady-lantern-25.movebigrocks.io",
		  "login_url":"https://steady-lantern-25.movebigrocks.io/login",
		  "bootstrap_url":"https://steady-lantern-25.movebigrocks.io/.well-known/mbr-instance.json",
		  "verification_url":"http://`+r.Host+`/sandbox/verify?token=sbv_cli_proof",
		  "manage_token":"sbm_cli_proof",
		  "activation_deadline_at":"2026-03-29T08:00:00Z",
		  "expires_at":"2026-04-02T08:00:00Z",
		  "created_at":"2026-03-28T08:00:00Z",
		  "updated_at":"2026-03-28T08:00:00Z",
		  "next_steps":["mbr auth login --url https://steady-lantern-25.movebigrocks.io"]
		}`)
	case r.Method == http.MethodGet && r.URL.Path == "/api/public/sandboxes/sbx_cli_proof":
		_, _ = io.WriteString(w, `{
		  "id":"sbx_cli_proof",
		  "slug":"steady-lantern-25",
		  "name":"Milestone Proof Sandbox",
		  "requested_email":"ops@example.com",
		  "status":"ready",
		  "runtime_url":"https://steady-lantern-25.movebigrocks.io",
		  "login_url":"https://steady-lantern-25.movebigrocks.io/login",
		  "bootstrap_url":"https://steady-lantern-25.movebigrocks.io/.well-known/mbr-instance.json",
		  "activation_deadline_at":"2026-03-29T08:00:00Z",
		  "expires_at":"`+s.currentExpiry()+`",
		  "created_at":"2026-03-28T08:00:00Z",
		  "updated_at":"2026-03-28T08:00:00Z"
		}`)
	case r.Method == http.MethodPost && r.URL.Path == "/api/public/sandboxes/sbx_cli_proof/extend":
		s.setExpiry("2026-05-02T08:00:00Z")
		_, _ = io.WriteString(w, `{
		  "id":"sbx_cli_proof",
		  "slug":"steady-lantern-25",
		  "name":"Milestone Proof Sandbox",
		  "requested_email":"ops@example.com",
		  "status":"ready",
		  "runtime_url":"https://steady-lantern-25.movebigrocks.io",
		  "login_url":"https://steady-lantern-25.movebigrocks.io/login",
		  "bootstrap_url":"https://steady-lantern-25.movebigrocks.io/.well-known/mbr-instance.json",
		  "activation_deadline_at":"2026-03-29T08:00:00Z",
		  "expires_at":"`+s.currentExpiry()+`",
		  "extended_at":"2026-03-28T09:00:00Z",
		  "created_at":"2026-03-28T08:00:00Z",
		  "updated_at":"2026-03-28T09:00:00Z"
		}`)
	case r.Method == http.MethodGet && r.URL.Path == "/api/public/sandboxes/sbx_cli_proof/export":
		_, _ = io.WriteString(w, `{
		  "export_version":"mbr-sandbox-export-v1",
		  "generated_at":"2026-03-28T09:00:00Z",
		  "file_name":"mbr-sandbox-steady-lantern-25-export.json",
		  "content_type":"application/json",
		  "includes":["sandbox_metadata","runtime_configuration","cli_handoff","public_bundle_catalog"],
		  "bundle":{
		    "sandbox":{"id":"sbx_cli_proof","status":"ready"},
		    "runtime_configuration":{"status":"ready","default_trial_days":5},
		    "public_bundle_catalog":[{"slug":"ats","channel":"launch"}]
		  }
		}`)
	case r.Method == http.MethodDelete && r.URL.Path == "/api/public/sandboxes/sbx_cli_proof":
		_, _ = io.WriteString(w, `{
		  "id":"sbx_cli_proof",
		  "slug":"steady-lantern-25",
		  "name":"Milestone Proof Sandbox",
		  "requested_email":"ops@example.com",
		  "status":"destroyed",
		  "runtime_url":"https://steady-lantern-25.movebigrocks.io",
		  "login_url":"https://steady-lantern-25.movebigrocks.io/login",
		  "bootstrap_url":"https://steady-lantern-25.movebigrocks.io/.well-known/mbr-instance.json",
		  "activation_deadline_at":"2026-03-29T08:00:00Z",
		  "expires_at":"`+s.currentExpiry()+`",
		  "destroyed_at":"2026-03-28T10:00:00Z",
		  "created_at":"2026-03-28T08:00:00Z",
		  "updated_at":"2026-03-28T10:00:00Z"
		}`)
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"not found"}`)
	}
}

func (s *sandboxProofState) snapshotRequests() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := make([]map[string]any, 0, len(s.Requests))
	for _, request := range s.Requests {
		item := map[string]any{}
		for key, value := range request {
			item[key] = value
		}
		clone = append(clone, item)
	}
	return clone
}

func (s *sandboxProofState) currentExpiry() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ExpiresAt
}

func (s *sandboxProofState) setExpiry(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ExpiresAt = value
}

func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
