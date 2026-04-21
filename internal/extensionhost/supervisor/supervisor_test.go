package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain doubles as the fake runtime binary. When the test binary is
// re-exec'd with MBR_SUPERVISOR_TEST_HELPER=<mode> in its environment, it
// runs one of a handful of test-only behaviors instead of the normal test
// suite. This is the standard "exec yourself" pattern from os/exec tests.
func TestMain(m *testing.M) {
	mode := strings.TrimSpace(os.Getenv("MBR_SUPERVISOR_TEST_HELPER"))
	if mode == "" {
		os.Exit(m.Run())
	}
	runFakeRuntime(mode)
}

func runFakeRuntime(mode string) {
	socketPath := os.Getenv("MBR_EXTENSION_RUNTIME_SOCKET_PATH")
	switch mode {
	case "exit_success":
		_, _ = os.Stdout.WriteString("hello from fake runtime\n")
		os.Exit(0)
	case "exit_nonzero":
		_, _ = os.Stderr.WriteString("about to fail\n")
		os.Exit(7)
	case "run_forever":
		// Create a marker file showing the env we received, then block on
		// SIGTERM. We use a marker rather than stdout only so tests can
		// assert on spawn timing without racing the log pipeline.
		if marker := os.Getenv("MBR_SUPERVISOR_TEST_MARKER"); marker != "" {
			_ = os.WriteFile(marker, []byte(socketPath), 0o600)
		}
		_, _ = os.Stdout.WriteString("runtime up\n")
		// Block until SIGTERM / SIGKILL.
		stop := make(chan os.Signal, 1)
		signalNotify(stop)
		<-stop
		_, _ = os.Stdout.WriteString("runtime shutting down\n")
		os.Exit(0)
	case "ignore_sigterm":
		// Like run_forever but explicitly ignores SIGTERM so the supervisor
		// is forced to escalate to SIGKILL.
		signal.Ignore(syscall.SIGTERM)
		if marker := os.Getenv("MBR_SUPERVISOR_TEST_MARKER"); marker != "" {
			_ = os.WriteFile(marker, []byte("up"), 0o600)
		}
		time.Sleep(10 * time.Minute)
		os.Exit(0)
	case "count_then_crash":
		// Increments a counter in a file and exits non-zero. Used to prove
		// the supervisor restarts crashed children.
		counterPath := os.Getenv("MBR_SUPERVISOR_TEST_COUNTER")
		if counterPath != "" {
			appendMarker(counterPath)
		}
		os.Exit(3)
	default:
		os.Exit(99)
	}
}

func appendMarker(path string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString("x")
}

func testLogger(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return logger, &buf
}

// writeFakeRuntime copies the test binary into binDir under the given name
// so the supervisor can exec a known-good helper via the TestMain re-exec
// mechanism.
func writeFakeRuntime(t *testing.T, binDir, binaryName string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	src, err := os.Executable()
	require.NoError(t, err)
	dst := filepath.Join(binDir, binaryName)
	in, err := os.Open(src)
	require.NoError(t, err)
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	require.NoError(t, err)
	defer out.Close()
	_, err = io.Copy(out, in)
	require.NoError(t, err)
	return dst
}

