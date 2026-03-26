package extensionsruntime

import (
	"path/filepath"
	"regexp"
	"strings"
)

const (
	HeaderInternalRequest       = "X-MBR-Internal-Extension-Request"
	HeaderUserID                = "X-MBR-User-ID"
	HeaderExtensionID           = "X-MBR-Extension-ID"
	HeaderExtensionSlug         = "X-MBR-Extension-Slug"
	HeaderExtensionPackageKey   = "X-MBR-Extension-Package-Key"
	HeaderWorkspaceID           = "X-MBR-Workspace-ID"
	HeaderUserName              = "X-MBR-User-Name"
	HeaderUserEmail             = "X-MBR-User-Email"
	HeaderSessionContextJSON    = "X-MBR-Session-Context-JSON"
	HeaderRouteParamsJSON       = "X-MBR-Route-Params-JSON"
	HeaderAdminExtensionNavJSON = "X-MBR-Admin-Extension-Nav-JSON"
	HeaderAdminWidgetsJSON      = "X-MBR-Admin-Extension-Widgets-JSON"
	HeaderShowAnalytics         = "X-MBR-Show-Analytics"
	HeaderShowErrorTracking     = "X-MBR-Show-Error-Tracking"
)

const (
	InternalConsumerPathPrefix = "/__mbr/runtime/consumers/"
	InternalJobPathPrefix      = "/__mbr/runtime/jobs/"
)

var unsafeSocketChars = regexp.MustCompile(`[^a-z0-9._-]+`)

func SocketPath(rootDir, packageKey string) string {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		rootDir = "./tmp/extensions"
	}
	return filepath.Join(rootDir, sanitizeSocketName(packageKey)+".sock")
}

func InternalConsumerPath(serviceTarget string) string {
	return InternalConsumerPathPrefix + sanitizePathSegment(serviceTarget)
}

func InternalJobPath(serviceTarget string) string {
	return InternalJobPathPrefix + sanitizePathSegment(serviceTarget)
}

func sanitizeSocketName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "/", "_")
	value = unsafeSocketChars.ReplaceAllString(value, "_")
	value = strings.Trim(value, "._-")
	if value == "" {
		return "extension"
	}
	return value
}

func sanitizePathSegment(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "/")
	return value
}
