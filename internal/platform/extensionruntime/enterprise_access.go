package extensionruntime

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/container"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
)

var enterpriseAccessHTTPClient = func() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}

const (
	enterpriseAccessSlug                 = "enterprise-access"
	enterpriseAccessProvidersJSONKey     = "providers_json"
	enterpriseAccessProviderIDsKey       = "providers"
	enterpriseAccessProviderCountKey     = "provider_count"
	enterpriseAccessConfiguredAtKey      = "providers_configured_at"
	enterpriseAccessDefaultCallbackPath  = "/webhooks/extensions/enterprise-access/callback/oidc"
	enterpriseAccessDefaultReturnPath    = "/admin/extensions/enterprise-access"
	enterpriseAccessOIDCStateMaxAge      = 15 * time.Minute
	enterpriseAccessProviderTypeOIDC     = "oidc"
	enterpriseAccessProviderStatusDraft  = "draft"
	enterpriseAccessProviderStatusActive = "active"
)

type enterpriseAccessProvider struct {
	ID               string            `json:"id"`
	ProviderType     string            `json:"providerType"`
	DisplayName      string            `json:"displayName"`
	Issuer           string            `json:"issuer,omitempty"`
	DiscoveryURL     string            `json:"discoveryUrl,omitempty"`
	AuthorizationURL string            `json:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty"`
	UserInfoURL      string            `json:"userInfoUrl,omitempty"`
	JWKSURL          string            `json:"jwksUrl,omitempty"`
	ClientID         string            `json:"clientId,omitempty"`
	ClientSecretRef  string            `json:"clientSecretRef,omitempty"`
	RedirectURL      string            `json:"redirectUrl,omitempty"`
	Scopes           []string          `json:"scopes,omitempty"`
	ClaimMapping     map[string]string `json:"claimMapping,omitempty"`
	Status           string            `json:"status,omitempty"`
	EnforceSSO       bool              `json:"enforceSSO,omitempty"`
	CreatedAt        time.Time         `json:"createdAt,omitempty"`
	UpdatedAt        time.Time         `json:"updatedAt,omitempty"`
}

type enterpriseAccessProviderInput struct {
	ID               string            `json:"id"`
	ProviderType     string            `json:"providerType"`
	DisplayName      string            `json:"displayName"`
	Issuer           string            `json:"issuer"`
	DiscoveryURL     string            `json:"discoveryUrl"`
	AuthorizationURL string            `json:"authorizationUrl"`
	TokenURL         string            `json:"tokenUrl"`
	UserInfoURL      string            `json:"userInfoUrl"`
	JWKSURL          string            `json:"jwksUrl"`
	ClientID         string            `json:"clientId"`
	ClientSecretRef  string            `json:"clientSecretRef"`
	RedirectURL      string            `json:"redirectUrl"`
	Scopes           []string          `json:"scopes"`
	ClaimMapping     map[string]string `json:"claimMapping"`
	Status           string            `json:"status"`
	EnforceSSO       bool              `json:"enforceSSO"`
}

type enterpriseAccessStartInput struct {
	ProviderID string `json:"providerId"`
	ReturnTo   string `json:"returnTo"`
}

type enterpriseAccessOIDCDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type enterpriseAccessTokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	IDToken          string `json:"id_token"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type enterpriseAccessState struct {
	ProviderID string `json:"providerId"`
	ReturnTo   string `json:"returnTo,omitempty"`
	Nonce      string `json:"nonce"`
	IssuedAt   int64  `json:"issuedAt"`
}

func registerEnterpriseAccessTargets(registry *Registry, c *container.Container) {
	if registry == nil {
		return
	}

	registry.Register("enterprise-access.admin.settings", func(ctx *gin.Context) {
		extension, providers, err := enterpriseAccessLoadProviders(ctx, c)
		if err != nil {
			enterpriseAccessRenderError(ctx, http.StatusServiceUnavailable, "Enterprise Access", err.Error())
			return
		}

		status, message := enterpriseAccessHealthFromProviders(c, providers)
		page, renderErr := enterpriseAccessSettingsPage(settingsPageData{
			ExtensionID:   extension.ID,
			ProviderCount: len(providers),
			Providers:     providers,
			HealthStatus:  string(status),
			HealthMessage: message,
			CallbackURL:   enterpriseAccessCallbackURL(c, ctx),
		})
		if renderErr != nil {
			enterpriseAccessRenderError(ctx, http.StatusInternalServerError, "Enterprise Access", renderErr.Error())
			return
		}
		ctx.Data(http.StatusOK, "text/html; charset=utf-8", page)
	})

	registry.Register("enterprise-access.admin.providers.upsert", func(ctx *gin.Context) {
		extension, providers, err := enterpriseAccessLoadProviders(ctx, c)
		if err != nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "failed",
				"message": err.Error(),
			})
			return
		}

		input, err := enterpriseAccessParseProviderInput(ctx)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status":  "failed",
				"message": err.Error(),
			})
			return
		}

		provider, err := enterpriseAccessProviderFromInput(input, enterpriseAccessCallbackURL(c, ctx), c.Config)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status":  "failed",
				"message": err.Error(),
			})
			return
		}

		providers = enterpriseAccessUpsertProvider(providers, provider)
		updated, err := enterpriseAccessSaveProviders(ctx, c, extension.ID, extension.Config, providers)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "failed",
				"message": err.Error(),
			})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"status":          "ok",
			"provider":        provider,
			"configuredCount": len(providers),
			"healthStatus":    updated.HealthStatus,
			"healthMessage":   updated.HealthMessage,
		})
	})

	registry.Register("enterprise-access.auth.oidc.start", func(ctx *gin.Context) {
		_, providers, err := enterpriseAccessLoadProviders(ctx, c)
		if err != nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "failed",
				"message": err.Error(),
			})
			return
		}

		startInput := enterpriseAccessParseStartInput(ctx)
		provider, err := enterpriseAccessSelectProvider(providers, startInput.ProviderID)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status":  "failed",
				"message": err.Error(),
			})
			return
		}
		if err := provider.ValidateWithPolicy(c.Config); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status":  "failed",
				"message": err.Error(),
			})
			return
		}

		discovery, err := enterpriseAccessResolveOIDCDiscovery(ctx.Request.Context(), provider)
		if err != nil {
			ctx.JSON(http.StatusBadGateway, gin.H{
				"status":  "failed",
				"message": err.Error(),
			})
			return
		}
		if err := enterpriseAccessValidateDiscoveryPolicy(c.Config, discovery); err != nil {
			ctx.JSON(http.StatusBadGateway, gin.H{
				"status":  "failed",
				"message": err.Error(),
			})
			return
		}

		authURL, err := enterpriseAccessBuildAuthorizationURL(c, ctx, provider, discovery, startInput.ReturnTo)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status":  "failed",
				"message": err.Error(),
			})
			return
		}

		if enterpriseAccessWantsJSON(ctx) {
			ctx.JSON(http.StatusOK, gin.H{
				"status":   "ready",
				"provider": provider,
				"authUrl":  authURL,
			})
			return
		}
		ctx.Redirect(http.StatusFound, authURL)
	})

	registry.Register("enterprise-access.auth.oidc.callback", func(ctx *gin.Context) {
		if c == nil || c.Platform == nil || c.Platform.Session == nil {
			enterpriseAccessRenderError(ctx, http.StatusServiceUnavailable, "Enterprise Access", "session services are not configured")
			return
		}

		if upstreamError := strings.TrimSpace(ctx.Query("error")); upstreamError != "" {
			message := upstreamError
			if description := strings.TrimSpace(ctx.Query("error_description")); description != "" {
				message = message + ": " + description
			}
			enterpriseAccessRenderError(ctx, http.StatusUnauthorized, "Enterprise Access", message)
			return
		}

		code := strings.TrimSpace(ctx.Query("code"))
		stateToken := strings.TrimSpace(ctx.Query("state"))
		if code == "" || stateToken == "" {
			enterpriseAccessRenderError(ctx, http.StatusBadRequest, "Enterprise Access", "missing code or state")
			return
		}

		state, err := enterpriseAccessVerifyState(c, stateToken)
		if err != nil {
			enterpriseAccessRenderError(ctx, http.StatusUnauthorized, "Enterprise Access", err.Error())
			return
		}

		extension, providers, err := enterpriseAccessLoadProviders(ctx, c)
		if err != nil {
			enterpriseAccessRenderError(ctx, http.StatusServiceUnavailable, "Enterprise Access", err.Error())
			return
		}

		provider, err := enterpriseAccessFindProvider(providers, state.ProviderID)
		if err != nil {
			enterpriseAccessRenderError(ctx, http.StatusUnauthorized, "Enterprise Access", err.Error())
			return
		}
		if err := provider.ValidateWithPolicy(c.Config); err != nil {
			enterpriseAccessRenderError(ctx, http.StatusUnauthorized, "Enterprise Access", err.Error())
			return
		}

		discovery, err := enterpriseAccessResolveOIDCDiscovery(ctx.Request.Context(), provider)
		if err != nil {
			enterpriseAccessRenderError(ctx, http.StatusBadGateway, "Enterprise Access", err.Error())
			return
		}
		if err := enterpriseAccessValidateDiscoveryPolicy(c.Config, discovery); err != nil {
			enterpriseAccessRenderError(ctx, http.StatusUnauthorized, "Enterprise Access", err.Error())
			return
		}

		tokenResponse, err := enterpriseAccessExchangeAuthorizationCode(ctx.Request.Context(), c.Config, provider, discovery, code)
		if err != nil {
			enterpriseAccessRenderError(ctx, http.StatusUnauthorized, "Enterprise Access", err.Error())
			return
		}

		claims, err := enterpriseAccessFetchUserInfo(ctx.Request.Context(), provider, discovery, tokenResponse.AccessToken)
		if err != nil {
			enterpriseAccessRenderError(ctx, http.StatusBadGateway, "Enterprise Access", err.Error())
			return
		}

		user, provisioningMessage, err := enterpriseAccessResolveUser(ctx, c, extension, provider, claims)
		if err != nil {
			enterpriseAccessRenderError(ctx, http.StatusUnauthorized, "Enterprise Access", err.Error())
			return
		}

		session, sessionToken, err := c.Platform.Session.CreateSession(ctx.Request.Context(), user, ctx.ClientIP(), ctx.Request.UserAgent())
		if err != nil {
			enterpriseAccessRenderError(ctx, http.StatusUnauthorized, "Enterprise Access", err.Error())
			return
		}

		enterpriseAccessSetSessionCookie(ctx, c.Config, sessionToken, session)

		redirectURL := enterpriseAccessCallbackRedirectURL(c, state.ReturnTo)
		if enterpriseAccessWantsJSON(ctx) {
			ctx.JSON(http.StatusOK, gin.H{
				"status":      "ok",
				"userEmail":   user.Email,
				"redirectUrl": redirectURL,
				"message":     provisioningMessage,
			})
			return
		}
		ctx.Redirect(http.StatusFound, redirectURL)
	})

	registry.Register("enterprise-access.runtime.health", func(ctx *gin.Context) {
		_, providers, err := enterpriseAccessLoadProviders(ctx, c)
		if err != nil {
			ctx.JSON(http.StatusOK, gin.H{
				"status":  "failed",
				"message": err.Error(),
			})
			return
		}
		status, message := enterpriseAccessHealthFromProviders(c, providers)
		ctx.JSON(http.StatusOK, gin.H{
			"status":        status,
			"message":       message,
			"providerCount": len(providers),
		})
	})
}

type settingsPageData struct {
	ExtensionID   string
	ProviderCount int
	Providers     []enterpriseAccessProvider
	HealthStatus  string
	HealthMessage string
	CallbackURL   string
}

var enterpriseAccessSettingsTemplate = template.Must(template.New("enterprise-access-settings").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Enterprise Access</title>
  <style>
    body { font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 0; background: #f5f7fb; color: #1f2937; }
    main { max-width: 980px; margin: 40px auto; padding: 0 24px 48px; }
    .card { background: #fff; border: 1px solid #dbe3f0; border-radius: 16px; padding: 24px; box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06); margin-bottom: 20px; }
    h1, h2 { margin-top: 0; }
    code, pre { background: #eef2ff; border-radius: 8px; }
    code { padding: 0.15rem 0.35rem; }
    pre { padding: 16px; overflow: auto; }
    table { width: 100%; border-collapse: collapse; }
    th, td { text-align: left; padding: 10px 12px; border-top: 1px solid #e5e7eb; vertical-align: top; }
    th { font-size: 0.85rem; text-transform: uppercase; color: #6b7280; }
    .healthy { color: #0f766e; }
    .degraded { color: #b45309; }
    .failed { color: #b91c1c; }
  </style>
</head>
<body>
  <main>
    <section class="card">
      <h1>Enterprise Access</h1>
      <p>Configure instance-wide enterprise SSO providers here. This extension stays in charge of identity federation, while core still owns users, sessions, permissions, and break-glass access.</p>
      <p><strong>Runtime status:</strong> <span class="{{.HealthStatus}}">{{.HealthStatus}}</span> &middot; {{.HealthMessage}}</p>
      <p><strong>Configured providers:</strong> {{.ProviderCount}}</p>
      <p><strong>OIDC callback URL:</strong> <code>{{.CallbackURL}}</code></p>
    </section>
    <section class="card">
      <h2>Providers</h2>
      {{if .Providers}}
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Status</th>
            <th>Issuer / Discovery</th>
            <th>Client</th>
          </tr>
        </thead>
        <tbody>
          {{range .Providers}}
          <tr>
            <td><strong>{{.DisplayName}}</strong><br><small><code>{{.ID}}</code></small></td>
            <td>{{.ProviderType}}</td>
            <td>{{.Status}}</td>
            <td>{{if .DiscoveryURL}}{{.DiscoveryURL}}{{else}}{{.Issuer}}{{end}}</td>
            <td>{{.ClientID}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
      {{else}}
      <p>No providers configured yet.</p>
      {{end}}
    </section>
    <section class="card">
      <h2>Agent-Friendly API</h2>
      <p>Agents can configure a provider by posting JSON to <code>/admin/extensions/enterprise-access/providers</code>.</p>
      <pre>{
  "providerType": "oidc",
  "displayName": "Acme SSO",
  "discoveryUrl": "https://login.acme.example/.well-known/openid-configuration",
  "clientId": "mbr-production",
  "clientSecretRef": "env:ACME_OIDC_CLIENT_SECRET",
  "status": "active",
  "redirectUrl": "{{.CallbackURL}}",
  "claimMapping": {
    "email": "email",
    "name": "name",
    "instance_role": "mbr_role"
  }
}</pre>
    </section>
  </main>
</body>
</html>`))

