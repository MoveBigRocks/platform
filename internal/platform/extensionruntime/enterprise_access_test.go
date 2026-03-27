package extensionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	appcontainer "github.com/movebigrocks/platform/internal/infrastructure/container"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	sqlstore "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestEnterpriseAccessProviderUpsertSettingsAndHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	env := newEnterpriseAccessTestEnv(t)
	registry := NewRegistry(env.container)

	providerPayload := map[string]interface{}{
		"providerType":    "oidc",
		"displayName":     "Acme SSO",
		"discoveryUrl":    "https://login.example.com/.well-known/openid-configuration",
		"clientId":        "mbr-admin",
		"clientSecretRef": "env:ACME_OIDC_CLIENT_SECRET",
		"status":          "active",
		"claimMapping": map[string]string{
			"email":         "email",
			"name":          "name",
			"instance_role": "mbr_role",
		},
	}
	t.Setenv("ACME_OIDC_CLIENT_SECRET", "super-secret")

	body, err := json.Marshal(providerPayload)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/extensions/enterprise-access/providers", strings.NewReader(string(body)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	require.True(t, registry.Dispatch("enterprise-access.admin.providers.upsert", ctx))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"configuredCount":1`)

	stored, err := env.store.Extensions().GetInstanceExtensionBySlug(context.Background(), enterpriseAccessSlug)
	require.NoError(t, err)
	_, hasLegacyPayload := stored.Config.GetString(enterpriseAccessProvidersJSONKey)
	assert.False(t, hasLegacyPayload, "providers should be stored in the extension-owned schema, not the generic config payload")
	count, ok := stored.Config.GetInt(enterpriseAccessProviderCountKey)
	require.True(t, ok)
	assert.EqualValues(t, 1, count)

	settings := httptest.NewRecorder()
	settingsCtx, _ := gin.CreateTestContext(settings)
	settingsCtx.Request = httptest.NewRequest(http.MethodGet, "/extensions/enterprise-access", nil)
	require.True(t, registry.Dispatch("enterprise-access.admin.settings", settingsCtx))
	require.Equal(t, http.StatusOK, settings.Code)
	assert.Contains(t, settings.Body.String(), "Acme SSO")
	assert.Contains(t, settings.Body.String(), "Configured providers:</strong> 1")

	health := httptest.NewRecorder()
	healthCtx, _ := gin.CreateTestContext(health)
	healthCtx.Request = httptest.NewRequest(http.MethodGet, "/extensions/enterprise-access/health", nil)
	require.True(t, registry.Dispatch("enterprise-access.runtime.health", healthCtx))
	require.Equal(t, http.StatusOK, health.Code)
	assert.Contains(t, health.Body.String(), `"status":"healthy"`)
}

func TestEnterpriseAccessOIDCStartUsesDiscoveryDocument(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ACME_OIDC_CLIENT_SECRET", "super-secret")

	withEnterpriseAccessHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonHTTPResponse(http.StatusOK, map[string]string{
			"issuer":                 "https://login.example.com",
			"authorization_endpoint": "https://login.example.com/oauth2/authorize",
			"token_endpoint":         "https://login.example.com/oauth2/token",
			"userinfo_endpoint":      "https://login.example.com/oauth2/userinfo",
		}), nil
	})

	env := newEnterpriseAccessTestEnv(t)
	configureEnterpriseAccessProvider(t, env, enterpriseAccessProvider{
		ID:              "acme-sso",
		ProviderType:    enterpriseAccessProviderTypeOIDC,
		DisplayName:     "Acme SSO",
		DiscoveryURL:    "https://login.example.com/.well-known/openid-configuration",
		ClientID:        "mbr-admin",
		ClientSecretRef: "env:ACME_OIDC_CLIENT_SECRET",
		RedirectURL:     enterpriseAccessCallbackURL(env.container, nil),
		Status:          enterpriseAccessProviderStatusActive,
		ClaimMapping:    enterpriseAccessDefaultClaimMapping(),
	})

	registry := NewRegistry(env.container)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/extensions/enterprise-access/oidc/start?providerId=acme-sso&format=json", nil)
	ctx.Request.Header.Set("Accept", "application/json")

	require.True(t, registry.Dispatch("enterprise-access.auth.oidc.start", ctx))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ready"`)
	assert.Contains(t, w.Body.String(), `https://login.example.com/oauth2/authorize`)
	assert.Contains(t, w.Body.String(), `state=`)
}

