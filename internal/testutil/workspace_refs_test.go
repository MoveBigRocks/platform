package testutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWorkspaceSiblingDirMissingDefaultsToNotExist(t *testing.T) {
	t.Setenv("MBR_WORKSPACE_ROOT", "")
	t.Setenv("MOVEBIGROCKS_WORKSPACE_ROOT", "")
	t.Setenv("MBR_REQUIRE_WORKSPACE_REFS", "")
	t.Setenv("MOVEBIGROCKS_REQUIRE_WORKSPACE_REFS", "")

	root := t.TempDir()
	_, err := ResolveWorkspaceSiblingDir(root, "extensions/ats")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestResolveWorkspaceSiblingDirMissingFailsWhenRefsRequired(t *testing.T) {
	t.Setenv("MBR_WORKSPACE_ROOT", "")
	t.Setenv("MOVEBIGROCKS_WORKSPACE_ROOT", "")
	t.Setenv("MBR_REQUIRE_WORKSPACE_REFS", "true")
	t.Setenv("MOVEBIGROCKS_REQUIRE_WORKSPACE_REFS", "")

	root := t.TempDir()
	_, err := ResolveWorkspaceSiblingDir(root, "extensions/ats")
	if err == nil {
		t.Fatal("expected error when workspace refs are required")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected hard failure instead of fs.ErrNotExist, got %v", err)
	}
}

func TestResolveWorkspaceSiblingDirUsesConfiguredWorkspaceRoot(t *testing.T) {
	workspaceRoot := t.TempDir()
	target := filepath.Join(workspaceRoot, "extensions", "ats")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	t.Setenv("MBR_WORKSPACE_ROOT", workspaceRoot)
	t.Setenv("MOVEBIGROCKS_WORKSPACE_ROOT", "")
	t.Setenv("MBR_REQUIRE_WORKSPACE_REFS", "true")
	t.Setenv("MOVEBIGROCKS_REQUIRE_WORKSPACE_REFS", "")

	root := filepath.Join(t.TempDir(), "platform")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir repo root: %v", err)
	}

	got, err := ResolveWorkspaceSiblingDir(root, "extensions/ats")
	if err != nil {
		t.Fatalf("resolve workspace sibling dir: %v", err)
	}
	if got != filepath.Clean(target) {
		t.Fatalf("expected %s, got %s", filepath.Clean(target), got)
	}
}

func TestResolveWorkspaceSiblingDirConfiguredRootMissingFailsBeforeSiblingFallback(t *testing.T) {
	workspaceRoot := t.TempDir()
	localSiblingRoot := t.TempDir()
	localSibling := filepath.Join(localSiblingRoot, "extensions", "ats")
	if err := os.MkdirAll(localSibling, 0o755); err != nil {
		t.Fatalf("mkdir local sibling: %v", err)
	}

	t.Setenv("MBR_WORKSPACE_ROOT", workspaceRoot)
	t.Setenv("MOVEBIGROCKS_WORKSPACE_ROOT", "")
	t.Setenv("MBR_REQUIRE_WORKSPACE_REFS", "true")
	t.Setenv("MOVEBIGROCKS_REQUIRE_WORKSPACE_REFS", "")

	root := filepath.Join(localSiblingRoot, "platform")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir repo root: %v", err)
	}

	_, err := ResolveWorkspaceSiblingDir(root, "extensions/ats")
	if err == nil {
		t.Fatal("expected configured workspace root miss to fail")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected hard failure instead of fs.ErrNotExist, got %v", err)
	}
}

func TestRequireWorkspaceRefsTruthTable(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		value   string
		wantSet bool
	}{
		{name: "empty", value: "", wantSet: false},
		{name: "true", key: "MBR_REQUIRE_WORKSPACE_REFS", value: "true", wantSet: true},
		{name: "one", key: "MBR_REQUIRE_WORKSPACE_REFS", value: "1", wantSet: true},
		{name: "yes", key: "MOVEBIGROCKS_REQUIRE_WORKSPACE_REFS", value: "yes", wantSet: true},
		{name: "off", key: "MOVEBIGROCKS_REQUIRE_WORKSPACE_REFS", value: "off", wantSet: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MBR_REQUIRE_WORKSPACE_REFS", "")
			t.Setenv("MOVEBIGROCKS_REQUIRE_WORKSPACE_REFS", "")
			if tc.key != "" {
				t.Setenv(tc.key, tc.value)
			}
			if got := RequireWorkspaceRefs(); got != tc.wantSet {
				t.Fatalf("expected %t, got %t", tc.wantSet, got)
			}
		})
	}
}