func enterpriseAccessSettingsPage(data settingsPageData) ([]byte, error) {
	var builder strings.Builder
	if err := enterpriseAccessSettingsTemplate.Execute(&builder, data); err != nil {
		return nil, err
	}
	return []byte(builder.String()), nil
}

func enterpriseAccessLoadProviders(ctx *gin.Context, c *container.Container) (*platformdomain.InstalledExtension, []enterpriseAccessProvider, error) {
	extension, err := enterpriseAccessLoadInstalledExtension(ctx.Request.Context(), c)
	if err != nil {
		return nil, nil, err
	}
	if c != nil && c.Store != nil {
		if providerStore, storeErr := newEnterpriseAccessProviderStore(c.Store); storeErr == nil {
			providers, listErr := providerStore.listProviders(ctx.Request.Context(), extension.ID)
			if listErr == nil {
				if len(providers) == 0 {
					configProviders, configErr := enterpriseAccessProvidersFromConfig(extension.Config)
					if configErr == nil && len(configProviders) > 0 {
						if saveErr := providerStore.replaceProviders(ctx.Request.Context(), extension.ID, configProviders); saveErr == nil {
							providers = configProviders
						}
					}
				}
				return extension, providers, nil
			}
		}
	}
	providers, err := enterpriseAccessProvidersFromConfig(extension.Config)
	if err != nil {
		return nil, nil, err
	}
	return extension, providers, nil
}

func enterpriseAccessLoadInstalledExtension(ctx context.Context, c *container.Container) (*platformdomain.InstalledExtension, error) {
	if c == nil || c.Store == nil {
		return nil, fmt.Errorf("enterprise-access runtime dependencies are not configured")
	}
	extension, err := c.Store.Extensions().GetInstanceExtensionBySlug(ctx, enterpriseAccessSlug)
	if err != nil || extension == nil {
		return nil, fmt.Errorf("enterprise-access is not installed on this instance")
	}
	if extension.Status != platformdomain.ExtensionStatusActive {
		return nil, fmt.Errorf("enterprise-access is installed but not active")
	}
	return extension, nil
}

func enterpriseAccessProvidersFromConfig(config shareddomain.TypedCustomFields) ([]enterpriseAccessProvider, error) {
	raw, ok := config.GetString(enterpriseAccessProvidersJSONKey)
	if !ok || strings.TrimSpace(raw) == "" {
		return []enterpriseAccessProvider{}, nil
	}

	var providers []enterpriseAccessProvider
	if err := json.Unmarshal([]byte(raw), &providers); err != nil {
		return nil, fmt.Errorf("decode configured identity providers: %w", err)
	}
	for i := range providers {
		providers[i].Normalize()
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].DisplayName < providers[j].DisplayName
	})
	return providers, nil
}

