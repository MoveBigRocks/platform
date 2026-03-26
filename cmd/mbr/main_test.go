package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/cliapi"
	"github.com/movebigrocks/platform/internal/clispec"
	knowledgedomain "github.com/movebigrocks/platform/internal/knowledge/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("MBR_DISABLE_KEYCHAIN", "1")
	os.Exit(m.Run())
}

func TestParseJSONObjectRejectsNonObject(t *testing.T) {
	t.Parallel()

	if _, err := parseJSONObject([]byte(`["not","an","object"]`)); err == nil {
		t.Fatalf("expected parseJSONObject to reject non-object json")
	}
}

func TestReadBundleFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.json")
	content := `{
	  "manifest": {
	    "slug": "ats",
	    "name": "ATS",
	    "version": "1.0.0",
	    "publisher": "DemandOps"
	  },
	  "assets": [
	    {
	      "path": "templates/careers/index.html",
	      "content": "<html></html>",
	      "contentType": "text/html"
	    }
	  ]
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write bundle file: %v", err)
	}

	bundle, err := readBundleFile(path)
	if err != nil {
		t.Fatalf("readBundleFile returned error: %v", err)
	}
	if bundle.Manifest["slug"] != "ats" {
		t.Fatalf("expected slug to be ats, got %#v", bundle.Manifest["slug"])
	}
	if len(bundle.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(bundle.Assets))
	}
}

func TestReadBundleDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	manifest := `{
	  "slug": "ats",
	  "name": "Applicant Tracking",
	  "version": "1.0.0",
	  "publisher": "DemandOps",
	  "kind": "product",
	  "scope": "workspace",
	  "risk": "standard"
	}`
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	assetsDir := filepath.Join(root, "assets", "templates", "careers")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "index.html"), []byte("<html><body>Careers</body></html>"), 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	migrationsDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatalf("mkdir migrations: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "000001_init.up.sql"), []byte("create table example ();"), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	bundle, err := readBundleFile(root)
	if err != nil {
		t.Fatalf("readBundleFile returned error: %v", err)
	}
	if bundle.Manifest["slug"] != "ats" {
		t.Fatalf("expected slug to be ats, got %#v", bundle.Manifest["slug"])
	}
	if len(bundle.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(bundle.Assets))
	}
	if bundle.Assets[0].Path != "templates/careers/index.html" {
		t.Fatalf("unexpected asset path %#v", bundle.Assets[0].Path)
	}
	if bundle.Assets[0].ContentType != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type %#v", bundle.Assets[0].ContentType)
	}
	if len(bundle.Migrations) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(bundle.Migrations))
	}
	if bundle.Migrations[0].Path != "000001_init.up.sql" {
		t.Fatalf("unexpected migration path %#v", bundle.Migrations[0].Path)
	}
	if bundle.Migrations[0].Content != "create table example ();" {
		t.Fatalf("unexpected migration content %#v", bundle.Migrations[0].Content)
	}
}

func TestFirstPartyReferenceBundlesValidateAgainstCurrentContract(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		expectMigrations bool
	}{
		{name: "ats", expectMigrations: false},
		{name: "web-analytics", expectMigrations: true},
		{name: "error-tracking", expectMigrations: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := firstPartyBundleRoot(t, tc.name)
			bundle, err := readBundleFile(root)
			if err != nil {
				t.Fatalf("readBundleFile(%s) returned error: %v", tc.name, err)
			}

			manifestBytes, err := json.Marshal(bundle.Manifest)
			if err != nil {
				t.Fatalf("marshal manifest: %v", err)
			}

			var manifest platformdomain.ExtensionManifest
			if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
				t.Fatalf("decode manifest: %v", err)
			}
			if err := manifest.Validate(); err != nil {
				t.Fatalf("manifest.Validate() returned error: %v", err)
			}

			if tc.expectMigrations && len(bundle.Migrations) == 0 {
				t.Fatalf("expected %s to ship migrations", tc.name)
			}
			if !tc.expectMigrations && len(bundle.Migrations) != 0 {
				t.Fatalf("expected %s to remain bundle-only, got %d migrations", tc.name, len(bundle.Migrations))
			}
			if tc.expectMigrations {
				highest := ""
				for _, migration := range bundle.Migrations {
					version := bundleMigrationVersion(migration.Path)
					if version > highest {
						highest = version
					}
				}
				if highest != manifest.Schema.TargetVersion {
					t.Fatalf("expected highest migration version %s to match target %s", highest, manifest.Schema.TargetVersion)
				}
			}
		})
	}
}

func firstPartyBundleRoot(t *testing.T, slug string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve first-party bundle root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "extensions", slug))
}

func bundleMigrationVersion(path string) string {
	base := filepath.Base(path)
	parts := strings.SplitN(base, "_", 2)
	if len(parts) == 0 {
		return base
	}
	return parts[0]
}

func TestReadBundleURL(t *testing.T) {
	t.Parallel()

	previous := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(strings.NewReader(`{"manifest":{"slug":"ats","name":"ATS","version":"1.0.0","publisher":"DemandOps"},"assets":[]}`)),
				}, nil
			}),
		}
	}
	defer func() {
		newHTTPClient = previous
	}()

	bundle, err := readBundleFile("https://example.test/ats.hext")
	if err != nil {
		t.Fatalf("readBundleFile returned error: %v", err)
	}
	if bundle.Manifest["slug"] != "ats" {
		t.Fatalf("expected slug to be ats, got %#v", bundle.Manifest["slug"])
	}
}

func TestRunSpecExportJSON(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{"spec", "export", "--json"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var spec clispec.Spec
	if err := json.Unmarshal(stdout.Bytes(), &spec); err != nil {
		t.Fatalf("decode spec: %v", err)
	}
	if spec.SchemaVersion != "mbr-cli-contract-v1" {
		t.Fatalf("unexpected schema version %q", spec.SchemaVersion)
	}
	if len(spec.Commands) == 0 {
		t.Fatalf("expected commands in exported spec")
	}

	var foundSpecExport bool
	var foundInstall bool
	var foundDeploy bool
	var foundUninstall bool
	var foundContextView bool
	var foundCatalogList bool
	var foundFormSpecsList bool
	var foundFormSpecsCreate bool
	var foundFormSpecsUpdate bool
	var foundFormSubmissionsCreate bool
	var foundKnowledgeList bool
	var foundKnowledgeImport bool
	var foundKnowledgeSync bool
	var foundKnowledgeCheckout bool
	var foundKnowledgeStatus bool
	var foundKnowledgePull bool
	var foundKnowledgePush bool
	var foundKnowledgeReviewQueue bool
	var foundTeamsList bool
	var foundAgentsList bool
	var foundAgentsCreate bool
	var foundAgentTokensCreate bool
	var foundAgentMembershipsGrant bool
	var foundQueuesList bool
	var foundQueuesItems bool
	var foundCasesHandoff bool
	var foundConversationsHandoff bool
	var foundConversationsEscalate bool
	for _, command := range spec.Commands {
		switch strings.Join(command.Path, " ") {
		case "context view":
			foundContextView = true
		case "spec export":
			foundSpecExport = true
		case "catalog list":
			foundCatalogList = true
		case "forms specs list":
			foundFormSpecsList = true
		case "forms specs create":
			foundFormSpecsCreate = true
		case "forms specs update":
			foundFormSpecsUpdate = true
		case "forms submissions create":
			foundFormSubmissionsCreate = true
		case "knowledge list":
			foundKnowledgeList = true
		case "knowledge import":
			foundKnowledgeImport = true
		case "knowledge checkout":
			foundKnowledgeCheckout = true
		case "knowledge review-queue":
			foundKnowledgeReviewQueue = true
		case "knowledge sync":
			foundKnowledgeSync = true
		case "knowledge status":
			foundKnowledgeStatus = true
		case "knowledge pull":
			foundKnowledgePull = true
		case "knowledge push":
			foundKnowledgePush = true
		case "teams list":
			foundTeamsList = true
		case "agents list":
			foundAgentsList = true
		case "agents create":
			foundAgentsCreate = true
		case "agents tokens create":
			foundAgentTokensCreate = true
		case "agents memberships grant":
			foundAgentMembershipsGrant = true
		case "queues list":
			foundQueuesList = true
		case "queues items":
			foundQueuesItems = true
		case "cases handoff":
			foundCasesHandoff = true
		case "conversations handoff":
			foundConversationsHandoff = true
		case "conversations escalate":
			foundConversationsEscalate = true
		case "extensions install":
			foundInstall = true
			if command.AuthMode != clispec.AuthModeBearerOrSession {
				t.Fatalf("expected extensions install auth mode %q, got %q", clispec.AuthModeBearerOrSession, command.AuthMode)
			}
			required := false
			for _, flag := range command.Flags {
				if flag.Name == "--license-token" && flag.Required {
					required = true
				}
			}
			if required {
				t.Fatalf("expected extensions install to advertise --license-token as optional")
			}
		case "extensions deploy":
			foundDeploy = true
		case "extensions uninstall":
			foundUninstall = true
			flags := map[string]bool{}
			for _, flag := range command.Flags {
				flags[flag.Name] = true
			}
			for _, expected := range []string{"--deactivate", "--reason", "--export-out", "--confirm-no-export", "--dry-run"} {
				if !flags[expected] {
					t.Fatalf("expected extensions uninstall to advertise %s", expected)
				}
			}
		}
	}
	if !foundSpecExport {
		t.Fatalf("expected spec export command in contract")
	}
	if !foundContextView {
		t.Fatalf("expected context view command in contract")
	}
	if !foundTeamsList {
		t.Fatalf("expected teams list command in contract")
	}
	if !foundAgentsList {
		t.Fatalf("expected agents list command in contract")
	}
	if !foundAgentsCreate {
		t.Fatalf("expected agents create command in contract")
	}
	if !foundAgentTokensCreate {
		t.Fatalf("expected agents tokens create command in contract")
	}
	if !foundAgentMembershipsGrant {
		t.Fatalf("expected agents memberships grant command in contract")
	}
	if !foundCatalogList {
		t.Fatalf("expected catalog list command in contract")
	}
	if !foundFormSpecsList {
		t.Fatalf("expected forms specs list command in contract")
	}
	if !foundFormSpecsCreate {
		t.Fatalf("expected forms specs create command in contract")
	}
	if !foundFormSpecsUpdate {
		t.Fatalf("expected forms specs update command in contract")
	}
	if !foundFormSubmissionsCreate {
		t.Fatalf("expected forms submissions create command in contract")
	}
	if !foundKnowledgeList {
		t.Fatalf("expected knowledge list command in contract")
	}
	if !foundKnowledgeImport {
		t.Fatalf("expected knowledge import command in contract")
	}
	if !foundKnowledgeCheckout {
		t.Fatalf("expected knowledge checkout command in contract")
	}
	if !foundKnowledgeReviewQueue {
		t.Fatalf("expected knowledge review-queue command in contract")
	}
	if !foundKnowledgeSync {
		t.Fatalf("expected knowledge sync command in contract")
	}
	if !foundKnowledgeStatus {
		t.Fatalf("expected knowledge status command in contract")
	}
	if !foundKnowledgePull {
		t.Fatalf("expected knowledge pull command in contract")
	}
	if !foundKnowledgePush {
		t.Fatalf("expected knowledge push command in contract")
	}
	if !foundQueuesList {
		t.Fatalf("expected queues list command in contract")
	}
	if !foundQueuesItems {
		t.Fatalf("expected queues items command in contract")
	}
	if !foundCasesHandoff {
		t.Fatalf("expected cases handoff command in contract")
	}
	if !foundConversationsHandoff {
		t.Fatalf("expected conversations handoff command in contract")
	}
	if !foundConversationsEscalate {
		t.Fatalf("expected conversations escalate command in contract")
	}
	if !foundInstall {
		t.Fatalf("expected extensions install command in contract")
	}
	if !foundDeploy {
		t.Fatalf("expected extensions deploy command in contract")
	}
	if !foundUninstall {
		t.Fatalf("expected extensions uninstall command in contract")
	}
}

func TestHelpIncludesSpecExportAndSessionNotes(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{"help"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "mbr spec export [--json]") {
		t.Fatalf("expected spec export usage in help, got %q", output)
	}
	if !strings.Contains(output, "Session-backed auth is required") {
		t.Fatalf("expected session-backed auth note in help, got %q", output)
	}
	if !strings.Contains(output, "mbr context set stores the current workspace and team") {
		t.Fatalf("expected context note in help, got %q", output)
	}
}

func TestRunContextSetAndViewJSON(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"context", "set",
		"--workspace", "ws_123",
		"--team", "team_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var setPayload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &setPayload); err != nil {
		t.Fatalf("decode set output: %v", err)
	}
	if got := setPayload["workspaceID"]; got != "ws_123" {
		t.Fatalf("unexpected workspaceID %#v", got)
	}
	if got := setPayload["teamID"]; got != "team_123" {
		t.Fatalf("unexpected teamID %#v", got)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{"context", "view", "--json"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var viewPayload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &viewPayload); err != nil {
		t.Fatalf("decode view output: %v", err)
	}
	if got := viewPayload["workspaceID"]; got != "ws_123" {
		t.Fatalf("unexpected workspaceID %#v", got)
	}
	if got := viewPayload["teamID"]; got != "team_123" {
		t.Fatalf("unexpected teamID %#v", got)
	}
}

func TestParseKnowledgeSyncDocumentSupportsTypeAlias(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "templates", "launch-email.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`---
title: Launch Email
type: template
---

# Launch Email
`), 0o600))

	document, err := parseKnowledgeSyncDocument(path, "templates/launch-email.md")
	require.NoError(t, err)
	assert.Equal(t, "template", document.Kind)
}

func TestResolveKnowledgeSyncKindInfersFromPath(t *testing.T) {
	t.Parallel()

	document := knowledgeSyncDocument{
		RelativePath: "marketing/best-practices/campaign-naming.md",
		BodyMarkdown: "# Campaign Naming Convention\n\nUse this format.",
	}

	kind, err := resolveKnowledgeSyncKind(document, "")
	require.NoError(t, err)
	assert.Equal(t, string(knowledgedomain.KnowledgeResourceKindBestPractice), kind)
}

func TestNormalizeKnowledgeKindValueAcceptsAliases(t *testing.T) {
	t.Parallel()

	kind, err := normalizeKnowledgeKindValue("best practices")
	require.NoError(t, err)
	assert.Equal(t, string(knowledgedomain.KnowledgeResourceKindBestPractice), kind)

	kind, err = normalizeKnowledgeKindValue("adr")
	require.NoError(t, err)
	assert.Equal(t, string(knowledgedomain.KnowledgeResourceKindDecision), kind)
}

func TestRunKnowledgeImportPreviewJSONUsesStoredContext(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"context", "set",
		"--workspace", "ws_preview",
		"--team", "team_marketing",
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	root := t.TempDir()
	path := filepath.Join(root, "rfcs", "queue-routing.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`# RFC: Queue Routing

