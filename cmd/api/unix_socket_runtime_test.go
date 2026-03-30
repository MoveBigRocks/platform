package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/movebigrocks/extension-sdk/runtimeproto"
	"github.com/stretchr/testify/require"
)

func newShortRuntimeDir(t *testing.T) string {
	t.Helper()

	runtimeDir := filepath.Join("/tmp", fmt.Sprintf("mbrsock-%d", time.Now().UnixNano()))
	require.NoError(t, os.MkdirAll(runtimeDir, 0o755))
	t.Cleanup(func() {
		_ = os.RemoveAll(runtimeDir)
	})
	return runtimeDir
}

func startUnixSocketTestServer(t *testing.T, runtimeDir, packageKey string, handler http.Handler) func() {
	t.Helper()

	socketPath := runtimeproto.SocketPath(runtimeDir, packageKey)
	require.NoError(t, os.MkdirAll(filepath.Dir(socketPath), 0o755))
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	server := &http.Server{
		Handler:           handler,
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
