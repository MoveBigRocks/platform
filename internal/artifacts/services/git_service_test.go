package artifactservices

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitServiceWriteHistoryAndDiff(t *testing.T) {
	service := NewGitService(t.TempDir())
	ctx := context.Background()
	repository := WorkspaceRepository("ws_123")

	first, err := service.Write(ctx, WriteParams{
		Repository:    repository,
		RelativePath:  "knowledge/teams/team_123/private/playbook.md",
		Content:       []byte("# Playbook\n\nDraft\n"),
		CommitMessage: "knowledge create playbook",
		ActorID:       "user_123",
	})
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if !first.Changed || strings.TrimSpace(first.Ref) == "" {
		t.Fatalf("expected first write to create a revision, got %#v", first)
	}

	second, err := service.Write(ctx, WriteParams{
		Repository:    repository,
		RelativePath:  "knowledge/teams/team_123/private/playbook.md",
		Content:       []byte("# Playbook\n\nPublished\n"),
		CommitMessage: "knowledge update playbook",
		ActorID:       "user_123",
	})
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if !second.Changed || second.Ref == first.Ref {
		t.Fatalf("expected a second distinct revision, got %#v", second)
	}

	history, err := service.History(ctx, repository, "knowledge/teams/team_123/private/playbook.md", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(history))
	}
	if history[0].Ref != second.Ref || history[1].Ref != first.Ref {
		t.Fatalf("unexpected history order: %#v", history)
	}

	fromRef, toRef, patch, err := service.Diff(ctx, repository, "knowledge/teams/team_123/private/playbook.md", first.Ref, second.Ref)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if fromRef != first.Ref || toRef != second.Ref {
		t.Fatalf("unexpected diff refs: from=%s to=%s", fromRef, toRef)
	}
	if !strings.Contains(patch, "-Draft") || !strings.Contains(patch, "+Published") {
		t.Fatalf("unexpected patch: %s", patch)
	}

	files, err := service.List(ctx, repository, "knowledge/teams/team_123/private")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 1 || files[0] != "knowledge/teams/team_123/private/playbook.md" {
		t.Fatalf("unexpected file list: %#v", files)
	}

	content, err := service.Read(ctx, ReadParams{
		Repository:   repository,
		RelativePath: "knowledge/teams/team_123/private/playbook.md",
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(content) != "# Playbook\n\nPublished\n" {
		t.Fatalf("unexpected content: %s", string(content))
	}
}

func TestGitServiceDelete(t *testing.T) {
	service := NewGitService(t.TempDir())
	ctx := context.Background()
	repository := WorkspaceRepository("ws_123")

	first, err := service.Write(ctx, WriteParams{
		Repository:    repository,
		RelativePath:  "knowledge/teams/team_123/private/playbook.md",
		Content:       []byte("# Playbook\n\nPublished\n"),
		CommitMessage: "knowledge create playbook",
		ActorID:       "user_123",
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	deleted, err := service.Delete(ctx, DeleteParams{
		Repository:    repository,
		RelativePath:  "knowledge/teams/team_123/private/playbook.md",
		CommitMessage: "knowledge delete playbook",
		ActorID:       "user_123",
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted.Changed || deleted.Ref == "" || deleted.Ref == first.Ref {
		t.Fatalf("expected delete to create a distinct revision, got %#v", deleted)
	}

	files, err := service.List(ctx, repository, "knowledge/teams/team_123/private")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected deleted file to be absent, got %#v", files)
	}

	if _, err := service.Read(ctx, ReadParams{
		Repository:   repository,
		RelativePath: "knowledge/teams/team_123/private/playbook.md",
	}); err == nil {
		t.Fatalf("expected read to fail after delete")
	}

	history, err := service.History(ctx, repository, "knowledge/teams/team_123/private/playbook.md", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected delete to remain visible in history, got %#v", history)
	}
	if history[0].Ref != deleted.Ref || history[1].Ref != first.Ref {
		t.Fatalf("unexpected history order after delete: %#v", history)
	}
}

func TestGitServiceRelativeRootDoesNotCreateNestedRepo(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	service := NewGitService("data/artifacts")
	ctx := context.Background()
	repository := WorkspaceRepository("ws_123")

	if _, err := service.Write(ctx, WriteParams{
		Repository:    repository,
		RelativePath:  "extensions/ats/surfaces/website/index.html",
		Content:       []byte("<html></html>\n"),
		CommitMessage: "seed ats website",
		ActorID:       "system",
	}); err != nil {
		t.Fatalf("write with relative root: %v", err)
	}

	expectedGitDir := filepath.Join(tmp, "data", "artifacts", "workspaces", "ws_123", ".git")
	if _, err := os.Stat(expectedGitDir); err != nil {
		t.Fatalf("expected git repo at %s: %v", expectedGitDir, err)
	}

	unexpectedGitDir := filepath.Join(tmp, "data", "artifacts", "workspaces", "data", "artifacts", "workspaces", "ws_123", ".git")
	if _, err := os.Stat(unexpectedGitDir); err == nil {
		t.Fatalf("unexpected nested git repo at %s", unexpectedGitDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat unexpected nested repo: %v", err)
	}
}

// TestGitServiceIsolatesFromInheritedGitEnv verifies that GitService does not
// escape into a parent GIT_DIR. Git hooks (pre-commit, post-commit, etc.) set
// GIT_DIR, GIT_WORK_TREE, and GIT_INDEX_FILE in the child process environment.
// Without sanitisation, a child `git commit` invoked via GitService would
// ignore cmd.Dir and land its commit in the hook-owning repository instead of
// the test's isolated temp repo, corrupting branch refs in the host repo.
func TestGitServiceIsolatesFromInheritedGitEnv(t *testing.T) {
	foreignDir := t.TempDir()
	if _, err := newForeignBareRepo(t, foreignDir); err != nil {
		t.Fatalf("init foreign bare repo: %v", err)
	}
	foreignGitDir := filepath.Join(foreignDir, ".git")

	t.Setenv("GIT_DIR", foreignGitDir)
	t.Setenv("GIT_WORK_TREE", foreignDir)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(foreignGitDir, "index"))

	service := NewGitService(t.TempDir())
	ctx := context.Background()
	repository := WorkspaceRepository("ws_isolate")

	result, err := service.Write(ctx, WriteParams{
		Repository:    repository,
		RelativePath:  "knowledge/teams/team_test/private/notes.md",
		Content:       []byte("# Notes\n"),
		CommitMessage: "knowledge create notes",
		ActorID:       "tester",
	})
	if err != nil {
		t.Fatalf("write under foreign GIT_DIR: %v", err)
	}
	if !result.Changed || strings.TrimSpace(result.Ref) == "" {
		t.Fatalf("expected a commit in the service's temp repo, got %#v", result)
	}

	// The foreign repo must be untouched: no refs, no commits.
	// Use an explicit GIT_DIR on sanitized env so neither the outer hook
	// context nor the test's t.Setenv GIT_DIR can lead the assertion astray.
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname)")
	cmd.Env = append(sanitizedGitEnv(os.Environ()), "GIT_DIR="+foreignGitDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("for-each-ref foreign repo: %v (output=%s)", err, strings.TrimSpace(string(out)))
	}
	refs := strings.TrimSpace(string(out))
	if refs != "" {
		t.Fatalf("foreign repo was polluted: refs %q landed in %s", refs, foreignGitDir)
	}
}

func newForeignBareRepo(t *testing.T, dir string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", "init", "-b", "main", dir)
	cmd.Env = append(sanitizedGitEnv(os.Environ()), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return dir, nil
}
