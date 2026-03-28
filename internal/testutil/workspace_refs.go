package testutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ResolveWorkspaceSiblingDir locates a sibling repository/directory in a full Move Big Rocks workspace.
// Callers may require these refs explicitly by setting MBR_REQUIRE_WORKSPACE_REFS=true.
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

	if RequireWorkspaceRefs() {
		return "", fmt.Errorf("required workspace sibling checkout not available for %s", rel)
	}

	return "", fs.ErrNotExist
}

// RequireWorkspaceRefs reports whether tests should fail instead of skip when
// canonical sibling repositories are missing from the local workspace.
func RequireWorkspaceRefs() bool {
	for _, key := range []string{"MBR_REQUIRE_WORKSPACE_REFS", "MOVEBIGROCKS_REQUIRE_WORKSPACE_REFS"} {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}
