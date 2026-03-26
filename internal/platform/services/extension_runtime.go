package platformservices

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"slices"
	"strings"

	artifactservices "github.com/movebigrocks/platform/internal/artifacts/services"
	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

var (
	reservedPublicExtensionPaths = []string{
		"/auth",
		"/health",
		"/metrics",
		"/.well-known",
		"/static",
		"/sandbox",
	}
	reservedAdminExtensionPaths = []string{
		"/health",
		"/auth",
		"/workspaces",
		"/users",
		"/cases",
		"/issues",
		"/forms",
		"/rules",
		"/applications",
		"/analytics",
		"/static",
		"/login",
		"/logout",
		"/actions",
		"/graphql",
	}
)

type ResolvedExtensionAssetRoute struct {
	Extension   *platformdomain.InstalledExtension
	Asset       *platformdomain.ExtensionAsset
	MountPath   string
	RequestPath string
	Source      string
}

type ResolvedExtensionServiceRoute struct {
	Extension   *platformdomain.InstalledExtension
	Endpoint    platformdomain.ExtensionEndpoint
	RequestPath string
	RouteParams map[string]string
}

type extensionAssetRouteScope string

const (
	extensionAssetRouteScopePublic extensionAssetRouteScope = "public"
	extensionAssetRouteScopeAdmin  extensionAssetRouteScope = "admin"
)

type extensionServiceRouteScope string

const (
	extensionServiceRouteScopePublic extensionServiceRouteScope = "public"
	extensionServiceRouteScopeAdmin  extensionServiceRouteScope = "admin"
)

type extensionAssetCandidate struct {
	extension       *platformdomain.InstalledExtension
	mountPath       string
	assetPath       string
	artifactSurface string
	artifactPath    string
	scope           extensionAssetRouteScope
	source          string
	exactOnly       bool
}

func (s *ExtensionService) ResolvePublicAssetRoute(ctx context.Context, requestPath string) (*ResolvedExtensionAssetRoute, error) {
	return s.resolveAssetRoute(ctx, extensionAssetRouteScopePublic, "", requestPath)
}

func (s *ExtensionService) ResolveAdminAssetRoute(ctx context.Context, workspaceID, requestPath string) (*ResolvedExtensionAssetRoute, error) {
	return s.resolveAssetRoute(ctx, extensionAssetRouteScopeAdmin, strings.TrimSpace(workspaceID), requestPath)
}

func (s *ExtensionService) ResolvePublicServiceRoute(ctx context.Context, method, requestPath string) (*ResolvedExtensionServiceRoute, error) {
	return s.resolveServiceRoute(ctx, extensionServiceRouteScopePublic, "", method, requestPath)
}

func (s *ExtensionService) ResolveAdminServiceRoute(ctx context.Context, workspaceID, method, requestPath string) (*ResolvedExtensionServiceRoute, error) {
	return s.resolveServiceRoute(ctx, extensionServiceRouteScopeAdmin, strings.TrimSpace(workspaceID), method, requestPath)
}

func (s *ExtensionService) validateRuntimeTopology(ctx context.Context, extension *platformdomain.InstalledExtension) error {
	publicMounts, adminMounts := collectExtensionMounts(extension)
	if err := validateReservedMounts(publicMounts, reservedPublicExtensionPaths, "public"); err != nil {
		return err
	}
	if err := validateAdminMountNamespace(adminMounts); err != nil {
		return err
	}
	if err := validateReservedMounts(adminMounts, reservedAdminExtensionPaths, "admin"); err != nil {
		return err
	}

	if s.workspaceStore == nil || s.extensionStore == nil {
		return nil
	}

	workspaces, err := s.workspaceStore.ListWorkspaces(ctx)
	if err != nil {
		return apierrors.DatabaseError("list workspaces for extension topology validation", err)
	}

	for _, workspace := range workspaces {
		installed, err := s.extensionStore.ListWorkspaceExtensions(ctx, workspace.ID)
		if err != nil {
			return apierrors.DatabaseError("list workspace extensions for topology validation", err)
		}
		instanceInstalled, err := s.extensionStore.ListInstanceExtensions(ctx)
		if err != nil {
			return apierrors.DatabaseError("list instance extensions for topology validation", err)
		}
		installed = append(installed, instanceInstalled...)
		for _, other := range installed {
			if other == nil || other.ID == extension.ID || other.Status != platformdomain.ExtensionStatusActive {
				continue
			}

			otherPublic, otherAdmin := collectExtensionMounts(other)
			if overlaps, ok := findMountOverlap(publicMounts, otherPublic); ok && !sharesServiceBackedPackage(extension, other) {
				return fmt.Errorf("public extension path %s conflicts with active extension %s", overlaps, other.Slug)
			}
			if workspace.ID == extension.WorkspaceID {
				if overlaps, ok := findMountOverlap(adminMounts, otherAdmin); ok {
					return fmt.Errorf("admin extension path %s conflicts with active extension %s in workspace", overlaps, other.Slug)
				}
			}
		}
	}

	return nil
}

