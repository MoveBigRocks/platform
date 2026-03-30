package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFleetRegisterUsesConfigAndWritesTrackingSecret(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "mbr.instance.yaml")
	config := `apiVersion: mbr.demandops.com/v1alpha1
kind: MBRInstance
metadata:
  name: acme-prod
  instanceID: inst_acme_prod
spec:
  deployment:
    release:
      core:
        version: v0.8.1
  auth:
    breakGlassAdminEmail: owner@example.com
  fleet:
    endpoint: https://movebigrocks.com
    registration:
      operatorEmail: ops@example.com
      useCase: internal_ops
      source: self_hosted
    heartbeat:
      enabled: true
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var gotRequest fleetRegisterInput
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.URL.Path != "/api/fleet/register" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"status":"registered","instance_id":"inst_acme_prod","instance_name":"acme-prod","lifecycle_status":"registered","secret_issued":true,"tracking_secret":"fleet_secret_123","message":"Store this tracking secret."}`)
	}))
	t.Cleanup(server.Close)

	previousClient := newHTTPClient
	newHTTPClient = func() *http.Client { return server.Client() }
	t.Cleanup(func() {
		newHTTPClient = previousClient
	})

	secretPath := filepath.Join(t.TempDir(), "fleet-secret.txt")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{
		"fleet", "register",
		"--config", configPath,
		"--fleet-url", server.URL,
		"--tracking-secret-out", secretPath,
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	if gotRequest.InstanceID != "inst_acme_prod" {
		t.Fatalf("unexpected instance ID %#v", gotRequest.InstanceID)
	}
	if gotRequest.InstanceName != "acme-prod" {
		t.Fatalf("unexpected instance name %#v", gotRequest.InstanceName)
	}
	if gotRequest.OperatorEmail != "ops@example.com" {
		t.Fatalf("unexpected operator email %#v", gotRequest.OperatorEmail)
	}
	if gotRequest.UseCase != fleetUseCaseInternalOps {
		t.Fatalf("unexpected use case %#v", gotRequest.UseCase)
	}
	if gotRequest.RegistrationSource != fleetRegistrationSourceSelfHosted {
		t.Fatalf("unexpected registration source %#v", gotRequest.RegistrationSource)
	}
	if gotRequest.PlatformVersion != "v0.8.1" {
		t.Fatalf("unexpected platform version %#v", gotRequest.PlatformVersion)
	}

	var payload fleetRegisterResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode json output: %v", err)
	}
	if payload.FleetURL != server.URL+"/api/fleet/register" {
		t.Fatalf("unexpected fleet URL %#v", payload.FleetURL)
	}
	if !payload.SecretIssued {
		t.Fatalf("expected secret to be issued")
	}

	secretData, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read tracking secret: %v", err)
	}
	if got := strings.TrimSpace(string(secretData)); got != "fleet_secret_123" {
		t.Fatalf("unexpected tracking secret contents %#v", got)
	}
}

func TestRunFleetRegisterAcceptsExistingTrackingSecretFile(t *testing.T) {
	t.Parallel()

	secretPath := filepath.Join(t.TempDir(), "tracking-secret.txt")
	if err := os.WriteFile(secretPath, []byte("existing_secret\n"), 0o600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}

	var gotRequest fleetRegisterInput
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"registered","instance_id":"inst_existing","instance_name":"existing","lifecycle_status":"registered","secret_issued":false,"message":"Instance registration updated."}`)
	}))
	t.Cleanup(server.Close)

	previousClient := newHTTPClient
	newHTTPClient = func() *http.Client { return server.Client() }
	t.Cleanup(func() {
		newHTTPClient = previousClient
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{
		"fleet", "register",
		"--fleet-url", server.URL,
		"--instance-id", "inst_existing",
		"--instance-name", "existing",
		"--operator-email", "owner@example.com",
		"--use-case", "startup",
		"--registration-source", "managed",
		"--platform-version", "v0.8.2",
		"--tracking-secret-file", secretPath,
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	if gotRequest.TrackingSecret != "existing_secret" {
		t.Fatalf("unexpected tracking secret %#v", gotRequest.TrackingSecret)
	}
	if gotRequest.UseCase != fleetUseCaseStartup {
		t.Fatalf("unexpected use case %#v", gotRequest.UseCase)
	}
	if gotRequest.RegistrationSource != fleetRegistrationSourceManaged {
		t.Fatalf("unexpected registration source %#v", gotRequest.RegistrationSource)
	}
}

func TestNormalizeFleetRegisterURLUsesAPIBaseURL(t *testing.T) {
	t.Parallel()

	got, err := normalizeFleetRegisterURL("https://movebigrocks.com")
	if err != nil {
		t.Fatalf("normalizeFleetRegisterURL returned error: %v", err)
	}
	if got != "https://api.movebigrocks.com/api/fleet/register" {
		t.Fatalf("unexpected normalized url %q", got)
	}
}

func TestLoadFleetRegisterConfigFallsBackToBreakGlassAdminEmail(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "mbr.instance.yaml")
	config := `metadata:
  name: demo
  instanceID: inst_demo
spec:
  deployment:
    release:
      core:
        version: v0.9.0
  auth:
    breakGlassAdminEmail: owner@example.com
  fleet:
    endpoint: https://movebigrocks.com
    registration:
      useCase: personal
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadFleetRegisterConfig(configPath)
	if err != nil {
		t.Fatalf("loadFleetRegisterConfig returned error: %v", err)
	}
	if cfg.OperatorEmail != "owner@example.com" {
		t.Fatalf("unexpected operator email %q", cfg.OperatorEmail)
	}
	if cfg.UseCase != fleetUseCasePersonal {
		t.Fatalf("unexpected use case %q", cfg.UseCase)
	}
	if cfg.RegistrationSource != fleetRegistrationSourceSelfHosted {
		t.Fatalf("unexpected registration source %q", cfg.RegistrationSource)
	}
}
