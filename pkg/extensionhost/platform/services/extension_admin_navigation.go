package platformservices

import (
	"context"
	"net/url"
	"strings"

	apierrors "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/errors"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

type ResolvedExtensionAdminNavigationItem struct {
	ExtensionID   string
	ExtensionSlug string
	WorkspaceID   string
	Section       string
	Title         string
	Icon          string
	Href          string
	ActivePage    string
}

type ResolvedExtensionDashboardWidget struct {
	ExtensionID   string
	ExtensionSlug string
	WorkspaceID   string
	Title         string
	Description   string
	Icon          string
	Href          string
}

func (s *ExtensionService) HasActiveExtensionInWorkspace(ctx context.Context, workspaceID, slug string) (bool, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	slug = strings.TrimSpace(slug)
	if workspaceID == "" || slug == "" {
		return false, apierrors.NewValidationErrors(
			apierrors.NewValidationError("workspace_id", "required"),
			apierrors.NewValidationError("slug", "required"),
		)
	}

	extension, err := s.extensionStore.GetInstalledExtensionBySlug(ctx, workspaceID, slug)
	if err == nil && extension != nil {
		return extension.Status == platformdomain.ExtensionStatusActive, nil
	}
	instanceExtension, err := s.extensionStore.GetInstanceExtensionBySlug(ctx, slug)
	if err != nil || instanceExtension == nil {
		return false, nil //nolint:nilerr // not found = not active
	}
	return instanceExtension.Status == platformdomain.ExtensionStatusActive, nil
}

func (s *ExtensionService) ListWorkspaceAdminNavigation(ctx context.Context, workspaceID string) ([]ResolvedExtensionAdminNavigationItem, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}

	installed, err := s.extensionsVisibleInWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, apierrors.DatabaseError("list workspace extensions", err)
	}

	result := make([]ResolvedExtensionAdminNavigationItem, 0)
	for _, extension := range activeExtensionsOnly(installed) {
		for _, item := range extension.Manifest.AdminNavigation {
			endpoint, ok := manifestAdminPageEndpoint(extension.Manifest, item.Endpoint)
			if !ok {
				continue
			}
			result = append(result, ResolvedExtensionAdminNavigationItem{
				ExtensionID:   extension.ID,
				ExtensionSlug: extension.Slug,
				WorkspaceID:   extension.WorkspaceID,
				Section:       item.Section,
				Title:         item.Title,
				Icon:          item.Icon,
				Href:          endpoint.MountPath,
				ActivePage:    item.ActivePage,
			})
		}
	}

	return result, nil
}

func (s *ExtensionService) ListWorkspaceDashboardWidgets(ctx context.Context, workspaceID string) ([]ResolvedExtensionDashboardWidget, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}

	installed, err := s.extensionsVisibleInWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, apierrors.DatabaseError("list workspace extensions", err)
	}

	result := make([]ResolvedExtensionDashboardWidget, 0)
	for _, extension := range activeExtensionsOnly(installed) {
		for _, widget := range extension.Manifest.DashboardWidgets {
			endpoint, ok := manifestAdminPageEndpoint(extension.Manifest, widget.Endpoint)
			if !ok {
				continue
			}
			result = append(result, ResolvedExtensionDashboardWidget{
				ExtensionID:   extension.ID,
				ExtensionSlug: extension.Slug,
				WorkspaceID:   extension.WorkspaceID,
				Title:         widget.Title,
				Description:   widget.Description,
				Icon:          widget.Icon,
				Href:          endpoint.MountPath,
			})
		}
	}

	return result, nil
}

func (s *ExtensionService) ListInstanceAdminNavigation(ctx context.Context) ([]ResolvedExtensionAdminNavigationItem, error) {
	if s.extensionStore == nil {
		return nil, nil
	}
	installed, err := s.allExtensions(ctx)
	if err != nil {
		return nil, apierrors.DatabaseError("list extensions for instance admin", err)
	}
	return buildResolvedAdminNavigation(activeExtensionsOnly(installed), true), nil
}