func sharesServiceBackedPackage(left, right *platformdomain.InstalledExtension) bool {
	if left == nil || right == nil {
		return false
	}
	if left.Manifest.RuntimeClass != platformdomain.ExtensionRuntimeClassServiceBacked ||
		right.Manifest.RuntimeClass != platformdomain.ExtensionRuntimeClassServiceBacked {
		return false
	}

	leftKey := left.Manifest.PackageKey()
	rightKey := right.Manifest.PackageKey()
	return leftKey != "" && leftKey == rightKey
}

func (s *ExtensionService) resolveAssetRoute(ctx context.Context, scope extensionAssetRouteScope, workspaceID, requestPath string) (*ResolvedExtensionAssetRoute, error) {
	if s.extensionStore == nil {
		return nil, nil
	}

	requestPath, ok := normalizeRequestPath(requestPath)
	if !ok {
		return nil, nil
	}

	extensions, err := s.listActiveExtensionsForScope(ctx, scope, workspaceID)
	if err != nil {
		return nil, err
	}

	candidates := buildAssetCandidates(scope, extensions)
	slices.SortFunc(candidates, func(a, b extensionAssetCandidate) int {
		if len(a.mountPath) != len(b.mountPath) {
			return len(b.mountPath) - len(a.mountPath)
		}
		if a.exactOnly != b.exactOnly {
			if a.exactOnly {
				return -1
			}
			return 1
		}
		return strings.Compare(a.source, b.source)
	})

	for _, candidate := range candidates {
		asset, err := s.resolveCandidateAsset(ctx, candidate, requestPath)
		if err != nil {
			return nil, err
		}
		if asset == nil {
			continue
		}
		return &ResolvedExtensionAssetRoute{
			Extension:   candidate.extension,
			Asset:       asset,
			MountPath:   candidate.mountPath,
			RequestPath: requestPath,
			Source:      candidate.source,
		}, nil
	}

	return nil, nil
}

func (s *ExtensionService) listActiveExtensionsForScope(ctx context.Context, scope extensionAssetRouteScope, workspaceID string) ([]*platformdomain.InstalledExtension, error) {
	if scope == extensionAssetRouteScopeAdmin {
		if strings.TrimSpace(workspaceID) == "" {
			installed, err := s.extensionStore.ListAllExtensions(ctx)
			if err != nil {
				return nil, apierrors.DatabaseError("list all extensions for instance admin", err)
			}
			return activeExtensionsOnly(installed), nil
		}
		installed, err := s.extensionsVisibleInWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, apierrors.DatabaseError("list workspace extensions", err)
		}
		return activeExtensionsOnly(installed), nil
	}

	if s.workspaceStore == nil {
		if s.extensionStore == nil {
			return nil, nil
		}
		installed, err := s.extensionStore.ListInstanceExtensions(ctx)
		if err != nil {
			return nil, apierrors.DatabaseError("list instance extensions", err)
		}
		return activeExtensionsOnly(installed), nil
	}
	workspaces, err := s.workspaceStore.ListWorkspaces(ctx)
	if err != nil {
		return nil, apierrors.DatabaseError("list workspaces", err)
	}

	result := make([]*platformdomain.InstalledExtension, 0)
	instanceInstalled, err := s.extensionStore.ListInstanceExtensions(ctx)
	if err != nil {
		return nil, apierrors.DatabaseError("list instance extensions", err)
	}
	result = append(result, activeExtensionsOnly(instanceInstalled)...)
	for _, workspace := range workspaces {
		installed, err := s.extensionStore.ListWorkspaceExtensions(ctx, workspace.ID)
		if err != nil {
			return nil, apierrors.DatabaseError("list workspace extensions", err)
		}
		result = append(result, activeExtensionsOnly(installed)...)
	}
	return result, nil
}