func enterpriseAccessSaveProviders(
	ctx *gin.Context,
	c *container.Container,
	extensionID string,
	current shareddomain.TypedCustomFields,
	providers []enterpriseAccessProvider,
) (*platformdomain.InstalledExtension, error) {
	if c == nil || c.Platform == nil || c.Platform.Extension == nil {
		return nil, fmt.Errorf("extension service is not configured")
	}
	now := time.Now().UTC()
	for i := range providers {
		if providers[i].CreatedAt.IsZero() {
			providers[i].CreatedAt = now
		}
		providers[i].UpdatedAt = now
		providers[i].Normalize()
	}

	sort.Slice(providers, func(i, j int) bool {
		return providers[i].DisplayName < providers[j].DisplayName
	})
	if c.Store != nil {
		providerStore, err := newEnterpriseAccessProviderStore(c.Store)
		if err == nil {
			if err := providerStore.replaceProviders(ctx.Request.Context(), extensionID, providers); err != nil {
				return nil, fmt.Errorf("persist identity providers: %w", err)
			}
		}
	}
	configMap := current.ToMap()
	providerIDs := make([]string, 0, len(providers))
	for _, provider := range providers {
		providerIDs = append(providerIDs, provider.ID)
	}
	delete(configMap, enterpriseAccessProvidersJSONKey)
	configMap[enterpriseAccessProviderIDsKey] = providerIDs
	configMap[enterpriseAccessProviderCountKey] = len(providers)
	configMap[enterpriseAccessConfiguredAtKey] = now.Format(time.RFC3339)

	updated, err := c.Platform.Extension.UpdateExtensionConfig(ctx.Request.Context(), extensionID, configMap)
	if err != nil {
		return nil, err
	}
	if refreshed, healthErr := c.Platform.Extension.CheckExtensionHealth(ctx.Request.Context(), extensionID); healthErr == nil && refreshed != nil {
		updated = refreshed
	}
	return updated, nil
}

func (p *enterpriseAccessProvider) Normalize() {
	if p == nil {
		return
	}
	p.ID = strings.TrimSpace(p.ID)
	p.ProviderType = strings.ToLower(strings.TrimSpace(p.ProviderType))
	p.DisplayName = strings.TrimSpace(p.DisplayName)
	p.Issuer = strings.TrimSpace(p.Issuer)
	p.DiscoveryURL = strings.TrimSpace(p.DiscoveryURL)
	p.AuthorizationURL = strings.TrimSpace(p.AuthorizationURL)
	p.TokenURL = strings.TrimSpace(p.TokenURL)
	p.UserInfoURL = strings.TrimSpace(p.UserInfoURL)
	p.JWKSURL = strings.TrimSpace(p.JWKSURL)
	p.ClientID = strings.TrimSpace(p.ClientID)
	p.ClientSecretRef = strings.TrimSpace(p.ClientSecretRef)
	p.RedirectURL = strings.TrimSpace(p.RedirectURL)
	p.Status = strings.ToLower(strings.TrimSpace(p.Status))
	if p.ProviderType == "" {
		p.ProviderType = enterpriseAccessProviderTypeOIDC
	}
	if p.Status == "" {
		p.Status = enterpriseAccessProviderStatusDraft
	}
	if len(p.Scopes) == 0 {
		p.Scopes = []string{"openid", "profile", "email"}
	}
	normalizedScopes := make([]string, 0, len(p.Scopes))
	for _, scope := range p.Scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" && !slicesContains(normalizedScopes, scope) {
			normalizedScopes = append(normalizedScopes, scope)
		}
	}
	if len(normalizedScopes) == 0 {
		normalizedScopes = []string{"openid", "profile", "email"}
	}
	p.Scopes = normalizedScopes
	if p.ClaimMapping == nil {
		p.ClaimMapping = enterpriseAccessDefaultClaimMapping()
	}
	if strings.TrimSpace(p.ClaimMapping["email"]) == "" {
		p.ClaimMapping["email"] = "email"
	}
	if strings.TrimSpace(p.ClaimMapping["name"]) == "" {
		p.ClaimMapping["name"] = "name"
	}
	if strings.TrimSpace(p.ClaimMapping["instance_role"]) == "" {
		p.ClaimMapping["instance_role"] = "mbr_role"
	}
}

func (p enterpriseAccessProvider) Validate() error {
	if p.ProviderType != enterpriseAccessProviderTypeOIDC {
		return fmt.Errorf("only oidc providers are supported in milestone 1")
	}
	if p.DisplayName == "" {
		return fmt.Errorf("displayName is required")
	}
	if p.ClientID == "" {
		return fmt.Errorf("clientId is required")
	}
	if p.RedirectURL == "" {
		return fmt.Errorf("redirectUrl is required")
	}
	if p.DiscoveryURL == "" && p.AuthorizationURL == "" {
		return fmt.Errorf("discoveryUrl or authorizationUrl is required")
	}
	if err := enterpriseAccessValidateProviderURL("issuer", p.Issuer, false); err != nil {
		return err
	}
	if err := enterpriseAccessValidateProviderURL("discoveryUrl", p.DiscoveryURL, false); err != nil {
		return err
	}
	if err := enterpriseAccessValidateProviderURL("authorizationUrl", p.AuthorizationURL, false); err != nil {
		return err
	}
	if err := enterpriseAccessValidateProviderURL("tokenUrl", p.TokenURL, false); err != nil {
		return err
	}
	if err := enterpriseAccessValidateProviderURL("userInfoUrl", p.UserInfoURL, false); err != nil {
		return err
	}
	if err := enterpriseAccessValidateProviderURL("jwksUrl", p.JWKSURL, false); err != nil {
		return err
	}
	if err := enterpriseAccessValidateProviderURL("redirectUrl", p.RedirectURL, true); err != nil {
		return err
	}
	switch p.Status {
	case enterpriseAccessProviderStatusDraft, enterpriseAccessProviderStatusActive, "disabled":
	default:
		return fmt.Errorf("status must be draft, active, or disabled")
	}
	return nil
}

func (p enterpriseAccessProvider) ValidateWithPolicy(cfg *config.Config) error {
	if err := p.Validate(); err != nil {
		return err
	}
	return enterpriseAccessValidateProviderPolicy(cfg, p)
}

func enterpriseAccessValidateProviderURL(field, value string, allowLocalHTTP bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL", field)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", field)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if allowLocalHTTP && parsed.Scheme == "http" && enterpriseAccessIsLocalHost(parsed.Hostname()) {
		return nil
	}
	return fmt.Errorf("%s must use https", field)
}

func enterpriseAccessIsLocalHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	switch host {
	case "localhost", "127.0.0.1", "::1", "lvh.me":
		return true
	default:
		return false
	}
}