func TestEnterpriseAccessOIDCCallbackCreatesSessionForProvisionedOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ACME_OIDC_CLIENT_SECRET", "super-secret")

	withEnterpriseAccessHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			return jsonHTTPResponse(http.StatusOK, map[string]string{
				"issuer":                 "https://idp.example",
				"authorization_endpoint": "https://idp.example/authorize",
				"token_endpoint":         "https://idp.example/token",
				"userinfo_endpoint":      "https://idp.example/userinfo",
			}), nil
		case "/token":
			return jsonHTTPResponse(http.StatusOK, map[string]interface{}{
				"access_token": "access-123",
				"token_type":   "Bearer",
			}), nil
		case "/userinfo":
			return jsonHTTPResponse(http.StatusOK, map[string]interface{}{
				"email":    "sso-user@example.com",
				"name":     "SSO User",
				"mbr_role": "operator",
			}), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, map[string]string{"error": "not found"}), nil
		}
	})

	env := newEnterpriseAccessTestEnv(t)
	configureEnterpriseAccessProvider(t, env, enterpriseAccessProvider{
		ID:              "acme-sso",
		ProviderType:    enterpriseAccessProviderTypeOIDC,
		DisplayName:     "Acme SSO",
		DiscoveryURL:    "https://idp.example/.well-known/openid-configuration",
		ClientID:        "mbr-admin",
		ClientSecretRef: "env:ACME_OIDC_CLIENT_SECRET",
		RedirectURL:     enterpriseAccessCallbackURL(env.container, nil),
		Status:          enterpriseAccessProviderStatusActive,
		ClaimMapping:    enterpriseAccessDefaultClaimMapping(),
	})
	enableJITProvisioning(t, env)

	registry := NewRegistry(env.container)

	startResp := httptest.NewRecorder()
	startCtx, _ := gin.CreateTestContext(startResp)
	startCtx.Request = httptest.NewRequest(http.MethodPost, "/extensions/enterprise-access/oidc/start?providerId=acme-sso&format=json", nil)
	startCtx.Request.Header.Set("Accept", "application/json")
	require.True(t, registry.Dispatch("enterprise-access.auth.oidc.start", startCtx))
	require.Equal(t, http.StatusOK, startResp.Code)

	var startPayload struct {
		AuthURL string `json:"authUrl"`
	}
	require.NoError(t, json.Unmarshal(startResp.Body.Bytes(), &startPayload))
	require.NotEmpty(t, startPayload.AuthURL)

	authURL, err := url.Parse(startPayload.AuthURL)
	require.NoError(t, err)
	state := authURL.Query().Get("state")
	require.NotEmpty(t, state)

	callbackResp := httptest.NewRecorder()
	callbackCtx, _ := gin.CreateTestContext(callbackResp)
	callbackCtx.Request = httptest.NewRequest(http.MethodGet, "/webhooks/extensions/enterprise-access/callback/oidc?code=code-123&state="+url.QueryEscape(state), nil)
	require.True(t, registry.Dispatch("enterprise-access.auth.oidc.callback", callbackCtx))
	require.Equal(t, http.StatusFound, callbackResp.Code)
	assert.Contains(t, callbackResp.Header().Get("Location"), "/extensions/enterprise-access")
	assert.Contains(t, callbackResp.Header().Get("Set-Cookie"), "mbr_session=")

	user, err := env.store.Users().GetUserByEmail(context.Background(), "sso-user@example.com")
	require.NoError(t, err)
	require.NotNil(t, user.InstanceRole)
	assert.Equal(t, platformdomain.InstanceRoleOperator, platformdomain.CanonicalizeInstanceRole(*user.InstanceRole))
}

