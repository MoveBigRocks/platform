package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
	"gopkg.in/yaml.v3"
)

const (
	fleetUseCaseInternalOps      = "internal_ops"
	fleetUseCaseStartup          = "startup"
	fleetUseCaseClientDeployment = "client_deployment"
	fleetUseCasePersonal         = "personal"
	fleetUseCaseOther            = "other"

	fleetRegistrationSourceSelfHosted = "self_hosted"
	fleetRegistrationSourceManaged    = "managed"
)

type fleetRegisterConfig struct {
	FleetURL           string
	InstanceID         string
	InstanceName       string
	OperatorEmail      string
	UseCase            string
	RegistrationSource string
	PlatformVersion    string
	HeartbeatEnabled   bool
}

type fleetRegisterInput struct {
	InstanceID         string `json:"instance_id"`
	InstanceName       string `json:"instance_name"`
	OperatorEmail      string `json:"operator_email"`
	UseCase            string `json:"use_case"`
	RegistrationSource string `json:"registration_source"`
	PlatformVersion    string `json:"platform_version,omitempty"`
	TrackingSecret     string `json:"tracking_secret,omitempty"`
}

type fleetRegisterResponse struct {
	Status          string `json:"status"`
	InstanceID      string `json:"instance_id"`
	InstanceName    string `json:"instance_name"`
	LifecycleStatus string `json:"lifecycle_status"`
	SecretIssued    bool   `json:"secret_issued"`
	TrackingSecret  string `json:"tracking_secret,omitempty"`
	Message         string `json:"message,omitempty"`
	FleetURL        string `json:"fleet_url,omitempty"`
}

func runFleet(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printFleetUsage(stderr)
		return 2
	}

	switch args[0] {
	case "register":
		fs := flag.NewFlagSet("mbr fleet register", flag.ContinueOnError)
		fs.SetOutput(stderr)
		configPath := fs.String("config", "", "Path to mbr.instance.yaml")
		fleetURL := fs.String("fleet-url", "", "Fleet control-plane URL, for example https://movebigrocks.com")
		instanceID := fs.String("instance-id", "", "Instance ID")
		instanceName := fs.String("instance-name", "", "Instance name")
		operatorEmail := fs.String("operator-email", "", "Operator email address")
		useCase := fs.String("use-case", "", "Use case: internal_ops, startup, client_deployment, personal, or other")
		registrationSource := fs.String("registration-source", "", "Registration source: self_hosted or managed")
		platformVersion := fs.String("platform-version", "", "Pinned Move Big Rocks core version")
		trackingSecret := fs.String("tracking-secret", "", "Existing tracking secret")
		trackingSecretFile := fs.String("tracking-secret-file", "", "Path to a file containing the existing tracking secret")
		trackingSecretOut := fs.String("tracking-secret-out", "", "Optional file to write the newly issued tracking secret to")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if fs.NArg() != 0 {
			fmt.Fprintln(stderr, "unexpected arguments")
			return 2
		}

		defaults, err := loadFleetRegisterConfig(strings.TrimSpace(*configPath))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		trackingSecretValue, err := resolveFleetTrackingSecret(*trackingSecret, *trackingSecretFile)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		input := fleetRegisterInput{
			InstanceID:         firstNonEmpty(*instanceID, defaults.InstanceID),
			InstanceName:       firstNonEmpty(*instanceName, defaults.InstanceName),
			OperatorEmail:      firstNonEmpty(*operatorEmail, defaults.OperatorEmail),
			UseCase:            normalizeFleetUseCase(firstNonEmpty(*useCase, defaults.UseCase)),
			RegistrationSource: normalizeFleetRegistrationSource(firstNonEmpty(*registrationSource, defaults.RegistrationSource)),
			PlatformVersion:    firstNonEmpty(*platformVersion, defaults.PlatformVersion),
			TrackingSecret:     trackingSecretValue,
		}
		fleetURLValue := firstNonEmpty(*fleetURL, defaults.FleetURL)
		if err := validateFleetRegisterInput(input, fleetURLValue); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		registerURL, err := normalizeFleetRegisterURL(fleetURLValue)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		response, err := runFleetRegister(ctx, registerURL, input)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		response.FleetURL = registerURL

		if response.SecretIssued && strings.TrimSpace(*trackingSecretOut) != "" {
			if err := writeFleetTrackingSecret(filepath.Clean(*trackingSecretOut), response.TrackingSecret); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}

		if *jsonOutput {
			return writeJSON(stdout, response, stderr)
		}

		fmt.Fprintf(stdout, "status:\t%s\n", response.Status)
		fmt.Fprintf(stdout, "fleetURL:\t%s\n", response.FleetURL)
		fmt.Fprintf(stdout, "instanceID:\t%s\n", response.InstanceID)
		fmt.Fprintf(stdout, "instanceName:\t%s\n", response.InstanceName)
		fmt.Fprintf(stdout, "lifecycleStatus:\t%s\n", response.LifecycleStatus)
		fmt.Fprintf(stdout, "secretIssued:\t%t\n", response.SecretIssued)
		if strings.TrimSpace(*trackingSecretOut) != "" && response.SecretIssued {
			fmt.Fprintf(stdout, "trackingSecretOut:\t%s\n", filepath.Clean(*trackingSecretOut))
		}
		if strings.TrimSpace(response.Message) != "" {
			fmt.Fprintf(stdout, "message:\t%s\n", response.Message)
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown fleet command %q\n\n", args[0])
		printFleetUsage(stderr)
		return 2
	}
}

