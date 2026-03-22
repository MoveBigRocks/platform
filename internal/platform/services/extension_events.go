package platformservices

import (
	"context"
	"fmt"
	"slices"
	"strings"

	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/pkg/eventbus"
)

func (s *ExtensionService) ListWorkspaceEventCatalog(ctx context.Context, workspaceID string) ([]platformdomain.ExtensionRuntimeEvent, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if s.extensionStore == nil {
		return catalogEntriesFromMap(coreExtensionEventCatalog()), nil
	}

	installed, err := s.extensionsVisibleInWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, apierrors.DatabaseError("list workspace extensions for event catalog", err)
	}

	catalog := coreExtensionEventCatalog()
	for _, extension := range activeExtensionsOnly(installed) {
		addPublishedExtensionEvents(catalog, extension)
	}
	for _, extension := range activeExtensionsOnly(installed) {
		addSubscribedExtensionEvents(catalog, extension)
	}

	return catalogEntriesFromMap(catalog), nil
}

func (s *ExtensionService) validateEventSubscriptions(ctx context.Context, extension *platformdomain.InstalledExtension) error {
	if extension == nil || len(extension.Manifest.Events.Subscribes) == 0 {
		return nil
	}

	knownEvents := map[string]struct{}{}
	for _, eventType := range eventbus.RegisteredEventTypes() {
		knownEvents[eventType.String()] = struct{}{}
	}
	for _, published := range extension.Manifest.Events.Publishes {
		knownEvents[published.Type] = struct{}{}
	}

	if s.extensionStore != nil {
		var (
			installed []*platformdomain.InstalledExtension
			err       error
		)
		if strings.TrimSpace(extension.WorkspaceID) != "" {
			installed, err = s.extensionsVisibleInWorkspace(ctx, extension.WorkspaceID)
		} else {
			installed, err = s.extensionStore.ListInstanceExtensions(ctx)
		}
		if err != nil {
			return apierrors.DatabaseError("list workspace extensions for event validation", err)
		}
		for _, other := range activeExtensionsOnly(installed) {
			if other == nil || other.ID == extension.ID {
				continue
			}
			for _, published := range other.Manifest.Events.Publishes {
				knownEvents[published.Type] = struct{}{}
			}
		}
	}

	for _, eventType := range extension.Manifest.Events.Subscribes {
		if _, ok := knownEvents[eventType]; ok {
			continue
		}
		return fmt.Errorf("subscribed event %s is not published by core or an active extension in the workspace", eventType)
	}

	return nil
}

func coreExtensionEventCatalog() map[string]*platformdomain.ExtensionRuntimeEvent {
	catalog := make(map[string]*platformdomain.ExtensionRuntimeEvent, len(eventbus.RegisteredEventTypes()))
	for _, eventType := range eventbus.RegisteredEventTypes() {
		catalog[eventType.String()] = &platformdomain.ExtensionRuntimeEvent{
			Type:          eventType.String(),
			SchemaVersion: eventType.Version(),
			Core:          true,
			Publishers:    []string{"core"},
			Subscribers:   []string{},
		}
	}
	return catalog
}

func addPublishedExtensionEvents(catalog map[string]*platformdomain.ExtensionRuntimeEvent, extension *platformdomain.InstalledExtension) {
	if extension == nil {
		return
	}
	for _, published := range extension.Manifest.Events.Publishes {
		entry, ok := catalog[published.Type]
		if !ok {
			entry = &platformdomain.ExtensionRuntimeEvent{
				Type:          published.Type,
				SchemaVersion: max(published.SchemaVersion, 1),
				Core:          false,
				Publishers:    []string{},
				Subscribers:   []string{},
			}
			catalog[published.Type] = entry
		}
		if strings.TrimSpace(entry.Description) == "" {
			entry.Description = published.Description
		}
		if published.SchemaVersion > entry.SchemaVersion {
			entry.SchemaVersion = published.SchemaVersion
		}
		entry.Publishers = appendUniqueSorted(entry.Publishers, extension.Slug)
	}
}

func addSubscribedExtensionEvents(catalog map[string]*platformdomain.ExtensionRuntimeEvent, extension *platformdomain.InstalledExtension) {
	if extension == nil {
		return
	}
	for _, eventType := range extension.Manifest.Events.Subscribes {
		entry, ok := catalog[eventType]
		if !ok {
			entry = &platformdomain.ExtensionRuntimeEvent{
				Type:          eventType,
				SchemaVersion: 1,
				Publishers:    []string{},
				Subscribers:   []string{},
			}
			catalog[eventType] = entry
		}
		entry.Subscribers = appendUniqueSorted(entry.Subscribers, extension.Slug)
	}
}

func catalogEntriesFromMap(catalog map[string]*platformdomain.ExtensionRuntimeEvent) []platformdomain.ExtensionRuntimeEvent {
	result := make([]platformdomain.ExtensionRuntimeEvent, 0, len(catalog))
	for _, entry := range catalog {
		if entry == nil {
			continue
		}
		result = append(result, *entry)
	}
	slices.SortFunc(result, func(left, right platformdomain.ExtensionRuntimeEvent) int {
		return strings.Compare(left.Type, right.Type)
	})
	return result
}

func appendUniqueSorted(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	if slices.Contains(items, value) {
		return items
	}
	items = append(items, value)
	slices.Sort(items)
	return items
}