We should route campaign form-based capture into dedicated queues.
`), 0o600))

	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{
		"knowledge", "import", root,
		"--mode", "preview",
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	var plans []knowledgeImportPlan
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &plans))
	require.Len(t, plans, 1)
	assert.Equal(t, "team_marketing", plans[0].TeamID)
	assert.Equal(t, string(knowledgedomain.KnowledgeResourceKindDecision), plans[0].Kind)
	assert.Equal(t, "queue-routing", plans[0].Slug)
}

func TestRunKnowledgeCheckoutJSONUsesStoredContext(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIKnowledgeCheckoutResources") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		filter, _ := req.Variables["filter"].(map[string]any)
		if got := req.Variables["workspaceID"]; got != "ws_checkout" {
			t.Fatalf("unexpected workspaceID %#v", got)
		}
		if got := filter["teamID"]; got != "team_marketing" {
			t.Fatalf("unexpected team filter %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"knowledgeResources": []map[string]any{
					{
						"id":                 "kr_123",
						"workspaceID":        "ws_checkout",
						"ownerTeamID":        "team_marketing",
						"slug":               "launch-plan",
						"title":              "Launch Plan",
						"kind":               "guide",
						"conceptSpecKey":     "core/guide",
						"conceptSpecVersion": "1",
						"sourceKind":         "workspace",
						"sourceRef":          nil,
						"pathRef":            nil,
						"artifactPath":       "knowledge/teams/team_marketing/private/launch-plan.md",
						"summary":            "Plan the launch",
						"bodyMarkdown":       "# Launch\n\nShip it.",
						"frontmatter":        map[string]any{},
						"supportedChannels":  []string{},
						"sharedWithTeamIDs":  []string{},
						"surface":            "private",
						"trustLevel":         "workspace",
						"searchKeywords":     []string{"launch"},
						"status":             "draft",
						"reviewStatus":       "draft",
						"contentHash":        "abc123",
						"revisionRef":        "rev_123",
						"publishedRevision":  nil,
						"reviewedAt":         nil,
						"publishedAt":        nil,
						"publishedByID":      nil,
						"createdByID":        "user_123",
						"createdAt":          "2026-03-21T10:00:00Z",
						"updatedAt":          "2026-03-21T10:00:00Z",
					},
				},
			},
		}
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"context", "set",
		"--workspace", "ws_checkout",
		"--team", "team_marketing",
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	root := filepath.Join(t.TempDir(), "checkout")
	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{
		"knowledge", "checkout", root,
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	var payload knowledgeCheckoutResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	assert.Equal(t, 1, payload.ResourceCount)

	rawManifest, err := os.ReadFile(filepath.Join(root, knowledgeCheckoutMetadataDir, knowledgeCheckoutManifestFile))
	require.NoError(t, err)
	assert.Contains(t, string(rawManifest), "\"workspaceID\": \"ws_checkout\"")

	rawMarkdown, err := os.ReadFile(filepath.Join(root, "knowledge", "teams", "team_marketing", "private", "launch-plan.md"))
	require.NoError(t, err)
	assert.Contains(t, string(rawMarkdown), "title: Launch Plan")
	assert.Contains(t, string(rawMarkdown), "# Launch")
}

func testKnowledgeCheckoutResource(revisionRef, body string) map[string]any {
	return map[string]any{
		"id":                 "kr_123",
		"workspaceID":        "ws_checkout",
		"ownerTeamID":        "team_marketing",
		"slug":               "launch-plan",
		"title":              "Launch Plan",
		"kind":               "guide",
		"conceptSpecKey":     "core/guide",
		"conceptSpecVersion": "1",
		"sourceKind":         "workspace",
		"sourceRef":          nil,
		"pathRef":            nil,
		"artifactPath":       "knowledge/teams/team_marketing/private/launch-plan.md",
		"summary":            "Plan the launch",
		"bodyMarkdown":       body,
		"frontmatter":        map[string]any{},
		"supportedChannels":  []string{},
		"sharedWithTeamIDs":  []string{},
		"surface":            "private",
		"trustLevel":         "workspace",
		"searchKeywords":     []string{"launch"},
		"status":             "draft",
		"reviewStatus":       "draft",
		"contentHash":        "abc123",
		"revisionRef":        revisionRef,
		"publishedRevision":  nil,
		"reviewedAt":         nil,
		"publishedAt":        nil,
		"publishedByID":      nil,
		"createdByID":        "user_123",
		"createdAt":          "2026-03-21T10:00:00Z",
		"updatedAt":          "2026-03-21T10:00:00Z",
	}
}

func TestRunKnowledgeStatusJSONDetectsAheadChanges(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIKnowledgeCheckoutResources") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"knowledgeResources": []map[string]any{
					testKnowledgeCheckoutResource("rev_123", "# Launch\n\nShip it."),
				},
			},
		}
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"context", "set",
		"--workspace", "ws_checkout",
		"--team", "team_marketing",
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	root := filepath.Join(t.TempDir(), "checkout")
	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{
		"knowledge", "checkout", root,
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	target := filepath.Join(root, "knowledge", "teams", "team_marketing", "private", "launch-plan.md")
	require.NoError(t, os.WriteFile(target, []byte("---\ntitle: Launch Plan\n---\n\n# Launch\n\nChanged locally.\n"), 0o644))

	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{
		"knowledge", "status", root,
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	var payload knowledgeCheckoutStatusResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	assert.Equal(t, "ahead", payload.Status)
	require.NotEmpty(t, payload.Entries)
	assert.Equal(t, "ahead", payload.Entries[0].State)
}

func TestRunKnowledgePullJSONAppliesServerChanges(t *testing.T) {
	callCount := 0
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIKnowledgeCheckoutResources") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		callCount++
		resource := testKnowledgeCheckoutResource("rev_123", "# Launch\n\nShip it.")
		if callCount >= 2 {
			resource = testKnowledgeCheckoutResource("rev_456", "# Launch\n\nPulled from server.")
		}
		return map[string]any{
			"data": map[string]any{
				"knowledgeResources": []map[string]any{resource},
			},
		}
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"context", "set",
		"--workspace", "ws_checkout",
		"--team", "team_marketing",
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	root := filepath.Join(t.TempDir(), "checkout")
	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{
		"knowledge", "checkout", root,
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{
		"knowledge", "pull", root,
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	var payload knowledgeCheckoutPullResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	assert.Equal(t, 1, payload.Summary.Updated)

	rawMarkdown, err := os.ReadFile(filepath.Join(root, "knowledge", "teams", "team_marketing", "private", "launch-plan.md"))
	require.NoError(t, err)
	assert.Contains(t, string(rawMarkdown), "Pulled from server.")
}

func TestRunKnowledgePushJSONUpdatesTrackedFiles(t *testing.T) {
	callCount := 0
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		switch {
		case strings.Contains(req.Query, "query CLIKnowledgeCheckoutResources"):
			callCount++
			resource := testKnowledgeCheckoutResource("rev_123", "# Launch\n\nShip it.")
			if callCount >= 3 {
				resource = testKnowledgeCheckoutResource("rev_456", "# Launch\n\nChanged locally.")
			}
			return map[string]any{
				"data": map[string]any{
					"knowledgeResources": []map[string]any{resource},
				},
			}
		case strings.Contains(req.Query, "query CLIKnowledgeResource"):
			return map[string]any{
				"data": map[string]any{
					"knowledgeResource": testKnowledgeCheckoutResource("rev_123", "# Launch\n\nShip it."),
				},
			}
		case strings.Contains(req.Query, "mutation CLIUpdateKnowledgeResource"):
			input, _ := req.Variables["input"].(map[string]any)
			assert.Equal(t, "# Launch\n\nChanged locally.", input["bodyMarkdown"])
			return map[string]any{
				"data": map[string]any{
					"updateKnowledgeResource": testKnowledgeCheckoutResource("rev_456", "# Launch\n\nChanged locally."),
				},
			}
		default:
			t.Fatalf("unexpected query: %s", req.Query)
			return nil
		}
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"context", "set",
		"--workspace", "ws_checkout",
		"--team", "team_marketing",
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	root := filepath.Join(t.TempDir(), "checkout")
	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{
		"knowledge", "checkout", root,
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	target := filepath.Join(root, "knowledge", "teams", "team_marketing", "private", "launch-plan.md")
	require.NoError(t, os.WriteFile(target, []byte("---\ntitle: Launch Plan\n---\n\n# Launch\n\nChanged locally.\n"), 0o644))

	stdout.Reset()
	stderr.Reset()
	exitCode = run(t.Context(), []string{
		"knowledge", "push", root,
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	var payload knowledgeCheckoutPushResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	assert.Equal(t, 1, payload.Summary.Updated)

	rawMarkdown, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(rawMarkdown), "Changed locally.")
}

func TestRunAuthLoginStoresConfig(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIHealthAuth") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"me": map[string]any{
					"__typename": "Agent",
					"id":         "agent_123",
				},
			},
		}
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"auth", "login",
		"--url", "https://app.mbr.test",
		"--token", "hat_test",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	cfg, err := cliapi.LoadStoredConfig()
	if err != nil {
		t.Fatalf("LoadStoredConfig returned error: %v", err)
	}
	if cfg.InstanceURL != "https://app.mbr.test" {
		t.Fatalf("unexpected stored instance url %q", cfg.InstanceURL)
	}
	if cfg.Token != "hat_test" {
		t.Fatalf("unexpected stored token %q", cfg.Token)
	}
}

func TestRunAuthLoginBrowserStoresSessionConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)

	previousHTTPClient := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/auth/cli/start":
					body := `{"requestID":"req_123","pollToken":"poll_123","authorizeURL":"https://admin.mbr.test/cli-login?request_id=req_123","adminBaseURL":"https://admin.mbr.test","adminGraphQLURL":"https://admin.mbr.test/graphql","expiresInSeconds":600,"intervalSeconds":0}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
					}, nil
				case "/auth/cli/poll":
					body := `{"status":"ready","userID":"user_123","sessionToken":"session_cli"}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
					}, nil
				default:
					t.Fatalf("unexpected browser-login path %q", r.URL.Path)
					return nil, nil
				}
			}),
		}
	}
	t.Cleanup(func() {
		newHTTPClient = previousHTTPClient
	})

	previousOpenBrowser := openBrowserURL
	var openedURL string
	openBrowserURL = func(rawURL string) error {
		openedURL = rawURL
		return nil
	}
	t.Cleanup(func() {
		openBrowserURL = previousOpenBrowser
	})

	previousClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/graphql" {
					t.Fatalf("unexpected graphql path %q", r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "" {
					t.Fatalf("unexpected authorization header %q", got)
				}
				cookie, err := r.Cookie("mbr_session")
				if err != nil {
					t.Fatalf("expected mbr_session cookie: %v", err)
				}
				if cookie.Value != "session_cli" {
					t.Fatalf("unexpected session cookie %q", cookie.Value)
				}

				body := `{"data":{"me":{"__typename":"User","id":"user_123"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() {
		newCLIClient = previousClient
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"auth", "login",
		"--url", "https://app.mbr.test",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	if openedURL != "https://admin.mbr.test/cli-login?request_id=req_123" {
		t.Fatalf("unexpected browser URL %q", openedURL)
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload["loginTransport"] != "browser_session" {
		t.Fatalf("unexpected login transport %#v", payload["loginTransport"])
	}

	cfg, err := cliapi.LoadStoredConfig()
	if err != nil {
		t.Fatalf("LoadStoredConfig returned error: %v", err)
	}
	if cfg.AuthMode != cliapi.AuthModeSession {
		t.Fatalf("unexpected auth mode %q", cfg.AuthMode)
	}
	if cfg.InstanceURL != "https://app.mbr.test" {
		t.Fatalf("unexpected instance url %q", cfg.InstanceURL)
	}
	if cfg.SessionToken != "session_cli" {
		t.Fatalf("unexpected session token %q", cfg.SessionToken)
	}
	if cfg.AdminBaseURL != "https://admin.mbr.test" {
		t.Fatalf("unexpected admin base url %q", cfg.AdminBaseURL)
	}
}

func TestRunAuthLogoutClearsConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	if _, err := cliapi.SaveStoredConfig("https://app.mbr.test", "hat_test"); err != nil {
		t.Fatalf("SaveStoredConfig returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"auth", "logout",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected config file to be removed, stat err=%v", err)
	}
}

func TestRunCasesListJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLICases") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"cases": map[string]any{
					"totalCount": 1,
					"edges": []map[string]any{
						{
							"node": map[string]any{
								"id":          "case-1",
								"caseID":      "HC-1001",
								"workspaceID": "ws_123",
								"subject":     "Candidate follow-up",
								"status":      "open",
								"priority":    "high",
								"queueID":     "col_123",
								"queue": map[string]any{
									"id":   "col_123",
									"name": "Engineering",
								},
								"contact": map[string]any{
									"id":    "contact_1",
									"email": "candidate@example.com",
									"name":  "Candidate Example",
								},
								"assignee": map[string]any{
									"id":    "user_1",
									"email": "owner@example.com",
									"name":  "Owner Example",
								},
								"createdAt":  "2026-03-13T09:00:00Z",
								"updatedAt":  "2026-03-13T09:30:00Z",
								"resolvedAt": nil,
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
		"cases", "list",
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload struct {
		TotalCount int          `json:"totalCount"`
		Cases      []caseOutput `json:"cases"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.TotalCount != 1 {
		t.Fatalf("expected totalCount 1, got %d", payload.TotalCount)
	}
	if len(payload.Cases) != 1 || payload.Cases[0].CaseID != "HC-1001" {
		t.Fatalf("unexpected cases payload: %#v", payload.Cases)
	}
}

func TestRunCasesShowByHumanID(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLICaseByHumanID") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["caseID"]; got != "HC-42" {
			t.Fatalf("expected caseID HC-42, got %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"caseByHumanID": map[string]any{
					"id":          "case-42",
					"caseID":      "HC-42",
					"workspaceID": "ws_123",
					"subject":     "Review applicant packet",
					"status":      "open",
					"priority":    "normal",
					"queueID":     "col_ats",
					"queue": map[string]any{
						"id":   "col_ats",
						"name": "Designer",
					},
					"contact": map[string]any{
						"id":    "contact_42",
						"email": "designer@example.com",
						"name":  "Designer Example",
					},
					"assignee": nil,
					"workThread": []map[string]any{
						{
							"id":                    "msg:conv_1",
							"kind":                  "conversation_message",
							"communicationID":       nil,
							"conversationMessageID": "conv_msg_1",
							"conversationSessionID": "sess_1",
							"channel":               nil,
							"direction":             nil,
							"role":                  "user",
							"visibility":            "customer",
							"subject":               nil,
							"body":                  "Please review the applicant packet.",
							"createdAt":             "2026-03-13T09:50:00Z",
						},
						{
							"id":                    "comm:comm_1",
							"kind":                  "case_communication",
							"communicationID":       "comm_1",
							"conversationMessageID": nil,
							"conversationSessionID": nil,
							"channel":               "internal_note",
							"direction":             "internal",
							"role":                  nil,
							"visibility":            nil,
							"subject":               "Review started",
							"body":                  "Designer review has started.",
							"createdAt":             "2026-03-13T10:05:00Z",
						},
					},
					"createdAt":  "2026-03-13T10:00:00Z",
					"updatedAt":  "2026-03-13T10:30:00Z",
					"resolvedAt": nil,
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"cases", "show", "HC-42",
		"--workspace", "ws_123",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "caseID:\tHC-42") {
		t.Fatalf("expected caseID in output, got %q", output)
	}
	if !strings.Contains(output, "subject:\tReview applicant packet") {
		t.Fatalf("expected subject in output, got %q", output)
	}
	if !strings.Contains(output, "workThread:\t2") {
		t.Fatalf("expected workThread count in output, got %q", output)
	}
	if !strings.Contains(output, "Please review the applicant packet.") {
		t.Fatalf("expected conversation-derived thread entry in output, got %q", output)
	}
	if !strings.Contains(output, "Designer review has started.") {
		t.Fatalf("expected case communication thread entry in output, got %q", output)
	}
}

func TestRunContactsListJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIContacts") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["workspaceID"]; got != "ws_123" {
			t.Fatalf("expected workspaceID ws_123, got %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"contacts": []map[string]any{
					{
						"id":    "contact_1",
						"email": "candidate@example.com",
						"name":  "Candidate Example",
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
		"contacts", "list",
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []contactOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].Email != "candidate@example.com" {
		t.Fatalf("unexpected contacts payload: %#v", payload)
	}
}

func TestRunWorkspacesListJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIWorkspaces") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"workspaces": []map[string]any{
					{
						"id":        "ws_123",
						"name":      "Hiring",
						"shortCode": "hiring",
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
		"workspaces", "list",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []workspaceOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].ShortCode != "hiring" {
		t.Fatalf("unexpected workspaces payload: %#v", payload)
	}
}

func TestRunQueuesListJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIQueues") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"queues": []map[string]any{
					{
						"id":          "col_123",
						"workspaceID": "ws_123",
						"slug":        "engineering",
						"name":        "Engineering",
						"description": "Hiring",
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
		"queues", "list",
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []queueOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].Slug != "engineering" {
		t.Fatalf("unexpected queues payload: %#v", payload)
	}
}

func TestRunQueuesListJSONUsesStoredWorkspaceContext(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIQueues") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["workspaceID"]; got != "ws_123" {
			t.Fatalf("unexpected workspaceID %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"queues": []map[string]any{
					{
						"id":          "queue_123",
						"workspaceID": "ws_123",
						"slug":        "support",
						"name":        "Support",
					},
				},
			},
		}
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")
	workspaceID := "ws_123"
	_, err := cliapi.SaveStoredContext(&workspaceID, nil, false)
	require.NoError(t, err)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"queues", "list",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []queueOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].Slug != "support" {
		t.Fatalf("unexpected queues payload: %#v", payload)
	}
}

func TestRunQueuesCreateJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "mutation CLICreateQueue") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		input, _ := req.Variables["input"].(map[string]any)
		if got := input["workspaceID"]; got != "ws_123" {
			t.Fatalf("unexpected workspaceID %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"createQueue": map[string]any{
					"id":          "col_123",
					"workspaceID": "ws_123",
					"slug":        "engineering",
					"name":        "Engineering",
					"description": "Hiring pipeline",
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"queues", "create",
		"--workspace", "ws_123",
		"--name", "Engineering",
		"--slug", "engineering",
		"--description", "Hiring pipeline",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload queueOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "col_123" || payload.Slug != "engineering" {
		t.Fatalf("unexpected queue payload: %#v", payload)
	}
}

func TestRunQueuesItemsJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIQueueItems") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["id"]; got != "queue_123" {
			t.Fatalf("unexpected queue id %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"queue": map[string]any{
					"items": []map[string]any{
						{
							"id":          "qi_123",
							"workspaceID": "ws_123",
							"queueID":     "queue_123",
							"itemKind":    "case",
							"caseID":      "case_123",
							"createdAt":   "2026-03-21T09:00:00Z",
							"updatedAt":   "2026-03-21T10:00:00Z",
							"case": map[string]any{
								"id":         "case_123",
								"humanID":    "ops-2603-abcd12",
								"subject":    "Refund approval",
								"status":     "open",
								"priority":   "high",
								"teamID":     "team_billing",
								"assigneeID": "user_123",
								"updatedAt":  "2026-03-21T10:00:00Z",
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
		"queues", "items", "queue_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []queueItemOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].ID != "qi_123" {
		t.Fatalf("unexpected queue items payload: %#v", payload)
	}
	if payload[0].Case == nil || payload[0].Case.Status != "open" {
		t.Fatalf("expected case summary in payload: %#v", payload)
	}
}

func TestRunConversationsHandoffJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "mutation CLIHandoffConversation") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["sessionID"]; got != "conv_123" {
			t.Fatalf("unexpected session id %#v", got)
		}
		input, _ := req.Variables["input"].(map[string]any)
		if got := input["queueID"]; got != "queue_support" {
			t.Fatalf("unexpected queue id %#v", got)
		}
		if got := input["teamID"]; got != "team_support" {
			t.Fatalf("unexpected team id %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"handoffConversation": map[string]any{
					"id":             "conv_123",
					"workspaceID":    "ws_123",
					"channel":        "chat",
					"status":         "waiting",
					"title":          "Refund follow-up",
					"handlingTeamID": "team_support",
					"openedAt":       "2026-03-21T09:00:00Z",
					"lastActivityAt": "2026-03-21T10:00:00Z",
					"updatedAt":      "2026-03-21T10:00:00Z",
					"participants":   []map[string]any{},
					"messages":       []map[string]any{},
					"outcomes":       []map[string]any{},
					"workingState":   nil,
					"metadata":       map[string]any{},
					"createdAt":      "2026-03-21T09:00:00Z",
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"conversations", "handoff", "conv_123",
		"--queue", "queue_support",
		"--team", "team_support",
		"--reason", "billing specialist needed",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload conversationSessionOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "conv_123" {
		t.Fatalf("unexpected conversation payload: %#v", payload)
	}
	if payload.HandlingTeamID == nil || *payload.HandlingTeamID != "team_support" {
		t.Fatalf("expected handling team in payload: %#v", payload)
	}
}

func TestRunConversationsEscalateJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "mutation CLIEscalateConversation") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["sessionID"]; got != "conv_123" {
			t.Fatalf("unexpected session id %#v", got)
		}
		input, _ := req.Variables["input"].(map[string]any)
		if got := input["queueID"]; got != "queue_billing" {
			t.Fatalf("unexpected queue id %#v", got)
		}
		if got := input["priority"]; got != "HIGH" {
			t.Fatalf("unexpected priority %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"escalateConversation": map[string]any{
					"id":          "case_123",
					"caseID":      "ops-2603-abcd12",
					"workspaceID": "ws_123",
					"subject":     "Refund approval",
					"status":      "new",
					"priority":    "high",
					"queueID":     "queue_billing",
					"queue": map[string]any{
						"id":   "queue_billing",
						"name": "Billing",
					},
					"contact":    nil,
					"assignee":   nil,
					"createdAt":  "2026-03-21T10:00:00Z",
					"updatedAt":  "2026-03-21T10:00:00Z",
					"resolvedAt": nil,
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"conversations", "escalate", "conv_123",
		"--queue", "queue_billing",
		"--priority", "high",
		"--subject", "Refund approval",
		"--reason", "needs durable approval workflow",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload caseOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "case_123" || payload.Subject != "Refund approval" {
		t.Fatalf("unexpected case payload: %#v", payload)
	}
}

func TestRunCatalogListJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIServiceCatalogNodes") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["workspaceID"]; got != "ws_123" {
			t.Fatalf("unexpected workspaceID %#v", got)
		}
		if got := req.Variables["parentNodeID"]; got != "node_root" {
			t.Fatalf("unexpected parentNodeID %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"serviceCatalogNodes": []map[string]any{
					{
						"id":           "node_refunds",
						"workspaceID":  "ws_123",
						"parentNodeID": "node_root",
						"slug":         "refunds",
						"pathSlug":     "support/refunds",
						"title":        "Refund Requests",
						"nodeKind":     "request_type",
						"status":       "active",
						"visibility":   "workspace",
						"displayOrder": 10,
						"createdAt":    "2026-03-20T10:00:00Z",
						"updatedAt":    "2026-03-20T10:00:00Z",
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
		"catalog", "list",
		"--workspace", "ws_123",
		"--parent", "node_root",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []serviceCatalogNodeOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].PathSlug != "support/refunds" {
		t.Fatalf("unexpected catalog payload: %#v", payload)
	}
}

func TestRunCatalogShowJSONByPath(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIServiceCatalogNodeByPath") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["workspaceID"]; got != "ws_123" {
			t.Fatalf("unexpected workspaceID %#v", got)
		}
		if got := req.Variables["path"]; got != "support/refunds" {
			t.Fatalf("unexpected path %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"serviceCatalogNodeByPath": map[string]any{
					"id":                  "node_refunds",
					"workspaceID":         "ws_123",
					"parentNodeID":        "node_root",
					"slug":                "refunds",
					"pathSlug":            "support/refunds",
					"title":               "Refund Requests",
					"descriptionMarkdown": "Collect refund context and route to billing.",
					"nodeKind":            "request_type",
					"status":              "active",
					"visibility":          "workspace",
					"supportedChannels":   []string{"web_chat", "email"},
					"defaultCaseCategory": "billing",
					"defaultQueueID":      "queue_billing",
					"defaultPriority":     "high",
					"searchKeywords":      []string{"refund", "billing"},
					"displayOrder":        10,
					"createdAt":           "2026-03-20T10:00:00Z",
					"updatedAt":           "2026-03-20T10:00:00Z",
					"bindings": []map[string]any{
						{
							"id":            "binding_123",
							"workspaceID":   "ws_123",
							"catalogNodeID": "node_refunds",
							"targetKind":    "queue",
							"targetID":      "queue_billing",
							"bindingKind":   "default",
							"confidence":    1.0,
							"createdAt":     "2026-03-20T10:00:00Z",
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
		"catalog", "show", "support/refunds",
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload serviceCatalogNodeOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "node_refunds" || payload.PathSlug != "support/refunds" || len(payload.Bindings) != 1 {
		t.Fatalf("unexpected catalog show payload: %#v", payload)
	}
}

func TestRunFormSpecsListJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIFormSpecs") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["workspaceID"]; got != "ws_123" {
			t.Fatalf("unexpected workspaceID %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"formSpecs": []map[string]any{
					{
						"id":                   "spec_123",
						"workspaceID":          "ws_123",
						"name":                 "Refund Request",
						"slug":                 "refund-request",
						"publicKey":            "pub_refund",
						"descriptionMarkdown":  "Collect refund details.",
						"fieldSpec":            map[string]any{"type": "object"},
						"evidenceRequirements": []map[string]any{{"name": "receipt"}},
						"inferenceRules":       []map[string]any{},
						"approvalPolicy":       map[string]any{},
						"submissionPolicy":     map[string]any{},
						"destinationPolicy":    map[string]any{},
						"supportedChannels":    []string{"web_chat"},
						"isPublic":             true,
						"status":               "active",
						"metadata":             map[string]any{"surface": "widget"},
						"createdAt":            "2026-03-20T10:00:00Z",
						"updatedAt":            "2026-03-20T10:00:00Z",
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
		"forms", "specs", "list",
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []formSpecOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].Slug != "refund-request" {
		t.Fatalf("unexpected forms specs payload: %#v", payload)
	}
}

func TestRunFormSpecShowJSONBySlug(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIFormSpecBySlug") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["slug"]; got != "incident-report" {
			t.Fatalf("unexpected slug %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"formSpecBySlug": map[string]any{
					"id":                   "spec_456",
					"workspaceID":          "ws_123",
					"name":                 "Incident Report",
					"slug":                 "incident-report",
					"fieldSpec":            map[string]any{"type": "object"},
					"evidenceRequirements": []map[string]any{},
					"inferenceRules":       []map[string]any{},
					"approvalPolicy":       map[string]any{},
					"submissionPolicy":     map[string]any{},
					"destinationPolicy":    map[string]any{},
					"supportedChannels":    []string{"operator_console"},
					"isPublic":             false,
					"status":               "draft",
					"metadata":             map[string]any{},
					"createdAt":            "2026-03-20T10:00:00Z",
					"updatedAt":            "2026-03-20T10:00:00Z",
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"forms", "specs", "show", "incident-report",
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload formSpecOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "spec_456" || payload.Slug != "incident-report" {
		t.Fatalf("unexpected forms spec show payload: %#v", payload)
	}
}

func TestRunFormSpecsCreateJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "mutation CLICreateFormSpec") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		input, _ := req.Variables["input"].(map[string]any)
		if got := input["workspaceID"]; got != "ws_123" {
			t.Fatalf("unexpected workspaceID %#v", got)
		}
		if got := input["name"]; got != "Refund Request" {
			t.Fatalf("unexpected name %#v", got)
		}
		if got := input["status"]; got != "active" {
			t.Fatalf("unexpected status %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"createFormSpec": map[string]any{
					"id":                   "spec_new",
					"workspaceID":          "ws_123",
					"name":                 "Refund Request",
					"slug":                 "refund-request",
					"descriptionMarkdown":  "Collect refund details.",
					"fieldSpec":            map[string]any{"type": "object"},
					"evidenceRequirements": []map[string]any{},
					"inferenceRules":       []map[string]any{},
					"approvalPolicy":       map[string]any{},
					"submissionPolicy":     map[string]any{},
					"destinationPolicy":    map[string]any{},
					"supportedChannels":    []string{"web_chat"},
					"isPublic":             true,
					"status":               "active",
					"metadata":             map[string]any{"surface": "widget"},
					"createdAt":            "2026-03-21T09:00:00Z",
					"updatedAt":            "2026-03-21T09:00:00Z",
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"forms", "specs", "create",
		"--workspace", "ws_123",
		"--input-json", `{"name":"Refund Request","slug":"refund-request","status":"active","isPublic":true,"supportedChannels":["web_chat"],"fieldSpec":{"type":"object"},"metadata":{"surface":"widget"}}`,
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload formSpecOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "spec_new" || payload.Slug != "refund-request" {
		t.Fatalf("unexpected forms spec create payload: %#v", payload)
	}
}

func TestRunFormSpecsUpdateJSONBySlug(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		switch {
		case strings.Contains(req.Query, "query CLIFormSpecBySlug"):
			return map[string]any{
				"data": map[string]any{
					"formSpecBySlug": map[string]any{
						"id":                   "spec_456",
						"workspaceID":          "ws_123",
						"name":                 "Incident Report",
						"slug":                 "incident-report",
						"fieldSpec":            map[string]any{"type": "object"},
						"evidenceRequirements": []map[string]any{},
						"inferenceRules":       []map[string]any{},
						"approvalPolicy":       map[string]any{},
						"submissionPolicy":     map[string]any{},
						"destinationPolicy":    map[string]any{},
						"supportedChannels":    []string{"operator_console"},
						"isPublic":             false,
						"status":               "draft",
						"metadata":             map[string]any{},
						"createdAt":            "2026-03-20T10:00:00Z",
						"updatedAt":            "2026-03-20T10:00:00Z",
					},
				},
			}
		case strings.Contains(req.Query, "mutation CLIUpdateFormSpec"):
			if got := req.Variables["id"]; got != "spec_456" {
				t.Fatalf("unexpected update id %#v", got)
			}
			input, _ := req.Variables["input"].(map[string]any)
			if got := input["status"]; got != "active" {
				t.Fatalf("unexpected status %#v", got)
			}
			return map[string]any{
				"data": map[string]any{
					"updateFormSpec": map[string]any{
						"id":                   "spec_456",
						"workspaceID":          "ws_123",
						"name":                 "Incident Report",
						"slug":                 "incident-report",
						"fieldSpec":            map[string]any{"type": "object"},
						"evidenceRequirements": []map[string]any{},
						"inferenceRules":       []map[string]any{},
						"approvalPolicy":       map[string]any{},
						"submissionPolicy":     map[string]any{},
						"destinationPolicy":    map[string]any{},
						"supportedChannels":    []string{"operator_console", "web_chat"},
						"isPublic":             true,
						"status":               "active",
						"metadata":             map[string]any{"updatedBy": "agent"},
						"createdAt":            "2026-03-20T10:00:00Z",
						"updatedAt":            "2026-03-21T09:30:00Z",
					},
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
		"forms", "specs", "update", "incident-report",
		"--workspace", "ws_123",
		"--input-json", `{"status":"active","isPublic":true,"supportedChannels":["operator_console","web_chat"],"metadata":{"updatedBy":"agent"}}`,
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload formSpecOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "spec_456" || payload.Status != "active" || !payload.IsPublic {
		t.Fatalf("unexpected forms spec update payload: %#v", payload)
	}
}

func TestRunFormSubmissionsListJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIFormSubmissions") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		filter, _ := req.Variables["filter"].(map[string]any)
		if got := filter["formSpecID"]; got != "spec_123" {
			t.Fatalf("unexpected formSpecID %#v", got)
		}
		if got := filter["status"]; got != "submitted" {
			t.Fatalf("unexpected status %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"formSubmissions": []map[string]any{
					{
						"id":               "sub_123",
						"workspaceID":      "ws_123",
						"formSpecID":       "spec_123",
						"status":           "submitted",
						"channel":          "web_chat",
						"submitterEmail":   "casey@example.com",
						"submitterName":    "Casey",
						"collectedFields":  map[string]any{"summary": "Need refund"},
						"missingFields":    map[string]any{},
						"evidence":         []map[string]any{},
						"validationErrors": []string{},
						"metadata":         map[string]any{"source": "widget"},
						"submittedAt":      "2026-03-20T10:05:00Z",
						"createdAt":        "2026-03-20T10:00:00Z",
						"updatedAt":        "2026-03-20T10:05:00Z",
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
		"forms", "submissions", "list",
		"--workspace", "ws_123",
		"--spec", "spec_123",
		"--status", "submitted",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []formSubmissionOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].ID != "sub_123" {
		t.Fatalf("unexpected forms submissions payload: %#v", payload)
	}
}

func TestRunFormSubmissionShowJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIFormSubmission") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"formSubmission": map[string]any{
					"id":                    "sub_789",
					"workspaceID":           "ws_123",
					"formSpecID":            "spec_456",
					"conversationSessionID": "conv_123",
					"caseID":                "case_123",
					"contactID":             "contact_123",
					"status":                "accepted",
					"channel":               "operator_console",
					"submitterEmail":        "ops@example.com",
					"submitterName":         "Ops",
					"completionToken":       "resume-123",
					"collectedFields":       map[string]any{"summary": "Major incident"},
					"missingFields":         map[string]any{},
					"evidence":              []map[string]any{{"name": "screenshot"}},
					"validationErrors":      []string{},
					"metadata":              map[string]any{"source": "console"},
					"submittedAt":           "2026-03-20T10:10:00Z",
					"createdAt":             "2026-03-20T10:00:00Z",
					"updatedAt":             "2026-03-20T10:10:00Z",
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"forms", "submissions", "show", "sub_789",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload formSubmissionOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "sub_789" || payload.Status != "accepted" {
		t.Fatalf("unexpected forms submission show payload: %#v", payload)
	}
}

func TestRunFormSubmissionsCreateJSONBySlug(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		switch {
		case strings.Contains(req.Query, "query CLIFormSpecBySlug"):
			return map[string]any{
				"data": map[string]any{
					"formSpecBySlug": map[string]any{
						"id":                   "spec_456",
						"workspaceID":          "ws_123",
						"name":                 "Incident Report",
						"slug":                 "incident-report",
						"fieldSpec":            map[string]any{"type": "object"},
						"evidenceRequirements": []map[string]any{},
						"inferenceRules":       []map[string]any{},
						"approvalPolicy":       map[string]any{},
						"submissionPolicy":     map[string]any{},
						"destinationPolicy":    map[string]any{},
						"supportedChannels":    []string{"operator_console"},
						"isPublic":             false,
						"status":               "active",
						"metadata":             map[string]any{},
						"createdAt":            "2026-03-20T10:00:00Z",
						"updatedAt":            "2026-03-20T10:00:00Z",
					},
				},
			}
		case strings.Contains(req.Query, "mutation CLICreateFormSubmission"):
			input, _ := req.Variables["input"].(map[string]any)
			if got := input["formSpecID"]; got != "spec_456" {
				t.Fatalf("unexpected formSpecID %#v", got)
			}
			if got := input["status"]; got != "submitted" {
				t.Fatalf("unexpected status %#v", got)
			}
			return map[string]any{
				"data": map[string]any{
					"createFormSubmission": map[string]any{
						"id":               "sub_new",
						"workspaceID":      "ws_123",
						"formSpecID":       "spec_456",
						"status":           "submitted",
						"channel":          "operator_console",
						"submitterEmail":   "ops@example.com",
						"submitterName":    "Ops",
						"collectedFields":  map[string]any{"summary": "Database timeout"},
						"missingFields":    map[string]any{},
						"evidence":         []map[string]any{},
						"validationErrors": []string{},
						"metadata":         map[string]any{"source": "agent"},
						"submittedAt":      "2026-03-21T09:45:00Z",
						"createdAt":        "2026-03-21T09:45:00Z",
						"updatedAt":        "2026-03-21T09:45:00Z",
					},
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
		"forms", "submissions", "create", "incident-report",
		"--workspace", "ws_123",
		"--input-json", `{"status":"submitted","channel":"operator_console","submitterEmail":"ops@example.com","submitterName":"Ops","collectedFields":{"summary":"Database timeout"},"metadata":{"source":"agent"}}`,
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload formSubmissionOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "sub_new" || payload.FormSpecID != "spec_456" {
		t.Fatalf("unexpected forms submission create payload: %#v", payload)
	}
}

func TestRunKnowledgeListJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIKnowledgeResources") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		filter, _ := req.Variables["filter"].(map[string]any)
		if got := filter["search"]; got != "refund" {
			t.Fatalf("unexpected search filter %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"knowledgeResources": []map[string]any{
					{
						"id":                "kr_123",
						"workspaceID":       "ws_123",
						"slug":              "refund-policy",
						"title":             "Refund Policy",
						"kind":              "policy",
						"sourceKind":        "workspace",
						"summary":           "How refunds are handled",
						"bodyMarkdown":      "# Refund Policy",
						"frontmatter":       map[string]any{"audience": "support"},
						"supportedChannels": []string{"chat", "email"},
						"trustLevel":        "workspace",
						"searchKeywords":    []string{"refund"},
						"status":            "active",
						"contentHash":       "abc123",
						"createdAt":         "2026-03-20T10:00:00Z",
						"updatedAt":         "2026-03-20T10:00:00Z",
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
		"knowledge", "list",
		"--workspace", "ws_123",
		"--search", "refund",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []knowledgeResourceOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].Slug != "refund-policy" {
		t.Fatalf("unexpected knowledge payload: %#v", payload)
	}
}

func TestRunKnowledgeShowJSONBySlug(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIKnowledgeResourceBySlug") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"knowledgeResourceBySlug": map[string]any{
					"id":                "kr_123",
					"workspaceID":       "ws_123",
					"ownerTeamID":       "team_123",
					"slug":              "incident-playbook",
					"title":             "Incident Playbook",
					"kind":              "guide",
					"sourceKind":        "workspace",
					"artifactPath":      "knowledge/teams/team_123/private/incident-playbook.md",
					"bodyMarkdown":      "# Incident Playbook",
					"frontmatter":       map[string]any{},
					"supportedChannels": []string{"chat"},
					"sharedWithTeamIDs": []string{},
					"surface":           "private",
					"trustLevel":        "workspace",
					"searchKeywords":    []string{"incident"},
					"status":            "draft",
					"reviewStatus":      "draft",
					"contentHash":       "def456",
					"revisionRef":       "rev_123",
					"createdAt":         "2026-03-20T10:00:00Z",
					"updatedAt":         "2026-03-20T10:00:00Z",
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"knowledge", "show", "incident-playbook",
		"--workspace", "ws_123",
		"--team", "team_123",
		"--surface", "private",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload knowledgeResourceOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "kr_123" || payload.Slug != "incident-playbook" {
		t.Fatalf("unexpected knowledge show payload: %#v", payload)
	}
}

func TestRunKnowledgeShowJSONBySlugUsesStoredContext(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIKnowledgeResourceBySlug") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["workspaceID"]; got != "ws_123" {
			t.Fatalf("unexpected workspaceID %#v", got)
		}
		if got := req.Variables["teamID"]; got != "team_123" {
			t.Fatalf("unexpected teamID %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"knowledgeResourceBySlug": map[string]any{
					"id":                "kr_123",
					"workspaceID":       "ws_123",
					"ownerTeamID":       "team_123",
					"slug":              "incident-playbook",
					"title":             "Incident Playbook",
					"kind":              "guide",
					"sourceKind":        "workspace",
					"artifactPath":      "knowledge/teams/team_123/private/incident-playbook.md",
					"bodyMarkdown":      "# Incident Playbook",
					"frontmatter":       map[string]any{},
					"supportedChannels": []string{"chat"},
					"sharedWithTeamIDs": []string{},
					"surface":           "private",
					"trustLevel":        "workspace",
					"searchKeywords":    []string{"incident"},
					"status":            "draft",
					"reviewStatus":      "draft",
					"contentHash":       "def456",
					"revisionRef":       "rev_123",
					"createdAt":         "2026-03-20T10:00:00Z",
					"updatedAt":         "2026-03-20T10:00:00Z",
				},
			},
		}
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")
	workspaceID := "ws_123"
	teamID := "team_123"
	_, err := cliapi.SaveStoredContext(&workspaceID, &teamID, false)
	require.NoError(t, err)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"knowledge", "show", "incident-playbook",
		"--surface", "private",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload knowledgeResourceOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "kr_123" || payload.OwnerTeamID != "team_123" {
		t.Fatalf("unexpected knowledge show payload: %#v", payload)
	}
}

func TestRunKnowledgeUpsertCreateJSON(t *testing.T) {
	requestCount := 0
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		requestCount++
		switch requestCount {
		case 1:
			if !strings.Contains(req.Query, "query CLIKnowledgeResourceBySlug") {
				t.Fatalf("unexpected first query: %s", req.Query)
			}
			return map[string]any{
				"data": map[string]any{
					"knowledgeResourceBySlug": nil,
				},
			}
		case 2:
			if !strings.Contains(req.Query, "mutation CLICreateKnowledgeResource") {
				t.Fatalf("unexpected second query: %s", req.Query)
			}
			input, _ := req.Variables["input"].(map[string]any)
			if got := input["slug"]; got != "refund-policy" {
				t.Fatalf("unexpected slug %#v", got)
			}
			if got := input["teamID"]; got != "team_123" {
				t.Fatalf("unexpected teamID %#v", got)
			}
			if got := input["bodyMarkdown"]; got != "# Refund Policy" {
				t.Fatalf("unexpected body %#v", got)
			}
			return map[string]any{
				"data": map[string]any{
					"createKnowledgeResource": map[string]any{
						"id":                "kr_123",
						"workspaceID":       "ws_123",
						"ownerTeamID":       "team_123",
						"slug":              "refund-policy",
						"title":             "Refund Policy",
						"kind":              "policy",
						"sourceKind":        "workspace",
						"artifactPath":      "knowledge/teams/team_123/private/refund-policy.md",
						"bodyMarkdown":      "# Refund Policy",
						"frontmatter":       map[string]any{},
						"supportedChannels": []string{"chat"},
						"sharedWithTeamIDs": []string{},
						"surface":           "private",
						"trustLevel":        "workspace",
						"searchKeywords":    []string{"refund"},
						"status":            "active",
						"reviewStatus":      "draft",
						"contentHash":       "abc123",
						"revisionRef":       "rev_123",
						"createdAt":         "2026-03-20T10:00:00Z",
						"updatedAt":         "2026-03-20T10:00:00Z",
					},
				},
			}
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"knowledge", "upsert",
		"--workspace", "ws_123",
		"--team", "team_123",
		"--slug", "refund-policy",
		"--title", "Refund Policy",
		"--kind", "policy",
		"--status", "active",
		"--body", "# Refund Policy",
		"--channels", "chat",
		"--keywords", "refund",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload knowledgeResourceOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "kr_123" || payload.Status != "active" {
		t.Fatalf("unexpected knowledge create payload: %#v", payload)
	}
}

func TestRunKnowledgeSyncJSONUsesStoredContext(t *testing.T) {
	requestCount := 0
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		requestCount++
		switch requestCount {
		case 1, 2, 3:
			if !strings.Contains(req.Query, "query CLIKnowledgeResourceBySlug") {
				t.Fatalf("unexpected lookup query: %s", req.Query)
			}
			var surface string
			if got, ok := req.Variables["surface"].(string); ok {
				surface = got
			}
			expectedSurfaces := []string{"private", "published", "workspace_shared"}
			if surface != expectedSurfaces[requestCount-1] {
				t.Fatalf("unexpected surface lookup %q at request %d", surface, requestCount)
			}
			return map[string]any{
				"data": map[string]any{
					"knowledgeResourceBySlug": nil,
				},
			}
		case 4:
			if !strings.Contains(req.Query, "mutation CLICreateKnowledgeResource") {
				t.Fatalf("unexpected create query: %s", req.Query)
			}
			input, _ := req.Variables["input"].(map[string]any)
			if got := input["workspaceID"]; got != "ws_123" {
				t.Fatalf("unexpected workspaceID %#v", got)
			}
			if got := input["teamID"]; got != "team_123" {
				t.Fatalf("unexpected teamID %#v", got)
			}
			if got := input["slug"]; got != "refund-policy" {
				t.Fatalf("unexpected slug %#v", got)
			}
			if got := input["pathRef"]; got != "refund-policy.md" {
				t.Fatalf("unexpected pathRef %#v", got)
			}
			if got := input["sourceKind"]; got != "imported" {
				t.Fatalf("unexpected sourceKind %#v", got)
			}
			return map[string]any{
				"data": map[string]any{
					"createKnowledgeResource": map[string]any{
						"id":                "kr_123",
						"workspaceID":       "ws_123",
						"ownerTeamID":       "team_123",
						"slug":              "refund-policy",
						"title":             "Refund Policy",
						"kind":              "guide",
						"sourceKind":        "imported",
						"sourceRef":         "refund-policy.md",
						"artifactPath":      "knowledge/teams/team_123/private/refund-policy.md",
						"summary":           "How refunds are handled",
						"bodyMarkdown":      "# Refund Policy\n\nRefunds are reviewed within 3 business days.",
						"frontmatter":       map[string]any{},
						"supportedChannels": []string{},
						"sharedWithTeamIDs": []string{},
						"surface":           "private",
						"trustLevel":        "workspace",
						"searchKeywords":    []string{"billing", "refund"},
						"status":            "draft",
						"reviewStatus":      "draft",
						"contentHash":       "abc123",
						"revisionRef":       "rev_123",
						"createdAt":         "2026-03-20T10:00:00Z",
						"updatedAt":         "2026-03-20T10:00:00Z",
					},
				},
			}
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil
		}
	})

	root := t.TempDir()
	knowledgePath := filepath.Join(root, "refund-policy.md")
	err := os.WriteFile(knowledgePath, []byte(`---
summary: How refunds are handled
search_keywords:
  - refund
  - billing
---

# Refund Policy

Refunds are reviewed within 3 business days.
`), 0o600)
	require.NoError(t, err)

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")
	workspaceID := "ws_123"
	teamID := "team_123"
	_, err = cliapi.SaveStoredContext(&workspaceID, &teamID, false)
	require.NoError(t, err)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"knowledge", "sync", root,
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []knowledgeSyncResult
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected 1 synced resource, got %#v", payload)
	}
	if payload[0].Action != "created" || payload[0].Slug != "refund-policy" {
		t.Fatalf("unexpected sync payload: %#v", payload)
	}
}

func TestRunKnowledgeUpsertUpdateJSON(t *testing.T) {
	requestCount := 0
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		requestCount++
		switch requestCount {
		case 1:
			if !strings.Contains(req.Query, "query CLIKnowledgeResourceBySlug") {
				t.Fatalf("unexpected first query: %s", req.Query)
			}
			return map[string]any{
				"data": map[string]any{
					"knowledgeResourceBySlug": map[string]any{
						"id":                "kr_123",
						"workspaceID":       "ws_123",
						"ownerTeamID":       "team_123",
						"slug":              "incident-playbook",
						"title":             "Incident Playbook",
						"kind":              "guide",
						"sourceKind":        "workspace",
						"artifactPath":      "knowledge/teams/team_123/private/incident-playbook.md",
						"bodyMarkdown":      "# Incident Playbook",
						"frontmatter":       map[string]any{},
						"supportedChannels": []string{"chat"},
						"sharedWithTeamIDs": []string{},
						"surface":           "private",
						"trustLevel":        "workspace",
						"searchKeywords":    []string{"incident"},
						"status":            "draft",
						"reviewStatus":      "draft",
						"contentHash":       "abc123",
						"revisionRef":       "rev_123",
						"createdAt":         "2026-03-20T10:00:00Z",
						"updatedAt":         "2026-03-20T10:00:00Z",
					},
				},
			}
		case 2:
			if !strings.Contains(req.Query, "mutation CLIUpdateKnowledgeResource") {
				t.Fatalf("unexpected second query: %s", req.Query)
			}
			input, _ := req.Variables["input"].(map[string]any)
			if got := input["title"]; got != "Incident Response Playbook" {
				t.Fatalf("unexpected title %#v", got)
			}
			return map[string]any{
				"data": map[string]any{
					"updateKnowledgeResource": map[string]any{
						"id":                "kr_123",
						"workspaceID":       "ws_123",
						"ownerTeamID":       "team_123",
						"slug":              "incident-playbook",
						"title":             "Incident Response Playbook",
						"kind":              "guide",
						"sourceKind":        "workspace",
						"artifactPath":      "knowledge/teams/team_123/private/incident-playbook.md",
						"bodyMarkdown":      "# Incident Response Playbook",
						"frontmatter":       map[string]any{},
						"supportedChannels": []string{"chat"},
						"sharedWithTeamIDs": []string{},
						"surface":           "private",
						"trustLevel":        "workspace",
						"searchKeywords":    []string{"incident"},
						"status":            "active",
						"reviewStatus":      "draft",
						"contentHash":       "xyz789",
						"revisionRef":       "rev_456",
						"createdAt":         "2026-03-20T10:00:00Z",
						"updatedAt":         "2026-03-20T11:00:00Z",
					},
				},
			}
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"knowledge", "upsert",
		"--workspace", "ws_123",
		"--team", "team_123",
		"--slug", "incident-playbook",
		"--title", "Incident Response Playbook",
		"--status", "active",
		"--body", "# Incident Response Playbook",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload knowledgeResourceOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.Title != "Incident Response Playbook" || payload.Status != "active" {
		t.Fatalf("unexpected knowledge update payload: %#v", payload)
	}
}

func TestRunKnowledgePushCreatesLocalOnlyFiles(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, knowledgeCheckoutMetadataDir, knowledgeCheckoutManifestFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0o755))
	rawManifest, err := json.MarshalIndent(knowledgeCheckoutManifest{
		SchemaVersion: knowledgeCheckoutSchemaVersion,
		InstanceURL:   "https://app.mbr.test",
		WorkspaceID:   "ws_123",
		Filters: knowledgeCheckoutFilter{
			WorkspaceID: "ws_123",
		},
		CheckedOutAt: "2026-03-24T10:00:00Z",
		Resources:    []knowledgeCheckoutManifestEntry{},
	}, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, append(rawManifest, '\n'), 0o600))

	localPath := filepath.Join(root, "knowledge", "teams", "team_123", "private", "refund-policy.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(localPath), 0o755))
	require.NoError(t, os.WriteFile(localPath, []byte(`---
title: Refund Policy
team_id: team_123
surface: private
kind: policy
status: active
summary: How refunds are handled
search_keywords:
  - refund
---

# Refund Policy

Refunds are reviewed within 3 business days.
`), 0o600))

	requestCount := 0
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		requestCount++
		switch requestCount {
		case 1:
			if !strings.Contains(req.Query, "query CLIKnowledgeCheckoutResources") {
				t.Fatalf("unexpected query: %s", req.Query)
			}
			return map[string]any{"data": map[string]any{"knowledgeResources": []any{}}}
		case 2, 3, 4:
			if !strings.Contains(req.Query, "query CLIKnowledgeResourceBySlug") {
				t.Fatalf("unexpected query: %s", req.Query)
			}
			return map[string]any{"data": map[string]any{"knowledgeResourceBySlug": nil}}
		case 5:
			if !strings.Contains(req.Query, "mutation CLICreateKnowledgeResource") {
				t.Fatalf("unexpected query: %s", req.Query)
			}
			return map[string]any{
				"data": map[string]any{
					"createKnowledgeResource": map[string]any{
						"id":                 "kr_123",
						"workspaceID":        "ws_123",
						"ownerTeamID":        "team_123",
						"slug":               "refund-policy",
						"title":              "Refund Policy",
						"kind":               "policy",
						"conceptSpecKey":     "core/policy",
						"conceptSpecVersion": "1",
						"sourceKind":         "workspace",
						"sourceRef":          "knowledge/teams/team_123/private/refund-policy.md",
						"pathRef":            "knowledge/teams/team_123/private/refund-policy.md",
						"artifactPath":       "knowledge/teams/team_123/private/refund-policy.md",
						"summary":            "How refunds are handled",
						"bodyMarkdown":       "# Refund Policy\n\nRefunds are reviewed within 3 business days.",
						"frontmatter":        map[string]any{},
						"supportedChannels":  []string{},
						"sharedWithTeamIDs":  []string{},
						"surface":            "private",
						"trustLevel":         "workspace",
						"searchKeywords":     []string{"refund"},
						"status":             "active",
						"reviewStatus":       "draft",
						"contentHash":        "abc123",
						"revisionRef":        "rev_123",
						"createdAt":          "2026-03-24T10:00:00Z",
						"updatedAt":          "2026-03-24T10:00:00Z",
					},
				},
			}
		case 6:
			if !strings.Contains(req.Query, "query CLIKnowledgeCheckoutResources") {
				t.Fatalf("unexpected query: %s", req.Query)
			}
			return map[string]any{
				"data": map[string]any{
					"knowledgeResources": []any{
						map[string]any{
							"id":                 "kr_123",
							"workspaceID":        "ws_123",
							"ownerTeamID":        "team_123",
							"slug":               "refund-policy",
							"title":              "Refund Policy",
							"kind":               "policy",
							"conceptSpecKey":     "core/policy",
							"conceptSpecVersion": "1",
							"sourceKind":         "workspace",
							"sourceRef":          "knowledge/teams/team_123/private/refund-policy.md",
							"pathRef":            "knowledge/teams/team_123/private/refund-policy.md",
							"artifactPath":       "knowledge/teams/team_123/private/refund-policy.md",
							"summary":            "How refunds are handled",
							"bodyMarkdown":       "# Refund Policy\n\nRefunds are reviewed within 3 business days.",
							"frontmatter":        map[string]any{},
							"supportedChannels":  []string{},
							"sharedWithTeamIDs":  []string{},
							"surface":            "private",
							"trustLevel":         "workspace",
							"searchKeywords":     []string{"refund"},
							"status":             "active",
							"reviewStatus":       "draft",
							"contentHash":        "abc123",
							"revisionRef":        "rev_123",
							"createdAt":          "2026-03-24T10:00:00Z",
							"updatedAt":          "2026-03-24T10:00:00Z",
						},
					},
				},
			}
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{"knowledge", "push", root, "--json"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload knowledgeCheckoutPushResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	require.Equal(t, 1, payload.Summary.Added)
	require.Equal(t, 0, payload.Summary.Updated)
	require.Equal(t, 0, payload.Summary.Deleted)
	require.Len(t, payload.Entries, 1)
	require.Equal(t, "created", payload.Entries[0].Action)
}

func TestRunKnowledgePushDeletesTrackedFiles(t *testing.T) {
	root := t.TempDir()
	rendered := `---
title: Incident Playbook
slug: incident-playbook
team_id: team_123
surface: private
kind: guide
concept_spec: core/guide
concept_spec_version: "1"
review_status: draft
status: draft
---

# Incident Playbook
`
	manifest := knowledgeCheckoutManifest{
		SchemaVersion: knowledgeCheckoutSchemaVersion,
		InstanceURL:   "https://app.mbr.test",
		WorkspaceID:   "ws_123",
		Filters: knowledgeCheckoutFilter{
			WorkspaceID: "ws_123",
		},
		CheckedOutAt: "2026-03-24T10:00:00Z",
		Resources: []knowledgeCheckoutManifestEntry{{
			ID:           "kr_123",
			OwnerTeamID:  "team_123",
			Surface:      "private",
			Slug:         "incident-playbook",
			Title:        "Incident Playbook",
			Kind:         "guide",
			ArtifactPath: "knowledge/teams/team_123/private/incident-playbook.md",
			RevisionRef:  "rev_123",
			RenderedHash: knowledgeRenderedHash(rendered),
		}},
	}
	manifestPath := filepath.Join(root, knowledgeCheckoutMetadataDir, knowledgeCheckoutManifestFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0o755))
	rawManifest, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, append(rawManifest, '\n'), 0o600))

	requestCount := 0
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		requestCount++
		switch requestCount {
		case 1:
			if !strings.Contains(req.Query, "query CLIKnowledgeCheckoutResources") {
				t.Fatalf("unexpected query: %s", req.Query)
			}
			return map[string]any{
				"data": map[string]any{
					"knowledgeResources": []any{
						map[string]any{
							"id":           "kr_123",
							"artifactPath": "knowledge/teams/team_123/private/incident-playbook.md",
							"revisionRef":  "rev_123",
						},
					},
				},
			}
		case 2:
			if !strings.Contains(req.Query, "mutation CLIDeleteKnowledgeResource") {
				t.Fatalf("unexpected query: %s", req.Query)
			}
			return map[string]any{
				"data": map[string]any{
					"deleteKnowledgeResource": map[string]any{
						"id":                 "kr_123",
						"workspaceID":        "ws_123",
						"ownerTeamID":        "team_123",
						"slug":               "incident-playbook",
						"title":              "Incident Playbook",
						"kind":               "guide",
						"conceptSpecKey":     "core/guide",
						"conceptSpecVersion": "1",
						"sourceKind":         "workspace",
						"artifactPath":       "knowledge/teams/team_123/private/incident-playbook.md",
						"bodyMarkdown":       "# Incident Playbook",
						"frontmatter":        map[string]any{},
						"supportedChannels":  []string{},
						"sharedWithTeamIDs":  []string{},
						"surface":            "private",
						"trustLevel":         "workspace",
						"searchKeywords":     []string{},
						"status":             "draft",
						"reviewStatus":       "draft",
						"contentHash":        "abc123",
						"revisionRef":        "rev_456",
						"createdAt":          "2026-03-24T10:00:00Z",
						"updatedAt":          "2026-03-24T11:00:00Z",
					},
				},
			}
		case 3:
			if !strings.Contains(req.Query, "query CLIKnowledgeCheckoutResources") {
				t.Fatalf("unexpected query: %s", req.Query)
			}
			return map[string]any{"data": map[string]any{"knowledgeResources": []any{}}}
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{"knowledge", "push", root, "--json"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload knowledgeCheckoutPushResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	require.Equal(t, 0, payload.Summary.Added)
	require.Equal(t, 0, payload.Summary.Updated)
	require.Equal(t, 1, payload.Summary.Deleted)
	require.Len(t, payload.Entries, 1)
	require.Equal(t, "deleted", payload.Entries[0].Action)
}

func TestRunConceptsHistoryJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIConceptSpecHistory") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"conceptSpecHistory": []any{
					map[string]any{
						"ref":         "rev_123",
						"committedAt": "2026-03-24T10:00:00Z",
						"subject":     "concept spec register strategy/campaign-brief@1",
					},
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{"concepts", "history", "strategy/campaign-brief", "--workspace", "ws_123", "--json"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []artifactRevisionOutput
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	require.Len(t, payload, 1)
	require.Equal(t, "rev_123", payload[0].Ref)
}

func TestRunConceptsDiffJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIConceptSpecDiff") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"conceptSpecDiff": map[string]any{
					"path":       "concepts/strategy/campaign-brief/v1/spec.yaml",
					"toRevision": "rev_123",
					"patch":      "diff --git a/spec.yaml b/spec.yaml",
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{"concepts", "diff", "strategy/campaign-brief", "--workspace", "ws_123", "--json"}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload knowledgeDiffOutput
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	require.Equal(t, "rev_123", payload.ToRevision)
	require.Contains(t, payload.Patch, "diff --git")
}

func TestRunTeamsListJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLITeams") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["workspaceID"]; got != "ws_123" {
			t.Fatalf("unexpected workspaceID %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"teams": []map[string]any{
					{
						"id":                  "team_123",
						"workspaceID":         "ws_123",
						"name":                "Marketing",
						"description":         "Launch operations",
						"emailAddress":        "marketing@example.com",
						"responseTimeHours":   4,
						"resolutionTimeHours": 24,
						"autoAssign":          true,
						"autoAssignKeywords":  []string{"launch", "campaign"},
						"isActive":            true,
						"createdAt":           "2026-03-20T10:00:00Z",
						"updatedAt":           "2026-03-20T10:00:00Z",
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
		"teams", "list",
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []teamOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].Name != "Marketing" {
		t.Fatalf("unexpected teams payload: %#v", payload)
	}
}

func TestRunTeamsCreateJSON(t *testing.T) {
	previousCLIClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/graphql" {
					t.Fatalf("unexpected graphql path %q", r.URL.Path)
				}
				cookie, err := r.Cookie("mbr_session")
				if err != nil || cookie.Value != "session_cli" {
					t.Fatalf("expected session cookie, got %v %#v", err, cookie)
				}

				var req testGraphQLRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				if !strings.Contains(req.Query, "mutation CLICreateTeam") {
					t.Fatalf("unexpected query: %s", req.Query)
				}
				input, ok := req.Variables["input"].(map[string]any)
				if !ok {
					t.Fatalf("expected input map, got %#v", req.Variables["input"])
				}
				if got := input["workspaceID"]; got != "ws_123" {
					t.Fatalf("unexpected workspaceID %#v", got)
				}
				if got := input["name"]; got != "Marketing" {
					t.Fatalf("unexpected team name %#v", got)
				}

				body, err := json.Marshal(map[string]any{
					"data": map[string]any{
						"createTeam": map[string]any{
							"id":                  "team_123",
							"workspaceID":         "ws_123",
							"name":                "Marketing",
							"description":         "Launch operations",
							"emailAddress":        "marketing@example.com",
							"responseTimeHours":   4,
							"resolutionTimeHours": 24,
							"autoAssign":          true,
							"autoAssignKeywords":  []string{"launch", "campaign"},
							"isActive":            true,
							"createdAt":           "2026-03-20T10:00:00Z",
							"updatedAt":           "2026-03-20T10:00:00Z",
						},
					},
				})
				require.NoError(t, err)

				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(bytes.NewReader(body)),
				}, nil
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() {
		newCLIClient = previousCLIClient
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	if _, err := cliapi.SaveStoredSessionConfig("https://app.mbr.test", "https://admin.mbr.test", "session_cli"); err != nil {
		t.Fatalf("SaveStoredSessionConfig returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"teams", "create",
		"--workspace", "ws_123",
		"--name", "Marketing",
		"--description", "Launch operations",
		"--email", "marketing@example.com",
		"--response-hours", "4",
		"--resolution-hours", "24",
		"--auto-assign",
		"--auto-assign-keywords", "launch,campaign",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload teamOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "team_123" || payload.Name != "Marketing" {
		t.Fatalf("unexpected team payload: %#v", payload)
	}
}

func TestRunTeamMembersListJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLITeamMembers") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["id"]; got != "team_123" {
			t.Fatalf("unexpected team id %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"team": map[string]any{
					"id": "team_123",
					"members": []map[string]any{
						{
							"id":          "tm_123",
							"teamID":      "team_123",
							"userID":      "user_123",
							"workspaceID": "ws_123",
							"role":        "lead",
							"isActive":    true,
							"joinedAt":    "2026-03-20T10:00:00Z",
							"createdAt":   "2026-03-20T10:00:00Z",
							"updatedAt":   "2026-03-20T10:00:00Z",
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
		"teams", "members", "list",
		"--team", "team_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []teamMemberOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].UserID != "user_123" {
		t.Fatalf("unexpected team members payload: %#v", payload)
	}
}

func TestRunTeamMembersAddJSON(t *testing.T) {
	previousCLIClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/graphql" {
					t.Fatalf("unexpected graphql path %q", r.URL.Path)
				}
				cookie, err := r.Cookie("mbr_session")
				if err != nil || cookie.Value != "session_cli" {
					t.Fatalf("expected session cookie, got %v %#v", err, cookie)
				}

				var req testGraphQLRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				if !strings.Contains(req.Query, "mutation CLIAddTeamMember") {
					t.Fatalf("unexpected query: %s", req.Query)
				}
				input, ok := req.Variables["input"].(map[string]any)
				if !ok {
					t.Fatalf("expected input map, got %#v", req.Variables["input"])
				}
				if got := input["teamID"]; got != "team_123" {
					t.Fatalf("unexpected teamID %#v", got)
				}
				if got := input["userID"]; got != "user_123" {
					t.Fatalf("unexpected userID %#v", got)
				}

				body, err := json.Marshal(map[string]any{
					"data": map[string]any{
						"addTeamMember": map[string]any{
							"id":          "tm_123",
							"teamID":      "team_123",
							"userID":      "user_123",
							"workspaceID": "ws_123",
							"role":        "lead",
							"isActive":    true,
							"joinedAt":    "2026-03-20T10:00:00Z",
							"createdAt":   "2026-03-20T10:00:00Z",
							"updatedAt":   "2026-03-20T10:00:00Z",
						},
					},
				})
				require.NoError(t, err)

				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(bytes.NewReader(body)),
				}, nil
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() {
		newCLIClient = previousCLIClient
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	if _, err := cliapi.SaveStoredSessionConfig("https://app.mbr.test", "https://admin.mbr.test", "session_cli"); err != nil {
		t.Fatalf("SaveStoredSessionConfig returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"teams", "members", "add",
		"--team", "team_123",
		"--user", "user_123",
		"--role", "lead",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload teamMemberOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "tm_123" || payload.Role != "lead" {
		t.Fatalf("unexpected team member payload: %#v", payload)
	}
}

func TestRunCasesListJSONSupportsQueueFilter(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLICases") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		filter, ok := req.Variables["filter"].(map[string]any)
		if !ok {
			t.Fatalf("expected filter map, got %#v", req.Variables["filter"])
		}
		if got := filter["queueID"]; got != "queue_123" {
			t.Fatalf("expected queue filter to map to queueID, got %#v", got)
		}
		return map[string]any{
			"data": map[string]any{
				"cases": map[string]any{
					"totalCount": 0,
					"edges":      []map[string]any{},
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"cases", "list",
		"--workspace", "ws_123",
		"--queue", "queue_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}
}

func TestRunAdminFormsListJSON(t *testing.T) {
	previousCLIClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/graphql" {
					t.Fatalf("unexpected graphql path %q", r.URL.Path)
				}
				cookie, err := r.Cookie("mbr_session")
				if err != nil || cookie.Value != "session_cli" {
					t.Fatalf("expected session cookie, got %v %#v", err, cookie)
				}

				var req testGraphQLRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				if !strings.Contains(req.Query, "query CLIAdminForms") {
					t.Fatalf("unexpected query: %s", req.Query)
				}
				filter, ok := req.Variables["filter"].(map[string]any)
				if !ok {
					t.Fatalf("expected filter map, got %#v", req.Variables["filter"])
				}
				if got := filter["workspaceID"]; got != "ws_123" {
					t.Fatalf("unexpected workspaceID %#v", got)
				}

				body, err := json.Marshal(map[string]any{
					"data": map[string]any{
						"adminForms": map[string]any{
							"totalCount": 1,
							"edges": []map[string]any{
								{"node": map[string]any{
									"id":                 "form_123",
									"workspaceID":        "ws_123",
									"workspaceName":      "Support",
									"name":               "Contact Us",
									"slug":               "contact-us",
									"description":        "Customer contact form",
									"status":             "active",
									"cryptoID":           "crypto_123",
									"isPublic":           true,
									"requiresCaptcha":    false,
									"collectEmail":       true,
									"autoCreateCase":     true,
									"autoCasePriority":   "medium",
									"autoCaseType":       "support",
									"autoAssignTeamID":   nil,
									"autoTags":           []string{"form-submission"},
									"notifyOnSubmission": false,
									"notificationEmails": []string{},
									"submissionMessage":  "Thanks",
									"redirectURL":        nil,
									"schemaData":         map[string]any{"fields": []map[string]any{{"name": "email"}}},
									"submissionCount":    3,
									"createdAt":          "2026-03-14T10:00:00Z",
									"updatedAt":          "2026-03-14T10:05:00Z",
									"createdByID":        "user_1",
								}},
							},
						},
					},
				})
				require.NoError(t, err)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() { newCLIClient = previousCLIClient })

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	require.NoError(t, func() error {
		_, err := cliapi.SaveStoredSessionConfig("https://app.mbr.test", "https://admin.mbr.test", "session_cli")
		return err
	}())

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := runAdminForms(t.Context(), []string{
		"list",
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	var payload struct {
		TotalCount int          `json:"totalCount"`
		Forms      []formOutput `json:"forms"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	require.Equal(t, 1, payload.TotalCount)
	require.Len(t, payload.Forms, 1)
	assert.Equal(t, "contact-us", payload.Forms[0].Slug)
	assert.Equal(t, "active", payload.Forms[0].Status)
}

func TestRunAdminFormsCreateJSON(t *testing.T) {
	previousCLIClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/graphql" {
					t.Fatalf("unexpected graphql path %q", r.URL.Path)
				}
				cookie, err := r.Cookie("mbr_session")
				if err != nil || cookie.Value != "session_cli" {
					t.Fatalf("expected session cookie, got %v %#v", err, cookie)
				}

				var req testGraphQLRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				if !strings.Contains(req.Query, "mutation CLICreateForm") {
					t.Fatalf("unexpected query: %s", req.Query)
				}
				input, ok := req.Variables["input"].(map[string]any)
				if !ok {
					t.Fatalf("expected input map, got %#v", req.Variables["input"])
				}
				assert.Equal(t, "ws_123", input["workspaceID"])
				assert.Equal(t, "Contact Us", input["name"])
				assert.Equal(t, "contact-us", input["slug"])
				assert.Equal(t, "active", input["status"])
				assert.Equal(t, true, input["isPublic"])
				assert.Equal(t, true, input["autoCreateCase"])
				schemaData, ok := input["schemaData"].(map[string]any)
				if !ok {
					t.Fatalf("expected schemaData map, got %#v", input["schemaData"])
				}
				if _, ok := schemaData["fields"].([]any); !ok {
					t.Fatalf("expected default fields array, got %#v", schemaData["fields"])
				}

				body, err := json.Marshal(map[string]any{
					"data": map[string]any{
						"adminCreateForm": map[string]any{
							"id":                 "form_123",
							"workspaceID":        "ws_123",
							"workspaceName":      "Support",
							"name":               "Contact Us",
							"slug":               "contact-us",
							"description":        "Customer contact form",
							"status":             "active",
							"cryptoID":           "crypto_123",
							"isPublic":           true,
							"requiresCaptcha":    false,
							"collectEmail":       true,
							"autoCreateCase":     true,
							"autoCasePriority":   nil,
							"autoCaseType":       nil,
							"autoAssignTeamID":   nil,
							"autoTags":           []string{},
							"notifyOnSubmission": false,
							"notificationEmails": []string{},
							"submissionMessage":  nil,
							"redirectURL":        nil,
							"schemaData":         map[string]any{"fields": []map[string]any{{"name": "name"}}},
							"submissionCount":    0,
							"createdAt":          "2026-03-14T10:00:00Z",
							"updatedAt":          "2026-03-14T10:00:00Z",
							"createdByID":        "user_1",
						},
					},
				})
				require.NoError(t, err)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() { newCLIClient = previousCLIClient })

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	require.NoError(t, func() error {
		_, err := cliapi.SaveStoredSessionConfig("https://app.mbr.test", "https://admin.mbr.test", "session_cli")
		return err
	}())

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := runAdminForms(t.Context(), []string{
		"create",
		"--workspace", "ws_123",
		"--name", "Contact Us",
		"--slug", "contact-us",
		"--description", "Customer form",
		"--public",
		"--auto-create-case",
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	var payload formOutput
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	assert.Equal(t, "form_123", payload.ID)
	assert.Equal(t, "contact-us", payload.Slug)
	assert.True(t, payload.IsPublic)
	assert.True(t, payload.AutoCreateCase)
}

func TestRunAutomationRulesListJSON(t *testing.T) {
	previousCLIClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/graphql" {
					t.Fatalf("unexpected graphql path %q", r.URL.Path)
				}
				cookie, err := r.Cookie("mbr_session")
				if err != nil || cookie.Value != "session_cli" {
					t.Fatalf("expected session cookie, got %v %#v", err, cookie)
				}
				var req testGraphQLRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				if !strings.Contains(req.Query, "query CLIAdminRules") {
					t.Fatalf("unexpected query: %s", req.Query)
				}
				body, err := json.Marshal(map[string]any{
					"data": map[string]any{
						"adminRules": map[string]any{
							"totalCount": 1,
							"edges": []map[string]any{
								{"node": map[string]any{
									"id":                   "rule_123",
									"workspaceID":          "ws_123",
									"workspaceName":        "Support",
									"title":                "Auto-tag support forms",
									"description":          "Tag new form submissions",
									"isActive":             true,
									"priority":             5,
									"maxExecutionsPerHour": 0,
									"maxExecutionsPerDay":  0,
									"conditions":           []map[string]any{{"field": "event", "operator": "equals", "value": "form.submitted"}},
									"actions":              []map[string]any{{"type": "add_tag", "value": "form-submission"}},
									"executionCount":       2,
									"lastExecutedAt":       "2026-03-14T11:00:00Z",
									"createdAt":            "2026-03-14T10:00:00Z",
									"updatedAt":            "2026-03-14T10:05:00Z",
									"createdByID":          "user_1",
								}},
							},
						},
					},
				})
				require.NoError(t, err)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() { newCLIClient = previousCLIClient })

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	require.NoError(t, func() error {
		_, err := cliapi.SaveStoredSessionConfig("https://app.mbr.test", "https://admin.mbr.test", "session_cli")
		return err
	}())

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"automation", "rules", "list",
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	var payload struct {
		TotalCount int          `json:"totalCount"`
		Rules      []ruleOutput `json:"rules"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	require.Equal(t, 1, payload.TotalCount)
	require.Len(t, payload.Rules, 1)
	assert.Equal(t, "rule_123", payload.Rules[0].ID)
	assert.Equal(t, "Auto-tag support forms", payload.Rules[0].Title)
}

func TestRunAutomationRulesCreateJSON(t *testing.T) {
	previousCLIClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/graphql" {
					t.Fatalf("unexpected graphql path %q", r.URL.Path)
				}
				cookie, err := r.Cookie("mbr_session")
				if err != nil || cookie.Value != "session_cli" {
					t.Fatalf("expected session cookie, got %v %#v", err, cookie)
				}
				var req testGraphQLRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				if !strings.Contains(req.Query, "mutation CLICreateRule") {
					t.Fatalf("unexpected query: %s", req.Query)
				}
				input, ok := req.Variables["input"].(map[string]any)
				if !ok {
					t.Fatalf("expected input map, got %#v", req.Variables["input"])
				}
				assert.Equal(t, "ws_123", input["workspaceID"])
				assert.Equal(t, "Auto-tag support forms", input["title"])
				assert.Equal(t, true, input["isActive"])
				if _, ok := input["conditions"].([]any); !ok {
					t.Fatalf("expected conditions array, got %#v", input["conditions"])
				}
				if _, ok := input["actions"].([]any); !ok {
					t.Fatalf("expected actions array, got %#v", input["actions"])
				}

				body, err := json.Marshal(map[string]any{
					"data": map[string]any{
						"adminCreateRule": map[string]any{
							"id":                   "rule_123",
							"workspaceID":          "ws_123",
							"workspaceName":        "Support",
							"title":                "Auto-tag support forms",
							"description":          "Tag new form submissions",
							"isActive":             true,
							"priority":             10,
							"maxExecutionsPerHour": 0,
							"maxExecutionsPerDay":  0,
							"conditions":           []map[string]any{{"field": "event", "operator": "equals", "value": "form.submitted"}},
							"actions":              []map[string]any{{"type": "add_tag", "value": "form-submission"}},
							"executionCount":       0,
							"lastExecutedAt":       nil,
							"createdAt":            "2026-03-14T10:00:00Z",
							"updatedAt":            "2026-03-14T10:00:00Z",
							"createdByID":          "user_1",
						},
					},
				})
				require.NoError(t, err)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() { newCLIClient = previousCLIClient })

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	require.NoError(t, func() error {
		_, err := cliapi.SaveStoredSessionConfig("https://app.mbr.test", "https://admin.mbr.test", "session_cli")
		return err
	}())

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"automation", "rules", "create",
		"--workspace", "ws_123",
		"--title", "Auto-tag support forms",
		"--description", "Tag new form submissions",
		"--conditions-json", `[{"field":"event","operator":"equals","value":"form.submitted"}]`,
		"--actions-json", `[{"type":"add_tag","value":"form-submission"}]`,
		"--active",
		"--priority", "10",
		"--json",
	}, stdout, stderr)
	require.Equal(t, 0, exitCode, "stderr=%s", stderr.String())

	var payload ruleOutput
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	assert.Equal(t, "rule_123", payload.ID)
	assert.True(t, payload.IsActive)
	assert.Equal(t, 10, payload.Priority)
}

func TestRunCasesSetStatusJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		switch {
		case strings.Contains(req.Query, "query CLICaseByHumanID"):
			return map[string]any{
				"data": map[string]any{
					"caseByHumanID": map[string]any{
						"id":          "case_123",
						"caseID":      "HC-42",
						"workspaceID": "ws_123",
						"subject":     "Review applicant packet",
						"status":      "open",
						"priority":    "normal",
						"queueID":     nil,
						"queue":       nil,
						"contact":     nil,
						"assignee":    nil,
						"createdAt":   "2026-03-13T10:00:00Z",
						"updatedAt":   "2026-03-13T10:30:00Z",
						"resolvedAt":  nil,
					},
				},
			}
		case strings.Contains(req.Query, "mutation CLIUpdateCaseStatus"):
			if got := req.Variables["status"]; got != "RESOLVED" {
				t.Fatalf("unexpected status %#v", got)
			}
			return map[string]any{
				"data": map[string]any{
					"updateCaseStatus": map[string]any{
						"id":          "case_123",
						"caseID":      "HC-42",
						"workspaceID": "ws_123",
						"subject":     "Review applicant packet",
						"status":      "resolved",
						"priority":    "normal",
						"queueID":     nil,
						"queue":       nil,
						"contact":     nil,
						"assignee":    nil,
						"createdAt":   "2026-03-13T10:00:00Z",
						"updatedAt":   "2026-03-13T10:45:00Z",
						"resolvedAt":  "2026-03-13T10:45:00Z",
					},
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
		"cases", "set-status", "HC-42",
		"--workspace", "ws_123",
		"--status", "resolved",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload caseOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.Status != "resolved" || payload.ID != "case_123" {
		t.Fatalf("unexpected case payload: %#v", payload)
	}
}

func TestRunCasesHandoffJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		switch {
		case strings.Contains(req.Query, "query CLICaseByHumanID"):
			return map[string]any{
				"data": map[string]any{
					"caseByHumanID": map[string]any{
						"id":                        "case_123",
						"caseID":                    "HC-42",
						"workspaceID":               "ws_123",
						"subject":                   "Review applicant packet",
						"status":                    "open",
						"priority":                  "normal",
						"teamID":                    "team_support",
						"queueID":                   "queue_triage",
						"queue":                     map[string]any{"id": "queue_triage", "name": "Triage"},
						"contact":                   nil,
						"assignee":                  nil,
						"originatingConversationID": "conv_123",
						"originatingConversation":   nil,
						"communications":            []any{},
						"createdAt":                 "2026-03-13T10:00:00Z",
						"updatedAt":                 "2026-03-13T10:30:00Z",
						"resolvedAt":                nil,
					},
				},
			}
		case strings.Contains(req.Query, "mutation CLIHandoffCase"):
			input, _ := req.Variables["input"].(map[string]any)
			if got := input["queueID"]; got != "queue_billing" {
				t.Fatalf("unexpected queueID %#v", got)
			}
			if got := input["teamID"]; got != "team_billing" {
				t.Fatalf("unexpected teamID %#v", got)
			}
			if got := input["reason"]; got != "refund specialist" {
				t.Fatalf("unexpected reason %#v", got)
			}
			return map[string]any{
				"data": map[string]any{
					"handoffCase": map[string]any{
						"id":                        "case_123",
						"caseID":                    "HC-42",
						"workspaceID":               "ws_123",
						"subject":                   "Review applicant packet",
						"status":                    "open",
						"priority":                  "normal",
						"teamID":                    "team_billing",
						"queueID":                   "queue_billing",
						"queue":                     map[string]any{"id": "queue_billing", "name": "Billing"},
						"contact":                   nil,
						"assignee":                  nil,
						"originatingConversationID": "conv_123",
						"createdAt":                 "2026-03-13T10:00:00Z",
						"updatedAt":                 "2026-03-13T10:45:00Z",
						"resolvedAt":                nil,
					},
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
		"cases", "handoff", "HC-42",
		"--workspace", "ws_123",
		"--team", "team_billing",
		"--queue", "queue_billing",
		"--reason", "refund specialist",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload caseOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.CaseID != "HC-42" {
		t.Fatalf("unexpected case payload: %#v", payload)
	}
	if payload.TeamID == nil || *payload.TeamID != "team_billing" {
		t.Fatalf("unexpected team handoff payload: %#v", payload)
	}
	if payload.QueueID == nil || *payload.QueueID != "queue_billing" {
		t.Fatalf("unexpected queue handoff payload: %#v", payload)
	}
}

func TestRunWorkspacesCreateJSON(t *testing.T) {
	previousHTTPClient := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/actions/workspaces" {
					t.Fatalf("unexpected admin path %q", r.URL.Path)
				}
				if auth := r.Header.Get("Authorization"); auth != "" {
					t.Fatalf("unexpected auth header %q", auth)
				}
				cookie, err := r.Cookie("mbr_session")
				if err != nil {
					t.Fatalf("expected mbr_session cookie: %v", err)
				}
				if cookie.Value != "session_cli" {
					t.Fatalf("unexpected session cookie %q", cookie.Value)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				if !strings.Contains(string(body), `"name":"Hiring"`) {
					t.Fatalf("unexpected request body %q", string(body))
				}
				responseBody, err := json.Marshal(map[string]any{
					"ID":          "ws_123",
					"Name":        "Hiring",
					"Slug":        "hiring",
					"Description": "Recruiting workspace",
				})
				if err != nil {
					t.Fatalf("marshal response: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusCreated,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(bytes.NewReader(responseBody)),
				}, nil
			}),
		}
	}
	t.Cleanup(func() {
		newHTTPClient = previousHTTPClient
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	if _, err := cliapi.SaveStoredSessionConfig("https://app.mbr.test", "https://admin.mbr.test", "session_cli"); err != nil {
		t.Fatalf("SaveStoredSessionConfig returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"workspaces", "create",
		"--url", "https://app.mbr.test",
		"--name", "Hiring",
		"--slug", "hiring",
		"--description", "Recruiting workspace",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload workspaceOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ShortCode != "hiring" {
		t.Fatalf("unexpected workspace output: %#v", payload)
	}
}

func TestRunHealthCheckJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if r.URL.Path != "/graphql" {
			t.Fatalf("unexpected graphql path %q", r.URL.Path)
		}
		return map[string]any{
			"data": map[string]any{
				"me": map[string]any{
					"__typename": "Agent",
					"id":         "agent_123",
				},
			},
		}
	})

	previousHTTPClient := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/health" {
					t.Fatalf("unexpected health path %q", r.URL.Path)
				}
				if r.Method != http.MethodGet {
					t.Fatalf("unexpected health method %q", r.Method)
				}
				body := `{"status":"ok","service":"Move Big Rocks Platform","version":"v1.1.0","git_commit":"abc123","build_date":"2026-03-13T12:00:00Z","build":{"instance_id":"inst_acme_123"}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		}
	}
	t.Cleanup(func() {
		newHTTPClient = previousHTTPClient
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"health", "check",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload healthCheckResult
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.Status != "ok" || payload.Service != "Move Big Rocks Platform" {
		t.Fatalf("unexpected health payload: %#v", payload)
	}
	if payload.InstanceID == nil || *payload.InstanceID != "inst_acme_123" {
		t.Fatalf("expected instance id in payload, got %#v", payload.InstanceID)
	}
	if !payload.AuthOK || payload.PrincipalType != "Agent" || payload.PrincipalID != "agent_123" {
		t.Fatalf("expected auth success in payload, got %#v", payload)
	}
}

func TestRunExtensionsMonitorJSON(t *testing.T) {
	requests := 0
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if got := req.Variables["id"]; got != "ext_123" {
			t.Fatalf("expected extension id ext_123, got %#v", got)
		}
		requests++
		if strings.Contains(req.Query, "mutation CLICheckExtensionHealth") {
			return map[string]any{
				"data": map[string]any{
					"checkExtensionHealth": map[string]any{
						"id":                "ext_123",
						"workspaceID":       "ws_123",
						"slug":              "ats",
						"name":              "Applicant Tracking",
						"publisher":         "DemandOps",
						"version":           "1.0.0",
						"kind":              "product",
						"scope":             "workspace",
						"risk":              "standard",
						"status":            "active",
						"validationStatus":  "passed",
						"validationMessage": "ready",
						"healthStatus":      "healthy",
						"healthMessage":     "runtime healthy",
					},
				},
			}
		}
		if !strings.Contains(req.Query, "query CLIExtension") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"extension": map[string]any{
					"id":                 "ext_123",
					"workspaceID":        "ws_123",
					"slug":               "ats",
					"name":               "Applicant Tracking",
					"publisher":          "DemandOps",
					"version":            "1.0.0",
					"description":        "ATS workspace pack",
					"kind":               "product",
					"scope":              "workspace",
					"risk":               "standard",
					"workspacePlan":      map[string]any{"mode": "provision_dedicated_workspace", "name": "People", "slug": "people", "description": "People operations"},
					"permissions":        []string{"case:read", "case:write"},
					"publicRoutes":       []map[string]any{{"pathPrefix": "/careers", "assetPath": "templates/careers"}},
					"adminRoutes":        []map[string]any{{"pathPrefix": "/extensions/ats", "assetPath": "templates/admin/ats"}},
					"endpoints":          []map[string]any{{"name": "careers-site", "class": "public_page", "mountPath": "/careers", "methods": []string{"GET"}, "auth": "public", "contentTypes": []string{"text/html"}, "maxBodyBytes": 0, "rateLimitPerMinute": 60, "workspaceBinding": "none", "assetPath": "templates/careers/index.html", "serviceTarget": nil}},
					"events":             map[string]any{"publishes": []map[string]any{{"type": "candidate.reviewed", "description": "Candidate reviewed", "schemaVersion": 1}}, "subscribes": []string{"case.created"}},
					"commands":           []map[string]any{{"name": "ats.jobs.publish", "description": "Publish a job"}},
					"agentSkills":        []map[string]any{{"name": "publish-job", "description": "Guide an agent through publishing a job", "assetPath": "agent-skills/publish-job.md"}},
					"customizableAssets": []string{"templates/careers/index.html"},
					"status":             "active",
					"validationStatus":   "passed",
					"validationMessage":  "ready",
					"healthStatus":       "healthy",
					"healthMessage":      "ok",
					"bundleSHA256":       "abc123",
					"bundleSize":         2048,
					"installedByID":      "user_1",
					"installedAt":        "2026-03-13T10:00:00Z",
					"activatedAt":        "2026-03-13T10:05:00Z",
					"deactivatedAt":      nil,
					"validatedAt":        "2026-03-13T10:04:00Z",
					"lastHealthCheckAt":  "2026-03-13T10:06:00Z",
					"assets":             []map[string]any{{"id": "asset_1", "path": "templates/careers/index.html", "kind": "template", "contentType": "text/html", "isCustomizable": true, "checksum": "assetsha", "size": 256, "updatedAt": "2026-03-13T10:03:00Z"}},
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "monitor",
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
	if payload.Slug != "ats" || payload.HealthStatus != "healthy" {
		t.Fatalf("unexpected extension payload: %#v", payload)
	}
	if len(payload.Endpoints) != 1 || payload.Endpoints[0].MountPath != "/careers" {
		t.Fatalf("unexpected endpoint payload: %#v", payload.Endpoints)
	}
	if len(payload.AgentSkills) != 1 || payload.AgentSkills[0].AssetPath != "agent-skills/publish-job.md" {
		t.Fatalf("unexpected agent skills payload: %#v", payload.AgentSkills)
	}
	if len(payload.Events.Publishes) != 1 || payload.Events.Publishes[0].Type != "candidate.reviewed" {
		t.Fatalf("unexpected event payload: %#v", payload.Events)
	}
	if requests != 2 {
		t.Fatalf("expected mutation + detail query, got %d requests", requests)
	}
}

func TestRunExtensionsUninstallDryRunJSON(t *testing.T) {
	requests := 0
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		requests++
		switch {
		case strings.Contains(req.Query, "query CLIExtensionArtifactFiles"):
			if got := req.Variables["surface"]; got != "careers" {
				t.Fatalf("expected artifact surface careers, got %#v", got)
			}
			return map[string]any{
				"data": map[string]any{
					"extensionArtifactFiles": []map[string]any{
						{
							"surface": "careers",
							"path":    "jobs/index.html",
						},
					},
				},
			}
		case strings.Contains(req.Query, "query CLIExtension"):
			return map[string]any{
				"data": map[string]any{
					"extension": map[string]any{
						"id":           "ext_123",
						"workspaceID":  "ws_123",
						"slug":         "ats",
						"name":         "Applicant Tracking",
						"publisher":    "DemandOps",
						"version":      "1.0.0",
						"kind":         "product",
						"scope":        "workspace",
						"risk":         "standard",
						"runtimeClass": "bundle",
						"storageClass": "owned_schema",
						"schema": map[string]any{
							"name":            "ext_ats",
							"packageKey":      "demandops/ats",
							"targetVersion":   "1.0.0",
							"migrationEngine": "sql",
						},
						"artifactSurfaces": []map[string]any{
							{
								"name":          "careers",
								"description":   "ATS careers site",
								"seedAssetPath": "templates/careers/index.html",
							},
						},
						"status":           "active",
						"validationStatus": "passed",
						"healthStatus":     "healthy",
						"bundleSHA256":     "abc123",
						"bundleSize":       2048,
						"installedAt":      "2026-03-13T10:00:00Z",
						"assets": []map[string]any{
							{
								"id":             "asset_1",
								"path":           "templates/careers/index.html",
								"kind":           "template",
								"contentType":    "text/html",
								"isCustomizable": true,
								"checksum":       "assetsha",
								"size":           256,
								"updatedAt":      "2026-03-13T10:03:00Z",
							},
						},
					},
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
		"extensions", "uninstall",
		"--id", "ext_123",
		"--dry-run",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload struct {
		ID      string `json:"id"`
		DryRun  bool   `json:"dryRun"`
		Planned struct {
			RequiresExportConfirmation bool `json:"requiresExportConfirmation"`
			SchemaCleanup              *struct {
				SuggestedSQL string `json:"suggestedSQL"`
			} `json:"schemaCleanup"`
			ArtifactFilesBySurface map[string][]extensionArtifactFileOutput `json:"artifactFilesBySurface"`
		} `json:"planned"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "ext_123" || !payload.DryRun {
		t.Fatalf("unexpected uninstall payload: %#v", payload)
	}
	if !payload.Planned.RequiresExportConfirmation {
		t.Fatalf("expected export confirmation requirement in payload: %#v", payload.Planned)
	}
	if payload.Planned.SchemaCleanup == nil || payload.Planned.SchemaCleanup.SuggestedSQL != "DROP SCHEMA IF EXISTS ext_ats CASCADE;" {
		t.Fatalf("expected schema cleanup guidance in payload: %#v", payload.Planned.SchemaCleanup)
	}
	if len(payload.Planned.ArtifactFilesBySurface["careers"]) != 1 {
		t.Fatalf("expected artifact files in payload: %#v", payload.Planned.ArtifactFilesBySurface)
	}
	if requests != 2 {
		t.Fatalf("expected detail and artifact list requests, got %d", requests)
	}
}

func TestRunExtensionsUninstallRequiresExportConfirmation(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIExtension") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"extension": map[string]any{
					"id":           "ext_123",
					"workspaceID":  "ws_123",
					"slug":         "ats",
					"name":         "Applicant Tracking",
					"publisher":    "DemandOps",
					"version":      "1.0.0",
					"kind":         "product",
					"scope":        "workspace",
					"risk":         "standard",
					"runtimeClass": "bundle",
					"storageClass": "shared_primitives_only",
					"status":       "inactive",
					"bundleSHA256": "abc123",
					"bundleSize":   2048,
					"installedAt":  "2026-03-13T10:00:00Z",
					"assets": []map[string]any{
						{
							"id":             "asset_1",
							"path":           "templates/careers/index.html",
							"kind":           "template",
							"contentType":    "text/html",
							"isCustomizable": true,
							"checksum":       "assetsha",
							"size":           256,
							"updatedAt":      "2026-03-13T10:03:00Z",
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
		"extensions", "uninstall",
		"--id", "ext_123",
	}, stdout, stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d stderr=%s stdout=%s", exitCode, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "--export-out PATH or --confirm-no-export") {
		t.Fatalf("expected export confirmation guidance, got %q", stderr.String())
	}
}

func TestRunExtensionsUninstallDeactivateAndExportJSON(t *testing.T) {
	requests := 0
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		requests++
		switch {
		case strings.Contains(req.Query, "query CLIExtensionArtifactFiles"):
			return map[string]any{
				"data": map[string]any{
					"extensionArtifactFiles": []map[string]any{
						{
							"surface": "careers",
							"path":    "jobs/index.html",
						},
					},
				},
			}
		case strings.Contains(req.Query, "query CLIExtension"):
			return map[string]any{
				"data": map[string]any{
					"extension": map[string]any{
						"id":           "ext_123",
						"workspaceID":  "ws_123",
						"slug":         "ats",
						"name":         "Applicant Tracking",
						"publisher":    "DemandOps",
						"version":      "1.0.0",
						"kind":         "product",
						"scope":        "workspace",
						"risk":         "standard",
						"runtimeClass": "bundle",
						"storageClass": "owned_schema",
						"schema": map[string]any{
							"name":            "ext_ats",
							"packageKey":      "demandops/ats",
							"targetVersion":   "1.0.0",
							"migrationEngine": "sql",
						},
						"artifactSurfaces": []map[string]any{
							{
								"name":          "careers",
								"description":   "ATS careers site",
								"seedAssetPath": "templates/careers/index.html",
							},
						},
						"status":           "active",
						"validationStatus": "passed",
						"healthStatus":     "healthy",
						"bundleSHA256":     "abc123",
						"bundleSize":       2048,
						"installedAt":      "2026-03-13T10:00:00Z",
						"assets": []map[string]any{
							{
								"id":             "asset_1",
								"path":           "templates/careers/index.html",
								"kind":           "template",
								"contentType":    "text/html",
								"isCustomizable": true,
								"checksum":       "assetsha",
								"size":           256,
								"updatedAt":      "2026-03-13T10:03:00Z",
							},
						},
					},
				},
			}
		case strings.Contains(req.Query, "mutation CLIDeactivateExtension"):
			if got := req.Variables["reason"]; got != "cleanup before uninstall" {
				t.Fatalf("expected deactivation reason, got %#v", got)
			}
			return map[string]any{
				"data": map[string]any{
					"deactivateExtension": map[string]any{
						"id":                "ext_123",
						"workspaceID":       "ws_123",
						"slug":              "ats",
						"name":              "Applicant Tracking",
						"publisher":         "DemandOps",
						"version":           "1.0.0",
						"kind":              "product",
						"scope":             "workspace",
						"risk":              "standard",
						"status":            "inactive",
						"validationStatus":  "passed",
						"validationMessage": "ready",
						"healthStatus":      "healthy",
						"healthMessage":     "runtime drained",
					},
				},
			}
		case strings.Contains(req.Query, "mutation CLIUninstallExtension"):
			return map[string]any{
				"data": map[string]any{
					"uninstallExtension": true,
				},
			}
		default:
			t.Fatalf("unexpected query: %s", req.Query)
			return nil
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	exportPath := filepath.Join(t.TempDir(), "extension-removal.json")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "uninstall",
		"--id", "ext_123",
		"--deactivate",
		"--reason", "cleanup before uninstall",
		"--export-out", exportPath,
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload struct {
		ID          string `json:"id"`
		Deactivated bool   `json:"deactivated"`
		Uninstalled bool   `json:"uninstalled"`
		ExportOut   string `json:"exportOut"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ID != "ext_123" || !payload.Deactivated || !payload.Uninstalled {
		t.Fatalf("unexpected uninstall payload: %#v", payload)
	}
	if payload.ExportOut != exportPath {
		t.Fatalf("expected export path %q, got %q", exportPath, payload.ExportOut)
	}

	bundle, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read export bundle: %v", err)
	}
	if !strings.Contains(string(bundle), `"schemaCleanup"`) {
		t.Fatalf("expected schema cleanup guidance in export bundle, got %q", string(bundle))
	}
	if !strings.Contains(string(bundle), `DROP SCHEMA IF EXISTS ext_ats CASCADE;`) {
		t.Fatalf("expected suggested SQL in export bundle, got %q", string(bundle))
	}
	if requests != 4 {
		t.Fatalf("expected detail, artifact list, deactivate, and uninstall requests, got %d", requests)
	}
}

func TestRunExtensionsShowText(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		return map[string]any{
			"data": map[string]any{
				"extension": map[string]any{
					"id":                 "ext_123",
					"workspaceID":        "ws_123",
					"slug":               "ats",
					"name":               "Applicant Tracking",
					"publisher":          "DemandOps",
					"version":            "1.0.0",
					"description":        "ATS workspace pack",
					"kind":               "product",
					"scope":              "workspace",
					"risk":               "standard",
					"workspacePlan":      map[string]any{"mode": "provision_dedicated_workspace", "name": "People", "slug": "people", "description": "People operations"},
					"permissions":        []string{"case:read", "case:write"},
					"publicRoutes":       []map[string]any{{"pathPrefix": "/careers", "assetPath": "templates/careers"}},
					"adminRoutes":        []map[string]any{},
					"endpoints":          []map[string]any{{"name": "careers-site", "class": "public_page", "mountPath": "/careers", "methods": []string{"GET"}, "auth": "public", "contentTypes": []string{"text/html"}, "maxBodyBytes": 0, "rateLimitPerMinute": 60, "workspaceBinding": "none", "assetPath": "templates/careers/index.html", "serviceTarget": nil}},
					"events":             map[string]any{"publishes": []map[string]any{{"type": "candidate.reviewed", "description": "Candidate reviewed", "schemaVersion": 1}}, "subscribes": []string{"case.created"}},
					"commands":           []map[string]any{{"name": "ats.jobs.publish", "description": "Publish a job"}},
					"agentSkills":        []map[string]any{{"name": "publish-job", "description": "Guide an agent through publishing a job", "assetPath": "agent-skills/publish-job.md"}},
					"customizableAssets": []string{"templates/careers/index.html"},
					"status":             "active",
					"validationStatus":   "passed",
					"validationMessage":  "ready",
					"healthStatus":       "healthy",
					"healthMessage":      "ok",
					"bundleSHA256":       "abc123",
					"bundleSize":         2048,
					"installedByID":      "user_1",
					"installedAt":        "2026-03-13T10:00:00Z",
					"activatedAt":        "2026-03-13T10:05:00Z",
					"deactivatedAt":      nil,
					"validatedAt":        "2026-03-13T10:04:00Z",
					"lastHealthCheckAt":  "2026-03-13T10:06:00Z",
					"assets":             []map[string]any{{"id": "asset_1", "path": "templates/careers/index.html", "kind": "template", "contentType": "text/html", "isCustomizable": true, "checksum": "assetsha", "size": 256, "updatedAt": "2026-03-13T10:03:00Z"}},
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
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "slug:\tats") {
		t.Fatalf("expected slug in output, got %q", output)
	}
	if !strings.Contains(output, "publicRoutes:") || !strings.Contains(output, "/careers -> templates/careers") {
		t.Fatalf("expected public routes in output, got %q", output)
	}
	if !strings.Contains(output, "events:") || !strings.Contains(output, "publishes\tcandidate.reviewed\tv1") {
		t.Fatalf("expected events in output, got %q", output)
	}
	if !strings.Contains(output, "agentSkills:") || !strings.Contains(output, "publish-job\tGuide an agent through publishing a job\tagent-skills/publish-job.md") {
		t.Fatalf("expected agent skills in output, got %q", output)
	}
}

func TestRunExtensionsUpgradeJSON(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "mutation CLIUpgradeExtension") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		if got := req.Variables["id"]; got != "ext_123" {
			t.Fatalf("expected extension id ext_123, got %#v", got)
		}
		input, ok := req.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("expected input map, got %#v", req.Variables["input"])
		}
		if got := input["licenseToken"]; got != "lic_upgrade" {
			t.Fatalf("expected license token lic_upgrade, got %#v", got)
		}
		if got, ok := input["bundleBase64"].(string); !ok || strings.TrimSpace(got) == "" {
			t.Fatalf("expected non-empty bundleBase64, got %#v", input["bundleBase64"])
		}
		return map[string]any{
			"data": map[string]any{
				"upgradeExtension": map[string]any{
					"id":                "ext_123",
					"workspaceID":       "ws_123",
					"slug":              "ats",
					"name":              "Applicant Tracking",
					"publisher":         "DemandOps",
					"version":           "1.1.0",
					"kind":              "product",
					"scope":             "workspace",
					"risk":              "standard",
					"status":            "active",
					"validationStatus":  "valid",
					"validationMessage": "manifest and installed assets validated",
					"healthStatus":      "healthy",
					"healthMessage":     "extension active",
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	root := t.TempDir()
	manifest := `{
	  "slug": "ats",
	  "name": "Applicant Tracking",
	  "version": "1.1.0",
	  "publisher": "DemandOps",
	  "kind": "product",
	  "scope": "workspace",
	  "risk": "standard",
	  "runtimeClass": "bundle",
	  "storageClass": "shared_primitives_only"
	}`
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	assetsDir := filepath.Join(root, "assets", "templates", "careers")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "index.html"), []byte("<html>v2</html>"), 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "upgrade", root,
		"--id", "ext_123",
		"--license-token", "lic_upgrade",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload extensionOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.Version != "1.1.0" || payload.Status != "active" {
		t.Fatalf("unexpected upgrade payload: %#v", payload)
	}
}

func TestRunExtensionsInstallProvisionedWorkspaceJSON(t *testing.T) {
	previousCLIClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/graphql" {
					t.Fatalf("unexpected graphql path %q", r.URL.Path)
				}
				if cookie, err := r.Cookie("mbr_session"); err != nil || cookie.Value != "session_cli" {
					t.Fatalf("expected mbr_session cookie, got %v %#v", err, cookie)
				}

				var req testGraphQLRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if !strings.Contains(req.Query, "mutation CLIInstallExtension") {
					t.Fatalf("unexpected query: %s", req.Query)
				}
				input, ok := req.Variables["input"].(map[string]any)
				if !ok {
					t.Fatalf("expected input map, got %#v", req.Variables["input"])
				}
				if got := input["workspaceID"]; got != "ws_provisioned" {
					t.Fatalf("expected provisioned workspace id, got %#v", got)
				}
				if got := input["licenseToken"]; got != "lic_ats" {
					t.Fatalf("expected license token lic_ats, got %#v", got)
				}
				body, err := json.Marshal(map[string]any{
					"data": map[string]any{
						"installExtension": map[string]any{
							"id":                "ext_123",
							"workspaceID":       "ws_provisioned",
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
							"healthStatus":      "inactive",
							"healthMessage":     "extension installed but not active",
						},
					},
				})
				if err != nil {
					t.Fatalf("marshal response: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(bytes.NewReader(body)),
				}, nil
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() {
		newCLIClient = previousCLIClient
	})

	previousHTTPClient := newHTTPClient
	newHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/actions/workspaces" {
					t.Fatalf("unexpected workspace path %q", r.URL.Path)
				}
				if cookie, err := r.Cookie("mbr_session"); err != nil || cookie.Value != "session_cli" {
					t.Fatalf("expected mbr_session cookie on workspace request, got %v %#v", err, cookie)
				}
				body, err := json.Marshal(map[string]any{
					"ID":          "ws_provisioned",
					"Name":        "People",
					"Slug":        "people",
					"Description": "People operations",
				})
				if err != nil {
					t.Fatalf("marshal workspace response: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusCreated,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(bytes.NewReader(body)),
				}, nil
			}),
		}
	}
	t.Cleanup(func() {
		newHTTPClient = previousHTTPClient
	})

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	if _, err := cliapi.SaveStoredSessionConfig("https://app.mbr.test", "https://admin.mbr.test", "session_cli"); err != nil {
		t.Fatalf("SaveStoredSessionConfig returned error: %v", err)
	}

	root := t.TempDir()
	manifest := `{
	  "slug": "ats",
	  "name": "Applicant Tracking",
	  "version": "1.0.0",
	  "publisher": "DemandOps",
	  "kind": "product",
	  "scope": "workspace",
	  "risk": "standard",
	  "workspacePlan": {
	    "mode": "provision_dedicated_workspace",
	    "name": "People",
	    "slug": "people",
	    "description": "People operations"
	  }
	}`
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "install",
		root,
		"--license-token", "lic_ats",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload struct {
		Extension            extensionOutput `json:"extension"`
		ProvisionedWorkspace workspaceOutput `json:"provisionedWorkspace"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.Extension.WorkspaceID == nil || *payload.Extension.WorkspaceID != "ws_provisioned" {
		t.Fatalf("unexpected extension payload: %#v", payload.Extension)
	}
	if payload.ProvisionedWorkspace.ID != "ws_provisioned" || payload.ProvisionedWorkspace.ShortCode != "people" {
		t.Fatalf("unexpected provisioned workspace: %#v", payload.ProvisionedWorkspace)
	}
}

func TestRunExtensionsInstallPublicBundleWithoutLicenseToken(t *testing.T) {
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "mutation CLIInstallExtension") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		input, ok := req.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("expected input map, got %#v", req.Variables["input"])
		}
		if _, exists := input["licenseToken"]; exists {
			t.Fatalf("did not expect licenseToken for public bundle install, got %#v", input["licenseToken"])
		}
		return map[string]any{
			"data": map[string]any{
				"installExtension": map[string]any{
					"id":                "ext_public",
					"workspaceID":       "ws_public",
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
					"healthStatus":      "inactive",
					"healthMessage":     "extension installed but not active",
				},
			},
		}
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	root := t.TempDir()
	manifest := `{
	  "slug": "ats",
	  "name": "Applicant Tracking",
	  "version": "1.0.0",
	  "publisher": "DemandOps",
	  "kind": "product",
	  "scope": "workspace",
	  "risk": "standard",
	  "runtimeClass": "bundle",
	  "storageClass": "shared_primitives_only"
	}`
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "install",
		root,
		"--workspace", "ws_public",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload struct {
		Extension extensionOutput `json:"extension"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.Extension.ID != "ext_public" || payload.Extension.WorkspaceID == nil || *payload.Extension.WorkspaceID != "ws_public" {
		t.Fatalf("unexpected install payload: %#v", payload.Extension)
	}
}

func TestRunExtensionsDeployJSONUsesStoredWorkspaceContext(t *testing.T) {
	requestCount := 0
	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		requestCount++
		switch requestCount {
		case 1:
			if !strings.Contains(req.Query, "query CLIDeployExtensions") {
				t.Fatalf("unexpected list query: %s", req.Query)
			}
			if got := req.Variables["workspaceID"]; got != "ws_sandbox" {
				t.Fatalf("unexpected workspaceID %#v", got)
			}
			return map[string]any{
				"data": map[string]any{
					"extensions": []map[string]any{},
				},
			}
		case 2:
			if !strings.Contains(req.Query, "mutation CLIInstallExtension") {
				t.Fatalf("unexpected install query: %s", req.Query)
			}
			return map[string]any{
				"data": map[string]any{
					"installExtension": map[string]any{
						"id":               "ext_123",
						"workspaceID":      "ws_sandbox",
						"slug":             "ats",
						"name":             "Applicant Tracking",
						"publisher":        "DemandOps",
						"version":          "1.0.0",
						"kind":             "product",
						"scope":            "workspace",
						"risk":             "standard",
						"status":           "installed",
						"validationStatus": "pending",
						"healthStatus":     "inactive",
					},
				},
			}
		case 3:
			if !strings.Contains(req.Query, "validateExtension") {
				t.Fatalf("unexpected validate query: %s", req.Query)
			}
			return map[string]any{
				"data": map[string]any{
					"validateExtension": map[string]any{
						"id":               "ext_123",
						"workspaceID":      "ws_sandbox",
						"slug":             "ats",
						"name":             "Applicant Tracking",
						"publisher":        "DemandOps",
						"version":          "1.0.0",
						"kind":             "product",
						"scope":            "workspace",
						"risk":             "standard",
						"status":           "installed",
						"validationStatus": "valid",
						"healthStatus":     "inactive",
					},
				},
			}
		case 4:
			if !strings.Contains(req.Query, "activateExtension") {
				t.Fatalf("unexpected activate query: %s", req.Query)
			}
			return map[string]any{
				"data": map[string]any{
					"activateExtension": map[string]any{
						"id":               "ext_123",
						"workspaceID":      "ws_sandbox",
						"slug":             "ats",
						"name":             "Applicant Tracking",
						"publisher":        "DemandOps",
						"version":          "1.0.0",
						"kind":             "product",
						"scope":            "workspace",
						"risk":             "standard",
						"status":           "active",
						"validationStatus": "valid",
						"healthStatus":     "healthy",
					},
				},
			}
		case 5:
			if !strings.Contains(req.Query, "checkExtensionHealth") {
				t.Fatalf("unexpected health query: %s", req.Query)
			}
			return map[string]any{
				"data": map[string]any{
					"checkExtensionHealth": map[string]any{
						"id":               "ext_123",
						"workspaceID":      "ws_sandbox",
						"slug":             "ats",
						"name":             "Applicant Tracking",
						"publisher":        "DemandOps",
						"version":          "1.0.0",
						"kind":             "product",
						"scope":            "workspace",
						"risk":             "standard",
						"status":           "active",
						"validationStatus": "valid",
						"healthStatus":     "healthy",
					},
				},
			}
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil
		}
	})

	root := t.TempDir()
	manifest := `{
	  "slug": "ats",
	  "name": "Applicant Tracking",
	  "version": "1.0.0",
	  "publisher": "DemandOps",
	  "kind": "product",
	  "scope": "workspace",
	  "risk": "standard"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), []byte(manifest), 0o600))

	configPath := filepath.Join(t.TempDir(), "mbr-config.json")
	t.Setenv("MBR_CONFIG_PATH", configPath)
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")
	workspaceID := "ws_sandbox"
	_, err := cliapi.SaveStoredContext(&workspaceID, nil, false)
	require.NoError(t, err)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "deploy",
		root,
		"--license-token", "lic_ats",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload struct {
		Operation  string          `json:"operation"`
		Configured bool            `json:"configured"`
		Activated  bool            `json:"activated"`
		Monitored  bool            `json:"monitored"`
		Extension  extensionOutput `json:"extension"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.Operation != "install" || !payload.Activated || !payload.Monitored {
		t.Fatalf("unexpected deploy payload: %#v", payload)
	}
	if payload.Extension.ID != "ext_123" || payload.Extension.HealthStatus != "healthy" {
		t.Fatalf("unexpected extension payload: %#v", payload.Extension)
	}
}

func TestRunAttachmentsUploadJSON(t *testing.T) {
	previousCLIClient := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/attachments" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer hat_test" {
					t.Fatalf("unexpected auth header: %q", got)
				}
				if err := r.ParseMultipartForm(2 << 20); err != nil {
					t.Fatalf("parse multipart: %v", err)
				}
				if got := r.FormValue("workspace_id"); got != "ws_123" {
					t.Fatalf("unexpected workspace: %q", got)
				}
				if got := r.FormValue("description"); got != "Resume" {
					t.Fatalf("unexpected description: %q", got)
				}
				file, header, err := r.FormFile("file")
				if err != nil {
					t.Fatalf("form file: %v", err)
				}
				defer file.Close()
				body, err := io.ReadAll(file)
				if err != nil {
					t.Fatalf("read upload body: %v", err)
				}
				if header.Filename != "resume.pdf" {
					t.Fatalf("unexpected filename: %s", header.Filename)
				}
				if string(body) != "%PDF-1.4" {
					t.Fatalf("unexpected upload body: %q", string(body))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(strings.NewReader(`{
						"id":"att_123",
						"workspaceID":"ws_123",
						"caseID":"",
						"filename":"resume.pdf",
						"contentType":"application/pdf",
						"size":8,
						"status":"clean",
						"description":"Resume",
						"source":"agent"
					}`)),
				}, nil
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() {
		newCLIClient = previousCLIClient
	})

	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	path := filepath.Join(t.TempDir(), "resume.pdf")
	require.NoError(t, os.WriteFile(path, []byte("%PDF-1.4"), 0o600))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"attachments", "upload", path,
		"--workspace", "ws_123",
		"--description", "Resume",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload attachmentOutput
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
	assert.Equal(t, "att_123", payload.ID)
	assert.Equal(t, "resume.pdf", payload.Filename)
	assert.Equal(t, int64(8), payload.Size)
	assert.Equal(t, "clean", payload.Status)
}

func TestRunExtensionsSkillsShowText(t *testing.T) {
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIExtensionSkill") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"extension": map[string]any{
					"id":   "ext_123",
					"slug": "ats",
					"agentSkills": []map[string]any{
						{
							"name":        "publish-job",
							"description": "Guide an agent through publishing a job",
							"assetPath":   "agent-skills/publish-job.md",
						},
					},
					"assets": []map[string]any{
						{
							"path":        "agent-skills/publish-job.md",
							"contentType": "text/markdown",
							"textContent": "# Publish Job\n\nUse this skill.\n",
						},
					},
				},
			},
		}
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "skills", "show",
		"--id", "ext_123",
		"--name", "publish-job",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}
	if stdout.String() != "# Publish Job\n\nUse this skill.\n" {
		t.Fatalf("unexpected skill content %q", stdout.String())
	}
}

func TestRunExtensionsSkillsListJSON(t *testing.T) {
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIExtensionSkills") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"extension": map[string]any{
					"id":   "ext_123",
					"slug": "ats",
					"agentSkills": []map[string]any{
						{
							"name":        "publish-job",
							"description": "Guide an agent through publishing a job",
							"assetPath":   "agent-skills/publish-job.md",
						},
						{
							"name":        "review-candidates",
							"description": "Guide an agent through triaging candidates",
							"assetPath":   "agent-skills/review-candidates.md",
						},
					},
				},
			},
		}
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "skills", "list",
		"--id", "ext_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []extensionAgentSkillOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 2 || payload[0].Name != "publish-job" || payload[1].Name != "review-candidates" {
		t.Fatalf("unexpected skill payload: %#v", payload)
	}
}

