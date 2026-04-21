// Package supervisor owns the lifecycle of extension runtime processes for
// the running core slot. One Supervisor instance reads a runtime manifest
// (see RFC-0016) and, for each entry, spawns the declared binary as a
// supervised child process. The supervisor restarts crashed children with
// exponential backoff, forwards their stdout/stderr into the core's
// structured logger, and cleanly SIGTERMs/SIGKILLs them on shutdown. This
// replaces the previous per-extension systemd unit pattern.
package supervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Manifest is the on-disk shape produced by cmd/reconcile-extensions.
// Duplicated here (rather than imported from internal/extensionhost/reconcile)
// to keep this package a narrow lifecycle layer with no cross-package pull.
type Manifest struct {
	Runtimes []ManifestEntry `json:"runtimes"`
}

// ManifestEntry describes a single extension runtime the supervisor must run.
// Only Slug, Binary, and Socket are load-bearing; the rest are retained for
// symmetry with the reconcile output and for logging.
type ManifestEntry struct {
	Slug       string `json:"slug"`
	PackageKey string `json:"packageKey"`
	Artifact   string `json:"artifact"`
	Binary     string `json:"binary"`
	Service    string `json:"service"`
	Socket     string `json:"socket"`
}

// Config configures a Supervisor. All paths are absolute on the host.
type Config struct {
	// ManifestPath is the path to runtime-manifest.json. Must exist and parse
	// at Start time.
	ManifestPath string

	// BinaryDir is the directory containing extension runtime binaries. The
	// supervisor joins ManifestEntry.Binary onto this path.
	BinaryDir string

	// SocketDir is the directory in which each runtime's unix socket is
	// created. Typically slot-scoped (e.g. /opt/mbr/tmp/extensions/blue).
	SocketDir string

	// StopTimeout is how long the supervisor waits for a child to exit after
	// SIGTERM before sending SIGKILL. Defaults to 10 seconds.
	StopTimeout time.Duration

	// InitialBackoff is the wait before the first restart attempt after a
	// crash. Defaults to 2 seconds.
	InitialBackoff time.Duration

	// MaxBackoff is the upper bound for exponential restart backoff. Defaults
	// to 60 seconds.
	MaxBackoff time.Duration

	// ExtraEnv is appended to each child's environment. The supervisor also
	// sets MBR_EXTENSION_RUNTIME_SOCKET_PATH per child.
	ExtraEnv []string

	// Logger is used for all supervisor and forwarded-child logging. Required.
	Logger *slog.Logger
}

// Supervisor owns a set of child processes for the runtimes declared in
// Manifest. Create with New, drive lifecycle with Start and Stop.
type Supervisor struct {
	cfg      Config
	manifest Manifest

	mu       sync.Mutex
	children map[string]*childState
	started  bool
	stopped  bool
	wg       sync.WaitGroup
	cancel   context.CancelFunc
}

type childState struct {
	entry ManifestEntry
	// cmd and pid are only meaningful while a process is running; they are
	// cleared between retries. Protected by Supervisor.mu.
	cmd *exec.Cmd
	pid int
}

