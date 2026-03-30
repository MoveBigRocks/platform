package extensionreconcile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/platform/extensionbundle"
	"github.com/movebigrocks/platform/internal/platform/extensiondesiredstate"
	sqlstore "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
	domain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

func TestApplyAndCheckWorkspaceExtensionLifecycle(t *testing.T) {
	store, cleanup := testutil.SetupTestPostgresStore(t)
	defer cleanup()

	workspace := &domain.Workspace{
		Name:      "Default",
		Slug:      "default",
		ShortCode: "DFLT",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, store.Workspaces().CreateWorkspace(context.Background(), workspace))

	options := []platformservices.ExtensionServiceOption{}
	if concrete, ok := store.(*sqlstore.Store); ok {
		options = append(options, platformservices.WithExtensionSchemaRuntime(concrete.ExtensionSchemaMigrator()))
	}
	service := platformservices.NewExtensionServiceWithOptions(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
		options...,
	)
	engine := NewEngine(
		DefaultBundleLoader{Config: extensionbundle.DefaultResolverConfigFromEnv()},
		store.Extensions(),
		store.Workspaces(),
		service,
	)
	engine.Actor = "system:test-reconcile"

	bundleDir := filepath.Join(t.TempDir(), "ats")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "manifest.json"), []byte(`{
	  "slug": "ats",
	  "name": "Applicant Tracking",
	  "version": "1.0.0",
	  "publisher": "DemandOps",
	  "kind": "product",
	  "scope": "workspace",
	  "risk": "standard",
	  "runtimeClass": "bundle",
	  "storageClass": "shared_primitives_only"
	}`), 0o600))

	presentDoc, err := extensiondesiredstate.Parse([]byte(`
extensions:
  installed:
    - slug: ats
      ref: ` + bundleDir + `
      scope: workspace
      workspace: default
      config:
        region: eu
`))
	require.NoError(t, err)

	applyResult, err := engine.Apply(context.Background(), presentDoc, "extensions/desired-state.yaml")
	require.NoError(t, err)
	require.True(t, applyResult.Clean)

	installed, err := store.Extensions().GetInstalledExtensionBySlug(context.Background(), workspace.ID, "ats")
	require.NoError(t, err)
	assert.Equal(t, domain.ExtensionStatusActive, installed.Status)
	assert.Equal(t, "1.0.0", installed.Version)
	region, ok := installed.Config.GetString("region")
	require.True(t, ok)
	assert.Equal(t, "eu", region)

	checkResult, err := engine.Check(context.Background(), presentDoc, "extensions/desired-state.yaml")
	require.NoError(t, err)
	assert.True(t, checkResult.Clean)

	absentDoc, err := extensiondesiredstate.Parse([]byte(`
extensions:
  installed:
    - slug: ats
      state: absent
      scope: workspace
      workspace: default
`))
	require.NoError(t, err)

	removeResult, err := engine.Apply(context.Background(), absentDoc, "extensions/desired-state.yaml")
	require.NoError(t, err)
	require.True(t, removeResult.Clean)

	_, err = store.Extensions().GetInstalledExtensionBySlug(context.Background(), workspace.ID, "ats")
	require.Error(t, err)
}
