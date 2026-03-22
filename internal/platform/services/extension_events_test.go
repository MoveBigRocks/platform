package platformservices

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestExtensionServiceActivateExtensionRejectsUnknownSubscribedEvent(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)
	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:   workspace.ID,
		InstalledByID: "user_123",
		LicenseToken:  "lic_unknown",
		Manifest: platformdomain.ExtensionManifest{
			SchemaVersion: 1,
			Slug:          "broken-subscriber",
			Name:          "Broken Subscriber",
			Version:       "1.0.0",
			Publisher:     "DemandOps",
			Kind:          platformdomain.ExtensionKindOperational,
			Scope:         platformdomain.ExtensionScopeWorkspace,
			Risk:          platformdomain.ExtensionRiskStandard,
			Events: platformdomain.ExtensionEventCatalog{
				Subscribes: []string{"ext.demandops.missing.event"},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, platformdomain.ExtensionValidationInvalid, installed.ValidationStatus)
	require.Contains(t, installed.ValidationMessage, "ext.demandops.missing.event")

	_, err = service.ActivateExtension(ctx, installed.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscribed event ext.demandops.missing.event")
}

func TestExtensionServiceListWorkspaceEventCatalogIncludesCoreAndActiveExtensionEvents(t *testing.T) {
	store, cleanup := testutil.SetupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workspace := testutil.NewIsolatedWorkspace(t)
	require.NoError(t, store.Workspaces().CreateWorkspace(ctx, workspace))

	service := NewExtensionService(store.Extensions(), store.Workspaces(), store.Queues(), store.Forms(), store.Rules(), store)

	publisher := installTestExtensionForEvents(t, ctx, service, workspace.ID, platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "publisher",
		Name:          "Publisher",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindOperational,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		Events: platformdomain.ExtensionEventCatalog{
			Publishes: []platformdomain.ExtensionEventDefinition{
				{Type: "ext.demandops.publisher.ready", Description: "Publisher ready", SchemaVersion: 1},
			},
		},
	}, "lic_pub")
	_, err := service.ActivateExtension(ctx, publisher.ID)
	require.NoError(t, err)

	subscriber := installTestExtensionForEvents(t, ctx, service, workspace.ID, platformdomain.ExtensionManifest{
		SchemaVersion: 1,
		Slug:          "subscriber",
		Name:          "Subscriber",
		Version:       "1.0.0",
		Publisher:     "DemandOps",
		Kind:          platformdomain.ExtensionKindOperational,
		Scope:         platformdomain.ExtensionScopeWorkspace,
		Risk:          platformdomain.ExtensionRiskStandard,
		Events: platformdomain.ExtensionEventCatalog{
			Subscribes: []string{"ext.demandops.publisher.ready", "case.created"},
		},
	}, "lic_sub")
	_, err = service.ActivateExtension(ctx, subscriber.ID)
	require.NoError(t, err)

	catalog, err := service.ListWorkspaceEventCatalog(ctx, workspace.ID)
	require.NoError(t, err)

	byType := map[string]platformdomain.ExtensionRuntimeEvent{}
	for _, event := range catalog {
		byType[event.Type] = event
	}

	coreEvent, ok := byType["case.created"]
	require.True(t, ok)
	assert.True(t, coreEvent.Core)
	assert.Equal(t, []string{"core"}, coreEvent.Publishers)
	assert.Equal(t, []string{"subscriber"}, coreEvent.Subscribers)

	published, ok := byType["ext.demandops.publisher.ready"]
	require.True(t, ok)
	assert.False(t, published.Core)
	assert.Equal(t, []string{"publisher"}, published.Publishers)
	assert.Equal(t, []string{"subscriber"}, published.Subscribers)
	assert.Equal(t, "Publisher ready", published.Description)
}

func installTestExtensionForEvents(
	t *testing.T,
	ctx context.Context,
	service *ExtensionService,
	workspaceID string,
	manifest platformdomain.ExtensionManifest,
	licenseToken string,
) *platformdomain.InstalledExtension {
	t.Helper()

	installed, err := service.InstallExtension(ctx, InstallExtensionParams{
		WorkspaceID:   workspaceID,
		InstalledByID: "user_123",
		LicenseToken:  strings.TrimSpace(licenseToken),
		Manifest:      manifest,
	})
	require.NoError(t, err)
	return installed
}
