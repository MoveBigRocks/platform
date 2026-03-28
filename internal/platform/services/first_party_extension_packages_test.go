package platformservices_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	automationhandlers "github.com/movebigrocks/platform/internal/automation/handlers"
	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	sqlstore "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
	servicehandlers "github.com/movebigrocks/platform/internal/service/handlers"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/internal/testutil"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

type extensionTestOutbox struct {
	events []interface{}
}

func (o *extensionTestOutbox) Publish(ctx context.Context, stream eventbus.Stream, event interface{}) error {
	o.events = append(o.events, event)
	return nil
}

func (o *extensionTestOutbox) PublishEvent(ctx context.Context, stream eventbus.Stream, event eventbus.Event) error {
	o.events = append(o.events, event)
	return nil
}

func TestFirstPartyATSExtensionSeedsAndProcessesApplications(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	manifest, assets, migrations := loadTestExtensionPackage(t, "ats")
	service := newTestExtensionService(t, store)

	installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:   workspace.ID,
		InstalledByID: "user_123",
		LicenseToken:  "lic_ats",
		Manifest:      manifest,
		Assets:        assets,
		Migrations:    migrations,
	})
	require.NoError(t, err)

	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusActive, activated.Status)
	assert.Equal(t, platformdomain.ExtensionValidationValid, activated.ValidationStatus)

	if concrete, ok := store.(*sqlstore.Store); ok {
		registration, err := concrete.ExtensionRuntime().GetExtensionPackageRegistration(ctx, manifest.PackageKey())
		require.NoError(t, err)
		assert.Equal(t, platformdomain.ExtensionSchemaRegistrationReady, registration.Status)
		assert.Equal(t, "000001", registration.CurrentSchemaVersion)
	}

	form, err := store.Forms().GetFormBySlug(ctx, workspace.ID, "job-application")
	require.NoError(t, err)
	assert.True(t, form.AutoCreateCase)

	outbox := &extensionTestOutbox{}
	formService := automationservices.NewFormServiceWithDeps(store.Forms(), nil, store, store, outbox)
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		outbox,
		serviceapp.WithTransactionRunner(store),
	)
	formHandler := servicehandlers.NewFormEventHandler(formService, caseService, outbox, store, logger.NewNop())
	rulesEngine := automationservices.NewRulesEngine(
		automationservices.NewRuleService(store.Rules()),
		caseService,
		platformservices.NewContactService(store.Contacts()),
		store.Rules(),
		outbox,
	)
	t.Cleanup(rulesEngine.Stop)
	ruleHandler := automationhandlers.NewRuleEvaluationHandler(rulesEngine, caseService, logger.NewNop())

	submission := servicedomain.NewPublicFormSubmission(workspace.ID, form.ID, shareddomain.MetadataFromMap(map[string]interface{}{
		"full_name": "Candidate Example",
		"email":     "candidate@example.com",
		"role_slug": "backend-engineer",
	}))
	submission.SubmitterEmail = "candidate@example.com"
	submission.SubmitterName = "Candidate Example"

	event := contracts.NewFormSubmittedEvent(
		form.ID,
		form.Slug,
		submission.ID,
		workspace.ID,
		submission.SubmitterEmail,
		submission.SubmitterName,
		submission.Data.ToInterfaceMap(),
	)
	require.NoError(t, formService.CreatePublicSubmission(ctx, workspace.ID, submission, &event))

	eventData, err := json.Marshal(event)
	require.NoError(t, err)
	require.NoError(t, formHandler.HandleFormSubmitted(ctx, eventData))

	var caseCreated shareddomain.CaseCreated
	foundCaseCreated := false
	for _, published := range outbox.events {
		evt, ok := published.(shareddomain.CaseCreated)
		if ok {
			caseCreated = evt
			foundCaseCreated = true
		}
	}
	require.True(t, foundCaseCreated, "expected case.created event from auto-created ATS case")

	caseCreatedData, err := json.Marshal(caseCreated)
	require.NoError(t, err)
	require.NoError(t, ruleHandler.HandleCaseCreated(ctx, caseCreatedData))

	cases, total, err := store.Cases().ListCases(ctx, contracts.CaseFilters{
		WorkspaceID: workspace.ID,
		Limit:       10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, cases, 1)
	assert.Contains(t, cases[0].Tags, "ats")
	assert.Contains(t, cases[0].Tags, "candidate")
	assert.Contains(t, cases[0].Tags, "applied")
	assert.Contains(t, cases[0].Tags, "ats-review")
	formSlug, ok := cases[0].CustomFields.GetString("form_slug")
	require.True(t, ok)
	assert.Equal(t, "job-application", formSlug)
}