// New loads the runtime manifest from cfg.ManifestPath, validates required
// fields on each entry, and returns a ready-to-Start Supervisor. Returns an
// error if the manifest is missing, unparseable, or references a binary that
// does not exist in BinaryDir.
func New(cfg Config) (*Supervisor, error) {
	if cfg.Logger == nil {
		return nil, errors.New("supervisor: Logger is required")
	}
	if strings.TrimSpace(cfg.ManifestPath) == "" {
		return nil, errors.New("supervisor: ManifestPath is required")
	}
	if strings.TrimSpace(cfg.BinaryDir) == "" {
		return nil, errors.New("supervisor: BinaryDir is required")
	}
	if strings.TrimSpace(cfg.SocketDir) == "" {
		return nil, errors.New("supervisor: SocketDir is required")
	}

	manifest, err := readManifest(cfg.ManifestPath)
	if err != nil {
		return nil, err
	}

	seenSlugs := make(map[string]struct{}, len(manifest.Runtimes))
	for i, entry := range manifest.Runtimes {
		if strings.TrimSpace(entry.Slug) == "" {
			return nil, fmt.Errorf("supervisor: runtime[%d] missing slug", i)
		}
		if strings.TrimSpace(entry.Binary) == "" {
			return nil, fmt.Errorf("supervisor: runtime %q missing binary", entry.Slug)
		}
		if strings.TrimSpace(entry.Socket) == "" {
			return nil, fmt.Errorf("supervisor: runtime %q missing socket", entry.Slug)
		}
		if _, dup := seenSlugs[entry.Slug]; dup {
			return nil, fmt.Errorf("supervisor: duplicate runtime slug %q", entry.Slug)
		}
		seenSlugs[entry.Slug] = struct{}{}
		binaryPath := filepath.Join(cfg.BinaryDir, entry.Binary)
		if _, err := os.Stat(binaryPath); err != nil {
			return nil, fmt.Errorf("supervisor: runtime %q binary %s: %w", entry.Slug, binaryPath, err)
		}
	}

	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = 10 * time.Second
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 2 * time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 60 * time.Second
	}

	return &Supervisor{
		cfg:      cfg,
		manifest: manifest,
		children: make(map[string]*childState, len(manifest.Runtimes)),
	}, nil
}

// Start spawns each runtime entry as a supervised child. Safe to call once;
// calling twice returns an error. The returned context-scoped goroutines
// exit on Stop.
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("supervisor: already started")
	}
	if s.stopped {
		s.mu.Unlock()
		return errors.New("supervisor: cannot start after Stop")
	}

	runCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.started = true

	if err := os.MkdirAll(s.cfg.SocketDir, 0o755); err != nil {
		s.started = false
		s.cancel = nil
		cancel()
		s.mu.Unlock()
		return fmt.Errorf("supervisor: create socket dir %s: %w", s.cfg.SocketDir, err)
	}

	for _, entry := range s.manifest.Runtimes {
		state := &childState{entry: entry}
		s.children[entry.Slug] = state
		s.wg.Add(1)
		go s.superviseChild(runCtx, state)
	}
	s.mu.Unlock()

	// Honor the caller's context as an additional shutdown signal. If the
	// caller cancels before Stop is invoked, we stop on their behalf.
	go func() {
		select {
		case <-ctx.Done():
			_ = s.Stop(context.Background())
		case <-runCtx.Done():
		}
	}()

	s.cfg.Logger.Info("extension runtime supervisor started",
		"runtimes", len(s.manifest.Runtimes),
		"socketDir", s.cfg.SocketDir,
	)
	return nil
}