func (s *ExtensionService) listActiveExtensionsForServiceScope(ctx context.Context, scope extensionServiceRouteScope, workspaceID string) ([]*platformdomain.InstalledExtension, error) {
	if scope == extensionServiceRouteScopeAdmin {
		if strings.TrimSpace(workspaceID) == "" {
			installed, err := s.extensionStore.ListAllExtensions(ctx)
			if err != nil {
				return nil, apierrors.DatabaseError("list all extensions for instance admin", err)
			}
			return activeExtensionsOnly(installed), nil
		}
		installed, err := s.extensionsVisibleInWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, apierrors.DatabaseError("list workspace extensions", err)
		}
		return activeExtensionsOnly(installed), nil
	}
	if s.workspaceStore == nil {
		if s.extensionStore == nil {
			return nil, nil
		}
		installed, err := s.extensionStore.ListInstanceExtensions(ctx)
		if err != nil {
			return nil, apierrors.DatabaseError("list instance extensions", err)
		}
		return activeExtensionsOnly(installed), nil
	}
	workspaces, err := s.workspaceStore.ListWorkspaces(ctx)
	if err != nil {
		return nil, apierrors.DatabaseError("list workspaces", err)
	}
	result := make([]*platformdomain.InstalledExtension, 0)
	instanceInstalled, err := s.extensionStore.ListInstanceExtensions(ctx)
	if err != nil {
		return nil, apierrors.DatabaseError("list instance extensions", err)
	}
	result = append(result, activeExtensionsOnly(instanceInstalled)...)
	for _, workspace := range workspaces {
		installed, err := s.extensionStore.ListWorkspaceExtensions(ctx, workspace.ID)
		if err != nil {
			return nil, apierrors.DatabaseError("list workspace extensions", err)
		}
		result = append(result, activeExtensionsOnly(installed)...)
	}
	return result, nil
}

func activeExtensionsOnly(installed []*platformdomain.InstalledExtension) []*platformdomain.InstalledExtension {
	result := make([]*platformdomain.InstalledExtension, 0, len(installed))
	for _, extension := range installed {
		if extension == nil || extension.Status != platformdomain.ExtensionStatusActive {
			continue
		}
		result = append(result, extension)
	}
	return result
}

func buildAssetCandidates(scope extensionAssetRouteScope, extensions []*platformdomain.InstalledExtension) []extensionAssetCandidate {
	candidates := make([]extensionAssetCandidate, 0)
	for _, extension := range extensions {
		if extension == nil {
			continue
		}
		switch scope {
		case extensionAssetRouteScopePublic:
			for _, route := range extension.Manifest.PublicRoutes {
				candidates = append(candidates, extensionAssetCandidate{
					extension:       extension,
					mountPath:       route.PathPrefix,
					assetPath:       route.AssetPath,
					artifactSurface: route.ArtifactSurface,
					artifactPath:    route.ArtifactPath,
					scope:           scope,
					source:          "public_route",
				})
			}
		case extensionAssetRouteScopeAdmin:
			for _, route := range extension.Manifest.AdminRoutes {
				candidates = append(candidates, extensionAssetCandidate{
					extension:       extension,
					mountPath:       route.PathPrefix,
					assetPath:       route.AssetPath,
					artifactSurface: route.ArtifactSurface,
					artifactPath:    route.ArtifactPath,
					scope:           scope,
					source:          "admin_route",
				})
			}
		}

		for _, endpoint := range extension.Manifest.Endpoints {
			switch endpoint.Class {
			case platformdomain.ExtensionEndpointClassPublicPage:
				if scope != extensionAssetRouteScopePublic || (endpoint.AssetPath == "" && endpoint.ArtifactSurface == "") {
					continue
				}
				candidates = append(candidates, extensionAssetCandidate{
					extension:       extension,
					mountPath:       endpoint.MountPath,
					assetPath:       endpoint.AssetPath,
					artifactSurface: endpoint.ArtifactSurface,
					artifactPath:    endpoint.ArtifactPath,
					scope:           scope,
					source:          "public_page",
					exactOnly:       true,
				})
			case platformdomain.ExtensionEndpointClassPublicAsset:
				if scope != extensionAssetRouteScopePublic || (endpoint.AssetPath == "" && endpoint.ArtifactSurface == "") {
					continue
				}
				candidates = append(candidates, extensionAssetCandidate{
					extension:       extension,
					mountPath:       endpoint.MountPath,
					assetPath:       endpoint.AssetPath,
					artifactSurface: endpoint.ArtifactSurface,
					artifactPath:    endpoint.ArtifactPath,
					scope:           scope,
					source:          "public_asset",
				})
			case platformdomain.ExtensionEndpointClassAdminPage:
				if scope != extensionAssetRouteScopeAdmin || (endpoint.AssetPath == "" && endpoint.ArtifactSurface == "") {
					continue
				}
				candidates = append(candidates, extensionAssetCandidate{
					extension:       extension,
					mountPath:       endpoint.MountPath,
					assetPath:       endpoint.AssetPath,
					artifactSurface: endpoint.ArtifactSurface,
					artifactPath:    endpoint.ArtifactPath,
					scope:           scope,
					source:          "admin_page",
					exactOnly:       true,
				})
			}
		}
	}
	return candidates
}

