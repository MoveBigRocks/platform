package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	platformhandlers "github.com/movebigrocks/platform/internal/platform/handlers"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
)

func main() {
	var (
		outPath   string
		version   string
		gitSHA    string
		buildDate string
	)

	flag.StringVar(&outPath, "out", "", "path to write the runtime bootstrap document")
	flag.StringVar(&version, "version", "", "runtime version string")
	flag.StringVar(&gitSHA, "git-sha", "", "runtime git commit")
	flag.StringVar(&buildDate, "build-date", "", "runtime build date")
	flag.Parse()

	if outPath == "" {
		fmt.Fprintln(os.Stderr, "--out is required")
		os.Exit(2)
	}

	publicBaseURL, adminBaseURL, apiBaseURL := runtimeBaseURLs()
	runtimeDomain := envString("SANDBOX_RUNTIME_DOMAIN", "movebigrocks.io")

	sandboxService := platformservices.NewSandboxService(nil, platformservices.SandboxServiceConfig{
		PublicBaseURL:    publicBaseURL,
		RuntimeDomain:    runtimeDomain,
		ActivationTTL:    envDuration("SANDBOX_ACTIVATION_TTL", 24*time.Hour),
		TrialTTL:         envDuration("SANDBOX_TRIAL_TTL", 5*24*time.Hour),
		ExtensionTTL:     envDuration("SANDBOX_EXTENSION_TTL", 30*24*time.Hour),
		VerificationPath: envString("SANDBOX_VERIFICATION_PATH", "/sandbox/verify"),
	})

	handler := platformhandlers.NewRuntimeBootstrapHandler(platformhandlers.RuntimeBootstrapConfig{
		PublicBaseURL: publicBaseURL,
		AdminBaseURL:  adminBaseURL,
		APIBaseURL:    apiBaseURL,
		Version:       version,
		GitCommit:     gitSHA,
		BuildDate:     buildDate,
		SandboxPolicy: sandboxService.BootstrapPolicy(),
	})

	document, err := json.MarshalIndent(handler.Document(), "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal bootstrap document: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, append(document, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write bootstrap document: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("wrote %s\n", outPath)
}

func runtimeBaseURLs() (string, string, string) {
	environment := envString("ENVIRONMENT", "development")
	domain := envString("DOMAIN", "movebigrocks.com")
	port := envString("PORT", "8080")

	protocol := "https"
	if environment == "development" {
		protocol = "http"
		if domain == "movebigrocks.com" {
			domain = "lvh.me"
		}
	}

	baseURL := fmt.Sprintf("%s://%s", protocol, domain)
	adminURL := fmt.Sprintf("%s://admin.%s", protocol, domain)
	apiURL := fmt.Sprintf("%s://api.%s", protocol, domain)

	if environment == "development" {
		baseURL = fmt.Sprintf("%s:%s", baseURL, port)
		adminURL = fmt.Sprintf("%s:%s", adminURL, port)
		apiURL = fmt.Sprintf("%s:%s", apiURL, port)
	}

	return envString("BASE_URL", baseURL), envString("ADMIN_BASE_URL", adminURL), envString("API_BASE_URL", apiURL)
}

func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}