func enterpriseAccessProviderFromInput(input enterpriseAccessProviderInput, defaultRedirectURL string, cfg *config.Config) (enterpriseAccessProvider, error) {
	provider := enterpriseAccessProvider{
		ID:               input.ID,
		ProviderType:     input.ProviderType,
		DisplayName:      input.DisplayName,
		Issuer:           input.Issuer,
		DiscoveryURL:     input.DiscoveryURL,
		AuthorizationURL: input.AuthorizationURL,
		TokenURL:         input.TokenURL,
		UserInfoURL:      input.UserInfoURL,
		JWKSURL:          input.JWKSURL,
		ClientID:         input.ClientID,
		ClientSecretRef:  input.ClientSecretRef,
		RedirectURL:      input.RedirectURL,
		Scopes:           append([]string(nil), input.Scopes...),
		ClaimMapping:     cloneStringMap(input.ClaimMapping),
		Status:           input.Status,
		EnforceSSO:       input.EnforceSSO,
	}
	provider.Normalize()
	if provider.RedirectURL == "" {
		provider.RedirectURL = strings.TrimSpace(defaultRedirectURL)
	}
	if provider.ID == "" {
		provider.ID = enterpriseAccessProviderID(provider.DisplayName)
	}
	if err := provider.ValidateWithPolicy(cfg); err != nil {
		return enterpriseAccessProvider{}, err
	}
	return provider, nil
}

func enterpriseAccessParseProviderInput(ctx *gin.Context) (enterpriseAccessProviderInput, error) {
	var input enterpriseAccessProviderInput
	contentType := strings.ToLower(strings.TrimSpace(ctx.GetHeader("Content-Type")))
	if strings.Contains(contentType, "application/json") {
		if err := ctx.ShouldBindJSON(&input); err != nil {
			return enterpriseAccessProviderInput{}, fmt.Errorf("invalid provider payload")
		}
		return input, nil
	}

	if err := ctx.Request.ParseForm(); err != nil {
		return enterpriseAccessProviderInput{}, fmt.Errorf("invalid provider payload")
	}
	input.ID = ctx.PostForm("id")
	input.ProviderType = ctx.PostForm("providerType")
	input.DisplayName = ctx.PostForm("displayName")
	input.Issuer = ctx.PostForm("issuer")
	input.DiscoveryURL = ctx.PostForm("discoveryUrl")
	input.AuthorizationURL = ctx.PostForm("authorizationUrl")
	input.TokenURL = ctx.PostForm("tokenUrl")
	input.UserInfoURL = ctx.PostForm("userInfoUrl")
	input.JWKSURL = ctx.PostForm("jwksUrl")
	input.ClientID = ctx.PostForm("clientId")
	input.ClientSecretRef = ctx.PostForm("clientSecretRef")
	input.RedirectURL = ctx.PostForm("redirectUrl")
	input.Status = ctx.PostForm("status")
	input.EnforceSSO = strings.EqualFold(ctx.PostForm("enforceSSO"), "true")
	if scopesRaw := strings.TrimSpace(ctx.PostForm("scopes")); scopesRaw != "" {
		for _, part := range strings.Split(scopesRaw, ",") {
			if scope := strings.TrimSpace(part); scope != "" {
				input.Scopes = append(input.Scopes, scope)
			}
		}
	}
	if claimMappingRaw := strings.TrimSpace(ctx.PostForm("claimMapping")); claimMappingRaw != "" {
		if err := json.Unmarshal([]byte(claimMappingRaw), &input.ClaimMapping); err != nil {
			return enterpriseAccessProviderInput{}, fmt.Errorf("claimMapping must be valid JSON")
		}
	}
	return input, nil
}

func enterpriseAccessUpsertProvider(providers []enterpriseAccessProvider, provider enterpriseAccessProvider) []enterpriseAccessProvider {
	for i := range providers {
		if providers[i].ID == provider.ID || strings.EqualFold(providers[i].DisplayName, provider.DisplayName) {
			provider.CreatedAt = providers[i].CreatedAt
			providers[i] = provider
			return providers
		}
	}
	providers = append(providers, provider)
	return providers
}

func enterpriseAccessParseStartInput(ctx *gin.Context) enterpriseAccessStartInput {
	input := enterpriseAccessStartInput{
		ProviderID: strings.TrimSpace(ctx.Query("providerId")),
		ReturnTo:   strings.TrimSpace(ctx.Query("returnTo")),
	}
	if input.ProviderID != "" || input.ReturnTo != "" {
		return input
	}

	contentType := strings.ToLower(strings.TrimSpace(ctx.GetHeader("Content-Type")))
	if strings.Contains(contentType, "application/json") {
		var decoded enterpriseAccessStartInput
		if err := ctx.ShouldBindJSON(&decoded); err == nil {
			decoded.ProviderID = strings.TrimSpace(decoded.ProviderID)
			decoded.ReturnTo = strings.TrimSpace(decoded.ReturnTo)
			return decoded
		}
	}
	if err := ctx.Request.ParseForm(); err == nil {
		input.ProviderID = strings.TrimSpace(ctx.PostForm("providerId"))
		input.ReturnTo = strings.TrimSpace(ctx.PostForm("returnTo"))
	}
	return input
}

func enterpriseAccessSelectProvider(providers []enterpriseAccessProvider, requestedID string) (enterpriseAccessProvider, error) {
	if requestedID != "" {
		return enterpriseAccessFindProvider(providers, requestedID)
	}
	for _, provider := range providers {
		if provider.Status == enterpriseAccessProviderStatusActive {
			return provider, nil
		}
	}
	for _, provider := range providers {
		if provider.Status == enterpriseAccessProviderStatusDraft {
			return provider, nil
		}
	}
	return enterpriseAccessProvider{}, fmt.Errorf("no usable OIDC provider is configured")
}

func enterpriseAccessFindProvider(providers []enterpriseAccessProvider, id string) (enterpriseAccessProvider, error) {
	id = strings.TrimSpace(id)
	for _, provider := range providers {
		if provider.ID == id {
			return provider, nil
		}
	}
	return enterpriseAccessProvider{}, fmt.Errorf("identity provider %s is not configured", id)
}