func (s *ExtensionService) resolveServiceRoute(
	ctx context.Context,
	scope extensionServiceRouteScope,
	workspaceID, method, requestPath string,
) (*ResolvedExtensionServiceRoute, error) {
	if s.extensionStore == nil {
		return nil, nil
	}

	requestPath, ok := normalizeRequestPath(requestPath)
	if !ok {
		return nil, nil
	}
	method = strings.TrimSpace(strings.ToUpper(method))
	if method == "" {
		return nil, nil
	}

	extensions, err := s.listActiveExtensionsForServiceScope(ctx, scope, workspaceID)
	if err != nil {
		return nil, err
	}

	var best *ResolvedExtensionServiceRoute
	bestScore := -1
	for _, extension := range extensions {
		if extension == nil {
			continue
		}
		for _, endpoint := range extension.Manifest.Endpoints {
			if !isServiceEndpointInScope(scope, endpoint) {
				continue
			}
			if endpoint.ServiceTarget == "" {
				continue
			}
			params, matched := matchServiceEndpointPath(endpoint.MountPath, requestPath)
			if !matched {
				continue
			}
			if !isEndpointMethodAllowed(endpoint, method) {
				continue
			}
			score := serviceEndpointSpecificity(endpoint.MountPath)
			if best != nil && score <= bestScore {
				continue
			}
			best = &ResolvedExtensionServiceRoute{
				Extension:   extension,
				Endpoint:    endpoint,
				RequestPath: requestPath,
				RouteParams: params,
			}
			bestScore = score
		}
	}

	return best, nil
}

func (s *ExtensionService) resolveCandidateAsset(ctx context.Context, candidate extensionAssetCandidate, requestPath string) (*platformdomain.ExtensionAsset, error) {
	if !mountMatches(candidate.mountPath, requestPath) {
		return nil, nil
	}
	relativePath := relativeMountPath(candidate.mountPath, requestPath)
	if candidate.exactOnly && relativePath != "" {
		return nil, nil
	}

	if strings.TrimSpace(candidate.artifactSurface) != "" {
		return s.resolveCandidateManagedArtifact(ctx, candidate, relativePath)
	}
	for _, assetPath := range candidateAssetPaths(candidate.assetPath, relativePath, candidate.exactOnly) {
		asset, err := s.extensionStore.GetExtensionAsset(ctx, candidate.extension.ID, assetPath)
		if err == nil && asset != nil {
			return asset, nil
		}
		if err != nil && !errors.Is(err, apierrors.ErrNotFound) {
			return nil, apierrors.DatabaseError("get extension asset", err)
		}
	}

	return nil, nil
}

