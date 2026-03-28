package platformhandlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	platformservices "github.com/movebigrocks/platform/internal/platform/services"
)

const canonicalDocsBaseURL = "https://movebigrocks.com"

type RuntimeBootstrapConfig struct {
	PublicBaseURL string
	AdminBaseURL  string
	APIBaseURL    string
	DocsBaseURL   string
	Version       string
	GitCommit     string
	BuildDate     string
	SandboxPolicy platformservices.SandboxBootstrapPolicy
}

type RuntimeBootstrapHandler struct {
	config RuntimeBootstrapConfig
}

func NewRuntimeBootstrapHandler(cfg RuntimeBootstrapConfig) *RuntimeBootstrapHandler {
	if strings.TrimSpace(cfg.DocsBaseURL) == "" {
		cfg.DocsBaseURL = canonicalDocsBaseURL
	}
	return &RuntimeBootstrapHandler{config: cfg}
}

func (h *RuntimeBootstrapHandler) GetBootstrapDocument(c *gin.Context) {
	c.JSON(http.StatusOK, h.Document())
}

func (h *RuntimeBootstrapHandler) Document() gin.H {
	cfg := h.config
	publicBaseURL := strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/")
	adminBaseURL := strings.TrimRight(strings.TrimSpace(cfg.AdminBaseURL), "/")
	apiBaseURL := strings.TrimRight(strings.TrimSpace(cfg.APIBaseURL), "/")
	docsBaseURL := strings.TrimRight(strings.TrimSpace(cfg.DocsBaseURL), "/")

	return gin.H{
		"product":     "Move Big Rocks",
		"version":     cfg.Version,
		"git_commit":  cfg.GitCommit,
		"build_date":  cfg.BuildDate,
		"runtime_url": publicBaseURL,
		"runtime": gin.H{
			"base_url":            publicBaseURL,
			"bootstrap_url":       joinURL(publicBaseURL, "/.well-known/mbr-instance.json"),
			"health_url":          joinURL(publicBaseURL, "/health"),
			"cli_login_start_url": joinURL(publicBaseURL, "/auth/cli/start"),
			"cli_login_poll_url":  joinURL(publicBaseURL, "/auth/cli/poll"),
			"admin_base_url":      adminBaseURL,
			"api_base_url":        apiBaseURL,
			"graphql_url":         joinURL(apiBaseURL, "/graphql"),
		},
		"cli": gin.H{
			"command":                "mbr",
			"docs_url":               joinURL(docsBaseURL, "/docs/cli"),
			"install_sh_url":         joinURL(docsBaseURL, "/install.sh"),
			"install_ps1_url":        joinURL(docsBaseURL, "/install.ps1"),
			"release_manifest_url":   joinURL(docsBaseURL, "/releases/latest.json"),
			"runtime_bootstrap_path": "/.well-known/mbr-instance.json",
			"source_repository":      "https://github.com/MoveBigRocks/platform.git",
		},
		"docs": gin.H{
			"docs_hub_url":         joinURL(docsBaseURL, "/docs"),
			"cli_guide_url":        joinURL(docsBaseURL, "/docs/cli"),
			"graphql_guide_url":    joinURL(docsBaseURL, "/docs/graphql"),
			"self_host_url":        joinURL(docsBaseURL, "/docs/self-host"),
			"agent_quickstart_url": joinURL(docsBaseURL, "/docs/agent-quickstart"),
			"extensions_url":       joinURL(docsBaseURL, "/extensions"),
		},
		"sandbox_policy": gin.H{
			"available":               cfg.SandboxPolicy.Available,
			"activation_window_hours": cfg.SandboxPolicy.ActivationWindowHours,
			"default_trial_days":      cfg.SandboxPolicy.DefaultTrialDays,
			"extension_days":          cfg.SandboxPolicy.ExtensionDays,
			"verification_path":       cfg.SandboxPolicy.VerificationPath,
			"create_url":              joinURL(publicBaseURL, "/api/public/sandboxes"),
			"manage_url_template":     joinURL(publicBaseURL, "/api/public/sandboxes/{sandbox_id}"),
			"extend_url_template":     joinURL(publicBaseURL, "/api/public/sandboxes/{sandbox_id}/extend"),
			"export_url_template":     joinURL(publicBaseURL, "/api/public/sandboxes/{sandbox_id}/export"),
		},
	}
}

func joinURL(base, path string) string {
	if strings.TrimSpace(base) == "" {
		return strings.TrimSpace(path)
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}
