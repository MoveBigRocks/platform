package extensiondesiredstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"gopkg.in/yaml.v3"
)

const (
	StatePresent = "present"
	StateAbsent  = "absent"
)

type Document struct {
	Extensions Extensions `yaml:"extensions"`
}

type Extensions struct {
	Installed []InstalledEntry `yaml:"installed"`
	Planned   []PlannedEntry   `yaml:"planned"`
}

type InstalledEntry struct {
	Slug             string                  `yaml:"slug"`
	State            string                  `yaml:"state,omitempty"`
	Source           string                  `yaml:"source,omitempty"`
	Ref              string                  `yaml:"ref,omitempty"`
	Publisher        string                  `yaml:"publisher,omitempty"`
	Kind             string                  `yaml:"kind,omitempty"`
	Scope            string                  `yaml:"scope,omitempty"`
	Risk             string                  `yaml:"risk,omitempty"`
	LicenseRequired  bool                    `yaml:"licenseRequired,omitempty"`
	Workspace        string                  `yaml:"workspace,omitempty"`
	Activate         *bool                   `yaml:"activate,omitempty"`
	Config           map[string]any          `yaml:"config,omitempty"`
	ConfigSecretRefs map[string]string       `yaml:"configSecretRefs,omitempty"`
	PreviewWorkspace string                  `yaml:"previewWorkspace,omitempty"`
	Verification     VerificationExpectation `yaml:"verification,omitempty"`
}

type PlannedEntry struct {
	Slug      string `yaml:"slug"`
	Publisher string `yaml:"publisher,omitempty"`
	Kind      string `yaml:"kind,omitempty"`
	Scope     string `yaml:"scope,omitempty"`
	Risk      string `yaml:"risk,omitempty"`
	Reason    string `yaml:"reason,omitempty"`
}

type VerificationExpectation struct {
	RequiresThreatModel       bool `yaml:"requiresThreatModel,omitempty"`
	RequiresStagingValidation bool `yaml:"requiresStagingValidation,omitempty"`
}

func LoadFile(path string) (Document, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Document{}, fmt.Errorf("read desired state file: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (Document, error) {
	var doc Document
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Document{}, fmt.Errorf("decode desired state yaml: %w", err)
	}
	doc.Normalize()
	if err := doc.Validate(); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func (d *Document) Normalize() {
	if d == nil {
		return
	}
	for i := range d.Extensions.Installed {
		d.Extensions.Installed[i].Normalize()
	}
	for i := range d.Extensions.Planned {
		d.Extensions.Planned[i].Normalize()
	}
}

func (d Document) Validate() error {
	for i, entry := range d.Extensions.Installed {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("extensions.installed[%d]: %w", i, err)
		}
	}
	for i, entry := range d.Extensions.Planned {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("extensions.planned[%d]: %w", i, err)
		}
	}
	return nil
}

func (e *InstalledEntry) Normalize() {
	if e == nil {
		return
	}
	e.Slug = strings.TrimSpace(e.Slug)
	e.State = normalizeState(e.State)
	e.Source = strings.TrimSpace(strings.ToLower(e.Source))
	e.Ref = strings.TrimSpace(e.Ref)
	e.Publisher = strings.TrimSpace(e.Publisher)
	e.Kind = strings.TrimSpace(strings.ToLower(e.Kind))
	e.Scope = strings.TrimSpace(strings.ToLower(e.Scope))
	e.Risk = strings.TrimSpace(strings.ToLower(e.Risk))
	e.Workspace = strings.TrimSpace(e.Workspace)
	e.PreviewWorkspace = strings.TrimSpace(e.PreviewWorkspace)
	if e.Config == nil {
		e.Config = map[string]any{}
	}
	if e.ConfigSecretRefs == nil {
		e.ConfigSecretRefs = map[string]string{}
	}
}

func (e InstalledEntry) Validate() error {
	if e.Slug == "" {
		return fmt.Errorf("slug is required")
	}
	switch e.DesiredState() {
	case StatePresent:
		if e.Ref == "" {
			return fmt.Errorf("ref is required when state is present")
		}
	case StateAbsent:
	default:
		return fmt.Errorf("state must be one of %q or %q", StatePresent, StateAbsent)
	}

	scope := platformdomain.ExtensionScope(e.Scope)
	switch scope {
	case platformdomain.ExtensionScopeWorkspace:
		if e.Workspace == "" {
			return fmt.Errorf("workspace is required for workspace-scoped entries")
		}
	case platformdomain.ExtensionScopeInstance:
		if e.Workspace != "" {
			return fmt.Errorf("workspace is not allowed for instance-scoped entries")
		}
	default:
		return fmt.Errorf("scope must be %q or %q", platformdomain.ExtensionScopeWorkspace, platformdomain.ExtensionScopeInstance)
	}

	if e.PreviewWorkspace != "" && scope != platformdomain.ExtensionScopeWorkspace {
		return fmt.Errorf("previewWorkspace is only supported for workspace-scoped entries")
	}

	for key, envName := range e.ConfigSecretRefs {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("configSecretRefs keys must not be empty")
		}
		if strings.TrimSpace(envName) == "" {
			return fmt.Errorf("configSecretRefs[%s] must reference a non-empty environment variable name", key)
		}
	}
	return nil
}

func (e InstalledEntry) DesiredState() string {
	if normalizeState(e.State) == "" {
		return StatePresent
	}
	return normalizeState(e.State)
}

func (e InstalledEntry) DesiredActive() bool {
	if e.Activate == nil {
		return true
	}
	return *e.Activate
}

func (e InstalledEntry) ResolveConfig(envLookup func(string) (string, bool)) (map[string]any, error) {
	config := cloneMap(e.Config)
	for key, envName := range e.ConfigSecretRefs {
		envName = strings.TrimSpace(envName)
		if envLookup == nil {
			return nil, fmt.Errorf("configSecretRefs[%s] requires an environment lookup function", key)
		}
		value, ok := envLookup(envName)
		if !ok {
			return nil, fmt.Errorf("required environment variable %s is not set for configSecretRefs[%s]", envName, key)
		}
		config[key] = value
	}
	return config, nil
}

func (e *PlannedEntry) Normalize() {
	if e == nil {
		return
	}
	e.Slug = strings.TrimSpace(e.Slug)
	e.Publisher = strings.TrimSpace(e.Publisher)
	e.Kind = strings.TrimSpace(strings.ToLower(e.Kind))
	e.Scope = strings.TrimSpace(strings.ToLower(e.Scope))
	e.Risk = strings.TrimSpace(strings.ToLower(e.Risk))
	e.Reason = strings.TrimSpace(e.Reason)
}

func (e PlannedEntry) Validate() error {
	if e.Slug == "" {
		return fmt.Errorf("slug is required")
	}
	if e.Scope != "" {
		switch platformdomain.ExtensionScope(e.Scope) {
		case platformdomain.ExtensionScopeWorkspace, platformdomain.ExtensionScopeInstance:
		default:
			return fmt.Errorf("scope must be %q or %q when set", platformdomain.ExtensionScopeWorkspace, platformdomain.ExtensionScopeInstance)
		}
	}
	return nil
}

func normalizeState(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", StatePresent:
		return strings.TrimSpace(strings.ToLower(value))
	case StateAbsent:
		return StateAbsent
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	data, err := json.Marshal(input)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	if out == nil {
		return map[string]any{}
	}
	return out
}