func enterpriseAccessResolveOIDCDiscovery(ctx context.Context, provider enterpriseAccessProvider) (*enterpriseAccessOIDCDiscovery, error) {
	if strings.TrimSpace(provider.DiscoveryURL) == "" {
		return &enterpriseAccessOIDCDiscovery{
			Issuer:                provider.Issuer,
			AuthorizationEndpoint: provider.AuthorizationURL,
			TokenEndpoint:         provider.TokenURL,
			UserInfoEndpoint:      provider.UserInfoURL,
			JWKSURI:               provider.JWKSURL,
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.DiscoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build OIDC discovery request: %w", err)
	}
	resp, err := enterpriseAccessHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch OIDC discovery document: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch OIDC discovery document: provider returned %s", resp.Status)
	}

	var discovery enterpriseAccessOIDCDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return nil, fmt.Errorf("decode OIDC discovery document: %w", err)
	}
	if discovery.AuthorizationEndpoint == "" {
		discovery.AuthorizationEndpoint = provider.AuthorizationURL
	}
	if discovery.TokenEndpoint == "" {
		discovery.TokenEndpoint = provider.TokenURL
	}
	if discovery.UserInfoEndpoint == "" {
		discovery.UserInfoEndpoint = provider.UserInfoURL
	}
	if discovery.JWKSURI == "" {
		discovery.JWKSURI = provider.JWKSURL
	}
	if discovery.Issuer == "" {
		discovery.Issuer = provider.Issuer
	}
	return &discovery, nil
}