func TestEnterpriseAccessOIDCCallbackDoesNotRedirectToExternalReturnTo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ACME_OIDC_CLIENT_SECRET", "super-secret")

	withEnterpriseAccessHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			return jsonHTTPResponse(http.StatusOK, map[string]string{
				"issuer":                 "https://idp.example",
				"authorization_endpoint": "https://idp.example/authorize",
				"token_endpoint":         "https://idp.example/token",
				"userinfo_endpoint":      "https://idp.example/userinfo",
			}), nil
		case "/token":
			return jsonHTTPResponse(http.StatusOK, map[string]interface{}{
				"access_token": "access-123",
				"token_type":   "Bearer",
			}), nil
		case "/userinfo":
			return jsonHTTPResponse(http.StatusOK, map[string]interface{}{
				"email":    "sso-user@example.com",
				"name":     "SSO User",
				"mbr_role": "operator",
			}), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, map[string]string{"error": "not found"}), nil
		}
	})

	env := newEnterpriseAccessTestEnv(t)
	configureEnterpriseAccessProvider(t, env, enterpriseAccessProvider{
		ID:              "acme-sso",
		ProviderType:    enterpriseAccessProviderTypeOIDC,
		DisplayName:     "Acme SSO",
		DiscoveryURL:    "https://idp.example/.well-known/openid-configuration",
		ClientID:        "mbr-admin",
		ClientSecretRef: "env:ACME_OIDC_CLIENT_SECRET",
		RedirectURL:     enterpriseAccessCallbackURL(env.container, nil),
		Status:          enterpriseAccessProviderStatusActive,
		ClaimMapping:    enterpriseAccessDefaultClaimMapping(),
	})
	enableJITProvisioning(t, env)

	registry := NewRegistry(env.container)

	startResp := httptest.NewRecorder()
	startCtx, _ := gin.CreateTestContext(startResp)
	startCtx.Request = httptest.NewRequest(http.MethodPost, "/extensions/enterprise-access/oidc/start?providerId=acme-sso&format=json&returnTo="+url.QueryEscape("https://evil.example/phish"), nil)
	startCtx.Request.Header.Set("Accept", "application/json")
	require.True(t, registry.Dispatch("enterprise-access.auth.oidc.start", startCtx))
	require.Equal(t, http.StatusOK, startResp.Code)

	var startPayload struct {
		AuthURL string `json:"authUrl"`
	}
	require.NoError(t, json.Unmarshal(startResp.Body.Bytes(), &startPayload))
	authURL, err := url.Parse(startPayload.AuthURL)
	require.NoError(t, err)

	callbackResp := httptest.NewRecorder()
	callbackCtx, _ := gin.CreateTestContext(callbackResp)
	callbackCtx.Request = httptest.NewRequest(http.MethodGet, "/webhooks/extensions/enterprise-access/callback/oidc?code=code-123&state="+url.QueryEscape(authURL.Query().Get("state")), nil)
	require.True(t, registry.Dispatch("enterprise-access.auth.oidc.callback", callbackCtx))
	require.Equal(t, http.StatusFound, callbackResp.Code)
	assert.Equal(t, "https://admin.mbr.test/extensions/enterprise-access", callbackResp.Header().Get("Location"))
}

