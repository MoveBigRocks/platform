package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestRunExtensionsShowJSONIncludesProofSurfaces(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIExtension") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["id"]; got != "ext_123" {
			t.Fatalf("expected extension id ext_123, got %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"extension": map[string]any{
					"id":                 "ext_123",
					"workspaceID":        "ws_123",
					"slug":               "sales-pipeline",
					"name":               "Sales Pipeline",
					"publisher":          "Move Big Rocks",
					"version":            "1.0.0",
					"kind":               "product",
					"scope":              "workspace",
					"risk":               "standard",
					"runtimeClass":       "service_backed",
					"storageClass":       "owned_schema",
					"status":             "active",
					"validationStatus":   "valid",
					"healthStatus":       "healthy",
					"bundleSHA256":       "abc123",
					"bundleSize":         2048,
					"installedAt":        "2026-03-27T10:00:00Z",
					"runtimeDiagnostics": map[string]any{"bootstrapStatus": "ready", "endpoints": []any{}, "eventConsumers": []any{}, "scheduledJobs": []any{}},
					"assets":             []any{},
					"resolvedAdminNavigation": []map[string]any{
						{
							"extensionID":   "ext_123",
							"extensionSlug": "sales-pipeline",
							"section":       "Revenue",
							"title":         "Pipeline",
							"icon":          "briefcase-business",
							"href":          "/extensions/sales-pipeline",
							"activePage":    "sales-pipeline",
						},
					},
					"resolvedDashboardWidgets": []map[string]any{
						{
							"extensionID":   "ext_123",
							"extensionSlug": "sales-pipeline",
							"title":         "Pipeline Snapshot",
							"description":   "Open the revenue board",
							"icon":          "briefcase-business",
							"href":          "/extensions/sales-pipeline",
						},
					},
					"seededResources": map[string]any{
						"queues": []map[string]any{
							{
								"slug":        "sales-follow-up",
								"resourceID":  "queue_1",
								"exists":      true,
								"matchesSeed": true,
								"problems":    []string{},
								"expected":    map[string]any{"slug": "sales-follow-up"},
								"actual":      map[string]any{"slug": "sales-follow-up"},
							},
						},
						"forms": []map[string]any{
							{
								"slug":        "sales-deal-intake",
								"resourceID":  "form_1",
								"exists":      true,
								"matchesSeed": true,
								"problems":    []string{},
								"expected":    map[string]any{"slug": "sales-deal-intake"},
								"actual":      map[string]any{"slug": "sales-deal-intake"},
							},
						},
						"automationRules": []map[string]any{
							{
								"key":         "ext.movebigrocks.sales_pipeline.intake",
								"resourceID":  "rule_1",
								"exists":      true,
								"matchesSeed": true,
								"problems":    []string{},
								"expected":    map[string]any{"key": "ext.movebigrocks.sales_pipeline.intake"},
								"actual":      map[string]any{"key": "ext.movebigrocks.sales_pipeline.intake"},
							},
						},
					},
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "show",
		"--id", "ext_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload extensionDetailOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload.ResolvedAdminNavigation) != 1 || payload.ResolvedAdminNavigation[0].Href != "/extensions/sales-pipeline" {
		t.Fatalf("unexpected resolved admin navigation: %#v", payload.ResolvedAdminNavigation)
	}
	if len(payload.ResolvedDashboardWidgets) != 1 || payload.ResolvedDashboardWidgets[0].Href != "/extensions/sales-pipeline" {
		t.Fatalf("unexpected resolved dashboard widgets: %#v", payload.ResolvedDashboardWidgets)
	}
	if len(payload.SeededResources.Queues) != 1 || !payload.SeededResources.Queues[0].MatchesSeed {
		t.Fatalf("unexpected seeded queue report: %#v", payload.SeededResources.Queues)
	}
	if len(payload.SeededResources.Forms) != 1 || !payload.SeededResources.Forms[0].MatchesSeed {
		t.Fatalf("unexpected seeded form report: %#v", payload.SeededResources.Forms)
	}
	if len(payload.SeededResources.AutomationRules) != 1 || !payload.SeededResources.AutomationRules[0].MatchesSeed {
		t.Fatalf("unexpected seeded automation report: %#v", payload.SeededResources.AutomationRules)
	}
}

func TestRunExtensionsNavJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIWorkspaceExtensionAdminNavigation") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["workspaceID"]; got != "ws_123" {
			t.Fatalf("expected workspace id ws_123, got %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"workspaceExtensionAdminNavigation": []map[string]any{
					{
						"extensionID":   "ext_123",
						"extensionSlug": "web-analytics",
						"section":       "Analytics",
						"title":         "Web Analytics",
						"icon":          "chart-column",
						"href":          "/extensions/web-analytics",
						"activePage":    "analytics",
					},
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "nav",
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []resolvedExtensionAdminNavigationItemOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].Href != "/extensions/web-analytics" {
		t.Fatalf("unexpected nav payload: %#v", payload)
	}
}

func TestRunExtensionsWidgetsInstanceJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIInstanceExtensionDashboardWidgets") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"instanceExtensionDashboardWidgets": []map[string]any{
					{
						"extensionID":   "ext_inst_1",
						"extensionSlug": "enterprise-access",
						"title":         "Access Overview",
						"description":   "Review access posture",
						"icon":          "shield-check",
						"href":          "/extensions/enterprise-access",
					},
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "widgets",
		"--instance",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []resolvedExtensionDashboardWidgetOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].Href != "/extensions/enterprise-access" {
		t.Fatalf("unexpected widget payload: %#v", payload)
	}
}