func TestFirstPartySalesPipelineExtensionInstallsAndActivates(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	manifest, assets, migrations := loadTestExtensionPackage(t, "sales-pipeline")
	service := newTestExtensionService(t, store)

	installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:   workspace.ID,
		InstalledByID: "user_123",
		LicenseToken:  "lic_sales",
		Manifest:      manifest,
		Assets:        assets,
		Migrations:    migrations,
	})
	require.NoError(t, err)

	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusActive, activated.Status)
	assert.Equal(t, platformdomain.ExtensionValidationValid, activated.ValidationStatus)

	form, err := store.Forms().GetFormBySlug(ctx, workspace.ID, "sales-deal-intake")
	require.NoError(t, err)
	assert.Equal(t, "sales-deal-intake", form.Slug)
	assert.True(t, form.AutoCreateCase)

	queue, err := store.Queues().GetQueueBySlug(ctx, workspace.ID, "sales-follow-up")
	require.NoError(t, err)
	assert.Equal(t, "Sales Follow-up", queue.Name)

	nav, err := service.ListWorkspaceAdminNavigation(ctx, workspace.ID)
	require.NoError(t, err)
	require.Len(t, nav, 1)
	assert.Equal(t, "/extensions/sales-pipeline", nav[0].Href)
}

func TestFirstPartyCommunityFeatureRequestsExtensionInstallsAndActivates(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	manifest, assets, migrations := loadTestExtensionPackage(t, "community-feature-requests")
	service := newTestExtensionService(t, store)

	installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:   workspace.ID,
		InstalledByID: "user_123",
		LicenseToken:  "lic_community",
		Manifest:      manifest,
		Assets:        assets,
		Migrations:    migrations,
	})
	require.NoError(t, err)

	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusActive, activated.Status)
	assert.Equal(t, platformdomain.ExtensionValidationValid, activated.ValidationStatus)

	queue, err := store.Queues().GetQueueBySlug(ctx, workspace.ID, "community-triage")
	require.NoError(t, err)
	assert.Equal(t, "Community Triage", queue.Name)

	nav, err := service.ListWorkspaceAdminNavigation(ctx, workspace.ID)
	require.NoError(t, err)
	require.Len(t, nav, 1)
	assert.Equal(t, "/extensions/community-feature-requests", nav[0].Href)
}

func TestExtensionServiceRejectsFormTriggeredCaseOnlyRule(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	manifest := platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "broken-ats",
		Name:          "Broken ATS",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindProduct,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		AutomationRules: []platformdomain.ExtensionAutomationSeed{
			{
				Key:   "ext.demandops.broken.form_tag",
				Title: "Broken form tag rule",
				Conditions: shareddomain.TypedSchemaFromMap(map[string]any{
					"operator": "and",
					"conditions": []any{
						map[string]any{
							"type":     "event",
							"field":    "trigger",
							"operator": "equals",
							"value":    "form_submitted",
						},
					},
				}),
				Actions: shareddomain.TypedSchemaFromMap(map[string]any{
					"actions": []any{
						map[string]any{
							"type":  "add_tag",
							"value": "review",
						},
					},
				}),
			},
		},
	}

	installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:  workspace.ID,
		LicenseToken: "lic_broken",
		Manifest:     manifest,
	})
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusFailed, installed.Status)
	assert.Equal(t, platformdomain.ExtensionValidationInvalid, installed.ValidationStatus)
	assert.Contains(t, installed.ValidationMessage, "requires case context")
}

func TestExtensionSDKTemplateInstallsAndActivates(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	manifest, assets := loadTestExtensionBundle(t, "sample-ops-pack")
	service := platformservices.NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
		WorkspaceID:   workspace.ID,
		InstalledByID: "user_123",
		LicenseToken:  "lic_template",
		Manifest:      manifest,
		Assets:        assets,
	})
	require.NoError(t, err)
	assert.Equal(t, "sample-ops-pack", installed.Slug)

	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusActive, activated.Status)
	assert.Equal(t, platformdomain.ExtensionValidationValid, activated.ValidationStatus)

	nav, err := service.ListWorkspaceAdminNavigation(ctx, workspace.ID)
	require.NoError(t, err)
	require.Len(t, nav, 1)
	assert.Equal(t, "/extensions/sample-ops-pack", nav[0].Href)
}

