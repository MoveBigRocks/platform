package refext

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	sqlstore "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
)

func InstallAndActivateReferenceExtension(t testing.TB, ctx context.Context, store stores.Store, workspaceID, extensionName string) *platformdomain.InstalledExtension {
	t.Helper()

	activated, err := EnsureReferenceExtensionActive(ctx, store, workspaceID, extensionName)
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
		return nil, fmt.Errorf("expected reference extension migrations in %s", root)
	}
	return migrations, nil
}

func referenceExtensionRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve reference extension root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "extensions", "first-party")), nil
}
