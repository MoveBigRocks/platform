package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrepareExtensionSourceReferencePacksPassLintContract(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		root string
	}{
		{name: "sdk-template", root: extensionSDKRoot(t)},
		{name: "ats", root: firstPartyBundleRoot(t, "ats")},
		{name: "community-feature-requests", root: firstPartyBundleRoot(t, "community-feature-requests")},
		{name: "error-tracking", root: firstPartyBundleRoot(t, "error-tracking")},
		{name: "sales-pipeline", root: firstPartyBundleRoot(t, "sales-pipeline")},
		{name: "web-analytics", root: firstPartyBundleRoot(t, "web-analytics")},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			prepared, err := prepareExtensionSource(t.Context(), tc.root, "")
			if err != nil {
				t.Fatalf("prepareExtensionSource(%s) returned error: %v", tc.name, err)
			}
			if !prepared.lint.ManifestValid {
				t.Fatalf("expected manifest to be valid for %s, got %q", tc.name, prepared.lint.ManifestMessage)
			}
			if !prepared.lint.ContractValid {
				t.Fatalf("expected contract to be valid for %s, got %v", tc.name, prepared.lint.Problems)
			}
		})
	}
}

func TestRunExtensionsLintWriteContractJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestBundleSource(t, root, "custom-ops-pack", "Custom Ops Pack")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "lint",
		root,
		"--write-contract",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload extensionLintOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode lint output: %v", err)
	}
	if !payload.ManifestValid {
		t.Fatalf("expected manifest to be valid, got %q", payload.ManifestMessage)
	}
	if !payload.ContractValid {
		t.Fatalf("expected contract to be valid, got %v", payload.Problems)
	}

	data, err := os.ReadFile(filepath.Join(root, defaultExtensionContractFile))
	if err != nil {
		t.Fatalf("read generated contract: %v", err)
	}
	var contract extensionContractFile
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatalf("decode generated contract: %v", err)
	}
	normalizeExtensionContract(&contract)
	if problems := compareExtensionContract(contract, payload.Derived); len(problems) > 0 {
		t.Fatalf("generated contract mismatch: %v", problems)
	}
}

func TestRunExtensionsVerifyJSON(t *testing.T) {
	root := t.TempDir()
	writeTestBundleSource(t, root, "custom-ops-pack", "Custom Ops Pack")
	writeTestBundleContract(t, root)

	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		switch {
		case strings.Contains(req.Query, "query CLIDeployExtensions"):
			if got := req.Variables["workspaceID"]; got != "ws_123" {
				t.Fatalf("expected workspace id ws_123, got %#v", got)
			}
			return map[string]any{
				"data": map[string]any{
					"extensions": []map[string]any{},
				},
			}
		case strings.Contains(req.Query, "mutation CLIInstallExtension"):
			return map[string]any{
				"data": map[string]any{
					"installExtension": testExtensionOutput("ext_123", "custom-ops-pack", "installed", "pending", "healthy"),
				},
			}
		case strings.Contains(req.Query, "mutation CLIValidateExtension"):
			return map[string]any{
				"data": map[string]any{
					"validateExtension": testExtensionOutput("ext_123", "custom-ops-pack", "installed", "valid", "healthy"),
				},
			}
		case strings.Contains(req.Query, "mutation CLIActivateExtension"):
			return map[string]any{
				"data": map[string]any{
					"activateExtension": testExtensionOutput("ext_123", "custom-ops-pack", "active", "valid", "healthy"),
				},
			}
		case strings.Contains(req.Query, "mutation CLICheckExtensionHealth"):
			return map[string]any{
				"data": map[string]any{
					"checkExtensionHealth": testExtensionOutput("ext_123", "custom-ops-pack", "active", "valid", "healthy"),
				},
			}
		case strings.Contains(req.Query, "query CLIExtension("):
			return map[string]any{
				"data": map[string]any{
					"extension": testExtensionDetail("ext_123", "custom-ops-pack", "Custom Ops Pack"),
				},
			}
		case strings.Contains(req.Query, "query CLIWorkspaceExtensionAdminNavigation"):
			return map[string]any{
				"data": map[string]any{
					"workspaceExtensionAdminNavigation": []map[string]any{
						{
							"extensionID":   "ext_123",
							"extensionSlug": "custom-ops-pack",
							"section":       "Extensions",
							"title":         "Custom Ops Pack",
							"icon":          "blocks",
							"href":          "/extensions/custom-ops-pack",
							"activePage":    "custom-ops-pack",
						},
					},
				},
			}
		case strings.Contains(req.Query, "query CLIWorkspaceExtensionDashboardWidgets"):
			return map[string]any{
				"data": map[string]any{
					"workspaceExtensionDashboardWidgets": []map[string]any{},
				},
			}
		case strings.Contains(req.Query, "query CLIInstanceExtensionAdminNavigation"):
			return map[string]any{
				"data": map[string]any{
					"instanceExtensionAdminNavigation": []map[string]any{
						{
							"extensionID":   "ext_123",
							"extensionSlug": "custom-ops-pack",
							"workspaceID":   "ws_123",
							"section":       "Extensions",
							"title":         "Custom Ops Pack",
							"icon":          "blocks",
							"href":          "/extensions/custom-ops-pack?workspace=ws_123",
							"activePage":    "custom-ops-pack",
						},
					},
				},
			}
		case strings.Contains(req.Query, "query CLIInstanceExtensionDashboardWidgets"):
			return map[string]any{
				"data": map[string]any{
					"instanceExtensionDashboardWidgets": []map[string]any{},
				},
			}
		default:
			t.Fatalf("unexpected query: %s", req.Query)
			return nil
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "verify",
		root,
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload extensionVerifyOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode verify output: %v", err)
	}
	if payload.Detail.Slug != "custom-ops-pack" {
		t.Fatalf("expected verified slug custom-ops-pack, got %q", payload.Detail.Slug)
	}
	if len(payload.WorkspaceResolvedAdminNavigation) != 1 {
		t.Fatalf("expected 1 resolved admin navigation item, got %#v", payload.WorkspaceResolvedAdminNavigation)
	}
	if len(payload.InstanceResolvedAdminNavigation) != 1 {
		t.Fatalf("expected 1 instance admin navigation item, got %#v", payload.InstanceResolvedAdminNavigation)
	}
	if payload.Detail.HealthStatus != "healthy" {
		t.Fatalf("expected healthy detail, got %q", payload.Detail.HealthStatus)
	}
}

func extensionSDKRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve extension SDK root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "extension-sdk"))
}

func writeTestBundleSource(t *testing.T, root, slug, name string) {
	t.Helper()

	manifest := `{
  "schemaVersion": 1,
  "slug": "` + slug + `",
  "name": "` + name + `",
  "version": "0.1.0",
  "publisher": "DemandOps",
  "kind": "product",
  "scope": "workspace",
  "risk": "standard",
  "runtimeClass": "bundle",
  "description": "A contract-test extension.",
  "workspacePlan": {
    "mode": "attach_to_existing_workspace"
  },
  "permissions": [
    "case:read"
  ],
  "adminNavigation": [
    {
      "name": "custom-dashboard",
      "section": "Extensions",
      "title": "` + name + `",
      "icon": "blocks",
      "endpoint": "custom-admin-dashboard",
      "activePage": "` + slug + `"
    }
  ],
  "endpoints": [
    {
      "name": "custom-admin-dashboard",
      "class": "admin_page",
      "mountPath": "/extensions/` + slug + `",
      "methods": ["GET", "HEAD"],
      "auth": "session",
      "workspaceBinding": "workspace_from_session",
      "assetPath": "admin/dashboard.html"
    },
    {
      "name": "custom-public-home",
      "class": "public_page",
      "mountPath": "/` + slug + `",
      "methods": ["GET", "HEAD"],
      "auth": "public",
      "assetPath": "public/index.html"
    }
  ],
  "commands": [
    {
      "name": "` + slug + `.open-dashboard",
      "description": "Open the admin dashboard."
    }
  ],
  "agentSkills": [
    {
      "name": "operate-` + slug + `",
      "description": "Validate and monitor the pack.",
      "assetPath": "agent-skills/operate-pack.md"
    }
  ],
  "customizableAssets": [
    "admin/dashboard.html",
    "public/index.html"
  ]
}`
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	writeTestAsset(t, filepath.Join(root, "assets", "admin", "dashboard.html"), "<html><body>dashboard</body></html>")
	writeTestAsset(t, filepath.Join(root, "assets", "public", "index.html"), "<html><body>public</body></html>")
	writeTestAsset(t, filepath.Join(root, "assets", "agent-skills", "operate-pack.md"), "# Operate\n")
}

