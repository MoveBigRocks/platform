package refext

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	sqlstore "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	platformservices "github.com/movebigrocks/platform/pkg/extensionhost/platform/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

func InstallAndActivateReferenceExtension(t testing.TB, ctx context.Context, store stores.Store, workspaceID, extensionName string) *platformdomain.InstalledExtension {
	t.Helper()

	activated, err := EnsureReferenceExtensionActive(ctx, store, workspaceID, extensionName)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("reference extension checkout not available for %s", extensionName)
	}
	require.NoError(t, err)
	return activated
}

func EnsureReferenceExtensionActive(ctx context.Context, store stores.Store, workspaceID, extensionName string) (*platformdomain.InstalledExtension, error) {
	service := extensionServiceForStore(store)

	existing, err := store.Extensions().GetInstalledExtensionBySlug(ctx, workspaceID, extensionName)
	if err == nil && existing != nil {
		if existing.Status == platformdomain.ExtensionStatusActive {
			return existing, nil
		}
		return service.ActivateExtension(ctx, existing.ID)
	}

	params, err := loadReferenceExtensionInstallParams(extensionName, workspaceID)
	if err != nil {
		return nil, err
	}

	installed, err := service.InstallExtension(ctx, params)
	if err != nil {
		return nil, err
	}

	return service.ActivateExtension(ctx, installed.ID)
}

func extensionServiceForStore(store stores.Store) *platformservices.ExtensionService {
	if concrete, ok := store.(*sqlstore.Store); ok {
		return platformservices.NewExtensionServiceWithOptions(
			store.Extensions(),
			store.Workspaces(),
			store.Queues(),
			store.Forms(),
			store.Rules(),
			store,
			platformservices.WithExtensionSchemaRuntime(concrete.ExtensionSchemaMigrator()),
		)
	}

	return platformservices.NewExtensionService(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
	)
}

func loadReferenceExtensionInstallParams(extensionName, workspaceID string) (platformservices.InstallExtensionParams, error) {
	root, err := referenceExtensionRoot()
	if err != nil {
		return platformservices.InstallExtensionParams{}, err
	}
	root = filepath.Join(root, extensionName)

	manifestBytes, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return platformservices.InstallExtensionParams{}, err
	}

	var manifest platformdomain.ExtensionManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return platformservices.InstallExtensionParams{}, err
	}

	migrations, err := loadReferenceExtensionMigrations(filepath.Join(root, "migrations"))
	if err != nil {
		return platformservices.InstallExtensionParams{}, err
	}

	return platformservices.InstallExtensionParams{
		WorkspaceID:  workspaceID,
		LicenseToken: "lic_" + strings.ReplaceAll(extensionName, "-", "_"),
		BundleBase64: encodeReferenceExtensionBundle(manifest, migrations),
		Manifest:     manifest,
		Migrations:   migrations,
	}, nil
}

func loadReferenceExtensionMigrations(root string) ([]platformservices.ExtensionMigrationInput, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	migrations := make([]platformservices.ExtensionMigrationInput, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, platformservices.ExtensionMigrationInput{
			Path:    entry.Name(),
			Content: content,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Path < migrations[j].Path
	})
	if len(migrations) == 0 {
		return nil, fmt.Errorf("expected ATS extension migrations in %s", root)
	}
	return migrations, nil
}

func encodeReferenceExtensionBundle(manifest platformdomain.ExtensionManifest, migrations []platformservices.ExtensionMigrationInput) string {
	payload := struct {
		Manifest   platformdomain.ExtensionManifest `json:"manifest"`
		Migrations []struct {
			Path    string `json:"path"`
			Content string `json:"content,omitempty"`
		} `json:"migrations,omitempty"`
	}{
		Manifest: manifest,
		Migrations: make([]struct {
			Path    string `json:"path"`
			Content string `json:"content,omitempty"`
		}, 0, len(migrations)),
	}
	for _, migration := range migrations {
		payload.Migrations = append(payload.Migrations, struct {
			Path    string `json:"path"`
			Content string `json:"content,omitempty"`
		}{
			Path:    strings.TrimSpace(migration.Path),
			Content: base64.StdEncoding.EncodeToString(migration.Content),
		})
	}

	data, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("marshal reference extension bundle: %v", err))
	}
	return base64.StdEncoding.EncodeToString(data)
}

func referenceExtensionRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve ATS extension root")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	return testutil.ResolveWorkspaceSiblingDir(repoRoot, "extensions")
}