func enterpriseAccessBuildAuthorizationURL(
	c *container.Container,
	ctx *gin.Context,
	provider enterpriseAccessProvider,
	discovery *enterpriseAccessOIDCDiscovery,
	returnTo string,
) (string, error) {
	authEndpoint := strings.TrimSpace(provider.AuthorizationURL)
	if authEndpoint == "" && discovery != nil {
		authEndpoint = strings.TrimSpace(discovery.AuthorizationEndpoint)
	}
	if authEndpoint == "" {
		return "", fmt.Errorf("provider %s is missing an authorization endpoint", provider.DisplayName)
	}

	state, err := enterpriseAccessIssueState(c, enterpriseAccessState{
		ProviderID: provider.ID,
		ReturnTo:   sanitizeReturnPath(returnTo),
		Nonce:      enterpriseAccessRandomToken(),
		IssuedAt:   time.Now().UTC().Unix(),
	})
	if err != nil {
		return "", err
	}

	redirectURL := strings.TrimSpace(provider.RedirectURL)
	if redirectURL == "" {
		redirectURL = enterpriseAccessCallbackURL(c, ctx)
	}

	values := url.Values{}
	values.Set("response_type", "code")
	values.Set("client_id", provider.ClientID)
	values.Set("redirect_uri", redirectURL)
	values.Set("scope", strings.Join(provider.Scopes, " "))
	values.Set("state", state)
	values.Set("nonce", enterpriseAccessRandomToken())

	u, err := url.Parse(authEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid authorization endpoint: %w", err)
	}
	query := u.Query()
	for key, valuesForKey := range values {
		for _, value := range valuesForKey {
			query.Set(key, value)
		}
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func enterpriseAccessExchangeAuthorizationCode(
	ctx context.Context,
	cfg *config.Config,
	provider enterpriseAccessProvider,
	discovery *enterpriseAccessOIDCDiscovery,
	code string,
) (*enterpriseAccessTokenResponse, error) {
	tokenURL := strings.TrimSpace(provider.TokenURL)
	if tokenURL == "" && discovery != nil {
		tokenURL = strings.TrimSpace(discovery.TokenEndpoint)
	}
	if tokenURL == "" {
		return nil, fmt.Errorf("provider %s is missing a token endpoint", provider.DisplayName)
	}

	clientSecret, err := enterpriseAccessResolveSecret(cfg, provider.ClientSecretRef)
	if err != nil {
		return nil, err
	}

	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", strings.TrimSpace(code))
	values.Set("redirect_uri", provider.RedirectURL)
	values.Set("client_id", provider.ClientID)
	if clientSecret != "" {
		values.Set("client_secret", clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := enterpriseAccessHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	defer resp.Body.Close()

	var tokenResponse enterpriseAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(tokenResponse.ErrorDescription)
		if message == "" {
			message = strings.TrimSpace(tokenResponse.Error)
		}
		if message == "" {
			message = resp.Status
		}
		return nil, fmt.Errorf("exchange authorization code: %s", message)
	}
	if strings.TrimSpace(tokenResponse.AccessToken) == "" {
		return nil, fmt.Errorf("exchange authorization code: provider returned no access_token")
	}
	return &tokenResponse, nil
}

func enterpriseAccessFetchUserInfo(
	ctx context.Context,
	provider enterpriseAccessProvider,
	discovery *enterpriseAccessOIDCDiscovery,
	accessToken string,
) (map[string]interface{}, error) {
	userInfoURL := strings.TrimSpace(provider.UserInfoURL)
	if userInfoURL == "" && discovery != nil {
		userInfoURL = strings.TrimSpace(discovery.UserInfoEndpoint)
	}
	if userInfoURL == "" {
		return nil, fmt.Errorf("provider %s is missing a userinfo endpoint", provider.DisplayName)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := enterpriseAccessHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch userinfo: provider returned %s", resp.Status)
	}

	var claims map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, fmt.Errorf("decode userinfo response: %w", err)
	}
	return claims, nil
}

func enterpriseAccessResolveUser(
	ctx *gin.Context,
	c *container.Container,
	extension *platformdomain.InstalledExtension,
	provider enterpriseAccessProvider,
	claims map[string]interface{},
) (*platformdomain.User, string, error) {
	if c == nil || c.Platform == nil || c.Platform.Session == nil {
		return nil, "", fmt.Errorf("session services are not configured")
	}

	emailClaim := provider.ClaimMapping["email"]
	if emailClaim == "" {
		emailClaim = "email"
	}
	email := enterpriseAccessClaimString(claims, emailClaim)
	if email == "" {
		email = enterpriseAccessClaimString(claims, "email")
	}
	if email == "" {
		return nil, "", fmt.Errorf("OIDC userinfo response did not include an email claim")
	}

	user, err := c.Platform.Session.GetUserByEmail(ctx.Request.Context(), email)
	if err == nil && user != nil {
		return user, "existing user authenticated with enterprise access", nil
	}

	jitProvisioning, _ := extension.Config.GetBool("jitProvisioning")
	if !jitProvisioning {
		return nil, "", fmt.Errorf("no Move Big Rocks user exists for %s and JIT provisioning is disabled", email)
	}
	if c.Platform.User == nil {
		return nil, "", fmt.Errorf("user management services are not configured")
	}

	nameClaim := provider.ClaimMapping["name"]
	if nameClaim == "" {
		nameClaim = "name"
	}
	name := enterpriseAccessClaimString(claims, nameClaim)
	if name == "" {
		name = email
	}

	instanceRole, err := enterpriseAccessResolveInstanceRole(provider, claims)
	if err != nil {
		return nil, "", err
	}
	if instanceRole == nil {
		return nil, "", fmt.Errorf("JIT provisioning requires an operator role claim for enterprise access")
	}

	user, err = c.Platform.User.CreateUser(ctx.Request.Context(), email, name, instanceRole)
	if err != nil {
		return nil, "", fmt.Errorf("provision user from enterprise identity: %w", err)
	}
	if err := c.Platform.User.UpdateUser(ctx.Request.Context(), user.ID, user.Email, user.Name, user.InstanceRole, user.IsActive, true); err == nil {
		user.EmailVerified = true
	}
	return user, "user provisioned via enterprise access", nil
}

func enterpriseAccessResolveInstanceRole(provider enterpriseAccessProvider, claims map[string]interface{}) (*platformdomain.InstanceRole, error) {
	claimKey := provider.ClaimMapping["instance_role"]
	if claimKey == "" {
		claimKey = "mbr_role"
	}
	roleValue := enterpriseAccessClaimString(claims, claimKey)
	if roleValue == "" {
		roleValue = enterpriseAccessClaimString(claims, "role")
	}
	if roleValue == "" {
		roleValue = enterpriseAccessClaimString(claims, "roles")
	}
	if roleValue == "" {
		return nil, nil
	}

	role := platformdomain.CanonicalizeInstanceRole(platformdomain.InstanceRole(roleValue))
	if !role.IsOperator() {
		return nil, fmt.Errorf("enterprise access claim %q does not map to a valid operator role", roleValue)
	}
	return &role, nil
}

func enterpriseAccessHealthFromProviders(c *container.Container, providers []enterpriseAccessProvider) (platformdomain.ExtensionHealthStatus, string) {
	if len(providers) == 0 {
		return platformdomain.ExtensionHealthDegraded, "no identity providers configured"
	}

	activeCount := 0
	for _, provider := range providers {
		if provider.Status != enterpriseAccessProviderStatusActive {
			continue
		}
		activeCount++
		if err := provider.ValidateWithPolicy(c.Config); err != nil {
			return platformdomain.ExtensionHealthDegraded, fmt.Sprintf("provider %s is incomplete: %v", provider.DisplayName, err)
		}
		if _, err := enterpriseAccessResolveSecret(c.Config, provider.ClientSecretRef); err != nil {
			return platformdomain.ExtensionHealthDegraded, err.Error()
		}
	}
	if activeCount == 0 {
		return platformdomain.ExtensionHealthDegraded, "no active identity providers configured"
	}
	return platformdomain.ExtensionHealthHealthy, fmt.Sprintf("%d active identity provider(s) configured", activeCount)
}

func enterpriseAccessIssueState(c *container.Container, payload enterpriseAccessState) (string, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode OIDC state: %w", err)
	}
	secret := enterpriseAccessStateSecret(c)
	if secret == "" {
		return "", fmt.Errorf("enterprise access state signing secret is not configured")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payloadJSON)
	signature := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(payloadJSON) + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func enterpriseAccessVerifyState(c *container.Container, token string) (*enterpriseAccessState, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid OIDC state")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid OIDC state")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid OIDC state")
	}
	secret := enterpriseAccessStateSecret(c)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payloadJSON)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, fmt.Errorf("invalid OIDC state")
	}

	var state enterpriseAccessState
	if err := json.Unmarshal(payloadJSON, &state); err != nil {
		return nil, fmt.Errorf("invalid OIDC state")
	}
	if state.IssuedAt == 0 || time.Since(time.Unix(state.IssuedAt, 0)) > enterpriseAccessOIDCStateMaxAge {
		return nil, fmt.Errorf("OIDC state has expired")
	}
	return &state, nil
}

func enterpriseAccessStateSecret(c *container.Container) string {
	if c == nil || c.Config == nil {
		return ""
	}
	return strings.TrimSpace(c.Config.Auth.JWTSecret)
}

func enterpriseAccessResolveSecret(cfg *config.Config, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", nil
	}
	if err := enterpriseAccessValidateSecretRefPolicy(cfg, ref); err != nil {
		return "", err
	}
	if strings.HasPrefix(ref, "env:") {
		key := strings.TrimSpace(strings.TrimPrefix(ref, "env:"))
		if key == "" {
			return "", fmt.Errorf("clientSecretRef env key is empty")
		}
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			return "", fmt.Errorf("clientSecretRef %s is missing from the environment", ref)
		}
		return value, nil
	}
	return "", fmt.Errorf("clientSecretRef must use a supported scheme")
}