func (s *ExtensionService) resolveCandidateManagedArtifact(ctx context.Context, candidate extensionAssetCandidate, relativePath string) (*platformdomain.ExtensionAsset, error) {
	if s.artifacts == nil {
		return nil, nil
	}
	repository := extensionArtifactRepository(candidate.extension)
	for _, artifactPath := range candidateAssetPaths(candidate.artifactPath, relativePath, candidate.exactOnly) {
		fullPath := extensionArtifactPath(candidate.extension, candidate.artifactSurface, artifactPath)
		content, err := s.artifacts.Read(ctx, artifactservices.ReadParams{
			Repository:   repository,
			RelativePath: fullPath,
		})
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "read extension artifact")
		}
		asset, err := platformdomain.NewExtensionAsset(
			candidate.extension.ID,
			fullPath,
			inferManagedArtifactContentType(fullPath, content),
			content,
			false,
		)
		if err != nil {
			return nil, apierrors.Wrap(err, apierrors.ErrorTypeInternal, "build managed extension artifact response")
		}
		return asset, nil
	}
	return nil, nil
}

func inferManagedArtifactContentType(assetPath string, content []byte) string {
	if contentType := strings.TrimSpace(mime.TypeByExtension(path.Ext(assetPath))); contentType != "" {
		return contentType
	}
	return http.DetectContentType(content)
}

func collectExtensionMounts(extension *platformdomain.InstalledExtension) (public []string, admin []string) {
	public = make([]string, 0, len(extension.Manifest.PublicRoutes)+len(extension.Manifest.Endpoints))
	admin = make([]string, 0, len(extension.Manifest.AdminRoutes)+len(extension.Manifest.Endpoints))

	for _, route := range extension.Manifest.PublicRoutes {
		if route.PathPrefix != "" {
			public = append(public, route.PathPrefix)
		}
	}
	for _, route := range extension.Manifest.AdminRoutes {
		if route.PathPrefix != "" {
			admin = append(admin, route.PathPrefix)
		}
	}
	for _, endpoint := range extension.Manifest.Endpoints {
		switch endpoint.Class {
		case platformdomain.ExtensionEndpointClassPublicPage,
			platformdomain.ExtensionEndpointClassPublicAsset,
			platformdomain.ExtensionEndpointClassPublicIngest,
			platformdomain.ExtensionEndpointClassWebhook:
			if endpoint.MountPath != "" {
				public = append(public, endpoint.MountPath)
			}
		case platformdomain.ExtensionEndpointClassAdminPage,
			platformdomain.ExtensionEndpointClassAdminAction,
			platformdomain.ExtensionEndpointClassExtensionAPI,
			platformdomain.ExtensionEndpointClassHealth:
			if endpoint.MountPath != "" {
				admin = append(admin, endpoint.MountPath)
			}
		}
	}

	return dedupeMounts(public), dedupeMounts(admin)
}

func dedupeMounts(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(out, value) {
			continue
		}
		out = append(out, value)
	}
	return out
}

func validateReservedMounts(mounts, reserved []string, scope string) error {
	for _, mount := range mounts {
		if mount == "/" {
			return fmt.Errorf("%s extension path / conflicts with the reserved %s root", scope, scope)
		}
		for _, reservedMount := range reserved {
			if pathOverlaps(mount, reservedMount) {
				return fmt.Errorf("%s extension path %s conflicts with reserved %s path %s", scope, mount, scope, reservedMount)
			}
		}
	}
	return nil
}

func validateAdminMountNamespace(mounts []string) error {
	for _, mount := range mounts {
		if mount == "" {
			continue
		}
		if mount != "/extensions" && !strings.HasPrefix(mount, "/extensions/") {
			return fmt.Errorf("admin extension path %s must be mounted under /extensions", mount)
		}
	}
	return nil
}

func findMountOverlap(left, right []string) (string, bool) {
	for _, leftMount := range left {
		for _, rightMount := range right {
			if pathOverlaps(leftMount, rightMount) {
				if len(leftMount) >= len(rightMount) {
					return leftMount, true
				}
				return rightMount, true
			}
		}
	}
	return "", false
}

