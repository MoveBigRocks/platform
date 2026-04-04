package extensionruntime

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
	"github.com/movebigrocks/extension-sdk/runtimeproto"
	"github.com/stretchr/testify/require"
)

func newRuntimeTestDir(t *testing.T) string {
	t.Helper()

	runtimeDir := filepath.Join("/tmp", fmt.Sprintf("mbr-runtime-%d", time.Now().UnixNano()))
	require.NoError(t, os.MkdirAll(runtimeDir, 0o755))
	t.Cleanup(func() {
		_ = os.RemoveAll(runtimeDir)
	})
	return runtimeDir
}

func startRuntimeSocketServer(
	t *testing.T,
	runtimeDir string,
	packageKey string,
	registerRoutes func(*gin.Engine),
	consumers map[string]func(context.Context, []byte) error,
	jobs map[string]func(context.Context) error,
) func() {
	t.Helper()

	engine := gin.New()
	engine.Use(gin.Recovery())
	if len(consumers) > 0 || len(jobs) > 0 {
		runtimehttp.RegisterInternalRoutes(engine, consumers, jobs)
	}
	if registerRoutes != nil {
		registerRoutes(engine)
	}

	socketPath := runtimeproto.SocketPath(runtimeDir, packageKey)
	require.NoError(t, os.MkdirAll(filepath.Dir(socketPath), 0o755))
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	server := &http.Server{
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		_ = server.Serve(listener)
	}()

	return func() {
		_ = server.Close()
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}
}