func enterpriseAccessClaimString(claims map[string]interface{}, key string) string {
	key = strings.TrimSpace(key)
	if key == "" || claims == nil {
		return ""
	}
	value, ok := claims[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []interface{}:
		for _, entry := range typed {
			if text := strings.TrimSpace(fmt.Sprintf("%v", entry)); text != "" {
				return text
			}
		}
	case []string:
		for _, entry := range typed {
			if text := strings.TrimSpace(entry); text != "" {
				return text
			}
		}
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
	return ""
}

func enterpriseAccessSetSessionCookie(ctx *gin.Context, cfg *config.Config, token string, session *platformdomain.Session) {
	if ctx == nil || cfg == nil || session == nil {
		return
	}
	secure := strings.TrimSpace(cfg.Server.Environment) != "development"
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(
		"mbr_session",
		token,
		int(session.ExpiresAt.Sub(session.CreatedAt).Seconds()),
		"/",
		strings.TrimSpace(cfg.Auth.CookieDomain),
		secure,
		true,
	)
}

func enterpriseAccessCallbackURL(c *container.Container, ctx *gin.Context) string {
	if c != nil && c.Config != nil && strings.TrimSpace(c.Config.Server.APIBaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(c.Config.Server.APIBaseURL), "/") + enterpriseAccessDefaultCallbackPath
	}
	if ctx != nil && ctx.Request != nil {
		scheme := "https"
		if forwarded := strings.TrimSpace(ctx.GetHeader("X-Forwarded-Proto")); forwarded != "" {
			scheme = forwarded
		} else if ctx.Request.TLS == nil {
			scheme = "http"
		}
		return fmt.Sprintf("%s://%s%s", scheme, ctx.Request.Host, enterpriseAccessDefaultCallbackPath)
	}
	return enterpriseAccessDefaultCallbackPath
}

func enterpriseAccessCallbackRedirectURL(c *container.Container, returnTo string) string {
	returnTo = sanitizeReturnPath(returnTo)
	if returnTo == "" {
		returnTo = enterpriseAccessDefaultReturnPath
	}
	if c != nil && c.Config != nil && strings.TrimSpace(c.Config.Server.AdminBaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(c.Config.Server.AdminBaseURL), "/") + returnTo
	}
	return returnTo
}

func sanitizeReturnPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		return enterpriseAccessDefaultReturnPath
	}
	return value
}

func enterpriseAccessValidateProviderPolicy(cfg *config.Config, provider enterpriseAccessProvider) error {
	checks := map[string]string{
		"issuer":           provider.Issuer,
		"discoveryUrl":     provider.DiscoveryURL,
		"authorizationUrl": provider.AuthorizationURL,
		"tokenUrl":         provider.TokenURL,
		"userInfoUrl":      provider.UserInfoURL,
		"jwksUrl":          provider.JWKSURL,
	}
	for field, value := range checks {
		if err := enterpriseAccessValidateAllowedHost(cfg, field, value); err != nil {
			return err
		}
	}
	return enterpriseAccessValidateSecretRefPolicy(cfg, provider.ClientSecretRef)
}

func enterpriseAccessValidateDiscoveryPolicy(cfg *config.Config, discovery *enterpriseAccessOIDCDiscovery) error {
	if discovery == nil {
		return nil
	}
	checks := map[string]string{
		"issuer":           discovery.Issuer,
		"authorizationUrl": discovery.AuthorizationEndpoint,
		"tokenUrl":         discovery.TokenEndpoint,
		"userInfoUrl":      discovery.UserInfoEndpoint,
		"jwksUrl":          discovery.JWKSURI,
	}
	for field, value := range checks {
		if err := enterpriseAccessValidateProviderURL(field, value, false); err != nil {
			return err
		}
		if err := enterpriseAccessValidateAllowedHost(cfg, field, value); err != nil {
			return err
		}
	}
	return nil
}

func enterpriseAccessValidateAllowedHost(cfg *config.Config, field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" || cfg == nil || len(cfg.EnterpriseAccess.AllowedHosts) == 0 {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL", field)
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return fmt.Errorf("%s host is required", field)
	}
	for _, allowed := range cfg.EnterpriseAccess.AllowedHosts {
		if enterpriseAccessHostMatches(host, allowed) {
			return nil
		}
	}
	return fmt.Errorf("%s host %s is not allowed by enterprise access policy", field, host)
}

func enterpriseAccessHostMatches(host, pattern string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if host == "" || pattern == "" {
		return false
	}
	if strings.HasPrefix(pattern, "*.") {
		base := strings.TrimPrefix(pattern, "*.")
		if !strings.HasSuffix(host, "."+base) {
			return false
		}
		return strings.TrimSuffix(host, "."+base) != ""
	}
	return host == pattern
}

func enterpriseAccessValidateSecretRefPolicy(cfg *config.Config, ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	if strings.HasPrefix(ref, "env:") {
		if cfg != nil && !cfg.EnterpriseAccess.AllowEnvSecretRefs {
			return fmt.Errorf("clientSecretRef env: secrets are disabled by enterprise access policy")
		}
		return nil
	}
	if strings.Contains(ref, ":") {
		return fmt.Errorf("clientSecretRef scheme is not supported")
	}
	return fmt.Errorf("clientSecretRef must use an explicit supported scheme")
}

func enterpriseAccessRandomToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func enterpriseAccessProviderID(displayName string) string {
	displayName = strings.ToLower(strings.TrimSpace(displayName))
	if displayName == "" {
		return "oidc-provider"
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range displayName {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	id := strings.Trim(builder.String(), "-")
	if id == "" {
		id = "oidc-provider"
	}
	return id
}

func enterpriseAccessRenderError(ctx *gin.Context, status int, title, message string) {
	ctx.Data(status, "text/html; charset=utf-8", []byte(fmt.Sprintf(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>%s</title></head>
<body style="font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; margin: 40px;">
  <h1>%s</h1>
  <p>%s</p>
</body>
</html>`, template.HTMLEscapeString(title), template.HTMLEscapeString(title), template.HTMLEscapeString(message))))
}

func enterpriseAccessDefaultClaimMapping() map[string]string {
	return map[string]string{
		"email":         "email",
		"name":          "name",
		"instance_role": "mbr_role",
	}
}

func enterpriseAccessWantsJSON(ctx *gin.Context) bool {
	if ctx == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(ctx.Query("format")), "json") {
		return true
	}
	return strings.Contains(strings.ToLower(ctx.GetHeader("Accept")), "application/json")
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func slicesContains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