func TestRunExtensionsSkillsListText(t *testing.T) {
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIExtensionSkills") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"extension": map[string]any{
					"id":   "ext_123",
					"slug": "ats",
					"agentSkills": []map[string]any{
						{
							"name":        "publish-job",
							"description": "Guide an agent through publishing a job",
							"assetPath":   "agent-skills/publish-job.md",
						},
					},
				},
			},
		}
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "skills", "list",
		"--id", "ext_123",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "publish-job\tGuide an agent through publishing a job\tagent-skills/publish-job.md") {
		t.Fatalf("unexpected output %q", stdout.String())
	}
}

func TestRunExtensionsEventsListJSON(t *testing.T) {
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIExtensionEventCatalog") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"extensionEventCatalog": []map[string]any{
					{
						"type":          "case.created",
						"description":   nil,
						"schemaVersion": 1,
						"core":          true,
						"publishers":    []string{"core"},
						"subscribers":   []string{"ats"},
					},
					{
						"type":          "ext.demandops.ats.application_received",
						"description":   "A candidate application was accepted.",
						"schemaVersion": 1,
						"core":          false,
						"publishers":    []string{"ats"},
						"subscribers":   []string{"review-bot"},
					},
				},
			},
		}
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "events", "list",
		"--workspace", "ws_123",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload []extensionRuntimeEventOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 2 || payload[0].Type != "case.created" || payload[1].Type != "ext.demandops.ats.application_received" {
		t.Fatalf("unexpected event payload: %#v", payload)
	}
}