func TestEnterpriseAccessPackInstallsAsInstanceScopedPrivilegedPack(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	manifest, assets, migrations := loadTestExtensionPackage(t, "enterprise-access")
	service := newTestExtensionService(t, store)

	installed, err := service.InstallExtension(ctx, platformservices.InstallExtensionParams{
		InstalledByID: "user_123",
		LicenseToken:  "lic_enterprise_access",
		Manifest:      manifest,
		Assets:        assets,
		Migrations:    migrations,
	})
	require.NoError(t, err)
	assert.Equal(t, "enterprise-access", installed.Slug)
	assert.Equal(t, platformdomain.ExtensionScopeInstance, installed.Manifest.Scope)
	assert.Equal(t, platformdomain.ExtensionKindIdentity, installed.Manifest.Kind)
	assert.Equal(t, platformdomain.ExtensionRiskPrivileged, installed.Manifest.Risk)

	activated, err := service.ActivateExtension(ctx, installed.ID)
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionStatusActive, activated.Status)

	nav, err := service.ListInstanceAdminNavigation(ctx)
	require.NoError(t, err)
	require.Len(t, nav, 1)
	assert.Equal(t, "Identity", nav[0].Section)
	assert.Equal(t, "Enterprise Access", nav[0].Title)
	assert.Equal(t, "/extensions/enterprise-access", nav[0].Href)
}

func loadTestExtensionBundle(t *testing.T, slug string) (platformdomain.ExtensionManifest, []platformservices.ExtensionAssetInput) {
	t.Helper()

	return loadRepoExtensionBundle(t, slug)
}

func newTestExtensionService(t testing.TB, store stores.Store) *platformservices.ExtensionService {
	t.Helper()

	options := []platformservices.ExtensionServiceOption{}
	if concrete, ok := store.(*sqlstore.Store); ok {
		options = append(options, platformservices.WithExtensionSchemaRuntime(concrete.ExtensionSchemaMigrator()))
	}
	return platformservices.NewExtensionServiceWithOptions(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
		options...,
	)
}

func loadTestExtensionPackage(t *testing.T, slug string) (platformdomain.ExtensionManifest, []platformservices.ExtensionAssetInput, []platformservices.ExtensionMigrationInput) {
	t.Helper()

	return loadRepoExtensionPackage(t, slug)
}

func loadRepoExtensionBundle(t *testing.T, relDir string) (platformdomain.ExtensionManifest, []platformservices.ExtensionAssetInput) {
	t.Helper()

	manifest, assets, _ := loadRepoExtensionPackage(t, relDir)
	return manifest, assets
}

func loadRepoExtensionPackage(t *testing.T, slug string) (platformdomain.ExtensionManifest, []platformservices.ExtensionAssetInput, []platformservices.ExtensionMigrationInput) {
	t.Helper()

	baseDir := canonicalExtensionSourceDir(t, slug)
	manifestPath := filepath.Join(baseDir, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var manifest platformdomain.ExtensionManifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))

	assetsDir := filepath.Join(baseDir, "assets")
	var assets []platformservices.ExtensionAssetInput
	require.NoError(t, filepath.Walk(assetsDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(assetsDir, path)
		if err != nil {
			return err
		}
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		assets = append(assets, platformservices.ExtensionAssetInput{
			Path:        filepath.ToSlash(relPath),
			ContentType: contentType,
			Content:     content,
		})
		return nil
	}))

	migrationsDir := filepath.Join(baseDir, "migrations")
	var migrations []platformservices.ExtensionMigrationInput
	if entries, err := os.ReadDir(migrationsDir); err == nil {
		migrations = make([]platformservices.ExtensionMigrationInput, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
				continue
			}
			content, readErr := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
			require.NoError(t, readErr)
			migrations = append(migrations, platformservices.ExtensionMigrationInput{
				Path:    entry.Name(),
				Content: content,
			})
		}
		sort.Slice(migrations, func(i, j int) bool {
			return migrations[i].Path < migrations[j].Path
		})
	}

	return manifest, assets, migrations
}

func canonicalExtensionSourceDir(t *testing.T, slug string) string {
	t.Helper()

	repoRoot := platformRepoRoot(t)

	var rel string
	useWorkspaceRef := true
	switch slug {
	case "ats", "community-feature-requests", "error-tracking", "sales-pipeline", "web-analytics":
		rel = filepath.Join("extensions", slug)
	case "enterprise-access":
		useWorkspaceRef = false
		rel = filepath.Join("testdata", "first-party-packs", slug)
	case "sample-ops-pack":
		rel = "extension-sdk"
	default:
		t.Fatalf("unknown canonical extension source %q", slug)
	}

	if !useWorkspaceRef {
		dir := filepath.Join(repoRoot, rel)
		_, err := os.Stat(dir)
		require.NoError(t, err)
		return dir
	}

	dir, err := testutil.ResolveWorkspaceSiblingDir(repoRoot, rel)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("reference source checkout not available for %s", slug)
	}
	require.NoError(t, err)
	return dir
}

func platformRepoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}
