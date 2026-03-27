package main

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

func TestResolvedAdminRouteWorkspaceIDUsesCurrentContextBeforeQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/extensions/sales-pipeline?workspace=ws_query", nil)
	ctx.Set("workspace_id", "ws_current")

	assert.Equal(t, "ws_current", resolvedAdminRouteWorkspaceID(ctx))
}

func TestResolvedAdminRouteWorkspaceIDFallsBackToQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/extensions/sales-pipeline?workspace=ws_query", nil)

	assert.Equal(t, "ws_query", resolvedAdminRouteWorkspaceID(ctx))
}

func TestApplyResolvedExtensionWorkspaceContextSetsWorkspaceID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	extension := &platformdomain.InstalledExtension{WorkspaceID: "ws_extension"}

	applyResolvedExtensionWorkspaceContext(ctx, extension)

	assert.Equal(t, "ws_extension", ctx.GetString("workspace_id"))
}
