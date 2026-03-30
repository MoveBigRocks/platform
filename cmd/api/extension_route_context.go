package main

import (
	"strings"

	"github.com/gin-gonic/gin"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

func resolvedAdminRouteWorkspaceID(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	if workspaceID := strings.TrimSpace(ctx.GetString("workspace_id")); workspaceID != "" {
		return workspaceID
	}
	for _, key := range []string{"workspace", "workspace_id", "workspaceID"} {
		if workspaceID := strings.TrimSpace(ctx.Query(key)); workspaceID != "" {
			return workspaceID
		}
	}
	return ""
}

func applyResolvedExtensionWorkspaceContext(ctx *gin.Context, extension *platformdomain.InstalledExtension) {
	if ctx == nil || extension == nil {
		return
	}
	if strings.TrimSpace(ctx.GetString("workspace_id")) != "" {
		return
	}
	if workspaceID := strings.TrimSpace(extension.WorkspaceID); workspaceID != "" {
		ctx.Set("workspace_id", workspaceID)
	}
}
