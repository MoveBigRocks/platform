package platformdomain

import (
	"fmt"
	"strings"
)

// RequiresPrivilegedInstallPolicy reports whether an extension must use the privileged install path.
func (m ExtensionManifest) RequiresPrivilegedInstallPolicy() bool {
	switch m.Kind {
	case ExtensionKindIdentity, ExtensionKindConnector:
		return true
	default:
		return m.Risk == ExtensionRiskPrivileged || m.Scope == ExtensionScopeInstance
	}
}

// ValidateGenericInstallPolicy enforces the generic runtime install constraints.
func (m ExtensionManifest) ValidateGenericInstallPolicy() error {
	if m.Scope != ExtensionScopeWorkspace {
		return fmt.Errorf("only workspace-scoped extensions are installable through the current runtime")
	}
	if m.Risk != ExtensionRiskStandard {
		return fmt.Errorf("privileged extensions are not installable through the generic runtime")
	}
	switch m.Kind {
	case ExtensionKindProduct, ExtensionKindOperational:
		return nil
	case ExtensionKindIdentity:
		return fmt.Errorf("identity extensions require the dedicated privileged runtime")
	case ExtensionKindConnector:
		return fmt.Errorf("connector extensions require the dedicated privileged runtime")
	default:
		return fmt.Errorf("unsupported extension kind %q", m.Kind)
	}
}

// ValidatePrivilegedInstallPolicy enforces privileged runtime install constraints.
func (m ExtensionManifest) ValidatePrivilegedInstallPolicy(workspaceID string, publisherAllowed func(string) bool) error {
	switch m.Kind {
	case ExtensionKindIdentity, ExtensionKindConnector:
	default:
		return fmt.Errorf("privileged runtime currently supports only identity and connector extensions")
	}
	if m.Risk != ExtensionRiskPrivileged {
		return fmt.Errorf("%s extensions must declare risk=privileged", m.Kind)
	}
	if m.RuntimeClass != ExtensionRuntimeClassServiceBacked {
		return fmt.Errorf("%s extensions require service_backed runtime", m.Kind)
	}
	switch m.Scope {
	case ExtensionScopeWorkspace:
		if strings.TrimSpace(workspaceID) == "" {
			return fmt.Errorf("workspace_id is required for privileged workspace-scoped extensions")
		}
	case ExtensionScopeInstance:
		if strings.TrimSpace(workspaceID) != "" {
			return fmt.Errorf("workspace_id is not allowed for privileged instance-scoped extensions")
		}
	default:
		return fmt.Errorf("unsupported privileged extension scope %q", m.Scope)
	}
	if publisherAllowed != nil && !publisherAllowed(m.Publisher) {
		return fmt.Errorf("privileged extensions are restricted to trusted first-party publishers; %s is not allowed", strings.TrimSpace(m.Publisher))
	}
	return nil
}

// BaseHealthStatus resolves extension health when no runtime probe is required.
func (e *InstalledExtension) BaseHealthStatus() (ExtensionHealthStatus, string, bool) {
	if e == nil {
		return ExtensionHealthFailed, "extension not found", true
	}
	if e.Status != ExtensionStatusActive {
		return ExtensionHealthInactive, "extension inactive", true
	}
	if e.ValidationStatus == ExtensionValidationInvalid {
		message := strings.TrimSpace(e.ValidationMessage)
		if message == "" {
			message = "extension validation failed"
		}
		return ExtensionHealthFailed, message, true
	}
	if e.Manifest.RuntimeClass != ExtensionRuntimeClassServiceBacked {
		return ExtensionHealthHealthy, "extension active", true
	}
	return "", "", false
}

// DefaultExtensionHealthMessage provides the default user-facing message for a runtime health state.
func DefaultExtensionHealthMessage(status ExtensionHealthStatus) string {
	switch status {
	case ExtensionHealthHealthy:
		return "service runtime healthy"
	case ExtensionHealthDegraded:
		return "service runtime degraded"
	case ExtensionHealthFailed:
		return "service runtime failed"
	default:
		return "service runtime health unknown"
	}
}