func TestEnterpriseAccessProviderValidationRejectsNonHTTPSDiscoveryURL(t *testing.T) {
	provider := enterpriseAccessProvider{
		ProviderType: enterpriseAccessProviderTypeOIDC,
		DisplayName:  "Acme SSO",
		DiscoveryURL: "http://idp.example/.well-known/openid-configuration",
		ClientID:     "mbr-admin",
		RedirectURL:  "https://admin.mbr.test/webhooks/extensions/enterprise-access/callback/oidc",
		Status:       enterpriseAccessProviderStatusActive,
	}

	err := provider.Validate()
	require.Error(t, err)
	assert.Equal(t, "discoveryUrl must use https", err.Error())
}

func TestEnterpriseAccessProviderValidationRejectsLiteralSecretRef(t *testing.T) {
	provider := enterpriseAccessProvider{
		ProviderType:    enterpriseAccessProviderTypeOIDC,
		DisplayName:     "Acme SSO",
		DiscoveryURL:    "https://idp.example/.well-known/openid-configuration",
		ClientID:        "mbr-admin",
		ClientSecretRef: "super-secret-value",
		RedirectURL:     "https://admin.mbr.test/webhooks/extensions/enterprise-access/callback/oidc",
		Status:          enterpriseAccessProviderStatusActive,
	}

	err := provider.ValidateWithPolicy(&config.Config{})
	require.Error(t, err)
	assert.Equal(t, "clientSecretRef must use an explicit supported scheme", err.Error())
}

func TestEnterpriseAccessProviderValidationRejectsHostOutsideAllowlist(t *testing.T) {
	provider := enterpriseAccessProvider{
		ProviderType:    enterpriseAccessProviderTypeOIDC,
		DisplayName:     "Acme SSO",
		DiscoveryURL:    "https://idp.example/.well-known/openid-configuration",
		ClientID:        "mbr-admin",
		ClientSecretRef: "env:ACME_OIDC_CLIENT_SECRET",
		RedirectURL:     "https://admin.mbr.test/webhooks/extensions/enterprise-access/callback/oidc",
		Status:          enterpriseAccessProviderStatusActive,
	}

	cfg := &config.Config{}
	cfg.EnterpriseAccess.AllowedHosts = []string{"login.example.com"}
	cfg.EnterpriseAccess.AllowEnvSecretRefs = true

	err := provider.ValidateWithPolicy(cfg)
	require.Error(t, err)
	assert.Equal(t, "discoveryUrl host idp.example is not allowed by enterprise access policy", err.Error())
}

type enterpriseAccessRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn enterpriseAccessRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func withEnterpriseAccessHTTPClient(t *testing.T, fn enterpriseAccessRoundTripFunc) {
	t.Helper()
	previous := enterpriseAccessHTTPClient
	enterpriseAccessHTTPClient = func() *http.Client {
		return &http.Client{
			Timeout:   5 * time.Second,
			Transport: fn,
		}
	}
	t.Cleanup(func() {
		enterpriseAccessHTTPClient = previous
	})
}

func jsonHTTPResponse(status int, payload interface{}) *http.Response {
	body, _ := json.Marshal(payload)
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}

type enterpriseAccessTestEnv struct {
	store     stores.Store
	container *appcontainer.Container
	service   *platformservices.ExtensionService
}

