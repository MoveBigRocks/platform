package cliapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	EnvInstanceURL = "MBR_URL"
	EnvAPIURL      = "MBR_API_URL" // Secondary env name still read during config loading.
	EnvToken       = "MBR_TOKEN"
	EnvSession     = "MBR_SESSION_TOKEN"
	EnvConfigPath  = "MBR_CONFIG_PATH"
)

type AuthMode string

const (
	AuthModeBearer  AuthMode = "bearer"
	AuthModeSession AuthMode = "session"
)

type Config struct {
	InstanceURL  string
	APIBaseURL   string
	AdminBaseURL string
	GraphQLURL   string
	Token        string
	SessionToken string
	AuthMode     AuthMode
	HTTPClient   *http.Client
}

type StoredConfig struct {
	InstanceURL        string   `json:"instanceURL,omitempty"`
	APIURL             string   `json:"apiURL,omitempty"` // Secondary serialized field still read during config loading.
	AdminBaseURL       string   `json:"adminBaseURL,omitempty"`
	CurrentWorkspaceID string   `json:"currentWorkspaceID,omitempty"`
	CurrentTeamID      string   `json:"currentTeamID,omitempty"`
	AuthMode           AuthMode `json:"authMode,omitempty"`
	Token              string   `json:"token,omitempty"`
	SessionToken       string   `json:"sessionToken,omitempty"`
	CredentialBackend  string   `json:"credentialBackend,omitempty"`
	CredentialKey      string   `json:"credentialKey,omitempty"`
}

func LoadConfig(instanceURLFlag, tokenFlag string) (Config, error) {
	stored, err := LoadStoredConfig()
	if err != nil {
		return Config{}, err
	}

	rawURL := strings.TrimSpace(instanceURLFlag)
	if rawURL == "" {
		rawURL = strings.TrimSpace(os.Getenv(EnvInstanceURL))
	}
	if rawURL == "" {
		rawURL = strings.TrimSpace(os.Getenv(EnvAPIURL))
	}
	if rawURL == "" {
		rawURL = strings.TrimSpace(stored.InstanceURL)
	}
	if rawURL == "" {
		rawURL = strings.TrimSpace(stored.APIURL)
	}
	if rawURL == "" {
		return Config{}, fmt.Errorf("missing Move Big Rocks URL: pass --url or set %s", EnvInstanceURL)
	}

	token := strings.TrimSpace(tokenFlag)
	if token == "" {
		token = strings.TrimSpace(os.Getenv(EnvToken))
	}
	if token == "" {
		token = strings.TrimSpace(stored.Token)
	}

	sessionToken := strings.TrimSpace(os.Getenv(EnvSession))
	if sessionToken == "" {
		sessionToken = strings.TrimSpace(stored.SessionToken)
	}

	if token == "" && sessionToken == "" {
		return Config{}, fmt.Errorf("missing credentials: pass --token, set %s, or run mbr auth login", EnvToken)
	}

	instanceURL, err := normalizeInstanceBaseURL(rawURL)
	if err != nil {
		return Config{}, err
	}
	apiBaseURL, err := NormalizeAPIBaseURL(rawURL)
	if err != nil {
		return Config{}, err
	}
	adminBaseURL := strings.TrimSpace(stored.AdminBaseURL)
	if adminBaseURL == "" {
		adminBaseURL, err = normalizeAdminBaseURL(rawURL)
		if err != nil {
			return Config{}, err
		}
	} else {
		adminBaseURL, err = normalizeAdminBaseURL(adminBaseURL)
		if err != nil {
			return Config{}, err
		}
	}

	authMode := AuthModeBearer
	graphQLURL, err := normalizeGraphQLURL(rawURL)
	if err != nil {
		return Config{}, err
	}
	if token == "" && sessionToken != "" {
		authMode = AuthModeSession
		graphQLURL, err = normalizeAdminGraphQLURL(adminBaseURL)
		if err != nil {
			return Config{}, err
		}
	}

	return Config{
		InstanceURL:  instanceURL,
		APIBaseURL:   apiBaseURL,
		AdminBaseURL: adminBaseURL,
		GraphQLURL:   graphQLURL,
		Token:        token,
		SessionToken: sessionToken,
		AuthMode:     authMode,
	}, nil
}

func ConfigPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv(EnvConfigPath)); override != "" {
		return override, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return filepath.Join(dir, "mbr", "config.json"), nil
}

func LoadStoredConfig() (StoredConfig, error) {
	path, err := ConfigPath()
	if err != nil {
		return StoredConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return StoredConfig{}, nil
		}
		return StoredConfig{}, fmt.Errorf("read CLI config: %w", err)
	}
	if len(data) == 0 {
		return StoredConfig{}, nil
	}

	var cfg StoredConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return StoredConfig{}, fmt.Errorf("decode CLI config: %w", err)
	}
	cfg.APIURL = strings.TrimSpace(cfg.APIURL)
	cfg.InstanceURL = strings.TrimSpace(cfg.InstanceURL)
	cfg.AdminBaseURL = strings.TrimSpace(cfg.AdminBaseURL)
	cfg.CurrentWorkspaceID = strings.TrimSpace(cfg.CurrentWorkspaceID)
	cfg.CurrentTeamID = strings.TrimSpace(cfg.CurrentTeamID)
	cfg.Token = strings.TrimSpace(cfg.Token)
	cfg.SessionToken = strings.TrimSpace(cfg.SessionToken)
	cfg.CredentialBackend = strings.TrimSpace(cfg.CredentialBackend)
	cfg.CredentialKey = strings.TrimSpace(cfg.CredentialKey)
	if cfg.AuthMode == "" {
		switch {
		case cfg.SessionToken != "" && cfg.Token == "":
			cfg.AuthMode = AuthModeSession
		case cfg.Token != "":
			cfg.AuthMode = AuthModeBearer
		}
	}
	if cfg.CredentialKey != "" && cfg.Token == "" && cfg.SessionToken == "" {
		if store := newCredentialStore(); store != nil {
			secret, err := store.Load(cfg.CredentialKey)
			if err == nil {
				switch cfg.AuthMode {
				case AuthModeSession:
					cfg.SessionToken = secret
				default:
					cfg.Token = secret
				}
			}
		}
	}
	return cfg, nil
}

