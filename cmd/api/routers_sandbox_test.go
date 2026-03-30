package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

type sandboxMemoryStore struct {
	byID                 map[string]*platformdomain.Sandbox
	bySlug               map[string]string
	byVerificationHashes map[string]string
}

func newSandboxMemoryStore() *sandboxMemoryStore {
	return &sandboxMemoryStore{
		byID:                 map[string]*platformdomain.Sandbox{},
		bySlug:               map[string]string{},
		byVerificationHashes: map[string]string{},
	}
}

func (s *sandboxMemoryStore) CreateSandbox(_ context.Context, sandbox *platformdomain.Sandbox) error {
	if sandbox == nil {
		return errors.New("sandbox is required")
	}
	if sandbox.ID == "" {
		sandbox.ID = "sbx_router"
	}
	s.byID[sandbox.ID] = cloneSandbox(sandbox)
	s.bySlug[sandbox.Slug] = sandbox.ID
	s.byVerificationHashes[sandbox.VerificationTokenHash] = sandbox.ID
	return nil
}

func (s *sandboxMemoryStore) GetSandbox(_ context.Context, sandboxID string) (*platformdomain.Sandbox, error) {
	if sandbox, ok := s.byID[sandboxID]; ok {
		return cloneSandbox(sandbox), nil
	}
	return nil, nil
}

func (s *sandboxMemoryStore) GetSandboxBySlug(_ context.Context, slug string) (*platformdomain.Sandbox, error) {
	if sandboxID, ok := s.bySlug[slug]; ok {
		return s.GetSandbox(context.Background(), sandboxID)
	}
	return nil, nil
}

func (s *sandboxMemoryStore) GetSandboxByVerificationTokenHash(_ context.Context, tokenHash string) (*platformdomain.Sandbox, error) {
	if sandboxID, ok := s.byVerificationHashes[tokenHash]; ok {
		return s.GetSandbox(context.Background(), sandboxID)
	}
	return nil, nil
}

func (s *sandboxMemoryStore) ListReapableSandboxes(_ context.Context, now time.Time) ([]*platformdomain.Sandbox, error) {
	reapable := make([]*platformdomain.Sandbox, 0)
	for _, sandbox := range s.byID {
		if due, _ := sandbox.ExpiryDue(now); due {
			reapable = append(reapable, cloneSandbox(sandbox))
		}
	}
	return reapable, nil
}

func (s *sandboxMemoryStore) UpdateSandbox(_ context.Context, sandbox *platformdomain.Sandbox) error {
	if sandbox == nil {
		return errors.New("sandbox is required")
	}
	s.byID[sandbox.ID] = cloneSandbox(sandbox)
	s.bySlug[sandbox.Slug] = sandbox.ID
	s.byVerificationHashes[sandbox.VerificationTokenHash] = sandbox.ID
	return nil
}