func newEnterpriseAccessTestEnv(t *testing.T) enterpriseAccessTestEnv {
	t.Helper()

	store, cleanup := testutil.SetupTestStore(t)
	t.Cleanup(cleanup)

	sessionService := platformservices.NewSessionService(store.Users(), store.Workspaces())
	userService := platformservices.NewUserManagementService(store.Users(), store.Workspaces())
	options := make([]platformservices.ExtensionServiceOption, 0, 1)
	if concrete, ok := store.(*sqlstore.Store); ok {
		options = append(options, platformservices.WithExtensionSchemaRuntime(concrete.ExtensionSchemaMigrator()))
	}
	extensionService := platformservices.NewExtensionServiceWithOptions(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
		options...,
	)

	manifest, assets, migrations := loadEnterpriseAccessReferencePackage(t)

	installed, err := extensionService.InstallExtension(context.Background(), platformservices.InstallExtensionParams{
		InstalledByID: "user_operator",
		LicenseToken:  "lic_enterprise_access",
		Manifest:      manifest,
		Assets:        assets,
		Migrations:    migrations,
	})
	require.NoError(t, err)
	_, err = extensionService.ActivateExtension(context.Background(), installed.ID)
	require.NoError(t, err)

	cfg := &config.Config{}
	cfg.Server.Environment = "development"
	cfg.Server.AdminBaseURL = "https://admin.mbr.test"
	cfg.Server.APIBaseURL = "https://api.mbr.test"
	cfg.Auth.JWTSecret = "test-enterprise-access-state-secret"
	cfg.EnterpriseAccess.AllowEnvSecretRefs = true

	c := &appcontainer.Container{
		Config: cfg,
		Store:  store,
		Platform: &appcontainer.PlatformContainer{
			Session:   sessionService,
			User:      userService,
			Extension: extensionService,
		},
	}

	return enterpriseAccessTestEnv{
		store:     store,
		container: c,
		service:   extensionService,
	}
}

func loadEnterpriseAccessReferencePackage(t *testing.T) (platformdomain.ExtensionManifest, []platformservices.ExtensionAssetInput, []platformservices.ExtensionMigrationInput) {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	baseDir, err := testutil.ResolveWorkspaceSiblingDir(repoRoot, filepath.Join("packs", "enterprise-access"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Skip("enterprise-access reference pack checkout not available")
		}
		require.NoError(t, err)
	}
	manifestPath := filepath.Join(baseDir, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var manifest platformdomain.ExtensionManifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))

	assetsDir := filepath.Join(baseDir, "assets")
	var assets []platformservices.ExtensionAssetInput
	require.NoError(t, filepath.Walk(assetsDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(assetsDir, path)
		if err != nil {
			return err
		}
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		assets = append(assets, platformservices.ExtensionAssetInput{
			Path:        filepath.ToSlash(relPath),
			ContentType: contentType,
			Content:     content,
		})
		return nil
	}))

	migrationsDir := filepath.Join(baseDir, "migrations")
	entries, err := os.ReadDir(migrationsDir)
	require.NoError(t, err)
	migrations := make([]platformservices.ExtensionMigrationInput, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
		require.NoError(t, err)
		migrations = append(migrations, platformservices.ExtensionMigrationInput{
			Path:    entry.Name(),
			Content: content,
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Path < migrations[j].Path
	})

	return manifest, assets, migrations
}

func configureEnterpriseAccessProvider(t *testing.T, env enterpriseAccessTestEnv, provider enterpriseAccessProvider) {
	t.Helper()
	installed, err := env.service.GetInstalledExtension(context.Background(), mustGetEnterpriseAccessID(t, env))
	require.NoError(t, err)
	provider.Normalize()
	providers := []enterpriseAccessProvider{provider}
	_, err = enterpriseAccessSaveProviders(newTestGinContext(t), env.container, installed.ID, installed.Config, providers)
	require.NoError(t, err)
}

func mustGetEnterpriseAccessID(t *testing.T, env enterpriseAccessTestEnv) string {
	t.Helper()
	installed, err := env.container.Store.Extensions().GetInstanceExtensionBySlug(context.Background(), enterpriseAccessSlug)
	require.NoError(t, err)
	return installed.ID
}

func enableJITProvisioning(t *testing.T, env enterpriseAccessTestEnv) {
	t.Helper()
	extensionID := mustGetEnterpriseAccessID(t, env)
	installed, err := env.service.GetInstalledExtension(context.Background(), extensionID)
	require.NoError(t, err)
	configMap := installed.Config.ToMap()
	configMap["jitProvisioning"] = true
	_, err = env.service.UpdateExtensionConfig(context.Background(), extensionID, configMap)
	require.NoError(t, err)
}

func newTestGinContext(t *testing.T) *gin.Context {
	t.Helper()
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return ctx
}