func loadFleetRegisterConfig(path string) (fleetRegisterConfig, error) {
	if path == "" {
		return fleetRegisterConfig{}, nil
	}

	type fleetConfigFile struct {
		Metadata struct {
			Name       string `yaml:"name"`
			InstanceID string `yaml:"instanceID"`
		} `yaml:"metadata"`
		Spec struct {
			Deployment struct {
				Release struct {
					Core struct {
						Version string `yaml:"version"`
					} `yaml:"core"`
				} `yaml:"release"`
			} `yaml:"deployment"`
			Auth struct {
				BreakGlassAdminEmail string `yaml:"breakGlassAdminEmail"`
			} `yaml:"auth"`
			Fleet struct {
				Endpoint     string `yaml:"endpoint"`
				Registration struct {
					OperatorEmail string `yaml:"operatorEmail"`
					UseCase       string `yaml:"useCase"`
					Source        string `yaml:"source"`
				} `yaml:"registration"`
				Heartbeat struct {
					Enabled bool `yaml:"enabled"`
				} `yaml:"heartbeat"`
			} `yaml:"fleet"`
		} `yaml:"spec"`
	}

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fleetRegisterConfig{}, fmt.Errorf("read instance config: %w", err)
	}

	var raw fleetConfigFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fleetRegisterConfig{}, fmt.Errorf("decode instance config yaml: %w", err)
	}

	return fleetRegisterConfig{
		FleetURL:           strings.TrimSpace(raw.Spec.Fleet.Endpoint),
		InstanceID:         strings.TrimSpace(raw.Metadata.InstanceID),
		InstanceName:       strings.TrimSpace(raw.Metadata.Name),
		OperatorEmail:      firstNonEmpty(raw.Spec.Fleet.Registration.OperatorEmail, raw.Spec.Auth.BreakGlassAdminEmail),
		UseCase:            normalizeFleetUseCase(raw.Spec.Fleet.Registration.UseCase),
		RegistrationSource: normalizeFleetRegistrationSource(raw.Spec.Fleet.Registration.Source),
		PlatformVersion:    strings.TrimSpace(raw.Spec.Deployment.Release.Core.Version),
		HeartbeatEnabled:   raw.Spec.Fleet.Heartbeat.Enabled,
	}, nil
}