func TestCreatePublicRouter_SandboxLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testutil.NewTestConfig(t)
	cfg.Server.BaseURL = "https://app.example.com"
	cfg.Server.AdminBaseURL = "https://admin.example.com"
	cfg.Server.APIBaseURL = "https://api.example.com"
	sandboxService := platformservices.NewSandboxService(
		newSandboxMemoryStore(),
		platformservices.SandboxServiceConfig{
			PublicBaseURL: "https://app.example.com",
			RuntimeDomain: "movebigrocks.io",
		},
		platformservices.WithSandboxProvisioner(platformservices.URLSandboxProvisioner{RuntimeDomain: "movebigrocks.io"}),
	)
	router := createPublicRouter(cfg, nil, nil, nil, sandboxService, nil, "test", "abc123", "2026-03-13")

	createReq := httptest.NewRequest(http.MethodPost, "/api/public/sandboxes", strings.NewReader(`{"email":"ops@example.com","name":"Sandbox Trial"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)

	require.Equal(t, http.StatusCreated, createW.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	sandboxID, _ := created["id"].(string)
	manageToken, _ := created["manage_token"].(string)
	status, _ := created["status"].(string)
	runtimeURL, _ := created["runtime_url"].(string)
	loginURL, _ := created["login_url"].(string)
	require.NotEmpty(t, sandboxID)
	require.NotEmpty(t, manageToken)
	require.Equal(t, "ready", status)
	require.Contains(t, runtimeURL, ".movebigrocks.io")
	require.Contains(t, loginURL, ".movebigrocks.io/login")

	showReq := httptest.NewRequest(http.MethodGet, "/api/public/sandboxes/"+sandboxID, nil)
	showReq.Header.Set("Authorization", "Bearer "+manageToken)
	showW := httptest.NewRecorder()
	router.ServeHTTP(showW, showReq)

	require.Equal(t, http.StatusOK, showW.Code)
	assert.Contains(t, showW.Body.String(), `"id":"`+sandboxID+`"`)
	assert.Contains(t, showW.Body.String(), `"status":"ready"`)

	exportReq := httptest.NewRequest(http.MethodGet, "/api/public/sandboxes/"+sandboxID+"/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+manageToken)
	exportW := httptest.NewRecorder()
	router.ServeHTTP(exportW, exportReq)

	require.Equal(t, http.StatusOK, exportW.Code)
	assert.Contains(t, exportW.Body.String(), `"export_version":"mbr-sandbox-export-v1"`)
	assert.Contains(t, exportW.Body.String(), `"bundle"`)
	assert.Contains(t, exportW.Body.String(), `"runtime_configuration"`)
	assert.Contains(t, exportW.Body.String(), `"public_bundle_catalog"`)
}

func TestCreatePublicRouter_ServesRuntimeBootstrapDocument(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testutil.NewTestConfig(t)
	cfg.Server.BaseURL = "https://app.example.com"
	cfg.Server.AdminBaseURL = "https://admin.example.com"
	cfg.Server.APIBaseURL = "https://api.example.com"

	sandboxService := platformservices.NewSandboxService(
		newSandboxMemoryStore(),
		platformservices.SandboxServiceConfig{
			PublicBaseURL: "https://app.example.com",
			RuntimeDomain: "movebigrocks.io",
			ActivationTTL: 24 * time.Hour,
			TrialTTL:      5 * 24 * time.Hour,
			ExtensionTTL:  30 * 24 * time.Hour,
		},
		platformservices.WithSandboxProvisioner(platformservices.URLSandboxProvisioner{RuntimeDomain: "movebigrocks.io"}),
	)

	router := createPublicRouter(cfg, nil, nil, nil, sandboxService, nil, "v0.test", "abc123", "2026-03-28")

	req := httptest.NewRequest(http.MethodGet, "/.well-known/mbr-instance.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	assert.Equal(t, "Move Big Rocks", payload["product"])
	assert.Equal(t, "v0.test", payload["version"])
	assert.Equal(t, "abc123", payload["git_commit"])

	runtimePayload, ok := payload["runtime"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://app.example.com", runtimePayload["base_url"])
	assert.Equal(t, "https://api.example.com/graphql", runtimePayload["graphql_url"])
	assert.Equal(t, "https://app.example.com/auth/cli/start", runtimePayload["cli_login_start_url"])

	cliPayload, ok := payload["cli"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "/.well-known/mbr-instance.json", cliPayload["runtime_bootstrap_path"])
	assert.Equal(t, "https://movebigrocks.com/install.sh", cliPayload["install_sh_url"])

	docsPayload, ok := payload["docs"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://movebigrocks.com/docs/cli", docsPayload["cli_guide_url"])

	sandboxPayload, ok := payload["sandbox_policy"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, sandboxPayload["available"])
	assert.Equal(t, float64(24), sandboxPayload["activation_window_hours"])
	assert.Equal(t, float64(5), sandboxPayload["default_trial_days"])
	assert.Equal(t, float64(30), sandboxPayload["extension_days"])
	assert.Equal(t, "https://app.example.com/api/public/sandboxes", sandboxPayload["create_url"])
}

func cloneSandbox(sandbox *platformdomain.Sandbox) *platformdomain.Sandbox {
	if sandbox == nil {
		return nil
	}
	clone := *sandbox
	if sandbox.VerifiedAt != nil {
		verifiedAt := *sandbox.VerifiedAt
		clone.VerifiedAt = &verifiedAt
	}
	if sandbox.ExpiresAt != nil {
		expiresAt := *sandbox.ExpiresAt
		clone.ExpiresAt = &expiresAt
	}
	if sandbox.ExpiredAt != nil {
		expiredAt := *sandbox.ExpiredAt
		clone.ExpiredAt = &expiredAt
	}
	if sandbox.ExtendedAt != nil {
		extendedAt := *sandbox.ExtendedAt
		clone.ExtendedAt = &extendedAt
	}
	if sandbox.DestroyedAt != nil {
		destroyedAt := *sandbox.DestroyedAt
		clone.DestroyedAt = &destroyedAt
	}
	return &clone
}
