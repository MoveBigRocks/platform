package testutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ResolveWorkspaceSiblingDir locates a sibling repository/directory in a full Move Big Rocks workspace.
// Standalone checkouts such as the platform CI job won't have these siblings available.
func ResolveWorkspaceSiblingDir(repoRoot, rel string) (string, error) {
	repoRoot = filepath.Clean(repoRoot)
	rel = filepath.Clean(rel)

	candidates := make([]string, 0, 3)
	if root := strings.TrimSpace(os.Getenv("MBR_WORKSPACE_ROOT")); root != "" {
		candidates = append(candidates, filepath.Join(root, rel))
	}
	if root := strings.TrimSpace(os.Getenv("MOVEBIGROCKS_WORKSPACE_ROOT")); root != "" {
		candidates = append(candidates, filepath.Join(root, rel))
	}
	candidates = append(candidates, filepath.Join(filepath.Dir(repoRoot), rel))

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return filepath.Clean(candidate), nil
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}

	return "", fs.ErrNotExist
}
