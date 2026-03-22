package cliapi

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv(envDisableCredentialStore, "1")
	os.Exit(m.Run())
}

func TestNormalizeGraphQLURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "mbr URL derives api graphql path",
			input: "https://movebigrocks.example.com",
			want:  "https://api.movebigrocks.example.com/graphql",
		},
		{
			name:  "api URL stays on api subdomain",
			input: "https://api.movebigrocks.example.com/",
			want:  "https://api.movebigrocks.example.com/graphql",
		},
		{
			name:  "admin URL rewrites to api graphql path",
			input: "https://admin.movebigrocks.example.com/admin/graphql",
			want:  "https://api.movebigrocks.example.com/graphql",
		},
		{
			name:  "localhost stays local",
			input: "http://localhost:8080",
			want:  "http://localhost:8080/graphql",
		},
		{
			name:    "missing scheme is rejected",
			input:   "movebigrocks.example.com",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeGraphQLURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeGraphQLURL returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoadConfigFallsBackToStoredConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mbr.json")
	t.Setenv(EnvConfigPath, configPath)
	if _, err := SaveStoredConfig("https://movebigrocks.example.com", "hat_saved"); err != nil {
		t.Fatalf("SaveStoredConfig returned error: %v", err)
	}

	got, err := LoadConfig("", "")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if got.InstanceURL != "https://movebigrocks.example.com" {
		t.Fatalf("got InstanceURL %q", got.InstanceURL)
	}
	if got.APIBaseURL != "https://api.movebigrocks.example.com" {
		t.Fatalf("got APIBaseURL %q", got.APIBaseURL)
	}
	if got.GraphQLURL != "https://api.movebigrocks.example.com/graphql" {
		t.Fatalf("got GraphQLURL %q", got.GraphQLURL)
	}
	if got.Token != "hat_saved" {
		t.Fatalf("got token %q", got.Token)
	}
}

func TestSaveStoredContextPersistsWorkspaceAndTeam(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mbr.json")
	t.Setenv(EnvConfigPath, configPath)
	if _, err := SaveStoredConfig("https://movebigrocks.example.com", "hat_saved"); err != nil {
		t.Fatalf("SaveStoredConfig returned error: %v", err)
	}

	workspaceID := "ws_ops"
	teamID := "team_support"
	if _, err := SaveStoredContext(&workspaceID, &teamID, false); err != nil {
		t.Fatalf("SaveStoredContext returned error: %v", err)
	}

	cfg, err := LoadStoredConfig()
	if err != nil {
		t.Fatalf("LoadStoredConfig returned error: %v", err)
	}
	if cfg.CurrentWorkspaceID != workspaceID {
		t.Fatalf("got CurrentWorkspaceID %q", cfg.CurrentWorkspaceID)
	}
	if cfg.CurrentTeamID != teamID {
		t.Fatalf("got CurrentTeamID %q", cfg.CurrentTeamID)
	}
	if cfg.InstanceURL != "https://movebigrocks.example.com" {
		t.Fatalf("got InstanceURL %q", cfg.InstanceURL)
	}
}

func TestSaveStoredContextClearsTeamWhenRequested(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mbr.json")
	t.Setenv(EnvConfigPath, configPath)
	if _, err := SaveStoredConfig("https://movebigrocks.example.com", "hat_saved"); err != nil {
		t.Fatalf("SaveStoredConfig returned error: %v", err)
	}

	workspaceID := "ws_ops"
	teamID := "team_support"
	if _, err := SaveStoredContext(&workspaceID, &teamID, false); err != nil {
		t.Fatalf("SaveStoredContext returned error: %v", err)
	}
	if _, err := SaveStoredContext(nil, nil, true); err != nil {
		t.Fatalf("SaveStoredContext clear returned error: %v", err)
	}

	cfg, err := LoadStoredConfig()
	if err != nil {
		t.Fatalf("LoadStoredConfig returned error: %v", err)
	}
	if cfg.CurrentTeamID != "" {
		t.Fatalf("expected CurrentTeamID to be cleared, got %q", cfg.CurrentTeamID)
	}
	if cfg.CurrentWorkspaceID != workspaceID {
		t.Fatalf("expected CurrentWorkspaceID to remain %q, got %q", workspaceID, cfg.CurrentWorkspaceID)
	}
}

func TestLoadConfigUsesStoredSessionConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mbr.json")
	t.Setenv(EnvConfigPath, configPath)
	if _, err := SaveStoredSessionConfig("https://movebigrocks.example.com", "https://admin.movebigrocks.example.com", "session_saved"); err != nil {
		t.Fatalf("SaveStoredSessionConfig returned error: %v", err)
	}

	got, err := LoadConfig("", "")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if got.InstanceURL != "https://movebigrocks.example.com" {
		t.Fatalf("got InstanceURL %q", got.InstanceURL)
	}
	if got.AuthMode != AuthModeSession {
		t.Fatalf("got auth mode %q", got.AuthMode)
	}
	if got.AdminBaseURL != "https://admin.movebigrocks.example.com" {
		t.Fatalf("got admin base url %q", got.AdminBaseURL)
	}
	if got.APIBaseURL != "https://api.movebigrocks.example.com" {
		t.Fatalf("got APIBaseURL %q", got.APIBaseURL)
	}
	if got.GraphQLURL != "https://admin.movebigrocks.example.com/admin/graphql" {
		t.Fatalf("got GraphQLURL %q", got.GraphQLURL)
	}
	if got.SessionToken != "session_saved" {
		t.Fatalf("got session token %q", got.SessionToken)
	}
}

func TestNormalizeAdminBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "api subdomain rewrites to admin",
			input: "https://api.movebigrocks.example.com/graphql",
			want:  "https://admin.movebigrocks.example.com",
		},
		{
			name:  "mbr host gains admin subdomain",
			input: "https://movebigrocks.example.com",
			want:  "https://admin.movebigrocks.example.com",
		},
		{
			name:  "localhost stays local",
			input: "http://localhost:8080/graphql",
			want:  "http://localhost:8080",
		},
		{
			name:    "invalid URL rejected",
			input:   "example.com",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeAdminBaseURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeAdminBaseURL returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConfigApplyAuthSessionSetsCookie(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://admin.example.com/admin/graphql", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	Config{
		AuthMode:     AuthModeSession,
		SessionToken: "session_saved",
	}.ApplyAuth(req)

	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("unexpected authorization header %q", got)
	}
	cookie, err := req.Cookie("mbr_session")
	if err != nil {
		t.Fatalf("expected session cookie: %v", err)
	}
	if cookie.Value != "session_saved" {
		t.Fatalf("got session cookie %q", cookie.Value)
	}
}

func TestClearStoredConfigRemovesConfigFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mbr.json")
	t.Setenv(EnvConfigPath, configPath)
	if _, err := SaveStoredConfig("https://movebigrocks.example.com", "hat_saved"); err != nil {
		t.Fatalf("SaveStoredConfig returned error: %v", err)
	}
	if err := ClearStoredConfig(); err != nil {
		t.Fatalf("ClearStoredConfig returned error: %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected config file to be removed, stat err=%v", err)
	}
}

func TestSaveStoredSessionConfigUsesCredentialStoreWhenAvailable(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mbr.json")
	t.Setenv(EnvConfigPath, configPath)

	store := &mockCredentialStore{backend: "mock-store"}
	previous := newCredentialStore
	newCredentialStore = func() credentialStore { return store }
	t.Cleanup(func() {
		newCredentialStore = previous
	})

	if _, err := SaveStoredSessionConfig("https://movebigrocks.example.com", "https://admin.movebigrocks.example.com", "session_saved"); err != nil {
		t.Fatalf("SaveStoredSessionConfig returned error: %v", err)
	}

	cfg, err := LoadStoredConfig()
	if err != nil {
		t.Fatalf("LoadStoredConfig returned error: %v", err)
	}
	if cfg.SessionToken != "session_saved" {
		t.Fatalf("expected session token from credential store, got %q", cfg.SessionToken)
	}
	if cfg.CredentialBackend != "mock-store" {
		t.Fatalf("unexpected credential backend %q", cfg.CredentialBackend)
	}
	if cfg.CredentialKey == "" {
		t.Fatalf("expected credential key to be set")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) == "" {
		t.Fatalf("expected config file to be written")
	}
	if cfg.Token != "" {
		t.Fatalf("expected bearer token to remain empty, got %q", cfg.Token)
	}
}

func TestClearStoredConfigDeletesCredentialStoreEntry(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mbr.json")
	t.Setenv(EnvConfigPath, configPath)

	store := &mockCredentialStore{backend: "mock-store"}
	previous := newCredentialStore
	newCredentialStore = func() credentialStore { return store }
	t.Cleanup(func() {
		newCredentialStore = previous
	})

	if _, err := SaveStoredConfig("https://movebigrocks.example.com", "hat_saved"); err != nil {
		t.Fatalf("SaveStoredConfig returned error: %v", err)
	}
	if err := ClearStoredConfig(); err != nil {
		t.Fatalf("ClearStoredConfig returned error: %v", err)
	}
	if len(store.deleted) != 1 {
		t.Fatalf("expected stored credential to be deleted, got %#v", store.deleted)
	}
}

type mockCredentialStore struct {
	backend string
	data    map[string]string
	deleted []string
}

func (m *mockCredentialStore) Name() string { return m.backend }

func (m *mockCredentialStore) Save(account, secret string) error {
	if m.data == nil {
		m.data = map[string]string{}
	}
	m.data[account] = secret
	return nil
}

func (m *mockCredentialStore) Load(account string) (string, error) {
	return m.data[account], nil
}

func (m *mockCredentialStore) Delete(account string) error {
	delete(m.data, account)
	m.deleted = append(m.deleted, account)
	return nil
}