func (s *ExtensionService) ListInstanceDashboardWidgets(ctx context.Context) ([]ResolvedExtensionDashboardWidget, error) {
	if s.extensionStore == nil {
		return nil, nil
	}
	installed, err := s.allExtensions(ctx)
	if err != nil {
		return nil, apierrors.DatabaseError("list extensions for instance admin", err)
	}
	return buildResolvedDashboardWidgets(activeExtensionsOnly(installed), true), nil
}

func buildResolvedAdminNavigation(installed []*platformdomain.InstalledExtension, instanceAdmin bool) []ResolvedExtensionAdminNavigationItem {
	result := make([]ResolvedExtensionAdminNavigationItem, 0)
	for _, extension := range activeExtensionsOnly(installed) {
		for _, item := range extension.Manifest.AdminNavigation {
			endpoint, ok := manifestAdminPageEndpoint(extension.Manifest, item.Endpoint)
			if !ok {
				continue
			}
			result = append(result, ResolvedExtensionAdminNavigationItem{
				ExtensionID:   extension.ID,
				ExtensionSlug: extension.Slug,
				WorkspaceID:   extension.WorkspaceID,
				Section:       item.Section,
				Title:         item.Title,
				Icon:          item.Icon,
				Href:          resolvedAdminHref(endpoint.MountPath, extension.WorkspaceID, instanceAdmin),
				ActivePage:    item.ActivePage,
			})
		}
	}
	return result
}

func buildResolvedDashboardWidgets(installed []*platformdomain.InstalledExtension, instanceAdmin bool) []ResolvedExtensionDashboardWidget {
	result := make([]ResolvedExtensionDashboardWidget, 0)
	for _, extension := range activeExtensionsOnly(installed) {
		for _, widget := range extension.Manifest.DashboardWidgets {
			endpoint, ok := manifestAdminPageEndpoint(extension.Manifest, widget.Endpoint)
			if !ok {
				continue
			}
			result = append(result, ResolvedExtensionDashboardWidget{
				ExtensionID:   extension.ID,
				ExtensionSlug: extension.Slug,
				WorkspaceID:   extension.WorkspaceID,
				Title:         widget.Title,
				Description:   widget.Description,
				Icon:          widget.Icon,
				Href:          resolvedAdminHref(endpoint.MountPath, extension.WorkspaceID, instanceAdmin),
			})
		}
	}
	return result
}

func resolvedAdminHref(baseHref, workspaceID string, instanceAdmin bool) string {
	baseHref = strings.TrimSpace(baseHref)
	workspaceID = strings.TrimSpace(workspaceID)
	if !instanceAdmin || workspaceID == "" || baseHref == "" {
		return baseHref
	}

	parsed, err := url.Parse(baseHref)
	if err != nil {
		return baseHref
	}
	query := parsed.Query()
	if strings.TrimSpace(query.Get("workspace")) == "" {
		query.Set("workspace", workspaceID)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (s *ExtensionService) allExtensions(ctx context.Context) ([]*platformdomain.InstalledExtension, error) {
	return s.extensionStore.ListAllExtensions(ctx)
}

func (s *ExtensionService) extensionsVisibleInWorkspace(ctx context.Context, workspaceID string) ([]*platformdomain.InstalledExtension, error) {
	installed, err := s.extensionStore.ListWorkspaceExtensions(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	instanceInstalled, err := s.extensionStore.ListInstanceExtensions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*platformdomain.InstalledExtension, 0, len(installed)+len(instanceInstalled))
	result = append(result, installed...)
	result = append(result, instanceInstalled...)
	return result, nil
}

func manifestAdminPageEndpoint(manifest platformdomain.ExtensionManifest, endpointName string) (platformdomain.ExtensionEndpoint, bool) {
	for _, endpoint := range manifest.Endpoints {
		if endpoint.Name != endpointName {
			continue
		}
		if endpoint.Class != platformdomain.ExtensionEndpointClassAdminPage {
			return platformdomain.ExtensionEndpoint{}, false
		}
		return endpoint, true
	}
	return platformdomain.ExtensionEndpoint{}, false
}
