package platformdomain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtensionInstallPoliciesCoverage(t *testing.T) {
	generic := ExtensionManifest{
		Kind:  ExtensionKindOperational,
		Scope: ExtensionScopeWorkspace,
		Risk:  ExtensionRiskStandard,
	}
	require.False(t, generic.RequiresPrivilegedInstallPolicy())
	require.NoError(t, generic.ValidateGenericInstallPolicy())

	identity := ExtensionManifest{
		Kind:  ExtensionKindIdentity,
		Scope: ExtensionScopeWorkspace,
		Risk:  ExtensionRiskStandard,
	}
	require.True(t, identity.RequiresPrivilegedInstallPolicy())
	require.EqualError(t, identity.ValidateGenericInstallPolicy(), "identity extensions require the dedicated privileged runtime")

	connector := ExtensionManifest{
		Kind:         ExtensionKindConnector,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskPrivileged,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
		Publisher:    "DemandOps",
	}
	require.True(t, connector.RequiresPrivilegedInstallPolicy())
	require.NoError(t, connector.ValidatePrivilegedInstallPolicy("ws_1", func(publisher string) bool {
		return publisher == "DemandOps"
	}))
	require.EqualError(t,
		connector.ValidatePrivilegedInstallPolicy("", func(string) bool { return true }),
		"workspace_id is required for privileged workspace-scoped extensions",
	)
	require.EqualError(t,
		connector.ValidatePrivilegedInstallPolicy("ws_1", func(string) bool { return false }),
		"privileged extensions are restricted to trusted first-party publishers; DemandOps is not allowed",
	)

	instanceIdentity := ExtensionManifest{
		Kind:         ExtensionKindIdentity,
		Scope:        ExtensionScopeInstance,
		Risk:         ExtensionRiskPrivileged,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
		Publisher:    "DemandOps",
	}
	require.NoError(t, instanceIdentity.ValidatePrivilegedInstallPolicy("", nil))
	require.EqualError(t,
		instanceIdentity.ValidatePrivilegedInstallPolicy("ws_1", nil),
		"workspace_id is not allowed for privileged instance-scoped extensions",
	)

	wrongRuntime := ExtensionManifest{
		Kind:         ExtensionKindConnector,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskPrivileged,
		RuntimeClass: ExtensionRuntimeClassBundle,
	}
	require.EqualError(t, wrongRuntime.ValidatePrivilegedInstallPolicy("ws_1", nil), "connector extensions require service_backed runtime")

	wrongKind := ExtensionManifest{
		Kind:         ExtensionKindProduct,
		Scope:        ExtensionScopeWorkspace,
		Risk:         ExtensionRiskPrivileged,
		RuntimeClass: ExtensionRuntimeClassServiceBacked,
	}
	require.EqualError(t, wrongKind.ValidatePrivilegedInstallPolicy("ws_1", nil), "privileged runtime currently supports only identity and connector extensions")
}