func writeManifest(t *testing.T, path string, entries ...ManifestEntry) {
	t.Helper()
	m := Manifest{Runtimes: entries}
	data, err := json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

func newTestSupervisor(t *testing.T, extraEnv []string) (*Supervisor, string) {
	t.Helper()
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	socketDir := filepath.Join(root, "sockets")
	manifestPath := filepath.Join(root, "runtime-manifest.json")

	writeFakeRuntime(t, binDir, "helper-runtime")
	writeManifest(t, manifestPath, ManifestEntry{
		Slug:       "helper",
		PackageKey: "test/helper",
		Binary:     "helper-runtime",
		Service:    "helper-runtime",
		Socket:     "test_helper.sock",
	})

	logger, _ := testLogger(t)
	s, err := New(Config{
		ManifestPath:   manifestPath,
		BinaryDir:      binDir,
		SocketDir:      socketDir,
		StopTimeout:    2 * time.Second,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     40 * time.Millisecond,
		ExtraEnv:       extraEnv,
		Logger:         logger,
	})
	require.NoError(t, err)
	return s, root
}

func TestNewRejectsMissingManifest(t *testing.T) {
	t.Parallel()
	logger, _ := testLogger(t)
	_, err := New(Config{
		ManifestPath: filepath.Join(t.TempDir(), "absent.json"),
		BinaryDir:    t.TempDir(),
		SocketDir:    t.TempDir(),
		Logger:       logger,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read manifest")
}

func TestNewRejectsMissingBinary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifestPath := filepath.Join(root, "runtime-manifest.json")
	writeManifest(t, manifestPath, ManifestEntry{
		Slug:    "ghost",
		Binary:  "ghost-runtime",
		Socket:  "ghost.sock",
		Service: "ghost-runtime",
	})
	logger, _ := testLogger(t)
	_, err := New(Config{
		ManifestPath: manifestPath,
		BinaryDir:    filepath.Join(root, "bin"),
		SocketDir:    filepath.Join(root, "sockets"),
		Logger:       logger,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost-runtime")
}

func TestNewRejectsDuplicateSlug(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifestPath := filepath.Join(root, "runtime-manifest.json")
	binDir := filepath.Join(root, "bin")
	writeFakeRuntime(t, binDir, "helper-runtime")
	writeManifest(t, manifestPath,
		ManifestEntry{Slug: "dup", Binary: "helper-runtime", Socket: "a.sock"},
		ManifestEntry{Slug: "dup", Binary: "helper-runtime", Socket: "b.sock"},
	)
	logger, _ := testLogger(t)
	_, err := New(Config{
		ManifestPath: manifestPath,
		BinaryDir:    binDir,
		SocketDir:    filepath.Join(root, "sockets"),
		Logger:       logger,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestSupervisorSpawnsChildWithSocketEnv(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	marker := filepath.Join(root, "marker")
	extraEnv := []string{
		"MBR_SUPERVISOR_TEST_HELPER=run_forever",
		"MBR_SUPERVISOR_TEST_MARKER=" + marker,
	}
	s, _ := newTestSupervisor(t, extraEnv)

	require.NoError(t, s.Start(context.Background()))
	t.Cleanup(func() {
		_ = s.Stop(context.Background())
	})

	require.Eventually(t, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}, 15*time.Second, 50*time.Millisecond, "child did not write marker")

	got, err := os.ReadFile(marker)
	require.NoError(t, err)
	// Child wrote the socket path it received via env. Expect the path to
	// sit inside the supervisor's configured SocketDir and match the entry.
	assert.Contains(t, string(got), string(os.PathSeparator)+"test_helper.sock")
}

func TestSupervisorRestartsAfterCrash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	socketDir := filepath.Join(root, "sockets")
	manifestPath := filepath.Join(root, "runtime-manifest.json")
	counterPath := filepath.Join(root, "attempts")
	writeFakeRuntime(t, binDir, "flaky-runtime")
	writeManifest(t, manifestPath, ManifestEntry{
		Slug:    "flaky",
		Binary:  "flaky-runtime",
		Socket:  "flaky.sock",
		Service: "flaky-runtime",
	})
	logger, _ := testLogger(t)
	s, err := New(Config{
		ManifestPath:   manifestPath,
		BinaryDir:      binDir,
		SocketDir:      socketDir,
		StopTimeout:    1 * time.Second,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		ExtraEnv: []string{
			"MBR_SUPERVISOR_TEST_HELPER=count_then_crash",
			"MBR_SUPERVISOR_TEST_COUNTER=" + counterPath,
		},
		Logger: logger,
	})
	require.NoError(t, err)

	require.NoError(t, s.Start(context.Background()))
	t.Cleanup(func() { _ = s.Stop(context.Background()) })

	// The count_then_crash helper increments a counter in a file on each
	// spawn and exits non-zero. Wait for at least three spawns to prove the
	// supervisor is restarting.
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(counterPath)
		if err != nil {
			return false
		}
		return len(data) >= 3
	}, 15*time.Second, 50*time.Millisecond, "supervisor did not retry after crashes")
}

func TestSupervisorSignalsShutdownOnStop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	marker := filepath.Join(root, "marker")
	extraEnv := []string{
		"MBR_SUPERVISOR_TEST_HELPER=run_forever",
		"MBR_SUPERVISOR_TEST_MARKER=" + marker,
	}
	s, _ := newTestSupervisor(t, extraEnv)

	require.NoError(t, s.Start(context.Background()))
	require.Eventually(t, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}, 15*time.Second, 50*time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, s.Stop(stopCtx))

	// Child should now be gone. The supervisor records pid while a child is
	// alive; after Stop and drain it should be cleared.
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, state := range s.children {
		assert.Equal(t, 0, state.pid, "child %s still recorded as alive", state.entry.Slug)
	}
}

func TestSupervisorEscalatesToSigkill(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	marker := filepath.Join(root, "marker")
	binDir := filepath.Join(root, "bin")
	socketDir := filepath.Join(root, "sockets")
	manifestPath := filepath.Join(root, "runtime-manifest.json")
	writeFakeRuntime(t, binDir, "stuck-runtime")
	writeManifest(t, manifestPath, ManifestEntry{
		Slug: "stuck", Binary: "stuck-runtime", Socket: "stuck.sock", Service: "stuck-runtime",
	})
	logger, _ := testLogger(t)
	s, err := New(Config{
		ManifestPath:   manifestPath,
		BinaryDir:      binDir,
		SocketDir:      socketDir,
		StopTimeout:    300 * time.Millisecond,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
		ExtraEnv: []string{
			"MBR_SUPERVISOR_TEST_HELPER=ignore_sigterm",
			"MBR_SUPERVISOR_TEST_MARKER=" + marker,
		},
		Logger: logger,
	})
	require.NoError(t, err)

	require.NoError(t, s.Start(context.Background()))
	require.Eventually(t, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}, 15*time.Second, 50*time.Millisecond)

	// The child ignores SIGTERM, so Stop must fall through to SIGKILL and
	// still return cleanly within the caller's context.
	stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, s.Stop(stopCtx))
}

func TestStopIsIdempotent(t *testing.T) {
	t.Parallel()
	s, _ := newTestSupervisor(t, []string{"MBR_SUPERVISOR_TEST_HELPER=run_forever"})
	require.NoError(t, s.Start(context.Background()))
	assert.NoError(t, s.Stop(context.Background()))
	assert.NoError(t, s.Stop(context.Background()))
}

func TestStartIsNotReentrant(t *testing.T) {
	t.Parallel()
	s, _ := newTestSupervisor(t, []string{"MBR_SUPERVISOR_TEST_HELPER=run_forever"})
	require.NoError(t, s.Start(context.Background()))
	t.Cleanup(func() { _ = s.Stop(context.Background()) })
	err := s.Start(context.Background())
	assert.Error(t, err)
}

func signalNotify(ch chan os.Signal) {
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
}

// Unused reference to keep imports readable on platforms where only a subset
// of the helpers is exercised.
var _ = strconv.Itoa