// Stop signals every supervised child to exit, waits up to StopTimeout for
// graceful termination, then SIGKILLs the stragglers and waits for their
// goroutines to drain. Safe to call multiple times; subsequent calls are
// no-ops. ctx can be used to bound the overall wait.
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.stopped = true
		s.mu.Unlock()
		return nil
	}
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	cancel := s.cancel
	s.cancel = nil
	running := make([]*childState, 0, len(s.children))
	for _, state := range s.children {
		running = append(running, state)
	}
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	// Phase 1: polite SIGTERM to every live child.
	for _, state := range running {
		s.mu.Lock()
		cmd := state.cmd
		s.mu.Unlock()
		if cmd == nil || cmd.Process == nil {
			continue
		}
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
			s.cfg.Logger.Warn("extension runtime SIGTERM failed",
				"slug", state.entry.Slug, "error", err)
		}
	}

	// Phase 2: wait for supervise goroutines to drain, bounded by
	// StopTimeout and by the caller's context.
	drained := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(drained)
	}()

	select {
	case <-drained:
		s.cfg.Logger.Info("extension runtime supervisor stopped")
		return nil
	case <-time.After(s.cfg.StopTimeout):
		s.cfg.Logger.Warn("extension runtime supervisor stop grace exceeded, killing stragglers",
			"grace", s.cfg.StopTimeout)
	case <-ctx.Done():
		s.cfg.Logger.Warn("extension runtime supervisor stop context canceled, killing stragglers",
			"error", ctx.Err())
	}

	// Phase 3: SIGKILL anything still alive.
	for _, state := range running {
		s.mu.Lock()
		cmd := state.cmd
		s.mu.Unlock()
		if cmd == nil || cmd.Process == nil {
			continue
		}
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			s.cfg.Logger.Warn("extension runtime SIGKILL failed",
				"slug", state.entry.Slug, "error", err)
		}
	}

	// Wait unconditionally for goroutines to drain after kill; bound by ctx.
	select {
	case <-drained:
		s.cfg.Logger.Info("extension runtime supervisor stopped (after SIGKILL)")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// superviseChild owns the spawn/wait/restart loop for a single runtime.
// Exits when the supervisor-wide context is canceled (via Stop).
func (s *Supervisor) superviseChild(ctx context.Context, state *childState) {
	defer s.wg.Done()

	backoff := s.cfg.InitialBackoff
	attempt := 0

	for {
		if ctx.Err() != nil {
			return
		}
		attempt++
		exited := s.runOnce(ctx, state, attempt)
		if ctx.Err() != nil {
			return
		}
		if exited == nil {
			// Clean exit (code 0) while the supervisor is still running. Treat
			// the same as a crash — runtimes are supposed to keep running until
			// Stop is called.
			s.cfg.Logger.Warn("extension runtime exited unexpectedly with code 0; restarting",
				"slug", state.entry.Slug, "attempt", attempt)
		} else {
			s.cfg.Logger.Warn("extension runtime exited; restarting after backoff",
				"slug", state.entry.Slug, "attempt", attempt,
				"error", exited.Error(), "backoff", backoff)
		}

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		backoff = nextBackoff(backoff, s.cfg.MaxBackoff)
	}
}

// runOnce spawns the child, waits for it to exit, and returns the exit error
// (nil on a clean exit code 0, *exec.ExitError on non-zero, or another
// wrapping error on Start failure). The child's process handle is registered
// on the state while alive so Stop can signal it.
func (s *Supervisor) runOnce(ctx context.Context, state *childState, attempt int) error {
	binaryPath := filepath.Join(s.cfg.BinaryDir, state.entry.Binary)
	socketPath := filepath.Join(s.cfg.SocketDir, state.entry.Socket)

	cmd := exec.Command(binaryPath)
	cmd.Env = append(append([]string{}, os.Environ()...), s.cfg.ExtraEnv...)
	cmd.Env = append(cmd.Env, "MBR_EXTENSION_RUNTIME_SOCKET_PATH="+socketPath)
	applyPdeathsig(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	s.mu.Lock()
	state.cmd = cmd
	state.pid = cmd.Process.Pid
	s.mu.Unlock()

	s.cfg.Logger.Info("extension runtime spawned",
		"slug", state.entry.Slug,
		"pid", cmd.Process.Pid,
		"socket", socketPath,
		"attempt", attempt,
	)

	// Forward stdout/stderr into the supervisor logger. Each reader runs in
	// its own goroutine and exits when the pipe closes on process exit.
	var streamWG sync.WaitGroup
	streamWG.Add(2)
	go s.forwardStream(&streamWG, stdout, state.entry.Slug, "stdout")
	go s.forwardStream(&streamWG, stderr, state.entry.Slug, "stderr")

	waitErr := cmd.Wait()
	streamWG.Wait()

	s.mu.Lock()
	state.cmd = nil
	state.pid = 0
	s.mu.Unlock()

	return waitErr
}

func (s *Supervisor) forwardStream(wg *sync.WaitGroup, r io.Reader, slug, stream string) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		s.cfg.Logger.Info("extension runtime output",
			"source", "extension_runtime",
			"slug", slug,
			"stream", stream,
			"line", line,
		)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		s.cfg.Logger.Warn("extension runtime stream scan error",
			"slug", slug, "stream", stream, "error", err)
	}
}

// nextBackoff doubles the current backoff and caps at max.
func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

func readManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("supervisor: read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("supervisor: decode manifest %s: %w", path, err)
	}
	return m, nil
}