func pathOverlaps(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	if left == "/" || right == "/" {
		return true
	}
	return strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

func normalizeRequestPath(value string) (string, bool) {
	if strings.Contains(value, "..") {
		return "", false
	}
	clean := path.Clean("/" + strings.TrimSpace(value))
	if clean == "." {
		clean = "/"
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	if clean != "/" {
		clean = strings.TrimRight(clean, "/")
	}
	return clean, true
}

func mountMatches(mountPath, requestPath string) bool {
	if mountPath == "" {
		return false
	}
	if requestPath == mountPath {
		return true
	}
	return strings.HasPrefix(requestPath, mountPath+"/")
}

func relativeMountPath(mountPath, requestPath string) string {
	if requestPath == mountPath {
		return ""
	}
	trimmed := strings.TrimPrefix(requestPath, mountPath)
	trimmed = strings.TrimPrefix(trimmed, "/")
	trimmed = path.Clean("/" + trimmed)
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "." {
		return ""
	}
	return trimmed
}

func candidateAssetPaths(assetPath, relativePath string, exactOnly bool) []string {
	assetPath = strings.TrimSpace(assetPath)
	if assetPath == "" {
		return nil
	}
	if exactOnly {
		if relativePath != "" {
			return nil
		}
		if path.Ext(assetPath) != "" {
			return []string{assetPath}
		}
		return []string{
			path.Join(assetPath, "index.html"),
			path.Join(assetPath, "index.htm"),
			assetPath,
		}
	}

	if relativePath == "" {
		if path.Ext(assetPath) != "" {
			return []string{assetPath}
		}
		return []string{
			path.Join(assetPath, "index.html"),
			path.Join(assetPath, "index.htm"),
			assetPath,
		}
	}

	if path.Ext(assetPath) != "" {
		return nil
	}

	return []string{path.Join(assetPath, relativePath)}
}

func isServiceEndpointInScope(scope extensionServiceRouteScope, endpoint platformdomain.ExtensionEndpoint) bool {
	switch endpoint.Class {
	case platformdomain.ExtensionEndpointClassPublicPage,
		platformdomain.ExtensionEndpointClassPublicAsset,
		platformdomain.ExtensionEndpointClassPublicIngest,
		platformdomain.ExtensionEndpointClassWebhook:
		return scope == extensionServiceRouteScopePublic
	case platformdomain.ExtensionEndpointClassAdminPage,
		platformdomain.ExtensionEndpointClassAdminAction,
		platformdomain.ExtensionEndpointClassExtensionAPI,
		platformdomain.ExtensionEndpointClassHealth:
		return scope == extensionServiceRouteScopeAdmin
	default:
		return false
	}
}

func isEndpointMethodAllowed(endpoint platformdomain.ExtensionEndpoint, method string) bool {
	if len(endpoint.Methods) == 0 {
		switch endpoint.Class {
		case platformdomain.ExtensionEndpointClassPublicIngest,
			platformdomain.ExtensionEndpointClassWebhook,
			platformdomain.ExtensionEndpointClassAdminAction,
			platformdomain.ExtensionEndpointClassExtensionAPI:
			return method == httpMethodPost
		case platformdomain.ExtensionEndpointClassHealth:
			return method == httpMethodGet || method == httpMethodHead
		default:
			return method == httpMethodGet || method == httpMethodHead
		}
	}
	return slices.Contains(endpoint.Methods, method)
}

const (
	httpMethodGet  = "GET"
	httpMethodHead = "HEAD"
	httpMethodPost = "POST"
)

func matchServiceEndpointPath(pattern, requestPath string) (map[string]string, bool) {
	patternSegments := routeSegments(pattern)
	requestSegments := routeSegments(requestPath)
	if len(patternSegments) != len(requestSegments) {
		return nil, false
	}
	if len(patternSegments) == 0 {
		return map[string]string{}, true
	}

	params := make(map[string]string)
	for i := range patternSegments {
		patternSegment := patternSegments[i]
		requestSegment := requestSegments[i]
		if isRouteParamSegment(patternSegment) {
			params[strings.TrimPrefix(patternSegment, ":")] = requestSegment
			continue
		}
		if patternSegment != requestSegment {
			return nil, false
		}
	}

	return params, true
}

func serviceEndpointSpecificity(pattern string) int {
	segments := routeSegments(pattern)
	score := 0
	for _, segment := range segments {
		score += 2
		if !isRouteParamSegment(segment) {
			score += 10
		}
	}
	return score
}

func routeSegments(value string) []string {
	normalized, ok := normalizeRequestPath(value)
	if !ok || normalized == "/" {
		return nil
	}
	return strings.Split(strings.Trim(normalized, "/"), "/")
}

func isRouteParamSegment(value string) bool {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, ":") {
		return false
	}
	return len(strings.TrimPrefix(value, ":")) > 0
}
