package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"github.com/movebigrocks/platform/internal/extensionhost/supervisor"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
	"github.com/movebigrocks/platform/pkg/logger"
)

// newExtensionSupervisor builds a Supervisor from the platform config, or
// returns (nil, nil) when no runtime manifest is present at the configured
// path. A missing manifest is a legitimate state — fresh installs before
// reconcile has produced the manifest, or hosts that don't run extensions —
// and must not prevent the core from starting. Any other error (malformed
// manifest, missing declared binary, missing socket dir permissions) is
// returned to the caller and MUST fail startup; silent skip would mask the
// same sudoers-style drift RFC-0016 exists to eliminate.
func newExtensionSupervisor(cfg *config.Config, log *logger.Logger) (*supervisor.Supervisor, error) {
	manifestPath := strings.TrimSpace(cfg.ExtensionRuntimeManifestPath)
	if manifestPath == "" {
		return nil, errors.New("extension runtime manifest path is not configured")
	}
	if _, err := os.Stat(manifestPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Info("Extension runtime manifest absent; supervisor disabled",
				"path", manifestPath)
			return nil, nil
		}
		return nil, fmt.Errorf("stat extension runtime manifest: %w", err)
	}

	binaryDir := strings.TrimSpace(cfg.ExtensionRuntimeBinaryDir)
	if binaryDir == "" {
		return nil, errors.New("extension runtime binary dir is not configured")
	}
	socketDir := strings.TrimSpace(cfg.ExtensionRuntimeDir)
	if socketDir == "" {
		return nil, errors.New("extension runtime socket dir is not configured")
	}

	// The supervisor uses slog for structured logging; the platform's primary
	// logger is the zap wrapper (pkg/logger). Output both end up in the same
	// journal stream on the server (systemd captures stderr), so using
	// slog.Default() here is sufficient and avoids adding a bridge API to
	// pkg/logger for a single call site.
	_ = log // reserved for future bridging; intentionally unused for now
	return supervisor.New(supervisor.Config{
		ManifestPath: manifestPath,
		BinaryDir:    binaryDir,
		SocketDir:    socketDir,
		Logger:       slog.Default(),
	})
}
