package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	"github.com/movebigrocks/platform/internal/testutil"
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
	sandboxService := platformservices.NewSandboxService(
		newSandboxMemoryStore(),
		platformservices.SandboxServiceConfig{
			PublicBaseURL: "https://movebigrocks.com",
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
