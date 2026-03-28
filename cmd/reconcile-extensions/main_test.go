package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunRenderRuntimeManifestFromDesiredState(t *testing.T) {
	root := t.TempDir()
	bundleDir := filepath.Join(root, "ats")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "manifest.json"), []byte(`{
	  "slug": "ats",
	  "name": "Applicant Tracking",
	  "version": "1.0.0",
	  "publisher": "DemandOps",
	  "kind": "product",
	  "scope": "workspace",
	  "risk": "standard",
	  "runtimeClass": "service_backed",
	  "storageClass": "owned_schema",
	  "schema": {
	    "name": "ats",
	    "packageKey": "demandops/ats",
	    "targetVersion": "1"
	  },
	  "runtime": {
	    "protocol": "unix_socket_http",
	    "ociReference": "ghcr.io/movebigrocks/mbr-ext-ats-runtime:v1.0.0"
	  }
	}`), 0o600))

	desiredState := filepath.Join(root, "desired-state.yaml")
	require.NoError(t, os.WriteFile(desiredState, []byte("extensions:\n  installed:\n    - slug: ats\n      ref: "+bundleDir+"\n      scope: workspace\n      workspace: default\n"), 0o600))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(t.Context(), []string{
		"render-runtime-manifest",
		"--desired-state", desiredState,
		"--output", "-",
	}, &stdout, &stderr)
	require.Equal(t, 0, exitCode, stderr.String())
	assert.Contains(t, stdout.String(), "\"slug\": \"ats\"")
	assert.Contains(t, stdout.String(), "\"artifact\": \"ghcr.io/movebigrocks/mbr-ext-ats-runtime:v1.0.0\"")
	assert.Empty(t, stderr.String())
}
