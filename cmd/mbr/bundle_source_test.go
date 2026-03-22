package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleExtensionBundle = `{
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

func TestReadBundleSourceOCIReference(t *testing.T) {
	previousHTTPClient := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/v2/movebigrocks/mbr-ext-ats/manifests/1.0.0":
					assert.Contains(t, r.Header.Get("Accept"), "application/vnd.oci.image.manifest.v1+json")
					return &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type": []string{"application/vnd.oci.image.manifest.v1+json"},
						},
						Body: io.NopCloser(strings.NewReader(`{
							"schemaVersion": 2,
							"mediaType": "application/vnd.oci.artifact.manifest.v1+json",
							"blobs": [
								{
									"mediaType": "application/vnd.mbr.extension.bundle.v1+json",
									"digest": "sha256:bundle",
									"size": 256
								}
							]
						}`)),
					}, nil
				case "/v2/movebigrocks/mbr-ext-ats/blobs/sha256:bundle":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type": []string{"application/json"},
						},
						Body: io.NopCloser(strings.NewReader(sampleExtensionBundle)),
					}, nil
				default:
					t.Fatalf("unexpected registry path %q", r.URL.Path)
					return nil, nil
				}
			}),
		}
	}
	defer func() {
		newHTTPClient = previousHTTPClient
	}()

	ref := "oci+http://registry.test/movebigrocks/mbr-ext-ats:1.0.0"
	bundle, err := readBundleSource(t.Context(), ref, "")
	require.NoError(t, err)
	assert.Equal(t, "ats", bundle.Manifest["slug"])
}

func TestReadBundleSourceMarketplaceAlias(t *testing.T) {
	previousHTTPClient := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch {
				case r.URL.Host == "marketplace.test":
					assert.Equal(t, "/resolve", r.URL.Path)
					assert.Equal(t, "demandops/ats@1.0.0", r.URL.Query().Get("ref"))
					assert.Equal(t, "Bearer lic_123", r.Header.Get("Authorization"))
					return &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type": []string{"application/json"},
						},
						Body: io.NopCloser(strings.NewReader(`{
							"bundleURL": "https://downloads.test/downloads/ats-1.0.0.hext",
							"headers": {
								"X-License-Grant": "granted"
							}
						}`)),
					}, nil
				case r.URL.Host == "downloads.test":
					assert.Equal(t, "/downloads/ats-1.0.0.hext", r.URL.Path)
					assert.Equal(t, "granted", r.Header.Get("X-License-Grant"))
					return &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type": []string{"application/json"},
						},
						Body: io.NopCloser(strings.NewReader(sampleExtensionBundle)),
					}, nil
				default:
					t.Fatalf("unexpected request URL %q", r.URL.String())
					return nil, nil
				}
			}),
		}
	}
	defer func() {
		newHTTPClient = previousHTTPClient
	}()

	t.Setenv(envMarketplaceURL, "https://marketplace.test")

	bundle, err := readBundleSource(t.Context(), "demandops/ats@1.0.0", "lic_123")
	require.NoError(t, err)
	assert.Equal(t, "ats", bundle.Manifest["slug"])
}

func TestReadBundleSourcePrefersExistingLocalPathOverMarketplaceAlias(t *testing.T) {
	root := t.TempDir()
	source := root + "/demandops/ats@1.0.0"
	require.NoError(t, os.MkdirAll(source, 0o755))
	require.NoError(t, os.WriteFile(source+"/manifest.json", []byte(`{
		"slug": "ats",
		"name": "Applicant Tracking",
		"version": "1.0.0",
		"publisher": "DemandOps",
		"kind": "product",
		"scope": "workspace",
		"risk": "standard"
	}`), 0o600))

	bundle, err := readBundleSource(t.Context(), source, "")
	require.NoError(t, err)
	assert.Equal(t, "ats", bundle.Manifest["slug"])
}

func TestRunExtensionsInstallMarketplaceAliasJSON(t *testing.T) {
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	previousHTTPClient := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch {
				case r.URL.Host == "marketplace.test":
					assert.Equal(t, "demandops/ats@1.0.0", r.URL.Query().Get("ref"))
					assert.Equal(t, "Bearer lic_123", r.Header.Get("Authorization"))
					return &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type": []string{"application/json"},
						},
						Body: io.NopCloser(strings.NewReader(`{
							"bundleURL": "https://downloads.test/ats.hext",
							"headers": {
								"X-License-Grant": "granted"
							}
						}`)),
					}, nil
				case r.URL.Host == "downloads.test":
					assert.Equal(t, "granted", r.Header.Get("X-License-Grant"))
					return &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type": []string{"application/json"},
						},
						Body: io.NopCloser(strings.NewReader(sampleExtensionBundle)),
					}, nil
				default:
					t.Fatalf("unexpected request URL %q", r.URL.String())
					return nil, nil
				}
			}),
		}
	}
	defer func() {
		newHTTPClient = previousHTTPClient
	}()
	t.Setenv(envMarketplaceURL, "https://marketplace.test")

	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "mutation CLIInstallExtension") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		input, ok := req.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("expected input map, got %#v", req.Variables["input"])
		}
		if got := input["workspaceID"]; got != "ws_123" {
			t.Fatalf("expected workspace ws_123, got %#v", got)
		}
		if got := input["licenseToken"]; got != "lic_123" {
			t.Fatalf("expected license token lic_123, got %#v", got)
		}
		if got, ok := input["bundleBase64"].(string); !ok || strings.TrimSpace(got) == "" {
			t.Fatalf("expected non-empty bundleBase64, got %#v", input["bundleBase64"])
		}
		manifest, ok := input["manifest"].(map[string]any)
		if !ok {
			t.Fatalf("expected manifest map, got %#v", input["manifest"])
		}
		if got := manifest["slug"]; got != "ats" {
			t.Fatalf("expected manifest slug ats, got %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"installExtension": map[string]any{
					"id":                "ext_123",
					"workspaceID":       "ws_123",
					"slug":              "ats",
					"name":              "Applicant Tracking",
					"publisher":         "DemandOps",
					"version":           "1.0.0",
					"kind":              "product",
					"scope":             "workspace",
					"risk":              "standard",
					"status":            "installed",
					"validationStatus":  "valid",
					"validationMessage": "manifest and installed assets validated",
					"healthStatus":      "pending",
					"healthMessage":     "extension installed",
				},
			},
		}
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "install", "demandops/ats@1.0.0",
		"--workspace", "ws_123",
		"--license-token", "lic_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload struct {
		Extension extensionOutput `json:"extension"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	assert.Equal(t, "ext_123", payload.Extension.ID)
	assert.Equal(t, "ats", payload.Extension.Slug)
	assert.Equal(t, "1.0.0", payload.Extension.Version)
}
