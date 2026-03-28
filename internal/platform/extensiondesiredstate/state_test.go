package extensiondesiredstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInstalledEntryDefaultsToPresentAndActive(t *testing.T) {
	doc, err := Parse([]byte(`
extensions:
  installed:
    - slug: ats
      ref: ghcr.io/movebigrocks/mbr-ext-ats:v1.0.0
      scope: workspace
      workspace: default
`))
	require.NoError(t, err)
	require.Len(t, doc.Extensions.Installed, 1)
	assert.Equal(t, StatePresent, doc.Extensions.Installed[0].DesiredState())
	assert.True(t, doc.Extensions.Installed[0].DesiredActive())
}

func TestResolveConfigMergesSecretRefs(t *testing.T) {
	entry := InstalledEntry{
		Slug:      "ats",
		Ref:       "ghcr.io/movebigrocks/mbr-ext-ats:v1.0.0",
		Scope:     "workspace",
		Workspace: "default",
		Config: map[string]any{
			"region": "eu",
		},
		ConfigSecretRefs: map[string]string{
			"apiKey": "ATS_API_KEY",
		},
	}
	entry.Normalize()

	config, err := entry.ResolveConfig(func(key string) (string, bool) {
		if key == "ATS_API_KEY" {
			return "secret-value", true
		}
		return "", false
	})
	require.NoError(t, err)
	assert.Equal(t, "eu", config["region"])
	assert.Equal(t, "secret-value", config["apiKey"])
}

func TestValidateRejectsMissingWorkspaceForWorkspaceScope(t *testing.T) {
	_, err := Parse([]byte(`
extensions:
  installed:
    - slug: ats
      ref: ghcr.io/movebigrocks/mbr-ext-ats:v1.0.0
      scope: workspace
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace is required")
}

func TestValidateAllowsExplicitAbsentWithoutRef(t *testing.T) {
	doc, err := Parse([]byte(`
extensions:
  installed:
    - slug: ats
      state: absent
      scope: workspace
      workspace: default
`))
	require.NoError(t, err)
	assert.Equal(t, StateAbsent, doc.Extensions.Installed[0].DesiredState())
}