func TestRunExtensionsSkillsShowJSON(t *testing.T) {
	t.Setenv("MBR_URL", "https://app.mbr.test")
	t.Setenv("MBR_TOKEN", "hat_test")

	withMockCLIClient(t, func(r *http.Request, req testGraphQLRequest) map[string]any {
		if !strings.Contains(req.Query, "query CLIExtensionSkill") {
			t.Fatalf("unexpected query: %s", req.Query)
		}
		return map[string]any{
			"data": map[string]any{
				"extension": map[string]any{
					"id":   "ext_123",
					"slug": "ats",
					"agentSkills": []map[string]any{
						{
							"name":        "review-candidates",
							"description": "Guide an agent through triaging candidates",
							"assetPath":   "agent-skills/review-candidates.md",
						},
					},
					"assets": []map[string]any{
						{
							"path":        "agent-skills/review-candidates.md",
							"contentType": "text/markdown",
							"textContent": "# Review Candidates\n",
						},
					},
				},
			},
		}
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"extensions", "skills", "show",
		"--id", "ext_123",
		"--name", "review-candidates",
		"--json",
	}, stdout, stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", exitCode, stderr.String())
	}

	var payload extensionSkillDetailOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ExtensionSlug != "ats" || payload.Name != "review-candidates" || payload.Content != "# Review Candidates\n" {
		t.Fatalf("unexpected skill payload: %#v", payload)
	}
}

func migrationVersion(path string) string {
	base := filepath.Base(path)
	parts := strings.SplitN(base, "_", 2)
	if len(parts) == 0 {
		return base
	}
	return parts[0]
}

type testGraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLHandler func(*http.Request, testGraphQLRequest) map[string]any

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withMockCLIClient(t *testing.T, handler graphQLHandler) {
	t.Helper()

	previous := newCLIClient
	newCLIClient = func(cfg cliapi.Config) *cliapi.Client {
		cfg.HTTPClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if got := r.Header.Get("Authorization"); got != "Bearer hat_test" {
					t.Fatalf("unexpected authorization header %q", got)
				}
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Fatalf("unexpected content-type %q", got)
				}

				var req testGraphQLRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("decode request: %v", err)
				}

				payload := handler(r, req)
				body, err := json.Marshal(payload)
				if err != nil {
					t.Fatalf("marshal response: %v", err)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(bytes.NewReader(body)),
				}, nil
			}),
		}
		return cliapi.NewClient(cfg)
	}
	t.Cleanup(func() {
		newCLIClient = previous
	})
}
