package extensionbundle

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleBundle = `{
  "manifest": {
    "slug": "ats",
    "name": "Applicant Tracking",
    "version": "1.0.0",
    "publisher": "DemandOps",
    "kind": "product",
    "scope": "workspace",
    "risk": "standard",
    "runtimeClass": "bundle",
    "storageClass": "shared_primitives_only"
  },
  "assets": []
}`

func TestReadSourceLocalBundleFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bundle.hext")
	require.NoError(t, os.WriteFile(path, []byte(sampleBundle), 0o600))

	payload, err := ReadSource(t.Context(), path, "", ResolverConfig{})
	require.NoError(t, err)
	assert.Equal(t, SourceKindLocal, payload.Kind)
	assert.Equal(t, "ats", payload.Bundle.Manifest["slug"])
}

func TestReadSourceLocalBundleDirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{
	  "slug": "ats",
	  "name": "Applicant Tracking",
	  "version": "1.0.0",
	  "publisher": "DemandOps",
	  "kind": "product",
	  "scope": "workspace",
	  "risk": "standard"
	}`), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "assets", "index.html"), []byte("<html></html>"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "migrations"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "migrations", "001_init.sql"), []byte("select 1;"), 0o600))

	payload, err := ReadSource(t.Context(), root, "", ResolverConfig{})
	require.NoError(t, err)
	assert.Equal(t, SourceKindLocal, payload.Kind)
	require.Len(t, payload.Bundle.Assets, 1)
	assert.Equal(t, "index.html", payload.Bundle.Assets[0].Path)
	require.Len(t, payload.Bundle.Migrations, 1)
	assert.Equal(t, "001_init.sql", payload.Bundle.Migrations[0].Path)
}

func TestReadSourceHTTPBundleURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bundle.hext", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleBundle))
	}))
	defer server.Close()

	payload, err := ReadSource(t.Context(), server.URL+"/bundle.hext", "", ResolverConfig{
		HTTPClient: server.Client,
	})
	require.NoError(t, err)
	assert.Equal(t, SourceKindHTTP, payload.Kind)
	assert.Equal(t, "ats", payload.Bundle.Manifest["slug"])
}

func TestReadSourceOCIReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/movebigrocks/mbr-ext-ats/manifests/1.0.0":
			assert.Contains(t, r.Header.Get("Accept"), "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			_, _ = w.Write([]byte(`{
			  "schemaVersion": 2,
			  "mediaType": "application/vnd.oci.artifact.manifest.v1+json",
			  "blobs": [{
			    "mediaType": "application/vnd.mbr.extension.bundle.v1+json",
			    "digest": "sha256:bundle",
			    "size": 256
			  }]
			}`))
		case "/v2/movebigrocks/mbr-ext-ats/blobs/sha256:bundle":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(sampleBundle))
		default:
			t.Fatalf("unexpected registry path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	ref := strings.Replace(server.URL, "http://", "oci+http://", 1) + "/movebigrocks/mbr-ext-ats:1.0.0"
	payload, err := ReadSource(t.Context(), ref, "", ResolverConfig{
		HTTPClient: server.Client,
	})
	require.NoError(t, err)
	assert.Equal(t, SourceKindOCI, payload.Kind)
	assert.Equal(t, "ats", payload.Bundle.Manifest["slug"])
}

func TestReadSourceMarketplaceAlias(t *testing.T) {
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/downloads/ats-1.0.0.hext", r.URL.Path)
		assert.Equal(t, "granted", r.Header.Get("X-License-Grant"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleBundle))
	}))
	defer downloadServer.Close()

	resolveServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/resolve", r.URL.Path)
		assert.Equal(t, "demandops/ats@1.0.0", r.URL.Query().Get("ref"))
		assert.Equal(t, "Bearer lic_123", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "bundleURL": "` + downloadServer.URL + `/downloads/ats-1.0.0.hext",
		  "headers": {
		    "X-License-Grant": "granted"
		  }
		}`))
	}))
	defer resolveServer.Close()

	payload, err := ReadSource(t.Context(), "demandops/ats@1.0.0", "lic_123", ResolverConfig{
		MarketplaceURL: resolveServer.URL,
		HTTPClient:     resolveServer.Client,
	})
	require.NoError(t, err)
	assert.Equal(t, SourceKindMarketplace, payload.Kind)
	assert.Equal(t, "ats", payload.Bundle.Manifest["slug"])
}