func writeTestBundleContract(t *testing.T, root string) {
	t.Helper()

	payload, err := readBundleDirectoryPayload(root)
	if err != nil {
		t.Fatalf("read bundle directory: %v", err)
	}
	manifest, err := decodeBundleManifest(payload.Bundle.Manifest)
	if err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if err := writeExtensionContract(filepath.Join(root, defaultExtensionContractFile), deriveExtensionContractFromManifest(manifest)); err != nil {
		t.Fatalf("write contract: %v", err)
	}
}

func writeTestAsset(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func testExtensionOutput(id, slug, status, validationStatus, healthStatus string) map[string]any {
	return map[string]any{
		"id":               id,
		"workspaceID":      "ws_123",
		"slug":             slug,
		"name":             "Custom Ops Pack",
		"publisher":        "DemandOps",
		"version":          "0.1.0",
		"kind":             "product",
		"scope":            "workspace",
		"risk":             "standard",
		"status":           status,
		"validationStatus": validationStatus,
		"healthStatus":     healthStatus,
		"healthMessage":    "",
	}
}

func testExtensionDetail(id, slug, name string) map[string]any {
	return map[string]any{
		"id":               id,
		"workspaceID":      "ws_123",
		"slug":             slug,
		"name":             name,
		"publisher":        "DemandOps",
		"version":          "0.1.0",
		"description":      "A contract-test extension.",
		"kind":             "product",
		"scope":            "workspace",
		"risk":             "standard",
		"runtimeClass":     "bundle",
		"storageClass":     "none",
		"permissions":      []string{"case:read"},
		"artifactSurfaces": []map[string]any{},
		"publicRoutes":     []map[string]any{},
		"adminRoutes":      []map[string]any{},
		"endpoints": []map[string]any{
			{
				"name":             "custom-admin-dashboard",
				"class":            "admin_page",
				"mountPath":        "/extensions/custom-ops-pack",
				"methods":          []string{"GET", "HEAD"},
				"auth":             "session",
				"workspaceBinding": "workspace_from_session",
				"assetPath":        "admin/dashboard.html",
			},
			{
				"name":      "custom-public-home",
				"class":     "public_page",
				"mountPath": "/custom-ops-pack",
				"methods":   []string{"GET", "HEAD"},
				"auth":      "public",
				"assetPath": "public/index.html",
			},
		},
		"adminNavigation": []map[string]any{
			{
				"name":       "custom-dashboard",
				"section":    "Extensions",
				"title":      "Custom Ops Pack",
				"icon":       "blocks",
				"endpoint":   "custom-admin-dashboard",
				"activePage": "custom-ops-pack",
			},
		},
		"dashboardWidgets": []map[string]any{},
		"resolvedAdminNavigation": []map[string]any{
			{
				"extensionID":   id,
				"extensionSlug": slug,
				"section":       "Extensions",
				"title":         "Custom Ops Pack",
				"icon":          "blocks",
				"href":          "/extensions/custom-ops-pack",
				"activePage":    "custom-ops-pack",
			},
		},
		"resolvedDashboardWidgets": []map[string]any{},
		"seededResources": map[string]any{
			"queues":          []map[string]any{},
			"forms":           []map[string]any{},
			"automationRules": []map[string]any{},
		},
		"events": map[string]any{
			"publishes":  []map[string]any{},
			"subscribes": []string{},
		},
		"eventConsumers": []map[string]any{},
		"scheduledJobs":  []map[string]any{},
		"commands": []map[string]any{
			{
				"name":        "custom-ops-pack.open-dashboard",
				"description": "Open the admin dashboard.",
			},
		},
		"agentSkills": []map[string]any{
			{
				"name":        "operate-custom-ops-pack",
				"description": "Validate and monitor the pack.",
				"assetPath":   "agent-skills/operate-pack.md",
			},
		},
		"customizableAssets": []string{"admin/dashboard.html", "public/index.html"},
		"status":             "active",
		"validationStatus":   "valid",
		"validationMessage":  "",
		"healthStatus":       "healthy",
		"healthMessage":      "",
		"bundleSHA256":       "abc123",
		"bundleSize":         2048,
		"installedAt":        "2026-03-27T10:00:00Z",
		"runtimeDiagnostics": map[string]any{
			"bootstrapStatus": "ready",
			"endpoints":       []map[string]any{},
			"eventConsumers":  []map[string]any{},
			"scheduledJobs":   []map[string]any{},
		},
		"assets": []map[string]any{},
	}
}
