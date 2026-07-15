package extensionruntime

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/movebigrocks/extension-sdk/runtimeproto"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

func TestApplyRuntimeIdentityHeadersIncludesEffectiveExtensionConfig(t *testing.T) {
	defaultConfig := shareddomain.NewTypedCustomFields()
	defaultConfig.SetString("mode", "b2b")
	defaultConfig.SetBool("showTotals", true)

	installedConfig := shareddomain.NewTypedCustomFields()
	installedConfig.SetString("mode", "agency")

	extension := &platformdomain.InstalledExtension{
		ID:          "ext_123",
		WorkspaceID: "ws_123",
		Slug:        "sales-pipeline",
		Manifest: platformdomain.ExtensionManifest{
			Publisher:     "demandops",
			Slug:          "sales-pipeline",
			DefaultConfig: defaultConfig,
		},
		Config: installedConfig,
	}

	headers := http.Header{}
	applyRuntimeIdentityHeaders(headers, extension)

	if got := headers.Get(runtimeproto.HeaderExtensionID); got != "ext_123" {
		t.Fatalf("expected extension id header, got %q", got)
	}
	if got := headers.Get(runtimeproto.HeaderWorkspaceID); got != "ws_123" {
		t.Fatalf("expected workspace header, got %q", got)
	}

	var config map[string]any
	if err := json.Unmarshal([]byte(headers.Get(runtimeproto.HeaderExtensionConfigJSON)), &config); err != nil {
		t.Fatalf("decode config header: %v", err)
	}
	if got := config["mode"]; got != "agency" {
		t.Fatalf("expected installed config override, got %#v", got)
	}
	if got := config["showTotals"]; got != true {
		t.Fatalf("expected default config to remain present, got %#v", got)
	}
}

func TestApplyForwardedHeadersAdvertisesHostAPIBaseURL(t *testing.T) {
	// A proxied inbound request must carry the host-API base URL so the
	// extension's host client can reach /__mbr/host/v1 regardless of which
	// public, admin, or workspace domain the request arrived on.
	extension := &platformdomain.InstalledExtension{ID: "ext_123", WorkspaceID: "ws_123"}
	headers := http.Header{}
	applyForwardedHeaders(headers, extension, platformdomain.ExtensionEndpoint{}, nil, "", "https://app.test", "https://admin.test", "https://api.test")

	if got := headers.Get(runtimeproto.HeaderAPIBaseURL); got != "https://api.test" {
		t.Fatalf("expected host-API base URL header, got %q", got)
	}
}

func TestApplyForwardedHeadersClearsSessionWorkspaceForInstanceEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/extensions/error-tracking/applications/new", nil)
	ctx.Set("workspace_id", "ws_session")
	ctx.Set("workspace_name", "Session workspace")
	ctx.Set("workspace_slug", "session-workspace")
	workspaceID := "ws_session"
	workspaceName := "Session workspace"
	workspaceSlug := "session-workspace"
	ctx.Set("session", &platformdomain.Session{CurrentContext: platformdomain.Context{
		Type:          platformdomain.ContextTypeWorkspace,
		WorkspaceID:   &workspaceID,
		WorkspaceName: &workspaceName,
		WorkspaceSlug: &workspaceSlug,
		Role:          string(platformdomain.WorkspaceRoleAdmin),
	}})
	instanceRole := platformdomain.InstanceRoleAdmin
	ctx.Set("auth_context", &platformdomain.AuthContext{InstanceRole: &instanceRole})

	headers := http.Header{runtimeproto.HeaderWorkspaceID: []string{"spoofed"}}
	applyForwardedHeaders(
		headers,
		&platformdomain.InstalledExtension{ID: "ext_error_tracking", Slug: "error-tracking"},
		platformdomain.ExtensionEndpoint{WorkspaceBinding: platformdomain.ExtensionWorkspaceBindingInstanceScoped},
		ctx,
		"",
		"https://app.test",
		"https://admin.test",
		"https://api.test",
	)

	if got := headers.Get(runtimeproto.HeaderWorkspaceID); got != "" {
		t.Fatalf("expected instance endpoint to omit workspace header, got %q", got)
	}
	var forwarded map[string]any
	if err := json.Unmarshal([]byte(headers.Get(runtimeproto.HeaderSessionContextJSON)), &forwarded); err != nil {
		t.Fatalf("decode forwarded session context: %v", err)
	}
	if got := forwarded["type"]; got != string(platformdomain.ContextTypeInstance) {
		t.Fatalf("expected instance session context, got %#v", got)
	}
	if _, exists := forwarded["workspace_id"]; exists {
		t.Fatalf("expected instance session context to omit workspace_id, got %#v", forwarded)
	}
	if got := forwarded["role"]; got != string(platformdomain.InstanceRoleAdmin) {
		t.Fatalf("expected instance role, got %#v", got)
	}
}

func TestDoUnixSocketRequestKeepsBodyReadableUntilClose(t *testing.T) {
	runtimeDir, err := os.MkdirTemp("/tmp", "mbr-runtime-*")
	if err != nil {
		t.Fatalf("create runtime dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(runtimeDir)
	})
	socketPath := runtimeproto.SocketPath(runtimeDir, "demandops/web-analytics")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	})

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			if _, err := io.WriteString(w, "part-one|"); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(25 * time.Millisecond)
			_, _ = io.WriteString(w, "part-two")
		}),
	}
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = server.Close()
	})

	registry := &Registry{runtimeDir: runtimeDir}
	extension := &platformdomain.InstalledExtension{
		ID:          "ext_analytics",
		WorkspaceID: "ws_123",
		Slug:        "web-analytics",
		Manifest: platformdomain.ExtensionManifest{
			Publisher: "demandops",
			Slug:      "web-analytics",
		},
	}

	resp, err := registry.doUnixSocketRequest(context.Background(), extension, http.MethodGet, "/extensions/web-analytics", nil, http.Header{})
	if err != nil {
		t.Fatalf("do unix socket request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read proxied body: %v", err)
	}
	if got := string(body); got != "part-one|part-two" {
		t.Fatalf("expected full streamed body, got %q", got)
	}
}