func resolveFleetTrackingSecret(inlineValue, path string) (string, error) {
	inlineValue = strings.TrimSpace(inlineValue)
	path = strings.TrimSpace(path)
	switch {
	case inlineValue != "" && path != "":
		return "", fmt.Errorf("pass either --tracking-secret or --tracking-secret-file, not both")
	case path == "":
		return inlineValue, nil
	default:
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return "", fmt.Errorf("read tracking secret file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
}

func validateFleetRegisterInput(input fleetRegisterInput, fleetURL string) error {
	switch {
	case strings.TrimSpace(fleetURL) == "":
		return fmt.Errorf("fleet URL is required")
	case strings.TrimSpace(input.InstanceID) == "":
		return fmt.Errorf("instance ID is required")
	case strings.TrimSpace(input.InstanceName) == "":
		return fmt.Errorf("instance name is required")
	case strings.TrimSpace(input.OperatorEmail) == "":
		return fmt.Errorf("operator email is required")
	case !strings.Contains(input.OperatorEmail, "@"):
		return fmt.Errorf("operator email must contain @")
	}

	switch input.UseCase {
	case fleetUseCaseInternalOps, fleetUseCaseStartup, fleetUseCaseClientDeployment, fleetUseCasePersonal, fleetUseCaseOther:
	default:
		return fmt.Errorf("use case must be one of: %s, %s, %s, %s, %s",
			fleetUseCaseInternalOps,
			fleetUseCaseStartup,
			fleetUseCaseClientDeployment,
			fleetUseCasePersonal,
			fleetUseCaseOther,
		)
	}

	switch input.RegistrationSource {
	case fleetRegistrationSourceSelfHosted, fleetRegistrationSourceManaged:
	default:
		return fmt.Errorf("registration source must be %s or %s", fleetRegistrationSourceSelfHosted, fleetRegistrationSourceManaged)
	}
	return nil
}

func normalizeFleetUseCase(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case fleetUseCaseInternalOps:
		return fleetUseCaseInternalOps
	case fleetUseCaseStartup:
		return fleetUseCaseStartup
	case fleetUseCaseClientDeployment:
		return fleetUseCaseClientDeployment
	case fleetUseCasePersonal:
		return fleetUseCasePersonal
	default:
		return fleetUseCaseOther
	}
}

func normalizeFleetRegistrationSource(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case fleetRegistrationSourceManaged:
		return fleetRegistrationSourceManaged
	default:
		return fleetRegistrationSourceSelfHosted
	}
}

func normalizeFleetRegisterURL(raw string) (string, error) {
	apiBaseURL, err := cliapi.NormalizeAPIBaseURL(raw)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(apiBaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid fleet base URL: %w", err)
	}
	u.Path = "/api/fleet/register"
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func runFleetRegister(ctx context.Context, registerURL string, input fleetRegisterInput) (fleetRegisterResponse, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return fleetRegisterResponse{}, fmt.Errorf("marshal fleet register request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, bytes.NewReader(body))
	if err != nil {
		return fleetRegisterResponse{}, fmt.Errorf("build fleet register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return fleetRegisterResponse{}, fmt.Errorf("perform fleet register request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fleetRegisterResponse{}, fmt.Errorf("read fleet register response: %w", err)
	}

	var payload fleetRegisterResponse
	if len(respBody) != 0 {
		if err := json.Unmarshal(respBody, &payload); err != nil {
			return fleetRegisterResponse{}, fmt.Errorf("decode fleet register response: %w", err)
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			message = strings.TrimSpace(string(respBody))
		}
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return fleetRegisterResponse{}, fmt.Errorf("fleet registration failed: status %d: %s", resp.StatusCode, message)
	}
	return payload, nil
}

func writeFleetTrackingSecret(path, secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return errors.New("fleet registration did not return a tracking secret")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create tracking secret directory: %w", err)
	}
	if err := os.WriteFile(path, append([]byte(secret), '\n'), 0o600); err != nil {
		return fmt.Errorf("write tracking secret file: %w", err)
	}
	return nil
}