func SaveStoredConfig(instanceURL, token string) (string, error) {
	return saveStoredCredentialConfig(strings.TrimSpace(instanceURL), "", AuthModeBearer, strings.TrimSpace(token))
}

func SaveStoredSessionConfig(instanceURL, adminBaseURL, sessionToken string) (string, error) {
	return saveStoredCredentialConfig(strings.TrimSpace(instanceURL), strings.TrimSpace(adminBaseURL), AuthModeSession, strings.TrimSpace(sessionToken))
}

func SaveStoredContext(workspaceID, teamID *string, clearTeam bool) (string, error) {
	cfg, err := LoadStoredConfig()
	if err != nil {
		return "", err
	}
	if workspaceID != nil {
		cfg.CurrentWorkspaceID = strings.TrimSpace(*workspaceID)
		if cfg.CurrentWorkspaceID == "" {
			cfg.CurrentTeamID = ""
		}
	}
	if clearTeam {
		cfg.CurrentTeamID = ""
	}
	if teamID != nil {
		cfg.CurrentTeamID = strings.TrimSpace(*teamID)
	}
	return writeStoredConfig(cfg)
}

func writeStoredConfig(cfg StoredConfig) (string, error) {
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode CLI config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create CLI config directory: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write CLI config: %w", err)
	}
	return path, nil
}

func ClearStoredConfig() error {
	stored, err := LoadStoredConfig()
	if err != nil {
		return err
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	var deleteErr error
	if stored.CredentialKey != "" {
		if store := newCredentialStore(); store != nil {
			deleteErr = store.Delete(stored.CredentialKey)
		}
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove CLI config: %w", err)
	}
	if deleteErr != nil {
		return deleteErr
	}
	return nil
}

func saveStoredCredentialConfig(instanceURL, adminBaseURL string, authMode AuthMode, secret string) (string, error) {
	cfg, err := LoadStoredConfig()
	if err != nil {
		return "", err
	}
	cfg.InstanceURL = instanceURL
	cfg.AdminBaseURL = adminBaseURL
	cfg.AuthMode = authMode
	cfg.Token = ""
	cfg.SessionToken = ""
	cfg.CredentialBackend = ""
	cfg.CredentialKey = ""

	if store := newCredentialStore(); store != nil && secret != "" {
		key := credentialKey(authMode, instanceURL)
		if err := store.Save(key, secret); err == nil {
			cfg.CredentialBackend = store.Name()
			cfg.CredentialKey = key
			return writeStoredConfig(cfg)
		}
	}

	switch authMode {
	case AuthModeSession:
		cfg.SessionToken = secret
	default:
		cfg.Token = secret
	}
	return writeStoredConfig(cfg)
}

func normalizeInstanceBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("Move Big Rocks URL is required")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid Move Big Rocks URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("Move Big Rocks URL must include scheme and host")
	}

	host := u.Hostname()
	switch {
	case host == "localhost" || host == "127.0.0.1" || host == "::1":
		// keep same host for local development
	case strings.HasPrefix(host, "api."):
		u.Host = strings.TrimPrefix(u.Host, "api.")
	case strings.HasPrefix(host, "admin."):
		u.Host = strings.TrimPrefix(u.Host, "admin.")
	}

	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func NormalizeAPIBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("Move Big Rocks URL is required")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid Move Big Rocks URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("Move Big Rocks URL must include scheme and host")
	}

	host := u.Hostname()
	switch {
	case host == "localhost" || host == "127.0.0.1" || host == "::1":
		// keep same host for local development
	case strings.HasPrefix(host, "api."):
		// already on API subdomain
	case strings.HasPrefix(host, "admin."):
		u.Host = "api." + strings.TrimPrefix(u.Host, "admin.")
	default:
		u.Host = "api." + u.Host
	}

	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func normalizeGraphQLURL(raw string) (string, error) {
	apiBaseURL, err := NormalizeAPIBaseURL(raw)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(apiBaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid API base URL: %w", err)
	}
	u.Path = "/graphql"
	return u.String(), nil
}

func normalizeAdminBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("Move Big Rocks URL is required")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid Move Big Rocks URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("Move Big Rocks URL must include scheme and host")
	}

	host := u.Hostname()
	switch {
	case host == "localhost" || host == "127.0.0.1" || host == "::1":
		// keep same host for local development
	case strings.HasPrefix(host, "api."):
		u.Host = "admin." + strings.TrimPrefix(u.Host, "api.")
	case !strings.HasPrefix(host, "admin."):
		u.Host = "admin." + u.Host
	}

	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func normalizeAdminGraphQLURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("admin base URL is required")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid admin base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("admin base URL must include scheme and host")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/graphql"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func (c Config) ApplyAuth(req *http.Request) {
	switch c.AuthMode {
	case AuthModeSession:
		if strings.TrimSpace(c.SessionToken) != "" {
			req.AddCookie(&http.Cookie{
				Name:     "mbr_session",
				Value:    c.SessionToken,
				Path:     "/",
				HttpOnly: true,
			})
		}
	default:
		if strings.TrimSpace(c.Token) != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
	}
}
